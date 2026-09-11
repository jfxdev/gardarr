// Package discord delivers Gardarr events to Discord incoming webhooks. It is
// intentionally separate from generic webhooks because Discord expects its
// own embed payload and its webhook URL is a credential.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	discordrepo "github.com/jfxdev/gardarr/internal/repository/discord"
	cryptoService "github.com/jfxdev/gardarr/internal/services/crypto"
	"github.com/jfxdev/gardarr/pkg/env"
	"github.com/jfxdev/gardarr/pkg/logger"
)

const (
	queueSizeDefault       = 100
	maxDiscordResponseSize = 1 << 20
)

type Input struct {
	Name           string
	WebhookURL     string
	Enabled        bool
	AllEvents      bool
	EventTypes     []string
	StatusFilter   []string
	CategoryFilter []string
	NameTerms      []string
}

type UpdateInput struct {
	Name           *string
	WebhookURL     *string
	Enabled        *bool
	AllEvents      *bool
	EventTypes     *[]string
	StatusFilter   *[]string
	CategoryFilter *[]string
	NameTerms      *[]string
}

type destination struct {
	integration *entities.DiscordIntegration
	webhookURL  string
	jobs        chan *entities.Event
	stop        chan struct{}
}
type target struct {
	id          uuid.UUID
	integration *entities.DiscordIntegration
	jobs        chan<- *entities.Event
}

type languageProvider interface {
	GetSettings(context.Context) (*entities.Settings, error)
}

type Service struct {
	repo         *discordrepo.Repository
	crypto       *cryptoService.CryptoService
	eventChan    <-chan *entities.Event
	client       *http.Client
	queueSize    int
	maxRetries   int
	backoff      time.Duration
	language     languageProvider
	mu           sync.RWMutex
	destinations map[uuid.UUID]*destination
}

func NewService(eventChan <-chan *entities.Event, db *database.Database, crypto *cryptoService.CryptoService, language languageProvider) *Service {
	queueSize := env.Get("WEBHOOK_QUEUE_SIZE").Default(queueSizeDefault).ValueInt()
	if queueSize <= 0 {
		queueSize = queueSizeDefault
	}
	maxRetries := env.Get("WEBHOOK_MAX_ATTEMPTS").Default(3).ValueInt()
	if maxRetries <= 0 {
		maxRetries = 1
	}
	backoff := env.Get("WEBHOOK_RETRY_BASE_DELAY").Default("2s").ValueDuration()
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	return &Service{repo: discordrepo.NewRepository(db), crypto: crypto, eventChan: eventChan, client: &http.Client{Timeout: 10 * time.Second}, queueSize: queueSize, maxRetries: maxRetries, backoff: backoff, language: language, destinations: make(map[uuid.UUID]*destination)}
}

