package entities

import "time"

type WorkerOverview struct {
	Worker       *Worker
	CollectedAt  time.Time
	Stats        *TaskStats
	MetricsError string
}
