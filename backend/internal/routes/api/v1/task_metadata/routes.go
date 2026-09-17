package task_metadata

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/mappers"
	"github.com/jfxdev/gardarr/internal/middlewares"
	task_metadata_service "github.com/jfxdev/gardarr/internal/services/task_metadata"
)

const (
	// MaxImageUploadSize defines the maximum allowed image upload size (5MB)
	MaxImageUploadSize = 5 * 1024 * 1024 // 5MB

	applyProviderMetadataError = "failed to apply provider metadata"
	maxReleaseParseFileSize    = 5 << 20
	maxReleaseParseRequestSize = maxReleaseParseFileSize + (1 << 20)
)

var (
	// AllowedImageMIMETypes defines the permitted image MIME types
	AllowedImageMIMETypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
)

// Module holds task metadata routes configuration
type Module struct {
	group   *gin.RouterGroup
	service *task_metadata_service.Service
	db      *database.Database
}

// NewModule creates a new task metadata module
func NewModule(router *gin.RouterGroup, db *database.Database, service *task_metadata_service.Service) *Module {
	return &Module{
		group:   router.Group("/tasks/metadata"),
		service: service,
		db:      db,
	}
}

// Register registers all task metadata routes
func (m *Module) Register() {
	// Protected routes - require authentication
	protected := m.group.Group("")
	protected.Use(middlewares.SessionMiddleware(m.db))

	// Metadata management - write operations
	protected.PUT("/:task_hash/description", m.updateTaskDescription)
	protected.PUT("/:task_hash/name", m.updateTaskName)
	protected.PUT("/:task_hash/position", m.updateImagePosition)
	protected.PUT("/:task_hash/brightness", m.updateImageBrightness)

	// Image management - write operations
	protected.POST("/:task_hash/image", m.uploadTaskImage)
	protected.DELETE("/:task_hash/image", m.deleteTaskImage)

	// Image serving - read operations (also protected)
	protected.GET("/:task_hash/image", m.getTaskImage)
	protected.GET("/:task_hash/thumbnail", m.getTaskThumbnail)

	// External metadata providers
	protected.POST("/release-parse", m.parseRelease)
	protected.POST("/release-parse/file", m.parseReleaseFile)
	protected.GET("/providers/:provider/status", m.providerStatus)
	protected.GET("/providers/:provider/image", m.getProviderImage)
	protected.GET("/:task_hash/providers/:provider/search", m.searchProvider)
	protected.POST("/:task_hash/providers/:provider", m.applyProvider)
}

// parseRelease previews deterministic release-name suggestions. It does not
// persist or contact any external metadata provider.
func (m *Module) parseRelease(c *gin.Context) {
	var body struct {
		RawName string `json:"raw_name" binding:"required,max=1000"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "raw_name is required"})
		return
	}

	result := m.service.ParseRelease(body.RawName)
	c.JSON(http.StatusOK, gin.H{
		"release":      result,
		"display_name": result.DisplayName(),
		"tags":         result.SuggestedTags(),
	})
}

func (m *Module) parseReleaseFile(c *gin.Context) {
	// FormFile parses the whole multipart body before returning its header, so
	// bound the request itself as well as the extracted torrent file.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxReleaseParseRequestSize)
	file, header, err := c.Request.FormFile("torrent")
	if err != nil || header.Size > maxReleaseParseFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid torrent file is required"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxReleaseParseFileSize+1))
	if err != nil || len(data) > maxReleaseParseFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid torrent file is required"})
		return
	}
	result, err := m.service.ParseReleaseFile(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"release": result, "display_name": result.DisplayName(), "tags": result.SuggestedTags()})
}

// uploadTaskImage handles image upload for a task
func (m *Module) uploadTaskImage(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "image file is required",
		})
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > MaxImageUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("image file exceeds maximum size of %dMB", MaxImageUploadSize/1024/1024),
		})
		return
	}

	// Validate Content-Type header
	contentType := header.Header.Get("Content-Type")
	if !AllowedImageMIMETypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid image type; only JPEG, PNG, GIF, and WebP are allowed",
		})
		return
	}

	// Sniff file content to confirm actual MIME type
	// Read first 512 bytes for detection (http.DetectContentType uses up to 512 bytes)
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to read image file",
		})
		return
	}

	// Detect actual content type from file content
	detectedType := http.DetectContentType(buffer[:n])
	if !AllowedImageMIMETypes[detectedType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "image file content does not match allowed types",
		})
		return
	}

	// Reset file pointer to beginning for service to read
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to process image file",
		})
		return
	}

	// Upload validated image (size already checked via header.Size)
	metadata, err := m.service.UploadImage(c.Request.Context(), taskHash, file, header)
	if err != nil {
		slog.Error("failed to upload image", "error", err, "task_hash", taskHash)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload image"})
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskMetadataResponse(metadata))
}

// updateTaskDescription updates the description of task metadata
func (m *Module) updateTaskDescription(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Parse request body
	var body struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Update description
	metadata, err := m.service.UpdateDescription(c.Request.Context(), taskHash, body.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskMetadataResponse(metadata))
}

// updateTaskName updates the name of task metadata
func (m *Module) updateTaskName(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Parse request body
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Update name
	metadata, err := m.service.UpdateName(c.Request.Context(), taskHash, body.Name)
	if err != nil {
		slog.Error("failed to update task name", "error", err, "task_hash", taskHash)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "unable to update task name",
		})
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskMetadataResponse(metadata))
}

// updateImagePosition updates the position of the image
func (m *Module) updateImagePosition(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Parse request body
	var body struct {
		ImagePositionY float64 `json:"image_position_y"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Update position
	metadata, err := m.service.UpdateImagePosition(c.Request.Context(), taskHash, body.ImagePositionY)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskMetadataResponse(metadata))
}

