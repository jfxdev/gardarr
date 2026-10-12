package workers

import (
	stdErrors "errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/mappers"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/services/bandwidthscheduler"
	"github.com/jfxdev/gardarr/internal/services/workermanager"
	"github.com/jfxdev/gardarr/pkg/errors"
	"github.com/jfxdev/gardarr/pkg/logger"
)

const invalidScheduleIDError = "Invalid schedule ID"
const taskTrackersPath = "/:id/task/:task_id/trackers"

// isValidTrackerURL mirrors the go-playground/validator "url" binding tag
// used on the JSON tracker schemas, for the DELETE route's query params
// (which bypass ShouldBindJSON and so aren't covered by struct tags).
func isValidTrackerURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	return parsed.Host != "" || parsed.Opaque != ""
}

// Module holds worker routes configuration
type Module struct {
	db                 *database.Database
	service            *workermanager.Service
	bandwidthScheduler *bandwidthscheduler.Service
	workersRouter      *gin.RouterGroup
	workerRouter       *gin.RouterGroup
}

func NewModule(router *gin.RouterGroup, db *database.Database, svc *workermanager.Service, bandwidthScheduler *bandwidthscheduler.Service) *Module {
	return &Module{
		db:                 db,
		service:            svc,
		bandwidthScheduler: bandwidthScheduler,
		workersRouter:      router.Group("/workers"),
		workerRouter:       router.Group("/worker"),
	}
}

