package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/jfxdev/gardarr/internal/constants"
	"github.com/jfxdev/gardarr/internal/infra/database"
	"github.com/jfxdev/gardarr/internal/middlewares"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/auth"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/category"
	discordRoutes "github.com/jfxdev/gardarr/internal/routes/api/v1/discord"
	eventsRoutes "github.com/jfxdev/gardarr/internal/routes/api/v1/events"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/health"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/integrations"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/profile"
	reportsRoutes "github.com/jfxdev/gardarr/internal/routes/api/v1/reports"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/settings"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/setup"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/signup"
	tagsRoutes "github.com/jfxdev/gardarr/internal/routes/api/v1/tags"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/task_metadata"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/users"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/version"
	"github.com/jfxdev/gardarr/internal/routes/api/v1/workers"
	wsRoutes "github.com/jfxdev/gardarr/internal/routes/api/v1/ws"
	metricsRoutes "github.com/jfxdev/gardarr/internal/routes/metrics"
	"github.com/jfxdev/gardarr/internal/schemas"
	"github.com/jfxdev/gardarr/internal/services/bandwidthscheduler"
	"github.com/jfxdev/gardarr/internal/services/crypto"
	discordService "github.com/jfxdev/gardarr/internal/services/discord"
	"github.com/jfxdev/gardarr/internal/services/eventpoller"
	eventsService "github.com/jfxdev/gardarr/internal/services/events"
	"github.com/jfxdev/gardarr/internal/services/integration"
	tgdbintegration "github.com/jfxdev/gardarr/internal/services/integrations/tgdb"
	tmdbintegration "github.com/jfxdev/gardarr/internal/services/integrations/tmdb"
	settingsService "github.com/jfxdev/gardarr/internal/services/settings"
	metadata "github.com/jfxdev/gardarr/internal/services/task_metadata"
	transferreport "github.com/jfxdev/gardarr/internal/services/transferreport"
	websocketSvc "github.com/jfxdev/gardarr/internal/services/websocket"
	"github.com/jfxdev/gardarr/internal/services/workermanager"
	"github.com/spf13/cobra"

	"github.com/jfxdev/gardarr/pkg/env"
	"github.com/jfxdev/gardarr/pkg/filters"
	"github.com/jfxdev/gardarr/pkg/validations"
	"github.com/pkg/errors"
)

var (
	router *gin.Engine
)

type routeDependencies struct {
	db                *database.Database
	workers           *workermanager.Service
	bandwidthSchedule *bandwidthscheduler.Service
	metadata          *metadata.Service
	integrations      *integration.Service
	providerConfig    *integration.ProviderConfigService
	transferReports   *transferreport.Service
	discord           *discordService.Service
	websocket         *websocketSvc.Hub
}

// getBaseURL returns the base URL from APP_URL env var or constructs it from APP_PORT
// Falls back to BASE_URL for backward compatibility
func getBaseURL() string {
	// Check APP_URL first
	if appURL := env.Get(constants.AppURLEnv).Value(); appURL != "" {
		return appURL
	}

	// Fall back to BASE_URL for backward compatibility
	if customURL := os.Getenv("BASE_URL"); customURL != "" {
		return customURL
	}

	// Default: construct from APP_PORT
	port := env.Get(constants.AppPortEnv).Default("3200").Value()
	return fmt.Sprintf("http://localhost:%s", port)
}

func getMediaDirectory() string {
	return env.Get(constants.TorrentImageUploadDirEnv).Default("/media/uploads/images").Value()
}

