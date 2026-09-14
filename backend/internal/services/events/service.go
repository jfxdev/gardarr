package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/repository/event"
	"github.com/jfxdev/gardarr/pkg/env"
)

const (
	// Log messages
	logMsgFailedToSaveTaskState   = "failed to save task state"
	logMsgFailedToDeleteTaskState = "failed to delete task state from database"
)

// Service handles event tracking and state change detection
type Service struct {
	repo             *event.Repository
	taskStates       map[uuid.UUID]map[string]*entities.TaskState // workerID -> taskHash -> state
	mu               sync.RWMutex
	retentionDays    int
	subscribers      []chan *entities.Event
	subscriberBuffer int
	droppedEvents    atomic.Int64
}

// NewService creates a new event service and loads existing state from database
// Returns error if state loading fails to ensure consistent initialization
func NewService(db *database.Database) (*Service, error) {
	s := &Service{
		repo:             event.NewRepository(db),
		taskStates:       make(map[uuid.UUID]map[string]*entities.TaskState),
		retentionDays:    env.Get("EVENT_RETENTION_DAYS").Default(7).ValueInt(),
		subscribers:      make([]chan *entities.Event, 0),
		subscriberBuffer: env.Get("EVENT_SUBSCRIBER_BUFFER").Default(256).ValueInt(),
	}

	// Load existing task states from database - fail fast if this fails
	if err := s.LoadStates(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load task states from database: %w", err)
	}

	return s, nil
}

// LoadStates loads all task states from the database into memory
func (s *Service) LoadStates(ctx context.Context) error {
	states, err := s.repo.LoadAllTaskStates(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.taskStates = states
	s.mu.Unlock()

	slog.Info("task states loaded from database",
		"workers", len(states),
	)

	return nil
}

// GetTaskStates returns a snapshot of all current task states in memory
func (s *Service) GetTaskStates() []*entities.TaskState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var states []*entities.TaskState
	for _, workerTasks := range s.taskStates {
		for _, state := range workerTasks {
			// Create a copy to prevent concurrent modification issues
			stateCopy := *state
			states = append(states, &stateCopy)
		}
	}
	return states
}

// Subscribe creates and returns a channel for real-time event emission.
// This allows multiple consumers (like integration services and websockets) to receive events as they occur.
// A bufferSize <= 0 uses the service default (EVENT_SUBSCRIBER_BUFFER, 256).
func (s *Service) Subscribe(bufferSize int) <-chan *entities.Event {
	if bufferSize <= 0 {
		bufferSize = s.subscriberBuffer
	}
	if bufferSize <= 0 {
		bufferSize = 256
	}
	ch := make(chan *entities.Event, bufferSize)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	return ch
}

// Record persists an application-level event and broadcasts it to real-time
// consumers. It is used for events that do not originate from torrent polling.
func (s *Service) Record(ctx context.Context, event *entities.Event) error {
	if event.UUID == uuid.Nil {
		event.UUID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return err
	}
	s.broadcastEvent(event)
	return nil
}

// RecordIfNew persists and broadcasts an event only when its UUID has not
// already been stored. It is safe for durable producers to retry.
func (s *Service) RecordIfNew(ctx context.Context, event *entities.Event) (bool, error) {
	if event.UUID == uuid.Nil {
		event.UUID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	created, err := s.repo.CreateEventIfAbsent(ctx, event)
	if err != nil || !created {
		return created, err
	}
	s.broadcastEvent(event)
	return true, nil
}

// broadcastEvent sends an event to all subscribers non-blocking
func (s *Service) broadcastEvent(event *entities.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full - skip emission to prevent blocking, but make the
			// drop observable instead of silently losing the event.
			totalDropped := s.droppedEvents.Add(1)
			slog.Warn("event subscriber channel full, dropping event",
				"event_id", event.UUID.String(),
				"event_type", event.Type,
				"total_dropped", totalDropped,
			)
		}
	}
}

