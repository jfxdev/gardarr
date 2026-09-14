package bandwidthscheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeWorkers struct {
	mu          sync.Mutex
	preferences *entities.InstancePreferences
	err         error
	applied     []Limits
	list        []*entities.Worker
}

func (f *fakeWorkers) ListWorkersBasic() ([]*entities.Worker, error) { return f.list, f.err }
func (f *fakeWorkers) GetPreferences(context.Context, *entities.Worker) (*entities.InstancePreferences, error) {
	return f.preferences, f.err
}
func (f *fakeWorkers) SetWorkerGlobalSpeedLimits(_ context.Context, _ *entities.Worker, body schemas.InstanceSetSpeedLimitSchema) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.applied = append(f.applied, Limits{DownloadLimit: *body.DownloadLimit, UploadLimit: *body.UploadLimit})
	return nil
}

type fakeSettings struct {
	timezone string
	entered  chan<- struct{}
	err      error
}

func (f fakeSettings) GetTimezone(context.Context) (string, error) {
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
	}
	return f.timezone, f.err
}

type fakeEvents struct {
	mu     sync.Mutex
	events []*entities.Event
	err    error
}

func (f *fakeEvents) Record(_ context.Context, event *entities.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
	return f.err
}

func setupService(t *testing.T) (*Service, *entities.Worker, *fakeWorkers, *fakeEvents) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := gdb.AutoMigrate(&models.Worker{}, &models.BandwidthSchedule{}); err != nil {
		t.Fatal(err)
	}
	worker := &entities.Worker{UUID: uuid.New(), Name: "worker", Address: "http://worker"}
	defaultDownload, defaultUpload := 1000, 2000
	if err := gdb.Create(&models.Worker{UUID: worker.UUID, Name: worker.Name, Address: worker.Address, DefaultDownloadSpeedLimit: &defaultDownload, DefaultUploadSpeedLimit: &defaultUpload}).Error; err != nil {
		t.Fatal(err)
	}
	workers := &fakeWorkers{preferences: &entities.InstancePreferences{GlobalRateLimits: entities.InstancePreferencesGlobalRateLimits{DownloadSpeedLimit: 1000, UploadSpeedLimit: 2000}}}
	events := &fakeEvents{}
	service := &Service{db: &database.Database{DB: gdb}, workers: workers, settings: fakeSettings{timezone: "UTC"}, events: events, lastApplied: make(map[uuid.UUID]applied), locks: make(map[uuid.UUID]*sync.Mutex)}
	return service, worker, workers, events
}

func TestResolve(t *testing.T) {
	weekday := Schedule{Name: "weekday", DaysOfWeek: []int{1}, StartTime: "08:00", EndTime: "18:00", DownloadLimit: 100, UploadLimit: 10, Priority: 1, Enabled: true}
	overnight := Schedule{Name: "overnight", DaysOfWeek: []int{1}, StartTime: "22:00", EndTime: "06:00", DownloadLimit: 50, UploadLimit: 5, Priority: 2, Enabled: true}
	allDay := Schedule{Name: "all-day", DaysOfWeek: []int{2}, StartTime: "00:00", EndTime: "00:00", DownloadLimit: 20, UploadLimit: 2, Priority: 1, Enabled: true}
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"start included", time.Date(2024, 1, 8, 8, 0, 0, 0, time.UTC), "weekday"},
		{"end excluded", time.Date(2024, 1, 8, 18, 0, 0, 0, time.UTC), ""},
		{"overnight start", time.Date(2024, 1, 8, 22, 0, 0, 0, time.UTC), "overnight"},
		{"overnight previous day", time.Date(2024, 1, 9, 5, 59, 0, 0, time.UTC), "overnight"},
		{"overnight end excluded", time.Date(2024, 1, 9, 6, 0, 0, 0, time.UTC), "all-day"},
		{"same time full day", time.Date(2024, 1, 9, 12, 0, 0, 0, time.UTC), "all-day"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve([]Schedule{weekday, overnight, allDay}, tt.now)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("expected no match, got %s", got.Name)
				}
				return
			}
			if got == nil || got.Name != tt.want {
				t.Fatalf("expected %s, got %#v", tt.want, got)
			}
		})
	}
}

