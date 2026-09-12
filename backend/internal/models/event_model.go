package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Event represents a system event in the database
type Event struct {
	UUID      uuid.UUID `gorm:"type:uuid;primaryKey;uniqueIndex"`
	WorkerID  uuid.UUID `gorm:"type:uuid;column:worker_id;index;not null;index:idx_events_worker_created,priority:1;index:idx_events_worker_hash_type,priority:1"`
	Type      string    `gorm:"size:100;index;not null;index:idx_events_worker_hash_type,priority:3"`
	TaskHash  string    `gorm:"size:255;index;index:idx_events_worker_hash_type,priority:2"`
	OldValue  string    `gorm:"size:255"`
	NewValue  string    `gorm:"size:255"`
	Metadata  string    `gorm:"type:text"` // JSON string
	CreatedAt time.Time `gorm:"index;not null;index:idx_events_worker_created,priority:2"`
}

func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
	if e.UUID == uuid.Nil {
		e.UUID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	return
}

// EventResponse represents the response body for event operations
type EventResponse struct {
	UUID      string                 `json:"uuid"`
	WorkerID  string                 `json:"worker_id,omitempty"`
	Type      string                 `json:"type"`
	TaskHash  string                 `json:"task_hash,omitempty"`
	OldValue  string                 `json:"old_value,omitempty"`
	NewValue  string                 `json:"new_value,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Contains all task details
	CreatedAt time.Time              `json:"created_at"`
}

// ListEventsResponse represents the response body for listing events
type ListEventsResponse struct {
	Events []*EventResponse `json:"events"`
	Total  int              `json:"total"`
}
