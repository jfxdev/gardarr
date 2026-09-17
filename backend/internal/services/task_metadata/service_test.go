package task_metadata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/gorm"
)

const (
	unexpectedErrFmt = "unexpected error: %v"
	tgdbImageURL     = "https://cdn.thegamesdb.net/images/large/boxart/front/123.jpg"
)

// setupTestService creates a Service with an in-memory DB and a temp uploadDir.
// It auto-migrates TaskMetadata, TaskState, and Worker tables.
func setupTestService(t *testing.T) (*Service, string) {
	t.Helper()
	db := database.SetupTestDB(t, &models.TaskMetadata{}, &models.TaskState{}, &models.Worker{})

	uploadDir := t.TempDir()

	svc, err := NewService(db, "http://localhost:3200", uploadDir, nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	return svc, uploadDir
}

func createTestMetadata(t *testing.T, svc *Service, metadata *models.TaskMetadata) {
	t.Helper()

	if metadata.UUID == uuid.Nil {
		metadata.UUID = uuid.New()
	}
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = time.Now()
	}
	if metadata.UpdatedAt.IsZero() {
		metadata.UpdatedAt = time.Now()
	}

	if err := svc.repo.Create(context.Background(), metadata); err != nil {
		t.Fatalf("create task_metadata failed: %v", err)
	}
}

// createTestFile creates a file with the given content in dir and returns its full path.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file %s: %v", p, err)
	}
	return p
}

// seedWorker inserts a worker into the DB and returns its UUID string.
func seedWorker(t *testing.T, svc *Service, name string) string {
	t.Helper()
	workerUUID := uuid.New()
	worker := &models.Worker{
		UUID:                         workerUUID,
		Name:                         name,
		Type:                         "qbt",
		Address:                      "http://test",
		EncryptedQBittorrentURL:      "tok",
		EncryptedQBittorrentUsername: "",
		EncryptedQBittorrentPassword: "",
		Icon:                         "",
		Color:                        "",
		CreatedAt:                    time.Now(),
		UpdatedAt:                    time.Now(),
	}
	if err := svc.db.DB.Create(worker).Error; err != nil {
		t.Fatalf("seed worker failed: %v", err)
	}
	return workerUUID.String()
}

// seedTaskState inserts a task_state row linking a hash to a worker.
func seedTaskState(t *testing.T, svc *Service, workerID, hash string) {
	t.Helper()
	if err := svc.db.DB.Exec(
		"INSERT INTO task_states (worker_id, hash, state, progress, updated_at) VALUES (?, ?, 'seeding', 100, ?)",
		workerID, hash, time.Now(),
	).Error; err != nil {
		t.Fatalf("seed task_state failed: %v", err)
	}
}

