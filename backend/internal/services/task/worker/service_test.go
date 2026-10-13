package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/schemas"
)

// mockRepository is a mock implementation of the task repository for testing
type mockRepository struct {
	tasks              map[string]*entities.Task
	stopError          error
	startError         error
	forceError         error
	deleteError        error
	createError        error
	limitsError        error
	filePriorityError  error
	queuePriorityError error
	peersError         error
	lastQueueAction    string
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		tasks: make(map[string]*entities.Task),
	}
}

func (m *mockRepository) List() ([]*entities.Task, error) {
	tasks := make([]*entities.Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (m *mockRepository) Get(hash string) (*entities.Task, error) {
	if task, exists := m.tasks[hash]; exists {
		return task, nil
	}
	return nil, errors.New("task not found")
}

func (m *mockRepository) Add(schema schemas.TaskCreateSchema) (*entities.Task, error) {
	if m.createError != nil {
		return nil, m.createError
	}

	task := &entities.Task{
		ID:        "test-hash",
		Name:      "Test Task",
		Hash:      "test-hash",
		Category:  schema.Category,
		Path:      schema.Directory,
		State:     "DOWNLOADING",
		MagnetURI: schema.MagnetURI,
	}

	m.tasks[task.Hash] = task
	return task, nil
}

func (m *mockRepository) AddFile(fileName string, fileData []byte, hashes []string, schema schemas.TaskCreateFromFileSchema) (*entities.Task, error) {
	if m.createError != nil {
		return nil, m.createError
	}

	task := &entities.Task{
		ID:       "test-file-hash",
		Name:     fileName,
		Hash:     "test-file-hash",
		Category: schema.Category,
		Path:     schema.Directory,
		State:    "DOWNLOADING",
	}

	m.tasks[task.Hash] = task
	return task, nil
}

func (m *mockRepository) Stop(hash string) error {
	if m.stopError != nil {
		return m.stopError
	}
	if task, exists := m.tasks[hash]; exists {
		task.State = "PAUSED_DOWNLOAD"
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) Start(hash string) error {
	if m.startError != nil {
		return m.startError
	}
	if task, exists := m.tasks[hash]; exists {
		task.State = "DOWNLOADING"
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) ForceResume(hash string) error {
	if m.forceError != nil {
		return m.forceError
	}
	if task, exists := m.tasks[hash]; exists {
		task.State = "FORCED_DOWNLOAD"
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) Delete(id string, deleteFiles bool) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	if _, exists := m.tasks[id]; exists {
		delete(m.tasks, id)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetTags(hash string, tags []string) error {
	if task, exists := m.tasks[hash]; exists {
		task.Tags = tags
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) AddTags(hash string, tags []string) error {
	if task, exists := m.tasks[hash]; exists {
		task.Tags = append(task.Tags, tags...)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetCategory(hash string, category string) error {
	if task, exists := m.tasks[hash]; exists {
		task.Category = category
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetShareLimit(hash string, limits entities.TaskShareLimit) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate setting share limit (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetLocation(hash string, schema schemas.TaskSetLocationSchema) error {
	if task, exists := m.tasks[hash]; exists {
		task.Path = schema.Location
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) Rename(hash string, schema schemas.TaskRenameSchema) error {
	if task, exists := m.tasks[hash]; exists {
		task.Name = schema.NewName
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetSuperSeeding(hash string, schema schemas.TaskSuperSeedingSchema) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate setting super seeding mode (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) ForceRecheck(hash string) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate force recheck (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) ForceReannounce(hash string) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate force reannounce (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetDownloadLimit(hash string, schema schemas.TaskSetDownloadLimitSchema) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate setting download limit (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) SetUploadLimit(hash string, schema schemas.TaskSetUploadLimitSchema) error {
	if _, exists := m.tasks[hash]; exists {
		// Simulate setting upload limit (in real implementation, this would be handled by qBittorrent)
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) ListFiles(hash string) ([]*entities.TaskFile, error) {
	if _, exists := m.tasks[hash]; exists {
		// Return mock files for testing
		return []*entities.TaskFile{
			{
				Index:        0,
				Name:         "file1.txt",
				Size:         1024,
				Progress:     0.5,
				Priority:     1,
				IsSeed:       false,
				PieceRange:   [2]int{0, 10},
				Availability: 1.0,
			},
			{
				Index:        1,
				Name:         "file2.txt",
				Size:         2048,
				Progress:     1.0,
				Priority:     1,
				IsSeed:       true,
				PieceRange:   [2]int{11, 20},
				Availability: 1.0,
			},
		}, nil
	}
	return nil, errors.New("task not found")
}

func (m *mockRepository) SetFilePriority(hash string, schema schemas.TaskSetFilePrioritySchema) error {
	if m.filePriorityError != nil {
		return m.filePriorityError
	}
	return m.requireExistingTask(hash)
}

func (m *mockRepository) ListTrackers(hash string) ([]*entities.TaskTracker, error) {
	if _, exists := m.tasks[hash]; exists {
		return []*entities.TaskTracker{
			{URL: "https://tracker.example/announce", Status: 2, Tier: 0, NumPeers: 5, NumSeeds: 3, NumLeeches: 2, NumDownloaded: 10},
		}, nil
	}
	return nil, errors.New("task not found")
}

func (m *mockRepository) requireExistingTask(hash string) error {
	if _, exists := m.tasks[hash]; exists {
		return nil
	}
	return errors.New("task not found")
}

func (m *mockRepository) AddTrackers(hash string, urls []string) error {
	return m.requireExistingTask(hash)
}

func (m *mockRepository) RemoveTrackers(hash string, urls []string) error {
	return m.requireExistingTask(hash)
}

func (m *mockRepository) EditTracker(hash, origURL, newURL string) error {
	return m.requireExistingTask(hash)
}

func (m *mockRepository) SetQueuePriority(hash string, action string) error {
	m.lastQueueAction = action
	if m.queuePriorityError != nil {
		return m.queuePriorityError
	}
	return m.requireExistingTask(hash)
}

func (m *mockRepository) ListPeers(hash string) ([]*entities.TaskPeer, error) {
	if m.peersError != nil {
		return nil, m.peersError
	}
	if _, exists := m.tasks[hash]; exists {
		return []*entities.TaskPeer{
			{IP: "203.0.113.5", Port: 51413, Client: "qBittorrent/4.6.0", Country: "Brazil", CountryCode: "BR"},
		}, nil
	}
	return nil, errors.New("task not found")
}

func (m *mockRepository) GetLimits(hash string) (*entities.TaskLimits, error) {
	if m.limitsError != nil {
		return nil, m.limitsError
	}
	return &entities.TaskLimits{
		DownloadLimit: 1024 * 1024 * 1024, // 1GB
		UploadLimit:   512 * 1024 * 1024,  // 512MB
		ShareLimit:    2.0,
	}, nil
}

func TestServiceStopTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{
		ID:    "test-hash",
		Hash:  "test-hash",
		Name:  "Test Task",
		State: "DOWNLOADING",
	}
	mockRepo.tasks["test-hash"] = task

	// Test successful stop
	err := service.StopTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if task.State != "PAUSED_DOWNLOAD" {
		t.Errorf("Expected task state to be PAUSED_DOWNLOAD, got %s", task.State)
	}

	// Test stop with repository error
	mockRepo.stopError = errors.New("repository error")
	err = service.StopTask(ctx, "test-hash")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Test stop with non-existent task
	mockRepo.stopError = nil
	err = service.StopTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceStartTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{
		ID:    "test-hash",
		Hash:  "test-hash",
		Name:  "Test Task",
		State: "PAUSED_DOWNLOAD",
	}
	mockRepo.tasks["test-hash"] = task

	// Test successful start
	err := service.StartTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if task.State != "DOWNLOADING" {
		t.Errorf("Expected task state to be DOWNLOADING, got %s", task.State)
	}

	// Test start with repository error
	mockRepo.startError = errors.New("repository error")
	err = service.StartTask(ctx, "test-hash")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Test start with non-existent task
	mockRepo.startError = nil
	err = service.StartTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceForceResumeTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{
		ID:    "test-hash",
		Hash:  "test-hash",
		Name:  "Test Task",
		State: "STALLED_DOWNLOAD",
	}
	mockRepo.tasks["test-hash"] = task

	// Test successful force resume
	err := service.ForceResumeTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if task.State != "FORCED_DOWNLOAD" {
		t.Errorf("Expected task state to be FORCED_DOWNLOAD, got %s", task.State)
	}

	// Test force resume with repository error
	mockRepo.forceError = errors.New("repository error")
	err = service.ForceResumeTask(ctx, "test-hash")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Test force resume with non-existent task
	mockRepo.forceError = nil
	err = service.ForceResumeTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceListTasks(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Test empty list
	tasks, err := service.ListTasks(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}

	// Add test tasks
	task1 := &entities.Task{ID: "hash1", Hash: "hash1", Name: "Task 1"}
	task2 := &entities.Task{ID: "hash2", Hash: "hash2", Name: "Task 2"}
	mockRepo.tasks["hash1"] = task1
	mockRepo.tasks["hash2"] = task2

	// Test list with tasks
	tasks, err = service.ListTasks(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
}

func TestServiceGetTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	// Test successful get
	retrieved, err := service.GetTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if retrieved.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, retrieved.ID)
	}

	// Test get with non-existent task
	_, err = service.GetTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceCreateTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	schema := schemas.TaskCreateSchema{
		MagnetURI: "magnet:?xt=urn:btih:test-hash",
		Category:  "test-category",
		Directory: "/test/path",
		Tags:      []string{"tag1", "tag2"},
	}

	// Test successful create
	task, err := service.CreateTask(ctx, schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if task == nil {
		t.Error("Expected task, got nil")
		return
	}

	if task.Category != schema.Category {
		t.Errorf("Expected category %s, got %s", schema.Category, task.Category)
	}
	if !task.WasCreated {
		t.Error("Expected a newly added task to be marked as created")
	}

	// Test create with repository error
	delete(mockRepo.tasks, task.Hash)
	mockRepo.createError = errors.New("repository error")
	_, err = service.CreateTask(ctx, schema)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestServiceCreateTaskFromFileDedupesV2InfoHash(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	info := "d9:file treede4:name8:test.iso12:piece lengthi16384e12:meta versioni2ee"
	torrent := []byte(fmt.Sprintf("d4:info%se", info))
	v2Hash := sha256.Sum256([]byte(info))
	existingHash := hex.EncodeToString(v2Hash[:])
	existing := &entities.Task{ID: existingHash, Hash: existingHash[:40], InfoHashV2: existingHash, Name: "Existing v2 Task"}
	mockRepo.tasks[existingHash] = existing

	task, err := service.CreateTaskFromFile(ctx, "test.torrent", torrent, schemas.TaskCreateFromFileSchema{
		Category: "movies",
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task != existing {
		t.Fatalf("expected existing v2 task, got %#v", task)
	}
	if task.WasCreated {
		t.Error("Expected a deduplicated task not to be marked as created")
	}
}

func TestServiceCreateTaskMarksDeduplicatedTaskAsNotCreated(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}
	existing := &entities.Task{
		ID:         "existing-hash",
		Hash:       "existing-hash",
		WasCreated: true,
		MagnetLink: entities.TaskMagnetLink{Hash: "existing-hash"},
	}
	mockRepo.tasks[existing.Hash] = existing

	task, err := service.CreateTask(ctx, schemas.TaskCreateSchema{
		MagnetURI: "magnet:?xt=urn:btih:existing-hash",
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if task != existing {
		t.Fatalf("expected existing task, got %#v", task)
	}
	if task.WasCreated {
		t.Error("Expected a deduplicated task to be marked as not created")
	}
}

func TestServiceCreateTaskDeduplicatesBase32Magnet(t *testing.T) {
	const hash = "5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36"
	mockRepo := newMockRepository()
	existing := &entities.Task{
		ID: hash, Hash: hash, WasCreated: true,
		MagnetLink: entities.TaskMagnetLink{Hash: "lignvs6vk4ppzxk6e6p4wdurhnwdnszw"},
	}
	mockRepo.tasks[hash] = existing
	// An Add call would fail, so success verifies that deduplication happened first.
	mockRepo.createError = errors.New("must not add a duplicate torrent")
	service := &service{repository: mockRepo}
	task, err := service.CreateTask(context.Background(), schemas.TaskCreateSchema{
		MagnetURI: "magnet:?xt=urn:btih:lignvs6vk4ppzxk6e6p4wdurhnwdnszw",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task != existing || task.WasCreated {
		t.Fatalf("expected existing task without creation flag, got %#v", task)
	}
}

func TestServiceDeleteTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	// Test successful delete
	err := service.DeleteTask(ctx, "test-hash", false)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify task was deleted
	if _, exists := mockRepo.tasks["test-hash"]; exists {
		t.Error("Expected task to be deleted")
	}

	// Test delete with repository error
	mockRepo.deleteError = errors.New("repository error")
	err = service.DeleteTask(ctx, "test-hash", false)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestServiceSetTaskShareLimit(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSetShareLimitSchema{
		RatioLimit:               2.0,
		SeedingTimeLimit:         3600,
		InactiveSeedingTimeLimit: -2,
	}

	// Test successful share limit setting
	err := service.SetTaskShareLimit(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.SetTaskShareLimit(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceSetFilePriority(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	priority := 0
	schema := schemas.TaskSetFilePrioritySchema{
		Indexes:  []int{0, 1},
		Priority: &priority,
	}

	// Test successful file priority setting
	err := service.SetFilePriority(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.SetFilePriority(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}

	// Test repository error propagation
	mockRepo.filePriorityError = fmt.Errorf("qbittorrent unavailable")
	err = service.SetFilePriority(ctx, "test-hash", schema)
	if err == nil {
		t.Error("Expected error when repository fails, got nil")
	}
}

func TestServiceSetTaskQueuePriority(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSetQueuePrioritySchema{Action: "top"}

	err := service.SetTaskQueuePriority(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if mockRepo.lastQueueAction != "top" {
		t.Errorf("Expected action 'top' to reach repository, got %q", mockRepo.lastQueueAction)
	}

	err = service.SetTaskQueuePriority(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}

	mockRepo.queuePriorityError = fmt.Errorf("qbittorrent unavailable")
	err = service.SetTaskQueuePriority(ctx, "test-hash", schema)
	if err == nil {
		t.Error("Expected error when repository fails, got nil")
	}
}

func TestServiceListTaskPeers(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	peers, err := service.ListTaskPeers(ctx, "test-hash")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(peers) != 1 || peers[0].CountryCode != "BR" {
		t.Errorf("Expected one BR peer, got %+v", peers)
	}

	_, err = service.ListTaskPeers(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}

	mockRepo.peersError = fmt.Errorf("qbittorrent unavailable")
	_, err = service.ListTaskPeers(ctx, "test-hash")
	if err == nil {
		t.Error("Expected error when repository fails, got nil")
	}
}

func TestServiceSetTaskLocation(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task", Path: "/old/path"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSetLocationSchema{
		Location: "/new/path",
	}

	// Test successful location setting
	err := service.SetTaskLocation(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the location was set
	if task.Path != "/new/path" {
		t.Errorf("Expected path /new/path, got %s", task.Path)
	}

	// Test with non-existent task
	err = service.SetTaskLocation(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceRenameTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Old Name"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskRenameSchema{
		NewName: "New Name",
	}

	// Test successful rename
	err := service.RenameTask(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the name was changed
	if task.Name != "New Name" {
		t.Errorf("Expected name 'New Name', got %s", task.Name)
	}

	// Test with non-existent task
	err = service.RenameTask(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceSetTaskSuperSeeding(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSuperSeedingSchema{
		Enabled: true,
	}

	// Test successful super seeding setting
	err := service.SetTaskSuperSeeding(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.SetTaskSuperSeeding(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceForceRecheckTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	// Test successful force recheck
	err := service.ForceRecheckTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.ForceRecheckTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceForceReannounceTask(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	// Test successful force reannounce
	err := service.ForceReannounceTask(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.ForceReannounceTask(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceSetTaskDownloadLimit(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSetDownloadLimitSchema{
		Limit: 1024000, // 1 MB/s in bytes
	}

	// Test successful download limit setting
	err := service.SetTaskDownloadLimit(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.SetTaskDownloadLimit(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceSetTaskUploadLimit(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	schema := schemas.TaskSetUploadLimitSchema{
		Limit: 512000, // 512 KB/s in bytes
	}

	// Test successful upload limit setting
	err := service.SetTaskUploadLimit(ctx, "test-hash", schema)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test with non-existent task
	err = service.SetTaskUploadLimit(ctx, "non-existent-hash", schema)
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceListTaskFiles(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Add a test task
	task := &entities.Task{ID: "test-hash", Hash: "test-hash", Name: "Test Task"}
	mockRepo.tasks["test-hash"] = task

	// Test successful list files
	files, err := service.ListTaskFiles(ctx, "test-hash")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Verify first file
	if files[0].Name != "file1.txt" {
		t.Errorf("Expected file name 'file1.txt', got %s", files[0].Name)
	}
	if files[0].Size != 1024 {
		t.Errorf("Expected file size 1024, got %d", files[0].Size)
	}
	if files[0].Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", files[0].Progress)
	}
	if files[0].IsSeed {
		t.Error("Expected IsSeed to be false")
	}

	// Verify second file
	if files[1].Name != "file2.txt" {
		t.Errorf("Expected file name 'file2.txt', got %s", files[1].Name)
	}
	if files[1].Size != 2048 {
		t.Errorf("Expected file size 2048, got %d", files[1].Size)
	}
	if files[1].Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", files[1].Progress)
	}
	if !files[1].IsSeed {
		t.Error("Expected IsSeed to be true")
	}

	// Test with non-existent task
	_, err = service.ListTaskFiles(ctx, "non-existent-hash")
	if err == nil {
		t.Error("Expected error for non-existent task, got nil")
	}
}

func TestServiceGetTasksStats(t *testing.T) {
	ctx := context.Background()
	mockRepo := newMockRepository()
	service := &service{repository: mockRepo}

	// Test with empty tasks list
	stats, err := service.GetTasksStats(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if stats.TotalDiskSize != 0 {
		t.Errorf("Expected total disk size 0, got %d", stats.TotalDiskSize)
	}
	if stats.CurrentUploadSpeed != 0 {
		t.Errorf("Expected current upload speed 0, got %d", stats.CurrentUploadSpeed)
	}
	if stats.CurrentDownloadSpeed != 0 {
		t.Errorf("Expected current download speed 0, got %d", stats.CurrentDownloadSpeed)
	}
	if stats.AverageRatio != 0 {
		t.Errorf("Expected average ratio 0, got %f", stats.AverageRatio)
	}
	if stats.MedianRatio != 0 {
		t.Errorf("Expected median ratio 0, got %f", stats.MedianRatio)
	}
	if stats.HighestRatio != 0 {
		t.Errorf("Expected highest ratio 0, got %f", stats.HighestRatio)
	}
	if stats.LowestRatio != 0 {
		t.Errorf("Expected lowest ratio 0, got %f", stats.LowestRatio)
	}
	if stats.ActiveTasksCount != 0 {
		t.Errorf("Expected active tasks count 0, got %d", stats.ActiveTasksCount)
	}
	if stats.ActiveSeeds != 0 {
		t.Errorf("Expected active seeds 0, got %d", stats.ActiveSeeds)
	}
	if stats.ActivePeers != 0 {
		t.Errorf("Expected active peers 0, got %d", stats.ActivePeers)
	}
	if len(stats.CategoryUsage) != 0 {
		t.Errorf("Expected empty category usage, got %v", stats.CategoryUsage)
	}
	if len(stats.TagsUsage) != 0 {
		t.Errorf("Expected empty tags usage, got %v", stats.TagsUsage)
	}

	// Add test tasks with various properties
	task1 := &entities.Task{
		ID:       "hash1",
		Hash:     "hash1",
		Name:     "Task 1",
		State:    "DOWNLOADING",
		Category: "movies",
		Size:     1024 * 1024 * 1024, // 1 GB
		Ratio:    1.5,
		Tags:     []string{"hd", "action"},
		Network: entities.TaskNetwork{
			Upload:   entities.TaskUpload{Speed: 1000000},   // 1 MB/s
			Download: entities.TaskDownload{Speed: 2000000}, // 2 MB/s
		},
		Pairs: entities.TaskPairs{
			Seeders:  10,
			Leechers: 5,
		},
	}

	task2 := &entities.Task{
		ID:       "hash2",
		Hash:     "hash2",
		Name:     "Task 2",
		State:    "SEEDING",
		Category: "tv",
		Size:     512 * 1024 * 1024, // 512 MB
		Ratio:    2.5,
		Tags:     []string{"hd", "comedy"},
		Network: entities.TaskNetwork{
			Upload:   entities.TaskUpload{Speed: 500000}, // 500 KB/s
			Download: entities.TaskDownload{Speed: 0},    // 0 KB/s
		},
		Pairs: entities.TaskPairs{
			Seeders:  15,
			Leechers: 3,
		},
	}

	task3 := &entities.Task{
		ID:       "hash3",
		Hash:     "hash3",
		Name:     "Task 3",
		State:    "error",
		Category: "movies",
		Size:     256 * 1024 * 1024, // 256 MB
		Ratio:    0.5,
		Tags:     []string{"4k", "action"},
		Network: entities.TaskNetwork{
			Upload:   entities.TaskUpload{Speed: 0},   // 0 KB/s
			Download: entities.TaskDownload{Speed: 0}, // 0 KB/s
		},
		Pairs: entities.TaskPairs{
			Seeders:  0,
			Leechers: 0,
		},
	}

	task4 := &entities.Task{
		ID:       "hash4",
		Hash:     "hash4",
		Name:     "Task 4",
		State:    "paused",
		Category: "music",
		Size:     128 * 1024 * 1024, // 128 MB
		Ratio:    3.0,
		Tags:     []string{"mp3", "rock"},
		Network: entities.TaskNetwork{
			Upload:   entities.TaskUpload{Speed: 250000}, // 250 KB/s
			Download: entities.TaskDownload{Speed: 0},    // 0 KB/s
		},
		Pairs: entities.TaskPairs{
			Seeders:  8,
			Leechers: 2,
		},
	}

	mockRepo.tasks["hash1"] = task1
	mockRepo.tasks["hash2"] = task2
	mockRepo.tasks["hash3"] = task3
	mockRepo.tasks["hash4"] = task4

	// Test with multiple tasks
	stats, err = service.GetTasksStats(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test total disk size (1GB + 512MB + 256MB + 128MB = 1.896GB)
	expectedTotalSize := int64(1024*1024*1024 + 512*1024*1024 + 256*1024*1024 + 128*1024*1024)
	if stats.TotalDiskSize != expectedTotalSize {
		t.Errorf("Expected total disk size %d, got %d", expectedTotalSize, stats.TotalDiskSize)
	}

	// Test current speeds (1MB/s + 500KB/s + 0 + 250KB/s = 1.75MB/s upload, 2MB/s download)
	expectedUploadSpeed := 1000000 + 500000 + 0 + 250000
	expectedDownloadSpeed := 2000000 + 0 + 0 + 0
	if stats.CurrentUploadSpeed != expectedUploadSpeed {
		t.Errorf("Expected current upload speed %d, got %d", expectedUploadSpeed, stats.CurrentUploadSpeed)
	}
	if stats.CurrentDownloadSpeed != expectedDownloadSpeed {
		t.Errorf("Expected current download speed %d, got %d", expectedDownloadSpeed, stats.CurrentDownloadSpeed)
	}

	// Test average ratio (1.5 + 2.5 + 0.5 + 3.0) / 4 = 1.875
	expectedAverageRatio := (1.5 + 2.5 + 0.5 + 3.0) / 4.0
	if stats.AverageRatio != expectedAverageRatio {
		t.Errorf("Expected average ratio %f, got %f", expectedAverageRatio, stats.AverageRatio)
	}

	// Test median ratio (sorted: 0.5, 1.5, 2.5, 3.0) -> median = (1.5 + 2.5) / 2 = 2.0
	expectedMedianRatio := (1.5 + 2.5) / 2.0
	if stats.MedianRatio != expectedMedianRatio {
		t.Errorf("Expected median ratio %f, got %f", expectedMedianRatio, stats.MedianRatio)
	}

	// Test highest ratio (sorted: 0.5, 1.5, 2.5, 3.0) -> highest = 3.0
	expectedHighestRatio := 3.0
	if stats.HighestRatio != expectedHighestRatio {
		t.Errorf("Expected highest ratio %f, got %f", expectedHighestRatio, stats.HighestRatio)
	}

	// Test lowest ratio (sorted: 0.5, 1.5, 2.5, 3.0) -> lowest = 0.5
	expectedLowestRatio := 0.5
	if stats.LowestRatio != expectedLowestRatio {
		t.Errorf("Expected lowest ratio %f, got %f", expectedLowestRatio, stats.LowestRatio)
	}

	// Test active tasks count (should exclude "error" and "paused" states)
	// Active: DOWNLOADING, SEEDING = 2 tasks
	expectedActiveCount := 2
	if stats.ActiveTasksCount != expectedActiveCount {
		t.Errorf("Expected active tasks count %d, got %d", expectedActiveCount, stats.ActiveTasksCount)
	}

	// Test active seeds (10 + 15 + 0 + 8 = 33)
	expectedActiveSeeds := 10 + 15 + 0 + 8
	if stats.ActiveSeeds != expectedActiveSeeds {
		t.Errorf("Expected active seeds %d, got %d", expectedActiveSeeds, stats.ActiveSeeds)
	}

	// Test active peers (5 + 3 + 0 + 2 = 10)
	expectedActivePeers := 5 + 3 + 0 + 2
	if stats.ActivePeers != expectedActivePeers {
		t.Errorf("Expected active peers %d, got %d", expectedActivePeers, stats.ActivePeers)
	}

	// Test category usage
	expectedCategoryUsage := map[string]int{
		"movies": 2, // task1 and task3
		"tv":     1, // task2
		"music":  1, // task4
	}
	if len(stats.CategoryUsage) != len(expectedCategoryUsage) {
		t.Errorf("Expected %d categories, got %d", len(expectedCategoryUsage), len(stats.CategoryUsage))
	}
	for category, expectedCount := range expectedCategoryUsage {
		if stats.CategoryUsage[category] != expectedCount {
			t.Errorf("Expected category '%s' count %d, got %d", category, expectedCount, stats.CategoryUsage[category])
		}
	}

	// Test tags usage
	expectedTagsUsage := map[string]int{
		"hd":     2, // task1 and task2
		"action": 2, // task1 and task3
		"comedy": 1, // task2
		"4k":     1, // task3
		"mp3":    1, // task4
		"rock":   1, // task4
	}
	if len(stats.TagsUsage) != len(expectedTagsUsage) {
		t.Errorf("Expected %d tags, got %d", len(expectedTagsUsage), len(stats.TagsUsage))
	}
	for tag, expectedCount := range expectedTagsUsage {
		if stats.TagsUsage[tag] != expectedCount {
			t.Errorf("Expected tag '%s' count %d, got %d", tag, expectedCount, stats.TagsUsage[tag])
		}
	}
}
