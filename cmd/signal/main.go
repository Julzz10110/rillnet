package main

import (
	"encoding/json"
	"net/http"
	"time"

	"rillnet/internal/core/services"
	repositories "rillnet/internal/infrastructure/repositories"
	"rillnet/internal/infrastructure/signal"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"
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
	peerRepo := repoFactory.CreatePeerRepository()
	meshRepo := repoFactory.CreateMeshRepository()

	// Initialize mesh service
	meshService := services.NewMeshService(peerRepo, meshRepo)

	// Initialize auth service (stream service not needed for signal server)
	authService := services.NewAuthService(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
		nil, // Stream service not needed for WebSocket token validation
	)

	// Initialize WebSocket server
	wsServer := signal.NewWebSocketServer(peerRepo, meshService, authService, cfg.Auth.AllowedOrigins)

	// Configure ping/pong intervals from config
	if cfg.Signal.PingInterval > 0 {
		wsServer.SetPingInterval(cfg.Signal.PingInterval)
	}
	if cfg.Signal.PongTimeout > 0 {
		wsServer.SetPongTimeout(cfg.Signal.PongTimeout)
	}

	// Configure rate limiting for WebSocket server from config
	if cfg.RateLimiting.Enabled {
		if cfg.RateLimiting.WebSocket.ConnectionsPerMinute > 0 {
			wsServer.SetConnectionRateLimit(cfg.RateLimiting.WebSocket.ConnectionsPerMinute)
		}
		if cfg.RateLimiting.WebSocket.MessagesPerSecond > 0 && cfg.RateLimiting.WebSocket.Burst > 0 {
			wsServer.SetMessageRateLimit(cfg.RateLimiting.WebSocket.MessagesPerSecond, cfg.RateLimiting.WebSocket.Burst)
		}
		if cfg.RateLimiting.WebSocket.MaxConcurrent > 0 {
			wsServer.SetMaxConcurrentConnections(cfg.RateLimiting.WebSocket.MaxConcurrent)
		}
		if cfg.RateLimiting.WebSocket.MaxMessageSizeBytes > 0 {
			wsServer.SetMaxMessageSize(cfg.RateLimiting.WebSocket.MaxMessageSizeBytes)
		}
	}

	// Setup HTTP routes
	http.HandleFunc("/ws", wsServer.HandleWebSocket)
	http.HandleFunc("/health", wsServer.HealthCheck)
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
		})
	})

	log.Infof("Starting RillNet Signaling server on %s", cfg.Signal.Address)
	if err := http.ListenAndServe(cfg.Signal.Address, nil); err != nil {
		log.Fatalf("Signal server failed: %v", err)
	}
}
