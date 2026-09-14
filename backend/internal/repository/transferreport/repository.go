package transferreport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const settingsID = "transfer"

var ErrNotFound = errors.New("transfer report not found")

type Repository struct{ db *database.Database }

func NewRepository(db *database.Database) *Repository { return &Repository{db: db} }

func defaultSettings() *entities.TransferReportSettings {
	return &entities.TransferReportSettings{Enabled: true, SnapshotsPerDay: 4, DailyReportTime: "00:05", WeeklyReportDay: 1, WeeklyReportTime: "00:10", TopN: 10}
}

func (r *Repository) GetSettings(ctx context.Context) (*entities.TransferReportSettings, error) {
	var row models.TransferReportSettings
	err := r.db.DB.WithContext(ctx).Where("id = ?", settingsID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return settingsEntity(row), nil
}

func (r *Repository) EnsureSettings(ctx context.Context) (*entities.TransferReportSettings, error) {
	settings, err := r.GetSettings(ctx)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	defaults := defaultSettings()
	row := models.TransferReportSettings{ID: settingsID, Enabled: defaults.Enabled, SnapshotsPerDay: defaults.SnapshotsPerDay, DailyReportTime: defaults.DailyReportTime, WeeklyReportDay: defaults.WeeklyReportDay, WeeklyReportTime: defaults.WeeklyReportTime, TopN: defaults.TopN}
	if err := r.db.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return nil, err
	}
	return r.GetSettings(ctx)
}

func (r *Repository) UpdateSettings(ctx context.Context, settings entities.TransferReportSettings) (*entities.TransferReportSettings, error) {
	if _, err := r.EnsureSettings(ctx); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{"enabled": settings.Enabled, "snapshots_per_day": settings.SnapshotsPerDay, "daily_report_time": settings.DailyReportTime, "weekly_report_day": settings.WeeklyReportDay, "weekly_report_time": settings.WeeklyReportTime, "top_n": settings.TopN}
	if err := r.db.DB.WithContext(ctx).Model(&models.TransferReportSettings{}).Where("id = ?", settingsID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.GetSettings(ctx)
}

func (r *Repository) HasRunAt(ctx context.Context, scheduledAt time.Time) (bool, error) {
	var count int64
	err := r.db.DB.WithContext(ctx).Model(&models.TransferSnapshotRun{}).Where("scheduled_at = ?", scheduledAt.UTC()).Count(&count).Error
	return count > 0, err
}

func (r *Repository) CountSnapshots(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB.WithContext(ctx).Model(&models.TransferSnapshot{}).Count(&count).Error
	return count, err
}

// CountRuns returns the total number of persisted snapshot runs.
func (r *Repository) CountRuns(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.DB.WithContext(ctx).Model(&models.TransferSnapshotRun{}).Count(&count).Error
	return count, err
}

// CountRunsBetween returns runs assigned to the half-open report period.
func (r *Repository) CountRunsBetween(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.DB.WithContext(ctx).
		Model(&models.TransferSnapshotRun{}).
		Where("scheduled_at > ? AND scheduled_at <= ?", start.UTC(), end.UTC()).
		Count(&count).Error
	return count, err
}

func (r *Repository) CreateRun(ctx context.Context, run *entities.TransferSnapshotRun, snapshots []entities.TransferSnapshot) error {
	errorsJSON, err := json.Marshal(run.WorkerErrors)
	if err != nil {
		return err
	}
	row := models.TransferSnapshotRun{UUID: run.UUID, ScheduledAt: run.ScheduledAt.UTC(), CapturedAt: run.CapturedAt.UTC(), WorkerErrors: string(errorsJSON)}
	if row.UUID == uuid.Nil {
		row.UUID = uuid.New()
		run.UUID = row.UUID
	}
	return r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return nil
		}
		rows := make([]models.TransferSnapshot, 0, len(snapshots))
		for _, snapshot := range snapshots {
			rows = append(rows, models.TransferSnapshot{UUID: snapshot.UUID, RunID: row.UUID, WorkerID: snapshot.WorkerID, Hash: snapshot.Hash, Name: snapshot.Name, Uploaded: snapshot.Uploaded, Downloaded: snapshot.Downloaded, CapturedAt: snapshot.CapturedAt.UTC()})
		}
		return tx.CreateInBatches(&rows, 200).Error
	})
}

func (r *Repository) ListSnapshotsUntil(ctx context.Context, until time.Time) ([]entities.TransferSnapshot, error) {
	var rows []models.TransferSnapshot
	if err := r.db.DB.WithContext(ctx).Where("captured_at <= ?", until.UTC()).Order("worker_id ASC, hash ASC, captured_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.TransferSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, entities.TransferSnapshot{UUID: row.UUID, RunID: row.RunID, WorkerID: row.WorkerID, Hash: row.Hash, Name: row.Name, Uploaded: row.Uploaded, Downloaded: row.Downloaded, CapturedAt: row.CapturedAt})
	}
	return out, nil
}

