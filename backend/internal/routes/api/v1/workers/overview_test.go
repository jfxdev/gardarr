package workers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/services/crypto"
	"github.com/jfxdev/gardarr/internal/services/workermanager"
)

func TestOverviewRequiresSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewModule(router.Group("/v1"), nil, nil, nil).Register()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/worker/"+"00000000-0000-0000-0000-000000000001"+"/overview", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverviewAuthenticatedResponseUsesOneTorrentListing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	var listings atomic.Int32
	qbt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "test", Path: "/", HttpOnly: true, Secure: true})
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/app/version":
			_, _ = w.Write([]byte("v5.1.0"))
		case "/api/v2/app/webapiVersion":
			_, _ = w.Write([]byte("2.11"))
		case "/api/v2/sync/maindata":
			_, _ = w.Write([]byte(`{"server_state":{"free_space_on_disk":9999,"global_ratio":"1.5"}}`))
		case "/api/v2/torrents/info":
			listings.Add(1)
			const hash = "5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36"
			_ = json.NewEncoder(w).Encode([]map[string]any{{"hash": hash, "magnet_uri": "magnet:?xt=urn:btih:" + hash, "size": 7000000000, "completed": int64(6000000000), "downloaded": 0, "state": "uploading", "dlspeed": 100, "upspeed": 200}})
		}
	}))
	defer qbt.Close()
	db := database.SetupTestDBWithCache(t, &models.User{}, &models.Session{}, &models.Worker{}, &models.Category{})
	c, err := crypto.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := workermanager.NewService(db, c, "", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := svc.CreateWorker(context.Background(), &schemas.WorkerCreateSchema{Name: "Fixture", Type: "qbittorrent", URL: qbt.URL, Username: "fixture", Password: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.DeleteWorker(context.Background(), worker.UUID.String()) })
	router := gin.New()
	NewModule(router.Group("/v1"), db, svc, nil).Register()
	token := createTestSession(t, db, "overview-fixture@example.test", "user")
	req := httptest.NewRequest(http.MethodGet, "/v1/worker/"+worker.UUID.String()+"/overview", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName, Value: token, HttpOnly: true, Secure: true})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for regular user, got %d: %s", w.Code, w.Body.String())
	}
	var result models.WorkerOverviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Worker.Status != "ACTIVE" || result.Worker.Instance.Server.FreeSpaceOnDisk != 9999 || result.CollectedAt.IsZero() {
		t.Fatalf("invalid health/collection: %#v", result)
	}
	if result.Metrics.StoredBytes == nil || *result.Metrics.StoredBytes != 6000000000 || *result.Metrics.TotalTorrents != 1 || *result.Metrics.ActiveTorrents != 1 || *result.Metrics.DownloadSpeed != 100 || *result.Metrics.UploadSpeed != 200 {
		t.Fatalf("invalid metrics: %#v", result.Metrics)
	}
	if listings.Load() != 1 {
		t.Fatalf("expected one torrent listing, got %d", listings.Load())
	}
}

func TestOverviewUnknownMetricsSerializeAsNull(t *testing.T) {
	data, err := json.Marshal(models.WorkerOverviewResponse{})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Metrics map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"stored_bytes", "total_torrents", "active_torrents", "download_speed", "upload_speed"} {
		if value, present := result.Metrics[field]; !present || value != nil {
			t.Fatalf("%s must be explicit null: %s", field, data)
		}
	}
}