// seedTaskMetadata inserts a task_metadata row.
func seedTaskMetadata(t *testing.T, svc *Service, taskHash, imagePath string) {
	t.Helper()
	ctx := context.Background()
	meta := &models.TaskMetadata{
		UUID:      uuid.New(),
		TaskHash:  taskHash,
		ImagePath: imagePath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := svc.repo.Create(ctx, meta); err != nil {
		t.Fatalf("seed task_metadata failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests for GetImageStorageStatsByWorker
// ---------------------------------------------------------------------------

func TestGetImageStorageStatsByWorkerEmpty(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	stats, err := svc.GetImageStorageStatsByWorker(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if stats.TotalImageCount != 0 {
		t.Errorf("expected 0 total images, got %d", stats.TotalImageCount)
	}
	if stats.TotalSizeBytes != 0 {
		t.Errorf("expected 0 total size, got %d", stats.TotalSizeBytes)
	}
	if stats.OrphanCount != 0 {
		t.Errorf("expected 0 orphans, got %d", stats.OrphanCount)
	}
}

func TestGetImageStorageStatsByWorkerPerWorker(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// Setup: 2 workers, each with 1 image file
	worker1 := seedWorker(t, svc, "Worker-1")
	worker2 := seedWorker(t, svc, "Worker-2")

	file1 := createTestFile(t, uploadDir, "hash1_img.jpg", "aaaa")   // 4 bytes
	file2 := createTestFile(t, uploadDir, "hash2_img.jpg", "bbbbbb") // 6 bytes

	seedTaskMetadata(t, svc, "hash1", file1)
	seedTaskMetadata(t, svc, "hash2", file2)
	seedTaskState(t, svc, worker1, "hash1")
	seedTaskState(t, svc, worker2, "hash2")

	stats, err := svc.GetImageStorageStatsByWorker(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}

	if stats.TotalImageCount != 2 {
		t.Errorf("expected 2 total images, got %d", stats.TotalImageCount)
	}
	if stats.TotalSizeBytes != 10 {
		t.Errorf("expected 10 total bytes, got %d", stats.TotalSizeBytes)
	}
	if stats.OrphanCount != 0 {
		t.Errorf("expected 0 orphans, got %d", stats.OrphanCount)
	}
	if len(stats.Workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(stats.Workers))
	}

	// Check workers are active (not removed)
	for _, w := range stats.Workers {
		if w.IsRemoved {
			t.Errorf("worker %s should not be marked as removed", w.WorkerID)
		}
	}
}

func TestGetImageStorageStatsByWorkerOrphanFileNoDB(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// File on disk but NO TaskMetadata entry → orphan
	createTestFile(t, uploadDir, "orphan_file.jpg", "orphandata")

	stats, err := svc.GetImageStorageStatsByWorker(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if stats.OrphanCount != 1 {
		t.Errorf("expected 1 orphan, got %d", stats.OrphanCount)
	}
	if stats.TotalImageCount != 1 {
		t.Errorf("expected 1 total image, got %d", stats.TotalImageCount)
	}
}

func TestGetImageStorageStatsByWorkerOrphanNoTaskState(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// File exists, TaskMetadata exists, but NO TaskState → orphan (task removed from worker)
	file := createTestFile(t, uploadDir, "hash_no_state.jpg", "data")
	seedTaskMetadata(t, svc, "hash_no_state", file)

	stats, err := svc.GetImageStorageStatsByWorker(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if stats.OrphanCount != 1 {
		t.Errorf("expected 1 orphan (no TaskState), got %d", stats.OrphanCount)
	}
}

func TestGetImageStorageStatsByWorkerRemovedWorker(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// Worker exists in TaskState but NOT in Worker table → is_removed = true
	removedWorkerID := uuid.New().String()
	file := createTestFile(t, uploadDir, "hash_removed.jpg", "content")
	seedTaskMetadata(t, svc, "hash_removed", file)
	seedTaskState(t, svc, removedWorkerID, "hash_removed")
	// Do NOT seed worker in workers table

	stats, err := svc.GetImageStorageStatsByWorker(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if len(stats.Workers) != 1 {
		t.Fatalf("expected 1 worker entry, got %d", len(stats.Workers))
	}
	if !stats.Workers[0].IsRemoved {
		t.Error("expected worker to be marked as removed")
	}
	if stats.Workers[0].ImageCount != 1 {
		t.Errorf("expected 1 image for removed worker, got %d", stats.Workers[0].ImageCount)
	}
}

// ---------------------------------------------------------------------------
// Tests for DeleteImagesByWorker
// ---------------------------------------------------------------------------

func TestDeleteImagesByWorkerDeletesFilesAndClearsDB(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	workerID := seedWorker(t, svc, "Worker-Del")
	file1 := createTestFile(t, uploadDir, "del1.jpg", "file1data")
	file2 := createTestFile(t, uploadDir, "del2.jpg", "file2data")
	seedTaskMetadata(t, svc, "del1", file1)
	seedTaskMetadata(t, svc, "del2", file2)
	seedTaskState(t, svc, workerID, "del1")
	seedTaskState(t, svc, workerID, "del2")

	deleted, err := svc.DeleteImagesByWorker(ctx, workerID)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify files are gone
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("file1 should have been deleted")
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("file2 should have been deleted")
	}

	// Verify DB image_path is cleared
	meta1, _ := svc.repo.GetByTaskHash(ctx, "del1")
	if meta1 != nil && meta1.ImagePath != "" {
		t.Errorf("expected image_path to be cleared for del1, got %q", meta1.ImagePath)
	}
	meta2, _ := svc.repo.GetByTaskHash(ctx, "del2")
	if meta2 != nil && meta2.ImagePath != "" {
		t.Errorf("expected image_path to be cleared for del2, got %q", meta2.ImagePath)
	}
}

func TestDeleteImagesByWorkerDoesNotAffectOtherWorkers(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	worker1 := seedWorker(t, svc, "Worker-Keep")
	worker2 := seedWorker(t, svc, "Worker-Remove")

	fileKeep := createTestFile(t, uploadDir, "keep.jpg", "keep")
	fileRemove := createTestFile(t, uploadDir, "remove.jpg", "remove")
	seedTaskMetadata(t, svc, "keep", fileKeep)
	seedTaskMetadata(t, svc, "remove", fileRemove)
	seedTaskState(t, svc, worker1, "keep")
	seedTaskState(t, svc, worker2, "remove")

	deleted, err := svc.DeleteImagesByWorker(ctx, worker2)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Worker1's file should still exist
	if _, err := os.Stat(fileKeep); os.IsNotExist(err) {
		t.Error("worker1 file should NOT have been deleted")
	}

	// Worker1's DB entry should still have image_path
	meta, _ := svc.repo.GetByTaskHash(ctx, "keep")
	if meta == nil || meta.ImagePath == "" {
		t.Error("worker1 metadata image_path should still be set")
	}
}

func TestDeleteImagesByWorkerNoTasks(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	deleted, err := svc.DeleteImagesByWorker(ctx, uuid.New().String())
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted for non-existent worker, got %d", deleted)
	}
}

func TestNewServiceUsesDedicatedHTTPClientTimeout(t *testing.T) {
	svc, _ := setupTestService(t)

	if svc.httpClient == nil {
		t.Fatal("expected http client to be initialized")
	}
	if svc.httpClient == http.DefaultClient {
		t.Fatal("expected a dedicated http client instance")
	}
	if svc.httpClient.Timeout != httpTimeout {
		t.Fatalf("expected timeout %v, got %v", httpTimeout, svc.httpClient.Timeout)
	}
	if svc.httpClient.CheckRedirect == nil {
		t.Fatal("expected provider image redirects to be disabled")
	}
}

func TestDeleteImagePreservesMetadataWhenOtherFieldsRemain(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	imagePath := createTestFile(t, uploadDir, "keep-metadata.jpg", "image")
	createTestMetadata(t, svc, &models.TaskMetadata{
		TaskHash:    "keep-meta",
		ImagePath:   imagePath,
		Name:        "Existing Name",
		ReleaseDate: "2024-02-03",
	})

	if err := svc.DeleteImage(ctx, "keep-meta"); err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}

	metadata, err := svc.repo.GetByTaskHash(ctx, "keep-meta")
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if metadata == nil {
		t.Fatal("expected metadata row to be preserved")
	}
	if metadata.ImagePath != "" {
		t.Fatalf("expected image path to be cleared, got %q", metadata.ImagePath)
	}
	if metadata.Name != "Existing Name" {
		t.Fatalf("expected name to be preserved, got %q", metadata.Name)
	}
	if metadata.ReleaseDate != "2024-02-03" {
		t.Fatalf("expected release date to be preserved, got %q", metadata.ReleaseDate)
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatal("expected deleted image file to be removed")
	}
}

func TestDeleteImageDeletesMetadataWhenNoFieldsRemain(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	imagePath := createTestFile(t, uploadDir, "delete-empty.jpg", "image")
	createTestMetadata(t, svc, &models.TaskMetadata{
		TaskHash:  "delete-empty",
		ImagePath: imagePath,
	})

	if err := svc.DeleteImage(ctx, "delete-empty"); err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}

	metadata, err := svc.repo.GetByTaskHash(ctx, "delete-empty")
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if metadata != nil {
		t.Fatal("expected metadata row to be deleted")
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatal("expected deleted image file to be removed")
	}
}

// ---------------------------------------------------------------------------
// Tests for DeleteOrphanImages
// ---------------------------------------------------------------------------

func TestDeleteOrphanImagesRemovesStaleOrphans(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// File has TaskMetadata but NO TaskState → stale orphan (should be deleted)
	staleFile := createTestFile(t, uploadDir, "stale.jpg", "staledata")
	seedTaskMetadata(t, svc, "stale_hash", staleFile)

	// File properly linked to a worker → should NOT be deleted
	worker := seedWorker(t, svc, "Active-Worker")
	linkedFile := createTestFile(t, uploadDir, "linked.jpg", "linkeddata")
	seedTaskMetadata(t, svc, "linked_hash", linkedFile)
	seedTaskState(t, svc, worker, "linked_hash")

	deleted, err := svc.DeleteOrphanImages(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 stale orphan deleted, got %d", deleted)
	}

	// Stale file should be gone
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Error("stale orphan file should have been deleted")
	}

	// Linked file should still exist
	if _, err := os.Stat(linkedFile); os.IsNotExist(err) {
		t.Error("linked file should NOT have been deleted")
	}

	// Stale metadata image_path should be cleared in DB
	meta, _ := svc.repo.GetByTaskHash(ctx, "stale_hash")
	if meta != nil && meta.ImagePath != "" {
		t.Errorf("stale orphan image_path should be cleared, got %q", meta.ImagePath)
	}
}

func TestDeleteOrphanImagesRemovesUnreferencedFiles(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// Create an orphan file (no TaskMetadata)
	orphan := createTestFile(t, uploadDir, "orphan.jpg", "orphandata")

	// Create a referenced file (has TaskMetadata + TaskState)
	worker := seedWorker(t, svc, "Worker-Ref")
	referenced := createTestFile(t, uploadDir, "ref.jpg", "refdata")
	seedTaskMetadata(t, svc, "ref_hash", referenced)
	seedTaskState(t, svc, worker, "ref_hash")

	deleted, err := svc.DeleteOrphanImages(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 orphan deleted, got %d", deleted)
	}

	// Orphan should be gone
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan file should have been deleted")
	}

	// Referenced file should still exist
	if _, err := os.Stat(referenced); os.IsNotExist(err) {
		t.Error("referenced file should NOT have been deleted")
	}
}

func TestDeleteOrphanImagesNoOrphans(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	ctx := context.Background()

	// All files are fully referenced (TaskMetadata + TaskState)
	worker := seedWorker(t, svc, "Worker-Good")
	file := createTestFile(t, uploadDir, "good.jpg", "gooddata")
	seedTaskMetadata(t, svc, "good_hash", file)
	seedTaskState(t, svc, worker, "good_hash")

	deleted, err := svc.DeleteOrphanImages(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 orphans deleted, got %d", deleted)
	}
}

func TestDeleteOrphanImagesEmptyDir(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	deleted, err := svc.DeleteOrphanImages(ctx)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted for empty dir, got %d", deleted)
	}
}

