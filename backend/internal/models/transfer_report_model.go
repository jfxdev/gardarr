package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TransferReportSettings struct {
	ID               string    `gorm:"type:varchar(32);primaryKey"`
	Enabled          bool      `gorm:"not null;default:true"`
	SnapshotsPerDay  int       `gorm:"not null;default:4"`
	DailyReportTime  string    `gorm:"size:5;not null;default:'00:05'"`
	WeeklyReportDay  int       `gorm:"not null;default:1"`
	WeeklyReportTime string    `gorm:"size:5;not null;default:'00:10'"`
	TopN             int       `gorm:"not null;default:10"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`
}

func (TransferReportSettings) TableName() string { return "transfer_report_settings" }

type TransferSnapshotRun struct {
	UUID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ScheduledAt  time.Time `gorm:"not null;uniqueIndex"`
	CapturedAt   time.Time `gorm:"not null;index"`
	WorkerErrors string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (TransferSnapshotRun) TableName() string { return "transfer_snapshot_runs" }

func (r *TransferSnapshotRun) BeforeCreate(_ *gorm.DB) error {
	if r.UUID == uuid.Nil {
		r.UUID = uuid.New()
	}
	return nil
}

type TransferSnapshot struct {
	UUID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	RunID      uuid.UUID `gorm:"type:uuid;not null;index"`
	WorkerID   uuid.UUID `gorm:"type:uuid;not null;index:idx_transfer_snapshot_worker_hash_time,priority:1"`
	Hash       string    `gorm:"size:255;not null;index:idx_transfer_snapshot_worker_hash_time,priority:2"`
	Name       string    `gorm:"size:512;not null;default:''"`
	Uploaded   int64     `gorm:"not null"`
	Downloaded int64     `gorm:"not null"`
	CapturedAt time.Time `gorm:"not null;index:idx_transfer_snapshot_worker_hash_time,priority:3;index"`
}

func (TransferSnapshot) TableName() string { return "transfer_snapshots" }

func (s *TransferSnapshot) BeforeCreate(_ *gorm.DB) error {
	if s.UUID == uuid.Nil {
		s.UUID = uuid.New()
	}
	return nil
}

type TransferReport struct {
	UUID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	PeriodType         string    `gorm:"size:16;not null;uniqueIndex"`
	PeriodStart        time.Time `gorm:"not null"`
	PeriodEnd          time.Time `gorm:"not null"`
	Timezone           string    `gorm:"size:100;not null"`
	GeneratedAt        time.Time `gorm:"not null"`
	Coverage           string    `gorm:"size:16;not null"`
	UnavailableWorkers string    `gorm:"type:text"`
	Upload             string    `gorm:"type:text"`
	Download           string    `gorm:"type:text"`
}

func (TransferReport) TableName() string { return "transfer_reports" }

func (r *TransferReport) BeforeCreate(_ *gorm.DB) error {
	if r.UUID == uuid.Nil {
		r.UUID = uuid.New()
	}
	return nil
}
