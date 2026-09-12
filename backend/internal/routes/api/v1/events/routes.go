package events

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/mappers"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/services/events"
)

// Module holds events routes configuration
type Module struct {
	group        *gin.RouterGroup
	eventService *events.Service
	db           *database.Database
}

// NewModule creates a new events module
func NewModule(router *gin.RouterGroup, db *database.Database) (*Module, error) {
	eventService, err := events.NewService(db)
	if err != nil {
		return nil, err
	}

	return &Module{
		group:        router.Group("/events"),
		eventService: eventService,
		db:           db,
	}, nil
}

// Register registers all events routes
func (m *Module) Register() {
	// Protected routes - require authentication
	protected := m.group.Group("")
	protected.Use(middlewares.SessionMiddleware(m.db))
	protected.GET("/", m.listEvents)
	protected.GET("/:uuid", m.getEvent)
}

// listEvents retrieves events with optional filters
func (m *Module) listEvents(c *gin.Context) {
	// Parse query parameters
	workerIDStr := c.Query("worker_id")
	eventType := c.Query("type")
	group := c.Query("group")
	search := c.Query("search")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // Max limit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var workerID *uuid.UUID
	if workerIDStr != "" {
		if parsed, err := uuid.Parse(workerIDStr); err == nil {
			workerID = &parsed
		}
	}

	// An explicit type takes precedence over group; otherwise group expands
	// to its full set of types (e.g. all worker.* types for the History
	// page's worker table). Neither set means no type filter at all.
	var eventTypes []string
	switch {
	case eventType != "":
		eventTypes = []string{eventType}
	case group == "worker":
		eventTypes = constants.WorkerEventTypes
	case group == "torrent":
		eventTypes = constants.TorrentEventTypes
	case group == "schedule":
		eventTypes = constants.ScheduleEventTypes
	case group == "report":
		eventTypes = constants.ReportEventTypes
	}

	// Get events from service
	eventsList, total, err := m.eventService.ListEvents(c.Request.Context(), workerID, eventTypes, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve events",
		})
		return
	}

	// Convert to response
	eventResponses := make([]*models.EventResponse, len(eventsList))
	for i, event := range eventsList {
		eventResponses[i] = mappers.ToEventResponse(event)
	}

	resp := models.ListEventsResponse{
		Events: eventResponses,
		Total:  int(total),
	}

	c.JSON(http.StatusOK, resp)
}

// getEvent retrieves a single event by UUID
func (m *Module) getEvent(c *gin.Context) {
	uuidStr := c.Param("uuid")
	if uuidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "UUID is required",
		})
		return
	}

	eventUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid UUID format",
		})
		return
	}

	event, err := m.eventService.GetEventByUUID(c.Request.Context(), eventUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Event not found",
		})
		return
	}

	c.JSON(http.StatusOK, mappers.ToEventResponse(event))
}
