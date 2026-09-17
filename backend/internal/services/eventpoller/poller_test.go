package eventpoller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/services/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskListOutcome struct {
	result *entities.TaskListResult
	err    error
}

type fakeWorkerService struct {
	mu       sync.Mutex
	workers  []*entities.Worker
	listErr  error
	outcomes map[uuid.UUID][]taskListOutcome
}

func (f *fakeWorkerService) ListWorkersBasic() ([]*entities.Worker, error) {
	return f.workers, f.listErr
}

func (f *fakeWorkerService) ListTasks(_ context.Context, workers []*entities.Worker) (*entities.TaskListResult, error) {
	if len(workers) != 1 {
		return nil, fmt.Errorf("expected one worker, got %d", len(workers))
	}

	workerID := workers[0].UUID
	f.mu.Lock()
	defer f.mu.Unlock()

	workerOutcomes := f.outcomes[workerID]
	if len(workerOutcomes) == 0 {
		return nil, fmt.Errorf("no task-list outcome for worker %s", workerID)
	}
	outcome := workerOutcomes[0]
	f.outcomes[workerID] = workerOutcomes[1:]
	return outcome.result, outcome.err
}

func task(hash, name, state string, progress float64) *entities.Task {
	return &entities.Task{
		Hash:     hash,
		Name:     name,
		State:    state,
		Progress: progress,
	}
}

func taskResult(tasks ...*entities.Task) *entities.TaskListResult {
	return &entities.TaskListResult{Tasks: tasks, Errors: make(map[string]string)}
}

func receiveEvents(t *testing.T, ch <-chan *entities.Event, count int) map[string]*entities.Event {
	t.Helper()

	received := make(map[string]*entities.Event, count)
	for len(received) < count {
		select {
		case event := <-ch:
			received[event.Type+":"+event.TaskHash] = event
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for events; received %#v", received)
		}
	}
	return received
}

func requireNoEvent(t *testing.T, ch <-chan *entities.Event) {
	t.Helper()
	select {
	case event := <-ch:
		t.Fatalf("unexpected event %s for %s", event.Type, event.TaskHash)
	default:
	}
}

func TestOfflineWorkerRecoveryEmitsOnlyRealTorrentDiff(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	ctx := context.Background()
	workerID := uuid.New()
	worker := &entities.Worker{UUID: workerID}

	beforeOutage, err := events.NewService(db)
	require.NoError(t, err)
	require.NoError(t, beforeOutage.TrackTasks(ctx, []*entities.Task{
		task("unchanged", "Unchanged", constants.TaskStatusDownloading, 0.4),
		task("changed", "Changed", constants.TaskStatusDownloading, 0.5),
		task("removed", "Removed", constants.TaskStatusUploading, 1),
	}, workerID, time.Now().UTC()))

	offlineWorkers := &fakeWorkerService{outcomes: map[uuid.UUID][]taskListOutcome{
		workerID: {{result: &entities.TaskListResult{
			Tasks:  []*entities.Task{},
			Errors: map[string]string{workerID.String(): "connection refused"},
		}}},
	}}
	(&Service{workers: offlineWorkers, eventService: beforeOutage}).pollWorker(ctx, worker, time.Now().UTC())

	// Reloading the service proves the outage did not delete the durable
	// snapshots, rather than merely leaving an in-memory copy behind.
	afterOutage, err := events.NewService(db)
	require.NoError(t, err)
	require.Len(t, afterOutage.GetTaskStates(), 3)
	eventCh := afterOutage.Subscribe(10)

	recoveredWorkers := &fakeWorkerService{outcomes: map[uuid.UUID][]taskListOutcome{
		workerID: {{result: taskResult(
			task("unchanged", "Unchanged", constants.TaskStatusDownloading, 0.4),
			task("changed", "Changed", constants.TaskStatusError, 0.5),
			task("added", "Added", constants.TaskStatusDownloading, 0.1),
		)}},
	}}
	(&Service{workers: recoveredWorkers, eventService: afterOutage}).pollWorker(ctx, worker, time.Now().UTC())

	received := receiveEvents(t, eventCh, 3)
	assert.Contains(t, received, constants.EventTypeTorrentStateChange+":changed")
	assert.Contains(t, received, constants.EventTypeTorrentAdded+":added")
	assert.Contains(t, received, constants.EventTypeTorrentRemoved+":removed")
	assert.NotContains(t, received, constants.EventTypeTorrentAdded+":unchanged")
	assert.NotContains(t, received, constants.EventTypeTorrentRemoved+":unchanged")
	requireNoEvent(t, eventCh)
}

func TestWorkerErrorDoesNotAffectSuccessfulWorkerRemovalDetection(t *testing.T) {
	db := database.SetupTestDBWithMigrations(t)
	ctx := context.Background()
	offlineID := uuid.New()
	onlineID := uuid.New()
	offlineWorker := &entities.Worker{UUID: offlineID}
	onlineWorker := &entities.Worker{UUID: onlineID}

	eventSvc, err := events.NewService(db)
	require.NoError(t, err)
	require.NoError(t, eventSvc.TrackTasks(ctx, []*entities.Task{
		task("offline-task", "Offline task", constants.TaskStatusDownloading, 0.2),
	}, offlineID, time.Now().UTC()))
	require.NoError(t, eventSvc.TrackTasks(ctx, []*entities.Task{
		task("removed-online-task", "Removed online task", constants.TaskStatusDownloading, 0.3),
	}, onlineID, time.Now().UTC()))
	eventCh := eventSvc.Subscribe(10)

	workers := &fakeWorkerService{
		workers: []*entities.Worker{offlineWorker, onlineWorker},
		outcomes: map[uuid.UUID][]taskListOutcome{
			offlineID: {
				{result: &entities.TaskListResult{
					Tasks:  []*entities.Task{},
					Errors: map[string]string{offlineID.String(): "offline"},
				}},
				{result: taskResult(task("offline-task", "Offline task", constants.TaskStatusDownloading, 0.2))},
			},
			onlineID: {{result: taskResult()}},
		},
	}
	poller := &Service{workers: workers, eventService: eventSvc}
	poller.pollOnce(ctx)

	received := receiveEvents(t, eventCh, 1)
	removed := received[constants.EventTypeTorrentRemoved+":removed-online-task"]
	require.NotNil(t, removed)
	assert.Equal(t, onlineID, removed.WorkerID)

	states := eventSvc.GetTaskStates()
	require.Len(t, states, 1)
	assert.Equal(t, offlineID, states[0].WorkerID)
	assert.Equal(t, "offline-task", states[0].Hash)

	// The recovered worker returns the same torrent, so its preserved
	// snapshot must suppress both a false removal and a false addition.
	poller.pollWorker(ctx, offlineWorker, time.Now().UTC())
	requireNoEvent(t, eventCh)
}
