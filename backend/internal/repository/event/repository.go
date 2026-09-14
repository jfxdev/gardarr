package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *database.Database
}

const (
	// Query conditions for task state operations
	taskStateByWorkerAndHash = "worker_id = ? AND hash = ?"

	// Batch size for loading task states to prevent memory spikes
	taskStateLoadBatchSize = 1000
)

func NewRepository(db *database.Database) *Repository {
	return &Repository{db: db}
}

// CreateEvent creates a new event in the database
func (r *Repository) CreateEvent(ctx context.Context, event *entities.Event) error {
	model, err := eventModel(event)
	if err != nil {
		return err
	}
	return r.db.DB.WithContext(ctx).Create(model).Error
}

// CreateEventIfAbsent stores an event once by UUID and reports whether this
// call created it. Callers can safely retry after a transient database error.
func (r *Repository) CreateEventIfAbsent(ctx context.Context, event *entities.Event) (bool, error) {
	model, err := eventModel(event)
	if err != nil {
		return false, err
	}
	result := r.db.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(model)
	return result.RowsAffected > 0, result.Error
}

func eventModel(event *entities.Event) (*models.Event, error) {
	var metadataJSON string
	if len(event.Metadata) > 0 {
		data, err := json.Marshal(event.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal event metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	return &models.Event{
		UUID:      event.UUID,
		WorkerID:  event.WorkerID,
		Type:      string(event.Type),
		TaskHash:  event.TaskHash,
		OldValue:  event.OldValue,
		NewValue:  event.NewValue,
		Metadata:  metadataJSON,
		CreatedAt: event.CreatedAt,
	}, nil
}

// ListEvents retrieves events with optional filters. eventTypes, when
// non-empty, restricts results to those types (IN clause) - used both for an
// exact single-type filter and for a whole event group (e.g. all worker.*
// types for the History page's worker table). search, when non-empty,
// case-insensitively matches the task hash or the raw metadata JSON (which
// carries the torrent/worker/schedule name shown in the UI).
func (r *Repository) ListEvents(ctx context.Context, workerID *uuid.UUID, eventTypes []string, search string, limit int, offset int) ([]*entities.Event, int64, error) {
	var events []models.Event
	var total int64

	query := r.db.DB.WithContext(ctx).Model(&models.Event{})

	// Apply filters
	if workerID != nil {
		query = query.Where("worker_id = ?", workerID)
	}
	if len(eventTypes) > 0 {
		query = query.Where("type IN ?", eventTypes)
	}
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(task_hash) LIKE ? OR LOWER(metadata) LIKE ?", like, like)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get events with pagination
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		return nil, 0, err
	}

	// Convert to entities
	result := make([]*entities.Event, len(events))
	for i, e := range events {
		var metadata map[string]interface{}

		if e.Metadata != "" {
			if err := json.Unmarshal([]byte(e.Metadata), &metadata); err != nil {
				slog.Warn("failed to unmarshal event metadata", "error", err, "event_uuid", e.UUID.String())
			}
		}

		result[i] = &entities.Event{
			UUID:      e.UUID,
			WorkerID:  e.WorkerID,
			Type:      e.Type,
			TaskHash:  e.TaskHash,
			OldValue:  e.OldValue,
			NewValue:  e.NewValue,
			Metadata:  metadata,
			CreatedAt: e.CreatedAt,
		}
	}

	return result, total, nil
}

// GetEventByUUID retrieves an event by its UUID
func (r *Repository) GetEventByUUID(ctx context.Context, uuid uuid.UUID) (*entities.Event, error) {
	var model models.Event
	if err := r.db.DB.WithContext(ctx).Where("uuid = ?", uuid).First(&model).Error; err != nil {
		return nil, err
	}

	var metadata map[string]interface{}

	if model.Metadata != "" {
		if err := json.Unmarshal([]byte(model.Metadata), &metadata); err != nil {
			slog.Warn("failed to unmarshal event metadata", "error", err, "event_uuid", model.UUID.String())
		}
	}

	return &entities.Event{
		UUID:      model.UUID,
		WorkerID:  model.WorkerID,
		Type:      model.Type,
		TaskHash:  model.TaskHash,
		OldValue:  model.OldValue,
		NewValue:  model.NewValue,
		Metadata:  metadata,
		CreatedAt: model.CreatedAt,
	}, nil
}

// deleteOldEventsBatchSize bounds each purge round trip so the first cleanup
// on a long-lived install doesn't hold the SQLite writer lock for one giant
// DELETE covering months of accumulated events.
const deleteOldEventsBatchSize = 10000

// DeleteOldEvents deletes events older than the specified date in batches
func (r *Repository) DeleteOldEvents(ctx context.Context, olderThan time.Time) error {
	for {
		result := r.db.DB.WithContext(ctx).
			Where("uuid IN (?)", r.db.DB.
				Model(&models.Event{}).
				Select("uuid").
				Where("created_at < ?", olderThan).
				Limit(deleteOldEventsBatchSize),
			).
			Delete(&models.Event{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected < deleteOldEventsBatchSize {
			return nil
		}
	}
}

// SaveTaskState saves or updates a task state in the database atomically
// Uses UPSERT to avoid TOCTOU race conditions
func (r *Repository) SaveTaskState(ctx context.Context, workerID uuid.UUID, hash string, state string, progress float64, updatedAt time.Time) error {
	taskState := &models.TaskState{
		WorkerID:  workerID,
		Hash:      hash,
		State:     state,
		Progress:  progress,
		UpdatedAt: updatedAt,
	}

	// Atomic upsert: insert or update on conflict
	return r.db.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "worker_id"},
				{Name: "hash"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"state", "progress", "updated_at"}),
		}).
		Create(taskState).Error
}

