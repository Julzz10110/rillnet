package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"
	httphandlers "rillnet/internal/handlers/http"
	"rillnet/internal/infrastructure/middleware"
	"rillnet/internal/infrastructure/monitoring"
	repositories "rillnet/internal/infrastructure/repositories"
	webrtcinfra "rillnet/internal/infrastructure/webrtc"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	startTime := time.Now()

	// Try multiple config paths
	configPaths := []string{
		"configs/config.yaml",
		"./configs/config.yaml",
		"/root/configs/config.yaml",
		"config.yaml",
	}

	var cfg *config.Config
	var err error

	for _, path := range configPaths {
		cfg, err = config.Load(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		// Fallback to defaults if config cannot be loaded
		cfg = config.DefaultConfig()
	}

	// Initialize logger
	zapLogger := logger.New(cfg.Logging.Level)
	defer zapLogger.Sync()

	log := zapLogger.Sugar()

	// Initialize repository factory
	repoFactory, err := repositories.NewRepositoryFactory(cfg, log)
	if err != nil {
		log.Fatalw("failed to create repository factory", "error", err)
	}
	defer repoFactory.Close()

	// Initialize repositories
	streamRepo := repoFactory.CreateStreamRepository()
	peerRepo := repoFactory.CreatePeerRepository()
	meshRepo := repoFactory.CreateMeshRepository()

	// Initialize services
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := services.NewMeshService(peerRepo, meshRepo)
	streamService := services.NewStreamService(streamRepo, peerRepo, meshRepo, meshService, metricsService)
	authService := services.NewAuthService(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
		streamService,
	)

	// WebRTC configuration (including STUN/TURN from config)
	var iceServers []webrtc.ICEServer
	if len(cfg.WebRTC.ICEServers) > 0 {
		for _, s := range cfg.WebRTC.ICEServers {
			iceServers = append(iceServers, webrtc.ICEServer{
				URLs:       s.URLs,
				Username:   s.Username,
				Credential: s.Credential,
			})
		}
	} else {
		// Fallback STUN server if not configured
		iceServers = []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		}
	}

	webrtcConfig := webrtcinfra.WebRTCConfig{
		ICEServers: iceServers,
		Simulcast:  cfg.WebRTC.Simulcast,
		MaxBitrate: cfg.WebRTC.MaxBitrate,
	}

	// Initialize SFU
	sfuService := webrtcinfra.NewSFUService(webrtcConfig, qualityService, metricsService, meshService)

	// Initialize monitoring
	prometheusCollector := monitoring.NewPrometheusCollector()
	fmt.Print("Prometheus Collector:", prometheusCollector)

	// Initialize HTTP handlers
	authHandler := httphandlers.NewAuthHandler(authService)
	streamHandler := httphandlers.NewStreamHandler(streamService, sfuService)

	// Configure Gin
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// Global HTTP rate limiting (if enabled)
	router.Use(middleware.NewHTTPRateLimitMiddleware(cfg))

	// Setup auth routes (public)
	authHandler.SetupRoutes(router)

	// Setup stream routes with authentication
	api := router.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(authService))
	{
		api.POST("/streams", streamHandler.CreateStream)
		api.GET("/streams/:id", streamHandler.GetStream)
		api.POST("/streams/:id/join", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.JoinStream)
		api.POST("/streams/:id/leave", streamHandler.LeaveStream)
		api.GET("/streams/:id/stats", streamHandler.GetStreamStats)
		api.GET("/streams", streamHandler.ListStreams)

		// WebRTC endpoints
		api.POST("/streams/:id/publisher/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.CreatePublisherOffer)
		api.POST("/streams/:id/publisher/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.HandlePublisherAnswer)
		api.POST("/streams/:id/subscriber/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.CreateSubscriberOffer)
		api.POST("/streams/:id/subscriber/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.HandleSubscriberAnswer)
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
		})
	})

	// Readiness endpoint (can be extended with real dependency checks)
	router.GET("/ready", func(c *gin.Context) {
		// Check repository health (Redis connection if enabled)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := repoFactory.HealthCheck(ctx); err != nil {
			c.JSON(503, gin.H{
				"status":       "not_ready",
				"timestamp":    time.Now(),
				"dependencies": "unhealthy",
				"error":        err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"status":       "ready",
			"timestamp":    time.Now(),
			"dependencies": "ok",
		})
	})

	// Prometheus metrics endpoint
	if cfg.Monitoring.PrometheusEnabled {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
		log.Info("Prometheus metrics enabled")
	}

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Infof("Starting RillNet Ingest server on %s", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signals or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalw("Server failed", "error", err)
	case sig := <-sigChan:
		log.Infow("Received shutdown signal", "signal", sig)
	}

	log.Info("Shutting down RillNet Ingest server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown HTTP server gracefully
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorw("Error during server shutdown", "error", err)
		// Force close if graceful shutdown fails
		if closeErr := srv.Close(); closeErr != nil {
			log.Errorw("Error force closing server", "error", closeErr)
		}
	} else {
		log.Info("Server shutdown gracefully")
	}

	// Close repository factory
	if err := repoFactory.Close(); err != nil {
		log.Errorw("Error closing repository factory", "error", err)
	}

	log.Info("RillNet Ingest server stopped")
}
