package models

import "time"

type WorkerOverviewMetrics struct {
	StoredBytes    *int64 `json:"stored_bytes"`
	TotalTorrents  *int   `json:"total_torrents"`
	ActiveTorrents *int   `json:"active_torrents"`
	DownloadSpeed  *int   `json:"download_speed"`
	UploadSpeed    *int   `json:"upload_speed"`
}

type WorkerOverviewResponse struct {
	Worker       *WorkerResponse       `json:"worker"`
	CollectedAt  time.Time             `json:"collected_at"`
	Metrics      WorkerOverviewMetrics `json:"metrics"`
	MetricsError string                `json:"metrics_error,omitempty"`
}
