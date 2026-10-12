package workermanager

import (
	"context"
	"time"

	"github.com/jfxdev/gardarr/internal/entities"
)

// Overview reuses confirmed health and lists torrents once. A metrics failure
// does not change the worker's health or turn missing data into a zero.
func (s *Service) Overview(ctx context.Context, id string) (*entities.WorkerOverview, error) {
	worker, err := s.GetWorker(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &entities.WorkerOverview{Worker: worker, CollectedAt: time.Now().UTC()}
	if worker.Status != entities.WorkerStatusActive {
		result.MetricsError = worker.Error
		return result, nil
	}
	stats, err := s.repository.GetWorkerTasksStats(worker)
	if err != nil {
		result.MetricsError = err.Error()
	} else {
		result.Stats = stats
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result.CollectedAt = time.Now().UTC()
	return result, nil
}