func TestValidateExternalImageURLRejections(t *testing.T) {
	svc, _ := setupTestService(t)
	provider := mockMetadataProvider{name: "tgdb", allowedImageHosts: []string{"cdn.thegamesdb.net"}}

	tests := []struct {
		name        string
		url         string
		errContains string
		setup       func(*Service)
	}{
		{
			name:        "malformed URL",
			url:         "://bad-url",
			errContains: "", // any error
		},
		{
			name:        "non-HTTPS",
			url:         "http://cdn.thegamesdb.net/images/test.jpg",
			errContains: "only https URLs are allowed",
		},
		{
			name:        "untrusted host",
			url:         "https://example.com/image.jpg",
			errContains: "untrusted host",
		},
		{
			name:        "private IP",
			url:         "https://cdn.thegamesdb.net/images/test.jpg",
			errContains: "disallowed IP address",
			setup: func(s *Service) {
				s.lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
					return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
				}
			},
		},
		{
			name:        "query string",
			url:         "https://cdn.thegamesdb.net/images/test.jpg?token=123",
			errContains: "query strings and fragments are not allowed",
		},
		{
			name:        "explicit port",
			url:         "https://cdn.thegamesdb.net:443/images/test.jpg",
			errContains: "explicit ports are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(svc)
			} else {
				svc.lookupIPAddr = net.DefaultResolver.LookupIPAddr
			}
			_, _, err := svc.validateExternalImageURL(context.Background(), provider, tt.url)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}

