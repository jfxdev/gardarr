package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribe(t *testing.T) {
	// Create a minimal service for testing
	svc := &Service{
		taskStates: make(map[uuid.UUID]map[string]*entities.TaskState),
	}

	// Initially empty
	assert.Empty(t, svc.subscribers)

	// Subscribe with buffer 100
	ch := svc.Subscribe(100)
	assert.NotNil(t, ch)
	assert.Equal(t, 100, cap(ch))
	assert.Equal(t, 1, len(svc.subscribers))

	// Subscribe with custom buffer 50
	ch2 := svc.Subscribe(50)
	assert.NotNil(t, ch2)
	assert.Equal(t, 50, cap(ch2))
	assert.Equal(t, 2, len(svc.subscribers))
}

func TestIsSignificantStateChange(t *testing.T) {
	tests := []struct {
		name        string
		oldState    string
		newState    string
		significant bool
	}{
		// Upload states - not significant
		{
			name:        "UPLOADING to STALLED_UPLOAD - not significant",
			oldState:    constants.TaskStatusUploading,
			newState:    constants.TaskStatusStalledUpload,
			significant: false,
		},
		{
			name:        "STALLED_UPLOAD to UPLOADING - not significant",
			oldState:    constants.TaskStatusStalledUpload,
			newState:    constants.TaskStatusUploading,
			significant: false,
		},
		// Download states - not significant
		{
			name:        "DOWNLOADING to STALLED_DOWNLOAD - not significant",
			oldState:    constants.TaskStatusDownloading,
			newState:    constants.TaskStatusStalledDownload,
			significant: false,
		},
		{
			name:        "STALLED_DOWNLOAD to DOWNLOADING - not significant",
			oldState:    constants.TaskStatusStalledDownload,
			newState:    constants.TaskStatusDownloading,
			significant: false,
		},
		// Different state groups - significant
		{
			name:        "DOWNLOADING to UPLOADING - significant",
			oldState:    constants.TaskStatusDownloading,
			newState:    constants.TaskStatusUploading,
			significant: true,
		},
		{
			name:        "UPLOADING to ERROR - significant",
			oldState:    constants.TaskStatusUploading,
			newState:    constants.TaskStatusError,
			significant: true,
		},
		{
			name:        "STALLED_UPLOAD to ERROR - significant",
			oldState:    constants.TaskStatusStalledUpload,
			newState:    constants.TaskStatusError,
			significant: true,
		},
		{
			name:        "ERROR to UPLOADING - significant",
			oldState:    constants.TaskStatusError,
			newState:    constants.TaskStatusUploading,
			significant: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSignificantStateChange(tt.oldState, tt.newState)
			assert.Equal(t, tt.significant, result)
		})
	}
}

func TestIsErrorState(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		isError bool
	}{
		{
			name:    "ERROR state",
			state:   constants.TaskStatusError,
			isError: true,
		},
		{
			name:    "MISSING_FILES state",
			state:   constants.TaskStatusMissingFiles,
			isError: true,
		},
		{
			name:    "UPLOADING is not error",
			state:   constants.TaskStatusUploading,
			isError: false,
		},
		{
			name:    "DOWNLOADING is not error",
			state:   constants.TaskStatusDownloading,
			isError: false,
		},
		{
			name:    "STALLED_UPLOAD is not error",
			state:   constants.TaskStatusStalledUpload,
			isError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isErrorState(tt.state)
			assert.Equal(t, tt.isError, result)
		})
	}
}

func TestLoadStates(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	// Initially should have empty states (loaded from empty DB)
	assert.NotNil(t, svc.taskStates)

	// Add some states to the database
	workerID := uuid.New()
	hash1 := "test-hash-1"
	hash2 := "test-hash-2"

	err = svc.repo.SaveTaskState(ctx, workerID, hash1, constants.TaskStatusUploading, 0.5, time.Now())
	require.NoError(t, err)

	err = svc.repo.SaveTaskState(ctx, workerID, hash2, constants.TaskStatusDownloading, 0.75, time.Now())
	require.NoError(t, err)

	// Reload states
	err = svc.LoadStates(ctx)
	require.NoError(t, err)

	// Verify states were loaded
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	assert.Contains(t, svc.taskStates, workerID)
	assert.Contains(t, svc.taskStates[workerID], hash1)
	assert.Contains(t, svc.taskStates[workerID], hash2)

	state1 := svc.taskStates[workerID][hash1]
	assert.Equal(t, constants.TaskStatusUploading, state1.State)
	assert.Equal(t, 0.5, state1.Progress)

	state2 := svc.taskStates[workerID][hash2]
	assert.Equal(t, constants.TaskStatusDownloading, state2.State)
	assert.Equal(t, 0.75, state2.Progress)
}