func (s *Service) Start(ctx context.Context) {
	s.Reload(ctx)
	go func() {
		for {
			select {
			case event, ok := <-s.eventChan:
				if !ok {
					return
				}
				s.dispatch(ctx, event)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) Reload(ctx context.Context) {
	configs, err := s.repo.List(ctx, true)
	if err != nil {
		logger.Error("discord: list integrations failed", "error", err.Error())
		return
	}
	next := make(map[uuid.UUID]*destination)
	for _, config := range configs {
		webhookURL, err := s.crypto.Decrypt(config.EncryptedWebhookURL)
		if err != nil {
			logger.Error("discord: decrypt integration failed", "discord_id", config.UUID.String(), "error", err.Error())
			continue
		}
		next[config.UUID] = &destination{integration: config, webhookURL: webhookURL, jobs: make(chan *entities.Event, s.queueSize), stop: make(chan struct{})}
	}
	s.mu.Lock()
	previous := s.destinations
	s.destinations = next
	s.mu.Unlock()
	for _, destination := range previous {
		close(destination.stop)
	}
	for id, destination := range next {
		go s.runDestination(ctx, id, destination)
	}
}

func (s *Service) runDestination(ctx context.Context, id uuid.UUID, destination *destination) {
	for {
		select {
		case event := <-destination.jobs:
			s.deliver(ctx, id, destination.webhookURL, event)
		case <-destination.stop:
			for {
				select {
				case event := <-destination.jobs:
					s.deliver(ctx, id, destination.webhookURL, event)
				default:
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) dispatch(ctx context.Context, event *entities.Event) {
	if event == nil {
		return
	}
	s.mu.RLock()
	targets := make([]target, 0, len(s.destinations))
	for id, value := range s.destinations {
		targets = append(targets, target{id: id, integration: value.integration, jobs: value.jobs})
	}
	s.mu.RUnlock()
	for _, target := range targets {
		if !matches(target.integration, event) {
			continue
		}
		select {
		case target.jobs <- event:
		default:
			s.saveHistory(ctx, target.id, event.Type, 0, "delivery queue full")
		}
	}
}

func (s *Service) Create(ctx context.Context, input Input) (*entities.DiscordIntegration, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}
	encrypted, err := s.crypto.Encrypt(strings.TrimSpace(input.WebhookURL))
	if err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, entities.DiscordIntegration{UUID: uuid.New(), Name: strings.TrimSpace(input.Name), EncryptedWebhookURL: encrypted, Enabled: input.Enabled, AllEvents: input.AllEvents, EventTypes: compact(input.EventTypes), StatusFilter: compact(input.StatusFilter), CategoryFilter: compact(input.CategoryFilter), NameTerms: compact(input.NameTerms)})
	if err == nil {
		s.Reload(ctx)
	}
	return created, err
}

func (s *Service) List(ctx context.Context) ([]*entities.DiscordIntegration, error) {
	return s.repo.List(ctx, false)
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*entities.DiscordIntegration, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input UpdateInput) (*entities.DiscordIntegration, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	updated := *existing
	if input.Name != nil {
		updated.Name = strings.TrimSpace(*input.Name)
	}
	if input.WebhookURL != nil {
		if err := validateWebhookURL(*input.WebhookURL); err != nil {
			return nil, err
		}
		encrypted, err := s.crypto.Encrypt(strings.TrimSpace(*input.WebhookURL))
		if err != nil {
			return nil, err
		}
		updated.EncryptedWebhookURL = encrypted
	}
	if input.Enabled != nil {
		updated.Enabled = *input.Enabled
	}
	if input.AllEvents != nil {
		updated.AllEvents = *input.AllEvents
	}
	if input.EventTypes != nil {
		updated.EventTypes = compact(*input.EventTypes)
	}
	if input.StatusFilter != nil {
		updated.StatusFilter = compact(*input.StatusFilter)
	}
	if input.CategoryFilter != nil {
		updated.CategoryFilter = compact(*input.CategoryFilter)
	}
	if input.NameTerms != nil {
		updated.NameTerms = compact(*input.NameTerms)
	}
	if err := validateInput(Input{Name: updated.Name, WebhookURL: "https://discord.com/api/webhooks/validated", Enabled: updated.Enabled, AllEvents: updated.AllEvents, EventTypes: updated.EventTypes, StatusFilter: updated.StatusFilter, CategoryFilter: updated.CategoryFilter, NameTerms: updated.NameTerms}); err != nil {
		return nil, err
	}
	result, err := s.repo.Update(ctx, updated)
	if err == nil {
		s.Reload(ctx)
	}
	return result, err
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err == nil {
		s.Reload(ctx)
	}
	return err
}
func (s *Service) History(ctx context.Context, id uuid.UUID, limit, offset int) ([]*entities.DiscordDeliveryHistory, int64, error) {
	return s.repo.ListHistory(ctx, id, limit, offset)
}

func (s *Service) Test(ctx context.Context, id uuid.UUID) error {
	config, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	webhookURL, err := s.crypto.Decrypt(config.EncryptedWebhookURL)
	if err != nil {
		return err
	}
	return s.deliverNow(ctx, id, webhookURL, &entities.Event{UUID: uuid.New(), Type: "test.discord", Metadata: map[string]interface{}{"name": "Gardarr", "test": true}, CreatedAt: time.Now().UTC()})
}

func (s *Service) deliver(ctx context.Context, id uuid.UUID, webhookURL string, event *entities.Event) {
	_ = s.deliverNow(ctx, id, webhookURL, event)
}

func (s *Service) deliverNow(ctx context.Context, id uuid.UUID, webhookURL string, event *entities.Event) error {
	body, err := json.Marshal(buildPayload(event, s.languageCode(ctx)))
	if err != nil {
		return err
	}
	var lastStatus int
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		status, retryAfter, err := s.post(ctx, webhookURL, body)
		lastStatus, lastErr = status, err
		if err == nil {
			s.saveHistory(ctx, id, event.Type, status, "")
			return nil
		}
		if !retryable(status) || attempt == s.maxRetries {
			break
		}
		delay := retryAfter
		if delay <= 0 {
			delay = s.backoff * time.Duration(1<<(attempt-1))
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			lastErr = ctx.Err()
			attempt = s.maxRetries
		}
	}
	message := "delivery failed"
	if lastErr != nil {
		message = lastErr.Error()
	}
	s.saveHistory(ctx, id, event.Type, lastStatus, message)
	return lastErr
}

func (s *Service) languageCode(ctx context.Context) string {
	if s.language == nil {
		return "en-US"
	}
	settings, err := s.language.GetSettings(ctx)
	if err != nil || settings == nil || settings.DefaultLanguage == "" {
		return "en-US"
	}
	return settings.DefaultLanguage
}

func (s *Service) post(ctx context.Context, rawURL string, body []byte) (int, time.Duration, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, 0, err
	}
	query := u.Query()
	query.Set("wait", "true")
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Gardarr-Discord/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, 0, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return 0, 0, context.DeadlineExceeded
		}
		return 0, 0, errors.New("discord request failed")
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxDiscordResponseSize))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, 0, nil
	}
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), responseBody)
	return resp.StatusCode, retryAfter, fmt.Errorf("discord returned status %d", resp.StatusCode)
}

