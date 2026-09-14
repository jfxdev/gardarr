package category

import (
	"context"
	"testing"

	"github.com/jfxdev/gardarr/internal/entities"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/models"
)

func TestRepositoryCreateCategory(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	category := entities.Category{
		Name:               "Movies",
		DefaultTags:        []string{"hd", "english"},
		DefaultDirectories: []string{"/movies", "/movies-4k"},
		MetadataSource:     "none",
	}

	created, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if created == nil {
		t.Error("Expected created category, got nil")
		return
	}

	if created.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if created.Name != category.Name {
		t.Errorf("Expected name %s, got %s", category.Name, created.Name)
	}

	if len(created.DefaultTags) != len(category.DefaultTags) {
		t.Errorf("Expected %d tags, got %d", len(category.DefaultTags), len(created.DefaultTags))
	}

	if len(created.DefaultDirectories) != len(category.DefaultDirectories) {
		t.Errorf("Expected %d default directories, got %d", len(category.DefaultDirectories), len(created.DefaultDirectories))
	}
}

func TestRepositoryCreateCategoryDuplicate(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	category := entities.Category{
		Name:               "Series",
		DefaultTags:        []string{"hd"},
		DefaultDirectories: []string{"/series"},
		MetadataSource:     "none",
	}

	// Create first category
	_, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Fatalf("Expected no error on first create, got %v", err)
	}

	// Attempt to create duplicate
	_, err = repo.CreateCategory(ctx, category)
	if err == nil {
		t.Error("Expected error for duplicate category, got nil")
	}
}

func TestRepositoryListCategories(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	// Test empty list
	categories, err := repo.ListCategories(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(categories) != 0 {
		t.Errorf("Expected 0 categories, got %d", len(categories))
	}

	// Create test categories
	testCategories := []entities.Category{
		{Name: "Movies", DefaultTags: []string{"hd"}, DefaultDirectories: []string{"/movies"}, MetadataSource: "none"},
		{Name: "Series", DefaultTags: []string{"hd"}, DefaultDirectories: []string{"/series"}, MetadataSource: "none"},
		{Name: "Music", DefaultTags: []string{"flac"}, DefaultDirectories: []string{"/music"}, MetadataSource: "none"},
	}

	for _, cat := range testCategories {
		_, err := repo.CreateCategory(ctx, cat)
		if err != nil {
			t.Fatalf("Failed to create category: %v", err)
		}
	}

	// List all categories
	categories, err = repo.ListCategories(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(categories) != len(testCategories) {
		t.Errorf("Expected %d categories, got %d", len(testCategories), len(categories))
	}
}

func TestRepositoryGetCategoryByID(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a category
	category := entities.Category{
		Name:               "Books",
		DefaultTags:        []string{"epub", "pdf"},
		DefaultDirectories: []string{"/books"},
		MetadataSource:     "none",
	}

	created, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Get by ID
	retrieved, err := repo.GetCategoryByID(ctx, created.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, retrieved.ID)
	}

	if retrieved.Name != created.Name {
		t.Errorf("Expected name %s, got %s", created.Name, retrieved.Name)
	}

	// Test non-existent ID
	_, err = repo.GetCategoryByID(ctx, "non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent ID, got nil")
	}
}

func TestRepositoryGetCategoryByName(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a category
	category := entities.Category{
		Name:               "Games",
		DefaultTags:        []string{"pc", "console"},
		DefaultDirectories: []string{"/games"},
		MetadataSource:     "none",
	}

	created, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Get by name
	retrieved, err := repo.GetCategoryByName(ctx, category.Name)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("Expected ID %s, got %s", created.ID, retrieved.ID)
	}

	if retrieved.Name != created.Name {
		t.Errorf("Expected name %s, got %s", created.Name, retrieved.Name)
	}

	// Test non-existent name
	_, err = repo.GetCategoryByName(ctx, "non-existent-name")
	if err == nil {
		t.Error("Expected error for non-existent name, got nil")
	}
}

func TestRepositoryUpdateCategory(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a category
	category := entities.Category{
		Name:               "Software",
		DefaultTags:        []string{"linux"},
		DefaultDirectories: []string{"/software"},
		MetadataSource:     "none",
		Color:              "#FF0000",
		Icon:               "code",
	}

	created, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Update the category (only mutable fields)
	created.DefaultTags = []string{"linux", "windows"}
	created.DefaultDirectories = []string{"/apps", "/applications"}
	created.Color = "#00FF00"
	created.Icon = "terminal"

	updated, err := repo.UpdateCategory(ctx, *created)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Name should remain unchanged (immutable)
	if updated.Name != "Software" {
		t.Errorf("Expected unchanged name 'Software', got %s", updated.Name)
	}

	// Test updated mutable fields
	if len(updated.DefaultTags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(updated.DefaultTags))
	}

	if len(updated.DefaultDirectories) != 2 || updated.DefaultDirectories[0] != "/apps" {
		t.Errorf("Expected updated default directories, got %v", updated.DefaultDirectories)
	}

	if updated.Color != "#00FF00" {
		t.Errorf("Expected color '#00FF00', got %s", updated.Color)
	}

	if updated.Icon != "terminal" {
		t.Errorf("Expected icon 'terminal', got %s", updated.Icon)
	}

	// Test update non-existent category
	nonExistent := entities.Category{
		ID:   "non-existent",
		Name: "Test",
	}
	_, err = repo.UpdateCategory(ctx, nonExistent)
	if err == nil {
		t.Error("Expected error for non-existent category update, got nil")
	}
}

func TestRepositoryDeleteCategory(t *testing.T) {
	db := database.SetupTestDB(t, &models.Category{})
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a category
	category := entities.Category{
		Name:               "Documents",
		DefaultTags:        []string{"pdf"},
		DefaultDirectories: []string{"/docs"},
		MetadataSource:     "none",
	}

	created, err := repo.CreateCategory(ctx, category)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Delete the category
	err = repo.DeleteCategory(ctx, created.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify deletion
	_, err = repo.GetCategoryByID(ctx, created.ID)
	if err == nil {
		t.Error("Expected error when retrieving deleted category, got nil")
	}

	// Test delete non-existent category
	err = repo.DeleteCategory(ctx, "non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent category deletion, got nil")
	}
}

func TestRepositoryToCategoryConversion(t *testing.T) {
	model := models.Category{
		ID:                 "test-id",
		Name:               "Test Category",
		DefaultTags:        models.StringArray{"tag1", "tag2"},
		DefaultDirectories: models.StringArray{"/test/path", "/test/alternate"},
		MetadataSource:     "none",
	}

	entity := toCategory(model)

	if entity.ID != model.ID {
		t.Errorf("Expected ID %s, got %s", model.ID, entity.ID)
	}

	if entity.Name != model.Name {
		t.Errorf("Expected name %s, got %s", model.Name, entity.Name)
	}

	if len(entity.DefaultTags) != len(model.DefaultTags) {
		t.Errorf("Expected %d tags, got %d", len(model.DefaultTags), len(entity.DefaultTags))
	}

	if len(entity.DefaultDirectories) != len(model.DefaultDirectories) {
		t.Errorf("Expected default directories %v, got %v", model.DefaultDirectories, entity.DefaultDirectories)
	}
}