// updateImageBrightness updates the brightness of the image
func (m *Module) updateImageBrightness(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Parse request body
	var body struct {
		ImageBrightness float64 `json:"image_brightness"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Update brightness
	metadata, err := m.service.UpdateImageBrightness(c.Request.Context(), taskHash, body.ImageBrightness)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskMetadataResponse(metadata))
}

// deleteTaskImage deletes the image from task metadata
func (m *Module) deleteTaskImage(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Delete image
	if err := m.service.DeleteImage(c.Request.Context(), taskHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// getTaskImage serves the task image
func (m *Module) getTaskImage(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Validate task hash to prevent path traversal in URL parameter
	if strings.Contains(taskHash, "..") || strings.Contains(taskHash, "/") || strings.Contains(taskHash, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid task_hash",
		})
		return
	}

	// Get image path (service validates path is within upload directory)
	imagePath, err := m.service.GetImagePath(c.Request.Context(), taskHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "image not found",
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "image file not found",
		})
		return
	}

	// Serve file (path is validated to be within upload directory)
	c.File(imagePath)
}

// getTaskThumbnail serves a thumbnail of the task image
func (m *Module) getTaskThumbnail(c *gin.Context) {
	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "task_hash is required",
		})
		return
	}

	// Validate task hash to prevent path traversal in URL parameter
	if strings.Contains(taskHash, "..") || strings.Contains(taskHash, "/") || strings.Contains(taskHash, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid task_hash",
		})
		return
	}

	// Get image path (service validates path is within upload directory)
	imagePath, err := m.service.GetImagePath(c.Request.Context(), taskHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "image not found",
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "image file not found",
		})
		return
	}

	c.File(imagePath)
}

// providerStatus returns the status of an external metadata provider
func (m *Module) providerStatus(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	status, err := m.service.GetProviderStatus(c.Request.Context(), provider)
	if err != nil {
		if errors.Is(err, task_metadata_service.ErrProviderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, status)
}

// getProviderImage proxies a validated provider image through Gardarr. This
// keeps previews same-origin for pages protected by COEP/COOP headers.
func (m *Module) getProviderImage(c *gin.Context) {
	provider := c.Param("provider")
	imageID := c.Query("image_id")
	if provider == "" || strings.TrimSpace(imageID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider and image_id are required"})
		return
	}

	body, contentType, err := m.service.GetProviderImage(c.Request.Context(), provider, imageID)
	if err != nil {
		switch {
		case errors.Is(err, task_metadata_service.ErrProviderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, task_metadata_service.ErrProviderImageInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			slog.Warn("failed to proxy provider image", "provider", provider, "error", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load provider image"})
		}
		return
	}

	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, contentType, body)
}

// searchProvider searches an external metadata provider
func (m *Module) searchProvider(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	// "auto" is only set by callers doing an initial search from the
	// torrent's raw release name, which release-parsing turns into a
	// cleaner query; a manually typed/edited re-search must be sent to the
	// provider verbatim, or the parser can silently drop words the user
	// intentionally typed.
	var results []task_metadata_service.MetadataProviderSearchResult
	var err error
	if c.Query("auto") == "true" {
		results, err = m.service.SearchProviderAuto(c.Request.Context(), provider, query)
	} else {
		results, err = m.service.SearchProvider(c.Request.Context(), provider, query)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// applyProvider applies provider metadata to a task
func (m *Module) applyProvider(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	taskHash := c.Param("task_hash")
	if taskHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_hash is required"})
		return
	}

	var body struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		ReleaseDate string `json:"release_date"`
		Description string `json:"description"`
		ImageID     string `json:"image_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var fallbackSelection *task_metadata_service.MetadataProviderSelection
	if strings.TrimSpace(body.Title) != "" ||
		strings.TrimSpace(body.ReleaseDate) != "" ||
		strings.TrimSpace(body.Description) != "" ||
		strings.TrimSpace(body.ImageID) != "" {
		fallbackSelection = &task_metadata_service.MetadataProviderSelection{
			ID:          body.ID,
			Title:       body.Title,
			ReleaseDate: body.ReleaseDate,
			Description: body.Description,
			ImageID:     body.ImageID,
		}
	}

	metadata, err := m.service.ApplyProviderSelectionWithFallback(c.Request.Context(), provider, taskHash, body.ID, fallbackSelection)
	if err != nil {
		slog.Error(applyProviderMetadataError, "provider", provider, "task_hash", taskHash, "selection_id", body.ID, "error", err)
		switch {
		case errors.Is(err, task_metadata_service.ErrProviderNotFound), errors.Is(err, task_metadata_service.ErrProviderSelectionNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": applyProviderMetadataError, "reason": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": applyProviderMetadataError, "reason": err.Error()})
		}
		return
	}

	response := mappers.ToTaskMetadataResponse(metadata.Metadata)
	response.Warning = metadata.Warning
	response.WarningReason = metadata.WarningReason
	c.JSON(http.StatusOK, response)
}
