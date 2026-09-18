package task_metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	task_metadata_repo "github.com/jfxdev/gardarr/internal/repository/task_metadata"
	"github.com/jfxdev/gardarr/internal/services/releaseparse"
	"github.com/jfxdev/gardarr/pkg/torrentfile"
)

const (
	MaxFileSize      = 10 << 20 // 10 MB
	AllowedMimeTypes = "image/jpeg,image/png,image/gif,image/webp"
	httpTimeout      = 10 * time.Second
)

var validExternalImagePath = regexp.MustCompile(`^/[A-Za-z0-9._~/%+-]+$`)

// validateTaskHash ensures the provided taskHash contains only safe characters (no slashes, no traversal)
func validateTaskHash(taskHash string) error {
	// Only allow alphanumeric, `_`, and `-`
	var validTaskHash = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validTaskHash.MatchString(taskHash) {
		return fmt.Errorf("invalid task_hash value")
	}
	return nil
}

// Service handles task metadata operations
type Service struct {
	repo             *task_metadata_repo.Repository
	db               *database.Database
	uploadDir        string
	baseURL          string
	providerRegistry *MetadataProviderRegistry
	httpClient       *http.Client
	lookupIPAddr     func(context.Context, string) ([]net.IPAddr, error)
}

type ApplyProviderSelectionResult struct {
	Metadata      *entities.TaskMetadata
	Warning       string
	WarningReason string
}

type resolvedProviderSelection struct {
	selection     *MetadataProviderSelection
	warning       string
	warningReason string
}

// NewService creates a new task metadata service
func NewService(db *database.Database, baseURL, uploadDir string, providerRegistry *MetadataProviderRegistry) (*Service, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		// Log error but continue
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	if providerRegistry == nil {
		providerRegistry = NewMetadataProviderRegistry()
	}

	return &Service{
		repo:             task_metadata_repo.NewRepository(db),
		db:               db,
		uploadDir:        uploadDir,
		baseURL:          baseURL,
		providerRegistry: providerRegistry,
		httpClient: &http.Client{
			Timeout: httpTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		lookupIPAddr: net.DefaultResolver.LookupIPAddr,
	}, nil
}

// GetByTaskHash retrieves metadata for a task
func (s *Service) GetByTaskHash(ctx context.Context, taskHash string) (*entities.TaskMetadata, error) {
	model, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}

	return s.modelToEntity(model), nil
}

// GetByTaskHashes retrieves metadata for multiple tasks (batch)
func (s *Service) GetByTaskHashes(ctx context.Context, taskHashes []string) (map[string]*entities.TaskMetadata, error) {
	modelsMap, err := s.repo.GetByTaskHashes(ctx, taskHashes)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*entities.TaskMetadata, len(modelsMap))
	for hash, model := range modelsMap {
		result[hash] = s.modelToEntity(model)
	}

	return result, nil
}

// UploadImage uploads an image for a task
func (s *Service) UploadImage(ctx context.Context, taskHash string, file multipart.File, header *multipart.FileHeader) (*entities.TaskMetadata, error) {
	// Validate taskHash to prevent path traversal
	if err := validateTaskHash(taskHash); err != nil {
		return nil, fmt.Errorf("invalid task_hash: %w", err)
	}

	// Validate file size
	if header.Size > MaxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", MaxFileSize)
	}

	// Validate MIME type
	contentType := header.Header.Get("Content-Type")
	if !s.isAllowedMimeType(contentType) {
		return nil, fmt.Errorf("invalid file type: %s. Allowed types: %s", contentType, AllowedMimeTypes)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s_%s%s", taskHash, time.Now().String(), ext)
	filePath, absFilePath, err := s.buildUploadPath(filename)
	if err != nil {
		return nil, err
	}

	// Create file
	dst, err := os.Create(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file contents
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(absFilePath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Check if metadata already exists
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		os.Remove(absFilePath)
		return nil, err
	}

	if existing != nil {
		// Delete old image if exists (validate path to prevent path traversal)
		if existing.ImagePath != "" {
			if err := s.validateImagePath(existing.ImagePath); err != nil {
				slog.Warn("skipping image deletion due to path validation failure", "error", err)
			} else {
				os.Remove(existing.ImagePath)
			}
		}

		// Update existing metadata
		existing.ImagePath = filePath
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(ctx, existing); err != nil {
			os.Remove(filePath)
			return nil, err
		}
		return s.modelToEntity(existing), nil
	}

	// Create new metadata
	metadata := &models.TaskMetadata{
		UUID:      uuid.New(),
		TaskHash:  taskHash,
		ImagePath: filePath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, metadata); err != nil {
		os.Remove(absFilePath)
		return nil, err
	}

	return s.modelToEntity(metadata), nil
}

// UpdateDescription updates the description of task metadata
func (s *Service) UpdateDescription(ctx context.Context, taskHash string, description string) (*entities.TaskMetadata, error) {
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing metadata
		existing.Description = description
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return s.modelToEntity(existing), nil
	}

	// Create new metadata with description only
	metadata := &models.TaskMetadata{
		UUID:        uuid.New(),
		TaskHash:    taskHash,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, metadata); err != nil {
		return nil, err
	}

	return s.modelToEntity(metadata), nil
}