// isErrorState checks if a state represents an error condition
func isErrorState(state string) bool {
	return state == constants.TaskStatusError || state == constants.TaskStatusMissingFiles
}

// isSignificantStateChange checks if a state change is significant enough to generate an event
// Some state changes are trivial (like STALLED_UPLOAD <-> UPLOADING) and happen frequently
func isSignificantStateChange(oldState, newState string) bool {
	// Define state groups that are considered similar
	uploadStates := map[string]bool{
		constants.TaskStatusUploading:     true,
		constants.TaskStatusStalledUpload: true,
	}

	downloadStates := map[string]bool{
		constants.TaskStatusDownloading:     true,
		constants.TaskStatusStalledDownload: true,
	}

	// If both states are in the same group, it's not significant
	if uploadStates[oldState] && uploadStates[newState] {
		return false
	}

	if downloadStates[oldState] && downloadStates[newState] {
		return false
	}

	// All other state changes are significant
	return true
}

// stateUpdate represents a pending state update to be persisted
type stateUpdate struct {
	hash      string
	name      string
	category  string
	size      int64
	state     string
	progress  float64
	timestamp time.Time
}

// completionCheck represents a pending completion event that needs database verification
type completionCheck struct {
	event *entities.Event
}

// TrackTasks processes current tasks and detects state changes
func (s *Service) TrackTasks(ctx context.Context, tasks []*entities.Task, workerID uuid.UUID, timestamp time.Time) error {
	if len(tasks) == 0 {
		return nil
	}

	// Channels for collecting updates and events
	eventsChan := make(chan *entities.Event, len(tasks)*2)
	updatesChan := make(chan stateUpdate, len(tasks))
	completionChecksChan := make(chan completionCheck, len(tasks))

	// First pass: update in-memory state and collect changes (minimal lock time)
	for _, task := range tasks {
		func(t *entities.Task) {
			s.mu.Lock()
			defer s.mu.Unlock()

			// Ensure worker map exists
			if s.taskStates[workerID] == nil {
				s.taskStates[workerID] = make(map[string]*entities.TaskState)
			}

			lastState, exists := s.taskStates[workerID][t.Hash]

			// Debug log for state comparison
			if exists {
				slog.Debug("task state comparison",
					"task_hash", t.Hash,
					"task_name", t.Name,
					"old_state", lastState.State,
					"new_state", t.State,
					"old_progress", lastState.Progress,
					"new_progress", t.Progress,
				)
			}

			// New task detected
			if !exists {
				s.taskStates[workerID][t.Hash] = &entities.TaskState{
					WorkerID:  workerID,
					Hash:      t.Hash,
					Name:      t.Name,
					Category:  t.Category,
					Size:      int64(t.Size),
					State:     t.State,
					Progress:  t.Progress,
					UpdatedAt: timestamp,
				}

				// Queue for persistence
				updatesChan <- stateUpdate{
					hash:      t.Hash,
					name:      t.Name,
					category:  t.Category,
					size:      int64(t.Size),
					state:     t.State,
					progress:  t.Progress,
					timestamp: timestamp,
				}

				// Create task added event
				eventsChan <- &entities.Event{
					UUID:      uuid.New(),
					WorkerID:  workerID,
					Type:      constants.EventTypeTorrentAdded,
					TaskHash:  t.Hash,
					NewValue:  t.State,
					Metadata:  s.buildBaseMetadata(t),
					CreatedAt: timestamp,
				}
				return
			}

			// State change detected
			if lastState.State != t.State {
				oldState := lastState.State
				oldProgress := lastState.Progress
				wasCompleted := lastState.Progress >= 1.0

				// Update state in memory
				lastState.State = t.State
				lastState.Progress = t.Progress
				lastState.Name = t.Name
				lastState.Category = t.Category
				lastState.Size = int64(t.Size)
				lastState.UpdatedAt = timestamp

				// Queue for persistence
				updatesChan <- stateUpdate{
					hash:      t.Hash,
					name:      t.Name,
					category:  t.Category,
					size:      int64(t.Size),
					state:     t.State,
					progress:  t.Progress,
					timestamp: timestamp,
				}

				// Check if this is a significant state change
				if !isSignificantStateChange(oldState, t.State) {
					slog.Debug("insignificant state change ignored",
						"task_name", t.Name,
						"task_hash", t.Hash,
						"old_state", oldState,
						"new_state", t.State,
					)
					return
				}

				slog.Info("state change detected",
					"task_name", t.Name,
					"task_hash", t.Hash,
					"old_state", oldState,
					"new_state", t.State,
					"old_progress", oldProgress,
					"new_progress", t.Progress,
				)

				// Create state change event
				eventsChan <- &entities.Event{
					UUID:      uuid.New(),
					WorkerID:  workerID,
					Type:      constants.EventTypeTorrentStateChange,
					TaskHash:  t.Hash,
					OldValue:  oldState,
					NewValue:  t.State,
					Metadata:  s.buildStateChangeMetadata(t, oldProgress),
					CreatedAt: timestamp,
				}

				// Check if task just completed - queue for verification to prevent duplicates on restart
				if t.Progress >= 1.0 && !wasCompleted && !isErrorState(t.State) {
					completionChecksChan <- s.buildCompletionCheck(workerID, t, timestamp)
				}
			} else if lastState.Progress != t.Progress {
				// Progress changed but state didn't
				oldProgress := lastState.Progress
				wasCompleted := oldProgress >= 1.0

				lastState.Progress = t.Progress
				lastState.Name = t.Name
				lastState.Category = t.Category
				lastState.Size = int64(t.Size)
				lastState.UpdatedAt = timestamp

				// Queue for persistence
				updatesChan <- stateUpdate{
					hash:      t.Hash,
					name:      t.Name,
					category:  t.Category,
					size:      int64(t.Size),
					state:     t.State,
					progress:  t.Progress,
					timestamp: timestamp,
				}

				// Check if task just reached 100% - queue for verification to prevent duplicates on restart
				if t.Progress >= 1.0 && !wasCompleted && !isErrorState(t.State) {
					completionChecksChan <- s.buildCompletionCheck(workerID, t, timestamp)
				}
			} else if lastState.Name != t.Name || lastState.Category != t.Category || lastState.Size != int64(t.Size) {
				// Keep the durable snapshot complete even when qBittorrent changes
				// metadata without changing state or progress. This information is
				// later used by removal events after a restart.
				lastState.Name = t.Name
				lastState.Category = t.Category
				lastState.Size = int64(t.Size)
				lastState.UpdatedAt = timestamp

				updatesChan <- stateUpdate{
					hash:      t.Hash,
					name:      t.Name,
					category:  t.Category,
					size:      int64(t.Size),
					state:     t.State,
					progress:  t.Progress,
					timestamp: timestamp,
				}
			}
		}(task)
	}

	close(updatesChan)
	close(eventsChan)
	close(completionChecksChan)

	// Second pass: persist all updates to database in a single batched upsert
	// instead of one round trip per task (the dominant DB cost per poll cycle
	// when many torrents change progress at once).
	updates := make([]*entities.TaskState, 0, len(updatesChan))
	for u := range updatesChan {
		updates = append(updates, &entities.TaskState{
			WorkerID:  workerID,
			Hash:      u.hash,
			Name:      u.name,
			Category:  u.category,
			Size:      u.size,
			State:     u.state,
			Progress:  u.progress,
			UpdatedAt: u.timestamp,
		})
	}

	if err := s.repo.SaveTaskStates(ctx, updates); err != nil {
		slog.Error(logMsgFailedToSaveTaskState,
			"error", err,
			"worker_id", workerID.String(),
			"count", len(updates),
		)
	}

	// Process events
	s.processEvents(ctx, eventsChan, workerID)

	// Process completion events with deduplication check
	// This prevents false "completed" events after app restart when the persisted
	// progress was less than 100% but the torrent was already completed
	s.processCompletionEvents(ctx, completionChecksChan, workerID)

	return nil
}

