package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/services/crypto"
	instanceservice "github.com/jfxdev/gardarr/internal/services/instance/worker"
	rssservice "github.com/jfxdev/gardarr/internal/services/rss/worker"
	taskservice "github.com/jfxdev/gardarr/internal/services/task/worker"
	"github.com/jfxdev/gardarr/pkg/env"
	"github.com/jfxdev/gardarr/pkg/logger"
	"github.com/jfxdev/go-qbt"
	"gorm.io/gorm"
)

type Repository struct {
	db              *database.Database
	crypto          *crypto.CryptoService
	http            *http.Client
	timeout         time.Duration
	maxRetries      int
	loginMaxRetries int
	retryBackoff    time.Duration

	clientMu    sync.Mutex
	clientCache map[uuid.UUID]*cachedClient
}

// cachedClient keeps a live *qbt.Client alive across calls for a given worker
// so we don't spin up a fresh cookie-cleanup goroutine and re-login on every
// single request. It's invalidated (and the old client closed) whenever the
// worker's resolved credentials change.
type cachedClient struct {
	client      *qbt.Client
	fingerprint string
	// loginMu serializes explicit Login calls against this shared client,
	// since qbt.Client's Login isn't safe to invoke concurrently (it writes
	// lastLoginTime unlocked) and multiple callers (poller, manual
	// availability/version checks) can race for the same worker.
	loginMu sync.Mutex
}

const (
	StatusConnected    = qbt.StatusConnected
	StatusUnauthorized = qbt.StatusUnauthorized
	StatusUnaccessible = qbt.StatusUnaccessible
)

func NewRepository(db *database.Database, crypto *crypto.CryptoService) (*Repository, error) {
	timeoutSeconds := constants.DefaultWorkerTimeoutSeconds
	if timeoutEnv := env.Get(constants.WorkerTimeoutSecondsEnv); timeoutEnv.Value() != "" {
		if parsedTimeout, err := strconv.Atoi(timeoutEnv.Value()); err == nil && parsedTimeout > 0 {
			timeoutSeconds = parsedTimeout
		}
	}

	return &Repository{
		db:              db,
		crypto:          crypto,
		http:            &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		timeout:         time.Duration(timeoutSeconds) * time.Second,
		maxRetries:      env.Get(constants.QBittorrentMaxRetriesEnv).Default(0).ValueInt(),
		loginMaxRetries: env.Get(constants.QBittorrentLoginMaxRetriesEnv).Default(5).ValueInt(),
		retryBackoff:    time.Duration(env.Get(constants.QBittorrentRetryBackoffEnv).Default(1).ValueInt()) * time.Second,
		clientCache:     make(map[uuid.UUID]*cachedClient),
	}, nil
}

// WorkerError represents a structured error with classification for direct
// qBittorrent connections registered as workers.
type WorkerError struct {
	Code      entities.WorkerErrorCode
	Message   string
	Permanent bool
}

func (e *WorkerError) Error() string {
	return e.Message
}

func classifyWorkerError(err error) *WorkerError {
	if err == nil {
		return nil
	}

	clientErr := qbt.ClassifyError(err)

	return &WorkerError{
		Code:      mapQbtErrorCode(clientErr.Code),
		Message:   clientErr.Message,
		Permanent: clientErr.Permanent,
	}
}

func mapQbtErrorCode(code qbt.ErrorCode) entities.WorkerErrorCode {
	switch code {
	case qbt.ErrorCodeAuthFailure:
		return entities.WorkerErrorCodeAuthFailure
	case qbt.ErrorCodeTimeout:
		return entities.WorkerErrorCodeTimeout
	case qbt.ErrorCodeDNS:
		return entities.WorkerErrorCodeDNS
	case qbt.ErrorCodeHTTPSRequired:
		return entities.WorkerErrorCodeHTTPSRequired
	case qbt.ErrorCodeSSLError:
		return entities.WorkerErrorCodeSSLError
	case qbt.ErrorCodeVersionIncompatible:
		return entities.WorkerErrorCodeVersionIncompatible
	case qbt.ErrorCodeConnectionRefused:
		return entities.WorkerErrorCodeConnectionRefused
	case qbt.ErrorCodeNetworkUnreachable:
		return entities.WorkerErrorCodeNetworkUnreachable
	case qbt.ErrorCodeBadGateway:
		return entities.WorkerErrorCodeBadGateway
	case qbt.ErrorCodeServiceUnavailable:
		return entities.WorkerErrorCodeServiceUnavailable
	case qbt.ErrorCodeUnknown:
		return entities.WorkerErrorCodeUnknown
	default:
		return entities.WorkerErrorCodeWorkerUnreachable
	}
}