// UpdateName updates the friendly name of task metadata
func (s *Service) UpdateName(ctx context.Context, taskHash string, name string) (*entities.TaskMetadata, error) {
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing metadata
		existing.Name = name
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return s.modelToEntity(existing), nil
	}

	// Create new metadata with name only
	metadata := &models.TaskMetadata{
		UUID:      uuid.New(),
		TaskHash:  taskHash,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, metadata); err != nil {
		return nil, err
	}

	return s.modelToEntity(metadata), nil
}

// UpdateImagePosition updates the vertical position of the image
func (s *Service) UpdateImagePosition(ctx context.Context, taskHash string, positionY float64) (*entities.TaskMetadata, error) {
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, fmt.Errorf("metadata not found")
	}

	// Update position
	existing.ImagePositionY = positionY
	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return s.modelToEntity(existing), nil
}

// UpdateImageBrightness updates the brightness of the image
func (s *Service) UpdateImageBrightness(ctx context.Context, taskHash string, brightness float64) (*entities.TaskMetadata, error) {
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, fmt.Errorf("metadata not found")
	}

	// Validate brightness range (15-85%)
	if brightness < 15 || brightness > 85 {
		return nil, fmt.Errorf("brightness must be between 15 and 85")
	}

	// Update brightness
	existing.ImageBrightness = brightness
	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return s.modelToEntity(existing), nil
}

// DeleteImage deletes the image from task metadata
func (s *Service) DeleteImage(ctx context.Context, taskHash string) error {
	metadata, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return err
	}
	if metadata == nil {
		return fmt.Errorf("metadata not found")
	}

	// Delete file if exists (validate path to prevent path traversal)
	if metadata.ImagePath != "" {
		if err := s.validateImagePath(metadata.ImagePath); err == nil {
			os.Remove(metadata.ImagePath)
		}
	}

	if metadata.Name == "" && metadata.Description == "" && metadata.ReleaseDate == "" {
		return s.repo.DeleteByTaskHash(ctx, taskHash)
	}

	metadata.ImagePath = ""
	metadata.UpdatedAt = time.Now()
	return s.repo.Update(ctx, metadata)
}

// Delete deletes task metadata completely
func (s *Service) Delete(ctx context.Context, taskHash string) error {
	metadata, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return err
	}
	if metadata == nil {
		return nil
	}

	// Delete file if exists (validate path to prevent path traversal)
	if metadata.ImagePath != "" {
		if err := s.validateImagePath(metadata.ImagePath); err == nil {
			os.Remove(metadata.ImagePath)
		}
	}

	return s.repo.DeleteByTaskHash(ctx, taskHash)
}

// GetImagePath returns the filesystem path for an image
func (s *Service) GetImagePath(ctx context.Context, taskHash string) (string, error) {
	metadata, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		return "", err
	}
	if metadata == nil || metadata.ImagePath == "" {
		return "", fmt.Errorf("image not found")
	}

	// Validate path is within upload directory to prevent path traversal
	if err := s.validateImagePath(metadata.ImagePath); err != nil {
		return "", fmt.Errorf("invalid image path: %w", err)
	}

	return metadata.ImagePath, nil
}

