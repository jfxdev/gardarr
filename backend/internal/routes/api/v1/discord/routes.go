package discord

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/middlewares"
	discordrepo "github.com/jfxdev/gardarr/internal/repository/discord"
	"github.com/jfxdev/gardarr/internal/schemas"
	discordservice "github.com/jfxdev/gardarr/internal/services/discord"
)

type Module struct {
	group   *gin.RouterGroup
	db      *database.Database
	service *discordservice.Service
}

func NewModule(router *gin.RouterGroup, db *database.Database, service *discordservice.Service) *Module {
	return &Module{group: router.Group("/integrations/discord"), db: db, service: service}
}
func (m *Module) Register() {
	protected := m.group.Group("")
	protected.Use(middlewares.SessionMiddleware(m.db))
	protected.POST("", m.create)
	protected.GET("", m.list)
	protected.GET("/:id", m.get)
	protected.PUT("/:id", m.update)
	protected.DELETE("/:id", m.delete)
	protected.POST("/:id/test", m.test)
	protected.GET("/:id/history", m.history)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Discord integration ID"})
		return uuid.Nil, false
	}
	return id, true
}
func createInput(body schemas.DiscordCreateRequest) discordservice.Input {
	enabled, all := true, true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	if body.AllEvents != nil {
		all = *body.AllEvents
	}
	return discordservice.Input{Name: body.Name, WebhookURL: body.WebhookURL, Enabled: enabled, AllEvents: all, EventTypes: body.EventTypes, StatusFilter: body.StatusFilter, CategoryFilter: body.CategoryFilter, NameTerms: body.NameTerms}
}

func (m *Module) create(c *gin.Context) {
	var body schemas.DiscordCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}
	created, err := m.service.Create(c.Request.Context(), createInput(body))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response(created))
}
func (m *Module) list(c *gin.Context) {
	values, err := m.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list Discord integrations"})
		return
	}
	out := make([]gin.H, 0, len(values))
	for _, value := range values {
		out = append(out, response(value))
	}
	c.JSON(http.StatusOK, out)
}
func (m *Module) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	value, err := m.service.Get(c.Request.Context(), id)
	if errors.Is(err, discordrepo.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response(value))
}
func (m *Module) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body schemas.DiscordUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}
	value, err := m.service.Update(c.Request.Context(), id, discordservice.UpdateInput{Name: body.Name, WebhookURL: body.WebhookURL, Enabled: body.Enabled, AllEvents: body.AllEvents, EventTypes: body.EventTypes, StatusFilter: body.StatusFilter, CategoryFilter: body.CategoryFilter, NameTerms: body.NameTerms})
	if errors.Is(err, discordrepo.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response(value))
}
func (m *Module) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	err := m.service.Delete(c.Request.Context(), id)
	if errors.Is(err, discordrepo.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
func (m *Module) test(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := m.service.Test(c.Request.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, discordrepo.ErrNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func (m *Module) history(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	values, total, err := m.service.History(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve Discord delivery history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": values, "total": total, "limit": limit, "offset": offset})
}
func response(value *entities.DiscordIntegration) gin.H {
	return gin.H{"uuid": value.UUID.String(), "name": value.Name, "webhook_configured": value.EncryptedWebhookURL != "", "enabled": value.Enabled, "all_events": value.AllEvents, "event_types": value.EventTypes, "status_filter": value.StatusFilter, "category_filter": value.CategoryFilter, "name_terms": value.NameTerms, "created_at": value.CreatedAt, "updated_at": value.UpdatedAt}
}
