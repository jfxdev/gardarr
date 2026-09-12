package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	transferreport "github.com/jfxdev/gardarr/internal/services/transferreport"
)

type routeWorkers struct{}

func (routeWorkers) ListWorkersBasic() ([]*entities.Worker, error) { return nil, nil }
func (routeWorkers) ListTasks(context.Context, []*entities.Worker) (*entities.TaskListResult, error) {
	return &entities.TaskListResult{}, nil
}

type routeTimezone struct{}

func (routeTimezone) GetTimezone(context.Context) (string, error) { return "UTC", nil }

type routeDiscordDelivery struct {
	event     *entities.Event
	delivered int
	err       error
}

func (f *routeDiscordDelivery) Deliver(_ context.Context, event *entities.Event) (int, error) {
	f.event = event
	return f.delivered, f.err
}

func reportsContext(t *testing.T, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, writer
}

func TestReportHandlersExposeSettingsAndLatestReports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := database.SetupTestDB(t, &models.TransferReportSettings{}, &models.TransferSnapshotRun{}, &models.TransferSnapshot{}, &models.TransferReport{})
	service := transferreport.NewService(db, routeWorkers{}, routeTimezone{}, nil)
	delivery := &routeDiscordDelivery{delivered: 2}
	module := NewModule(gin.New().Group("/v1"), db, service, delivery)
	module.Register()

	ctx, writer := reportsContext(t, http.MethodGet, "")
	module.getSettings(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte(`"snapshots_per_day":4`)) {
		t.Fatalf("settings response: %d %s", writer.Code, writer.Body.String())
	}

	ctx, writer = reportsContext(t, http.MethodPut, `{}`)
	module.updateSettings(ctx)
	if writer.Code != http.StatusBadRequest {
		t.Fatalf("expected binding error, got %d: %s", writer.Code, writer.Body.String())
	}
	ctx, writer = reportsContext(t, http.MethodPut, `{"enabled":true,"snapshots_per_day":5,"daily_report_time":"00:05","weekly_report_day":1,"weekly_report_time":"00:10","top_n":10}`)
	module.updateSettings(ctx)
	if writer.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected validation error, got %d: %s", writer.Code, writer.Body.String())
	}
	ctx, writer = reportsContext(t, http.MethodPut, `{"enabled":false,"snapshots_per_day":6,"daily_report_time":"01:05","weekly_report_day":0,"weekly_report_time":"02:10","top_n":3}`)
	module.updateSettings(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte(`"snapshots_per_day":6`)) {
		t.Fatalf("updated settings response: %d %s", writer.Code, writer.Body.String())
	}

	ctx, writer = reportsContext(t, http.MethodGet, "")
	module.getLatest(ctx)
	if writer.Code != http.StatusOK || writer.Body.String() != `{"daily":null,"weekly":null}` {
		t.Fatalf("latest response: %d %s", writer.Code, writer.Body.String())
	}

	ctx, writer = reportsContext(t, http.MethodGet, "")
	module.getCurrent(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte(`"daily"`)) {
		t.Fatalf("current response: %d %s", writer.Code, writer.Body.String())
	}

	ctx, writer = reportsContext(t, http.MethodPost, "")
	module.captureSnapshot(ctx)
	if ctx.Writer.Status() != http.StatusNoContent {
		t.Fatalf("manual snapshot response: %d %s", ctx.Writer.Status(), writer.Body.String())
	}

	ctx, writer = reportsContext(t, http.MethodPost, `{"source":"current","period_type":"daily"}`)
	module.sendDiscord(ctx)
	if writer.Code != http.StatusOK || !bytes.Contains(writer.Body.Bytes(), []byte(`"delivered":2`)) || delivery.event == nil || delivery.event.Type != "report.transfer.daily" || delivery.event.Metadata["in_progress"] != true {
		t.Fatalf("manual Discord response: %d %s event=%#v", writer.Code, writer.Body.String(), delivery.event)
	}
}

func TestReportResponseIncludesRankingPayload(t *testing.T) {
	if reportResponse(nil) != nil {
		t.Fatal("nil report must remain null")
	}
	report := &entities.TransferReport{UUID: uuid.New(), PeriodType: entities.TransferReportPeriodDaily, Timezone: "UTC", Coverage: "partial", UnavailableWorkers: []string{"worker-a"}, Upload: []entities.TransferRankItem{{Rank: 1, Name: "Alpha", Hash: "a", Bytes: 42}}}
	encoded, err := json.Marshal(reportResponse(report))
	if err != nil || !bytes.Contains(encoded, []byte(`"unavailable_workers":["worker-a"]`)) || !bytes.Contains(encoded, []byte(`"bytes":42`)) {
		t.Fatalf("report payload did not retain ranking/coverage: %s err=%v", encoded, err)
	}
}