func (m Module) Register() {
	m.workersRouter.Use(middlewares.SessionMiddleware(m.db))
	m.workerRouter.Use(middlewares.SessionMiddleware(m.db))

	m.workersRouter.GET("/", m.listWorkers)
	m.workersRouter.POST("/tasks/bulk", m.bulkWorkerTaskAction)
	m.workersRouter.GET("/rss/feeds", m.listAllRSSFeeds)
	m.workersRouter.GET("/rss/rules", m.listAllRSSRules)

	// Worker lifecycle and instance-wide configuration require admin privileges.
	adminWorker := m.workerRouter.Group("")
	adminWorker.Use(middlewares.RequireAdminRole())
	adminWorker.POST("/", m.createWorker)
	adminWorker.PUT("/:id", m.updateWorker)
	adminWorker.DELETE("/:id", m.deleteWorker)
	adminWorker.POST("/:id/speed/limits", m.setWorkerSpeedLimits)
	adminWorker.POST("/:id/active/limits", m.setWorkerMaxActiveTorrents)
	adminWorker.POST("/:id/peers/ban", m.banWorkerPeer)

	m.workerRouter.GET("/:id", m.getWorker)
	m.workerRouter.GET("/:id/overview", m.getWorkerOverview)

	m.workerRouter.GET("/:id/schedules", m.listSchedules)
	m.workerRouter.POST("/:id/schedules", m.createSchedule)
	m.workerRouter.GET("/:id/schedules/preview", m.previewSchedule)
	m.workerRouter.PUT("/:id/schedules/order", m.reorderSchedules)
	m.workerRouter.GET("/:id/schedules/:schedule_id", m.getSchedule)
	m.workerRouter.PUT("/:id/schedules/:schedule_id", m.updateSchedule)
	m.workerRouter.DELETE("/:id/schedules/:schedule_id", m.deleteSchedule)

	m.workerRouter.GET("/:id/version", m.getWorkerVersion)
	m.workerRouter.GET("/:id/preferences", m.getWorkerPreferences)
	m.workerRouter.GET("/:id/logs", m.getWorkerLogs)
	m.workerRouter.POST("/:id/task", m.createWorkerTask)
	m.workerRouter.POST("/:id/task/file", m.createWorkerTaskFromFile)
	m.workerRouter.GET("/:id/task/:task_id", m.getWorkerTask)
	m.workerRouter.DELETE("/:id/task/:task_id", m.deleteWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/stop", m.stopWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/start", m.startWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/force_download", m.forceDownloadWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/force_resume", m.forceResumeWorkerTask)
	m.workerRouter.PUT("/:id/task/:task_id/share_limit", m.setWorkerTaskShareLimit)
	m.workerRouter.POST("/:id/task/:task_id/location", m.setWorkerTaskLocation)
	m.workerRouter.POST("/:id/task/:task_id/rename", m.renameWorkerTask)
	m.workerRouter.GET("/:id/task/:task_id/limits", m.getWorkerTaskLimits)
	m.workerRouter.POST("/:id/task/:task_id/super_seeding", m.setWorkerTaskSuperSeeding)
	m.workerRouter.POST("/:id/task/:task_id/force_recheck", m.forceRecheckWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/force_reannounce", m.forceReannounceWorkerTask)
	m.workerRouter.POST("/:id/task/:task_id/limit_download_rate", m.setWorkerTaskDownloadLimit)
	m.workerRouter.POST("/:id/task/:task_id/limit_upload_rate", m.setWorkerTaskUploadLimit)
	m.workerRouter.PUT("/:id/task/:task_id/tags", m.setWorkerTaskTags)
	m.workerRouter.PUT("/:id/task/:task_id/category", m.setWorkerTaskCategory)
	m.workerRouter.GET("/:id/task/:task_id/files", m.listWorkerTaskFiles)
	m.workerRouter.PUT("/:id/task/:task_id/files/priority", m.setWorkerTaskFilePriority)
	m.workerRouter.PUT("/:id/task/:task_id/priority", m.setWorkerTaskQueuePriority)
	m.workerRouter.GET("/:id/task/:task_id/peers", m.listWorkerTaskPeers)
	m.workerRouter.GET(taskTrackersPath, m.listWorkerTaskTrackers)
	m.workerRouter.POST(taskTrackersPath, m.addWorkerTaskTrackers)
	m.workerRouter.DELETE(taskTrackersPath, m.removeWorkerTaskTrackers)
	m.workerRouter.PUT(taskTrackersPath, m.editWorkerTaskTracker)

	m.workerRouter.GET("/:id/rss/feeds", m.listWorkerRSSFeeds)
	m.workerRouter.POST("/:id/rss/feeds", m.addWorkerRSSFeed)
	m.workerRouter.DELETE("/:id/rss/feeds", m.removeWorkerRSSFeed)
	m.workerRouter.PUT("/:id/rss/feeds/url", m.setWorkerRSSFeedURL)
	m.workerRouter.POST("/:id/rss/folders", m.addWorkerRSSFolder)
	m.workerRouter.POST("/:id/rss/items/move", m.moveWorkerRSSItem)
	m.workerRouter.POST("/:id/rss/items/refresh", m.refreshWorkerRSSItem)
	m.workerRouter.POST("/:id/rss/items/mark_read", m.markWorkerRSSItemAsRead)
	m.workerRouter.GET("/:id/rss/rules", m.listWorkerRSSRules)
	m.workerRouter.PUT("/:id/rss/rules/:rule_name", m.setWorkerRSSRule)
	m.workerRouter.POST("/:id/rss/rules/:rule_name/rename", m.renameWorkerRSSRule)
	m.workerRouter.DELETE("/:id/rss/rules/:rule_name", m.removeWorkerRSSRule)
	m.workerRouter.GET("/:id/rss/rules/:rule_name/matching_articles", m.getWorkerRSSMatchingArticles)
}

func (m *Module) createWorker(c *gin.Context) {
	var body schemas.WorkerCreateSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	result, err := m.service.CreateWorker(c.Request.Context(), &body)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToWorkerResponse(result))
}