func (s *Service) buildUploadPath(filename string) (string, string, error) {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return "", "", fmt.Errorf("invalid generated filename")
	}

	filePath := filepath.Join(s.uploadDir, filename)
	if err := s.validateImagePath(filePath); err != nil {
		return "", "", fmt.Errorf("invalid image path: %w", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	return filePath, absFilePath, nil
}

// validateImagePath ensures the path is within the upload directory to prevent path traversal attacks
func (s *Service) validateImagePath(imagePath string) error {
	// Clean and resolve the absolute path
	absPath, err := filepath.Abs(imagePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Clean and resolve the absolute upload directory
	absUploadDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return fmt.Errorf("failed to resolve upload directory: %w", err)
	}

	// Ensure the path is within the upload directory
	relPath, err := filepath.Rel(absUploadDir, absPath)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Check for path traversal (if relPath starts with "..", it's outside the directory)
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path traversal detected")
	}

	return nil
}

type uploadFileInfo struct {
	path string
	size int64
}

// scanUploadDir reads all files from the upload directory and returns a map keyed by full path.
func (s *Service) scanUploadDir() (map[string]uploadFileInfo, error) {
	entries, err := os.ReadDir(s.uploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload directory: %w", err)
	}

	filesByPath := make(map[string]uploadFileInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(s.uploadDir, entry.Name())
		filesByPath[fullPath] = uploadFileInfo{path: fullPath, size: info.Size()}
	}
	return filesByPath, nil
}

func (s *Service) readAndValidateRemoteImage(resp *http.Response) ([]byte, string, error) {
	limitedBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxFileSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image response: %w", err)
	}
	if len(limitedBody) == 0 {
		return nil, "", fmt.Errorf("invalid image response: empty body")
	}
	if len(limitedBody) > MaxFileSize {
		return nil, "", fmt.Errorf("image size exceeds maximum allowed size of %d bytes", MaxFileSize)
	}

	detectedContentType := normalizeContentType(http.DetectContentType(limitedBody))
	responseContentType := normalizeContentType(resp.Header.Get("Content-Type"))
	if responseContentType != "" && !strings.HasPrefix(responseContentType, "image/") {
		return nil, "", fmt.Errorf("invalid image content type: %s", responseContentType)
	}
	if !strings.HasPrefix(detectedContentType, "image/") || !s.isAllowedMimeType(detectedContentType) {
		return nil, "", fmt.Errorf("invalid image content type: %s", detectedContentType)
	}
	if responseContentType != "" && responseContentType != detectedContentType {
		return nil, "", fmt.Errorf("image content type mismatch: %s", responseContentType)
	}

	return limitedBody, detectedContentType, nil
}

// queryHashToWorkerID maps task hashes to their worker IDs via the task_states table.
func (s *Service) queryHashToWorkerID(ctx context.Context, taskHashes []string) (map[string]string, error) {
	hashToWorkerID := make(map[string]string)
	if len(taskHashes) == 0 {
		return hashToWorkerID, nil
	}
	var taskStates []struct {
		Hash     string    `gorm:"column:hash"`
		WorkerID uuid.UUID `gorm:"column:worker_id"`
	}
	if err := s.db.DB.WithContext(ctx).
		Table("task_states").
		Select("hash, worker_id").
		Where("hash IN ?", taskHashes).
		Find(&taskStates).Error; err != nil {
		return nil, fmt.Errorf("failed to query task_states: %w", err)
	}
	for _, ts := range taskStates {
		hashToWorkerID[ts.Hash] = ts.WorkerID.String()
	}
	return hashToWorkerID, nil
}

// queryWorkerNames returns a map of worker UUID strings to their display names.
func (s *Service) queryWorkerNames(ctx context.Context) map[string]string {
	workerNames := make(map[string]string)
	var workers []struct {
		UUID uuid.UUID `gorm:"column:uuid"`
		Name string    `gorm:"column:name"`
	}
	if err := s.db.DB.WithContext(ctx).
		Table("workers").
		Select("uuid, name").
		Find(&workers).Error; err != nil {
		slog.Warn("failed to query workers", "error", err)
		return workerNames
	}
	for _, w := range workers {
		workerNames[w.UUID.String()] = w.Name
	}
	return workerNames
}

// GetImageStorageStatsByWorker returns per-worker image storage stats using a filesystem-first approach
func (s *Service) GetImageStorageStatsByWorker(ctx context.Context) (*models.ImageStorageStatsResponse, error) {
	// 1. Scan uploadDir for all files
	filesByPath, err := s.scanUploadDir()
	if err != nil {
		return nil, err
	}

	// 2. Get all TaskMetadata with images → build map[imagePath]*TaskMetadata
	metadataList, err := s.repo.GetAllWithImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata with images: %w", err)
	}
	metaByPath := make(map[string]*models.TaskMetadata, len(metadataList))
	for i := range metadataList {
		metaByPath[metadataList[i].ImagePath] = &metadataList[i]
	}

	// 3. Collect all task hashes that have files on disk
	var taskHashes []string
	for _, meta := range metadataList {
		if _, exists := filesByPath[meta.ImagePath]; exists {
			taskHashes = append(taskHashes, meta.TaskHash)
		}
	}

	// 4. Query TaskState to map task_hash → worker_id
	hashToWorkerID, err := s.queryHashToWorkerID(ctx, taskHashes)
	if err != nil {
		return nil, err
	}

	// 5. Get all known worker UUIDs and names
	workerNames := s.queryWorkerNames(ctx)

	// 6. Group files by worker
	type workerAccum struct {
		count int
		size  int64
	}
	workerStats := make(map[string]*workerAccum)
	var orphanCount int
	var orphanSize int64
	var totalSize int64
	var totalCount int

	for filePath, fi := range filesByPath {
		totalSize += fi.size
		totalCount++

		meta, hasMeta := metaByPath[filePath]
		if !hasMeta {
			// File on disk but no TaskMetadata → orphan
			orphanCount++
			orphanSize += fi.size
			continue
		}

		workerID, hasWorker := hashToWorkerID[meta.TaskHash]
		if !hasWorker {
			// TaskMetadata exists but no TaskState → orphan (task removed)
			orphanCount++
			orphanSize += fi.size
			continue
		}

		if _, ok := workerStats[workerID]; !ok {
			workerStats[workerID] = &workerAccum{}
		}
		workerStats[workerID].count++
		workerStats[workerID].size += fi.size
	}

	// 7. Build response
	var workerList []models.WorkerImageStats
	for workerID, stats := range workerStats {
		name, isActive := workerNames[workerID]
		workerList = append(workerList, models.WorkerImageStats{
			WorkerID:       workerID,
			WorkerName:     name,
			IsRemoved:      !isActive,
			ImageCount:     stats.count,
			TotalSizeBytes: stats.size,
		})
	}

	return &models.ImageStorageStatsResponse{
		Workers:         workerList,
		OrphanCount:     orphanCount,
		OrphanSizeBytes: orphanSize,
		TotalSizeBytes:  totalSize,
		TotalImageCount: totalCount,
	}, nil
}