func TestResolveDSTUsesWallClock(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// 01:30 occurs twice on the fall-back transition; both occurrences use the
	// same local minute and must resolve the same schedule.
	schedule := Schedule{Name: "night", DaysOfWeek: []int{0}, StartTime: "01:00", EndTime: "02:00", Enabled: true}
	for _, now := range []time.Time{time.Date(2024, 11, 3, 1, 30, 0, 0, loc), time.Date(2024, 11, 3, 1, 30, 0, 0, loc).Add(time.Hour)} {
		if got := Resolve([]Schedule{schedule}, now); got == nil {
			t.Fatalf("expected schedule for %s", now)
		}
	}
}

func TestScheduleCRUDCapturesBaselineAndPreservesColor(t *testing.T) {
	service, worker, workers, _ := setupService(t)
	enabled := false
	reconciled := make(chan struct{}, 1)
	service.settings = fakeSettings{timezone: "UTC", entered: reconciled}
	input := Input{Name: "weekday", DaysOfWeek: []int{1, 2}, StartTime: "08:00", EndTime: "18:00", DownloadLimit: 300, UploadLimit: 30, Enabled: &enabled}
	created, err := service.Create(context.Background(), worker, input)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-reconciled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the schedule reconciliation")
	}
	lock := service.lockFor(worker.UUID)
	lock.Lock()
	lastApplied := len(service.lastApplied)
	lock.Unlock()
	if lastApplied != 0 {
		t.Fatalf("disabled schedule should not update scheduler state, got %d entries", lastApplied)
	}
	if created.Color == "" {
		t.Fatal("expected a neutral schedule color")
	}
	workers.mu.Lock()
	if len(workers.applied) != 0 {
		workers.mu.Unlock()
		t.Fatal("disabled schedule should not apply a limit")
	}
	workers.mu.Unlock()

	var storedWorker models.Worker
	if err := service.db.DB.First(&storedWorker, "uuid = ?", worker.UUID).Error; err != nil {
		t.Fatal(err)
	}
	if storedWorker.DefaultDownloadSpeedLimit == nil || *storedWorker.DefaultDownloadSpeedLimit != 1000 {
		t.Fatalf("expected captured baseline, got %#v", storedWorker.DefaultDownloadSpeedLimit)
	}

	input.Name = "workday"
	updated, err := service.Update(context.Background(), worker, created.UUID, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "workday" || updated.Color != created.Color {
		t.Fatalf("update should retain assigned color: %#v", updated)
	}
	items, err := service.List(context.Background(), worker.UUID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected one schedule, %v / %#v", err, items)
	}
	if err := service.Delete(context.Background(), worker, created.UUID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), worker.UUID, created.UUID); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("expected deleted schedule to be absent, got %v", err)
	}
}