func (r *Repository) CreateWorker(ctx context.Context, worker entities.Worker) (*entities.Worker, error) {
	db := r.db.DB.WithContext(ctx)

	encURL, err := r.crypto.Encrypt(worker.QBittorrentURL)
	if err != nil {
		return nil, err
	}

	encUsername, err := r.crypto.Encrypt(worker.QBittorrentUsername)
	if err != nil {
		return nil, err
	}

	encPassword, err := r.crypto.Encrypt(worker.QBittorrentPassword)
	if err != nil {
		return nil, err
	}

	handler := &models.Worker{
		Name:                         worker.Name,
		Type:                         worker.Type,
		Address:                      worker.Address,
		EncryptedQBittorrentURL:      encURL,
		EncryptedQBittorrentUsername: encUsername,
		EncryptedQBittorrentPassword: encPassword,
		Icon:                         worker.Icon,
		Color:                        worker.Color,
		DefaultDownloadSpeedLimit:    worker.DefaultDownloadSpeedLimit,
		DefaultUploadSpeedLimit:      worker.DefaultUploadSpeedLimit,
	}

	if err := db.Create(handler).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("worker already exists")
		}
		return nil, err
	}

	return toWorker(*handler), nil
}

func (r *Repository) ListWorkers() ([]*entities.Worker, error) {
	var handler []models.Worker
	if err := r.db.DB.Find(&handler).Error; err != nil {
		return nil, err
	}

	result := make([]*entities.Worker, len(handler))
	for i, item := range handler {
		result[i] = toWorker(item)
	}

	return result, nil
}

func (r *Repository) GetWorkerByUUID(uid uuid.UUID) (*entities.Worker, error) {
	var handler models.Worker
	if err := r.db.DB.Where("uuid = ?", uid).First(&handler).Error; err != nil {
		return nil, err
	}

	return toWorker(handler), nil
}

func (r *Repository) CheckWorkerAvailability(worker *entities.Worker) error {
	entry, err := r.getClientEntry(worker)
	if err != nil {
		return err
	}
	client := entry.client

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	entry.loginMu.Lock()
	err = client.Login(ctx)
	entry.loginMu.Unlock()
	if err != nil {
		workerErr := classifyWorkerError(err)
		logger.Error("worker connection failed",
			"worker", worker.Name,
			"address", worker.Address,
			"error_code", workerErr.Code,
			"error", err.Error(),
		)
		return workerErr
	}

	if version, err := client.GetAppVersion(); err != nil {
		workerErr := classifyWorkerError(err)
		logger.Error("worker availability check failed",
			"worker", worker.Name,
			"address", worker.Address,
			"error_code", workerErr.Code,
			"error", err.Error(),
		)
		return workerErr
	} else if version == "" {
		return &WorkerError{
			Code:      entities.WorkerErrorCodeUnknown,
			Message:   "failed to get app version",
			Permanent: false,
		}
	}

	return nil
}

func (r *Repository) GetInstance(worker *entities.Worker) (*entities.Instance, error) {
	if err := r.CheckWorkerAvailability(worker); err != nil {
		return nil, err
	}

	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return instanceservice.New(client).GetInstance(context.Background())
}

func (r *Repository) GetInstanceWithoutDecrypt(worker *entities.Worker) (*entities.Instance, error) {
	client, err := r.newClient(worker.QBittorrentURL, worker.QBittorrentUsername, worker.QBittorrentPassword)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		return nil, classifyWorkerError(err)
	}

	return instanceservice.New(client).GetInstance(context.Background())
}

