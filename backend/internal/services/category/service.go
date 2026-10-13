package category

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"github.com/jfxdev/gardarr/internal/repository/category"
)

var ErrInvalidPreferredWorker = errors.New("preferred worker must be an existing worker UUID")

type Service struct {
	db         *database.Database
	repository *category.Repository
}

func NewService(db *database.Database) *Service {
	return &Service{
		db:         db,
		repository: category.NewRepository(db),
	}
}

// CreateCategory creates a new category
func (s *Service) CreateCategory(ctx context.Context, cat entities.Category) (*entities.Category, error) {
	if err := s.validatePreferredWorker(ctx, cat.PreferredWorkerID); err != nil {
		return nil, err
	}
	if cat.PreferredWorkerID != "" {
		cat.PreferredWorkerID = uuid.MustParse(cat.PreferredWorkerID).String()
	}
	return s.repository.CreateCategory(ctx, cat)
}

// ListCategories retrieves all categories
func (s *Service) ListCategories(ctx context.Context) ([]*entities.Category, error) {
	return s.repository.ListCategories(ctx)
}

// GetCategoryByID retrieves a category by its ID
func (s *Service) GetCategoryByID(ctx context.Context, id string) (*entities.Category, error) {
	return s.repository.GetCategoryByID(ctx, id)
}

// GetCategoryByName retrieves a category by its name
func (s *Service) GetCategoryByName(ctx context.Context, name string) (*entities.Category, error) {
	return s.repository.GetCategoryByName(ctx, name)
}

// UpdateCategory updates an existing category
func (s *Service) UpdateCategory(ctx context.Context, cat entities.Category) (*entities.Category, error) {
	if err := s.validatePreferredWorker(ctx, cat.PreferredWorkerID); err != nil {
		return nil, err
	}
	if cat.PreferredWorkerID != "" {
		cat.PreferredWorkerID = uuid.MustParse(cat.PreferredWorkerID).String()
	}
	return s.repository.UpdateCategory(ctx, cat)
}

// DeleteCategory removes a category by ID
func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	return s.repository.DeleteCategory(ctx, id)
}

// Validate the stored registration only; offline workers remain valid preferences.
func (s *Service) validatePreferredWorker(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := uuid.Validate(id); err != nil {
		return ErrInvalidPreferredWorker
	}
	parsed, _ := uuid.Parse(id)
	var count int64
	if err := s.db.DB.WithContext(ctx).Model(&models.Worker{}).Where("uuid = ?", parsed).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrInvalidPreferredWorker
	}
	return nil
}
