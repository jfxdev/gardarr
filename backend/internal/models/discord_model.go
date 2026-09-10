package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DiscordIntegration struct {
	UUID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name                string    `gorm:"size:100;not null"`
	EncryptedWebhookURL string    `gorm:"type:text;not null"`
	Enabled             bool      `gorm:"not null;default:true"`
	AllEvents           bool      `gorm:"not null;default:true"`
	EventTypes          string    `gorm:"type:text"`
	StatusFilter        string    `gorm:"type:text"`
	CategoryFilter      string    `gorm:"type:text"`
	NameTerms           string    `gorm:"type:text"`
	CreatedAt           time.Time `gorm:"autoCreateTime"`
	UpdatedAt           time.Time `gorm:"autoUpdateTime"`
}

func (DiscordIntegration) TableName() string { return "discord_integrations" }

func (d *DiscordIntegration) BeforeCreate(_ *gorm.DB) error {
	if d.UUID == uuid.Nil {
		d.UUID = uuid.New()
	}
	return nil
}

type DiscordDeliveryHistory struct {
	UUID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	DiscordID  uuid.UUID `gorm:"type:uuid;not null;index"`
	EventType  string    `gorm:"size:100;not null"`
	StatusCode int       `gorm:"not null"`
	Error      string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index"`
}

func (DiscordDeliveryHistory) TableName() string { return "discord_delivery_history" }

func (d *DiscordDeliveryHistory) BeforeCreate(_ *gorm.DB) error {
	if d.UUID == uuid.Nil {
		d.UUID = uuid.New()
	}
	return nil
}