func TestPreviewAndReconcileApplyOnlyWhenTargetChanges(t *testing.T) {
	service, worker, workers, events := setupService(t)
	now := time.Now().UTC()
	day := int(now.Weekday())
	row := models.BandwidthSchedule{UUID: uuid.New(), WorkerUUID: worker.UUID, Name: "active", DaysOfWeek: uint8(1 << day), StartMinute: 0, EndMinute: 0, DownloadLimit: 500, UploadLimit: 50, Priority: 1, Color: "#64748b", Enabled: true}
	if err := service.db.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := service.Preview(context.Background(), worker.UUID)
	if err != nil || preview.Source != "schedule" || preview.Schedule == nil {
		t.Fatalf("expected active schedule preview, %v / %#v", err, preview)
	}

	service.ReconcileWorker(context.Background(), worker)
	service.ReconcileWorker(context.Background(), worker)
	workers.mu.Lock()
	applications := append([]Limits(nil), workers.applied...)
	workers.mu.Unlock()
	if len(applications) != 1 || applications[0] != (Limits{DownloadLimit: 500, UploadLimit: 50}) {
		t.Fatalf("expected one application, got %#v", applications)
	}
	events.mu.Lock()
	eventCount := len(events.events)
	events.mu.Unlock()
	if eventCount != 1 {
		t.Fatalf("expected one applied event, got %d", eventCount)
	}

	if err := service.db.DB.Model(&models.BandwidthSchedule{}).Where("uuid = ?", row.UUID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	service.ReconcileWorker(context.Background(), worker)
	workers.mu.Lock()
	applications = append([]Limits(nil), workers.applied...)
	workers.mu.Unlock()
	if len(applications) != 2 || applications[1] != (Limits{DownloadLimit: 1000, UploadLimit: 2000}) {
		t.Fatalf("expected baseline restoration, got %#v", applications)
	}
}

func TestReconcileWorkerReloadsAppliedScheduleAfterRestart(t *testing.T) {
	service, worker, workers, events := setupService(t)
	day := int(time.Now().UTC().Weekday())
	scheduleID := uuid.New()
	row := models.BandwidthSchedule{
		UUID: scheduleID, WorkerUUID: worker.UUID, Name: "active", DaysOfWeek: uint8(1 << day),
		StartMinute: 0, EndMinute: 0, DownloadLimit: 500, UploadLimit: 50, Priority: 1, Color: "#64748b", Enabled: true,
	}
	if err := service.db.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	service.ReconcileWorker(context.Background(), worker)
	workers.mu.Lock()
	firstApplications := len(workers.applied)
	workers.mu.Unlock()
	if firstApplications != 1 || len(events.events) != 1 {
		t.Fatalf("expected initial schedule application and event, got %d / %d", firstApplications, len(events.events))
	}

	var storedWorker models.Worker
	if err := service.db.DB.First(&storedWorker, "uuid = ?", worker.UUID).Error; err != nil {
		t.Fatal(err)
	}
	if storedWorker.LastAppliedBandwidthScheduleUUID == nil || *storedWorker.LastAppliedBandwidthScheduleUUID != scheduleID ||
		storedWorker.LastAppliedDownloadSpeedLimit == nil || *storedWorker.LastAppliedDownloadSpeedLimit != 500 ||
		storedWorker.LastAppliedUploadSpeedLimit == nil || *storedWorker.LastAppliedUploadSpeedLimit != 50 {
		t.Fatalf("expected applied schedule state to be persisted, got %#v", storedWorker)
	}

	restartedWorkers := &fakeWorkers{preferences: workers.preferences}
	restartedEvents := &fakeEvents{}
	restarted := &Service{
		db: service.db, workers: restartedWorkers, settings: fakeSettings{timezone: "UTC"}, events: restartedEvents,
		lastApplied: make(map[uuid.UUID]applied), locks: make(map[uuid.UUID]*sync.Mutex),
	}
	restarted.ReconcileWorker(context.Background(), worker)
	restartedWorkers.mu.Lock()
	restartApplications := len(restartedWorkers.applied)
	restartedWorkers.mu.Unlock()
	if restartApplications != 0 || len(restartedEvents.events) != 0 {
		t.Fatalf("restart must not reapply or emit an unchanged schedule, got %d / %d", restartApplications, len(restartedEvents.events))
	}

	if err := restarted.db.DB.Model(&models.BandwidthSchedule{}).Where("uuid = ?", scheduleID).Update("download_limit", 600).Error; err != nil {
		t.Fatal(err)
	}
	restarted.ReconcileWorker(context.Background(), worker)
	restartedWorkers.mu.Lock()
	changedApplications := append([]Limits(nil), restartedWorkers.applied...)
	restartedWorkers.mu.Unlock()
	if len(changedApplications) != 1 || changedApplications[0] != (Limits{DownloadLimit: 600, UploadLimit: 50}) || len(restartedEvents.events) != 1 {
		t.Fatalf("expected only the real post-restart diff, got %#v / %d events", changedApplications, len(restartedEvents.events))
	}
}

func TestReconcileWorkerSeedsLegacyScheduleWithoutDuplicateEvent(t *testing.T) {
	service, worker, workers, events := setupService(t)
	day := int(time.Now().UTC().Weekday())
	scheduleID := uuid.New()
	row := models.BandwidthSchedule{
		UUID: scheduleID, WorkerUUID: worker.UUID, Name: "already active", DaysOfWeek: uint8(1 << day),
		StartMinute: 0, EndMinute: 0, DownloadLimit: 500, UploadLimit: 50, Priority: 1, Color: "#64748b", Enabled: true,
	}
	if err := service.db.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	workers.preferences = &entities.InstancePreferences{GlobalRateLimits: entities.InstancePreferencesGlobalRateLimits{DownloadSpeedLimit: 500, UploadSpeedLimit: 50}}

	service.ReconcileWorker(context.Background(), worker)
	workers.mu.Lock()
	applications := len(workers.applied)
	workers.mu.Unlock()
	if applications != 0 || len(events.events) != 0 {
		t.Fatalf("already active legacy schedule must be seeded silently, got %d applications / %d events", applications, len(events.events))
	}

	var storedWorker models.Worker
	if err := service.db.DB.First(&storedWorker, "uuid = ?", worker.UUID).Error; err != nil {
		t.Fatal(err)
	}
	if storedWorker.LastAppliedBandwidthScheduleUUID == nil || *storedWorker.LastAppliedBandwidthScheduleUUID != scheduleID {
		t.Fatalf("expected legacy schedule state to be persisted, got %#v", storedWorker.LastAppliedBandwidthScheduleUUID)
	}
}

func TestApplyManualDefaultStoresBaselineWithoutSchedulerState(t *testing.T) {
	service, worker, workers, _ := setupService(t)
	if err := service.ApplyManualDefault(context.Background(), worker, Limits{DownloadLimit: 42, UploadLimit: 24}); err != nil {
		t.Fatal(err)
	}
	workers.mu.Lock()
	applications := append([]Limits(nil), workers.applied...)
	workers.mu.Unlock()
	if len(applications) != 1 || applications[0] != (Limits{DownloadLimit: 42, UploadLimit: 24}) {
		t.Fatalf("manual value was not applied: %#v", applications)
	}
	if len(service.lastApplied) != 0 {
		t.Fatal("manual override must not be treated as scheduler-applied")
	}
}

func TestSchedulerQueriesStatusesAndCapturesMissingBaseline(t *testing.T) {
	service, worker, workers, _ := setupService(t)
	if err := service.db.DB.Model(&models.Worker{}).Where("uuid = ?", worker.UUID).Updates(map[string]interface{}{"default_download_speed_limit": nil, "default_upload_speed_limit": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.captureBaseline(context.Background(), worker); err != nil {
		t.Fatal(err)
	}
	statuses, err := service.Statuses(context.Background(), []uuid.UUID{worker.UUID})
	if err != nil || statuses[worker.UUID].Active || statuses[worker.UUID].Limits == nil {
		t.Fatalf("expected default status, %v / %#v", err, statuses)
	}

	workers.list = []*entities.Worker{worker}
	service.reconcileAll(context.Background())
	workers.mu.Lock()
	applications := append([]Limits(nil), workers.applied...)
	workers.mu.Unlock()
	if len(applications) != 0 {
		t.Fatalf("baseline should not overwrite after startup, got %#v", applications)
	}
}

func TestNewServiceAndOverlapDetection(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	if service.interval <= 0 || service.lastApplied == nil || service.locks == nil {
		t.Fatalf("scheduler was not initialized: %#v", service)
	}
	a := models.BandwidthSchedule{DaysOfWeek: 1 << 1, StartMinute: 60, EndMinute: 120}
	b := models.BandwidthSchedule{DaysOfWeek: 1 << 1, StartMinute: 90, EndMinute: 150}
	if !windowsOverlap(a, b) {
		t.Fatal("expected overlapping windows")
	}
	b.StartMinute, b.EndMinute = 180, 240
	if windowsOverlap(a, b) {
		t.Fatal("expected distinct windows")
	}
	overnight := models.BandwidthSchedule{DaysOfWeek: 1 << 1, StartMinute: 22 * 60, EndMinute: 2 * 60}
	followingDay := models.BandwidthSchedule{DaysOfWeek: 1 << 2, StartMinute: 60, EndMinute: 3 * 60}
	if !windowsOverlap(overnight, followingDay) {
		t.Fatal("expected overnight window to overlap the following day")
	}
	followingDay.StartMinute = 2 * 60
	if windowsOverlap(overnight, followingDay) {
		t.Fatal("expected adjacent overnight windows not to overlap")
	}
}

func TestScheduleLimitAndReorder(t *testing.T) {
	service, worker, _, _ := setupService(t)
	enabled := false
	for i := 0; i < maxSchedulesPerWorker; i++ {
		_, err := service.Create(context.Background(), worker, Input{
			Name:          "rule-" + string(rune('a'+i)),
			DaysOfWeek:    []int{1},
			StartTime:     "08:00",
			EndTime:       "18:00",
			DownloadLimit: 100 + i,
			UploadLimit:   10 + i,
			Enabled:       &enabled,
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := service.Create(context.Background(), worker, Input{Name: "one too many", DaysOfWeek: []int{1}, StartTime: "08:00", EndTime: "18:00", Enabled: &enabled}); !errors.Is(err, ErrScheduleLimit) {
		t.Fatalf("expected schedule limit, got %v", err)
	}

	items, err := service.List(context.Background(), worker.UUID)
	if err != nil || len(items) != maxSchedulesPerWorker {
		t.Fatalf("list schedules: %v / %#v", err, items)
	}
	for index, item := range items {
		if item.Priority != -index {
			t.Fatalf("new rules must follow their list position, got priority %d at %d", item.Priority, index)
		}
	}
	orderedIDs := []uuid.UUID{items[2].UUID, items[4].UUID, items[0].UUID, items[3].UUID, items[1].UUID}
	reordered, err := service.Reorder(context.Background(), worker, OrderInput{ScheduleIDs: orderedIDs})
	if err != nil {
		t.Fatal(err)
	}
	for index, item := range reordered {
		if item.UUID != orderedIDs[index] || item.Priority != len(orderedIDs)-index {
			t.Fatalf("unexpected reordered item %d: %#v", index, item)
		}
	}
	if _, err := service.Reorder(context.Background(), worker, OrderInput{ScheduleIDs: orderedIDs[:4]}); !errors.Is(err, ErrInvalidScheduleOrder) {
		t.Fatalf("expected incomplete order to be rejected, got %v", err)
	}
}

func TestStatusesHandlesEveryResultType(t *testing.T) {
	service, worker, _, _ := setupService(t)
	if statuses, err := service.Statuses(context.Background(), nil); err != nil || len(statuses) != 0 {
		t.Fatalf("expected empty result for no workers, got %v / %#v", err, statuses)
	}

	withoutDefaults := uuid.New()
	if err := service.db.DB.Create(&models.Worker{UUID: withoutDefaults, Name: "unmanaged", Address: "http://unmanaged"}).Error; err != nil {
		t.Fatal(err)
	}

	day := int(time.Now().UTC().Weekday())
	active := models.BandwidthSchedule{
		UUID: uuid.New(), WorkerUUID: worker.UUID, Name: "active", DaysOfWeek: uint8(1 << day),
		StartMinute: 0, EndMinute: 0, DownloadLimit: 500, UploadLimit: 50, Priority: 1, Enabled: true,
	}
	if err := service.db.DB.Create(&active).Error; err != nil {
		t.Fatal(err)
	}

	statuses, err := service.Statuses(context.Background(), []uuid.UUID{worker.UUID, withoutDefaults})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses[worker.UUID]; !got.Active || got.Schedule == nil || got.Limits == nil || got.Limits.DownloadLimit != 500 {
		t.Fatalf("expected active schedule status, got %#v", got)
	}
	if got := statuses[withoutDefaults]; got.Active || got.Limits != nil || got.Schedule != nil {
		t.Fatalf("expected unmanaged status, got %#v", got)
	}

	if _, err := service.Statuses(context.Background(), []uuid.UUID{uuid.New()}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected missing worker error, got %v", err)
	}
}

func TestSchedulerValidationAndFailurePaths(t *testing.T) {
	service, worker, workers, events := setupService(t)

	service.settings = fakeSettings{err: errors.New("settings unavailable")}
	if _, err := service.Preview(context.Background(), worker.UUID); err == nil {
		t.Fatal("expected timezone lookup failure")
	}
	service.settings = fakeSettings{timezone: "not/a-timezone"}
	if _, err := service.Statuses(context.Background(), []uuid.UUID{worker.UUID}); err == nil {
		t.Fatal("expected invalid timezone failure")
	}
	service.settings = fakeSettings{timezone: "UTC"}

	for _, input := range []Input{
		{Name: "rule", DaysOfWeek: []int{1}, StartTime: "25:00", EndTime: "01:00"},
		{Name: "rule", DaysOfWeek: []int{1}, StartTime: "01:00", EndTime: "01:60"},
		{Name: "   ", DaysOfWeek: []int{1}, StartTime: "01:00", EndTime: "02:00"},
		{Name: "rule", DaysOfWeek: []int{7}, StartTime: "01:00", EndTime: "02:00"},
		{Name: "rule", DaysOfWeek: nil, StartTime: "01:00", EndTime: "02:00"},
	} {
		if _, err := inputToModel(worker.UUID, input); err == nil {
			t.Fatalf("expected invalid input to be rejected: %#v", input)
		}
	}

	if _, err := service.Get(context.Background(), worker.UUID, uuid.New()); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("expected missing schedule error, got %v", err)
	}
	if err := service.Delete(context.Background(), worker, uuid.New()); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("expected missing delete error, got %v", err)
	}
	if _, err := service.Update(context.Background(), worker, uuid.New(), Input{Name: "rule", DaysOfWeek: []int{1}, StartTime: "01:00", EndTime: "02:00"}); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("expected missing update error, got %v", err)
	}
	if _, err := service.Reorder(context.Background(), worker, OrderInput{ScheduleIDs: []uuid.UUID{uuid.Nil}}); !errors.Is(err, ErrInvalidScheduleOrder) {
		t.Fatalf("expected nil UUID order to be rejected, got %v", err)
	}
	duplicate := uuid.New()
	if _, err := service.Reorder(context.Background(), worker, OrderInput{ScheduleIDs: []uuid.UUID{duplicate, duplicate}}); !errors.Is(err, ErrInvalidScheduleOrder) {
		t.Fatalf("expected duplicate UUID order to be rejected, got %v", err)
	}

	workers.err = errors.New("worker unavailable")
	if err := service.ApplyManualDefault(context.Background(), worker, Limits{DownloadLimit: 1, UploadLimit: 2}); err == nil {
		t.Fatal("expected manual default failure")
	}
	workers.err = nil
	events.err = errors.New("event unavailable")
	target := applied{limits: Limits{DownloadLimit: 1, UploadLimit: 2}, source: "schedule"}
	if err := service.apply(context.Background(), worker, target, applied{}, false, nil); err != nil {
		t.Fatalf("event errors must not prevent applying limits: %v", err)
	}
}

func TestDayRangesAndPriorityConflicts(t *testing.T) {
	allDay := models.BandwidthSchedule{DaysOfWeek: 1 << 3, StartMinute: 120, EndMinute: 120}
	ranges := dayRanges(allDay)
	if len(ranges) != 1 || ranges[0] != (scheduleDayRange{day: 3, start: 0, end: 1440}) {
		t.Fatalf("expected a full-day range, got %#v", ranges)
	}
	if got := dayRanges(models.BandwidthSchedule{}); len(got) != 0 {
		t.Fatalf("expected no ranges, got %#v", got)
	}
	if !modelMatches(allDay, time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("expected the model matcher to use schedule conversion")
	}

	service, worker, _, _ := setupService(t)
	existing := models.BandwidthSchedule{UUID: uuid.New(), WorkerUUID: worker.UUID, DaysOfWeek: 1 << 1, StartMinute: 60, EndMinute: 120, Priority: 1, Enabled: true}
	if err := service.db.DB.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	candidate := existing
	candidate.UUID = uuid.New()
	if err := service.checkPriorityConflict(context.Background(), candidate, uuid.Nil); !errors.Is(err, ErrPriorityConflict) {
		t.Fatalf("expected overlap conflict, got %v", err)
	}
	if err := service.checkPriorityConflict(context.Background(), candidate, existing.UUID); err != nil {
		t.Fatalf("expected excluded rule not to conflict, got %v", err)
	}
}
