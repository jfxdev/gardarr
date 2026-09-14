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

type fakeRecorder struct {
	events []*entities.Event
	err    error
	seen   map[uuid.UUID]bool
}

func (f *fakeRecorder) RecordIfNew(_ context.Context, event *entities.Event) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.seen == nil {
		f.seen = make(map[uuid.UUID]bool)
	}
	if f.seen[event.UUID] {
		return false, nil
	}
	f.seen[event.UUID] = true
	f.events = append(f.events, event)
	return true, nil
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

func TestRankSnapshotsExcludesTheStartBoundary(t *testing.T) {
	worker := uuid.New()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ranking := rankSnapshots([]entities.TransferSnapshot{
		{WorkerID: worker, Hash: "boundary", Name: "Boundary", Uploaded: 10, CapturedAt: start.Add(-time.Hour)},
		{WorkerID: worker, Hash: "boundary", Name: "Boundary", Uploaded: 20, CapturedAt: start},
		{WorkerID: worker, Hash: "boundary", Name: "Boundary", Uploaded: 25, CapturedAt: start.Add(time.Hour)},
	}, start, start.Add(24*time.Hour), 10)
	if len(ranking.upload) != 1 || ranking.upload[0].Bytes != 5 {
		t.Fatalf("start-boundary traffic was included: %#v", ranking.upload)
	}
}

func TestDelayedCaptureUsesScheduledBoundaryForReports(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{
			name:  "daily",
			start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "weekly",
			start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, workers, _ := testService(t)
			workerID := uuid.New()
			workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}}}}

			baseline := tc.end.Add(-6 * time.Hour)
			if err := service.capture(context.Background(), baseline, baseline); err != nil {
				t.Fatalf("capture baseline: %v", err)
			}
			workers.tasks[0].Network.Upload.Amount = 150
			observedAt := tc.end.Add(15 * time.Minute)
			if err := service.capture(context.Background(), tc.end, observedAt); err != nil {
				t.Fatalf("capture delayed slot: %v", err)
			}

			snapshots, err := service.repo.ListSnapshotsUntil(context.Background(), tc.end)
			if err != nil {
				t.Fatalf("list scheduled snapshots: %v", err)
			}
			if len(snapshots) != 2 || !snapshots[1].CapturedAt.Equal(tc.end) {
				t.Fatalf("delayed snapshot was not assigned to its slot: %#v", snapshots)
			}
			ranking := rankSnapshots(snapshots, tc.start, tc.end, 10)
			if len(ranking.upload) != 1 || ranking.upload[0].Bytes != 50 {
				t.Fatalf("scheduled-boundary transfer was not included: %#v", ranking.upload)
			}
		})
	}
}

func TestDelayedCaptureCountsAsCoverageForItsScheduledPeriod(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{
			name:  "daily",
			start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "weekly",
			start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, workers, _ := testService(t)
			workerID := uuid.New()
			workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}}}}

			baseline := tc.end.Add(-6 * time.Hour)
			if err := service.capture(context.Background(), baseline, baseline); err != nil {
				t.Fatalf("capture baseline: %v", err)
			}
			workers.tasks[0].Network.Upload.Amount = 150
			if err := service.capture(context.Background(), tc.end, tc.end.Add(15*time.Minute)); err != nil {
				t.Fatalf("capture delayed slot: %v", err)
			}

			report, err := service.buildReport(context.Background(), tc.name, tc.start, tc.end, "UTC", 10, false)
			if err != nil {
				t.Fatalf("build report: %v", err)
			}
			if report.Coverage != "complete" || len(report.Upload) != 1 || report.Upload[0].Bytes != 50 {
				t.Fatalf("scheduled capture was marked unavailable: %#v", report)
			}
		})
	}
}

func TestGeneratePendingRefreshesUnavailableReportWhenSnapshotsArrive(t *testing.T) {
	service, workers, recorder := testService(t)
	settings, err := service.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	end := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -1)
	existing := entities.TransferReport{UUID: uuid.New(), PeriodType: entities.TransferReportPeriodDaily, PeriodStart: start, PeriodEnd: end, Timezone: "UTC", GeneratedAt: end.Add(5 * time.Minute), Coverage: "unavailable"}
	if _, err := service.repo.UpsertLatest(context.Background(), existing); err != nil {
		t.Fatalf("store unavailable report: %v", err)
	}

	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}}}}
	baseline := end.Add(-6 * time.Hour)
	if err := service.capture(context.Background(), baseline, baseline); err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	workers.tasks[0].Network.Upload.Amount = 150
	if err := service.capture(context.Background(), end, end.Add(15*time.Minute)); err != nil {
		t.Fatalf("capture delayed slot: %v", err)
	}

	service.now = func() time.Time { return end.Add(6 * time.Minute) }
	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err != nil {
		t.Fatalf("refresh report: %v", err)
	}
	daily, _, err := service.GetLatest(context.Background())
	if err != nil || daily == nil || daily.UUID != existing.UUID || daily.Coverage != "complete" || len(daily.Upload) != 1 || daily.Upload[0].Bytes != 50 {
		t.Fatalf("unavailable report was not refreshed: %#v err=%v", daily, err)
	}
	if len(recorder.events) != 1 || recorder.events[0].UUID != uuid.NewSHA1(existing.UUID, []byte("transfer-report-event")) {
		t.Fatalf("refresh should keep the original report event: %#v", recorder.events)
	}
}