func TestGetProviderImageReturnsValidatedImage(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
	})
	trustTGDBImageHost(svc)
	setHTTPRoundTripper(svc, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://cdn.thegamesdb.net/images/large/boxart/front/123.jpg" {
			t.Fatalf("unexpected provider image URL: %s", req.URL.String())
		}
		return httpTestResponse(http.StatusOK, testPNGBytes(), "image/png"), nil
	})

	body, contentType, err := svc.GetProviderImage(context.Background(), "tgdb", "boxart/front/123.jpg")
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", contentType)
	}
	if !bytes.Equal(body, testPNGBytes()) {
		t.Fatal("provider image body did not match the validated response")
	}
}

func TestGetProviderImageRejectsInvalidRequests(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
	})

	if _, _, err := svc.GetProviderImage(context.Background(), "unknown", "front.jpg"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("unknown provider error = %v, want ErrProviderNotFound", err)
	}
	if _, _, err := svc.GetProviderImage(context.Background(), "tgdb", " "); !errors.Is(err, ErrProviderImageInvalid) {
		t.Fatalf("empty image id error = %v, want ErrProviderImageInvalid", err)
	}
}

func defaultProviderSelection(imageURL string) *MetadataProviderSelection {
	return &MetadataProviderSelection{
		ID:          "123",
		Title:       "My Game",
		ReleaseDate: "2024-01-01",
		Description: "description",
		ImageURL:    imageURL,
	}
}

