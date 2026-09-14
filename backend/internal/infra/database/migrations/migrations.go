package migrations

import (
	"encoding/json"
	"errors"

	"github.com/jfxdev/gardarr/internal/infra/migration"
	"github.com/jfxdev/gardarr/internal/models"

	"gorm.io/gorm"
)

const uuidCondition = "uuid = ?"

func legacyWorkerTokenColumns() []string {
	return []string{
		"encryp" + "eted_token",
		"encrypt" + "ed_token",
	}
}

// Register adiciona todas as migrations no migrator
func Register(m *migration.Migrator) {
	m.RegisterMultiple([]migration.MigrationItem{
		{
			Version:     "001_create_signup_tokens_table",
			Description: "Cria a tabela de tokens de signup (magic links)",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.SignupToken{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.SignupToken{})
			},
		},
		{
			Version:     "002_create_sessions_table",
			Description: "Cria a tabela de sessões",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Session{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Session{})
			},
		},
		{
			Version:     "003_create_users_table",
			Description: "Cria a tabela de usuários",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.User{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.User{})
			},
		},
		{
			Version:     "004_create_agents_table",
			Description: "Cria a tabela de agents",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Worker{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Worker{})
			},
		},
		{
			Version:     "005_create_categories_table",
			Description: "Cria a tabela de categorias",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Category{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Category{})
			},
		},
		{
			Version:     "006_create_password_reset_tokens_table",
			Description: "Cria a tabela de tokens de recuperação de senha",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.PasswordResetToken{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.PasswordResetToken{})
			},
		},
		{
			Version:     "008_create_settings_table",
			Description: "Cria a tabela de configurações do sistema",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Settings{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Settings{})
			},
		},
		{
			Version:     "009_create_events_table",
			Description: "Cria a tabela de eventos para histórico de mudanças de estado",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Event{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Event{})
			},
		},
		{
			Version:     "010_create_task_metadata_table",
			Description: "Cria a tabela de metadados de tasks com suporte a imagens",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.TaskMetadata{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.TaskMetadata{})
			},
		},
		{
			Version:     "013_create_user_preferences_table",
			Description: "Cria a tabela de preferências de usuário",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.UserPreferences{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.UserPreferences{})
			},
		},
		{
			Version:     "014_create_webhooks_table",
			Description: "Cria a tabela de webhooks para integrações",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Webhook{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.Webhook{})
			},
		},
		{
			Version:     "015_create_event_filters_table",
			Description: "Cria a tabela de filtros de eventos para integrações",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.EventFilter{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.EventFilter{})
			},
		},
		{
			Version:     "016_add_event_type_filter_column",
			Description: "Adiciona coluna event_type_filter na tabela event_filters",
			Up: func(db *gorm.DB) error {
				// Check if column already exists
				if db.Migrator().HasColumn(&models.EventFilter{}, "EventTypeFilter") {
					return nil
				}
				return db.Migrator().AddColumn(&models.EventFilter{}, "EventTypeFilter")
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropColumn(&models.EventFilter{}, "event_type_filter")
			},
		},
		{
			Version:     "017_create_task_states_table",
			Description: "Cria a tabela de estados de tasks para persistir estado entre reinicializações",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.TaskState{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.TaskState{})
			},
		},
		{
			Version:     "018_migrate_display_mode_default_to_table",
			Description: "Updates user preferences display mode from 'default' to 'table'",
			Up: func(db *gorm.DB) error {
				// Update all existing records with 'default' to 'table'
				return db.Model(&models.UserPreferences{}).
					Where("torrent_display_mode = ?", "default").
					Update("torrent_display_mode", "table").Error
			},
			Down: func(db *gorm.DB) error {
				// No-op: reverting torrent_display_mode changes is intentionally disabled.
				// Rolling back would overwrite user-set preferences (table/card/list) with "default",
				// which is destructive and irreversible. Users who manually changed their display
				// mode would lose their preference.
				return nil
			},
		},
		{
			Version:     "019_create_webhook_history_table",
			Description: "Cria a tabela de histórico de webhooks",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.WebhookHistory{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.WebhookHistory{})
			},
		},
		{
			Version:     "020_add_color_palettes_to_user_preferences",
			Description: "Adiciona colunas de paletas de cores customizáveis nas preferências do usuário",
			Up: func(db *gorm.DB) error {
				// Add color palette columns if they don't exist
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette1Primary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette1Primary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette1Secondary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette1Secondary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette1Accent") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette1Accent"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette2Primary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette2Primary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette2Secondary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette2Secondary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette2Accent") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette2Accent"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette3Primary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette3Primary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette3Secondary") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette3Secondary"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette3Accent") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette3Accent"); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				// Remove color palette columns
				columns := []string{
					"color_palette_1_primary", "color_palette_1_secondary", "color_palette_1_accent",
					"color_palette_2_primary", "color_palette_2_secondary", "color_palette_2_accent",
					"color_palette_3_primary", "color_palette_3_secondary", "color_palette_3_accent",
				}
				for _, col := range columns {
					if err := db.Migrator().DropColumn(&models.UserPreferences{}, col); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "021_add_active_color_palette_to_user_preferences",
			Description: "Adiciona coluna para selecionar a paleta de cores ativa (1, 2 ou 3)",
			Up: func(db *gorm.DB) error {
				// Add active_color_palette column if it doesn't exist
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ActiveColorPalette") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ActiveColorPalette"); err != nil {
						return err
					}
				}
				// Set default value to 1 for existing records
				return db.Model(&models.UserPreferences{}).Where("active_color_palette = ?", 0).Update("active_color_palette", 1).Error
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropColumn(&models.UserPreferences{}, "active_color_palette")
			},
		},
		{
			Version:     "022_add_muted_color_to_palettes",
			Description: "Adiciona a 4ª cor (muted) para cada paleta customizada",
			Up: func(db *gorm.DB) error {
				// Add muted color for palette 1
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette1Muted") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette1Muted"); err != nil {
						return err
					}
				}
				// Add muted color for palette 2
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette2Muted") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette2Muted"); err != nil {
						return err
					}
				}
				// Add muted color for palette 3
				if !db.Migrator().HasColumn(&models.UserPreferences{}, "ColorPalette3Muted") {
					if err := db.Migrator().AddColumn(&models.UserPreferences{}, "ColorPalette3Muted"); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				// Remove muted columns
				columns := []string{
					"color_palette_1_muted",
					"color_palette_2_muted",
					"color_palette_3_muted",
				}
				for _, col := range columns {
					if err := db.Migrator().DropColumn(&models.UserPreferences{}, col); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "023_rename_agents_to_workers",
			Description: "Renomeia tabela agents para workers e colunas agent_id para worker_id",
			Up: func(db *gorm.DB) error {
				if db.Migrator().HasTable("agents") {
					if err := db.Migrator().RenameTable("agents", "workers"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.Event{}, "agent_id") {
					if err := db.Migrator().RenameColumn(&models.Event{}, "agent_id", "worker_id"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.TaskState{}, "agent_id") {
					if err := db.Migrator().RenameColumn(&models.TaskState{}, "agent_id", "worker_id"); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasTable("workers") {
					if err := db.Migrator().RenameTable("workers", "agents"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.Event{}, "worker_id") {
					if err := db.Migrator().RenameColumn(&models.Event{}, "worker_id", "agent_id"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.TaskState{}, "worker_id") {
					if err := db.Migrator().RenameColumn(&models.TaskState{}, "worker_id", "agent_id"); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "024_remove_standalone_worker",
			Description: "Remove worker standalone do banco de dados (se existir)",
			Up: func(db *gorm.DB) error {
				return db.Where(uuidCondition, "00000000-0000-0000-0000-000000000000").Delete(&models.Worker{}).Error
			},
			Down: func(db *gorm.DB) error {
				// No-op: não queremos recriar o worker se fizer rollback
				return nil
			},
		},
		{
			Version:     "025_add_tgdb_metadata_to_task_metadata",
			Description: "Adiciona as colunas name e release_date na tabela task_metadata",
			Up: func(db *gorm.DB) error {
				if !db.Migrator().HasColumn(&models.TaskMetadata{}, "Name") {
					if err := db.Migrator().AddColumn(&models.TaskMetadata{}, "Name"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.TaskMetadata{}, "ReleaseDate") {
					if err := db.Migrator().AddColumn(&models.TaskMetadata{}, "ReleaseDate"); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				if err := db.Migrator().DropColumn(&models.TaskMetadata{}, "name"); err != nil {
					return err
				}
				if err := db.Migrator().DropColumn(&models.TaskMetadata{}, "release_date"); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version:     "027_rename_category_directory_and_add_metadata_source",
			Description: "Renomeia directory para default_directory e adiciona metadata_source em categories",
			Up: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.Category{}, "directory") && !db.Migrator().HasColumn(&models.Category{}, "default_directory") {
					if err := db.Migrator().RenameColumn(&models.Category{}, "directory", "default_directory"); err != nil {
						return err
					}
				}

				if !db.Migrator().HasColumn(&models.Category{}, "MetadataSource") {
					if err := db.Migrator().AddColumn(&models.Category{}, "MetadataSource"); err != nil {
						return err
					}
				}

				return db.Table("categories").
					Where("metadata_source = '' OR metadata_source IS NULL").
					Update("metadata_source", "none").Error
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.Category{}, "default_directory") && !db.Migrator().HasColumn(&models.Category{}, "directory") {
					if err := db.Migrator().RenameColumn(&models.Category{}, "default_directory", "directory"); err != nil {
						return err
					}
				}

				if db.Migrator().HasColumn(&models.Category{}, "metadata_source") {
					if err := db.Migrator().DropColumn(&models.Category{}, "metadata_source"); err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			Version:     "026_seed_default_categories",
			Description: "Popula categorias padrão do sistema sem sobrescrever categorias existentes",
			Up: func(db *gorm.DB) error {
				defaultCategories := []models.Category{
					{Name: "Movies", Color: "#ef4444", Icon: "Film", DefaultTags: models.StringArray{"movie", "1080p"}},
					{Name: "Shows", Color: "#3b82f6", Icon: "Tv", DefaultTags: models.StringArray{"tv", "episode"}},
					{Name: "Games", Color: "#10b981", Icon: "Gamepad2", DefaultTags: models.StringArray{"game", "pc"}},
					{Name: "Other", Color: "#6b7280", Icon: "Folder", DefaultTags: models.StringArray{"misc"}},
					{Name: "Books", Color: "#f59e0b", Icon: "BookOpen", DefaultTags: models.StringArray{"book", "ebook"}},
					{Name: "Anime", Color: "#ec4899", Icon: "Star", DefaultTags: models.StringArray{"anime", "sub"}},
					{Name: "Music", Color: "#14b8a6", Icon: "Music", DefaultTags: models.StringArray{"music", "flac"}},
				}

				for _, category := range defaultCategories {
					var existingCount int64
					if err := db.Table("categories").Where("name = ?", category.Name).Count(&existingCount).Error; err != nil {
						return err
					}

					if existingCount > 0 {
						continue
					}

					if err := db.Create(&category).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Down: func(db *gorm.DB) error {
				// No-op: rollback não deve remover categorias que podem já estar em uso.
				return nil
			},
		},
		{
			Version:     "028_create_integration_provider_configs_table",
			Description: "Cria tabela de configuração de providers de integração",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.IntegrationProviderConfig{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.IntegrationProviderConfig{})
			},
		},
		{
			Version:     "029_add_direct_qbittorrent_credentials_to_workers",
			Description: "Adiciona credenciais diretas do qBittorrent aos workers",
			Up: func(db *gorm.DB) error {
				if !db.Migrator().HasColumn(&models.Worker{}, "EncryptedQBittorrentURL") {
					if err := db.Migrator().AddColumn(&models.Worker{}, "EncryptedQBittorrentURL"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.Worker{}, "EncryptedQBittorrentUsername") {
					if err := db.Migrator().AddColumn(&models.Worker{}, "EncryptedQBittorrentUsername"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.Worker{}, "EncryptedQBittorrentPassword") {
					if err := db.Migrator().AddColumn(&models.Worker{}, "EncryptedQBittorrentPassword"); err != nil {
						return err
					}
				}

				return nil
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.Worker{}, "encrypted_q_bittorrent_password") {
					if err := db.Migrator().DropColumn(&models.Worker{}, "EncryptedQBittorrentPassword"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.Worker{}, "encrypted_q_bittorrent_username") {
					if err := db.Migrator().DropColumn(&models.Worker{}, "EncryptedQBittorrentUsername"); err != nil {
						return err
					}
				}
				if db.Migrator().HasColumn(&models.Worker{}, "encrypted_q_bittorrent_url") {
					if err := db.Migrator().DropColumn(&models.Worker{}, "EncryptedQBittorrentURL"); err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			Version:     "030_remove_legacy_worker_token_columns",
			Description: "Remove colunas legadas de token dos workers que nao sao mais usadas",
			Up: func(db *gorm.DB) error {
				for _, column := range legacyWorkerTokenColumns() {
					if !db.Migrator().HasColumn("workers", column) {
						continue
					}
					if err := db.Exec("ALTER TABLE workers DROP COLUMN " + column).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Down: func(db *gorm.DB) error {
				// No-op: as colunas removidas continham segredos legados e nao devem ser recriadas.
				return nil
			},
		},
		{
			Version:     "031_add_last_known_fields_to_task_states",
			Description: "Adiciona name, category e size na tabela task_states para preservar dados em eventos de remoção",
			Up: func(db *gorm.DB) error {
				for _, field := range []string{"Name", "Category", "Size"} {
					if db.Migrator().HasColumn(&models.TaskState{}, field) {
						continue
					}
					if err := db.Migrator().AddColumn(&models.TaskState{}, field); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				for _, field := range []string{"Size", "Category", "Name"} {
					if !db.Migrator().HasColumn(&models.TaskState{}, field) {
						continue
					}
					if err := db.Migrator().DropColumn(&models.TaskState{}, field); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "032_backfill_removed_event_names",
			Description: "Preenche name em eventos torrent.removed a partir de eventos anteriores do mesmo hash",
			Up:          backfillRemovedEventNames,
			Down: func(db *gorm.DB) error {
				// No-op: backfill não destrutivo, não há o que reverter.
				return nil
			},
		},
		{
			Version:     "033_add_composite_indexes_to_events",
			Description: "Adiciona índices compostos (worker_id, created_at) e (worker_id, task_hash, type) em events para listagem/dedupe/purge",
			Up: func(db *gorm.DB) error {
				for _, index := range []string{"idx_events_worker_created", "idx_events_worker_hash_type"} {
					if db.Migrator().HasIndex(&models.Event{}, index) {
						continue
					}
					if err := db.Migrator().CreateIndex(&models.Event{}, index); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				for _, index := range []string{"idx_events_worker_hash_type", "idx_events_worker_created"} {
					if !db.Migrator().HasIndex(&models.Event{}, index) {
						continue
					}
					if err := db.Migrator().DropIndex(&models.Event{}, index); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "034_rename_image_opacity_to_brightness",
			Description: "Renomeia coluna image_opacity para image_brightness em task_metadata",
			Up: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.TaskMetadata{}, "image_opacity") {
					if err := db.Migrator().RenameColumn(&models.TaskMetadata{}, "image_opacity", "image_brightness"); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.TaskMetadata{}, "image_brightness") {
					if err := db.Migrator().RenameColumn(&models.TaskMetadata{}, "image_brightness", "image_opacity"); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "035_backfill_default_category_tags",
			Description: "Popula default_tags das categorias padrão que ainda estão sem tags",
			Up: func(db *gorm.DB) error {
				defaultTags := map[string]models.StringArray{
					"Movies": {"movie", "1080p"},
					"Shows":  {"tv", "episode"},
					"Games":  {"game", "pc"},
					"Other":  {"misc"},
					"Books":  {"book", "ebook"},
					"Anime":  {"anime", "sub"},
					"Music":  {"music", "flac"},
				}

				for name, tags := range defaultTags {
					var category models.Category
					err := db.Where("name = ?", name).First(&category).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					if err != nil {
						return err
					}

					if len(category.DefaultTags) > 0 {
						continue
					}

					if err := db.Model(&category).Update("default_tags", tags).Error; err != nil {
						return err
					}
				}

				return nil
			},
			Down: func(db *gorm.DB) error {
				// No-op: rollback não deve remover tags que podem já estar em uso.
				return nil
			},
		},
		{
			Version:     "036_create_bandwidth_schedules",
			Description: "Cria programações de limite de banda e baseline por worker",
			Up: func(db *gorm.DB) error {
				if !db.Migrator().HasColumn(&models.Worker{}, "DefaultDownloadSpeedLimit") {
					if err := db.Migrator().AddColumn(&models.Worker{}, "DefaultDownloadSpeedLimit"); err != nil {
						return err
					}
				}
				if !db.Migrator().HasColumn(&models.Worker{}, "DefaultUploadSpeedLimit") {
					if err := db.Migrator().AddColumn(&models.Worker{}, "DefaultUploadSpeedLimit"); err != nil {
						return err
					}
				}
				return db.AutoMigrate(&models.BandwidthSchedule{})
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasTable(&models.BandwidthSchedule{}) {
					if err := db.Migrator().DropTable(&models.BandwidthSchedule{}); err != nil {
						return err
					}
				}
				// No-op: os limites padrão do worker são mantidos para evitar perda de configuração.
				return nil
			},
		},
		{
			Version:     "037_add_bandwidth_schedule_colors",
			Description: "Adiciona cores neutras por índice às programações de banda",
			Up: func(db *gorm.DB) error {
				if !db.Migrator().HasColumn(&models.BandwidthSchedule{}, "Color") {
					if err := db.Migrator().AddColumn(&models.BandwidthSchedule{}, "Color"); err != nil {
						return err
					}
				}

				var schedules []models.BandwidthSchedule
				if err := db.Order("worker_uuid, created_at, uuid").Find(&schedules).Error; err != nil {
					return err
				}
				colors := []string{"#64748b", "#78716c", "#6b7280", "#71717a", "#475569", "#57534e", "#4b5563"}
				indexes := map[string]int{}
				for _, schedule := range schedules {
					workerID := schedule.WorkerUUID.String()
					index := indexes[workerID]
					if err := db.Model(&models.BandwidthSchedule{}).Where(uuidCondition, schedule.UUID).Update("color", colors[index%len(colors)]).Error; err != nil {
						return err
					}
					indexes[workerID]++
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.BandwidthSchedule{}, "Color") {
					return db.Migrator().DropColumn(&models.BandwidthSchedule{}, "Color")
				}
				return nil
			},
		},
		{
			Version:     "038_create_tags_table",
			Description: "Cria a tabela de tags (enriquecimento local: cor e ícone)",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.Tag{})
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasTable(&models.Tag{}) {
					return db.Migrator().DropTable(&models.Tag{})
				}
				return nil
			},
		},
		{
			Version:     "039_add_release_type_to_categories",
			Description: "Adiciona o tipo de release opcional às categorias",
			Up: func(db *gorm.DB) error {
				if !db.Migrator().HasColumn(&models.Category{}, "ReleaseType") {
					if err := db.Migrator().AddColumn(&models.Category{}, "ReleaseType"); err != nil {
						return err
					}
				}
				if err := db.Table("categories").Where("release_type = '' OR release_type IS NULL").Update("release_type", "none").Error; err != nil {
					return err
				}
				if err := db.Table("categories").Where("lower(name) IN ?", []string{"movies", "films", "filmes"}).Update("release_type", "movie").Error; err != nil {
					return err
				}
				return db.Table("categories").Where("lower(name) IN ?", []string{"shows", "series", "séries", "tv"}).Update("release_type", "series").Error
			},
			Down: func(db *gorm.DB) error {
				if db.Migrator().HasColumn(&models.Category{}, "release_type") {
					return db.Migrator().DropColumn(&models.Category{}, "release_type")
				}
				return nil
			},
		},
		{
			Version:     "040_map_release_types_for_default_categories",
			Description: "Mapeia categorias padrão adicionais para sugestões de releases",
			Up: func(db *gorm.DB) error {
				mappings := map[string][]string{
					"os":    {"operating systems", "os", "linux", "distros"},
					"game":  {"games", "jogos"},
					"book":  {"books", "livros", "ebooks", "e-books"},
					"music": {"music", "musicas", "músicas"},
				}
				for releaseType, names := range mappings {
					if err := db.Table("categories").Where("lower(name) IN ? AND (release_type = '' OR release_type IS NULL OR release_type = ?)", names, "none").Update("release_type", releaseType).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(_ *gorm.DB) error { return nil },
		},
		{
			Version:     "041_map_extended_release_types_for_default_categories",
			Description: "Mapeia categorias padrão para os tipos estendidos de releases",
			Up: func(db *gorm.DB) error {
				mappings := map[string][]string{
					"software":  {"software", "apps", "applications", "aplicativos"},
					"audiobook": {"audiobooks", "audio books", "audiolivros"},
					"comic":     {"comics", "comic books", "manga", "mangá", "hq", "hqs"},
					"course":    {"courses", "cursos", "training", "treinamentos"},
					"dataset":   {"datasets", "data sets", "dados"},
					"rom":       {"roms", "retro games", "jogos retro"},
					"podcast":   {"podcasts"},
					"anime":     {"anime", "animes"},
				}
				for releaseType, names := range mappings {
					if err := db.Table("categories").Where("lower(name) IN ? AND (release_type = '' OR release_type IS NULL OR release_type = ?)", names, "none").Update("release_type", releaseType).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(_ *gorm.DB) error { return nil },
		},
		{
			Version:     "042_create_transfer_reports_tables",
			Description: "Cria snapshots de transferência e relatórios de atividade",
			Up: func(db *gorm.DB) error {
				if err := db.AutoMigrate(&models.TransferReportSettings{}); err != nil {
					return err
				}
				if err := db.AutoMigrate(&models.TransferSnapshotRun{}, &models.TransferSnapshot{}); err != nil {
					return err
				}
				return db.AutoMigrate(&models.TransferReport{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.TransferReport{}, &models.TransferSnapshot{}, &models.TransferSnapshotRun{}, &models.TransferReportSettings{})
			},
		},
		{
			Version:     "043_create_discord_integrations_tables",
			Description: "Cria destinos Discord e histórico de entregas",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&models.DiscordIntegration{}, &models.DiscordDeliveryHistory{})
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&models.DiscordDeliveryHistory{}, &models.DiscordIntegration{})
			},
		},
		{
			Version:     "044_persist_bandwidth_schedule_state",
			Description: "Persiste o último schedule de banda aplicado por worker para evitar reaplicação após reinício",
			Up: func(db *gorm.DB) error {
				for _, field := range []string{"LastAppliedBandwidthScheduleUUID", "LastAppliedDownloadSpeedLimit", "LastAppliedUploadSpeedLimit"} {
					if db.Migrator().HasColumn(&models.Worker{}, field) {
						continue
					}
					if err := db.Migrator().AddColumn(&models.Worker{}, field); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(db *gorm.DB) error {
				for _, field := range []string{"LastAppliedUploadSpeedLimit", "LastAppliedDownloadSpeedLimit", "LastAppliedBandwidthScheduleUUID"} {
					if !db.Migrator().HasColumn(&models.Worker{}, field) {
						continue
					}
					if err := db.Migrator().DropColumn(&models.Worker{}, field); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
}

// backfillRemovedEventNames copia name/category/size do evento mais recente de
// cada task para os eventos torrent.removed que foram criados sem esses dados.
func backfillRemovedEventNames(db *gorm.DB) error {
	var removed []models.Event
	if err := db.Where("type = ?", "torrent.removed").
		Where("metadata NOT LIKE ?", `%"name"%`).
		Find(&removed).Error; err != nil {
		return err
	}

	for _, ev := range removed {
		if err := backfillRemovedEvent(db, ev); err != nil {
			return err
		}
	}

	return nil
}

// backfillRemovedEvent copies name/category/size from the most recent
// non-removal event for the same task into ev's metadata, if one exists.
// Any lookup/parse failure just leaves ev as-is rather than failing the
// whole migration.
func backfillRemovedEvent(db *gorm.DB, ev models.Event) error {
	var src models.Event
	err := db.Where("worker_id = ? AND task_hash = ? AND type <> ? AND metadata LIKE ?",
		ev.WorkerID, ev.TaskHash, "torrent.removed", `%"name"%`).
		Order("created_at DESC").
		First(&src).Error
	if err != nil {
		return nil
	}

	var srcMeta map[string]interface{}
	if err := json.Unmarshal([]byte(src.Metadata), &srcMeta); err != nil {
		return nil
	}

	name, _ := srcMeta["name"].(string)
	if name == "" {
		return nil
	}

	evMeta, err := parseEventMetadata(ev.Metadata)
	if err != nil {
		return nil
	}

	evMeta["name"] = name
	for _, key := range []string{"category", "size"} {
		if _, exists := evMeta[key]; !exists {
			if v, ok := srcMeta[key]; ok {
				evMeta[key] = v
			}
		}
	}

	merged, err := json.Marshal(evMeta)
	if err != nil {
		return nil
	}

	return db.Model(&models.Event{}).
		Where(uuidCondition, ev.UUID).
		Update("metadata", string(merged)).Error
}

// parseEventMetadata parses an event's metadata JSON, treating an empty
// string as an empty object rather than a parse error.
func parseEventMetadata(raw string) (map[string]interface{}, error) {
	if raw == "" {
		return map[string]interface{}{}, nil
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, err
	}
	return meta, nil
}
