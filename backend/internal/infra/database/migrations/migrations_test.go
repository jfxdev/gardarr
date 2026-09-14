package migrations

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfxdev/gardarr/internal/infra/migration"
	"github.com/jfxdev/gardarr/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var expectedSeededCategories = map[string]struct {
	Icon               string
	Color              string
	DefaultTags        []string
	DefaultDirectories []string
}{
	"Movies": {Icon: "Film", Color: "#ef4444", DefaultTags: []string{"movie", "1080p"}, DefaultDirectories: []string{"/downloads/movies"}},
	"Shows":  {Icon: "Tv", Color: "#3b82f6", DefaultTags: []string{"tv", "episode"}, DefaultDirectories: []string{"/downloads/shows"}},
	"Games":  {Icon: "Gamepad2", Color: "#10b981", DefaultTags: []string{"game", "pc"}, DefaultDirectories: []string{"/downloads/games"}},
	"Other":  {Icon: "Folder", Color: "#6b7280", DefaultTags: []string{"misc"}, DefaultDirectories: []string{"/downloads/other"}},
	"Books":  {Icon: "BookOpen", Color: "#f59e0b", DefaultTags: []string{"book", "ebook"}, DefaultDirectories: []string{"/downloads/books"}},
	"Anime":  {Icon: "Star", Color: "#ec4899", DefaultTags: []string{"anime", "sub"}, DefaultDirectories: []string{"/downloads/anime"}},
	"Music":  {Icon: "Music", Color: "#14b8a6", DefaultTags: []string{"music", "flac"}, DefaultDirectories: []string{"/downloads/music"}},
}

func TestMigration007AddColorIconToCategories(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create migrator and register migrations
	m := migration.NewMigrator(db)
	Register(m)

	// Run all migrations
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify that the categories table exists
	if !db.Migrator().HasTable(&models.Category{}) {
		t.Error("Expected categories table to exist")
	}

	// Verify that color column exists
	if !db.Migrator().HasColumn(&models.Category{}, "color") {
		t.Error("Expected color column to exist in categories table")
	}

	// Verify that icon column exists
	if !db.Migrator().HasColumn(&models.Category{}, "icon") {
		t.Error("Expected icon column to exist in categories table")
	}

	// Test creating a category with color and icon
	category := models.Category{
		ID:                 "test-category-id",
		Name:               "Test Category",
		Color:              "#FF5733",
		Icon:               "Folder",
		DefaultTags:        models.StringArray{"tag1", "tag2"},
		DefaultDirectories: models.StringArray{"/path1"},
		MetadataSource:     "none",
	}

	if err := db.Create(&category).Error; err != nil {
		t.Errorf("Failed to create category with color and icon: %v", err)
	}

	// Retrieve and verify
	var retrieved models.Category
	if err := db.Where("id = ?", "test-category-id").First(&retrieved).Error; err != nil {
		t.Errorf("Failed to retrieve category: %v", err)
	}

	if retrieved.Color != "#FF5733" {
		t.Errorf("Expected color #FF5733, got %s", retrieved.Color)
	}

	if retrieved.Icon != "Folder" {
		t.Errorf("Expected icon 'Folder', got %s", retrieved.Icon)
	}
}

func TestMigration026SeedsDefaultCategories(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	assertSeededCategories(t, db)
}

func TestMigrationAllMigrationsCanRunTwice(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create migrator and register migrations
	m := migration.NewMigrator(db)
	Register(m)

	// Run all migrations first time
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations first time: %v", err)
	}

	// Run all migrations second time (should skip already applied)
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	assertSeededCategories(t, db)
}

