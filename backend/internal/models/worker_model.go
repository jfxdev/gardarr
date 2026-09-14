package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Worker struct {
	UUID                         uuid.UUID `gorm:"type:uuid;primaryKey;uniqueIndex"`
	Name                         string    `gorm:"size:100;uniqueIndex"`
	Type                         string    `gorm:"size:25"`
	Address                      string    `gorm:"size:600;not null"`
	EncryptedQBittorrentURL      string    `gorm:"size:600"`
	EncryptedQBittorrentUsername string    `gorm:"size:600"`
	EncryptedQBittorrentPassword string    `gorm:"size:600"`
	Icon                         string    `gorm:"size:100"`
	Color                        string    `gorm:"size:50"`
	// Nil means Gardarr has never been asked to manage this worker's baseline.
	DefaultDownloadSpeedLimit *int `gorm:"column:default_download_speed_limit"`
	DefaultUploadSpeedLimit   *int `gorm:"column:default_upload_speed_limit"`
	// These nullable fields persist the active schedule last applied by Gardarr.
	// They let the scheduler distinguish a restart from a real schedule change.
	LastAppliedBandwidthScheduleUUID *uuid.UUID `gorm:"type:uuid;column:last_applied_bandwidth_schedule_uuid"`
	LastAppliedDownloadSpeedLimit    *int       `gorm:"column:last_applied_download_speed_limit"`
	LastAppliedUploadSpeedLimit      *int       `gorm:"column:last_applied_upload_speed_limit"`
	CreatedAt                        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt                        time.Time  `gorm:"autoUpdateTime"`
}

// BandwidthSchedule stores a recurring local-time speed-limit window. DaysOfWeek
// is a Sunday-first bitmask (bit 0 = Sunday).
type BandwidthSchedule struct {
	UUID          uuid.UUID `gorm:"type:uuid;primaryKey;uniqueIndex"`
	WorkerUUID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Worker        Worker    `gorm:"foreignKey:WorkerUUID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Name          string    `gorm:"size:100;not null"`
	DaysOfWeek    uint8     `gorm:"not null"`
	StartMinute   int       `gorm:"not null"`
	EndMinute     int       `gorm:"not null"`
	DownloadLimit int       `gorm:"not null"`
	UploadLimit   int       `gorm:"not null"`
	Priority      int       `gorm:"not null;default:0;index"`
	Color         string    `gorm:"size:7;not null;default:'#64748b'"`
	Enabled       bool      `gorm:"not null;default:true"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (BandwidthSchedule) TableName() string { return "bandwidth_schedules" }

func (s *BandwidthSchedule) BeforeCreate(tx *gorm.DB) (err error) {
	if s.UUID == uuid.Nil {
		s.UUID = uuid.New()
	}
	return nil
}

func (a *Worker) TableName() string {
	return "workers"
}

func (a *Worker) BeforeCreate(tx *gorm.DB) (err error) {
	a.CreatedAt = time.Now()
	if a.UUID == uuid.Nil {
		a.UUID = uuid.New()
	}

	return
}

func (a *Worker) BeforeUpdate(tx *gorm.DB) (err error) {
	a.UpdatedAt = time.Now()
	return
}

type WorkerResponse struct {
	UUID                    string                           `json:"uuid"`
	Name                    string                           `json:"name"`
	Address                 string                           `json:"address"`
	Status                  string                           `json:"status"`
	Error                   string                           `json:"error,omitempty"`
	ErrorCode               string                           `json:"error_code,omitempty"`
	Permanent               bool                             `json:"permanent,omitempty"`
	Icon                    string                           `json:"icon,omitempty"`
	Color                   string                           `json:"color,omitempty"`
	Instance                InstanceResponse                 `json:"instance"`
	BandwidthScheduleStatus *BandwidthScheduleStatusResponse `json:"bandwidth_schedule_status,omitempty"`
}

type BandwidthScheduleStatusResponse struct {
	Active        bool   `json:"active"`
	Name          string `json:"name,omitempty"`
	DownloadLimit *int   `json:"download_limit,omitempty"`
	UploadLimit   *int   `json:"upload_limit,omitempty"`
}

type WorkerVersionResponse struct {
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	Date           string `json:"date"`
	QbittorrentURL string `json:"qbittorrent_url,omitempty"`
}
