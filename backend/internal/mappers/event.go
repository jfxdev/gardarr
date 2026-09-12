package mappers

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/models"
)

// ToEventResponse converts an event entity to a response model
func ToEventResponse(event *entities.Event) *models.EventResponse {
	if event == nil {
		return nil
	}

	response := &models.EventResponse{
		UUID:      event.UUID.String(),
		Type:      event.Type,
		TaskHash:  event.TaskHash,
		OldValue:  event.OldValue,
		NewValue:  event.NewValue,
		Metadata:  event.Metadata,
		CreatedAt: event.CreatedAt,
	}
	if event.WorkerID != uuid.Nil {
		response.WorkerID = event.WorkerID.String()
	}
	return response
}

// ToEventEntity converts an event model to an entity
func ToEventEntity(model *models.Event) *entities.Event {
	if model == nil {
		return nil
	}

	var metadata map[string]interface{}

	if model.Metadata != "" {
		_ = json.Unmarshal([]byte(model.Metadata), &metadata)
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
	}
}