// processCompletionEvents handles completion events with deduplication
// It checks if a completion event already exists in the database before creating a new one
func (s *Service) processCompletionEvents(ctx context.Context, checksChan <-chan completionCheck, workerID uuid.UUID) {
	for check := range checksChan {
		// Verify if this torrent already has a completion event
		hasExisting, err := s.repo.HasCompletedEvent(ctx, workerID, check.event.TaskHash)
		if err != nil {
			slog.Error("failed to check for existing completion event",
				"error", err,
				"worker_id", workerID.String(),
				"task_hash", check.event.TaskHash,
			)
			continue
		}

		if hasExisting {
			slog.Debug("skipping duplicate completion event",
				"task_hash", check.event.TaskHash,
				"worker_id", workerID.String(),
			)
			continue
		}

		// No existing completion event, create new one
		if err := s.repo.CreateEvent(ctx, check.event); err != nil {
			slog.Error("failed to create completion event",
				"error", err,
				"worker_id", workerID.String(),
				"task_hash", check.event.TaskHash,
			)
			continue
		}

		slog.Info("torrent completed",
			"task_hash", check.event.TaskHash,
			"task_name", check.event.Metadata["name"],
		)

		// Emit to real-time subscribers
		s.broadcastEvent(check.event)
	}
}

func (s *Service) buildStateChangeMetadata(t *entities.Task, oldProgress float64) map[string]interface{} {
	m := s.buildBaseMetadata(t)
	m["old_progress"] = oldProgress
	m["new_progress"] = t.Progress
	return m
}

