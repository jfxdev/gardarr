package discord

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("discord integration not found")

type Repository struct{ db *database.Database }

func NewRepository(db *database.Database) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, integration entities.DiscordIntegration) (*entities.DiscordIntegration, error) {
	row, err := discordModel(integration)
	if err != nil {
		return nil, err
	}
	if err := r.db.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return discordEntity(row)
}

func (r *Repository) List(ctx context.Context, enabledOnly bool) ([]*entities.DiscordIntegration, error) {
	var rows []models.DiscordIntegration
	query := r.db.DB.WithContext(ctx).Order("created_at DESC")
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.DiscordIntegration, 0, len(rows))
	for _, row := range rows {
		value, err := discordEntity(row)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*entities.DiscordIntegration, error) {
	var row models.DiscordIntegration
	err := r.db.DB.WithContext(ctx).Where("uuid = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return discordEntity(row)
}

func (r *Repository) Update(ctx context.Context, integration entities.DiscordIntegration) (*entities.DiscordIntegration, error) {
	row, err := discordModel(integration)
	if err != nil {
		return nil, err
	}
	result := r.db.DB.WithContext(ctx).Model(&models.DiscordIntegration{}).Where("uuid = ?", integration.UUID).Updates(map[string]interface{}{"name": row.Name, "encrypted_webhook_url": row.EncryptedWebhookURL, "enabled": row.Enabled, "all_events": row.AllEvents, "event_types": row.EventTypes, "status_filter": row.StatusFilter, "category_filter": row.CategoryFilter, "name_terms": row.NameTerms})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, integration.UUID)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("discord_id = ?", id).Delete(&models.DiscordDeliveryHistory{}).Error; err != nil {
			return err
		}
		result := tx.Where("uuid = ?", id).Delete(&models.DiscordIntegration{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) CreateHistory(ctx context.Context, history entities.DiscordDeliveryHistory) error {
	row := models.DiscordDeliveryHistory{UUID: history.UUID, DiscordID: history.DiscordID, EventType: history.EventType, StatusCode: history.StatusCode, Error: history.Error, CreatedAt: history.CreatedAt}
	return r.db.DB.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListHistory(ctx context.Context, id uuid.UUID, limit, offset int) ([]*entities.DiscordDeliveryHistory, int64, error) {
	var rows []models.DiscordDeliveryHistory
	var total int64
	query := r.db.DB.WithContext(ctx).Model(&models.DiscordDeliveryHistory{}).Where("discord_id = ?", id)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*entities.DiscordDeliveryHistory, 0, len(rows))
	for _, row := range rows {
		out = append(out, &entities.DiscordDeliveryHistory{UUID: row.UUID, DiscordID: row.DiscordID, EventType: row.EventType, StatusCode: row.StatusCode, Error: row.Error, CreatedAt: row.CreatedAt})
	}
	return out, total, nil
}

func discordModel(value entities.DiscordIntegration) (models.DiscordIntegration, error) {
	eventTypes, err := json.Marshal(value.EventTypes)
	if err != nil {
		return models.DiscordIntegration{}, err
	}
	status, err := json.Marshal(value.StatusFilter)
	if err != nil {
		return models.DiscordIntegration{}, err
	}
	category, err := json.Marshal(value.CategoryFilter)
	if err != nil {
		return models.DiscordIntegration{}, err
	}
	names, err := json.Marshal(value.NameTerms)
	if err != nil {
		return models.DiscordIntegration{}, err
	}
	return models.DiscordIntegration{UUID: value.UUID, Name: value.Name, EncryptedWebhookURL: value.EncryptedWebhookURL, Enabled: value.Enabled, AllEvents: value.AllEvents, EventTypes: string(eventTypes), StatusFilter: string(status), CategoryFilter: string(category), NameTerms: string(names)}, nil
}

func discordEntity(row models.DiscordIntegration) (*entities.DiscordIntegration, error) {
	value := &entities.DiscordIntegration{UUID: row.UUID, Name: row.Name, EncryptedWebhookURL: row.EncryptedWebhookURL, Enabled: row.Enabled, AllEvents: row.AllEvents, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	for _, target := range []struct {
		raw string
		out *[]string
	}{{row.EventTypes, &value.EventTypes}, {row.StatusFilter, &value.StatusFilter}, {row.CategoryFilter, &value.CategoryFilter}, {row.NameTerms, &value.NameTerms}} {
		if target.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(target.raw), target.out); err != nil {
			return nil, err
		}
	}
	return value, nil
}