func (r *Repository) GetWorkerVersion(worker *entities.Worker) (*entities.WorkerVersion, error) {
	entry, err := r.getClientEntry(worker)
	if err != nil {
		return nil, err
	}
	client := entry.client

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	entry.loginMu.Lock()
	err = client.Login(ctx)
	entry.loginMu.Unlock()
	if err != nil {
		return nil, classifyWorkerError(err)
	}

	version, err := client.GetAppVersion()
	if err != nil {
		return nil, classifyWorkerError(err)
	}

	urlValue, err := r.resolveWorkerURL(worker)
	if err != nil {
		return nil, err
	}

	return &entities.WorkerVersion{
		Version:        version,
		Commit:         "",
		Date:           "",
		QbittorrentURL: urlValue,
	}, nil
}

func (r *Repository) GetWorkerTasksStats(worker *entities.Worker) (*entities.TaskStats, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).GetTasksStats(context.Background())
}

func (r *Repository) GetWorkerPreferences(worker *entities.Worker) (*entities.InstancePreferences, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return instanceservice.New(client).GetPreferences(context.Background())
}

func (r *Repository) UpdateWorker(ctx context.Context, uid uuid.UUID, updates map[string]interface{}) (*entities.Worker, error) {
	db := r.db.DB.WithContext(ctx)

	var worker models.Worker
	if err := db.Where("uuid = ?", uid).First(&worker).Error; err != nil {
		return nil, err
	}

	mapped := make(map[string]interface{}, len(updates))
	for key, value := range updates {
		switch key {
		case "url":
			urlValue, ok := value.(string)
			if !ok {
				continue
			}
			encURL, err := r.crypto.Encrypt(urlValue)
			if err != nil {
				return nil, err
			}
			mapped["address"] = urlValue
			mapped["encrypted_q_bittorrent_url"] = encURL
		case "username":
			username, ok := value.(string)
			if !ok {
				continue
			}
			encUsername, err := r.crypto.Encrypt(username)
			if err != nil {
				return nil, err
			}
			mapped["encrypted_q_bittorrent_username"] = encUsername
		case "password":
			password, ok := value.(string)
			if !ok {
				continue
			}
			encPassword, err := r.crypto.Encrypt(password)
			if err != nil {
				return nil, err
			}
			mapped["encrypted_q_bittorrent_password"] = encPassword
		default:
			mapped[key] = value
		}
	}

	if err := db.Model(&worker).Updates(mapped).Error; err != nil {
		return nil, err
	}

	if err := db.Where("uuid = ?", uid).First(&worker).Error; err != nil {
		return nil, err
	}

	return toWorker(worker), nil
}

func (r *Repository) DeleteWorker(uid uuid.UUID) error {
	if err := r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Category{}).Where("preferred_worker_id = ?", uid.String()).Update("preferred_worker_id", "").Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uid).Delete(&models.Worker{}).Error
	}); err != nil {
		return err
	}

	r.evictClient(uid)
	return nil
}

func (r *Repository) GetWorkerLogs(worker *entities.Worker, normal, info, warning, critical bool, lastKnownID int) ([]*qbt.LogEntry, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return instanceservice.New(client).GetLogs(context.Background(), normal, info, warning, critical, lastKnownID)
}

func (r *Repository) ListWorkerRSSFeeds(worker *entities.Worker, withData bool) (map[string]*entities.RSSFeed, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return rssservice.New(client).ListFeeds(context.Background(), withData)
}

func (r *Repository) AddWorkerRSSFeed(worker *entities.Worker, feedURL, path string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).AddFeed(context.Background(), feedURL, path)
}

func (r *Repository) RemoveWorkerRSSFeed(worker *entities.Worker, path string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).RemoveFeed(context.Background(), path)
}

func (r *Repository) SetWorkerRSSFeedURL(worker *entities.Worker, path, feedURL string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).SetFeedURL(context.Background(), path, feedURL)
}

