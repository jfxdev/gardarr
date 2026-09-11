// Package transferreport snapshots qBittorrent's cumulative byte counters and
// turns their deltas into small, durable daily and weekly rankings.
package transferreport

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/repository/transferreport"
	"github.com/jfxdev/gardarr/pkg/logger"
)

const snapshotRetention = 15 * 24 * time.Hour

var allowedSnapshots = map[int]bool{1: true, 2: true, 3: true, 4: true, 6: true, 8: true, 12: true, 24: true}

type workerService interface {
	ListWorkersBasic() ([]*entities.Worker, error)
	ListTasks(context.Context, []*entities.Worker) (*entities.TaskListResult, error)
}

type timezoneProvider interface {
	GetTimezone(context.Context) (string, error)
}
type eventRecorder interface {
	RecordIfNew(context.Context, *entities.Event) (bool, error)
}

type Service struct {
	repo     *transferreport.Repository
	workers  workerService
	settings timezoneProvider
	events   eventRecorder
	now      func() time.Time
}

func NewService(db *database.Database, workers workerService, settings timezoneProvider, events eventRecorder) *Service {
	return &Service{repo: transferreport.NewRepository(db), workers: workers, settings: settings, events: events, now: time.Now}
}

func (s *Service) GetSettings(ctx context.Context) (*entities.TransferReportSettings, error) {
	return s.repo.EnsureSettings(ctx)
}

func (s *Service) UpdateSettings(ctx context.Context, input entities.TransferReportSettings) (*entities.TransferReportSettings, error) {
	if !allowedSnapshots[input.SnapshotsPerDay] {
		return nil, fmt.Errorf("snapshots_per_day must be one of 1, 2, 3, 4, 6, 8, 12 or 24")
	}
	if input.TopN < 1 || input.TopN > 50 {
		return nil, fmt.Errorf("top_n must be between 1 and 50")
	}
	if !validClock(input.DailyReportTime) || !validClock(input.WeeklyReportTime) {
		return nil, fmt.Errorf("report times must use HH:MM")
	}
	if input.WeeklyReportDay < 0 || input.WeeklyReportDay > 6 {
		return nil, fmt.Errorf("weekly_report_day must be between 0 and 6")
	}
	return s.repo.UpdateSettings(ctx, input)
}

