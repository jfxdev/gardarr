package reports

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/schemas"
	discordservice "github.com/jfxdev/gardarr/internal/services/discord"
	transferreport "github.com/jfxdev/gardarr/internal/services/transferreport"
)

type discordDelivery interface {
	Deliver(context.Context, *entities.Event) (int, error)
}

type Module struct {
	group   *gin.RouterGroup
	db      *database.Database
	service *transferreport.Service
	discord discordDelivery
}

func NewModule(router *gin.RouterGroup, db *database.Database, service *transferreport.Service, discord discordDelivery) *Module {
	return &Module{group: router.Group("/reports/transfer"), db: db, service: service, discord: discord}
}

func (m *Module) Register() {
	protected := m.group.Group("")
	protected.Use(middlewares.SessionMiddleware(m.db))
	protected.GET("/settings", m.getSettings)
	protected.PUT("/settings", m.updateSettings)
	protected.GET("/latest", m.getLatest)
	protected.GET("/current", m.getCurrent)
	protected.POST("/snapshot", m.captureSnapshot)
	protected.POST("/send-discord", m.sendDiscord)
}

func (m *Module) getSettings(c *gin.Context) {
	settings, err := m.service.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve transfer report settings"})
		return
	}
	c.JSON(http.StatusOK, settingsResponse(settings))
}

func (m *Module) updateSettings(c *gin.Context) {
	var body schemas.TransferReportSettingsRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}
	settings, err := m.service.UpdateSettings(c.Request.Context(), entities.TransferReportSettings{Enabled: body.Enabled, SnapshotsPerDay: body.SnapshotsPerDay, DailyReportTime: body.DailyReportTime, WeeklyReportDay: body.WeeklyReportDay, WeeklyReportTime: body.WeeklyReportTime, TopN: body.TopN})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settingsResponse(settings))
}

func (m *Module) getLatest(c *gin.Context) {
	daily, weekly, err := m.service.GetLatest(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve transfer reports"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"daily": reportResponse(daily), "weekly": reportResponse(weekly)})
}

func (m *Module) getCurrent(c *gin.Context) {
	daily, weekly, err := m.service.GetCurrent(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve current transfer rankings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"daily": reportResponse(daily), "weekly": reportResponse(weekly)})
}

func (m *Module) captureSnapshot(c *gin.Context) {
	if err := m.service.CaptureNow(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to capture transfer snapshot"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (m *Module) sendDiscord(c *gin.Context) {
	if m.discord == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Discord delivery is unavailable"})
		return
	}
	var body schemas.TransferReportDiscordSendRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}
	report, inProgress, err := m.service.GetNotificationReport(c.Request.Context(), body.Source, body.PeriodType)
	if errors.Is(err, transferreport.ErrReportNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transfer report not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare transfer report notification"})
		return
	}
	delivered, err := m.discord.Deliver(c.Request.Context(), transferreport.BuildNotificationEvent(report, inProgress))
	if errors.Is(err, discordservice.ErrNoMatchingDestinations) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No matching enabled Discord destinations"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Could not send Discord notification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivered": delivered})
}

func settingsResponse(settings *entities.TransferReportSettings) gin.H {
	return gin.H{"enabled": settings.Enabled, "snapshots_per_day": settings.SnapshotsPerDay, "daily_report_time": settings.DailyReportTime, "weekly_report_day": settings.WeeklyReportDay, "weekly_report_time": settings.WeeklyReportTime, "top_n": settings.TopN, "updated_at": settings.UpdatedAt}
}

func reportResponse(report *entities.TransferReport) interface{} {
	if report == nil {
		return nil
	}
	return gin.H{"uuid": report.UUID.String(), "period_type": report.PeriodType, "period_start": report.PeriodStart, "period_end": report.PeriodEnd, "timezone": report.Timezone, "generated_at": report.GeneratedAt, "coverage": report.Coverage, "unavailable_workers": report.UnavailableWorkers, "upload": report.Upload, "download": report.Download}
}
