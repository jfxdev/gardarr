package transferreport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
)

type fakeWorkers struct {
	tasks  []*entities.Task
	errors map[string]string
	err    error
}

func (f *fakeWorkers) ListWorkersBasic() ([]*entities.Worker, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []*entities.Worker{{UUID: uuid.New()}}, nil
}

func (f *fakeWorkers) ListTasks(context.Context, []*entities.Worker) (*entities.TaskListResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &entities.TaskListResult{Tasks: f.tasks, Errors: f.errors}, nil
}

type fakeTimezone struct{ value string }

func (f fakeTimezone) GetTimezone(context.Context) (string, error) { return f.value, nil }

type fakeRecorder struct{ events []*entities.Event }

func (f *fakeRecorder) Record(_ context.Context, event *entities.Event) error {
	f.events = append(f.events, event)
	return nil
}

func testService(t *testing.T) (*Service, *fakeWorkers, *fakeRecorder) {
	t.Helper()
	db := database.SetupTestDB(t, &models.TransferReportSettings{}, &models.TransferSnapshotRun{}, &models.TransferSnapshot{}, &models.TransferReport{})
	workers := &fakeWorkers{}
	recorder := &fakeRecorder{}
	service := NewService(db, workers, fakeTimezone{value: "UTC"}, recorder)
	return service, workers, recorder
}

func TestRankSnapshotsUsesPositiveDeltasAndConsolidatesWorkers(t *testing.T) {
	workerA, workerB := uuid.New(), uuid.New()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	snapshots := []entities.TransferSnapshot{
		{WorkerID: workerA, Hash: "ABC", Name: "Alpha", Uploaded: 100, Downloaded: 50, CapturedAt: start.Add(-time.Hour)},
		{WorkerID: workerA, Hash: "ABC", Name: "Alpha", Uploaded: 170, Downloaded: 100, CapturedAt: start.Add(time.Hour)},
		{WorkerID: workerB, Hash: "abc", Name: "Alpha", Uploaded: 10, Downloaded: 20, CapturedAt: start.Add(-time.Hour)},
		{WorkerID: workerB, Hash: "abc", Name: "Alpha", Uploaded: 40, Downloaded: 80, CapturedAt: start.Add(2 * time.Hour)},
		// The first observation for Beta is only a baseline and must not rank.
		{WorkerID: workerA, Hash: "BETA", Name: "Beta", Uploaded: 999, Downloaded: 999, CapturedAt: start.Add(time.Hour)},
	}

	ranking := rankSnapshots(snapshots, start, start.Add(24*time.Hour), 10)
	if len(ranking.upload) != 1 || ranking.upload[0].Bytes != 100 || ranking.upload[0].Hash != "ABC" {
		t.Fatalf("unexpected upload ranking: %#v", ranking.upload)
	}
	if len(ranking.download) != 1 || ranking.download[0].Bytes != 110 {
		t.Fatalf("unexpected download ranking: %#v", ranking.download)
	}
}

func TestRankSnapshotsTreatsCounterResetAsPostResetTraffic(t *testing.T) {
	worker := uuid.New()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ranking := rankSnapshots([]entities.TransferSnapshot{
		{WorkerID: worker, Hash: "reset", Name: "Reset", Uploaded: 1000, Downloaded: 1000, CapturedAt: start.Add(-time.Hour)},
		{WorkerID: worker, Hash: "reset", Name: "Reset", Uploaded: 20, Downloaded: 50, CapturedAt: start.Add(time.Hour)},
	}, start, start.Add(24*time.Hour), 10)
	if ranking.upload[0].Bytes != 20 || ranking.download[0].Bytes != 50 {
		t.Fatalf("expected current counters after reset, got %#v", ranking)
	}
}

func TestExpectedPeriodUsesMondayWeek(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	settings := &entities.TransferReportSettings{DailyReportTime: "00:05", WeeklyReportDay: 1, WeeklyReportTime: "00:10"}
	now := time.Date(2026, 9, 7, 0, 11, 0, 0, loc) // Monday
	start, end, due := expectedPeriod(now, settings, loc, entities.TransferReportPeriodWeekly)
	if !due || start.Weekday() != time.Monday || end.Weekday() != time.Monday || end.Sub(start) != 7*24*time.Hour {
		t.Fatalf("unexpected weekly period: %v %v due=%v", start, end, due)
	}
}