func trustTGDBImageHost(svc *Service) {
	svc.lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	}
}

func httpTestResponse(statusCode int, body []byte, contentType string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
}

func setHTTPRoundTripper(svc *Service, fn func(*http.Request) (*http.Response, error)) {
	svc.httpClient = &http.Client{Transport: roundTripFunc(fn)}
}

func applyProviderSelection(t *testing.T, svc *Service, taskHash string) *ApplyProviderSelectionResult {
	t.Helper()

	result, err := svc.ApplyProviderSelection(context.Background(), "tgdb", taskHash, "123")
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	return result
}

func assertEmptyUploadDir(t *testing.T, uploadDir, reason string) {
	t.Helper()

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatalf("failed to read upload dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files to be written %s, found %d", reason, len(entries))
	}
}

func assertMetadataWithoutImageWarning(t *testing.T, result *ApplyProviderSelectionResult, reason string) {
	t.Helper()

	if result == nil || result.Metadata == nil || result.Metadata.ImagePath != "" {
		t.Fatalf("expected metadata without image when %s is skipped, got %#v", reason, result)
	}
	if result.WarningReason == "" {
		t.Fatalf("expected warning reason for %s, got %#v", reason, result)
	}
}

func setupMockedProviderService(t *testing.T, selectionImageURL string, bodyBytes []byte, contentType string) (*Service, string) {
	svc, uploadDir := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
		selection:         defaultProviderSelection(selectionImageURL),
	})
	trustTGDBImageHost(svc)
	setHTTPRoundTripper(svc, func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "cdn.thegamesdb.net" {
			return nil, fmt.Errorf("unexpected host: %s", req.URL.Hostname())
		}
		return httpTestResponse(http.StatusOK, bodyBytes, contentType), nil
	})
	return svc, uploadDir
}