// DeleteImagesByWorker deletes all images belonging to a specific worker
func (s *Service) DeleteImagesByWorker(ctx context.Context, workerID string) (int, error) {
	// 1. Get all task hashes belonging to this worker from TaskState
	var taskHashes []string
	if err := s.db.DB.WithContext(ctx).
		Table("task_states").
		Select("hash").
		Where("worker_id = ?", workerID).
		Pluck("hash", &taskHashes).Error; err != nil {
		return 0, fmt.Errorf("failed to query task_states: %w", err)
	}

	if len(taskHashes) == 0 {
		return 0, nil
	}

	// 2. Get TaskMetadata for those hashes that have images
	metadataMap, err := s.repo.GetByTaskHashes(ctx, taskHashes)
	if err != nil {
		return 0, fmt.Errorf("failed to get metadata: %w", err)
	}

	// 3. Delete files from disk
	var deletedCount int
	var hashesWithImages []string
	for hash, meta := range metadataMap {
		if meta.ImagePath == "" {
			continue
		}
		if err := s.validateImagePath(meta.ImagePath); err != nil {
			slog.Warn("skipping image deletion due to path validation failure", "error", err, "path", meta.ImagePath)
			continue
		}
		if err := os.Remove(meta.ImagePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to delete image file", "error", err, "path", meta.ImagePath)
			continue
		}
		hashesWithImages = append(hashesWithImages, hash)
		deletedCount++
	}

	// 4. Clear image paths in DB
	if len(hashesWithImages) > 0 {
		if _, err := s.repo.ClearImagePathsByTaskHashes(ctx, hashesWithImages); err != nil {
			slog.Error("failed to clear image paths in DB", "error", err)
			return deletedCount, fmt.Errorf("files deleted but failed to clear DB: %w", err)
		}
	}

	return deletedCount, nil
}

// queryHashesWithState returns the set of task hashes (from the provided list) that have a row in task_states.
func (s *Service) queryHashesWithState(ctx context.Context, taskHashes []string) (map[string]bool, error) {
	hashHasState := make(map[string]bool)
	if len(taskHashes) == 0 {
		return hashHasState, nil
	}
	var taskStates []struct {
		Hash string `gorm:"column:hash"`
	}
	if err := s.db.DB.WithContext(ctx).
		Table("task_states").
		Select("hash").
		Where("hash IN ?", taskHashes).
		Find(&taskStates).Error; err != nil {
		return nil, fmt.Errorf("failed to query task_states: %w", err)
	}
	for _, ts := range taskStates {
		hashHasState[ts.Hash] = true
	}
	return hashHasState, nil
}

// removeValidatedFile validates the path is within the upload dir, then removes it.
// Returns true if the file was successfully removed.
func (s *Service) removeValidatedFile(filePath string) bool {
	if err := s.validateImagePath(filePath); err != nil {
		slog.Warn("skipping orphan deletion due to path validation failure", "error", err)
		return false
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to delete orphan file", "error", err, "path", filePath)
		return false
	}
	return true
}

// clearImagePaths clears image_path in DB for the given task hashes, if any.
func (s *Service) clearImagePaths(ctx context.Context, hashes []string) {
	if len(hashes) == 0 {
		return
	}
	if _, err := s.repo.ClearImagePathsByTaskHashes(ctx, hashes); err != nil {
		slog.Error("failed to clear image paths for orphans in DB", "error", err)
	}
}