func TestCaptureNowStoresAnImmediateSnapshot(t *testing.T) {
	service, workers, _ := testService(t)
	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "manual", Name: "Manual", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 42}}}}
	now := time.Date(2026, 9, 2, 15, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }

	if err := service.CaptureNow(context.Background()); err != nil {
		t.Fatalf("capture now: %v", err)
	}
	snapshots, err := service.repo.ListSnapshotsUntil(context.Background(), now)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 || !snapshots[0].CapturedAt.Equal(now) {
		t.Fatalf("manual snapshot not stored at the current time: %#v", snapshots)
	}
}

func TestGetCurrentBuildsRankingsFromPersistedSnapshots(t *testing.T) {
	service, workers, _ := testService(t)
	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}}}}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	if err := service.capture(context.Background(), now.Add(-11*time.Hour), now.Add(-11*time.Hour)); err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	workers.tasks[0].Network.Upload.Amount = 150
	if err := service.capture(context.Background(), now, now); err != nil {
		t.Fatalf("capture current value: %v", err)
	}
	service.now = func() time.Time { return now }

	daily, weekly, err := service.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	for _, report := range []*entities.TransferReport{daily, weekly} {
		if report == nil || report.Coverage != "complete" || len(report.Upload) != 1 || report.Upload[0].Bytes != 50 {
			t.Fatalf("unexpected current report: %#v", report)
		}
	}
}

func TestGetCurrentClearsPartialWhenWorkerRecovers(t *testing.T) {
	service, workers, _ := testService(t)
	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "movie", Name: "Movie", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}}}}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	if err := service.capture(context.Background(), now.Add(-6*time.Hour), now.Add(-6*time.Hour)); err != nil {
		t.Fatalf("capture baseline: %v", err)
	}
	// A run where the worker was unavailable earlier in the day.
	workers.tasks[0].Network.Upload.Amount = 130
	workers.errors = map[string]string{workerID.String(): "offline"}
	if err := service.capture(context.Background(), now.Add(-3*time.Hour), now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("capture outage: %v", err)
	}
	// The worker has since recovered: the latest run has no errors.
	workers.errors = nil
	workers.tasks[0].Network.Upload.Amount = 150
	if err := service.capture(context.Background(), now, now); err != nil {
		t.Fatalf("capture recovery: %v", err)
	}
	service.now = func() time.Time { return now }

	daily, _, err := service.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if daily.Coverage != "complete" || len(daily.UnavailableWorkers) != 0 {
		t.Fatalf("recovered worker should clear the partial flag: %#v", daily)
	}

	// Completed reports keep the whole-period union so history stays accurate.
	completed, err := service.buildReport(context.Background(), entities.TransferReportPeriodDaily, now.Add(-9*time.Hour), now.Add(time.Hour), "UTC", 10, false)
	if err != nil {
		t.Fatalf("build completed report: %v", err)
	}
	if completed.Coverage != "partial" || len(completed.UnavailableWorkers) != 1 {
		t.Fatalf("completed report should record the earlier outage: %#v", completed)
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

	report, err := service.buildReport(context.Background(), entities.TransferReportPeriodDaily, baseline.Add(time.Hour), baseline.Add(25*time.Hour), "UTC", 10, false)
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

func TestGeneratePendingRetriesFailedReportEventWithoutDuplicatingIt(t *testing.T) {
	service, _, recorder := testService(t)
	settings, err := service.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 3, 0, 6, 0, 0, time.UTC) }
	recorder.err = errors.New("temporary event storage failure")
	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err == nil {
		t.Fatal("expected event failure")
	}
	recorder.err = nil
	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err != nil {
		t.Fatalf("retry event: %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one retried event, got %d", len(recorder.events))
	}
	if err := service.generatePending(context.Background(), settings, "UTC", time.UTC, service.now(), entities.TransferReportPeriodDaily); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected no duplicate event, got %d", len(recorder.events))
	}
}

func TestBuildReportKeepsZeroTransferCoverageComplete(t *testing.T) {
	service, workers, _ := testService(t)
	workerID := uuid.New()
	workers.tasks = []*entities.Task{{WorkerID: workerID, Hash: "idle", Name: "Idle", Network: entities.TaskNetwork{Upload: entities.TaskUpload{Amount: 100}, Download: entities.TaskDownload{Amount: 100}}}}
	start := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if err := service.capture(context.Background(), start.Add(time.Hour), start.Add(time.Hour)); err != nil {
		t.Fatalf("capture: %v", err)
	}
	report, err := service.buildReport(context.Background(), entities.TransferReportPeriodDaily, start, start.Add(24*time.Hour), "UTC", 10, false)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if report.Coverage != "complete" || len(report.Upload) != 0 || len(report.Download) != 0 {
		t.Fatalf("expected complete idle report, got %#v", report)
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
	if slot, ok := snapshotSlot(time.Date(2026, 9, 1, 7, 1, 0, 0, time.UTC), 4); !ok || slot.Hour() != 6 {
		t.Fatalf("expected latest six-hour slot, got %v ok=%v", slot, ok)
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

func TestReconcileCreatesOneBaselineWithoutTorrents(t *testing.T) {
	service, _, _ := testService(t)
	service.now = func() time.Time { return time.Date(2026, 9, 2, 1, 15, 0, 0, time.UTC) }
	service.reconcile(context.Background())
	service.now = func() time.Time { return time.Date(2026, 9, 2, 1, 16, 0, 0, time.UTC) }
	service.reconcile(context.Background())
	if count, err := service.repo.CountRuns(context.Background()); err != nil || count != 1 {
		t.Fatalf("expected one baseline run without torrents, count=%d err=%v", count, err)
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