func TestApplyProviderSelectionAcceptsTrustedProviderImage(t *testing.T) {
	svc, uploadDir := setupMockedProviderService(t, tgdbImageURL, testPNGBytes(), "image/png")

	result, err := svc.ApplyProviderSelection(
		context.Background(),
		"tgdb",
		"task123",
		"123",
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if result == nil || result.Metadata == nil || result.Metadata.ImagePath == "" {
		t.Fatal("expected image path to be set")
	}
	if _, err := os.Stat(result.Metadata.ImagePath); err != nil {
		t.Fatalf("expected downloaded image to exist: %v", err)
	}
	if !strings.HasPrefix(result.Metadata.ImagePath, uploadDir) {
		t.Fatalf("expected image path under upload dir, got %s", result.Metadata.ImagePath)
	}
	if filepath.Ext(result.Metadata.ImagePath) != ".png" {
		t.Fatalf("expected detected .png extension to be used, got %s", result.Metadata.ImagePath)
	}
}

func TestApplyProviderSelectionDeletesOldImageAfterSuccessfulUpdate(t *testing.T) {
	svc, uploadDir := setupMockedProviderService(t, tgdbImageURL, testPNGBytes(), "image/png")
	ctx := context.Background()

	oldImagePath := createTestFile(t, uploadDir, "old-image.jpg", "old")
	createTestMetadata(t, svc, &models.TaskMetadata{
		TaskHash:    "task-update-success",
		Name:        "Old Name",
		Description: "Old description",
		ImagePath:   oldImagePath,
	})

	result, err := svc.ApplyProviderSelection(
		ctx,
		"tgdb",
		"task-update-success",
		"123",
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if result == nil || result.Metadata == nil || result.Metadata.ImagePath == "" {
		t.Fatal("expected updated metadata with image path")
	}
	if result.Metadata.ImagePath == oldImagePath {
		t.Fatal("expected a newly downloaded image path")
	}
	if _, err := os.Stat(result.Metadata.ImagePath); err != nil {
		t.Fatalf("expected new image to exist: %v", err)
	}
	if _, err := os.Stat(oldImagePath); !os.IsNotExist(err) {
		t.Fatal("expected old image to be removed after successful update")
	}

	stored, err := svc.repo.GetByTaskHash(ctx, "task-update-success")
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if stored == nil || stored.ImagePath != result.Metadata.ImagePath {
		t.Fatal("expected database to reference the new image path")
	}
}

func TestApplyProviderSelectionRemovesNewImageWhenUpdateFails(t *testing.T) {
	svc, uploadDir := setupMockedProviderService(t, tgdbImageURL, testPNGBytes(), "image/png")
	ctx := context.Background()

	oldImagePath := createTestFile(t, uploadDir, "old-image-failure.jpg", "old")
	createTestMetadata(t, svc, &models.TaskMetadata{
		TaskHash:    "task-update-failure",
		Name:        "Old Name",
		Description: "Old description",
		ImagePath:   oldImagePath,
	})

	callbackName := "test:fail_task_metadata_update"
	if err := svc.db.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "task_metadata" {
			_ = tx.AddError(errors.New("forced update failure"))
		}
	}); err != nil {
		t.Fatalf("failed to register update callback: %v", err)
	}
	defer func() {
		_ = svc.db.DB.Callback().Update().Remove(callbackName)
	}()

	result, err := svc.ApplyProviderSelection(
		ctx,
		"tgdb",
		"task-update-failure",
		"123",
	)
	if err == nil || !strings.Contains(err.Error(), "forced update failure") {
		t.Fatalf("expected forced update failure, got result=%v err=%v", result, err)
	}

	stored, repoErr := svc.repo.GetByTaskHash(ctx, "task-update-failure")
	if repoErr != nil {
		t.Fatalf(unexpectedErrFmt, repoErr)
	}
	if stored == nil {
		t.Fatal("expected original metadata row to remain")
	}
	if stored.ImagePath != oldImagePath {
		t.Fatalf("expected database to keep old image path, got %q", stored.ImagePath)
	}
	if _, err := os.Stat(oldImagePath); err != nil {
		t.Fatalf("expected old image to remain after failed update: %v", err)
	}

	entries, readErr := os.ReadDir(uploadDir)
	if readErr != nil {
		t.Fatalf("failed to read upload dir: %v", readErr)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(oldImagePath) {
		t.Fatalf("expected only the original image to remain, found %v", entries)
	}
}

func TestApplyProviderSelectionSkipsOversizedImage(t *testing.T) {
	oversizedBody := append(testPNGBytes(), bytes.Repeat([]byte("a"), MaxFileSize)...)
	svc, uploadDir := setupMockedProviderService(t, tgdbImageURL, oversizedBody, "image/png")

	result := applyProviderSelection(t, svc, "task123")
	assertMetadataWithoutImageWarning(t, result, "oversized image")
	assertEmptyUploadDir(t, uploadDir, "for oversized image")
}

func TestApplyProviderSelectionSkipsNonImageContent(t *testing.T) {
	svc, uploadDir := setupMockedProviderService(t, tgdbImageURL, []byte("not-an-image"), "text/plain")

	result := applyProviderSelection(t, svc, "task123")
	assertMetadataWithoutImageWarning(t, result, "invalid image content")
	assertEmptyUploadDir(t, uploadDir, "for invalid image")
}

func TestApplyProviderSelectionSkipsImageWhenDownloadReturns404(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
		selection:         defaultProviderSelection(tgdbImageURL),
	})
	trustTGDBImageHost(svc)
	setHTTPRoundTripper(svc, func(*http.Request) (*http.Response, error) {
		return httpTestResponse(http.StatusNotFound, nil, "text/plain"), nil
	})

	result, err := svc.ApplyProviderSelection(
		context.Background(),
		"tgdb",
		"task123",
		"123",
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if result == nil || result.Metadata == nil {
		t.Fatal("expected metadata result")
	}
	if result.Metadata.Name != "My Game" {
		t.Fatalf("expected metadata name to be saved, got %q", result.Metadata.Name)
	}
	if result.Metadata.ImagePath != "" {
		t.Fatalf("expected image path to be empty when download fails, got %q", result.Metadata.ImagePath)
	}
	if result.WarningReason == "" {
		t.Fatalf("expected warning reason when image download fails, got %#v", result)
	}

	assertEmptyUploadDir(t, uploadDir, "when image download fails")
}