func (m *Module) listWorkers(c *gin.Context) {
	// live=false skips the per-worker qBittorrent connectivity/instance
	// check and returns DB rows only (fast path for the /workers page's
	// initial render). Defaults to true so every existing caller keeps
	// getting real status unless it explicitly opts out.
	live := c.DefaultQuery("live", "true") != "false"

	var result []*entities.Worker
	var err error
	if live {
		result, err = m.service.ListWorkers()
	} else {
		result, err = m.service.ListWorkersBasic()
	}
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	resp := make([]*models.WorkerResponse, len(result))
	ids := make([]uuid.UUID, 0, len(result))
	for _, item := range result {
		ids = append(ids, item.UUID)
	}
	statuses := map[uuid.UUID]bandwidthscheduler.Status{}
	if m.bandwidthScheduler != nil {
		if resolved, statusErr := m.bandwidthScheduler.Statuses(c.Request.Context(), ids); statusErr == nil {
			statuses = resolved
		} else {
			logger.Debug("bandwidth scheduler: status lookup failed", "error", statusErr.Error())
		}
	}
	for i, item := range result {
		resp[i] = mappers.ToWorkerResponse(item)
		if status, ok := statuses[item.UUID]; ok && status.Active && status.Schedule != nil && status.Limits != nil {
			d, u := status.Limits.DownloadLimit, status.Limits.UploadLimit
			resp[i].BandwidthScheduleStatus = &models.BandwidthScheduleStatusResponse{Active: true, Name: status.Schedule.Name, DownloadLimit: &d, UploadLimit: &u}
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (m *Module) getWorker(c *gin.Context) {
	id := c.Param("id")

	result, err := m.service.GetWorker(c.Request.Context(), id)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToWorkerResponse(result))
}

func (m *Module) getWorkerPreferences(c *gin.Context) {
	id := c.Param("id")

	// First get the worker to ensure it exists
	worker, err := m.service.GetWorker(c.Request.Context(), id)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	// Then get the worker preferences
	preferences, err := m.service.GetPreferences(c.Request.Context(), worker)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToInstancePreferencesResponse(preferences))
}

func (m *Module) updateWorker(c *gin.Context) {
	id := c.Param("id")

	var body schemas.WorkerUpdateSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	result, err := m.service.UpdateWorker(c.Request.Context(), id, &body)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToWorkerResponse(result))
}