func TestTrackTasks_NewTask(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	workerID := uuid.New()
	timestamp := time.Now()

	tasks := []*entities.Task{
		{
			Hash:     "new-task-hash",
			Name:     "New Task",
			State:    constants.TaskStatusDownloading,
			Progress: 0.25,
		},
	}

	err = svc.TrackTasks(ctx, tasks, workerID, timestamp)
	require.NoError(t, err)

	// Verify state was saved in memory
	svc.mu.RLock()
	state := svc.taskStates[workerID]["new-task-hash"]
	svc.mu.RUnlock()

	assert.NotNil(t, state)
	assert.Equal(t, constants.TaskStatusDownloading, state.State)
	assert.Equal(t, 0.25, state.Progress)

	// Verify state was persisted to database
	states, err := svc.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	assert.Contains(t, states, "new-task-hash")
}

func TestTrackTasks_SignificantStateChange(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	workerID := uuid.New()
	taskHash := "test-hash"

	// Set initial state
	svc.mu.Lock()
	svc.taskStates[workerID] = map[string]*entities.TaskState{
		taskHash: {
			WorkerID:  workerID,
			Hash:      taskHash,
			State:     constants.TaskStatusDownloading,
			Progress:  0.5,
			UpdatedAt: time.Now(),
		},
	}
	svc.mu.Unlock()

	// Track task with significant state change (DOWNLOADING -> UPLOADING)
	tasks := []*entities.Task{
		{
			Hash:     taskHash,
			Name:     "Test Task",
			State:    constants.TaskStatusUploading,
			Progress: 1.0,
		},
	}

	err = svc.TrackTasks(ctx, tasks, workerID, time.Now())
	require.NoError(t, err)

	// Verify state was updated
	svc.mu.RLock()
	state := svc.taskStates[workerID][taskHash]
	svc.mu.RUnlock()

	assert.Equal(t, constants.TaskStatusUploading, state.State)
	assert.Equal(t, 1.0, state.Progress)
}

func TestTrackTasks_InsignificantStateChange(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	workerID := uuid.New()
	taskHash := "test-hash"

	// Set initial state
	svc.mu.Lock()
	svc.taskStates[workerID] = map[string]*entities.TaskState{
		taskHash: {
			WorkerID:  workerID,
			Hash:      taskHash,
			State:     constants.TaskStatusUploading,
			Progress:  0.5,
			UpdatedAt: time.Now(),
		},
	}
	svc.mu.Unlock()

	// Track task with insignificant state change (UPLOADING -> STALLED_UPLOAD)
	tasks := []*entities.Task{
		{
			Hash:     taskHash,
			Name:     "Test Task",
			State:    constants.TaskStatusStalledUpload,
			Progress: 0.5,
		},
	}

	err = svc.TrackTasks(ctx, tasks, workerID, time.Now())
	require.NoError(t, err)

	// Verify state was updated in memory
	svc.mu.RLock()
	state := svc.taskStates[workerID][taskHash]
	svc.mu.RUnlock()

	assert.Equal(t, constants.TaskStatusStalledUpload, state.State)

	// Verify state was persisted to database even for insignificant changes
	states, err := svc.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	assert.Equal(t, constants.TaskStatusStalledUpload, states[taskHash].State)
}

func TestTrackTasks_PersistsLastKnownFields(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	workerID := uuid.New()

	tasks := []*entities.Task{
		{
			Hash:     "fields-hash",
			Name:     "Ubuntu ISO",
			Category: "linux",
			Size:     4096,
			State:    constants.TaskStatusDownloading,
			Progress: 0.25,
		},
	}

	err = svc.TrackTasks(ctx, tasks, workerID, time.Now())
	require.NoError(t, err)

	states, err := svc.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	require.Contains(t, states, "fields-hash")
	assert.Equal(t, "Ubuntu ISO", states["fields-hash"].Name)
	assert.Equal(t, "linux", states["fields-hash"].Category)
	assert.Equal(t, int64(4096), states["fields-hash"].Size)
}

func TestTrackTasks_UpdatesNameOnRename(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	svc, err := NewService(db)
	require.NoError(t, err)
	ctx := context.Background()

	workerID := uuid.New()
	taskHash := "rename-hash"

	err = svc.TrackTasks(ctx, []*entities.Task{
		{
			Hash:     taskHash,
			Name:     "Old Name",
			State:    constants.TaskStatusDownloading,
			Progress: 0.5,
		},
	}, workerID, time.Now())
	require.NoError(t, err)

	// Same hash and telemetry, only the name changed.
	err = svc.TrackTasks(ctx, []*entities.Task{
		{
			Hash:     taskHash,
			Name:     "New Name",
			State:    constants.TaskStatusDownloading,
			Progress: 0.5,
		},
	}, workerID, time.Now())
	require.NoError(t, err)

	svc.mu.RLock()
	inMemory := svc.taskStates[workerID][taskHash].Name
	svc.mu.RUnlock()
	assert.Equal(t, "New Name", inMemory)

	states, err := svc.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", states[taskHash].Name)
}

