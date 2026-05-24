package testutil

import (
	"context"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	httphandlers "rillnet/internal/handlers/http"
	"rillnet/internal/infrastructure/middleware"
	repositories "rillnet/internal/infrastructure/repositories"
	webrtcinfra "rillnet/internal/infrastructure/webrtc"
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"
	"rillnet/pkg/retry"

	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
	"go.uber.org/zap"
)

// IngestTestEnv holds a test ingest HTTP router and dependencies.
type IngestTestEnv struct {
	Router      *gin.Engine
	Factory     *repositories.RepositoryFactory
	StreamRepo  ports.StreamRepository
	AuthService services.AuthService
	Logger      *zap.SugaredLogger
}

// Close releases resources.
func (e *IngestTestEnv) Close() {
	if e.Factory != nil {
		_ = e.Factory.Close()
	}
}

// NewIngestTestEnv builds an ingest API router matching production wiring.
func NewIngestTestEnv(t *testing.T, cfg *config.Config) *IngestTestEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	log := logger.New("error").Sugar()
	factory, err := repositories.NewRepositoryFactory(cfg, log)
	if err != nil {
		t.Fatalf("repository factory: %v", err)
	}

	streamRepo := factory.CreateStreamRepository()
	peerRepo := factory.CreatePeerRepository()
	meshRepo := factory.CreateMeshRepository()

	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := services.NewMeshService(peerRepo, meshRepo, cfg.Mesh, log)
	streamService := services.NewStreamService(streamRepo, peerRepo, meshRepo, meshService, metricsService)
	authService := services.NewAuthService(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
		streamService,
	)

	var iceServers []webrtc.ICEServer
	for _, s := range cfg.WebRTC.ICEServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}
	if len(iceServers) == 0 {
		iceServers = []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}
	}

	webrtcConfig := webrtcinfra.WebRTCConfig{
		ICEServers: iceServers,
		Simulcast:  cfg.WebRTC.Simulcast,
		MaxBitrate: cfg.WebRTC.MaxBitrate,
	}

	retryCfg := retry.Config{
		Enabled:      cfg.Retry.Enabled,
		MaxAttempts:  cfg.Retry.MaxAttempts,
		InitialDelay: cfg.Retry.InitialDelay,
		MaxDelay:     cfg.Retry.MaxDelay,
		Multiplier:   cfg.Retry.Multiplier,
		Jitter:       cfg.Retry.Jitter,
	}
	cbCfg := circuitbreaker.Config{
		FailureThreshold:    cfg.CircuitBreaker.FailureThreshold,
		SuccessThreshold:    cfg.CircuitBreaker.SuccessThreshold,
		Timeout:             cfg.CircuitBreaker.Timeout,
		MaxRequestsHalfOpen: cfg.CircuitBreaker.MaxRequestsHalfOpen,
	}
	sfuService := webrtcinfra.NewSFUService(webrtcConfig, qualityService, metricsService, meshService, retryCfg, cbCfg)

	authHandler := httphandlers.NewAuthHandler(authService)
	streamHandler := httphandlers.NewStreamHandler(streamService, sfuService)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.ErrorHandlerMiddleware(log))
	router.Use(middleware.SimpleCORSMiddleware())

	router.POST("/api/v1/auth/register", authHandler.Register)
	router.POST("/api/v1/auth/login", authHandler.Login)
	router.POST("/api/v1/auth/refresh", authHandler.RefreshToken)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	router.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := factory.HealthCheck(ctx); err != nil {
			c.JSON(503, gin.H{"status": "not_ready", "dependencies": "unhealthy"})
			return
		}
		c.JSON(200, gin.H{"status": "ready", "dependencies": "ok"})
	})

	streamAPI := router.Group("/api/v1/streams")
	streamAPI.Use(middleware.AuthMiddleware(authService))
	{
		streamAPI.POST("", streamHandler.CreateStream)
		streamAPI.GET("", streamHandler.ListStreams)
		streamAPI.GET("/:id", streamHandler.GetStream)
		streamAPI.POST("/:id/join", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.JoinStream)
		streamAPI.POST("/:id/leave", streamHandler.LeaveStream)
		streamAPI.GET("/:id/stats", streamHandler.GetStreamStats)
		streamAPI.POST("/:id/publisher/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.CreatePublisherOffer)
		streamAPI.POST("/:id/publisher/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleOwner), streamHandler.HandlePublisherAnswer)
		streamAPI.POST("/:id/subscriber/offer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.CreateSubscriberOffer)
		streamAPI.POST("/:id/subscriber/answer", middleware.StreamPermissionMiddleware(authService, domain.RoleViewer), streamHandler.HandleSubscriberAnswer)
	}

	return &IngestTestEnv{
		Router:      router,
		Factory:     factory,
		StreamRepo:  streamRepo,
		AuthService: authService,
		Logger:      log,
	}
}

// SmokeTestConfig returns config for stack smoke integration tests.
func SmokeTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Auth.JWTSecret = "smoke-test-jwt-secret-min-32-chars"
	cfg.Auth.AccessTokenTTL = 15 * time.Minute
	cfg.Auth.RefreshTokenTTL = 24 * time.Hour
	cfg.Redis.Enabled = true
	cfg.Redis.Address = RedisAddr()
	cfg.Redis.DB = 15 // isolated test DB index
	return cfg
}