// buildBaseMetadata creates the common metadata map for a task
func (s *Service) buildBaseMetadata(t *entities.Task) map[string]interface{} {
	return map[string]interface{}{
		"name":      t.Name,
		"hash":      t.Hash,
		"state":     t.State,
		"category":  t.Category,
		"tags":      t.Tags,
		"directory": t.Path,
		"size":      t.Size,
		"progress":  t.Progress,
		"ratio":     t.Ratio,
	}
}

// buildCompletionCheck creates a completion check for a completed task
func (s *Service) buildCompletionCheck(workerID uuid.UUID, t *entities.Task, timestamp time.Time) completionCheck {
	return completionCheck{
		event: &entities.Event{
			UUID:      uuid.New(),
			WorkerID:  workerID,
			Type:      constants.EventTypeTorrentCompleted,
			TaskHash:  t.Hash,
			NewValue:  t.State,
			Metadata:  s.buildBaseMetadata(t),
			CreatedAt: timestamp,
		},
	}
}

// processEvents handles event persistence and real-time emission
func (s *Service) processEvents(ctx context.Context, eventsChan <-chan *entities.Event, workerID uuid.UUID) {
	for event := range eventsChan {
		if err := s.repo.CreateEvent(ctx, event); err != nil {
			slog.Error("failed to create event",
				"error", err,
				"worker_id", workerID.String(),
				"event_type", event.Type,
				"task_hash", event.TaskHash,
			)
			continue
		}

		// Emit to real-time subscribers
		s.broadcastEvent(event)
	}
}