func Run(cmd *cobra.Command, args []string) error {
	// Validate filesystem permissions before starting
	// Get all directory paths from environment variables
	mediaDir := getMediaDirectory()
	dbPath := env.Get("DATABASE_FILE_PATH").Default("/data/gardarr_database.db").Value()

	// Validate data directories
	if err := validations.ValidateDataDirectories(mediaDir); err != nil {
		log.Printf("❌ Filesystem validation failed: %v", err)
		return fmt.Errorf("filesystem validation failed: %w", err)
	}

	// Validate database path (this will also validate the database directory)
	if err := validations.ValidateDatabasePath(dbPath); err != nil {
		log.Printf("❌ Database path validation failed: %v", err)
		return fmt.Errorf("database path validation failed: %w", err)
	}

	log.Println("✅ Filesystem validation passed - all directories are writable")

	cryptoSvc, err := crypto.NewCryptoService()
	if err != nil {
		return err
	}

	db, err := database.NewDatabase()
	if err != nil {
		panic(fmt.Sprintf("erro ao conectar no banco: %v", err))
	}

	if err := db.Ping(context.Background()); err != nil {
		panic(fmt.Sprintf("erro ao fazer conexão com banco: %v", err))
	}

	if err := database.RunMigrations(db); err != nil {
		panic(fmt.Sprintf("erro ao rodar migrations: %v", err))
	}

	// Initialize settings service and bootstrap default settings
	settingsSvc := settingsService.NewService(db)
	if err := settingsSvc.Initialize(context.Background()); err != nil {
		panic(fmt.Sprintf("erro ao inicializar configurações: %v", err))
	}

	providerConfigSvc := integration.NewProviderConfigService(db, cryptoSvc)
	if err := providerConfigSvc.BootstrapTGDBFromEnv(context.Background()); err != nil {
		panic(fmt.Sprintf("erro ao inicializar integração TGDB: %v", err))
	}
	if err := providerConfigSvc.BootstrapTMDBFromEnv(context.Background()); err != nil {
		panic(fmt.Sprintf("erro ao inicializar integração TMDB: %v", err))
	}

	// Get base URL for building image URLs
	baseURL := getBaseURL()
	mediaDirectory := getMediaDirectory()

	allowedOrigins := setRouter()

	// Events service - tracks task state changes. Constructed before
	// workerSvc so its channel can be handed to the worker health monitor,
	// which records worker.offline/worker.recovered transitions on it.
	eventSvc, err := eventsService.NewService(db)
	if err != nil {
		return fmt.Errorf("failed to initialize events service: %w", err)
	}
	eventChan := eventSvc.Subscribe(0) // 0 = default buffer (EVENT_SUBSCRIBER_BUFFER)

	workerSvc, err := workermanager.NewService(db, cryptoSvc, baseURL, mediaDirectory, eventSvc)
	if err != nil {
		return err
	}

	ctx, cancelPoller := context.WithCancel(context.Background())
	defer cancelPoller()

	// Worker health monitor — probes registered workers on its own cadence
	// and caches status/instance data for ListWorkers/ListTasks instead of
	// probing inline on every call.
	workerSvc.StartHealthMonitor(ctx)

	// Event poller — polls workers for task state changes to feed events system
	eventPollerSvc := eventpoller.NewService(workerSvc, eventSvc)
	eventPollerSvc.Start(ctx)

	// Bandwidth scheduler — applies configured global rate limits in the
	// Gardarr settings timezone.
	bandwidthSchedulerSvc := bandwidthscheduler.NewService(db, workerSvc, settingsSvc, eventSvc)
	bandwidthSchedulerSvc.Start(ctx)

	// Subscribe Discord before report reconciliation: report events are
	// broadcast live and are not replayed to subscribers created afterwards.
	discordSvc := discordService.NewService(eventSvc.Subscribe(0), db, cryptoSvc, settingsSvc)
	discordSvc.Start(ctx)

	transferReportSvc := transferreport.NewService(db, workerSvc, settingsSvc, eventSvc)
	transferReportSvc.Start(ctx)

	// Periodic cleanup — enforces EVENT_RETENTION_DAYS and prunes stale task states
	eventSvc.StartCleanupJob(ctx)

	// WebSocket Hub
	wsHub := websocketSvc.NewHub(eventSvc, workerSvc)
	go wsHub.Start(ctx)

	// Integration service - consumes events in real-time
	integrationSvc := integration.NewService(eventChan, db)
	integrationSvc.Start(ctx)

	metaSvc, err := metadata.NewService(
		db,
		baseURL,
		mediaDirectory,
		metadata.NewMetadataProviderRegistry(
			tgdbintegration.NewMetadataProvider(providerConfigSvc),
			tmdbintegration.NewMetadataProvider(providerConfigSvc),
		),
	)
	if err != nil {
		return err
	}

	if err = setRoutes(routeDependencies{
		db:                db,
		workers:           workerSvc,
		bandwidthSchedule: bandwidthSchedulerSvc,
		metadata:          metaSvc,
		integrations:      integrationSvc,
		providerConfig:    providerConfigSvc,
		transferReports:   transferReportSvc,
		discord:           discordSvc,
		websocket:         wsHub,
	}, allowedOrigins); err != nil {
		return err
	}

	// Register Prometheus metrics endpoint (optional, requires METRICS_USERNAME + METRICS_PASSWORD)
	if metricsModule := metricsRoutes.NewModule(router, workerSvc); metricsModule != nil {
		metricsModule.Register()
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", env.Get(constants.AppPortEnv).Default("3200").Value()),
		Handler: router,
		// set timeout due CWE-400 - Potential Slowloris Attack
		ReadHeaderTimeout: 5 * time.Second,
		// Bound how long a client connection can stay open reading a request
		// body, writing a response, or sitting idle between keep-alive
		// requests, so a slow/stalled client can't hold a handler goroutine
		// (and its file descriptor) open indefinitely.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below. A listen error
	// (e.g. port already in use) is reported back on a channel instead of
	// calling log.Fatalf from inside the goroutine, so the deferred cleanup
	// registered earlier in Run (e.g. cancelPoller) still runs.
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			return errors.Wrap(err, "server failed to start: ")
		}
		return nil
	case <-quit:
	}

	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "Server forced to shutdown: ")
	}

	log.Println("Server exiting")

	return nil
}