// DeleteOrphanImages deletes orphan files from uploadDir.
// An orphan is either: (a) a file not referenced by any TaskMetadata, or
// (b) a file referenced by TaskMetadata but whose task_hash has no TaskState.
// This method mirrors the exact same filesystem-first classification used by GetImageStorageStatsByWorker.
func (s *Service) DeleteOrphanImages(ctx context.Context) (int, error) {
	// 1. Scan uploadDir for all files (filesystem-first, matching stats logic)
	filesByPath, err := s.scanUploadDir()
	if err != nil {
		return 0, err
	}

	// 2. Get all TaskMetadata with images → build map[imagePath]*TaskMetadata
	metadataList, err := s.repo.GetAllWithImages(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get metadata with images: %w", err)
	}
	metaByPath := make(map[string]*models.TaskMetadata, len(metadataList))
	for i := range metadataList {
		metaByPath[metadataList[i].ImagePath] = &metadataList[i]
	}

	// 3. Collect only task hashes that have files on disk (matching stats logic)
	var taskHashes []string
	for _, meta := range metadataList {
		if _, exists := filesByPath[meta.ImagePath]; exists {
			taskHashes = append(taskHashes, meta.TaskHash)
		}
	}

	// 4. Query TaskState to find which hashes still have an worker link
	hashHasState, err := s.queryHashesWithState(ctx, taskHashes)
	if err != nil {
		return 0, err
	}

	// 5. Walk files and delete orphans (same classification as stats)
	var deletedCount int
	var hashesToClear []string
	for filePath := range filesByPath {
		meta, hasMeta := metaByPath[filePath]

		// Case A: file on disk, no TaskMetadata → pure FS orphan
		// Case B: file on disk, has TaskMetadata, but no TaskState → stale orphan
		isOrphan := !hasMeta || !hashHasState[meta.TaskHash]
		if !isOrphan {
			continue
		}

		if !s.removeValidatedFile(filePath) {
			continue
		}
		deletedCount++

		// If it had a TaskMetadata record, clear image_path in DB
		if hasMeta {
			hashesToClear = append(hashesToClear, meta.TaskHash)
		}
	}

	// 6. Clear image paths in DB for stale orphans
	s.clearImagePaths(ctx, hashesToClear)

	return deletedCount, nil
}

// Helper functions

func (s *Service) isAllowedMimeType(contentType string) bool {
	contentType = normalizeContentType(contentType)
	allowedTypes := strings.Split(AllowedMimeTypes, ",")
	for _, allowed := range allowedTypes {
		if strings.TrimSpace(allowed) == contentType {
			return true
		}
	}
	return false
}

func normalizeContentType(contentType string) string {
	if contentType == "" {
		return ""
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.ToLower(mediaType)
	}

	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func (s *Service) selectImageExtension(rawExt string, contentType string) string {
	validExts := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
	}
	detectedExts := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}

	normalizedExt := strings.ToLower(rawExt)
	if mimeType, ok := validExts[normalizedExt]; ok && mimeType == contentType {
		return normalizedExt
	}

	if detectedExt, ok := detectedExts[contentType]; ok {
		return detectedExt
	}

	return ".jpg"
}

func (s *Service) modelToEntity(model *models.TaskMetadata) *entities.TaskMetadata {
	entity := &entities.TaskMetadata{
		UUID:            model.UUID,
		TaskHash:        model.TaskHash,
		ImagePath:       model.ImagePath,
		Name:            model.Name,
		ReleaseDate:     model.ReleaseDate,
		Description:     model.Description,
		ImagePositionY:  model.ImagePositionY,
		ImageBrightness: model.ImageBrightness,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}

	// Generate URLs if image exists
	if model.ImagePath != "" {
		filename := filepath.Base(model.ImagePath)
		entity.ImageURL = fmt.Sprintf("%s/media/%s", s.baseURL, filename)
		entity.ThumbnailURL = fmt.Sprintf("%s/media/%s", s.baseURL, filename)
	}

	return entity
}

func (s *Service) GetProviderStatus(ctx context.Context, providerName string) (*MetadataProviderStatus, error) {
	provider, ok := s.providerRegistry.Get(providerName)
	if !ok {
		return nil, ErrProviderNotFound
	}

	return provider.Status(ctx)
}

func (s *Service) SearchProvider(ctx context.Context, providerName string, query string) ([]MetadataProviderSearchResult, error) {
	provider, ok := s.providerRegistry.Get(providerName)
	if !ok {
		return nil, ErrProviderNotFound
	}

	return provider.Search(ctx, query)
}

// GetProviderImage retrieves a provider-owned image through Gardarr so browser
// clients do not have to embed third-party CDN resources. Provider host and
// response validation are shared with the image persistence flow below.
func (s *Service) GetProviderImage(ctx context.Context, providerName string, imageID string) ([]byte, string, error) {
	provider, ok := s.providerRegistry.Get(providerName)
	if !ok {
		return nil, "", ErrProviderNotFound
	}

	imageID = strings.TrimSpace(imageID)
	if imageID == "" || len(imageID) > 1024 {
		return nil, "", ErrProviderImageInvalid
	}

	imageURL, err := provider.BuildImageURL(imageID)
	if err != nil || strings.TrimSpace(imageURL) == "" {
		return nil, "", fmt.Errorf("%w: invalid image id", ErrProviderImageInvalid)
	}

	body, contentType, _, err := s.fetchProviderImage(ctx, provider, imageURL)
	if err != nil {
		return nil, "", err
	}

	return body, contentType, nil
}

