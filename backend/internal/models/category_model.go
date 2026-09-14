package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StringArray is a custom type to handle []string in GORM
type StringArray []string

// Value implements the driver.Valuer interface for database storage
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan implements the sql.Scanner interface for database retrieval
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return errors.New("failed to scan StringArray")
	}
}

type Category struct {
	ID                 string      `gorm:"type:varchar(100);primaryKey"`
	Name               string      `gorm:"size:100;not null;uniqueIndex"`
	DefaultTags        StringArray `gorm:"type:text"`
	DefaultDirectories StringArray `gorm:"column:default_directories;type:text"`
	MetadataSource     string      `gorm:"size:50;not null;default:'none'"`
	ReleaseType        string      `gorm:"size:20;not null;default:'none'"`
	Color              string      `gorm:"size:50"`
	Icon               string      `gorm:"size:100"`
	CreatedAt          time.Time   `gorm:"autoCreateTime"`
	UpdatedAt          time.Time   `gorm:"autoUpdateTime"`
}

func (c *Category) BeforeCreate(tx *gorm.DB) (err error) {
	c.CreatedAt = time.Now()
	if c.ID == "" {
		c.ID = uuid.New().String()
	}

	return
}

func (c *Category) BeforeUpdate(tx *gorm.DB) (err error) {
	c.UpdatedAt = time.Now()
	return
}

// CategoryResponse represents the response body for category operations
type CategoryResponse struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	DefaultTags        StringArray `json:"default_tags"`
	DefaultDirectories StringArray `json:"default_directories"`
	MetadataSource     string      `json:"metadata_source"`
	ReleaseType        string      `json:"release_type"`
	Color              string      `json:"color,omitempty"`
	Icon               string      `json:"icon,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}
