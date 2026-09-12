package discord

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	cryptoService "github.com/jfxdev/gardarr/internal/services/crypto"
)

const discordTestKey = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="

type fakeLanguage struct{ code string }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (f fakeLanguage) GetSettings(context.Context) (*entities.Settings, error) {
	return &entities.Settings{DefaultLanguage: f.code}, nil
}

func testDiscordService(t *testing.T) (*Service, context.Context, context.CancelFunc) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", discordTestKey)
	crypto, err := cryptoService.NewCryptoService()
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	db := database.SetupTestDB(t, &models.DiscordIntegration{}, &models.DiscordDeliveryHistory{})
	ctx, cancel := context.WithCancel(context.Background())
	service := NewService(make(chan *entities.Event), db, crypto, fakeLanguage{code: "pt-BR"})
	service.backoff = time.Millisecond
	service.maxRetries = 2
	return service, ctx, cancel
}

func TestValidateWebhookURL(t *testing.T) {
	if err := validateWebhookURL("https://discord.com/api/webhooks/123/token"); err != nil {
		t.Fatalf("expected valid Discord URL: %v", err)
	}
	for _, raw := range []string{"http://discord.com/api/webhooks/123/token", "https://example.com/api/webhooks/123/token", "https://discord.com/not-webhooks"} {
		if err := validateWebhookURL(raw); err == nil {
			t.Fatalf("expected URL rejection: %s", raw)
		}
	}
}

func TestMatchesLeavesGlobalReportsUnaffectedByTorrentFilters(t *testing.T) {
	config := &entities.DiscordIntegration{AllEvents: false, EventTypes: []string{constants.EventTypeTransferReportDaily}, CategoryFilter: []string{"movies"}, NameTerms: []string{"linux"}}
	event := &entities.Event{Type: constants.EventTypeTransferReportDaily}
	if !matches(config, event) {
		t.Fatal("report should not be filtered by torrent-only filters")
	}
}