// SearchProviderAuto turns a release name into the most useful provider query
// before delegating to the existing provider abstraction.
func (s *Service) SearchProviderAuto(ctx context.Context, providerName string, rawName string) ([]MetadataProviderSearchResult, error) {
	return s.SearchProvider(ctx, providerName, releaseparse.Parse(rawName).SearchQuery())
}

// ParseRelease previews deterministic release-name suggestions from a raw
// name (magnet display name or arbitrary text). It does not contact any
// external metadata provider.
func (s *Service) ParseRelease(rawName string) releaseparse.Result {
	return releaseparse.Parse(rawName)
}

// ParseReleaseFile previews release-name suggestions from an uploaded
// .torrent file: it parses the torrent's own name and refines the guess
// using its payload file extensions when the name alone carries no format
// hint (e.g. a generically named batch of ebooks or comics).
func (s *Service) ParseReleaseFile(data []byte) (releaseparse.Result, error) {
	name, extensions, err := torrentfile.InspectInfo(data)
	if err != nil {
		return releaseparse.Result{}, err
	}
	result := releaseparse.Parse(name)
	if len(extensions) > 0 {
		result = result.RefineWithFileExtensions(extensions)
	}
	return result, nil
}

// ApplyProviderSelection resolves provider-owned metadata and applies it to a task.
func (s *Service) ApplyProviderSelection(ctx context.Context, providerName string, taskHash string, selectionID string) (*ApplyProviderSelectionResult, error) {
	return s.ApplyProviderSelectionWithFallback(ctx, providerName, taskHash, selectionID, nil)
}

func (s *Service) ApplyProviderSelectionWithFallback(
	ctx context.Context,
	providerName string,
	taskHash string,
	selectionID string,
	fallback *MetadataProviderSelection,
) (*ApplyProviderSelectionResult, error) {
	slog.Info("applying provider metadata", "provider", providerName, "task_hash", taskHash, "selection_id", selectionID, "has_fallback", fallback != nil)
	provider, ok := s.providerRegistry.Get(providerName)
	if !ok {
		return nil, ErrProviderNotFound
	}

	if err := validateTaskHash(taskHash); err != nil {
		return nil, fmt.Errorf("invalid task_hash: %w", err)
	}

	resolved, err := s.resolveProviderSelection(ctx, provider, taskHash, selectionID, fallback)
	if err != nil {
		return nil, err
	}

	filePath, imageWarning, imageWarningReason, err := s.processProviderImage(ctx, provider, taskHash, selectionID, resolved.selection)
	if err != nil {
		return nil, err
	}

	if imageWarning != "" && resolved.warning == "" {
		resolved.warning = imageWarning
		resolved.warningReason = imageWarningReason
	}

	metadata, err := s.saveProviderSelection(ctx, taskHash, resolved.selection, filePath)
	if err != nil {
		return nil, err
	}

	s.logProviderMetadataApplied(provider, taskHash, selectionID, filePath != "", resolved.warning, resolved.warningReason)
	return &ApplyProviderSelectionResult{
		Metadata:      s.modelToEntity(metadata),
		Warning:       resolved.warning,
		WarningReason: resolved.warningReason,
	}, nil
}

func (s *Service) resolveProviderSelection(
	ctx context.Context,
	provider MetadataProvider,
	taskHash string,
	selectionID string,
	fallback *MetadataProviderSelection,
) (*resolvedProviderSelection, error) {
	selection, err := provider.Resolve(ctx, selectionID)
	if err != nil {
		slog.Warn("provider resolve failed", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID, "error", err, "has_fallback", fallback != nil)
		if fallback == nil {
			return nil, err
		}

		slog.Info("using provider fallback selection after resolve failure", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID)
		return &resolvedProviderSelection{
			selection:     cloneFallbackSelection(fallback),
			warning:       "provider_selection_fallback_used",
			warningReason: err.Error(),
		}, nil
	}

	slog.Info("provider selection resolved", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID)
	if selection != nil {
		return &resolvedProviderSelection{selection: selection}, nil
	}

	if fallback == nil {
		return nil, ErrProviderSelectionNotFound
	}

	slog.Info("using provider fallback selection after empty resolve", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID)
	return &resolvedProviderSelection{
		selection:     cloneFallbackSelection(fallback),
		warning:       "provider_selection_fallback_used",
		warningReason: ErrProviderSelectionNotFound.Error(),
	}, nil
}