// DetectRemovedTasks checks for tasks that are no longer present concurrently
func (s *Service) DetectRemovedTasks(ctx context.Context, currentTasks []*entities.Task, workerID uuid.UUID, timestamp time.Time) error {
	// Build map of current task hashes
	currentHashes := make(map[string]bool)
	for _, task := range currentTasks {
		currentHashes[task.Hash] = true
	}

	s.mu.Lock()
	// Collect hashes to check for this specific worker
	var hashesToCheck []string
	if workerTasks, exists := s.taskStates[workerID]; exists {
		for hash := range workerTasks {
			if !currentHashes[hash] {
				hashesToCheck = append(hashesToCheck, hash)
			}
		}
	}
	s.mu.Unlock()

	if len(hashesToCheck) == 0 {
		return nil
	}

	// Building one event per removed hash is cheap (a map copy + struct
	// literal) - a goroutine per hash isn't earning its overhead and, on a
	// worker outage/recovery or bulk delete, spawning thousands of them at
	// once was a needless goroutine/lock-contention spike. Do it inline
	// under a single lock acquisition instead.
	events := make([]*entities.Event, 0, len(hashesToCheck))
	var hashesToDelete []string

	s.mu.Lock()
	workerTasks, workerExists := s.taskStates[workerID]
	for _, hash := range hashesToCheck {
		if !workerExists {
			break
		}
		state, exists := workerTasks[hash]
		if !exists {
			continue
		}

		events = append(events, &entities.Event{
			UUID:     uuid.New(),
			WorkerID: workerID,
			Type:     constants.EventTypeTorrentRemoved,
			TaskHash: hash,
			OldValue: state.State,
			Metadata: map[string]interface{}{
				"name":          state.Name,
				"hash":          hash,
				"category":      state.Category,
				"size":          state.Size,
				"last_progress": state.Progress,
			},
			CreatedAt: timestamp,
		})

		delete(workerTasks, hash)
		hashesToDelete = append(hashesToDelete, hash)
	}
	s.mu.Unlock()

	// Process events
	eventsChan := make(chan *entities.Event, len(events))
	for _, e := range events {
		eventsChan <- e
	}
	close(eventsChan)
	s.processEvents(ctx, eventsChan, workerID)

	// Delete from database (no lock held)
	if err := s.repo.DeleteTaskStates(ctx, workerID, hashesToDelete); err != nil {
		slog.Error(logMsgFailedToDeleteTaskState,
			"error", err,
			"worker_id", workerID.String(),
			"hashes", len(hashesToDelete),
		)
	}

	return nil
}

// ListEvents retrieves events with optional filters
func (s *Service) ListEvents(ctx context.Context, workerID *uuid.UUID, eventTypes []string, search string, limit int, offset int) ([]*entities.Event, int64, error) {
	return s.repo.ListEvents(ctx, workerID, eventTypes, search, limit, offset)
}

// GetEventByUUID retrieves an event by its UUID
func (s *Service) GetEventByUUID(ctx context.Context, uuid uuid.UUID) (*entities.Event, error) {
	return s.repo.GetEventByUUID(ctx, uuid)
}

// StartCleanupJob starts a background job that enforces event retention.
// Task states are durable snapshots and are deliberately not aged out: they
// are removed only after a successful worker poll confirms that a torrent is
// gone. Expiring them by time would make every torrent look newly added after
// a long Gardarr or worker outage.
func (s *Service) StartCleanupJob(ctx context.Context) {
	go func() {
		interval := env.Get("EVENT_CLEANUP_INTERVAL").Default("24h").ValueDuration()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run cleanup immediately on start
		s.runCleanup(ctx)

		for {
			select {
			case <-ticker.C:
				s.runCleanup(ctx)
			case <-ctx.Done():
				slog.Info("event cleanup job stopped")
				return
			}
		}
	}()
}

func (s *Service) runCleanup(ctx context.Context) {
	if err := s.PurgeOldEvents(ctx); err != nil {
		slog.Error("failed to purge old events", "error", err)
	}
}

// PurgeOldEvents deletes events older than retention period
func (s *Service) PurgeOldEvents(ctx context.Context) error {
	if s.retentionDays <= 0 {
		return nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -s.retentionDays)
	return s.repo.DeleteOldEvents(ctx, cutoff)
}