func (r *Repository) AddWorkerRSSFolder(worker *entities.Worker, path string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).AddFolder(context.Background(), path)
}

func (r *Repository) MoveWorkerRSSItem(worker *entities.Worker, itemPath, destPath string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).MoveItem(context.Background(), itemPath, destPath)
}

func (r *Repository) RefreshWorkerRSSItem(worker *entities.Worker, itemPath string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).RefreshItem(context.Background(), itemPath)
}

func (r *Repository) MarkWorkerRSSItemAsRead(worker *entities.Worker, itemPath, articleID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).MarkAsRead(context.Background(), itemPath, articleID)
}

func (r *Repository) ListWorkerRSSRules(worker *entities.Worker) (map[string]*entities.RSSRule, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return rssservice.New(client).ListRules(context.Background())
}

func (r *Repository) SetWorkerRSSRule(worker *entities.Worker, ruleName string, rule entities.RSSRule) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).SetRule(context.Background(), ruleName, rule)
}

func (r *Repository) RenameWorkerRSSRule(worker *entities.Worker, ruleName, newRuleName string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).RenameRule(context.Background(), ruleName, newRuleName)
}

func (r *Repository) RemoveWorkerRSSRule(worker *entities.Worker, ruleName string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return rssservice.New(client).RemoveRule(context.Background(), ruleName)
}

func (r *Repository) GetWorkerRSSMatchingArticles(worker *entities.Worker, ruleName string) (map[string][]string, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return rssservice.New(client).MatchingArticles(context.Background(), ruleName)
}

func (r *Repository) ListWorkerTasks(worker *entities.Worker) ([]*entities.Task, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).ListTasks(context.Background())
}

func (r *Repository) CreateWorkerTask(worker *entities.Worker, schema schemas.TaskCreateSchema) (*entities.Task, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).CreateTask(context.Background(), schema)
}

func (r *Repository) CreateWorkerTaskFromFile(worker *entities.Worker, fileName string, fileData []byte, schema schemas.TaskCreateFromFileSchema) (*entities.Task, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).CreateTaskFromFile(context.Background(), fileName, fileData, schema)
}

func (r *Repository) DeleteWorkerTask(worker *entities.Worker, id string, purge bool) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).DeleteTask(context.Background(), id, purge)
}

func (r *Repository) StopWorkerTask(worker *entities.Worker, taskID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).StopTask(context.Background(), taskID)
}

func (r *Repository) StartWorkerTask(worker *entities.Worker, taskID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).StartTask(context.Background(), taskID)
}

func (r *Repository) ForceDownloadWorkerTask(worker *entities.Worker, taskID string) error {
	return r.ForceResumeWorkerTask(worker, taskID)
}

func (r *Repository) GetWorkerTask(worker *entities.Worker, taskID string) (*entities.Task, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).GetTask(context.Background(), taskID)
}

func (r *Repository) ForceResumeWorkerTask(worker *entities.Worker, taskID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).ForceResumeTask(context.Background(), taskID)
}

func (r *Repository) SetWorkerTaskShareLimit(worker *entities.Worker, taskID string, schema schemas.TaskSetShareLimitSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskShareLimit(context.Background(), taskID, schema)
}

func (r *Repository) SetWorkerTaskLocation(worker *entities.Worker, taskID string, schema schemas.TaskSetLocationSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskLocation(context.Background(), taskID, schema)
}

func (r *Repository) RenameWorkerTask(worker *entities.Worker, taskID string, schema schemas.TaskRenameSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).RenameTask(context.Background(), taskID, schema)
}

func (r *Repository) SetWorkerTaskSuperSeeding(worker *entities.Worker, taskID string, schema schemas.TaskSuperSeedingSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskSuperSeeding(context.Background(), taskID, schema)
}

func (r *Repository) ForceRecheckWorkerTask(worker *entities.Worker, taskID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).ForceRecheckTask(context.Background(), taskID)
}

func (r *Repository) ForceReannounceWorkerTask(worker *entities.Worker, taskID string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).ForceReannounceTask(context.Background(), taskID)
}

