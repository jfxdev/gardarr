package workers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jfxdev/gardarr/internal/mappers"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/pkg/errors"
)

func (m *Module) getWorkerOverview(c *gin.Context) {
	result, err := m.service.Overview(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	response := models.WorkerOverviewResponse{
		Worker:       mappers.ToWorkerResponse(result.Worker),
		CollectedAt:  result.CollectedAt,
		MetricsError: result.MetricsError,
	}
	if stats := result.Stats; stats != nil {
		response.Metrics = models.WorkerOverviewMetrics{
			StoredBytes:    stats.StoredBytes,
			TotalTorrents:  &stats.TotalTasksCount,
			ActiveTorrents: &stats.ActiveTasksCount,
			DownloadSpeed:  &stats.CurrentDownloadSpeed,
			UploadSpeed:    &stats.CurrentUploadSpeed,
		}
	}
	c.JSON(http.StatusOK, response)
}
