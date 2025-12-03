package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	osignal "os/signal"
	"syscall"
	"time"

	"rillnet/internal/core/services"
	repositories "rillnet/internal/infrastructure/repositories"
	signalserver "rillnet/internal/infrastructure/signal"
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
	wsServer := signalserver.NewWebSocketServer(peerRepo, meshService, authService, cfg.Auth.AllowedOrigins)

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
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.HandleWebSocket)
	mux.HandleFunc("/health", wsServer.HealthCheck)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now(),
			"uptime":    time.Since(startTime).String(),
		})
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:    cfg.Signal.Address,
		Handler: mux,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Infof("Starting RillNet Signaling server on %s", cfg.Signal.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signals or server error
	sigChan := make(chan os.Signal, 1)
	osignal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalw("Signal server failed", "error", err)
	case sig := <-sigChan:
		log.Infow("Received shutdown signal", "signal", sig)
	}

	log.Info("Shutting down RillNet Signaling server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Signal.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown WebSocket server gracefully (close all connections)
	if err := wsServer.Shutdown(shutdownCtx); err != nil {
		log.Errorw("Error during WebSocket server shutdown", "error", err)
	}

	// Shutdown HTTP server gracefully
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorw("Error during HTTP server shutdown", "error", err)
		// Force close if graceful shutdown fails
		if closeErr := srv.Close(); closeErr != nil {
			log.Errorw("Error force closing server", "error", closeErr)
		}
	} else {
		log.Info("HTTP server shutdown gracefully")
	}

	// Close repository factory
	if err := repoFactory.Close(); err != nil {
		log.Errorw("Error closing repository factory", "error", err)
	}

	log.Info("RillNet Signaling server stopped")
}
