// Package bandwidthscheduler manages recurring global speed limits for workers.
package bandwidthscheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	eventsservice "github.com/jfxdev/gardarr/internal/services/events"
	settingsservice "github.com/jfxdev/gardarr/internal/services/settings"
	"github.com/jfxdev/gardarr/internal/services/workermanager"
	"github.com/jfxdev/gardarr/pkg/env"
	"github.com/jfxdev/gardarr/pkg/logger"
	"gorm.io/gorm"
)

const (
	maxSchedulesPerWorker = 5
	uuidCondition         = "uuid = ?"
)

var neutralScheduleColors = []string{"#64748b", "#78716c", "#6b7280", "#71717a", "#475569", "#57534e", "#4b5563"}

var (
	ErrScheduleNotFound     = errors.New("bandwidth schedule not found")
	ErrPriorityConflict     = errors.New("overlapping schedules must use different priorities")
	ErrScheduleLimit        = errors.New("a worker can have at most 5 schedules")
	ErrInvalidScheduleOrder = errors.New("schedule_ids must be a complete, unique worker schedule list")
)

type Limits struct {
	DownloadLimit int `json:"download_limit"`
	UploadLimit   int `json:"upload_limit"`
}

type Schedule struct {
	UUID          uuid.UUID `json:"uuid"`
	WorkerUUID    uuid.UUID `json:"worker_uuid"`
	Name          string    `json:"name"`
	DaysOfWeek    []int     `json:"days_of_week"`
	StartTime     string    `json:"start_time"`
	EndTime       string    `json:"end_time"`
	DownloadLimit int       `json:"download_limit"`
	UploadLimit   int       `json:"upload_limit"`
	Priority      int       `json:"priority"`
	Color         string    `json:"color"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Input struct {
	Name          string `json:"name" binding:"required,max=100"`
	DaysOfWeek    []int  `json:"days_of_week" binding:"required,min=1,max=7"`
	StartTime     string `json:"start_time" binding:"required"`
	EndTime       string `json:"end_time" binding:"required"`
	DownloadLimit int    `json:"download_limit" binding:"gte=0"`
	UploadLimit   int    `json:"upload_limit" binding:"gte=0"`
	Enabled       *bool  `json:"enabled"`
}

// OrderInput contains every schedule UUID for a worker, from highest to
// lowest priority. Requiring the complete list keeps reordering atomic.
type OrderInput struct {
	ScheduleIDs []uuid.UUID `json:"schedule_ids" binding:"required,min=1,max=5"`
}

type Preview struct {
	Timezone string    `json:"timezone"`
	Source   string    `json:"source"`
	Schedule *Schedule `json:"schedule,omitempty"`
	Limits   *Limits   `json:"limits,omitempty"`
}

type Status struct {
	Active   bool      `json:"active"`
	Schedule *Schedule `json:"schedule,omitempty"`
	Limits   *Limits   `json:"limits,omitempty"`
}

type applied struct {
	limits     Limits
	source     string
	scheduleID uuid.UUID
}

type workerService interface {
	ListWorkersBasic() ([]*entities.Worker, error)
	GetPreferences(context.Context, *entities.Worker) (*entities.InstancePreferences, error)
	SetWorkerGlobalSpeedLimits(context.Context, *entities.Worker, schemas.InstanceSetSpeedLimitSchema) error
}

type timezoneProvider interface {
	GetTimezone(context.Context) (string, error)
}

type eventRecorder interface {
	Record(context.Context, *entities.Event) error
}

type Service struct {
	db          *database.Database
	workers     workerService
	settings    timezoneProvider
	events      eventRecorder
	interval    time.Duration
	mu          sync.Mutex
	lastApplied map[uuid.UUID]applied
	locks       map[uuid.UUID]*sync.Mutex
}

func NewService(db *database.Database, workers *workermanager.Service, settings *settingsservice.Service, events *eventsservice.Service) *Service {
	interval := env.Get("BANDWIDTH_SCHEDULE_INTERVAL").Default("1m").ValueDuration()
	if interval <= 0 {
		interval = time.Minute
	}
	return &Service{db: db, workers: workers, settings: settings, events: events, interval: interval, lastApplied: make(map[uuid.UUID]applied), locks: make(map[uuid.UUID]*sync.Mutex)}
}

func (s *Service) Start(ctx context.Context) {
	go func() {
		s.reconcileAll(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reconcileAll(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) lockFor(id uuid.UUID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[id] == nil {
		s.locks[id] = &sync.Mutex{}
	}
	return s.locks[id]
}

func (s *Service) reconcileAll(ctx context.Context) {
	workers, err := s.workers.ListWorkersBasic()
	if err != nil {
		logger.Debug("bandwidth scheduler: list workers failed", "error", err.Error())
		return
	}
	var wg sync.WaitGroup
	for _, worker := range workers {
		wg.Add(1)
		go func(w *entities.Worker) { defer wg.Done(); s.ReconcileWorker(ctx, w) }(worker)
	}
	wg.Wait()
}

func (s *Service) ReconcileWorker(ctx context.Context, worker *entities.Worker) {
	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	preview, err := s.preview(ctx, worker.UUID, time.Now())
	if err != nil || preview.Limits == nil {
		if err != nil {
			logger.Debug("bandwidth scheduler: preview failed", "worker_id", worker.UUID.String(), "error", err.Error())
		}
		return
	}
	desired := applied{limits: *preview.Limits, source: preview.Source}
	if preview.Schedule != nil {
		desired.scheduleID = preview.Schedule.UUID
	}
	s.mu.Lock()
	last, exists := s.lastApplied[worker.UUID]
	s.mu.Unlock()
	if !exists {
		last, exists, err = s.loadLastApplied(ctx, worker.UUID)
		if err != nil {
			logger.Debug("bandwidth scheduler: load last applied state failed", "worker_id", worker.UUID.String(), "error", err.Error())
			return
		}
		if exists {
			s.mu.Lock()
			s.lastApplied[worker.UUID] = last
			s.mu.Unlock()
		}
		if !exists && desired.source == "schedule" {
			// Upgrades from versions that kept this state only in memory have no
			// durable marker yet. Seed it without an event when qBittorrent already
			// has the desired limits, so the deployment restart itself is silent.
			preferences, preferencesErr := s.workers.GetPreferences(ctx, worker)
			if preferencesErr != nil {
				logger.Debug("bandwidth scheduler: inspect current limits failed", "worker_id", worker.UUID.String(), "error", preferencesErr.Error())
				return
			}
			current := preferences.GlobalRateLimits
			if current.DownloadSpeedLimit == desired.limits.DownloadLimit && current.UploadSpeedLimit == desired.limits.UploadLimit {
				if persistErr := s.persistLastApplied(ctx, worker.UUID, desired); persistErr != nil {
					logger.Debug("bandwidth scheduler: seed applied state failed", "worker_id", worker.UUID.String(), "error", persistErr.Error())
					return
				}
				s.mu.Lock()
				s.lastApplied[worker.UUID] = desired
				s.mu.Unlock()
				return
			}
		}
	}
	// A scheduler restart must not overwrite an out-of-band manual qBittorrent
	// change just because no schedule is active. The baseline is applied once a
	// schedule that Gardarr applied ends (or on an explicit schedule deletion).
	if !exists && preview.Source == "default" {
		return
	}
	if exists && last == desired {
		return
	}
	if err := s.apply(ctx, worker, desired, last, exists, preview.Schedule); err != nil {
		logger.Debug("bandwidth scheduler: apply failed", "worker_id", worker.UUID.String(), "error", err.Error())
		return
	}
	s.mu.Lock()
	if desired.source == "schedule" {
		s.lastApplied[worker.UUID] = desired
	} else {
		delete(s.lastApplied, worker.UUID)
	}
	s.mu.Unlock()
}

func (s *Service) apply(ctx context.Context, worker *entities.Worker, target, previous applied, hadPrevious bool, schedule *Schedule) error {
	d, u := target.limits.DownloadLimit, target.limits.UploadLimit
	if err := s.workers.SetWorkerGlobalSpeedLimits(ctx, worker, schemas.InstanceSetSpeedLimitSchema{DownloadLimit: &d, UploadLimit: &u}); err != nil {
		return err
	}
	if err := s.persistLastApplied(ctx, worker.UUID, target); err != nil {
		return fmt.Errorf("persist applied bandwidth schedule: %w", err)
	}
	old := map[string]int{}
	if hadPrevious {
		old["download_limit"] = previous.limits.DownloadLimit
		old["upload_limit"] = previous.limits.UploadLimit
	}
	meta := map[string]interface{}{"source": target.source, "download_limit": d, "upload_limit": u, "old_limits": old}
	if schedule != nil {
		meta["schedule_uuid"] = schedule.UUID.String()
		meta["schedule_name"] = schedule.Name
	}
	if err := s.events.Record(ctx, &entities.Event{WorkerID: worker.UUID, Type: constants.EventTypeBandwidthScheduleApplied, Metadata: meta, OldValue: fmt.Sprintf("%d/%d", previous.limits.DownloadLimit, previous.limits.UploadLimit), NewValue: fmt.Sprintf("%d/%d", d, u)}); err != nil {
		logger.Debug("bandwidth scheduler: event failed", "error", err.Error())
	}
	return nil
}

func (s *Service) loadLastApplied(ctx context.Context, workerID uuid.UUID) (applied, bool, error) {
	var row models.Worker
	err := s.db.DB.WithContext(ctx).
		Select("last_applied_bandwidth_schedule_uuid", "last_applied_download_speed_limit", "last_applied_upload_speed_limit").
		Where(uuidCondition, workerID).
		First(&row).Error
	if err != nil {
		return applied{}, false, err
	}
	if row.LastAppliedBandwidthScheduleUUID == nil && row.LastAppliedDownloadSpeedLimit == nil && row.LastAppliedUploadSpeedLimit == nil {
		return applied{}, false, nil
	}
	if row.LastAppliedBandwidthScheduleUUID == nil || row.LastAppliedDownloadSpeedLimit == nil || row.LastAppliedUploadSpeedLimit == nil {
		return applied{}, false, errors.New("incomplete persisted bandwidth schedule state")
	}
	return applied{
		limits: Limits{
			DownloadLimit: *row.LastAppliedDownloadSpeedLimit,
			UploadLimit:   *row.LastAppliedUploadSpeedLimit,
		},
		source:     "schedule",
		scheduleID: *row.LastAppliedBandwidthScheduleUUID,
	}, true, nil
}

func (s *Service) persistLastApplied(ctx context.Context, workerID uuid.UUID, state applied) error {
	values := map[string]interface{}{
		"last_applied_bandwidth_schedule_uuid": nil,
		"last_applied_download_speed_limit":    nil,
		"last_applied_upload_speed_limit":      nil,
	}
	if state.source == "schedule" {
		values["last_applied_bandwidth_schedule_uuid"] = state.scheduleID
		values["last_applied_download_speed_limit"] = state.limits.DownloadLimit
		values["last_applied_upload_speed_limit"] = state.limits.UploadLimit
	}
	return s.db.DB.WithContext(ctx).Model(&models.Worker{}).Where(uuidCondition, workerID).Updates(values).Error
}

func (s *Service) List(ctx context.Context, workerID uuid.UUID) ([]Schedule, error) {
	var rows []models.BandwidthSchedule
	if err := s.db.DB.WithContext(ctx).Where("worker_uuid = ?", workerID).Order("priority DESC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toSchedules(rows), nil
}

func (s *Service) Get(ctx context.Context, workerID, id uuid.UUID) (*Schedule, error) {
	var row models.BandwidthSchedule
	err := s.db.DB.WithContext(ctx).Where("worker_uuid = ? AND uuid = ?", workerID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrScheduleNotFound
	}
	if err != nil {
		return nil, err
	}
	out := toSchedule(row)
	return &out, nil
}

func (s *Service) Create(ctx context.Context, worker *entities.Worker, input Input) (*Schedule, error) {
	row, err := inputToModel(worker.UUID, input)
	if err != nil {
		return nil, err
	}
	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	var count int64
	if err := s.db.DB.WithContext(ctx).Model(&models.BandwidthSchedule{}).Where("worker_uuid = ?", worker.UUID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= maxSchedulesPerWorker {
		return nil, ErrScheduleLimit
	}
	if count > 0 {
		var lowestPriority int
		if err := s.db.DB.WithContext(ctx).Model(&models.BandwidthSchedule{}).Where("worker_uuid = ?", worker.UUID).Select("MIN(priority)").Scan(&lowestPriority).Error; err != nil {
			return nil, err
		}
		// New rules are appended to the visual order and therefore receive the
		// lowest priority. Drag-and-drop is the only way to change that order.
		row.Priority = lowestPriority - 1
	}
	if err := s.captureBaseline(ctx, worker); err != nil {
		return nil, err
	}
	if err := s.checkPriorityConflict(ctx, row, uuid.Nil); err != nil {
		return nil, err
	}
	row.Color = neutralScheduleColor(int(count))
	if err := s.db.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	out := toSchedule(row)
	go s.ReconcileWorker(ctx, worker)
	return &out, nil
}

func (s *Service) Update(ctx context.Context, worker *entities.Worker, id uuid.UUID, input Input) (*Schedule, error) {
	row, err := inputToModel(worker.UUID, input)
	if err != nil {
		return nil, err
	}
	row.UUID = id
	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	var current models.BandwidthSchedule
	if err := s.db.DB.WithContext(ctx).Where("worker_uuid = ? AND uuid = ?", worker.UUID, id).First(&current).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrScheduleNotFound
	} else if err != nil {
		return nil, err
	}
	// Rule order, rather than form input, owns priority.
	row.Priority = current.Priority
	if err := s.checkPriorityConflict(ctx, row, id); err != nil {
		return nil, err
	}
	row.Color = current.Color
	if err := s.db.DB.WithContext(ctx).Model(&current).Updates(map[string]interface{}{"name": row.Name, "days_of_week": row.DaysOfWeek, "start_minute": row.StartMinute, "end_minute": row.EndMinute, "download_limit": row.DownloadLimit, "upload_limit": row.UploadLimit, "enabled": row.Enabled}).Error; err != nil {
		return nil, err
	}
	row.CreatedAt = current.CreatedAt
	row.UpdatedAt = time.Now()
	current = row
	out := toSchedule(current)
	go s.ReconcileWorker(ctx, worker)
	return &out, nil
}

func (s *Service) Delete(ctx context.Context, worker *entities.Worker, id uuid.UUID) error {
	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	result := s.db.DB.WithContext(ctx).Where("worker_uuid = ? AND uuid = ?", worker.UUID, id).Delete(&models.BandwidthSchedule{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrScheduleNotFound
	}
	go s.ReconcileWorker(ctx, worker)
	return nil
}

// Reorder persists the visual rule order as unique priorities. The first UUID
// has the highest priority so it is the rule that wins an overlap.
func (s *Service) Reorder(ctx context.Context, worker *entities.Worker, input OrderInput) ([]Schedule, error) {
	if len(input.ScheduleIDs) == 0 || len(input.ScheduleIDs) > maxSchedulesPerWorker {
		return nil, ErrInvalidScheduleOrder
	}
	ids := make(map[uuid.UUID]struct{}, len(input.ScheduleIDs))
	for _, id := range input.ScheduleIDs {
		if id == uuid.Nil {
			return nil, ErrInvalidScheduleOrder
		}
		if _, exists := ids[id]; exists {
			return nil, ErrInvalidScheduleOrder
		}
		ids[id] = struct{}{}
	}

	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	var rows []models.BandwidthSchedule
	if err := s.db.DB.WithContext(ctx).Where("worker_uuid = ?", worker.UUID).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(input.ScheduleIDs) {
		return nil, ErrInvalidScheduleOrder
	}
	for _, row := range rows {
		if _, exists := ids[row.UUID]; !exists {
			return nil, ErrInvalidScheduleOrder
		}
	}
	if err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, id := range input.ScheduleIDs {
			if err := tx.Model(&models.BandwidthSchedule{}).Where("worker_uuid = ? AND uuid = ?", worker.UUID, id).Update("priority", len(input.ScheduleIDs)-index).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	items, err := s.List(ctx, worker.UUID)
	if err != nil {
		return nil, err
	}
	go s.ReconcileWorker(ctx, worker)
	return items, nil
}

// ApplyManualDefault applies a user-requested limit and persists it as the
// future fallback, while intentionally leaving lastApplied untouched so this
// explicit override survives until the next schedule boundary.
func (s *Service) ApplyManualDefault(ctx context.Context, worker *entities.Worker, limits Limits) error {
	lock := s.lockFor(worker.UUID)
	lock.Lock()
	defer lock.Unlock()
	d, u := limits.DownloadLimit, limits.UploadLimit
	if err := s.workers.SetWorkerGlobalSpeedLimits(ctx, worker, schemas.InstanceSetSpeedLimitSchema{DownloadLimit: &d, UploadLimit: &u}); err != nil {
		return err
	}
	return s.db.DB.WithContext(ctx).Model(&models.Worker{}).Where(uuidCondition, worker.UUID).Updates(map[string]interface{}{"default_download_speed_limit": d, "default_upload_speed_limit": u}).Error
}

func (s *Service) Preview(ctx context.Context, workerID uuid.UUID) (*Preview, error) {
	return s.preview(ctx, workerID, time.Now())
}
func (s *Service) Statuses(ctx context.Context, workerIDs []uuid.UUID) (map[uuid.UUID]Status, error) {
	result := make(map[uuid.UUID]Status, len(workerIDs))
	if len(workerIDs) == 0 {
		return result, nil
	}

	tz, err := s.settings.GetTimezone(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}

	var workers []models.Worker
	if err := s.db.DB.WithContext(ctx).
		Select("uuid", "default_download_speed_limit", "default_upload_speed_limit").
		Where("uuid IN ?", workerIDs).
		Find(&workers).Error; err != nil {
		return nil, err
	}
	workersByID := make(map[uuid.UUID]models.Worker, len(workers))
	for _, worker := range workers {
		workersByID[worker.UUID] = worker
	}

	var rows []models.BandwidthSchedule
	if err := s.db.DB.WithContext(ctx).
		Where("worker_uuid IN ?", workerIDs).
		Order("priority DESC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	schedulesByWorker := make(map[uuid.UUID][]Schedule, len(workerIDs))
	for _, row := range rows {
		schedulesByWorker[row.WorkerUUID] = append(schedulesByWorker[row.WorkerUUID], toSchedule(row))
	}

	now := time.Now().In(loc)
	for _, id := range workerIDs {
		worker, ok := workersByID[id]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		active := Resolve(schedulesByWorker[id], now)
		if active != nil {
			result[id] = Status{Active: true, Schedule: active, Limits: &Limits{active.DownloadLimit, active.UploadLimit}}
			continue
		}
		if worker.DefaultDownloadSpeedLimit != nil && worker.DefaultUploadSpeedLimit != nil {
			result[id] = Status{Limits: &Limits{*worker.DefaultDownloadSpeedLimit, *worker.DefaultUploadSpeedLimit}}
			continue
		}
		result[id] = Status{}
	}
	return result, nil
}

func (s *Service) preview(ctx context.Context, workerID uuid.UUID, now time.Time) (*Preview, error) {
	tz, err := s.settings.GetTimezone(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	var worker models.Worker
	if err := s.db.DB.WithContext(ctx).Select("uuid", "default_download_speed_limit", "default_upload_speed_limit").Where(uuidCondition, workerID).First(&worker).Error; err != nil {
		return nil, err
	}
	rows, err := s.List(ctx, workerID)
	if err != nil {
		return nil, err
	}
	active := Resolve(rows, now.In(loc))
	if active != nil {
		return &Preview{Timezone: tz, Source: "schedule", Schedule: active, Limits: &Limits{active.DownloadLimit, active.UploadLimit}}, nil
	}
	if worker.DefaultDownloadSpeedLimit == nil || worker.DefaultUploadSpeedLimit == nil {
		return &Preview{Timezone: tz, Source: "unmanaged"}, nil
	}
	return &Preview{Timezone: tz, Source: "default", Limits: &Limits{*worker.DefaultDownloadSpeedLimit, *worker.DefaultUploadSpeedLimit}}, nil
}

func (s *Service) captureBaseline(ctx context.Context, worker *entities.Worker) error {
	var row models.Worker
	if err := s.db.DB.WithContext(ctx).Select("uuid", "default_download_speed_limit", "default_upload_speed_limit").Where(uuidCondition, worker.UUID).First(&row).Error; err != nil {
		return err
	}
	if row.DefaultDownloadSpeedLimit != nil && row.DefaultUploadSpeedLimit != nil {
		return nil
	}
	prefs, err := s.workers.GetPreferences(ctx, worker)
	if err != nil {
		return fmt.Errorf("capture worker baseline: %w", err)
	}
	d, u := prefs.GlobalRateLimits.DownloadSpeedLimit, prefs.GlobalRateLimits.UploadSpeedLimit
	return s.db.DB.WithContext(ctx).Model(&models.Worker{}).Where(uuidCondition, worker.UUID).Updates(map[string]interface{}{"default_download_speed_limit": d, "default_upload_speed_limit": u}).Error
}

func (s *Service) checkPriorityConflict(ctx context.Context, candidate models.BandwidthSchedule, excluding uuid.UUID) error {
	var rows []models.BandwidthSchedule
	q := s.db.DB.WithContext(ctx).Where("worker_uuid = ? AND enabled = ? AND priority = ?", candidate.WorkerUUID, true, candidate.Priority)
	if excluding != uuid.Nil {
		q = q.Where("uuid <> ?", excluding)
	}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if windowsOverlap(candidate, row) {
			return ErrPriorityConflict
		}
	}
	return nil
}

func Resolve(schedules []Schedule, now time.Time) *Schedule {
	candidates := make([]Schedule, 0)
	for _, schedule := range schedules {
		if schedule.Enabled && scheduleMatches(schedule, now) {
			candidates = append(candidates, schedule)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Priority > candidates[j].Priority })
	return &candidates[0]
}
func scheduleMatches(s Schedule, now time.Time) bool {
	start, _ := parseTime(s.StartTime)
	end, _ := parseTime(s.EndTime)
	minute := now.Hour()*60 + now.Minute()
	today := int(now.Weekday())
	mask := daysMask(s.DaysOfWeek)
	if start == end {
		return mask&(1<<today) != 0
	}
	if start < end {
		return mask&(1<<today) != 0 && minute >= start && minute < end
	}
	if minute >= start {
		return mask&(1<<today) != 0
	}
	previous := (today + 6) % 7
	return minute < end && mask&(1<<previous) != 0
}
func windowsOverlap(a, b models.BandwidthSchedule) bool {
	for _, aRange := range dayRanges(a) {
		for _, bRange := range dayRanges(b) {
			if aRange.day == bRange.day && aRange.start < bRange.end && bRange.start < aRange.end {
				return true
			}
		}
	}
	return false
}

type scheduleDayRange struct {
	day        int
	start, end int
}

func dayRanges(schedule models.BandwidthSchedule) []scheduleDayRange {
	ranges := make([]scheduleDayRange, 0, 14)
	for day := 0; day < 7; day++ {
		if schedule.DaysOfWeek&(1<<day) == 0 {
			continue
		}
		switch {
		case schedule.StartMinute == schedule.EndMinute:
			ranges = append(ranges, scheduleDayRange{day: day, start: 0, end: 1440})
		case schedule.StartMinute < schedule.EndMinute:
			ranges = append(ranges, scheduleDayRange{day: day, start: schedule.StartMinute, end: schedule.EndMinute})
		default:
			ranges = append(ranges,
				scheduleDayRange{day: day, start: schedule.StartMinute, end: 1440},
				scheduleDayRange{day: (day + 1) % 7, start: 0, end: schedule.EndMinute},
			)
		}
	}
	return ranges
}
func modelMatches(m models.BandwidthSchedule, t time.Time) bool {
	return scheduleMatches(toSchedule(m), t)
}
func inputToModel(workerID uuid.UUID, input Input) (models.BandwidthSchedule, error) {
	start, err := parseTime(input.StartTime)
	if err != nil {
		return models.BandwidthSchedule{}, err
	}
	end, err := parseTime(input.EndTime)
	if err != nil {
		return models.BandwidthSchedule{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return models.BandwidthSchedule{}, errors.New("name is required")
	}
	for _, day := range input.DaysOfWeek {
		if day < 0 || day > 6 {
			return models.BandwidthSchedule{}, errors.New("days_of_week must contain values from 0 to 6")
		}
	}
	mask := daysMask(input.DaysOfWeek)
	if mask == 0 {
		return models.BandwidthSchedule{}, errors.New("days_of_week must contain values from 0 to 6")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return models.BandwidthSchedule{WorkerUUID: workerID, Name: strings.TrimSpace(input.Name), DaysOfWeek: mask, StartMinute: start, EndMinute: end, DownloadLimit: input.DownloadLimit, UploadLimit: input.UploadLimit, Enabled: enabled}, nil
}
func parseTime(v string) (int, error) {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 0, errors.New("time must be HH:MM")
	}
	h, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, errors.New("time must be HH:MM")
	}
	return h*60 + m, nil
}
func daysMask(days []int) uint8 {
	var result uint8
	for _, day := range days {
		if day >= 0 && day <= 6 {
			result |= 1 << day
		}
	}
	return result
}
func maskDays(mask uint8) []int {
	out := make([]int, 0, 7)
	for i := 0; i < 7; i++ {
		if mask&(1<<i) != 0 {
			out = append(out, i)
		}
	}
	return out
}
func formatTime(minute int) string { return fmt.Sprintf("%02d:%02d", minute/60, minute%60) }
func neutralScheduleColor(index int) string {
	return neutralScheduleColors[index%len(neutralScheduleColors)]
}
func toSchedule(m models.BandwidthSchedule) Schedule {
	return Schedule{UUID: m.UUID, WorkerUUID: m.WorkerUUID, Name: m.Name, DaysOfWeek: maskDays(m.DaysOfWeek), StartTime: formatTime(m.StartMinute), EndTime: formatTime(m.EndMinute), DownloadLimit: m.DownloadLimit, UploadLimit: m.UploadLimit, Priority: m.Priority, Color: m.Color, Enabled: m.Enabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func toSchedules(rows []models.BandwidthSchedule) []Schedule {
	out := make([]Schedule, len(rows))
	for i, row := range rows {
		out[i] = toSchedule(row)
	}
	return out
}
