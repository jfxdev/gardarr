package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/repository/webhook_history"
	"github.com/jfxdev/gardarr/pkg/env"
	"github.com/jfxdev/gardarr/pkg/logger"
)

// Payload represents the webhook POST payload structure
type Payload struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	WorkerID  string                 `json:"worker_id,omitempty"`
	TaskHash  string                 `json:"task_hash"`
	OldValue  string                 `json:"old_value,omitempty"`
	NewValue  string                 `json:"new_value,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// Service manages webhook delivery for events
type Service struct {
	webhookURL         string
	webhookID          uuid.UUID
	insecureSkipVerify bool
	timeoutSeconds     int
	httpClient         *http.Client
	enabled            bool
	historyRepo        *webhook_history.Repository
	maxAttempts        int
	baseBackoff        time.Duration
}

// deliveryError carries the HTTP status returned by the webhook endpoint so
// the retry loop can distinguish permanent client errors (4xx) from
// transient ones (5xx/429/network).
type deliveryError struct {
	statusCode int
}

func (e *deliveryError) Error() string {
	return fmt.Sprintf("webhook returned status %d", e.statusCode)
}

// isRetryable reports whether a delivery error is worth retrying: network
// failures and server-side errors are; 4xx responses (except 429) mean the
// request itself is rejected and will keep failing.
func isRetryable(err error) bool {
	var de *deliveryError
	if errors.As(err, &de) {
		return de.statusCode >= 500 || de.statusCode == http.StatusTooManyRequests
	}
	return true
}

// NewService creates a new webhook service that sends events to the specified URL
func NewService(webhookID uuid.UUID, webhookURL string, insecureSkipVerify bool, timeoutSeconds int, historyRepo *webhook_history.Repository) *Service {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10 // Default timeout
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
	}

	return &Service{
		webhookURL:         webhookURL,
		webhookID:          webhookID,
		insecureSkipVerify: insecureSkipVerify,
		timeoutSeconds:     timeoutSeconds,
		httpClient: &http.Client{
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
			Transport: transport,
		},
		enabled:     true,
		historyRepo: historyRepo,
		maxAttempts: env.Get("WEBHOOK_MAX_ATTEMPTS").Default(3).ValueInt(),
		baseBackoff: env.Get("WEBHOOK_RETRY_BASE_DELAY").Default("2s").ValueDuration(),
	}
}

// SendEventWithRetry delivers an event, retrying transient failures with
// exponential backoff (baseBackoff, 2x baseBackoff, ...). Permanent failures
// (4xx other than 429) abort immediately. Only the final attempt's outcome is
// recorded in webhook history, so a retried delivery produces exactly one row.
func (s *Service) SendEventWithRetry(ctx context.Context, event *entities.Event) error {
	if !s.enabled {
		return nil
	}

	maxAttempts := s.maxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var result deliveryResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result = s.deliver(ctx, event)
		if result.err == nil {
			s.saveHistory(ctx, event, result.statusCode, result.responseBody, result.requestBody, "")
			return nil
		}
		if !isRetryable(result.err) || attempt == maxAttempts {
			break
		}

		delay := s.baseBackoff * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}

	s.saveHistory(ctx, event, result.statusCode, result.responseBody, result.requestBody, result.err.Error())

	if !isRetryable(result.err) {
		return result.err
	}
	return fmt.Errorf("webhook delivery failed after %d attempts: %w", maxAttempts, result.err)
}

// SendEvent sends an event to the configured webhook URL, recording the
// single attempt in webhook history.
func (s *Service) SendEvent(ctx context.Context, event *entities.Event) error {
	if !s.enabled {
		return nil
	}

	result := s.deliver(ctx, event)
	s.saveHistory(ctx, event, result.statusCode, result.responseBody, result.requestBody, errMsg(result.err))
	return result.err
}

// deliveryResult carries the outcome of a single delivery attempt.
type deliveryResult struct {
	statusCode   int
	responseBody string
	requestBody  string
	err          error
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// deliver performs a single HTTP delivery attempt without touching webhook
// history, so callers can decide when/whether to persist the outcome.
func (s *Service) deliver(ctx context.Context, event *entities.Event) deliveryResult {
	if !s.enabled {
		return deliveryResult{}
	}

	if event == nil {
		return deliveryResult{err: fmt.Errorf("event is nil")}
	}

	payload := s.buildPayload(event)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal webhook payload",
			"service", "webhook",
			"event_id", event.UUID.String(),
			"error", err,
		)
		return deliveryResult{requestBody: string(jsonData), err: fmt.Errorf("failed to marshal payload: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create webhook request",
			"service", "webhook",
			"event_id", event.UUID.String(),
			"error", err,
		)
		return deliveryResult{requestBody: string(jsonData), err: fmt.Errorf("failed to create request: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Gardarr-Webhook/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send webhook",
			"service", "webhook",
			"event_id", event.UUID.String(),
			"url", s.webhookURL,
			"error", err,
		)
		return deliveryResult{requestBody: string(jsonData), err: fmt.Errorf("failed to send webhook: %w", err)}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read webhook response body",
			"service", "webhook",
			"event_id", event.UUID.String(),
			"url", s.webhookURL,
			"status_code", resp.StatusCode,
			"error", err,
		)
		// Use empty string as fallback to allow processing to continue
		respBody = []byte{}
	}
	respBodyStr := string(respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("Webhook returned non-success status",
			"service", "webhook",
			"event_id", event.UUID.String(),
			"status_code", resp.StatusCode,
			"url", s.webhookURL,
		)
		deliveryErr := &deliveryError{statusCode: resp.StatusCode}
		return deliveryResult{statusCode: resp.StatusCode, responseBody: respBodyStr, requestBody: string(jsonData), err: deliveryErr}
	}

	logger.Debug("Webhook sent successfully",
		"service", "webhook",
		"event_id", event.UUID.String(),
		"event_type", event.Type,
		"status_code", resp.StatusCode,
	)

	return deliveryResult{statusCode: resp.StatusCode, responseBody: respBodyStr, requestBody: string(jsonData)}
}

// buildPayload converts an Event entity to a webhook Payload
func (s *Service) buildPayload(event *entities.Event) *Payload {
	payload := &Payload{
		EventID:   event.UUID.String(),
		EventType: event.Type,
		TaskHash:  event.TaskHash,
		OldValue:  event.OldValue,
		NewValue:  event.NewValue,
		Metadata:  event.Metadata,
		Timestamp: event.CreatedAt.Format(time.RFC3339),
	}
	if event.WorkerID != uuid.Nil {
		payload.WorkerID = event.WorkerID.String()
	}
	return payload
}

// SetEnabled enables or disables the webhook service
func (s *Service) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Enabled returns whether the webhook service is enabled
func (s *Service) Enabled() bool {
	return s.enabled
}

// saveHistory saves webhook delivery attempt to history
func (s *Service) saveHistory(ctx context.Context, event *entities.Event, statusCode int, responseBody, requestBody, errorMsg string) {
	if s.historyRepo == nil {
		return
	}

	// Extract task name and status from event metadata
	taskName := ""
	taskStatus := event.NewValue
	if event.Metadata != nil {
		if name, ok := event.Metadata["name"].(string); ok {
			taskName = name
		}
	}
	if taskName == "" {
		// Fallback so history entries are never blank-named
		taskName = event.TaskHash
	}

	history := entities.WebhookHistory{
		UUID:         uuid.New(),
		WebhookID:    s.webhookID,
		TaskHash:     event.TaskHash,
		TaskName:     taskName,
		TaskStatus:   taskStatus,
		StatusCode:   statusCode,
		ResponseBody: responseBody,
		RequestBody:  requestBody,
		Error:        errorMsg,
	}

	if _, err := s.historyRepo.CreateWebhookHistory(ctx, history); err != nil {
		logger.Error("Failed to save webhook history",
			"service", "webhook",
			"error", err,
		)
	}
}