func TestApplyProviderSelectionUsesDetectedExtensionWhenURLHasNoValidImageExt(t *testing.T) {
	svc, _ := setupMockedProviderService(t, "https://cdn.thegamesdb.net/images/large/boxart/front/123.txt", testPNGBytes(), "image/png")

	result, err := svc.ApplyProviderSelection(
		context.Background(),
		"tgdb",
		"task123",
		"123",
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if filepath.Ext(result.Metadata.ImagePath) != ".png" {
		t.Fatalf("expected detected .png extension, got %s", result.Metadata.ImagePath)
	}
}

func TestApplyProviderSelectionWithFallbackUsesImageIDForDownload(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
		resolveErr:        fmt.Errorf("unexpected status code: 404"),
	})
	trustTGDBImageHost(svc)
	setHTTPRoundTripper(svc, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://cdn.thegamesdb.net/images/large/front.jpg" {
			t.Fatalf("expected image download request built from image id, got %s", req.URL.String())
		}
		return httpTestResponse(http.StatusOK, testPNGBytes(), "image/png"), nil
	})

	result, err := svc.ApplyProviderSelectionWithFallback(
		context.Background(),
		"tgdb",
		"task123",
		"123",
		&MetadataProviderSelection{
			ID:          "123",
			Title:       "Fallback Game",
			ReleaseDate: "2024-01-01",
			Description: "Overview",
			ImageID:     "front.jpg",
		},
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if result == nil || result.Metadata == nil {
		t.Fatal("expected metadata result")
	}
	if result.Metadata.Name != "Fallback Game" {
		t.Fatalf("expected fallback metadata name to be saved, got %q", result.Metadata.Name)
	}
	if result.Metadata.ImagePath == "" {
		t.Fatal("expected image path when fallback includes image id")
	}
	if result.Warning != "provider_selection_fallback_used" {
		t.Fatalf("expected fallback warning, got %q", result.Warning)
	}
	if result.WarningReason == "" {
		t.Fatalf("expected warning reason for fallback, got %#v", result)
	}

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatalf("failed to read upload dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one file to be written during fallback, found %d", len(entries))
	}
}