func cloneFallbackSelection(selection *MetadataProviderSelection) *MetadataProviderSelection {
	if selection == nil {
		return nil
	}

	return &MetadataProviderSelection{
		ID:          selection.ID,
		Title:       selection.Title,
		ReleaseDate: selection.ReleaseDate,
		Description: selection.Description,
		ImageID:     selection.ImageID,
	}
}

func (s *Service) processProviderImage(
	ctx context.Context,
	provider MetadataProvider,
	taskHash string,
	selectionID string,
	selection *MetadataProviderSelection,
) (string, string, string, error) {
	imageURL, err := s.providerImageURL(provider, selection)
	if err != nil {
		return "", "provider_image_skipped", err.Error(), nil
	}
	if imageURL == "" {
		return "", "", "", nil
	}

	slog.Info("processing provider image", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID, "image_url", imageURL)
	body, contentType, parsedURL, err := s.fetchProviderImage(ctx, provider, imageURL)
	if err != nil {
		return "", "provider_image_skipped", s.logSkippedProviderImage("download failure", provider, taskHash, err), nil
	}

	filePath, err := s.saveProviderImage(taskHash, provider.Name(), parsedURL.Path, contentType, body)
	if err != nil {
		return "", "", "", err
	}

	return filePath, "", "", nil
}

func (s *Service) fetchProviderImage(ctx context.Context, provider MetadataProvider, imageURL string) ([]byte, string, *url.URL, error) {
	_, parsedURL, err := s.validateExternalImageURL(ctx, provider, imageURL)
	if err != nil {
		return nil, "", nil, err
	}

	req, err := newProviderImageRequest(ctx, provider, parsedURL)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create image request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", nil, fmt.Errorf("failed to download image, status code: %d", resp.StatusCode)
	}

	body, contentType, err := s.readAndValidateRemoteImage(resp)
	if err != nil {
		return nil, "", nil, err
	}

	return body, contentType, parsedURL, nil
}

// newProviderImageRequest creates the network request from a provider-owned
// origin instead of the external image URL. Only the already-validated path is
// copied onto the request, so request input can never select a network host.
func newProviderImageRequest(ctx context.Context, provider MetadataProvider, validatedURL *url.URL) (*http.Request, error) {
	if validatedURL == nil {
		return nil, fmt.Errorf("validated image URL is required")
	}

	var trustedOrigin string
	for _, allowedHost := range provider.AllowedImageHosts() {
		if strings.EqualFold(validatedURL.Hostname(), allowedHost) {
			trustedOrigin = (&url.URL{
				Scheme: "https",
				Host:   strings.ToLower(allowedHost),
			}).String()
			break
		}
	}
	if trustedOrigin == "" {
		return nil, fmt.Errorf("validated image URL has no trusted provider origin")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trustedOrigin, nil)
	if err != nil {
		return nil, err
	}
	req.URL.Path = validatedURL.Path
	req.URL.RawPath = validatedURL.RawPath
	return req, nil
}

func (s *Service) providerImageURL(provider MetadataProvider, selection *MetadataProviderSelection) (string, error) {
	if selection == nil {
		return "", nil
	}
	if strings.TrimSpace(selection.ImageID) == "" {
		if strings.TrimSpace(selection.ImageURL) != "" {
			return selection.ImageURL, nil
		}
		return "", nil
	}

	imageURL, err := provider.BuildImageURL(selection.ImageID)
	if err != nil {
		slog.Warn("skipping provider image due to invalid image id", "provider", provider.Name(), "image_id", selection.ImageID, "error", err)
		return "", fmt.Errorf("invalid provider image id: %w", err)
	}

	return imageURL, nil
}

func (s *Service) logSkippedProviderImage(reason string, provider MetadataProvider, taskHash string, err error) string {
	slog.Warn("skipping provider image due to "+reason, "provider", provider.Name(), "task_hash", taskHash, "error", err)
	return err.Error()
}

func (s *Service) saveProviderImage(taskHash string, providerName string, imagePath string, contentType string, body []byte) (string, error) {
	lastPart := filepath.Base(imagePath)
	ext := s.selectImageExtension(filepath.Ext(lastPart), contentType)
	filename := fmt.Sprintf("%s_%s_%s%s", taskHash, providerName, time.Now().Format("20060102150405"), ext)
	filePath, absFilePath, err := s.buildUploadPath(filename)
	if err != nil {
		return "", err
	}

	dst, err := os.Create(absFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create image file: %w", err)
	}

	if _, err := io.Copy(dst, bytes.NewReader(body)); err != nil {
		dst.Close()
		os.Remove(absFilePath)
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(absFilePath)
		return "", fmt.Errorf("failed to finalize image file: %w", err)
	}

	return filePath, nil
}