func TestBuildPayloadRendersEmptyReportPlaceholder(t *testing.T) {
	event := &entities.Event{UUID: uuid.New(), Type: constants.EventTypeTransferReportDaily, CreatedAt: time.Now(), Metadata: map[string]interface{}{"period_start": "2026-09-01T00:00:00Z", "period_end": "2026-09-02T00:00:00Z", "coverage": "unavailable", "upload": []entities.TransferRankItem{}, "download": []entities.TransferRankItem{}}}
	payload := buildPayload(event, "pt-BR")
	if payload.Embeds[0].Title != "Gardarr · Relatório diário" || len(payload.Embeds[0].Fields) != 2 || payload.Embeds[0].Fields[0].Name != "Upload" || payload.Embeds[0].Fields[1].Name != "Download" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestBuildPayloadUsesCurrentReportDescription(t *testing.T) {
	event := &entities.Event{Type: constants.EventTypeTransferReportDaily, CreatedAt: time.Now(), Metadata: map[string]interface{}{"in_progress": true, "upload": []entities.TransferRankItem{}, "download": []entities.TransferRankItem{}}}
	if description := buildPayload(event, "en-US").Embeds[0].Description; description != "Top transfer activity so far today." {
		t.Fatalf("current report description = %q", description)
	}
}

func TestDeliverSendsMatchingDiscordDestinations(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	id := uuid.New()
	service.destinations[id] = &destination{integration: &entities.DiscordIntegration{UUID: id, AllEvents: false, EventTypes: []string{constants.EventTypeTransferReportDaily}}, webhookURL: server.URL}

	delivered, err := service.Deliver(ctx, &entities.Event{UUID: uuid.New(), Type: constants.EventTypeTransferReportDaily, Metadata: map[string]interface{}{"upload": []entities.TransferRankItem{}, "download": []entities.TransferRankItem{}}, CreatedAt: time.Now()})
	if err != nil || delivered != 1 || calls.Load() != 1 {
		t.Fatalf("manual delivery = %d, err=%v, calls=%d", delivered, err, calls.Load())
	}
	if _, err := service.Deliver(ctx, &entities.Event{Type: constants.EventTypeTransferReportWeekly}); !errors.Is(err, ErrNoMatchingDestinations) {
		t.Fatalf("expected no matching destination error, got %v", err)
	}
}

func TestRankingTextUsesMedalsCompactNamesAndBoldBytes(t *testing.T) {
	longName := strings.Repeat("a", 60)
	value := []entities.TransferRankItem{
		{Rank: 1, Name: "Gold", Bytes: 1024},
		{Rank: 2, Name: "Silver", Bytes: 2048},
		{Rank: 3, Name: "Bronze", Bytes: 3072},
		{Rank: 4, Name: longName, Bytes: 4096},
	}
	got := rankingText(value, "en-US")
	want := "🥇 Gold — **1.0 KB**\n\n🥈 Silver — **2.0 KB**\n\n🥉 Bronze — **3.0 KB**\n\n• " + strings.Repeat("a", 47) + "... — **4.0 KB**"
	if got != want {
		t.Fatalf("rankingText() = %q, want %q", got, want)
	}
}

func TestServiceStoresEncryptedDestinationsAndPreservesURLOnUpdate(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	created, err := service.Create(ctx, Input{Name: "  Alerts  ", WebhookURL: "https://discord.com/api/webhooks/123/token", Enabled: true, AllEvents: false, EventTypes: []string{constants.EventTypeTorrentCompleted, constants.EventTypeTorrentCompleted}, CategoryFilter: []string{" movies ", "movies"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "Alerts" || created.EncryptedWebhookURL == "" || created.EncryptedWebhookURL == "https://discord.com/api/webhooks/123/token" || len(created.EventTypes) != 1 {
		t.Fatalf("created integration was not normalized/encrypted: %#v", created)
	}
	listed, err := service.List(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %#v err=%v", listed, err)
	}
	originalSecret := listed[0].EncryptedWebhookURL
	name := "Renamed"
	enabled := false
	updated, err := service.Update(ctx, created.UUID, UpdateInput{Name: &name, Enabled: &enabled})
	if err != nil || updated.Name != name || updated.Enabled || updated.EncryptedWebhookURL != originalSecret {
		t.Fatalf("update should preserve secret: %#v err=%v", updated, err)
	}
	if _, err := service.Update(ctx, created.UUID, UpdateInput{WebhookURL: ptr("https://example.com/api/webhooks/nope")}); err == nil {
		t.Fatal("expected invalid host to be rejected")
	}
	if err := service.Delete(ctx, created.UUID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if values, err := service.List(ctx); err != nil || len(values) != 0 {
		t.Fatalf("deleted integration remained: %#v err=%v", values, err)
	}
}

func TestServiceCreatesDisabledFilteredDestination(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	created, err := service.Create(ctx, Input{Name: "Paused reports", WebhookURL: "https://discord.com/api/webhooks/123/token", Enabled: false, AllEvents: false, EventTypes: []string{constants.EventTypeTransferReportDaily}})
	if err != nil {
		t.Fatalf("create disabled destination: %v", err)
	}
	if created.Enabled || created.AllEvents {
		t.Fatalf("explicit false values were not persisted: %#v", created)
	}
}

func TestTransportFailureDoesNotPersistWebhookToken(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	service.maxRetries = 1
	service.client = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}
	id := uuid.New()
	webhookURL := "https://discord.com/api/webhooks/123/secret-token"
	if err := service.deliverNow(ctx, id, webhookURL, &entities.Event{UUID: uuid.New(), Type: constants.EventTypeTorrentCompleted, CreatedAt: time.Now()}); err == nil {
		t.Fatal("expected transport failure")
	}
	history, total, err := service.History(ctx, id, 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("history: %#v total=%d err=%v", history, total, err)
	}
	if history[0].Error != "discord request failed" || strings.Contains(history[0].Error, "secret-token") {
		t.Fatalf("webhook token leaked in history: %q", history[0].Error)
	}
}

func TestDeliveryRetriesAndRecordsHistoryWithoutSecret(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("wait") != "true" || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Discord request headers/query")
		}
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retry_after":0}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	id := uuid.New()
	if err := service.deliverNow(ctx, id, server.URL, &entities.Event{UUID: uuid.New(), Type: constants.EventTypeTransferReportDaily, CreatedAt: time.Now(), Metadata: map[string]interface{}{"upload": []entities.TransferRankItem{}, "download": []entities.TransferRankItem{}}}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected one retry, got %d attempts", attempts.Load())
	}
	history, total, err := service.History(ctx, id, 10, 0)
	if err != nil || total != 1 || history[0].StatusCode != http.StatusNoContent || history[0].Error != "" {
		t.Fatalf("unexpected delivery history: %#v total=%d err=%v", history, total, err)
	}
}

func TestFiltersAndFormattingCoverTorrentAndGlobalEvents(t *testing.T) {
	if err := validateInput(Input{Name: "x", WebhookURL: "https://discord.com/api/webhooks/1/x", AllEvents: false, EventTypes: []string{"unknown"}}); err == nil {
		t.Fatal("unknown event type should be rejected")
	}
	if err := validateInput(Input{Name: "x", WebhookURL: "https://discord.com/api/webhooks/1/x", AllEvents: false}); err == nil {
		t.Fatal("selected events require an event type")
	}
	config := &entities.DiscordIntegration{Enabled: true, AllEvents: false, EventTypes: []string{constants.EventTypeTorrentCompleted}, StatusFilter: []string{"UPLOADING"}, CategoryFilter: []string{"movies"}, NameTerms: []string{"linux"}}
	matching := &entities.Event{Type: constants.EventTypeTorrentCompleted, NewValue: "UPLOADING", Metadata: map[string]interface{}{"category": "Movies", "name": "Linux ISO"}}
	if !matches(config, matching) {
		t.Fatal("expected matching torrent event")
	}
	matching.NewValue = "ERROR"
	if matches(config, matching) {
		t.Fatal("torrent filters should suppress delivery")
	}
	if parseRetryAfter("2", nil) != 2*time.Second || !retryable(429) || retryable(400) || !knownEventType(constants.EventTypeTransferReportWeekly) {
		t.Fatal("retry/event type helpers returned an unexpected result")
	}
	payload := buildPayload(&entities.Event{Type: constants.EventTypeTransferReportWeekly, CreatedAt: time.Now(), Metadata: map[string]interface{}{"period_start": "a", "period_end": "b", "coverage": "partial", "upload": []entities.TransferRankItem{{Rank: 1, Name: "Alpha", Bytes: 1024}}, "download": []entities.TransferRankItem{}}}, "pt-BR")
	if payload.Embeds[0].Title != "Gardarr · Relatório semanal" || len(payload.Embeds[0].Fields) != 2 || payload.Embeds[0].Fields[0].Value == "" || rankingText(nil, "pt-BR") != "Nenhuma movimentação registrada." || humanBytes(1024) != "1.0 KB" {
		t.Fatalf("unexpected formatted payload: %#v", payload)
	}
}

func TestStartDispatchesToReloadedDestinationAndTestDelivers(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	events := make(chan *entities.Event, 1)
	service.eventChan = events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	secret, err := service.crypto.Encrypt(server.URL)
	if err != nil {
		t.Fatalf("encrypt test URL: %v", err)
	}
	integration, err := service.repo.Create(ctx, entities.DiscordIntegration{UUID: uuid.New(), Name: "Local", EncryptedWebhookURL: secret, Enabled: true, AllEvents: true})
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}
	service.Start(ctx)
	events <- &entities.Event{UUID: uuid.New(), Type: constants.EventTypeTorrentCompleted, NewValue: "UPLOADING", Metadata: map[string]interface{}{"name": "Alpha"}, CreatedAt: time.Now()}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		history, total, historyErr := service.History(ctx, integration.UUID, 10, 0)
		if historyErr == nil && total == 1 && history[0].EventType == constants.EventTypeTorrentCompleted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if history, total, err := service.History(ctx, integration.UUID, 10, 0); err != nil || total != 1 || history[0].StatusCode != http.StatusNoContent {
		t.Fatalf("dispatch history: %#v total=%d err=%v", history, total, err)
	}
	if err := service.Test(ctx, integration.UUID); err != nil {
		t.Fatalf("test delivery: %v", err)
	}
}

func TestDiscordFormattingAndFailurePaths(t *testing.T) {
	service, ctx, cancel := testDiscordService(t)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	if err := service.deliverNow(ctx, uuid.New(), server.URL, &entities.Event{UUID: uuid.New(), Type: "custom.event", CreatedAt: time.Now(), NewValue: "OK", Metadata: map[string]interface{}{"name": "A very useful torrent"}}); err == nil {
		t.Fatal("non-retryable delivery failure should be returned")
	}
	for _, event := range []*entities.Event{
		{Type: constants.EventTypeTorrentCompleted},
		{Type: constants.EventTypeWorkerOffline},
		{Type: constants.EventTypeWorkerRecovered},
		{Type: "test.discord"},
		{Type: "other.event"},
	} {
		if title, _, _ := eventTitle(event, "pt-BR"); title == "" {
			t.Fatalf("missing localized title for %s", event.Type)
		}
		if title, _, _ := eventTitle(event, "en-US"); title == "" {
			t.Fatalf("missing English title for %s", event.Type)
		}
	}
	if got := buildPayload(&entities.Event{Type: constants.EventTypeTorrentCompleted, CreatedAt: time.Now(), NewValue: "UPLOADING", Metadata: map[string]interface{}{"name": "Alpha"}}, "en-US"); len(got.Embeds[0].Fields) != 2 {
		t.Fatalf("torrent payload should contain name and status: %#v", got)
	}
}

func ptr(value string) *string { return &value }