func (m *Module) deleteWorker(c *gin.Context) {
	id := c.Param("id")

	if err := m.service.DeleteWorker(c.Request.Context(), id); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// bulkWorkerTaskAction applies one action to many tasks, possibly spanning
// multiple workers. Returns 200 with per-worker partial failures in the body.
func (m *Module) bulkWorkerTaskAction(c *gin.Context) {
	var body schemas.BulkTaskActionSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	result, err := m.service.BulkTaskAction(c.Request.Context(), body)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (m *Module) createWorkerTask(c *gin.Context) {
	id := c.Param("id")

	var body schemas.TaskCreateSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	result, err := m.service.CreateWorkerTask(c.Request.Context(), id, body)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskResponse(result))
}

// maxTorrentFileSize bounds uploaded .torrent files (metadata only; real
// torrents rarely exceed a few hundred KB).
const maxTorrentFileSize = 5 << 20 // 5MB

// createWorkerTaskFromFile adds a torrent from an uploaded .torrent file
// (multipart form: "torrent" file part + category/directory/tags fields).
func (m *Module) createWorkerTaskFromFile(c *gin.Context) {
	id := c.Param("id")

	var body schemas.TaskCreateFromFileSchema
	if err := c.ShouldBind(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	fileHeader, err := c.FormFile("torrent")
	if err != nil {
		respErr := errors.NewBadRequestError("Missing torrent file", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if fileHeader.Size > maxTorrentFileSize {
		respErr := errors.NewBadRequestError("Torrent file too large (max 5MB)", nil)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".torrent") {
		respErr := errors.NewBadRequestError("File must have .torrent extension", nil)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		respErr := errors.NewBadRequestError("Failed to read torrent file", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(io.LimitReader(file, maxTorrentFileSize+1))
	if err != nil {
		respErr := errors.NewBadRequestError("Failed to read torrent file", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}
	if len(fileData) > maxTorrentFileSize {
		respErr := errors.NewBadRequestError("Torrent file too large (max 5MB)", nil)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	result, err := m.service.CreateWorkerTaskFromFile(c.Request.Context(), id, fileHeader.Filename, fileData, body)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskResponse(result))
}

func (m *Module) stopWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.StopWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task stopped successfully"})
}

func (m *Module) startWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.StartWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task started successfully"})
}

func (m *Module) forceDownloadWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.ForceDownloadWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task force download initiated successfully"})
}

func (m *Module) getWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	result, err := m.service.GetWorkerTask(c.Request.Context(), workerID, taskID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskResponse(result))
}

func (m *Module) deleteWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	// Get purge parameter from query string
	purge := c.DefaultQuery("purge", "false") == "true"

	if err := m.service.DeleteWorkerTask(c.Request.Context(), workerID, taskID, purge); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (m *Module) forceResumeWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.ForceResumeWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task force resumed successfully"})
}

func (m *Module) setWorkerTaskShareLimit(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetShareLimitSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskShareLimit(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task share limit set successfully"})
}

func (m *Module) setWorkerTaskLocation(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetLocationSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskLocation(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task location set successfully"})
}

func (m *Module) renameWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskRenameSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.RenameWorkerTask(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task renamed successfully"})
}

func (m *Module) setWorkerTaskSuperSeeding(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSuperSeedingSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskSuperSeeding(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task super seeding mode set successfully"})
}

func (m *Module) forceRecheckWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.ForceRecheckWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task force recheck initiated successfully"})
}

func (m *Module) forceReannounceWorkerTask(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	if err := m.service.ForceReannounceWorkerTask(c.Request.Context(), workerID, taskID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task force reannounce initiated successfully"})
}

func (m *Module) setWorkerTaskDownloadLimit(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetDownloadLimitSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskDownloadLimit(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task download limit set successfully"})
}

func (m *Module) setWorkerTaskUploadLimit(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetUploadLimitSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskUploadLimit(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task upload limit set successfully"})
}

func (m *Module) listWorkerTaskFiles(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	files, err := m.service.ListWorkerTaskFiles(c.Request.Context(), workerID, taskID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskFilesResponse(files))
}

func (m *Module) setWorkerTaskFilePriority(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetFilePrioritySchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskFilePriority(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task file priority set successfully"})
}

func (m *Module) listWorkerTaskTrackers(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	trackers, err := m.service.ListWorkerTaskTrackers(c.Request.Context(), workerID, taskID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskTrackersResponse(trackers))
}

func (m *Module) addWorkerTaskTrackers(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskAddTrackersSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.AddWorkerTaskTrackers(c.Request.Context(), workerID, taskID, body.Urls); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trackers added successfully"})
}

// removeWorkerTaskTrackers accepts the tracker URLs to remove as repeated
// query params (?url=a&url=b) rather than a body, matching this route
// group's convention for DELETE requests.
func (m *Module) removeWorkerTaskTrackers(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	urls := c.QueryArray("url")
	if len(urls) == 0 {
		respErr := errors.NewBadRequestError("at least one url query param is required", nil)
		c.JSON(respErr.StatusCode, respErr)
		return
	}
	for _, trackerURL := range urls {
		if !isValidTrackerURL(trackerURL) {
			respErr := errors.NewBadRequestError("invalid tracker url: "+trackerURL, nil)
			c.JSON(respErr.StatusCode, respErr)
			return
		}
	}

	if err := m.service.RemoveWorkerTaskTrackers(c.Request.Context(), workerID, taskID, urls); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// editWorkerTaskTracker swaps a tracker's URL in place, preserving
// per-tracker state (like tier) that a remove+add pair would lose.
func (m *Module) editWorkerTaskTracker(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskEditTrackerSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.EditWorkerTaskTracker(c.Request.Context(), workerID, taskID, body.OrigURL, body.NewURL); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tracker updated successfully"})
}

func (m *Module) setWorkerTaskTags(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetTagsSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskTags(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task tags updated successfully"})
}

func (m *Module) setWorkerTaskCategory(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetCategorySchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskCategory(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task category updated successfully"})
}

func (m *Module) setWorkerTaskQueuePriority(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	var body schemas.TaskSetQueuePrioritySchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerTaskQueuePriority(c.Request.Context(), workerID, taskID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task queue priority updated successfully"})
}

func (m *Module) listWorkerTaskPeers(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	peers, err := m.service.ListWorkerTaskPeers(c.Request.Context(), workerID, taskID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskPeersResponse(peers))
}

// banWorkerPeer bans a peer instance-wide - this affects every torrent on
// the worker's qBittorrent instance, not just the task the peer was found
// on. It's an admin-only action (see adminWorker group in Register).
func (m *Module) banWorkerPeer(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.WorkerBanPeerSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.BanWorkerPeer(c.Request.Context(), workerID, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Peer banned successfully"})
}

func (m *Module) getWorkerVersion(c *gin.Context) {
	workerID := c.Param("id")

	version, err := m.service.GetWorkerVersion(c.Request.Context(), workerID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToWorkerVersionResponse(version))
}

func (m *Module) getWorkerTaskLimits(c *gin.Context) {
	workerID := c.Param("id")
	taskID := c.Param("task_id")

	worker, err := m.service.GetWorker(c.Request.Context(), workerID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	limits, err := m.service.GetWorkerTaskLimits(c.Request.Context(), worker, taskID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToTaskLimitsResponse(limits))
}

func (m *Module) setWorkerMaxActiveTorrents(c *gin.Context) {
	var body schemas.InstanceSetMaxActiveTorrentLimitsSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	workerID := c.Param("id")
	worker, err := m.service.GetWorker(c.Request.Context(), workerID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	if err := m.service.SetWorkerGlobalActiveLimits(c.Request.Context(), worker, body); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Instance max active torrents set successfully"})
}

func (m *Module) setWorkerSpeedLimits(c *gin.Context) {
	var body schemas.InstanceSetSpeedLimitSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	workerID := c.Param("id")
	worker, err := m.service.GetWorker(c.Request.Context(), workerID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	if m.bandwidthScheduler != nil {
		err = m.bandwidthScheduler.ApplyManualDefault(c.Request.Context(), worker, bandwidthscheduler.Limits{DownloadLimit: *body.DownloadLimit, UploadLimit: *body.UploadLimit})
	} else {
		err = m.service.SetWorkerGlobalSpeedLimits(c.Request.Context(), worker, body)
	}
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Instance speed limits set successfully"})
}

func (m *Module) listSchedules(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	items, err := m.bandwidthScheduler.List(c.Request.Context(), worker.UUID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (m *Module) createSchedule(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	var input bandwidthscheduler.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	item, err := m.bandwidthScheduler.Create(c.Request.Context(), worker, input)
	if err != nil {
		m.handleScheduleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (m *Module) getSchedule(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	id, ok := m.scheduleID(c)
	if !ok {
		return
	}
	item, err := m.bandwidthScheduler.Get(c.Request.Context(), worker.UUID, id)
	if err != nil {
		m.handleScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (m *Module) updateSchedule(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	var input bandwidthscheduler.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	id, ok := m.scheduleID(c)
	if !ok {
		return
	}
	item, err := m.bandwidthScheduler.Update(c.Request.Context(), worker, id, input)
	if err != nil {
		m.handleScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (m *Module) deleteSchedule(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	id, ok := m.scheduleID(c)
	if !ok {
		return
	}
	if err := m.bandwidthScheduler.Delete(c.Request.Context(), worker, id); err != nil {
		m.handleScheduleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (m *Module) reorderSchedules(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	var input bandwidthscheduler.OrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	items, err := m.bandwidthScheduler.Reorder(c.Request.Context(), worker, input)
	if err != nil {
		m.handleScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (m *Module) previewSchedule(c *gin.Context) {
	if !m.requireBandwidthScheduler(c) {
		return
	}
	worker, err := m.service.GetWorker(c.Request.Context(), c.Param("id"))
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	preview, err := m.bandwidthScheduler.Preview(c.Request.Context(), worker.UUID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (m *Module) requireBandwidthScheduler(c *gin.Context) bool {
	if m.bandwidthScheduler != nil {
		return true
	}
	respErr := errors.NewServiceUnavailableError("Bandwidth scheduler is unavailable", nil)
	c.JSON(respErr.StatusCode, respErr)
	return false
}

func (m *Module) scheduleID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("schedule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidScheduleIDError})
		return uuid.Nil, false
	}
	return id, true
}

func (m *Module) handleScheduleError(c *gin.Context, err error) {
	switch {
	case stdErrors.Is(err, bandwidthscheduler.ErrScheduleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case stdErrors.Is(err, bandwidthscheduler.ErrPriorityConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case stdErrors.Is(err, bandwidthscheduler.ErrScheduleLimit):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case stdErrors.Is(err, bandwidthscheduler.ErrInvalidScheduleOrder):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		errors.HandleError(c, err)
	}
}

func (m *Module) getWorkerLogs(c *gin.Context) {
	workerID := c.Param("id")

	normal := c.Query("normal") == "true"
	info := c.Query("info") == "true"
	warning := c.Query("warning") == "true"
	critical := c.Query("critical") == "true"

	lastKnownID := 0
	if idStr := c.Query("last_known_id"); idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil {
			lastKnownID = id
		}
	}

	result, err := m.service.GetWorkerLogs(c.Request.Context(), workerID, normal, info, warning, critical, lastKnownID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// listAllRSSFeeds aggregates feeds across every registered worker - the
// cross-instance view, tagged per feed with its worker_id.
func (m *Module) listAllRSSFeeds(c *gin.Context) {
	result, err := m.service.ListAllRSSFeeds(c.Request.Context())
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"feeds":  mappers.ToRSSFeedListResponse(result.Feeds),
		"errors": result.Errors,
	})
}

// listAllRSSRules aggregates auto-downloading rules across every registered
// worker - the cross-instance view, tagged per rule with its worker_id.
func (m *Module) listAllRSSRules(c *gin.Context) {
	result, err := m.service.ListAllRSSRules(c.Request.Context())
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rules":  mappers.ToRSSRuleListResponse(result.Rules),
		"errors": result.Errors,
	})
}

// listWorkerRSSFeeds lists every feed registered on the instance.
// with_data=true also includes each feed's articles.
func (m *Module) listWorkerRSSFeeds(c *gin.Context) {
	workerID := c.Param("id")
	withData := c.Query("with_data") == "true"

	feeds, err := m.service.ListWorkerRSSFeeds(c.Request.Context(), workerID, withData)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToRSSFeedsResponse(feeds))
}

func (m *Module) addWorkerRSSFeed(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSAddFeedSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.AddWorkerRSSFeed(c.Request.Context(), workerID, body.URL, body.Path); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS feed added successfully"})
}

func (m *Module) removeWorkerRSSFeed(c *gin.Context) {
	workerID := c.Param("id")
	path := c.Query("path")
	if path == "" {
		respErr := errors.NewBadRequestError("path is required", nil)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.RemoveWorkerRSSFeed(c.Request.Context(), workerID, path); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// setWorkerRSSFeedURL edits a feed's URL in place. Requires qBittorrent
// 4.6.0+ (WebAPI v2.9.1+); on older servers, remove and re-add the feed instead.
func (m *Module) setWorkerRSSFeedURL(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSSetFeedURLSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.SetWorkerRSSFeedURL(c.Request.Context(), workerID, body.Path, body.URL); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS feed URL updated successfully"})
}

func (m *Module) addWorkerRSSFolder(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSAddFolderSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.AddWorkerRSSFolder(c.Request.Context(), workerID, body.Path); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS folder added successfully"})
}

// moveWorkerRSSItem moves or renames a feed/folder - qBittorrent uses the
// same endpoint for both.
func (m *Module) moveWorkerRSSItem(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSMoveItemSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.MoveWorkerRSSItem(c.Request.Context(), workerID, body.ItemPath, body.DestPath); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS item moved successfully"})
}

// refreshWorkerRSSItem forces an immediate refresh instead of waiting for
// qBittorrent's own poll interval.
func (m *Module) refreshWorkerRSSItem(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSRefreshItemSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.RefreshWorkerRSSItem(c.Request.Context(), workerID, body.ItemPath); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS item refresh requested successfully"})
}

func (m *Module) markWorkerRSSItemAsRead(c *gin.Context) {
	workerID := c.Param("id")

	var body schemas.RSSMarkAsReadSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.MarkWorkerRSSItemAsRead(c.Request.Context(), workerID, body.ItemPath, body.ArticleID); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS item marked as read successfully"})
}

func (m *Module) listWorkerRSSRules(c *gin.Context) {
	workerID := c.Param("id")

	rules, err := m.service.ListWorkerRSSRules(c.Request.Context(), workerID)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mappers.ToRSSRulesResponse(rules))
}

// setWorkerRSSRule creates or updates an auto-downloading rule - qBittorrent's
// API doesn't distinguish the two.
func (m *Module) setWorkerRSSRule(c *gin.Context) {
	workerID := c.Param("id")
	ruleName := c.Param("rule_name")

	var body schemas.RSSSetRuleSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	rule := entities.RSSRule{
		Enabled:              body.Enabled,
		MustContain:          body.MustContain,
		MustNotContain:       body.MustNotContain,
		UseRegex:             body.UseRegex,
		EpisodeFilter:        body.EpisodeFilter,
		SmartFilter:          body.SmartFilter,
		AffectedFeeds:        body.AffectedFeeds,
		IgnoreDays:           body.IgnoreDays,
		AddPaused:            body.AddPaused,
		AssignedCategory:     body.AssignedCategory,
		SavePath:             body.SavePath,
		TorrentContentLayout: body.TorrentContentLayout,
	}

	if err := m.service.SetWorkerRSSRule(c.Request.Context(), workerID, ruleName, rule); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS rule saved successfully"})
}

func (m *Module) renameWorkerRSSRule(c *gin.Context) {
	workerID := c.Param("id")
	ruleName := c.Param("rule_name")

	var body schemas.RSSRenameRuleSchema
	if err := c.ShouldBindJSON(&body); err != nil {
		respErr := errors.NewBadRequestError("Invalid request body", err)
		c.JSON(respErr.StatusCode, respErr)
		return
	}

	if err := m.service.RenameWorkerRSSRule(c.Request.Context(), workerID, ruleName, body.NewRuleName); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSS rule renamed successfully"})
}

func (m *Module) removeWorkerRSSRule(c *gin.Context) {
	workerID := c.Param("id")
	ruleName := c.Param("rule_name")

	if err := m.service.RemoveWorkerRSSRule(c.Request.Context(), workerID, ruleName); err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// getWorkerRSSMatchingArticles previews which currently-known articles a
// rule would match, grouped by feed - lets the caller test a rule before
// saving it.
func (m *Module) getWorkerRSSMatchingArticles(c *gin.Context) {
	workerID := c.Param("id")
	ruleName := c.Param("rule_name")

	matches, err := m.service.GetWorkerRSSMatchingArticles(c.Request.Context(), workerID, ruleName)
	if err != nil {
		errors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, matches)
}