func (s *Service) saveHistory(ctx context.Context, id uuid.UUID, eventType string, status int, message string) {
	if err := s.repo.CreateHistory(ctx, entities.DiscordDeliveryHistory{UUID: uuid.New(), DiscordID: id, EventType: eventType, StatusCode: status, Error: message, CreatedAt: time.Now().UTC()}); err != nil {
		logger.Error("discord: save delivery history failed", "discord_id", id.String(), "error", err.Error())
	}
}

func validateInput(input Input) error {
	if strings.TrimSpace(input.Name) == "" || len(strings.TrimSpace(input.Name)) > 100 {
		return errors.New("name is required and must be at most 100 characters")
	}
	if input.WebhookURL != "" {
		if err := validateWebhookURL(input.WebhookURL); err != nil {
			return err
		}
	}
	if !input.AllEvents && len(input.EventTypes) == 0 {
		return errors.New("select at least one event type or enable all_events")
	}
	for _, eventType := range input.EventTypes {
		if !knownEventType(eventType) {
			return fmt.Errorf("unknown event type: %s", eventType)
		}
	}
	return nil
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("invalid Discord webhook URL")
	}
	host := strings.ToLower(u.Hostname())
	validHost := host == "discord.com" || host == "discordapp.com" || strings.HasSuffix(host, ".discord.com") || strings.HasSuffix(host, ".discordapp.com")
	if u.Scheme != "https" || !validHost || !strings.HasPrefix(u.EscapedPath(), "/api/webhooks/") || u.User != nil {
		return errors.New("webhook URL must be an official HTTPS Discord incoming webhook")
	}
	return nil
}

