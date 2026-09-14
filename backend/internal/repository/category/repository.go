package category

import (
	"context"
	"errors"
	"strings"

	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/gorm"
)

type Repository struct {
	db *database.Database
}

func NewRepository(db *database.Database) *Repository {
	return &Repository{
		db: db,
	}
}

// CreateCategory inserts a new category into the database
func (r *Repository) CreateCategory(ctx context.Context, category entities.Category) (*entities.Category, error) {
	model := &models.Category{
		ID:                 category.ID,
		Name:               category.Name,
		DefaultTags:        models.StringArray(category.DefaultTags),
		DefaultDirectories: models.StringArray(category.DefaultDirectories),
		MetadataSource:     category.MetadataSource,
		ReleaseType:        category.ReleaseType,
		Color:              category.Color,
		Icon:               category.Icon,
	}

	if err := r.db.DB.WithContext(ctx).Create(model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, errors.New("category already exists")
		}

		return nil, err
	}

	return toCategory(*model), nil
}

// ListCategories retrieves all categories from the database
func (r *Repository) ListCategories(ctx context.Context) ([]*entities.Category, error) {
	var models []models.Category
	if err := r.db.DB.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}

	result := make([]*entities.Category, len(models))
	for i, item := range models {
		result[i] = toCategory(item)
	}

	return result, nil
}

// GetCategoryByID retrieves a single category by its ID
func (r *Repository) GetCategoryByID(ctx context.Context, id string) (*entities.Category, error) {
	var model models.Category
	if err := r.db.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}

	return toCategory(model), nil
}

// GetCategoryByName retrieves a single category by its name
func (r *Repository) GetCategoryByName(ctx context.Context, name string) (*entities.Category, error) {
	var model models.Category
	if err := r.db.DB.WithContext(ctx).Where("name = ?", name).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}

	return toCategory(model), nil
}

// UpdateCategory updates an existing category in the database
// Note: Name and ID are immutable and will not be updated
func (r *Repository) UpdateCategory(ctx context.Context, category entities.Category) (*entities.Category, error) {
	// Only update mutable fields
	updates := map[string]interface{}{
		"default_tags":        models.StringArray(category.DefaultTags),
		"default_directories": models.StringArray(category.DefaultDirectories),
		"metadata_source":     category.MetadataSource,
		"release_type":        category.ReleaseType,
		"color":               category.Color,
		"icon":                category.Icon,
	}

	if err := r.db.DB.WithContext(ctx).Model(&models.Category{}).Where("id = ?", category.ID).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}

	return r.GetCategoryByID(ctx, category.ID)
}

// DeleteCategory removes a category from the database by ID
func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	result := r.db.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.Category{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("category not found")
	}

	return nil
}

// toCategory converts a models.Category to entities.Category
func toCategory(model models.Category) *entities.Category {
	return &entities.Category{
		ID:                 model.ID,
		Name:               model.Name,
		DefaultTags:        []string(model.DefaultTags),
		DefaultDirectories: []string(model.DefaultDirectories),
		MetadataSource:     model.MetadataSource,
		ReleaseType:        model.ReleaseType,
		Color:              model.Color,
		Icon:               model.Icon,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}