func TestRestartUsesPersistedSnapshotAndEmitsOnlyDiff(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	ctx := context.Background()
	workerID := uuid.New()
	oldTimestamp := time.Now().UTC().Add(-30 * 24 * time.Hour)

	beforeRestart, err := NewService(db)
	require.NoError(t, err)
	require.NoError(t, beforeRestart.TrackTasks(ctx, []*entities.Task{
		{Hash: "unchanged", Name: "Unchanged", State: constants.TaskStatusDownloading, Progress: 0.4},
		{Hash: "changed", Name: "Changed", State: constants.TaskStatusDownloading, Progress: 0.5},
		{Hash: "removed", Name: "Removed", State: constants.TaskStatusUploading, Progress: 1.0},
	}, workerID, oldTimestamp))

	// Event retention may purge old history, but the comparison snapshot must
	// survive indefinitely so an outage or reboot cannot turn it into additions.
	beforeRestart.runCleanup(ctx)
	states, err := beforeRestart.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	require.Len(t, states, 3)

	afterRestart, err := NewService(db)
	require.NoError(t, err)
	eventCh := afterRestart.Subscribe(10)
	now := time.Now().UTC()

	require.NoError(t, afterRestart.TrackTasks(ctx, []*entities.Task{
		{Hash: "unchanged", Name: "Unchanged", State: constants.TaskStatusDownloading, Progress: 0.4},
		{Hash: "changed", Name: "Changed", State: constants.TaskStatusError, Progress: 0.5},
		{Hash: "added", Name: "Added", State: constants.TaskStatusDownloading, Progress: 0.1},
	}, workerID, now))
	require.NoError(t, afterRestart.DetectRemovedTasks(ctx, []*entities.Task{
		{Hash: "unchanged"},
		{Hash: "changed"},
		{Hash: "added"},
	}, workerID, now))

	received := make(map[string]*entities.Event)
	for len(received) < 3 {
		select {
		case event := <-eventCh:
			received[event.Type+":"+event.TaskHash] = event
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for diff events; received %#v", received)
		}
	}

	assert.Contains(t, received, constants.EventTypeTorrentStateChange+":changed")
	assert.Contains(t, received, constants.EventTypeTorrentAdded+":added")
	assert.Contains(t, received, constants.EventTypeTorrentRemoved+":removed")
	assert.NotContains(t, received, constants.EventTypeTorrentAdded+":unchanged")

	select {
	case event := <-eventCh:
		t.Fatalf("unexpected extra event after restart: %s for %s", event.Type, event.TaskHash)
	default:
	}

	states, err = afterRestart.repo.LoadTaskStates(ctx, workerID)
	require.NoError(t, err)
	assert.Contains(t, states, "unchanged")
	assert.Contains(t, states, "changed")
	assert.Contains(t, states, "added")
	assert.NotContains(t, states, "removed")
}

func TestDetectRemovedTasks_MetadataIncludesName(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
	}{
		{name: "with name", taskName: "Debian ISO"},
		{name: "with empty name", taskName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database.SetupTestDBWithMigrations(t)
			svc, err := NewService(db)
			require.NoError(t, err)
			ctx := context.Background()

			workerID := uuid.New()
			taskHash := "removed-hash"

			err = svc.TrackTasks(ctx, []*entities.Task{
				{
					Hash:     taskHash,
					Name:     tt.taskName,
					Category: "isos",
					Size:     1024,
					State:    constants.TaskStatusUploading,
					Progress: 1.0,
				},
			}, workerID, time.Now())
			require.NoError(t, err)

			// Task no longer present in poll -> removal event
			err = svc.DetectRemovedTasks(ctx, []*entities.Task{}, workerID, time.Now())
			require.NoError(t, err)

			events, _, err := svc.repo.ListEvents(ctx, &workerID, []string{constants.EventTypeTorrentRemoved}, "", 10, 0)
			require.NoError(t, err)
			require.Len(t, events, 1)

			ev := events[0]
			assert.Equal(t, taskHash, ev.TaskHash)
			assert.Equal(t, tt.taskName, ev.Metadata["name"])
			assert.Equal(t, "isos", ev.Metadata["category"])
			assert.Equal(t, taskHash, ev.Metadata["hash"])
			assert.Equal(t, 1.0, ev.Metadata["last_progress"])
		})
	}
}