func TestServiceCapturesBuildsAndPersistsDailyReport(t *testing.T) {
	service, workers, recorder := testService(t)
	workerID := uuid.New()
	baseline := time.Date(2026, 9, 1, 23, 0, 0, 0, time.UTC)
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}, Download: entities.TaskDownload{Amount: 200}}}}
	if err := service.capture(context.Background(), baseline, baseline); err != nil {
		t.Fatalf("baseline capture: %v", err)
	}
	workers.tasks[0].Network.Upload.Amount = 180
	workers.tasks[0].Network.Download.Amount = 260
	workers.errors = map[string]string{uuid.New().String(): "offline"}
	current := baseline.Add(2 * time.Hour)
	if err := service.capture(context.Background(), current, current); err != nil {
		t.Fatalf("movement capture: %v", err)
	}

	report, err := service.buildReport(context.Background(), entities.TransferReportPeriodDaily, baseline.Add(time.Hour), baseline.Add(25*time.Hour), "UTC", 10)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.Coverage != "partial" || len(report.Upload) != 1 || report.Upload[0].Bytes != 80 || report.Download[0].Bytes != 60 {
		t.Fatalf("unexpected report: %#v", report)
	}

	settings, err := service.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 3, 0, 6, 0, 0, time.UTC) }
	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err != nil {
		t.Fatalf("generate report: %v", err)
	}
	daily, weekly, err := service.GetLatest(context.Background())
	if err != nil || daily == nil || weekly != nil || len(recorder.events) != 1 {
		t.Fatalf("latest reports/events: daily=%#v weekly=%#v events=%d err=%v", daily, weekly, len(recorder.events), err)
	}
	if recorder.events[0].Type != "report.transfer.daily" || recorder.events[0].Metadata["coverage"] != "partial" {
		t.Fatalf("unexpected event: %#v", recorder.events[0])
	}

	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err != nil {
		t.Fatalf("idempotent generation: %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected no duplicate event, got %d", len(recorder.events))
	}
}

func TestSettingsAndRankingHelpersValidateInputs(t *testing.T) {
	service, _, _ := testService(t)
	valid := entities.TransferReportSettings{Enabled: true, SnapshotsPerDay: 6, DailyReportTime: "01:05", WeeklyReportDay: 0, WeeklyReportTime: "02:10", TopN: 2}
	if _, err := service.UpdateSettings(context.Background(), valid); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	for _, input := range []entities.TransferReportSettings{
		{SnapshotsPerDay: 5, DailyReportTime: "00:05", WeeklyReportTime: "00:10", TopN: 1},
		{SnapshotsPerDay: 4, DailyReportTime: "nope", WeeklyReportTime: "00:10", TopN: 1},
		{SnapshotsPerDay: 4, DailyReportTime: "00:05", WeeklyReportTime: "00:10", TopN: 51},
	} {
		if _, err := service.UpdateSettings(context.Background(), input); err == nil {
			t.Fatalf("expected invalid settings error for %#v", input)
		}
	}
	if _, ok := snapshotSlot(time.Date(2026, 9, 1, 6, 0, 0, 0, time.UTC), 4); !ok {
		t.Fatal("expected six-hour snapshot slot")
	}
	if _, ok := snapshotSlot(time.Date(2026, 9, 1, 7, 1, 0, 0, time.UTC), 4); ok {
		t.Fatal("unexpected non-aligned snapshot slot")
	}
	items := sortRankings([]entities.TransferRankItem{{Name: "Zulu", Hash: "z", Bytes: 10}, {Name: "alpha", Hash: "b", Bytes: 10}, {Name: "Alpha", Hash: "a", Bytes: 10}}, 2)
	if len(items) != 2 || items[0].Hash != "a" || items[1].Hash != "b" || items[0].Rank != 1 {
		t.Fatalf("unexpected sorted items: %#v", items)
	}
	workers := uniqueWorkerIDs([]map[string]string{{"b": "failed", "a": "failed"}, {"a": "again"}})
	if len(workers) != 2 || workers[0] != "a" || !validClock("23:59") || validClock("24:00") {
		t.Fatalf("unexpected helper result: %#v", workers)
	}
	if counterDelta(20, 10) != 10 || counterDelta(20, 25) != 5 {
		t.Fatal("counter deltas are incorrect")
	}
	if _, _, err := service.GetLatest(context.Background()); err != nil && !errors.Is(err, nil) {
		t.Fatalf("empty latest reports should not fail: %v", err)
	}
}

func TestReconcileCreatesBaselineAndScheduledSnapshot(t *testing.T) {
	service, workers, _ := testService(t)
	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "alpha", Name: "Alpha", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}, Download: entities.TaskDownload{Amount: 100}}}}
	service.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
	service.reconcile(context.Background())
	if count, err := service.repo.CountSnapshots(context.Background()); err != nil || count != 1 {
		t.Fatalf("expected immediate baseline, count=%d err=%v", count, err)
	}
	workers.tasks[0].Network.Upload.Amount = 150
	workers.tasks[0].Network.Download.Amount = 175
	service.now = func() time.Time { return time.Date(2026, 9, 2, 6, 0, 0, 0, time.UTC) }
	service.reconcile(context.Background())
	if count, err := service.repo.CountSnapshots(context.Background()); err != nil || count != 2 {
		t.Fatalf("expected aligned snapshot, count=%d err=%v", count, err)
	}
	settings, err := service.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	settings.Enabled = false
	if _, err := service.UpdateSettings(context.Background(), *settings); err != nil {
		t.Fatalf("disable settings: %v", err)
	}
	service.reconcile(context.Background())
}