func knownEventType(value string) bool {
	eventTypes := append([]string{}, constants.TorrentEventTypes...)
	eventTypes = append(eventTypes, constants.WorkerEventTypes...)
	eventTypes = append(eventTypes, constants.ScheduleEventTypes...)
	eventTypes = append(eventTypes, constants.ReportEventTypes...)
	for _, candidate := range eventTypes {
		if value == candidate {
			return true
		}
	}
	return false
}
func compact(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
func retryable(status int) bool {
	return status == 0 || status == http.StatusTooManyRequests || status >= 500
}
func parseRetryAfter(header string, body []byte) time.Duration {
	if value, err := strconv.ParseFloat(header, 64); err == nil && value > 0 {
		return time.Duration(value * float64(time.Second))
	}
	var payload struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.RetryAfter > 0 {
		return time.Duration(payload.RetryAfter * float64(time.Second))
	}
	return 0
}

func matches(config *entities.DiscordIntegration, event *entities.Event) bool {
	if !config.AllEvents {
		found := false
		for _, value := range config.EventTypes {
			if value == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if !strings.HasPrefix(event.Type, "torrent.") {
		return true
	}
	if len(config.StatusFilter) > 0 && !containsFold(config.StatusFilter, event.NewValue) {
		return false
	}
	category, name := metadataString(event, "category"), metadataString(event, "name")
	if len(config.CategoryFilter) > 0 && !containsFold(config.CategoryFilter, category) {
		return false
	}
	if len(config.NameTerms) > 0 {
		found := false
		for _, term := range config.NameTerms {
			if strings.Contains(strings.ToLower(name), strings.ToLower(term)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
func metadataString(event *entities.Event, key string) string {
	if event.Metadata == nil {
		return ""
	}
	value, _ := event.Metadata[key].(string)
	return value
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}
type embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}
type payload struct {
	Embeds []embed `json:"embeds"`
}

func buildPayload(event *entities.Event, language string) payload {
	title, description, color := eventTitle(event, language)
	fields := []embedField{}
	if strings.HasPrefix(event.Type, "report.transfer.") {
		fields = append(fields, reportFields(event, language)...)
	} else if name := metadataString(event, "name"); name != "" {
		fields = append(fields, embedField{Name: "Torrent", Value: truncate(name, 1024), Inline: false})
	}
	if event.NewValue != "" {
		fields = append(fields, embedField{Name: "Status", Value: truncate(event.NewValue, 1024), Inline: true})
	}
	return payload{Embeds: []embed{{Title: title, Description: description, Color: color, Fields: fields, Timestamp: event.CreatedAt.UTC().Format(time.RFC3339)}}}
}

func eventTitle(event *entities.Event, language string) (string, string, int) {
	if language == "pt-BR" {
		switch event.Type {
		case constants.EventTypeTransferReportDaily:
			return "Gardarr · Relatório diário", "Ranking de transferências do dia encerrado.", 0x5865F2
		case constants.EventTypeTransferReportWeekly:
			return "Gardarr · Relatório semanal", "Ranking de transferências da semana encerrada.", 0x5865F2
		case constants.EventTypeTorrentCompleted:
			return "Gardarr · Torrent concluído", "Um torrent terminou o download.", 0x57F287
		case constants.EventTypeWorkerOffline:
			return "Gardarr · Worker indisponível", "Um worker não está acessível.", 0xED4245
		case constants.EventTypeWorkerRecovered:
			return "Gardarr · Worker recuperado", "Um worker está acessível novamente.", 0x57F287
		case "test.discord":
			return "Gardarr · Discord conectado", "Esta é uma notificação de teste.", 0x57F287
		}
	}
	switch event.Type {
	case constants.EventTypeTransferReportDaily:
		return "Gardarr · Daily transfer report", "Top transfer activity for the completed day.", 0x5865F2
	case constants.EventTypeTransferReportWeekly:
		return "Gardarr · Weekly transfer report", "Top transfer activity for the completed week.", 0x5865F2
	case constants.EventTypeTorrentCompleted:
		return "Gardarr · Torrent completed", "A torrent completed downloading.", 0x57F287
	case constants.EventTypeWorkerOffline:
		return "Gardarr · Worker offline", "A worker is no longer reachable.", 0xED4245
	case constants.EventTypeWorkerRecovered:
		return "Gardarr · Worker recovered", "A worker is reachable again.", 0x57F287
	case "test.discord":
		return "Gardarr · Discord connected", "This is a test notification.", 0x57F287
	default:
		return "Gardarr · " + strings.ReplaceAll(event.Type, ".", " "), "A Gardarr event was emitted.", 0xFEE75C
	}
}

func reportFields(event *entities.Event, language string) []embedField {
	fields := []embedField{{Name: "Upload", Value: rankingText(event.Metadata["upload"], language), Inline: false}, {Name: "Download", Value: rankingText(event.Metadata["download"], language), Inline: false}}
	if workers, ok := event.Metadata["unavailable_workers"]; ok && fmt.Sprint(workers) != "[]" {
		fields = append(fields, embedField{Name: "Unavailable workers", Value: truncate(fmt.Sprint(workers), 1024), Inline: false})
	}
	return fields
}
func rankingText(value interface{}, language string) string {
	empty := "No movement recorded."
	if language == "pt-BR" {
		empty = "Nenhuma movimentação registrada."
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return empty
	}
	var items []struct {
		Rank  int    `json:"rank"`
		Name  string `json:"name"`
		Bytes int64  `json:"bytes"`
	}
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return empty
	}
	lines := make([]string, 0, len(items))
	length := 0
	for _, item := range items {
		line := fmt.Sprintf("%s %s — **%s**", rankingMarker(item.Rank), truncateRankingName(item.Name), humanBytes(item.Bytes))
		separatorLength := 0
		if len(lines) > 0 {
			separatorLength = 2
		}
		if length+separatorLength+len([]rune(line)) > 1024 {
			break
		}
		lines = append(lines, line)
		length += separatorLength + len([]rune(line))
	}
	if len(lines) == 0 {
		return empty
	}
	return strings.Join(lines, "\n\n")
}

func rankingMarker(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "•"
	}
}

func truncateRankingName(value string) string {
	const maxRunes = 50
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-3]) + "..."
}
func humanBytes(value int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	amount := float64(value)
	index := 0
	for amount >= 1024 && index < len(units)-1 {
		amount /= 1024
		index++
	}
	if amount >= 100 {
		return fmt.Sprintf("%.0f %s", amount, units[index])
	}
	return fmt.Sprintf("%.1f %s", amount, units[index])
}
func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}