func (s *Service) saveProviderSelection(
	ctx context.Context,
	taskHash string,
	selection *MetadataProviderSelection,
	filePath string,
) (*models.TaskMetadata, error) {
	existing, err := s.repo.GetByTaskHash(ctx, taskHash)
	if err != nil {
		s.removeImageFile(filePath)
		return nil, err
	}

	if existing != nil {
		return s.updateExistingProviderMetadata(ctx, existing, selection, filePath)
	}

	return s.createProviderMetadata(ctx, taskHash, selection, filePath)
}

func (s *Service) updateExistingProviderMetadata(
	ctx context.Context,
	existing *models.TaskMetadata,
	selection *MetadataProviderSelection,
	filePath string,
) (*models.TaskMetadata, error) {
	existing.Name = selection.Title
	existing.ReleaseDate = selection.ReleaseDate
	if selection.Description != "" {
		existing.Description = selection.Description
	}
	existing.UpdatedAt = time.Now()

	if filePath == "" {
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	oldImagePath := existing.ImagePath
	existing.ImagePath = filePath
	if err := s.repo.Update(ctx, existing); err != nil {
		s.removeImageFile(filePath)
		return nil, err
	}

	s.removeValidatedImageFile(oldImagePath)
	return existing, nil
}

func (s *Service) createProviderMetadata(
	ctx context.Context,
	taskHash string,
	selection *MetadataProviderSelection,
	filePath string,
) (*models.TaskMetadata, error) {
	now := time.Now()
	metadata := &models.TaskMetadata{
		UUID:        uuid.New(),
		TaskHash:    taskHash,
		Name:        selection.Title,
		ReleaseDate: selection.ReleaseDate,
		Description: selection.Description,
		ImagePath:   filePath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, metadata); err != nil {
		s.removeImageFile(filePath)
		return nil, err
	}

	return metadata, nil
}

func (s *Service) removeImageFile(filePath string) {
	if filePath != "" {
		os.Remove(filePath)
	}
}

func (s *Service) removeValidatedImageFile(filePath string) {
	if filePath == "" {
		return
	}

	if err := s.validateImagePath(filePath); err == nil {
		os.Remove(filePath)
	}
}

func (s *Service) logProviderMetadataApplied(provider MetadataProvider, taskHash string, selectionID string, hasImage bool, warning string, warningReason string) {
	slog.Info("provider metadata applied", "provider", provider.Name(), "task_hash", taskHash, "selection_id", selectionID, "has_image", hasImage, "warning", warning, "warning_reason", warningReason)
}

func (s *Service) validateExternalImageURL(ctx context.Context, provider MetadataProvider, rawURL string) (string, *url.URL, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("invalid image URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return "", nil, fmt.Errorf("invalid image URL: only https URLs are allowed")
	}

	host := parsedURL.Hostname()
	if host == "" {
		return "", nil, fmt.Errorf("invalid image URL: missing hostname")
	}

	if parsedURL.Port() != "" {
		return "", nil, fmt.Errorf("invalid image URL: explicit ports are not allowed")
	}

	if parsedURL.User != nil {
		return "", nil, fmt.Errorf("invalid image URL: userinfo is not allowed")
	}

	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return "", nil, fmt.Errorf("invalid image URL: query strings and fragments are not allowed")
	}

	allowedHosts := make(map[string]struct{}, len(provider.AllowedImageHosts()))
	for _, allowedHost := range provider.AllowedImageHosts() {
		allowedHosts[strings.ToLower(allowedHost)] = struct{}{}
	}

	if _, ok := allowedHosts[strings.ToLower(host)]; !ok {
		return "", nil, fmt.Errorf("invalid image URL: untrusted host")
	}

	cleanPath := path.Clean(parsedURL.EscapedPath())
	if cleanPath == "." {
		cleanPath = "/"
	}
	if !strings.HasPrefix(cleanPath, "/") || !validExternalImagePath.MatchString(cleanPath) {
		return "", nil, fmt.Errorf("invalid image URL: unsupported path")
	}

	ips, err := s.lookupIPAddr(ctx, host)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve image host: %w", err)
	}

	if len(ips) == 0 {
		return "", nil, fmt.Errorf("failed to resolve image host: no IP addresses found")
	}

	for _, ipAddr := range ips {
		if isDisallowedRemoteIP(ipAddr.IP) {
			return "", nil, fmt.Errorf("invalid image URL: resolved to a disallowed IP address")
		}
	}

	decodedPath, err := url.PathUnescape(cleanPath)
	if err != nil {
		return "", nil, fmt.Errorf("invalid image URL: failed to decode path: %w", err)
	}

	sanitizedURL := &url.URL{
		Scheme:  "https",
		Host:    strings.ToLower(host),
		Path:    decodedPath,
		RawPath: cleanPath,
	}

	return sanitizedURL.String(), sanitizedURL, nil
}

func isDisallowedRemoteIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}

	return false
}
