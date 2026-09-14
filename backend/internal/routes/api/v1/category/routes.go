package category

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/services/category"
)

// Module holds category routes configuration
type Module struct {
	group   *gin.RouterGroup
	service *category.Service
	db      *database.Database
}

// NewModule creates a new category module
func NewModule(router *gin.RouterGroup, db *database.Database) *Module {
	return &Module{
		group:   router.Group("/categories"),
		service: category.NewService(db),
		db:      db,
	}
}

// Register registers all category routes
func (m *Module) Register() {
	// Apply authentication middleware to all routes
	m.group.Use(middlewares.SessionMiddleware(m.db))

	m.group.POST("", m.createCategory)
	m.group.GET("", m.listCategories)
	m.group.GET("/:id", m.getCategoryByID)
	m.group.PUT("/:id", m.updateCategory)
	m.group.DELETE("/:id", m.deleteCategory)
}

// createCategory creates a new category
func (m *Module) createCategory(c *gin.Context) {
	var body schemas.CategoryCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	category := entities.Category{
		Name:               body.Name,
		DefaultTags:        body.DefaultTags,
		DefaultDirectories: body.DefaultDirectories,
		MetadataSource:     defaultMetadataSource(body.MetadataSource),
		ReleaseType:        defaultReleaseType(body.ReleaseType),
		Color:              body.Color,
		Icon:               body.Icon,
	}

	created, err := m.service.CreateCategory(c.Request.Context(), category)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "category with this name already exists" {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, m.toResponse(created))
}

// listCategories retrieves all categories
func (m *Module) listCategories(c *gin.Context) {
	categories, err := m.service.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve categories"})
		return
	}

	response := make([]models.CategoryResponse, len(categories))
	for i, cat := range categories {
		response[i] = m.toResponse(cat)
	}

	c.JSON(http.StatusOK, response)
}

// getCategoryByID retrieves a category by its ID
func (m *Module) getCategoryByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	category, err := m.service.GetCategoryByID(c.Request.Context(), id)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "category not found" {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, m.toResponse(category))
}

// updateCategory updates an existing category
func (m *Module) updateCategory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	var body schemas.CategoryUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	// Get existing category to preserve unchanged fields
	existing, err := m.service.GetCategoryByID(c.Request.Context(), id)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "category not found" {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// Update only mutable fields (name and ID are immutable)
	updated := entities.Category{
		ID:                 id,
		Name:               existing.Name, // Name is immutable
		DefaultTags:        existing.DefaultTags,
		DefaultDirectories: existing.DefaultDirectories,
		MetadataSource:     existing.MetadataSource,
		ReleaseType:        existing.ReleaseType,
		Color:              existing.Color,
		Icon:               existing.Icon,
	}

	// Update only provided fields
	if body.DefaultTags != nil {
		updated.DefaultTags = body.DefaultTags
	}
	if body.DefaultDirectories != nil {
		updated.DefaultDirectories = body.DefaultDirectories
	}
	if body.MetadataSource != "" {
		updated.MetadataSource = body.MetadataSource
	}
	if body.ReleaseType != "" {
		updated.ReleaseType = body.ReleaseType
	}
	if body.Color != "" {
		updated.Color = body.Color
	}
	if body.Icon != "" {
		updated.Icon = body.Icon
	}

	result, err := m.service.UpdateCategory(c.Request.Context(), updated)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "category not found" {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, m.toResponse(result))
}

// deleteCategory removes a category by ID
func (m *Module) deleteCategory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category ID is required"})
		return
	}

	if err := m.service.DeleteCategory(c.Request.Context(), id); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "category not found" {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// toResponse converts an entity to a response model
func (m *Module) toResponse(cat *entities.Category) models.CategoryResponse {
	return models.CategoryResponse{
		ID:                 cat.ID,
		Name:               cat.Name,
		DefaultTags:        cat.DefaultTags,
		DefaultDirectories: models.StringArray(cat.DefaultDirectories),
		MetadataSource:     defaultMetadataSource(cat.MetadataSource),
		ReleaseType:        defaultReleaseType(cat.ReleaseType),
		Color:              cat.Color,
		Icon:               cat.Icon,
		CreatedAt:          cat.CreatedAt,
		UpdatedAt:          cat.UpdatedAt,
	}
}

func defaultMetadataSource(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// validReleaseTypes mirrors the schema's oneof binding for release_type.
var validReleaseTypes = map[string]bool{
	"none": true, "movie": true, "series": true, "os": true, "game": true,
	"book": true, "music": true, "software": true, "audiobook": true,
	"comic": true, "course": true, "dataset": true, "rom": true,
	"podcast": true, "anime": true,
}

// defaultReleaseType falls back to "none" for both an empty value and any
// value outside the schema's allowed set, guarding responses against stale
// release_type data left behind by a schema change or a direct DB edit.
func defaultReleaseType(value string) string {
	if !validReleaseTypes[value] {
		return "none"
	}
	return value
}
