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
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	httphandlers "rillnet/internal/handlers/http"
	"rillnet/internal/infrastructure/middleware"
	"rillnet/internal/infrastructure/monitoring"
	reliability "rillnet/internal/infrastructure/reliability"
	repositories "rillnet/internal/infrastructure/repositories"
	webrtcinfra "rillnet/internal/infrastructure/webrtc"
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"
	"rillnet/pkg/retry"

	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	startTime := time.Now()

	cfg, err := config.LoadResolved()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	zapLogger := logger.New(cfg.Logging.Level)
	defer func() { _ = zapLogger.Sync() }()

	log := zapLogger.Sugar()

	// Initialize repository factory
	repoFactory, err := repositories.NewRepositoryFactory(cfg, log)
	if err != nil {
		log.Fatalw("failed to create repository factory", "error", err)
	}
	defer func() { _ = repoFactory.Close() }()

	// Initialize repositories
	streamRepo := repoFactory.CreateStreamRepository()
	peerRepo := repoFactory.CreatePeerRepository()
	meshRepo := repoFactory.CreateMeshRepository()

	// Initialize services
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	baseMeshService := services.NewMeshService(peerRepo, meshRepo, cfg.Mesh, log)

	// Wrap mesh service with retry and circuit breaker if enabled
	var meshService ports.MeshService
	if cfg.Retry.Enabled || cfg.CircuitBreaker.Enabled {
		retryCfg := retry.Config{
			Enabled:      cfg.Retry.Enabled,
			MaxAttempts:  cfg.Retry.MaxAttempts,
			InitialDelay: cfg.Retry.InitialDelay,
			MaxDelay:     cfg.Retry.MaxDelay,
			Multiplier:   cfg.Retry.Multiplier,
			Jitter:       cfg.Retry.Jitter,
		}
		cbCfg := circuitbreaker.Config{
			FailureThreshold:   cfg.CircuitBreaker.FailureThreshold,
			SuccessThreshold:   cfg.CircuitBreaker.SuccessThreshold,
			Timeout:            cfg.CircuitBreaker.Timeout,
			MaxRequestsHalfOpen: cfg.CircuitBreaker.MaxRequestsHalfOpen,
		}
		meshService = reliability.NewMeshServiceWrapper(baseMeshService, retryCfg, cbCfg, log)
	} else {
		meshService = baseMeshService
	}

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

	// Configure retry and circuit breaker for SFU
	retryCfg := retry.Config{
		Enabled:      cfg.Retry.Enabled,
		MaxAttempts:  cfg.Retry.MaxAttempts,
		InitialDelay: cfg.Retry.InitialDelay,
		MaxDelay:     cfg.Retry.MaxDelay,
		Multiplier:   cfg.Retry.Multiplier,
		Jitter:       cfg.Retry.Jitter,
	}
	cbCfg := circuitbreaker.Config{
		FailureThreshold:   cfg.CircuitBreaker.FailureThreshold,
		SuccessThreshold:   cfg.CircuitBreaker.SuccessThreshold,
		Timeout:            cfg.CircuitBreaker.Timeout,
		MaxRequestsHalfOpen: cfg.CircuitBreaker.MaxRequestsHalfOpen,
	}

	// Initialize SFU
	sfuService := webrtcinfra.NewSFUService(webrtcConfig, qualityService, metricsService, meshService, retryCfg, cbCfg)

	// Initialize monitoring
	_ = monitoring.NewPrometheusCollector()

	// Initialize HTTP handlers
	authHandler := httphandlers.NewAuthHandler(authService)
	streamHandler := httphandlers.NewStreamHandler(streamService, sfuService)

	// Configure Gin
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// CORS middleware (must be first)
	if len(cfg.Auth.AllowedOrigins) > 0 {
		router.Use(middleware.CORSMiddleware(cfg.Auth.AllowedOrigins))
	} else {
		// Allow all origins for development if not configured
		router.Use(middleware.SimpleCORSMiddleware())
	}

	// Setup auth routes FIRST (public) - before any other middleware that might interfere
	// Register directly on router to avoid any group conflicts
	log.Info("Registering auth routes directly on router...")
	router.POST("/api/v1/auth/register", authHandler.Register)
	router.POST("/api/v1/auth/login", authHandler.Login)
	router.POST("/api/v1/auth/refresh", authHandler.RefreshToken)
	log.Info("Auth routes registered: /api/v1/auth/register, /api/v1/auth/login, /api/v1/auth/refresh")

	// Health check endpoint (must be before rate limiting)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
		})
	})
	router.OPTIONS("/health", func(c *gin.Context) {
		c.Status(204)
	})

	// Readiness endpoint (must be before rate limiting)
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
	router.OPTIONS("/ready", func(c *gin.Context) {
		c.Status(204)
	})

	// Prometheus metrics endpoint (must be before rate limiting)
	if cfg.Monitoring.PrometheusEnabled {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
		log.Info("Prometheus metrics enabled")
	}

	// Global HTTP rate limiting (if enabled) - applied after health/metrics/auth endpoints
	router.Use(middleware.NewHTTPRateLimitMiddleware(cfg))

	// Setup stream routes with authentication
	// Register stream routes directly with full path to avoid conflicts with auth routes
	streamAPI := router.Group("/api/v1/streams")
	streamAPI.Use(middleware.AuthMiddleware(authService))
	{
		streamAPI.POST("", streamHandler.CreateStream)
		streamAPI.GET("", streamHandler.ListStreams)
		streamAPI.GET("/:id", streamHandler.GetStream)
		streamAPI.POST("/:id/join", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.JoinStream)
		streamAPI.POST("/:id/leave", streamHandler.LeaveStream)
		streamAPI.GET("/:id/stats", streamHandler.GetStreamStats)
		streamAPI.GET("/:id/webrtc/ready", streamHandler.GetWebRTCReadiness)

		// WebRTC endpoints
		streamAPI.POST("/:id/publisher/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.CreatePublisherOffer)
		streamAPI.POST("/:id/publisher/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.HandlePublisherAnswer)
		streamAPI.POST("/:id/subscriber/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.CreateSubscriberOffer)
		streamAPI.POST("/:id/subscriber/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.HandleSubscriberAnswer)
	}

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
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
