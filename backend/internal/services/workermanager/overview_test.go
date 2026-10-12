package workermanager

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	workerrepository "github.com/jfxdev/gardarr/internal/repository/worker"
	"github.com/jfxdev/gardarr/internal/services/workerhealth"
)

type overviewRepo struct {
	workerrepository.RepositoryInterface
	worker *entities.Worker
	stats  *entities.TaskStats
	err    error
	calls  int
}

func (r *overviewRepo) GetWorkerByUUID(uuid.UUID) (*entities.Worker, error) {
	copy := *r.worker
	return &copy, nil
}
func (r *overviewRepo) GetWorkerTasksStats(*entities.Worker) (*entities.TaskStats, error) {
	r.calls++
	return r.stats, r.err
}

func TestOverviewHealthMetricsAndRecovery(t *testing.T) {
	id := uuid.New()
	health := workerhealth.NewService(noopProber{}, nil)
	repo := &overviewRepo{worker: &entities.Worker{UUID: id}}
	svc := &Service{repository: repo, health: health}
	health.Seed(id, nil, errors.New("offline"))
	result, err := svc.Overview(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 0 || result.Stats != nil || result.Worker.Status != entities.WorkerStatusErrored {
		t.Fatal("offline worker was queried or represented as zero")
	}

	health.Seed(id, &entities.Instance{Server: entities.InstanceServer{FreeSpaceOnDisk: 123}}, nil)
	repo.err = errors.New("torrent listing failed")
	result, err = svc.Overview(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || result.Stats != nil || result.MetricsError == "" || result.Worker.Status != entities.WorkerStatusActive {
		t.Fatal("metrics failure changed health or lost missingness")
	}

	stored := int64(6000000000)
	repo.err = nil
	repo.stats = &entities.TaskStats{StoredBytes: &stored, TotalTasksCount: 3, ActiveTasksCount: 2, CurrentDownloadSpeed: 100, CurrentUploadSpeed: 200}
	result, err = svc.Overview(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 2 || result.Stats != repo.stats || result.MetricsError != "" || result.CollectedAt.IsZero() {
		t.Fatal("recovery did not publish fresh metrics from a single call")
	}
	if result.Worker.Instance.Server.FreeSpaceOnDisk != 123 {
		t.Fatal("cached instance not reused")
	}
}

func TestOverviewCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Service{}).Overview(ctx, uuid.NewString()); !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
}