// GetAllAllowedOrigins returns a list of allowed origins based on APP_URL and APP_DOMAINS
func GetAllAllowedOrigins() []string {
	allowedOrigins := originsFromBaseURL(getBaseURL())

	appDomains := env.Get(constants.AppDomainsEnv).Default("").Value()
	for _, d := range strings.Split(appDomains, ",") {
		d = strings.TrimSpace(d)
		if d == "" || filters.ContainsString(allowedOrigins, d) {
			continue
		}
		allowedOrigins = append(allowedOrigins, d)
	}

	return allowedOrigins
}

// originsFromBaseURL builds the initial allowed-origins list from APP_URL,
// including common localhost dev ports when the base URL points at localhost.
func originsFromBaseURL(baseURL string) []string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return []string{strings.TrimRight(baseURL, "/")}
	}

	// Build origin as scheme://host (host includes port if present)
	origins := []string{u.Scheme + "://" + u.Host}

	// Allow common development origins for localhost
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		origins = append(origins, "http://localhost:3500")
	}

	return origins
}

// securityHeadersMiddleware adds comprehensive security headers
func securityHeadersMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build dynamic CSP directives based on allowed origins
		// Only adding the host part (without scheme) to make CSP cleaner, or just raw origin
		cspOrigins := strings.Join(allowedOrigins, " ")

		// Content Security Policy - Restrictive but compatible with React/Vite
		csp := []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'", // Vite needs unsafe-inline/eval in dev
			"style-src 'self' 'unsafe-inline'",                // React/Tailwind need unsafe-inline
			"img-src 'self' data: blob: https:",
			"font-src 'self' data:",
			"connect-src 'self' ws: wss: " + cspOrigins, // Allow WebSocket for dev HMR and allowed domains
			"frame-ancestors 'none'",
			"base-uri 'self'",
			"form-action 'self'",
			"object-src 'none'",
			"media-src 'self'",
			"worker-src 'self' blob:",
			"manifest-src 'self'",
		}

		// Allow custom CSP override via environment variable
		if customCSP := os.Getenv("CUSTOM_CSP"); customCSP != "" {
			c.Header("Content-Security-Policy", customCSP)
		} else {
			c.Header("Content-Security-Policy", strings.Join(csp, "; "))
		}

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// XSS Protection (legacy but still good to have)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Strict Transport Security - Force HTTPS (enable in production)
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Referrer Policy - Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Permissions Policy - Restrict browser features
		permissions := []string{
			"geolocation=()",
			"midi=()",
			"notifications=()",
			"push=()",
			"sync-xhr=()",
			"microphone=()",
			"camera=()",
			"magnetometer=()",
			"gyroscope=()",
			"speaker=()",
			"vibrate=()",
			"fullscreen=(self)",
			"payment=()",
		}
		c.Header("Permissions-Policy", strings.Join(permissions, ", "))

		// Cross-Origin policies - different handling for media vs other routes
		if strings.HasPrefix(c.Request.URL.Path, "/media/") {
			// For authenticated media: allow cross-origin since frontend might be on different port
			c.Header("Cross-Origin-Embedder-Policy", "unsafe-none")
			c.Header("Cross-Origin-Resource-Policy", "cross-origin")
			c.Header("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		} else {
			// Strict policies for API and other routes
			c.Header("Cross-Origin-Embedder-Policy", "require-corp")
			c.Header("Cross-Origin-Resource-Policy", "same-origin")
			c.Header("Cross-Origin-Opener-Policy", "same-origin")
		}

		c.Next()
	}
}