// SaveTaskStates upserts a batch of task states in a single round trip.
// Used instead of N individual SaveTaskState calls when a poll cycle touches
// many tasks at once, since that was the dominant DB cost per poll.
func (r *Repository) SaveTaskStates(ctx context.Context, states []*entities.TaskState) error {
	if len(states) == 0 {
		return nil
	}

	rows := make([]*models.TaskState, 0, len(states))
	for _, s := range states {
		rows = append(rows, &models.TaskState{
			WorkerID:  s.WorkerID,
			Hash:      s.Hash,
			Name:      s.Name,
			Category:  s.Category,
			Size:      s.Size,
			State:     s.State,
			Progress:  s.Progress,
			UpdatedAt: s.UpdatedAt,
		})
	}

	// CreateInBatches keeps each INSERT within SQLite's default bind-variable
	// limit and stops one oversized/bad batch from failing the entire poll
	// cycle's persistence.
	return r.db.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "worker_id"},
				{Name: "hash"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"name", "category", "size", "state", "progress", "updated_at"}),
		}).
		CreateInBatches(&rows, 100).Error
}

// LoadTaskStates loads all task states for a specific worker from the database
func (r *Repository) LoadTaskStates(ctx context.Context, workerID uuid.UUID) (map[string]*entities.TaskState, error) {
	var states []models.TaskState
	if err := r.db.DB.WithContext(ctx).
		Where("worker_id = ?", workerID).
		Find(&states).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*entities.TaskState, len(states))
	for _, s := range states {
		result[s.Hash] = &entities.TaskState{
			WorkerID:  s.WorkerID,
			Hash:      s.Hash,
			Name:      s.Name,
			Category:  s.Category,
			Size:      s.Size,
			State:     s.State,
			Progress:  s.Progress,
			UpdatedAt: s.UpdatedAt,
		}
	}

	return result, nil
}

// LoadAllTaskStates loads all task states from the database grouped by worker
// Uses batching to prevent memory spikes during startup with large datasets
func (r *Repository) LoadAllTaskStates(ctx context.Context) (map[uuid.UUID]map[string]*entities.TaskState, error) {
	result := make(map[uuid.UUID]map[string]*entities.TaskState)
	offset := 0

	for {
		var batch []models.TaskState
		err := r.db.DB.WithContext(ctx).
			Order("worker_id, hash").
			Limit(taskStateLoadBatchSize).
			Offset(offset).
			Find(&batch).Error

		if err != nil {
			return nil, err
		}

		// No more records
		if len(batch) == 0 {
			break
		}

		// Process batch
		for _, s := range batch {
			if result[s.WorkerID] == nil {
				result[s.WorkerID] = make(map[string]*entities.TaskState)
			}
			result[s.WorkerID][s.Hash] = &entities.TaskState{
				WorkerID:  s.WorkerID,
				Hash:      s.Hash,
				Name:      s.Name,
				Category:  s.Category,
				Size:      s.Size,
				State:     s.State,
				Progress:  s.Progress,
				UpdatedAt: s.UpdatedAt,
			}
		}

		// Move to next batch
		offset += taskStateLoadBatchSize

		// If we got fewer records than batch size, we're done
		if len(batch) < taskStateLoadBatchSize {
			break
		}

		// Log progress for large datasets
		if offset%5000 == 0 {
			slog.Info("loading task states in progress",
				"loaded", offset,
				"workers", len(result),
			)
		}
	}

	return result, nil
}

// DeleteTaskState removes a task state from the database
func (r *Repository) DeleteTaskState(ctx context.Context, workerID uuid.UUID, hash string) error {
	return r.db.DB.WithContext(ctx).
		Where(taskStateByWorkerAndHash, workerID, hash).
		Delete(&models.TaskState{}).Error
}

// DeleteTaskStates removes multiple task states for a worker in a single
// round trip, instead of one DELETE per hash.
func (r *Repository) DeleteTaskStates(ctx context.Context, workerID uuid.UUID, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	return r.db.DB.WithContext(ctx).
		Where("worker_id = ? AND hash IN ?", workerID, hashes).
		Delete(&models.TaskState{}).Error
}

// HasCompletedEvent checks if a torrent.completed event already exists for the given task hash
func (r *Repository) HasCompletedEvent(ctx context.Context, workerID uuid.UUID, taskHash string) (bool, error) {
	result := r.db.DB.WithContext(ctx).
		Model(&models.Event{}).
		Select("1").
		Where("worker_id = ? AND task_hash = ? AND type = ?", workerID, taskHash, constants.EventTypeTorrentCompleted).
		Limit(1).
		Find(&models.Event{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