func (r *Repository) ListRunErrors(ctx context.Context, start, end time.Time) ([]map[string]string, error) {
	var rows []models.TransferSnapshotRun
	if err := r.db.DB.WithContext(ctx).Where("scheduled_at > ? AND scheduled_at <= ?", start.UTC(), end.UTC()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if row.WorkerErrors == "" || row.WorkerErrors == "{}" {
			continue
		}
		var values map[string]string
		if err := json.Unmarshal([]byte(row.WorkerErrors), &values); err == nil && len(values) > 0 {
			out = append(out, values)
		}
	}
	return out, nil
}

// LatestRunErrors returns the worker error map from the most recent run in the
// period. It reflects current availability rather than the whole-period union.
func (r *Repository) LatestRunErrors(ctx context.Context, start, end time.Time) (map[string]string, error) {
	var row models.TransferSnapshotRun
	err := r.db.DB.WithContext(ctx).
		Where("scheduled_at > ? AND scheduled_at <= ?", start.UTC(), end.UTC()).
		Order("scheduled_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.WorkerErrors == "" || row.WorkerErrors == "{}" {
		return nil, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(row.WorkerErrors), &values); err != nil {
		return nil, nil
	}
	return values, nil
}

func (r *Repository) GetLatest(ctx context.Context, periodType string) (*entities.TransferReport, error) {
	var row models.TransferReport
	err := r.db.DB.WithContext(ctx).Where("period_type = ?", periodType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return reportEntity(row)
}

func (r *Repository) UpsertLatest(ctx context.Context, report entities.TransferReport) (*entities.TransferReport, error) {
	workers, err := json.Marshal(report.UnavailableWorkers)
	if err != nil {
		return nil, err
	}
	upload, err := json.Marshal(report.Upload)
	if err != nil {
		return nil, err
	}
	download, err := json.Marshal(report.Download)
	if err != nil {
		return nil, err
	}
	row := models.TransferReport{UUID: report.UUID, PeriodType: report.PeriodType, PeriodStart: report.PeriodStart.UTC(), PeriodEnd: report.PeriodEnd.UTC(), Timezone: report.Timezone, GeneratedAt: report.GeneratedAt.UTC(), Coverage: report.Coverage, UnavailableWorkers: string(workers), Upload: string(upload), Download: string(download)}
	if row.UUID == uuid.Nil {
		row.UUID = uuid.New()
	}
	if err := r.db.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "period_type"}}, DoUpdates: clause.AssignmentColumns([]string{"uuid", "period_start", "period_end", "timezone", "generated_at", "coverage", "unavailable_workers", "upload", "download"})}).Create(&row).Error; err != nil {
		return nil, err
	}
	return reportEntity(row)
}

func (r *Repository) DeleteSnapshotsBefore(ctx context.Context, cutoff time.Time) error {
	return r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("captured_at < ?", cutoff.UTC()).Delete(&models.TransferSnapshot{}).Error; err != nil {
			return err
		}
		return tx.Where("captured_at < ?", cutoff.UTC()).Delete(&models.TransferSnapshotRun{}).Error
	})
}

func settingsEntity(row models.TransferReportSettings) *entities.TransferReportSettings {
	return &entities.TransferReportSettings{Enabled: row.Enabled, SnapshotsPerDay: row.SnapshotsPerDay, DailyReportTime: row.DailyReportTime, WeeklyReportDay: row.WeeklyReportDay, WeeklyReportTime: row.WeeklyReportTime, TopN: row.TopN, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func reportEntity(row models.TransferReport) (*entities.TransferReport, error) {
	var workers []string
	var upload, download []entities.TransferRankItem
	if row.UnavailableWorkers != "" {
		if err := json.Unmarshal([]byte(row.UnavailableWorkers), &workers); err != nil {
			return nil, err
		}
	}
	if row.Upload != "" {
		if err := json.Unmarshal([]byte(row.Upload), &upload); err != nil {
			return nil, err
		}
	}
	if row.Download != "" {
		if err := json.Unmarshal([]byte(row.Download), &download); err != nil {
			return nil, err
		}
	}
	return &entities.TransferReport{UUID: row.UUID, PeriodType: row.PeriodType, PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd, Timezone: row.Timezone, GeneratedAt: row.GeneratedAt, Coverage: row.Coverage, UnavailableWorkers: workers, Upload: upload, Download: download}, nil
}