func TestApplyProviderSelectionWithFallbackWarnsOnEmptyResolution(t *testing.T) {
	svc, uploadDir := setupTestService(t)
	svc.providerRegistry = NewMetadataProviderRegistry(mockMetadataProvider{
		name:              "tgdb",
		allowedImageHosts: []string{"cdn.thegamesdb.net"},
		selection:         nil,
	})
	setHTTPRoundTripper(svc, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("expected no image download request during fallback, got %s", req.URL.String())
		return nil, nil
	})

	result, err := svc.ApplyProviderSelectionWithFallback(
		context.Background(),
		"tgdb",
		"task123",
		"123",
		&MetadataProviderSelection{
			ID:          "123",
			Title:       "Fallback Game",
			ReleaseDate: "2024-01-01",
			Description: "Overview",
		},
	)
	if err != nil {
		t.Fatalf(unexpectedErrFmt, err)
	}
	if result == nil || result.Metadata == nil {
		t.Fatal("expected metadata result")
	}
	if result.Warning != "provider_selection_fallback_used" {
		t.Fatalf("expected fallback warning, got %q", result.Warning)
	}
	if result.WarningReason != ErrProviderSelectionNotFound.Error() {
		t.Fatalf("expected fallback warning reason %q, got %q", ErrProviderSelectionNotFound.Error(), result.WarningReason)
	}
	if result.Metadata.ImagePath != "" {
		t.Fatalf("expected no image path when fallback is used, got %q", result.Metadata.ImagePath)
	}

	assertEmptyUploadDir(t, uploadDir, "during fallback")
}

func testPNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xE5, 0x27, 0xD4, 0xA2, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}

type mockMetadataProvider struct {
	name              string
	allowedImageHosts []string
	selection         *MetadataProviderSelection
	resolveErr        error
}

func (m mockMetadataProvider) Name() string {
	return m.name
}

func (m mockMetadataProvider) Status(context.Context) (*MetadataProviderStatus, error) {
	return &MetadataProviderStatus{Provider: m.name, Active: true}, nil
}

func (m mockMetadataProvider) Search(context.Context, string) ([]MetadataProviderSearchResult, error) {
	return nil, nil
}

func (m mockMetadataProvider) Resolve(context.Context, string) (*MetadataProviderSelection, error) {
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	if m.selection == nil {
		return nil, ErrProviderSelectionNotFound
	}
	return m.selection, nil
}

func (m mockMetadataProvider) BuildImageURL(imageID string) (string, error) {
	if strings.TrimSpace(imageID) == "" {
		return "", nil
	}
	return "https://cdn.thegamesdb.net/images/large/" + imageID, nil
}

func (m mockMetadataProvider) AllowedImageHosts() []string {
	return m.allowedImageHosts
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
