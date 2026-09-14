package category

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
)

// setupTestRouter creates a test router with the category routes
func setupTestRouter(t *testing.T) (*gin.Engine, *database.Database) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	db := database.SetupTestDBWithCache(t, &models.Category{}, &models.User{}, &models.Session{})

	// Create the API v1 group
	v1 := router.Group("/api/v1")

	// Register category routes without middleware for testing
	module := NewModule(v1, db)
	categoriesGroup := v1.Group("/categories")

	categoriesGroup.POST("", module.createCategory)
	categoriesGroup.GET("", module.listCategories)
	categoriesGroup.GET("/:id", module.getCategoryByID)
	categoriesGroup.PUT("/:id", module.updateCategory)
	categoriesGroup.DELETE("/:id", module.deleteCategory)

	return router, db
}

// sendJSONRequest is a helper to reduce test duplication
func sendJSONRequest(router *gin.Engine, method, url string, body interface{}) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestRoutes_CreateCategory_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := map[string]interface{}{
		"name":                "Test Category",
		"default_tags":        []string{"tag1", "tag2"},
		"default_directories": []string{"/path1", "/path2"},
		"metadata_source":     "tgdb",
		"color":               "#FF5733",
		"icon":                "folder-icon",
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Name != "Test Category" {
		t.Errorf("Expected name 'Test Category', got '%s'", response.Name)
	}

	if response.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if response.Color != "#FF5733" {
		t.Errorf("Expected color '#FF5733', got '%s'", response.Color)
	}

	if response.Icon != "folder-icon" {
		t.Errorf("Expected icon 'folder-icon', got '%s'", response.Icon)
	}

	if response.MetadataSource != "tgdb" {
		t.Errorf("Expected metadata source 'tgdb', got '%s'", response.MetadataSource)
	}

	if len(response.DefaultDirectories) != 2 {
		t.Errorf("Expected 2 default directories, got %v", response.DefaultDirectories)
	}
}

func TestRoutes_CreateCategory_ValidationError(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Missing required field 'name'
	body := map[string]interface{}{
		"default_tags": []string{"tag1"},
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRoutes_ListCategories(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create some categories first
	categories := []map[string]interface{}{
		{"name": "Category 1", "default_tags": []string{"tag1"}},
		{"name": "Category 2", "default_tags": []string{"tag2"}},
	}

	for _, cat := range categories {
		sendJSONRequest(router, "POST", "/api/v1/categories", cat)
	}

	// List all categories
	w := sendJSONRequest(router, "GET", "/api/v1/categories", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(response))
	}
}

func TestRoutes_GetCategoryByID_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create a category
	body := map[string]interface{}{
		"name":         "Test Category",
		"default_tags": []string{"tag1"},
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	var created models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Get by ID
	w = sendJSONRequest(router, "GET", "/api/v1/categories/"+created.ID, nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, response.ID)
	}
}

func TestRoutes_GetCategoryByID_NotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	w := sendJSONRequest(router, "GET", "/api/v1/categories/non-existent-id", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRoutes_UpdateCategory_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create a category
	body := map[string]interface{}{
		"name":         "Immutable Name",
		"default_tags": []string{"tag1"},
		"color":        "#FF0000",
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	var created models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Update the category (only mutable fields)
	updateBody := map[string]interface{}{
		"default_tags":        []string{"tag1", "tag2"},
		"default_directories": []string{"/updated/path", "/alternate/path"},
		"metadata_source":     "tgdb",
		"color":               "#00FF00",
		"icon":                "new-icon",
	}

	w = sendJSONRequest(router, "PUT", "/api/v1/categories/"+created.ID, updateBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Name should remain immutable
	if response.Name != "Immutable Name" {
		t.Errorf("Expected name to remain 'Immutable Name', got '%s'", response.Name)
	}

	// Mutable fields should be updated
	if response.Color != "#00FF00" {
		t.Errorf("Expected color '#00FF00', got '%s'", response.Color)
	}

	if response.Icon != "new-icon" {
		t.Errorf("Expected icon 'new-icon', got '%s'", response.Icon)
	}

	if len(response.DefaultDirectories) != 2 || response.DefaultDirectories[0] != "/updated/path" {
		t.Errorf("Expected updated default directories, got %v", response.DefaultDirectories)
	}

	if response.MetadataSource != "tgdb" {
		t.Errorf("Expected metadata source 'tgdb', got '%s'", response.MetadataSource)
	}

	if len(response.DefaultTags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(response.DefaultTags))
	}
}

func TestRoutes_UpdateCategory_NameIsImmutable(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create a category
	body := map[string]interface{}{
		"name":         "Original Name",
		"default_tags": []string{"tag1"},
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	var created models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Try to update the name (should be ignored)
	updateBody := map[string]interface{}{
		"name":         "Attempted New Name",
		"default_tags": []string{"tag1", "tag2"},
	}

	w = sendJSONRequest(router, "PUT", "/api/v1/categories/"+created.ID, updateBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Name should remain unchanged (immutable)
	if response.Name != "Original Name" {
		t.Errorf("Expected name to remain 'Original Name', got '%s'", response.Name)
	}
}

func TestRoutes_UpdateCategory_NotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := map[string]interface{}{
		"default_tags": []string{"tag1"},
	}

	w := sendJSONRequest(router, "PUT", "/api/v1/categories/non-existent-id", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRoutes_DeleteCategory_Success(t *testing.T) {
	router, _ := setupTestRouter(t)

	// Create a category
	body := map[string]interface{}{
		"name":         "To Be Deleted",
		"default_tags": []string{"tag1"},
	}

	w := sendJSONRequest(router, "POST", "/api/v1/categories", body)

	var created models.CategoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Delete the category
	w = sendJSONRequest(router, "DELETE", "/api/v1/categories/"+created.ID, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Verify deletion
	w = sendJSONRequest(router, "GET", "/api/v1/categories/"+created.ID, nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d after deletion, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRoutes_DeleteCategory_NotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	w := sendJSONRequest(router, "DELETE", "/api/v1/categories/non-existent-id", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
