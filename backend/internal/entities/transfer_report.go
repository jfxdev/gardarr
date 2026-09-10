package entities

import (
	"time"

	"github.com/google/uuid"
)

const (
	TransferReportPeriodDaily  = "daily"
	TransferReportPeriodWeekly = "weekly"
)

// TransferReportSettings controls the local-time scheduler used to collect
// qBittorrent's cumulative transfer counters and publish rankings.
type TransferReportSettings struct {
	Enabled          bool
	SnapshotsPerDay  int
	DailyReportTime  string
	WeeklyReportDay  int
	WeeklyReportTime string
	TopN             int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TransferSnapshotRun struct {
	UUID         uuid.UUID
	ScheduledAt  time.Time
	CapturedAt   time.Time
	WorkerErrors map[string]string
	CreatedAt    time.Time
}

type TransferSnapshot struct {
	UUID       uuid.UUID
	RunID      uuid.UUID
	WorkerID   uuid.UUID
	Hash       string
	Name       string
	Uploaded   int64
	Downloaded int64
	CapturedAt time.Time
}

type TransferRankItem struct {
	Rank  int    `json:"rank"`
	Name  string `json:"name"`
	Hash  string `json:"hash"`
	Bytes int64  `json:"bytes"`
}

type TransferReport struct {
	UUID               uuid.UUID
	PeriodType         string
	PeriodStart        time.Time
	PeriodEnd          time.Time
	Timezone           string
	GeneratedAt        time.Time
	Coverage           string
	UnavailableWorkers []string
	Upload             []TransferRankItem
	Download           []TransferRankItem
}