func TestMigrationsCreateBandwidthSchedulesAndPersistentRuntimeState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	m := migration.NewMigrator(db)
	Register(m)
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	if !db.Migrator().HasTable(&models.BandwidthSchedule{}) {
		t.Fatal("Expected bandwidth schedules table to exist")
	}
	if !db.Migrator().HasColumn(&models.Worker{}, "DefaultDownloadSpeedLimit") || !db.Migrator().HasColumn(&models.Worker{}, "DefaultUploadSpeedLimit") {
		t.Fatal("Expected worker baseline columns to exist")
	}
	if !db.Migrator().HasColumn(&models.BandwidthSchedule{}, "Color") {
		t.Fatal("Expected schedule color column to exist")
	}
	for _, column := range []string{"LastAppliedBandwidthScheduleUUID", "LastAppliedDownloadSpeedLimit", "LastAppliedUploadSpeedLimit"} {
		if !db.Migrator().HasColumn(&models.Worker{}, column) {
			t.Fatalf("Expected persistent bandwidth schedule column %s to exist", column)
		}
	}
}

func TestMigration042And043CreateTransferAndDiscordTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	m := migration.NewMigrator(db)
	Register(m)
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	for _, model := range []interface{}{&models.TransferReportSettings{}, &models.TransferSnapshotRun{}, &models.TransferSnapshot{}, &models.TransferReport{}, &models.DiscordIntegration{}, &models.DiscordDeliveryHistory{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("Expected table for %T", model)
		}
	}
}

func TestMigration030RemovesLegacyWorkerTokenColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	legacyColumns := legacyWorkerTokenColumns()

	createWorkersSQL := `
		CREATE TABLE workers (
			uuid TEXT PRIMARY KEY,
			name TEXT UNIQUE,
			type TEXT,
			address TEXT NOT NULL,
			` + legacyColumns[0] + ` TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)
	`
	if err := db.Exec(createWorkersSQL).Error; err != nil {
		t.Fatalf("Failed to create legacy workers table: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	worker := models.Worker{
		Name:    "test-worker",
		Type:    "qbittorrent",
		Address: "http://localhost:8080",
	}

	if err := db.Create(&worker).Error; err != nil {
		t.Fatalf("Expected worker insert without legacy token column to succeed, got: %v", err)
	}

	var count int64
	if err := db.Model(&models.Worker{}).Where("name = ?", worker.Name).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count inserted workers: %v", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 inserted worker, got %d", count)
	}

	cols, err := db.Migrator().ColumnTypes("workers")
	if err != nil {
		t.Fatalf("Failed to inspect workers columns: %v", err)
	}

	columnNames := make([]string, 0, len(cols))
	for _, col := range cols {
		columnNames = append(columnNames, col.Name())
	}

	for _, legacyColumn := range legacyColumns {
		if slices.Contains(columnNames, legacyColumn) {
			t.Fatalf("Legacy worker token column still present after migration: %s", fmt.Sprint(columnNames))
		}
	}
}

func TestMigration026SeedsOnlyMissingCategories(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	existingMovies := models.Category{
		ID:                 "custom-movies-id",
		Name:               "Movies",
		Color:              "#000000",
		Icon:               "Archive",
		DefaultDirectories: models.StringArray{"/custom/movies"},
		MetadataSource:     "none",
		DefaultTags:        models.StringArray{"existing"},
	}

	if err := db.Where("name = ?", "Movies").Delete(&models.Category{}).Error; err != nil {
		t.Fatalf("Failed to remove seeded Movies category: %v", err)
	}

	if err := db.Create(&existingMovies).Error; err != nil {
		t.Fatalf("Failed to create existing Movies category: %v", err)
	}

	if err := db.Table("migrations").Where("version = ?", "026_seed_default_categories").Delete(&migration.Migration{}).Error; err != nil {
		t.Fatalf("Failed to reset migration record: %v", err)
	}

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to rerun migrations after seeding existing category: %v", err)
	}

	var movies []models.Category
	if err := db.Where("name = ?", "Movies").Find(&movies).Error; err != nil {
		t.Fatalf("Failed to load Movies categories: %v", err)
	}

	if len(movies) != 1 {
		t.Fatalf("Expected exactly 1 Movies category, got %d", len(movies))
	}

	if movies[0].ID != existingMovies.ID {
		t.Errorf("Expected existing Movies category to be preserved, got ID %s", movies[0].ID)
	}

	if movies[0].Color != existingMovies.Color {
		t.Errorf("Expected existing Movies color %s to be preserved, got %s", existingMovies.Color, movies[0].Color)
	}

	if movies[0].Icon != existingMovies.Icon {
		t.Errorf("Expected existing Movies icon %s to be preserved, got %s", existingMovies.Icon, movies[0].Icon)
	}
	if !slices.Equal([]string(movies[0].DefaultDirectories), []string(existingMovies.DefaultDirectories)) {
		t.Errorf("Expected existing Movies directories %v to be preserved, got %v", existingMovies.DefaultDirectories, movies[0].DefaultDirectories)
	}

	var categories []models.Category
	if err := db.Find(&categories).Error; err != nil {
		t.Fatalf("Failed to list categories: %v", err)
	}

	if len(categories) != len(expectedSeededCategories) {
		t.Fatalf("Expected %d total categories after reseed, got %d", len(expectedSeededCategories), len(categories))
	}

	for _, category := range categories {
		if category.Name == "Movies" {
			continue
		}

		expected, ok := expectedSeededCategories[category.Name]
		if !ok {
			t.Fatalf("Unexpected category found: %s", category.Name)
		}

		if category.Icon != expected.Icon {
			t.Errorf("Expected icon %s for %s, got %s", expected.Icon, category.Name, category.Icon)
		}

		if category.Color != expected.Color {
			t.Errorf("Expected color %s for %s, got %s", expected.Color, category.Name, category.Color)
		}
	}
}