func (r *Repository) SetWorkerTaskDownloadLimit(worker *entities.Worker, taskID string, schema schemas.TaskSetDownloadLimitSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskDownloadLimit(context.Background(), taskID, schema)
}

func (r *Repository) SetWorkerTaskUploadLimit(worker *entities.Worker, taskID string, schema schemas.TaskSetUploadLimitSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskUploadLimit(context.Background(), taskID, schema)
}

func (r *Repository) ListWorkerTaskFiles(worker *entities.Worker, taskID string) ([]*entities.TaskFile, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).ListTaskFiles(context.Background(), taskID)
}

func (r *Repository) SetWorkerTaskFilePriority(worker *entities.Worker, taskID string, schema schemas.TaskSetFilePrioritySchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetFilePriority(context.Background(), taskID, schema)
}

func (r *Repository) ListWorkerTaskTrackers(worker *entities.Worker, taskID string) ([]*entities.TaskTracker, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).ListTaskTrackers(context.Background(), taskID)
}

func (r *Repository) AddWorkerTaskTrackers(worker *entities.Worker, taskID string, urls []string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).AddTaskTrackers(context.Background(), taskID, urls)
}

func (r *Repository) RemoveWorkerTaskTrackers(worker *entities.Worker, taskID string, urls []string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).RemoveTaskTrackers(context.Background(), taskID, urls)
}

func (r *Repository) EditWorkerTaskTracker(worker *entities.Worker, taskID, origURL, newURL string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).EditTaskTracker(context.Background(), taskID, origURL, newURL)
}

func (r *Repository) SetWorkerTaskTags(worker *entities.Worker, taskID string, schema schemas.TaskSetTagsSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskTags(context.Background(), taskID, schema)
}

// AddWorkerTaskTags adds tags to one or more tasks (taskID may be a
// "|"-separated hash batch) without the per-hash diff SetWorkerTaskTags does.
func (r *Repository) AddWorkerTaskTags(worker *entities.Worker, taskID string, tags []string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).AddTaskTags(context.Background(), taskID, tags)
}

// CreateWorkerTags creates one or more tags on the worker's qBittorrent
// server without attaching them to any torrent.
func (r *Repository) CreateWorkerTags(worker *entities.Worker, tags []string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return client.CreateTags(tags)
}

// DeleteWorkerTags removes one or more tags from the worker's qBittorrent
// server, detaching them from every torrent that carries them.
func (r *Repository) DeleteWorkerTags(worker *entities.Worker, tags []string) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return client.DeleteTags(tags)
}

func (r *Repository) SetWorkerTaskCategory(worker *entities.Worker, taskID string, schema schemas.TaskSetCategorySchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskCategory(context.Background(), taskID, schema)
}

func (r *Repository) SetWorkerTaskQueuePriority(worker *entities.Worker, taskID string, schema schemas.TaskSetQueuePrioritySchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return taskservice.New(client).SetTaskQueuePriority(context.Background(), taskID, schema)
}

func (r *Repository) ListWorkerTaskPeers(worker *entities.Worker, taskID string) ([]*entities.TaskPeer, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).ListTaskPeers(context.Background(), taskID)
}

// BanWorkerPeer bans a peer instance-wide on the given worker's qBittorrent
// server - it affects every torrent on that instance, not just one task.
func (r *Repository) BanWorkerPeer(worker *entities.Worker, ip string, port int) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return client.BanPeers([]string{fmt.Sprintf("%s:%d", ip, port)})
}

func (r *Repository) GetWorkerTaskLimits(worker *entities.Worker, taskID string) (*entities.TaskLimits, error) {
	client, err := r.getClient(worker)
	if err != nil {
		return nil, err
	}

	return taskservice.New(client).GetTaskLimits(context.Background(), taskID)
}

func (r *Repository) SetWorkerGlobalSpeedLimits(worker *entities.Worker, schema schemas.InstanceSetSpeedLimitSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return instanceservice.New(client).SetSpeedLimit(context.Background(), schema)
}