func setRouter() []string {
	// Set GIN mode based on GIN_MODE environment variable (case-insensitive)
	// Falls back to LOG_LEVEL for compatibility, defaults to release mode
	ginMode := strings.TrimSpace(env.Get("GIN_MODE").Default("").Value())

	if ginMode == "" {
		// Fall back to LOG_LEVEL for compatibility
		ginMode = strings.TrimSpace(env.Get("LOG_LEVEL").Default("release").Value())
	}

	if strings.EqualFold(ginMode, "debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router = gin.Default()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		schemas.RegisterCustomValidators(v)
	}

	allowedOrigins := GetAllAllowedOrigins()

	corsConfig := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	router.Use(cors.New(corsConfig))

	// Setup Security Headers
	router.Use(securityHeadersMiddleware(allowedOrigins))

	return allowedOrigins
}

// buildSafeMediaPath extracts and validates path components safely
func buildSafeMediaPath(requestedFile string) (string, error) {
	components := strings.Split(requestedFile, "/")
	var safeComponents []string
	for _, comp := range components {
		if comp == "" || comp == "." || comp == ".." {
			continue
		}
		if err := validations.ValidatePathComponent(comp); err != nil {
			return "", err
		}
		// Using filepath.Base breaks the SonarQube taint trace (S2083)
		safeComponents = append(safeComponents, filepath.Base(comp))
	}

	if len(safeComponents) == 0 {
		return "", errors.New("invalid file path")
	}

	return filepath.Join(safeComponents...), nil
}

// createMediaHandler returns a handler for serving authenticated media files
func createMediaHandler(absMediaPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestedFile := strings.TrimPrefix(c.Param("filepath"), "/")

		safePath, err := buildSafeMediaPath(requestedFile)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
			return
		}

		// Use http.Dir which provides secure file access and inherently prevents directory traversal
		fs := http.Dir(absMediaPath)
		file, err := fs.Open(safePath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access file"})
			return
		}
		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot serve directories"})
			return
		}

		// Set no-cache headers and serve file securely
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
	}
}

// spaFallbackHandler serves the SPA index.html for non-API routes
func spaFallbackHandler(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
		return
	}

	indexPath := filepath.Join("./web", "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		c.File(indexPath)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "Frontend not found"})
	}
}

func setRoutes(dependencies routeDependencies, allowedOrigins []string) error {
	// Get current working directory
	wd, _ := os.Getwd()
	webPath := filepath.Join(wd, "web")
	assetsPath := filepath.Join(webPath, "assets")

	// Serve static files from the web directory FIRST
	router.Static("/assets", assetsPath)
	router.StaticFile("/logo.ico", filepath.Join(webPath, "logo.ico"))

	// Serve uploaded media files with authentication required
	mediaPath := getMediaDirectory()
	absMediaPath, err := filepath.Abs(filepath.Clean(mediaPath))
	if err != nil {
		return fmt.Errorf("failed to resolve media directory path: %w", err)
	}
	router.GET("/media/*filepath", middlewares.SessionMiddleware(dependencies.db), createMediaHandler(absMediaPath))

	// API routes
	v1 := router.Group("/v1")
	health.NewModule(v1, dependencies.db).Register()
	auth.NewModule(v1, dependencies.db, dependencies.websocket).Register()
	workers.NewModule(v1, dependencies.db, dependencies.workers, dependencies.bandwidthSchedule).Register()
	category.NewModule(v1, dependencies.db).Register()
	tagsRoutes.NewModule(v1, dependencies.db, dependencies.workers).Register()
	users.NewModule(v1, dependencies.db).Register()
	profile.NewModule(v1, dependencies.db).Register()
	signup.NewModule(v1, dependencies.db).Register()
	setup.NewModule(v1, dependencies.db).Register()
	settings.NewModule(v1, dependencies.db, dependencies.metadata).Register()
	reportsRoutes.NewModule(v1, dependencies.db, dependencies.transferReports).Register()
	discordRoutes.NewModule(v1, dependencies.db, dependencies.discord).Register()
	version.NewModule(v1, dependencies.db).Register()
	eventsModule, err := eventsRoutes.NewModule(v1, dependencies.db)
	if err != nil {
		return fmt.Errorf("failed to initialize events module: %w", err)
	}
	eventsModule.Register()
	wsRoutes.NewModule(v1, dependencies.db, dependencies.websocket, allowedOrigins).Register()
	task_metadata.NewModule(v1, dependencies.db, dependencies.metadata).Register()
	integrations.NewModule(v1, dependencies.db, dependencies.integrations, dependencies.providerConfig).Register()

	// Serve the main index.html for all non-API routes (SPA fallback)
	router.NoRoute(spaFallbackHandler)

	return nil
}
