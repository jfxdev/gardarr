package discord

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	cryptoService "github.com/jfxdev/gardarr/internal/services/crypto"
	discordservice "github.com/jfxdev/gardarr/internal/services/discord"
)

const routeDiscordKey = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="

func discordContext(t *testing.T, method, body, id string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: id}}
	return ctx, writer
}

func testDiscordModule(t *testing.T) (*Module, *discordservice.Service) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", routeDiscordKey)
	crypto, err := cryptoService.NewCryptoService()
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	db := database.SetupTestDB(t, &models.DiscordIntegration{}, &models.DiscordDeliveryHistory{})
	service := discordservice.NewService(make(chan *entities.Event), db, crypto, nil)
	module := NewModule(gin.New().Group("/v1"), db, service)
	module.Register()
	return module, service
}

func TestDiscordHandlersCreateListUpdateAndDeleteWithoutLeakingURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	module, _ := testDiscordModule(t)
	ctx, writer := discordContext(t, http.MethodPost, `{"name":"Alerts","webhook_url":"https://discord.com/api/webhooks/123/token"}`, "")
	module.create(ctx)
	if writer.Code != http.StatusCreated || bytes.Contains(writer.Body.Bytes(), []byte("token")) || !bytes.Contains(writer.Body.Bytes(), []byte(`"webhook_configured":true`)) {
		t.Fatalf("create response leaked/failed: %d %s", writer.Code, writer.Body.String())
	}
	var created struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &created); err != nil || created.UUID == "" {
		t.Fatalf("decode created response: %v %#v", err, created)
	}

	ctx, writer = discordContext(t, http.MethodGet, "", "")
	module.list(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte("Alerts")) {
		t.Fatalf("list response: %d %s", writer.Code, writer.Body.String())
	}
	ctx, writer = discordContext(t, http.MethodGet, "", "not-a-uuid")
	module.get(ctx)
	if writer.Code != http.StatusBadRequest {
		t.Fatalf("invalid id should be bad request: %d", writer.Code)
	}
	ctx, writer = discordContext(t, http.MethodPut, `{"name":"Updated","all_events":false,"event_types":["report.transfer.daily"]}`, created.UUID)
	module.update(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte("Updated")) {
		t.Fatalf("update response: %d %s", writer.Code, writer.Body.String())
	}
	ctx, writer = discordContext(t, http.MethodGet, "", created.UUID)
	module.history(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte(`"total":0`)) {
		t.Fatalf("history response: %d %s", writer.Code, writer.Body.String())
	}
	ctx, writer = discordContext(t, http.MethodDelete, "", created.UUID)
	module.delete(ctx)
	if ctx.Writer.Status() != http.StatusNoContent {
		t.Fatalf("delete response: %d %s", ctx.Writer.Status(), writer.Body.String())
	}
}

func TestDiscordRouteHelpersUseSafeDefaultsAndResponses(t *testing.T) {
	input := createInput(schemas.DiscordCreateRequest{})
	if !input.Enabled || !input.AllEvents {
		t.Fatalf("optional booleans should default true: %#v", input)
	}
	value := &entities.DiscordIntegration{UUID: uuid.New(), Name: "safe", EncryptedWebhookURL: "ciphertext", Enabled: true, AllEvents: true}
	encoded, err := json.Marshal(response(value))
	if err != nil || bytes.Contains(encoded, []byte("ciphertext")) || !bytes.Contains(encoded, []byte(`"webhook_configured":true`)) {
		t.Fatalf("unsafe response: %s err=%v", encoded, err)
	}
	ctx, writer := discordContext(t, http.MethodPost, "", uuid.New().String())
	module, _ := testDiscordModule(t)
	module.test(ctx)
	if writer.Code != http.StatusNotFound {
		t.Fatalf("missing destination should be not found: %d", writer.Code)
	}
}