func (r *Repository) SetWorkerGlobalActiveLimits(worker *entities.Worker, schema schemas.InstanceSetMaxActiveTorrentLimitsSchema) error {
	client, err := r.getClient(worker)
	if err != nil {
		return err
	}

	return instanceservice.New(client).SetMaxActiveTorrentLimits(context.Background(), schema)
}

func (r *Repository) getClient(worker *entities.Worker) (*qbt.Client, error) {
	entry, err := r.getClientEntry(worker)
	if err != nil {
		return nil, err
	}
	return entry.client, nil
}

// getClientEntry returns the cached client entry for a worker, including its
// per-client loginMu, so callers that invoke Login explicitly can serialize
// against other concurrent callers sharing the same client.
func (r *Repository) getClientEntry(worker *entities.Worker) (*cachedClient, error) {
	urlValue, err := r.resolveWorkerURL(worker)
	if err != nil {
		return nil, err
	}

	username, err := r.resolveWorkerCredential(worker.QBittorrentUsername)
	if err != nil {
		return nil, err
	}

	password, err := r.resolveWorkerCredential(worker.QBittorrentPassword)
	if err != nil {
		return nil, err
	}

	fingerprint := urlValue + "|" + username + "|" + password

	r.clientMu.Lock()
	defer r.clientMu.Unlock()

	if cached, ok := r.clientCache[worker.UUID]; ok {
		if cached.fingerprint == fingerprint {
			return cached, nil
		}
		// Credentials changed (or stale cache) - abort any in-flight
		// requests still using the old creds immediately, then retire the
		// old client without blocking the caller on its logout round trip.
		cached.client.Cancel()
		go cached.client.Close()
		delete(r.clientCache, worker.UUID)
	}

	client, err := r.newClient(urlValue, username, password)
	if err != nil {
		return nil, err
	}

	entry := &cachedClient{client: client, fingerprint: fingerprint}
	r.clientCache[worker.UUID] = entry
	return entry, nil
}

// evictClient removes and closes any cached client for the given worker. Call
// this whenever a worker is deleted so its background goroutines are stopped.
func (r *Repository) evictClient(uid uuid.UUID) {
	r.clientMu.Lock()
	cached, ok := r.clientCache[uid]
	if ok {
		delete(r.clientCache, uid)
	}
	r.clientMu.Unlock()

	if ok {
		// Abort any in-flight requests immediately, then best-effort logout
		// in the background - the caller must not wait on the round trip.
		cached.client.Cancel()
		go cached.client.Close()
	}
}

func (r *Repository) newClient(urlValue, username, password string) (*qbt.Client, error) {
	client, err := qbt.New(qbt.Config{
		BaseURL:         urlValue,
		Username:        username,
		Password:        password,
		RequestTimeout:  r.timeout,
		MaxRetries:      r.maxRetries,
		RetryBackoff:    r.retryBackoff,
		MaxLoginRetries: r.loginMaxRetries,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (r *Repository) resolveWorkerURL(worker *entities.Worker) (string, error) {
	if worker.QBittorrentURL != "" {
		return r.resolveWorkerCredential(worker.QBittorrentURL)
	}
	if worker.Address != "" {
		return worker.Address, nil
	}
	return "", fmt.Errorf("worker does not have a configured qBittorrent URL")
}

func (r *Repository) resolveWorkerCredential(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	decrypted, err := r.crypto.Decrypt(value)
	if err != nil {
		return "", err
	}

	return decrypted, nil
}

func toWorker(item models.Worker) *entities.Worker {
	return &entities.Worker{
		UUID:                      item.UUID,
		Name:                      item.Name,
		Type:                      item.Type,
		Address:                   item.Address,
		QBittorrentURL:            item.EncryptedQBittorrentURL,
		QBittorrentUsername:       item.EncryptedQBittorrentUsername,
		QBittorrentPassword:       item.EncryptedQBittorrentPassword,
		Icon:                      item.Icon,
		Color:                     item.Color,
		DefaultDownloadSpeedLimit: item.DefaultDownloadSpeedLimit,
		DefaultUploadSpeedLimit:   item.DefaultUploadSpeedLimit,
	}
}