func (s *Service) GetLatest(ctx context.Context) (daily, weekly *entities.TransferReport, err error) {
	daily, err = s.repo.GetLatest(ctx, entities.TransferReportPeriodDaily)
	if errors.Is(err, transferreport.ErrNotFound) {
		daily, err = nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	weekly, err = s.repo.GetLatest(ctx, entities.TransferReportPeriodWeekly)
	if errors.Is(err, transferreport.ErrNotFound) {
		weekly, err = nil, nil
	}
	return daily, weekly, err
}

// GetCurrent builds in-progress daily and weekly rankings from persisted snapshots.
func (s *Service) GetCurrent(ctx context.Context) (daily, weekly *entities.TransferReport, err error) {
	settings, err := s.repo.EnsureSettings(ctx)
	if err != nil {
		return nil, nil, err
	}
	timezone, err := s.settings.GetTimezone(ctx)
	if err != nil {
		return nil, nil, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, err
	}
	now := s.now().UTC()
	localNow := now.In(location)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	daily, err = s.buildReport(ctx, entities.TransferReportPeriodDaily, dayStart.UTC(), now, timezone, settings.TopN)
	if err != nil {
		return nil, nil, err
	}
	daysSinceWeekStart := (int(localNow.Weekday()) - settings.WeeklyReportDay + 7) % 7
	weekStart := dayStart.AddDate(0, 0, -daysSinceWeekStart)
	weekly, err = s.buildReport(ctx, entities.TransferReportPeriodWeekly, weekStart.UTC(), now, timezone, settings.TopN)
	if err != nil {
		return nil, nil, err
	}
	return daily, weekly, nil
}

// CaptureNow records the current worker counters outside the scheduled cadence.
func (s *Service) CaptureNow(ctx context.Context) error {
	now := s.now().UTC()
	return s.capture(ctx, now, now)
}

// Start runs one reconciliation immediately, then checks local-time slots at
// minute granularity. Settings are read each reconciliation so UI updates take
// effect without a process restart.
func (s *Service) Start(ctx context.Context) {
	go func() {
		s.reconcile(ctx)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reconcile(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) reconcile(ctx context.Context) {
	settings, err := s.repo.EnsureSettings(ctx)
	if err != nil {
		logger.Error("transfer reports: load settings failed", "error", err.Error())
		return
	}
	if !settings.Enabled {
		return
	}
	timezone, err := s.settings.GetTimezone(ctx)
	if err != nil {
		logger.Error("transfer reports: load timezone failed", "error", err.Error())
		return
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		logger.Error("transfer reports: invalid timezone", "timezone", timezone, "error", err.Error())
		return
	}
	now := s.now().UTC()

	localNow := now.In(location)
	if slot, ok := snapshotSlot(localNow, settings.SnapshotsPerDay); ok {
		// A first run is a baseline, so historic qBittorrent counters are never
		// attributed to the period in which reporting was enabled. Count runs,
		// not snapshots: a healthy worker with no torrents creates no snapshots.
		runCount, err := s.repo.CountRuns(ctx)
		if err != nil {
			logger.Error("transfer reports: count snapshot runs failed", "error", err.Error())
		}
		if err == nil && runCount == 0 {
			if err := s.capture(ctx, slot.UTC(), now); err != nil {
				logger.Error("transfer reports: baseline failed", "error", err.Error())
			}
		} else {
			exists, err := s.repo.HasRunAt(ctx, slot.UTC())
			if err != nil {
				logger.Error("transfer reports: check snapshot slot failed", "error", err.Error())
			} else if !exists {
				if err := s.capture(ctx, slot.UTC(), now); err != nil {
					logger.Error("transfer reports: snapshot failed", "error", err.Error())
				}
			}
		}
	}

	if err := s.generatePending(ctx, settings, timezone, location, localNow, entities.TransferReportPeriodDaily); err != nil {
		logger.Error("transfer reports: daily generation failed", "error", err.Error())
	}
	if err := s.generatePending(ctx, settings, timezone, location, localNow, entities.TransferReportPeriodWeekly); err != nil {
		logger.Error("transfer reports: weekly generation failed", "error", err.Error())
	}
	if err := s.repo.DeleteSnapshotsBefore(ctx, now.Add(-snapshotRetention)); err != nil {
		logger.Error("transfer reports: cleanup failed", "error", err.Error())
	}
}

func snapshotSlot(now time.Time, perDay int) (time.Time, bool) {
	if perDay <= 0 || 24%perDay != 0 {
		return time.Time{}, false
	}
	interval := 24 / perDay
	hour := now.Hour() - now.Hour()%interval
	return time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location()), true
}

func (s *Service) capture(ctx context.Context, scheduledAt, observedAt time.Time) error {
	workers, err := s.workers.ListWorkersBasic()
	if err != nil {
		return err
	}
	result, err := s.workers.ListTasks(ctx, workers)
	if err != nil {
		return err
	}
	// Runs retain when qBittorrent was actually observed for diagnostics, while
	// snapshots are assigned to their scheduled slot so delayed reconciliation
	// remains in the report period it is filling.
	run := &entities.TransferSnapshotRun{UUID: uuid.New(), ScheduledAt: scheduledAt, CapturedAt: observedAt, WorkerErrors: result.Errors}
	snapshots := make([]entities.TransferSnapshot, 0, len(result.Tasks))
	for _, task := range result.Tasks {
		snapshots = append(snapshots, entities.TransferSnapshot{UUID: uuid.New(), RunID: run.UUID, WorkerID: task.WorkerID, Hash: task.Hash, Name: task.Name, Uploaded: int64(task.Network.Upload.Amount), Downloaded: int64(task.Network.Download.Amount), CapturedAt: scheduledAt})
	}
	return s.repo.CreateRun(ctx, run, snapshots)
}

func (s *Service) generatePending(ctx context.Context, settings *entities.TransferReportSettings, timezone string, location *time.Location, now time.Time, periodType string) error {
	start, end, due := expectedPeriod(now, settings, location, periodType)
	if !due {
		return nil
	}
	existing, err := s.repo.GetLatest(ctx, periodType)
	if err != nil && !errors.Is(err, transferreport.ErrNotFound) {
		return err
	}
	if existing != nil && existing.PeriodStart.Equal(start.UTC()) && existing.PeriodEnd.Equal(end.UTC()) {
		if existing.Coverage == "unavailable" {
			runCount, err := s.repo.CountRunsBetween(ctx, start.UTC(), end.UTC())
			if err != nil {
				return err
			}
			if runCount > 0 {
				refreshed, err := s.buildReport(ctx, periodType, start.UTC(), end.UTC(), timezone, settings.TopN)
				if err != nil {
					return err
				}
				// Keep the durable event identity: refreshing coverage must not
				// create a second Discord notification for the same period.
				refreshed.UUID = existing.UUID
				refreshed.GeneratedAt = existing.GeneratedAt
				existing, err = s.repo.UpsertLatest(ctx, *refreshed)
				if err != nil {
					return err
				}
			}
		}
		return s.recordReportEvent(ctx, existing)
	}
	report, err := s.buildReport(ctx, periodType, start.UTC(), end.UTC(), timezone, settings.TopN)
	if err != nil {
		return err
	}
	saved, err := s.repo.UpsertLatest(ctx, *report)
	if err != nil {
		return err
	}
	return s.recordReportEvent(ctx, saved)
}

func (s *Service) recordReportEvent(ctx context.Context, report *entities.TransferReport) error {
	if s.events == nil {
		return nil
	}
	eventType := constants.EventTypeTransferReportDaily
	if report.PeriodType == entities.TransferReportPeriodWeekly {
		eventType = constants.EventTypeTransferReportWeekly
	}
	eventID := uuid.NewSHA1(report.UUID, []byte("transfer-report-event"))
	if _, err := s.events.RecordIfNew(ctx, &entities.Event{UUID: eventID, Type: eventType, Metadata: reportMetadata(report), CreatedAt: report.GeneratedAt}); err != nil {
		return fmt.Errorf("record report event: %w", err)
	}
	return nil
}

func expectedPeriod(now time.Time, settings *entities.TransferReportSettings, loc *time.Location, periodType string) (time.Time, time.Time, bool) {
	local := now.In(loc)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	if periodType == entities.TransferReportPeriodDaily {
		dueTime := combineClock(dayStart, settings.DailyReportTime)
		if local.Before(dueTime) {
			return time.Time{}, time.Time{}, false
		}
		return dayStart.AddDate(0, 0, -1), dayStart, true
	}
	daysSince := (int(local.Weekday()) - settings.WeeklyReportDay + 7) % 7
	end := dayStart.AddDate(0, 0, -daysSince)
	if daysSince == 0 && local.Before(combineClock(dayStart, settings.WeeklyReportTime)) {
		return time.Time{}, time.Time{}, false
	}
	return end.AddDate(0, 0, -7), end, true
}

func (s *Service) buildReport(ctx context.Context, periodType string, start, end time.Time, timezone string, topN int) (*entities.TransferReport, error) {
	snapshots, err := s.repo.ListSnapshotsUntil(ctx, end)
	if err != nil {
		return nil, err
	}
	items := rankSnapshots(snapshots, start, end, topN)
	runCount, err := s.repo.CountRunsBetween(ctx, start, end)
	if err != nil {
		return nil, err
	}
	errorsByRun, err := s.repo.ListRunErrors(ctx, start, end)
	if err != nil {
		return nil, err
	}
	unavailable := uniqueWorkerIDs(errorsByRun)
	coverage := "complete"
	if runCount == 0 {
		coverage = "unavailable"
	}
	if len(unavailable) > 0 {
		coverage = "partial"
	}
	return &entities.TransferReport{UUID: uuid.New(), PeriodType: periodType, PeriodStart: start, PeriodEnd: end, Timezone: timezone, GeneratedAt: s.now().UTC(), Coverage: coverage, UnavailableWorkers: unavailable, Upload: items.upload, Download: items.download}, nil
}

type rankings struct{ upload, download []entities.TransferRankItem }
type aggregate struct {
	name, hash           string
	uploaded, downloaded int64
}

func rankSnapshots(snapshots []entities.TransferSnapshot, start, end time.Time, topN int) rankings {
	previous := make(map[string]entities.TransferSnapshot)
	aggregates := make(map[string]*aggregate)
	for _, snapshot := range snapshots {
		key := snapshot.WorkerID.String() + "\x00" + strings.ToLower(snapshot.Hash)
		prior, hasPrior := previous[key]
		if hasPrior && snapshot.CapturedAt.After(start) && !snapshot.CapturedAt.After(end) {
			up := counterDelta(prior.Uploaded, snapshot.Uploaded)
			down := counterDelta(prior.Downloaded, snapshot.Downloaded)
			if up > 0 || down > 0 {
				hash := strings.ToLower(snapshot.Hash)
				a := aggregates[hash]
				if a == nil {
					a = &aggregate{name: snapshot.Name, hash: snapshot.Hash}
					aggregates[hash] = a
				}
				if a.name == "" && snapshot.Name != "" {
					a.name = snapshot.Name
				}
				a.uploaded += up
				a.downloaded += down
			}
		}
		previous[key] = snapshot
	}
	upload := make([]entities.TransferRankItem, 0, len(aggregates))
	download := make([]entities.TransferRankItem, 0, len(aggregates))
	for _, item := range aggregates {
		if item.uploaded > 0 {
			upload = append(upload, entities.TransferRankItem{Name: item.name, Hash: item.hash, Bytes: item.uploaded})
		}
		if item.downloaded > 0 {
			download = append(download, entities.TransferRankItem{Name: item.name, Hash: item.hash, Bytes: item.downloaded})
		}
	}
	return rankings{upload: sortRankings(upload, topN), download: sortRankings(download, topN)}
}

func counterDelta(previous, current int64) int64 {
	if current >= previous {
		return current - previous
	}
	return current
}

func sortRankings(items []entities.TransferRankItem, topN int) []entities.TransferRankItem {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Bytes != items[j].Bytes {
			return items[i].Bytes > items[j].Bytes
		}
		if strings.EqualFold(items[i].Name, items[j].Name) {
			return strings.ToLower(items[i].Hash) < strings.ToLower(items[j].Hash)
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	if len(items) > topN {
		items = items[:topN]
	}
	for index := range items {
		items[index].Rank = index + 1
	}
	return items
}

func uniqueWorkerIDs(all []map[string]string) []string {
	set := make(map[string]bool)
	for _, run := range all {
		for workerID := range run {
			set[workerID] = true
		}
	}
	result := make([]string, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func reportMetadata(report *entities.TransferReport) map[string]interface{} {
	return map[string]interface{}{"report_uuid": report.UUID.String(), "period_type": report.PeriodType, "period_start": report.PeriodStart.Format(time.RFC3339), "period_end": report.PeriodEnd.Format(time.RFC3339), "timezone": report.Timezone, "coverage": report.Coverage, "unavailable_workers": report.UnavailableWorkers, "upload": report.Upload, "download": report.Download}
}

func validClock(value string) bool { _, err := time.Parse("15:04", value); return err == nil }
func combineClock(day time.Time, value string) time.Time {
	parts := strings.Split(value, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
}
