package main

import (
	"encoding/json"
	"net/http"
	"time"

	"rillnet/internal/core/services"
	"rillnet/internal/infrastructure/repositories/memory"
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

	// Initialize repositories
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()

	// Initialize mesh service
	meshService := services.NewMeshService(peerRepo, meshRepo)

	// Initialize WebSocket server
	wsServer := signal.NewWebSocketServer(peerRepo, meshService)

	// Configure ping/pong intervals from config
	if cfg.Signal.PingInterval > 0 {
		wsServer.SetPingInterval(cfg.Signal.PingInterval)
	}
	if cfg.Signal.PongTimeout > 0 {
		wsServer.SetPongTimeout(cfg.Signal.PongTimeout)
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
