package task_metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	taskmetadatasvc "github.com/jfxdev/gardarr/internal/services/task_metadata"
)

type routeMockProvider struct {
	selection  *taskmetadatasvc.MetadataProviderSelection
	resolveErr error
}

const taskMetadataApplyURL = "/api/v1/tasks/metadata/task-123/providers/tgdb"

var fallbackProviderPayload = map[string]string{
	"id":           "123",
	"title":        "Fallback Game",
	"release_date": "2024-01-01",
	"description":  "Overview",
}

func (m routeMockProvider) Name() string {
	return "tgdb"
}

func (m routeMockProvider) Status(_ context.Context) (*taskmetadatasvc.MetadataProviderStatus, error) {
	return &taskmetadatasvc.MetadataProviderStatus{Provider: "tgdb", Active: true}, nil
}

func (m routeMockProvider) Search(_ context.Context, _ string) ([]taskmetadatasvc.MetadataProviderSearchResult, error) {
	return nil, nil
}

func (m routeMockProvider) Resolve(_ context.Context, _ string) (*taskmetadatasvc.MetadataProviderSelection, error) {
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	return m.selection, nil
}

func (m routeMockProvider) BuildImageURL(imageID string) (string, error) {
	return "https://cdn.thegamesdb.net/images/large/" + imageID, nil
}

func (m routeMockProvider) AllowedImageHosts() []string {
	return []string{"cdn.thegamesdb.net"}
}

func setupTaskMetadataApplyRouter(t *testing.T, provider taskmetadatasvc.MetadataProvider) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	registry := taskmetadatasvc.NewMetadataProviderRegistry(provider)
	service, err := taskmetadatasvc.NewService(db, "http://localhost:3200", t.TempDir(), registry)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	module := NewModule(router.Group("/api/v1"), db, service)
	router.POST("/api/v1/tasks/metadata/:task_hash/providers/:provider", module.applyProvider)

	return router
}

func setupTaskMetadataImageRouter(t *testing.T, providers ...taskmetadatasvc.MetadataProvider) (*gin.Engine, *Module) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	service, err := taskmetadatasvc.NewService(
		db,
		"http://localhost:3200",
		t.TempDir(),
		taskmetadatasvc.NewMetadataProviderRegistry(providers...),
	)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	module := NewModule(router.Group("/api/v1"), db, service)
	router.GET("/api/v1/tasks/metadata/providers/:provider/image", module.getProviderImage)
	return router, module
}

