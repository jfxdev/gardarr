package entities

import (
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID             string
	WorkerID       uuid.UUID
	CreatedAt      time.Time
	CompletedAt    *time.Time
	Name           string
	Hash           string
	InfoHashV1     string
	InfoHashV2     string
	State          string
	Category       string
	Path           string
	Size           int
	CompletedBytes *int64
	Active         bool
	Priority       int
	Ratio          float64
	Progress       float64
	Popularity     float64
	MagnetLink     TaskMagnetLink
	Tags           []string
	MagnetURI      string
	Pairs          TaskPairs
	NumSeeds       int
	SuperSeeding   bool
	Network        TaskNetwork
	Metadata       *TaskMetadata // Optional task metadata
	WasCreated     bool
}

type TaskMagnetLink struct {
	Hash        string   `json:"hash"`
	DisplayName string   `json:"display_name"`
	Trackers    []string `json:"trackers"`
	ExactLength string   `json:"exact_length"`
	ExactSource string   `json:"exact_source"`
}

type TaskPairs struct {
	SwarmSeeders  int
	SwarmLeechers int
	Seeders       int
	Leechers      int
}

type TaskNetwork struct {
	Download TaskDownload
	Upload   TaskUpload
}

type TaskDownload struct {
	Speed  int
	Amount int
}

type TaskUpload struct {
	Speed  int
	Amount int
}

type TaskCreationConfig struct {
	URI       string
	Category  string
	Tags      []string
	Directory string
}

type TaskFile struct {
	Index        int
	Name         string
	Size         int64
	Progress     float64
	Priority     int
	IsSeed       bool
	PieceRange   [2]int
	Availability float64
}

// TaskTracker is a single tracker entry reported by qBittorrent for a
// torrent. Status follows qBittorrent's own tracker status codes (0
// disabled, 1 not contacted, 2 working, 3 updating, 4 not working).
type TaskTracker struct {
	URL           string
	Status        int
	Tier          int
	NumPeers      int
	NumSeeds      int
	NumLeeches    int
	NumDownloaded int
	Message       string
}

// TaskPeer is a single peer entry reported by qBittorrent for a torrent.
// Country/CountryCode come from qBittorrent's own GeoIP resolution.
type TaskPeer struct {
	IP            string
	Port          int
	Client        string
	Flags         string
	FlagsDesc     string
	Connection    string
	Country       string
	CountryCode   string
	Downloaded    int64
	DownloadSpeed int
	Progress      float64
	Uploaded      int64
	UploadSpeed   int
	Relevance     float64
}

// TaskStatuses is the map reference of status in qBittorrent API
var TaskStatuses = map[string]string{
	"error":              "ERROR",
	"missingFiles":       "MISSING_FILES",
	"uploading":          "UPLOADING",
	"pausedUP":           "PAUSED_UPLOAD",
	"stoppedUP":          "STOPPED_UPLOAD",
	"queuedUP":           "QUEUED_UPLOAD",
	"stalledUP":          "STALLED_UPLOAD",
	"checkingUP":         "CHECKING_UPLOAD",
	"forcedUP":           "FORCED_UPLOAD",
	"allocating":         "ALLOCATING",
	"downloading":        "DOWNLOADING",
	"metaDL":             "METADATA_DOWNLOAD",
	"forcedMetaDL":       "FORCED_METADATA_DOWNLOAD",
	"pausedDL":           "PAUSED_DOWNLOAD",
	"stoppedDL":          "STOPPED_DOWNLOAD",
	"queuedDL":           "QUEUED_DOWNLOAD",
	"forcedDL":           "FORCED_DOWNLOAD",
	"stalledDL":          "STALLED_DOWNLOAD",
	"checkingDL":         "CHECKING_DOWNLOAD",
	"checkingResumeData": "CHECKING_RESUME_DATA",
	"moving":             "MOVING",
	"unknown":            "UNKNOWN",
}

var InactiveTaskStates = []string{
	"error",
	"paused",
	"completed",
	"missingFiles",
	"pausedUP",
	"stoppedUP",
	"queuedUP",
	"stalledUP",
	"checkingUP",
	"queuedDL",
	"stalledDL",
	"checkingDL",
	"checkingResumeData",
	"moving",
	"unknown",
}

type TaskStats struct {
	StoredBytes          *int64
	TotalDiskSize        int64
	CurrentUploadSpeed   int
	CurrentDownloadSpeed int
	AverageRatio         float64
	MedianRatio          float64
	HighestRatio         float64
	LowestRatio          float64
	ActiveTasksCount     int
	TotalTasksCount      int
	ActiveSeeds          int
	ActivePeers          int
	SwarmSeeders         int
	SwarmLeechers        int
	CategoryUsage        map[string]int
	TagsUsage            map[string]int
	WordCloud            map[string]int
}

type TaskLimits struct {
	DownloadLimit            int
	UploadLimit              int
	ShareLimit               float64 // Deprecated: use specific limits below
	RatioLimit               float64 // -2 = use global, -1 = no limit
	SeedingTimeLimit         int     // in minutes, -2 = use global, -1 = no limit
	InactiveSeedingTimeLimit int     // in minutes, -2 = use global, -1 = no limit
}

type TaskShareLimit struct {
	RatioLimit               float64
	SeedingTimeLimit         int
	InactiveSeedingTimeLimit int
}

type TaskListResult struct {
	Tasks  []*Task           `json:"tasks"`
	Errors map[string]string `json:"errors"` // Map of WorkerUUID -> ErrorMessage
}

// BulkTaskActionResult summarizes a bulk action: how many tasks were
// affected and, per failing worker, why its whole batch failed.
type BulkTaskActionResult struct {
	Succeeded int               `json:"succeeded"`
	Failed    map[string]string `json:"failed"` // Map of WorkerUUID -> ErrorMessage
}
