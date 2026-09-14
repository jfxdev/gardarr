package mappers

import (
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/models"
)

func ToWorkerResponse(e *entities.Worker) *models.WorkerResponse {
	if e == nil {
		return &models.WorkerResponse{}
	}

	return &models.WorkerResponse{
		UUID:      e.UUID.String(),
		Name:      e.Name,
		Address:   e.Address,
		Status:    e.Status,
		Error:     e.Error,
		ErrorCode: string(e.ErrorCode),
		Permanent: e.Permanent,
		Icon:      e.Icon,
		Color:     e.Color,
		Instance:  ToInstanceResponse(e.Instance),
	}
}

func ToWorkerVersion(m models.WorkerVersionResponse) *entities.WorkerVersion {
	return &entities.WorkerVersion{
		Version:        m.Version,
		Commit:         m.Commit,
		Date:           m.Date,
		QbittorrentURL: m.QbittorrentURL,
	}
}

func ToWorkerVersionResponse(e *entities.WorkerVersion) models.WorkerVersionResponse {
	if e == nil {
		return models.WorkerVersionResponse{}
	}

	return models.WorkerVersionResponse{
		Version:        e.Version,
		Commit:         e.Commit,
		Date:           e.Date,
		QbittorrentURL: e.QbittorrentURL,
	}
}