func getTaskMetadataImage(t *testing.T, router *gin.Engine, provider, imageID string) *httptest.ResponseRecorder {
	t.Helper()

	url := "/api/v1/tasks/metadata/providers/" + provider + "/image"
	if imageID != "" {
		url += "?image_id=" + imageID
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func sendTaskMetadataJSONRequest(t *testing.T, router *gin.Engine, method, url string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func applyProviderRoute(t *testing.T, provider routeMockProvider, payload map[string]string) models.TaskMetadataResponse {
	t.Helper()

	router := setupTaskMetadataApplyRouter(t, provider)
	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, taskMetadataApplyURL, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.TaskMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return response
}

func TestParseReleaseRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	service, err := taskmetadatasvc.NewService(db, "http://localhost:3200", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	module := NewModule(router.Group("/api/v1"), db, service)
	router.POST("/api/v1/tasks/metadata/release-parse", module.parseRelease)

	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/metadata/release-parse", map[string]string{
		"raw_name": "The.Matrix.1999.2160p.BluRay.x265-GROUP",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var response struct {
		DisplayName string   `json:"display_name"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.DisplayName != "The Matrix (1999)" {
		t.Fatalf("display_name = %q", response.DisplayName)
	}
	if !maps.Equal(map[string]bool{"quality::2160p": true, "source::bluray": true, "codec::x265": true, "group::group": true}, sliceSet(response.Tags)) {
		t.Fatalf("tags = %v", response.Tags)
	}
}

func TestParseReleaseFileRouteRejectsOversizedMultipartBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	service, err := taskmetadatasvc.NewService(db, "http://localhost:3200", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	module := NewModule(router.Group("/api/v1"), db, service)
	router.POST("/api/v1/tasks/metadata/release-parse/file", module.parseReleaseFile)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("torrent", "large.torrent")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := file.Write(bytes.Repeat([]byte{'x'}, maxReleaseParseRequestSize)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/metadata/release-parse/file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func sliceSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func TestApplyProviderRouteSuccess(t *testing.T) {
	router := setupTaskMetadataApplyRouter(t, routeMockProvider{
		selection: &taskmetadatasvc.MetadataProviderSelection{
			ID:          "123",
			Title:       "Test Game",
			ReleaseDate: "2024-01-01",
			Description: "Overview",
		},
	})

	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/metadata/task-123/providers/tgdb", map[string]string{
		"id": "123",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.TaskMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.TaskHash != "task-123" || response.Name != "Test Game" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestApplyProviderRouteRequiresID(t *testing.T) {
	router := setupTaskMetadataApplyRouter(t, routeMockProvider{})

	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/metadata/task-123/providers/tgdb", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestApplyProviderRouteReturnsNotFoundForUnknownProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	service, err := taskmetadatasvc.NewService(db, "http://localhost:3200", t.TempDir(), taskmetadatasvc.NewMetadataProviderRegistry())
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	module := NewModule(router.Group("/api/v1"), db, service)
	router.POST("/api/v1/tasks/metadata/:task_hash/providers/:provider", module.applyProvider)

	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/metadata/task-123/providers/tgdb", map[string]string{
		"id": "123",
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestApplyProviderRouteReturnsNotFoundForUnknownSelection(t *testing.T) {
	router := setupTaskMetadataApplyRouter(t, routeMockProvider{
		resolveErr: taskmetadatasvc.ErrProviderSelectionNotFound,
	})

	w := sendTaskMetadataJSONRequest(t, router, http.MethodPost, "/api/v1/tasks/metadata/task-123/providers/tgdb", map[string]string{
		"id": "404",
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestApplyProviderRouteFallsBackToProvidedSelectionOnResolveError(t *testing.T) {
	response := applyProviderRoute(t, routeMockProvider{
		resolveErr: fmt.Errorf("unexpected status code: 404"),
	}, fallbackProviderPayload)

	if response.TaskHash != "task-123" || response.Name != "Fallback Game" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.Warning == "" || response.WarningReason == "" {
		t.Fatalf("expected warning fields to be set, got %#v", response)
	}
}

func TestApplyProviderRouteIgnoresLegacyImageURLField(t *testing.T) {
	payload := maps.Clone(fallbackProviderPayload)
	payload["image_url"] = "https://cdn.thegamesdb.net/images/large/front.jpg"

	response := applyProviderRoute(t, routeMockProvider{
		resolveErr: fmt.Errorf("unexpected status code: 404"),
	}, payload)

	if response.ImageURL != "" {
		t.Fatalf("expected legacy image_url payload to be ignored, got image_url=%q", response.ImageURL)
	}
	if response.Name != "Fallback Game" {
		t.Fatalf("expected fallback metadata to be applied, got %#v", response)
	}
}

func TestGetProviderImageRouteRequiresImageID(t *testing.T) {
	router, _ := setupTaskMetadataImageRouter(t, routeMockProvider{})
	w := getTaskMetadataImage(t, router, "tgdb", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestGetProviderImageRouteReturnsNotFoundForUnknownProvider(t *testing.T) {
	router, _ := setupTaskMetadataImageRouter(t)
	w := getTaskMetadataImage(t, router, "unknown", "front.jpg")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestGetProviderImageRouteRejectsInvalidImageID(t *testing.T) {
	router, _ := setupTaskMetadataImageRouter(t, routeMockProvider{})
	w := getTaskMetadataImage(t, router, "tgdb", strings.Repeat("x", 1025))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestGetProviderImageRouteReturnsBadGatewayForProviderFailure(t *testing.T) {
	router, _ := setupTaskMetadataImageRouter(t, routeMockProvider{})
	w := getTaskMetadataImage(t, router, "tgdb", "front.jpg%3Ftoken=unexpected")

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadGateway, w.Body.String())
	}
}

func TestRegisterIncludesProviderImageRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDB(t, &models.TaskMetadata{})
	service, err := taskmetadatasvc.NewService(
		db,
		"http://localhost:3200",
		t.TempDir(),
		taskmetadatasvc.NewMetadataProviderRegistry(routeMockProvider{}),
	)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	module := NewModule(router.Group("/api/v1"), db, service)
	module.Register()

	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/tasks/metadata/providers/:provider/image" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("provider image route was not registered")
	}
}