func assertSeededCategories(t *testing.T, db *gorm.DB) {
	t.Helper()

	var categories []models.Category
	if err := db.Find(&categories).Error; err != nil {
		t.Fatalf("Failed to list categories: %v", err)
	}

	if len(categories) != len(expectedSeededCategories) {
		t.Fatalf("Expected %d seeded categories, got %d", len(expectedSeededCategories), len(categories))
	}

	seenNames := make([]string, 0, len(categories))
	for _, category := range categories {
		expected, ok := expectedSeededCategories[category.Name]
		if !ok {
			t.Fatalf("Unexpected category found: %s", category.Name)
		}

		seenNames = append(seenNames, category.Name)

		if category.Icon != expected.Icon {
			t.Errorf("Expected icon %s for %s, got %s", expected.Icon, category.Name, category.Icon)
		}

		if category.Color != expected.Color {
			t.Errorf("Expected color %s for %s, got %s", expected.Color, category.Name, category.Color)
		}

		if category.ID == "" {
			t.Errorf("Expected seeded category %s to have a generated ID", category.Name)
		}

		if !slices.Equal([]string(category.DefaultDirectories), expected.DefaultDirectories) {
			t.Errorf("Expected default directories %v for %s, got %v", expected.DefaultDirectories, category.Name, category.DefaultDirectories)
		}

		if !slices.Equal([]string(category.DefaultTags), expected.DefaultTags) {
			t.Errorf("Expected default tags %v for %s, got %v", expected.DefaultTags, category.Name, []string(category.DefaultTags))
		}

		if category.MetadataSource != "none" {
			t.Errorf("Expected metadata source 'none' for %s, got %s", category.Name, category.MetadataSource)
		}
	}

	for name := range expectedSeededCategories {
		if !slices.Contains(seenNames, name) {
			t.Errorf("Expected category %s to be seeded", name)
		}
	}
}

func TestMigration045AddsCategoryDefaultDirectories(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	if !db.Migrator().HasColumn(&models.Category{}, "default_directories") {
		t.Error("Expected default_directories column to exist in categories table")
	}

	if !db.Migrator().HasColumn(&models.Category{}, "metadata_source") {
		t.Error("Expected metadata_source column to exist in categories table")
	}
}

