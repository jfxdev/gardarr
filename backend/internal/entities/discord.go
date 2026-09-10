package entities

import (
	"time"

	"github.com/google/uuid"
)

// DiscordIntegration is an incoming-webhook destination. The URL is always
// encrypted at rest and is deliberately absent from API response types.
type DiscordIntegration struct {
	UUID                uuid.UUID
	Name                string
	EncryptedWebhookURL string
	Enabled             bool
	AllEvents           bool
	EventTypes          []string
	StatusFilter        []string
	CategoryFilter      []string
	NameTerms           []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type DiscordDeliveryHistory struct {
	UUID       uuid.UUID
	DiscordID  uuid.UUID
	EventType  string
	StatusCode int
	Error      string
	CreatedAt  time.Time
}