func TestMigration045PreservesLegacyDefaultDirectory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT, default_directory TEXT)`).Error; err != nil {
		t.Fatalf("Failed to create legacy categories table: %v", err)
	}
	if err := db.Exec(`INSERT INTO categories (id, name, default_directory) VALUES (?, ?, ?)`, "legacy-category", "Legacy", "/downloads/legacy").Error; err != nil {
		t.Fatalf("Failed to insert legacy category: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	var category models.Category
	if err := db.Where("id = ?", "legacy-category").First(&category).Error; err != nil {
		t.Fatalf("Failed to load migrated category: %v", err)
	}
	if !slices.Equal([]string(category.DefaultDirectories), []string{"/downloads/legacy"}) {
		t.Errorf("Expected legacy directory to be preserved, got %v", category.DefaultDirectories)
	}
}

func TestMigration031AddsLastKnownFieldsToTaskStates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	for _, column := range []string{"name", "category", "size"} {
		if !db.Migrator().HasColumn(&models.TaskState{}, column) {
			t.Errorf("Expected %s column to exist in task_states table", column)
		}
	}
}

func TestMigration032BackfillsRemovedEventNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	workerID := uuid.New()
	now := time.Now().UTC()

	added := models.Event{
		UUID:      uuid.New(),
		WorkerID:  workerID,
		Type:      "torrent.added",
		TaskHash:  "hash-with-source",
		Metadata:  `{"name":"Fedora ISO","category":"linux","size":2048}`,
		CreatedAt: now.Add(-2 * time.Hour),
	}
	removedWithSource := models.Event{
		UUID:      uuid.New(),
		WorkerID:  workerID,
		Type:      "torrent.removed",
		TaskHash:  "hash-with-source",
		Metadata:  `{"last_progress":1}`,
		CreatedAt: now.Add(-1 * time.Hour),
	}
	removedOrphan := models.Event{
		UUID:      uuid.New(),
		WorkerID:  workerID,
		Type:      "torrent.removed",
		TaskHash:  "hash-orphan",
		Metadata:  `{"last_progress":0.5}`,
		CreatedAt: now,
	}

	for _, ev := range []models.Event{added, removedWithSource, removedOrphan} {
		if err := db.Create(&ev).Error; err != nil {
			t.Fatalf("Failed to seed event: %v", err)
		}
	}

	// Rerun only the backfill migration against the seeded data
	if err := db.Table("migrations").Where("version = ?", "032_backfill_removed_event_names").Delete(&migration.Migration{}).Error; err != nil {
		t.Fatalf("Failed to reset migration record: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("Failed to rerun migrations: %v", err)
	}

	var backfilled models.Event
	if err := db.Where("uuid = ?", removedWithSource.UUID).First(&backfilled).Error; err != nil {
		t.Fatalf("Failed to load backfilled event: %v", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(backfilled.Metadata), &meta); err != nil {
		t.Fatalf("Failed to parse backfilled metadata: %v", err)
	}

	if meta["name"] != "Fedora ISO" {
		t.Errorf("Expected backfilled name 'Fedora ISO', got %v", meta["name"])
	}
	if meta["category"] != "linux" {
		t.Errorf("Expected backfilled category 'linux', got %v", meta["category"])
	}
	if meta["last_progress"] != float64(1) {
		t.Errorf("Expected last_progress preserved as 1, got %v", meta["last_progress"])
	}

	var orphan models.Event
	if err := db.Where("uuid = ?", removedOrphan.UUID).First(&orphan).Error; err != nil {
		t.Fatalf("Failed to load orphan event: %v", err)
	}
	if orphan.Metadata != removedOrphan.Metadata {
		t.Errorf("Expected orphan event metadata untouched, got %s", orphan.Metadata)
	}
}

func TestMigration028CreatesIntegrationProviderConfigsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	if !db.Migrator().HasTable(&models.IntegrationProviderConfig{}) {
		t.Error("Expected integration provider configs table to exist")
	}
}

func TestMigration033AddsCompositeIndexesToEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	for _, index := range []string{"idx_events_worker_created", "idx_events_worker_hash_type"} {
		if !db.Migrator().HasIndex(&models.Event{}, index) {
			t.Errorf("Expected index %s to exist on events table", index)
		}
	}
}

func TestMigration034RenamesImageOpacityToBrightness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	m := migration.NewMigrator(db)
	Register(m)

	if err := m.Up(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	if !db.Migrator().HasColumn(&models.TaskMetadata{}, "image_brightness") {
		t.Error("Expected column image_brightness to exist on task_metadata table")
	}
	if db.Migrator().HasColumn(&models.TaskMetadata{}, "image_opacity") {
		t.Error("Expected column image_opacity to no longer exist on task_metadata table")
	}
}
