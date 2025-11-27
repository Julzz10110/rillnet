package main

import (
	"log"
	"net/http"

	"rillnet/internal/core/services"
	"rillnet/internal/infrastructure/repositories/memory"
	"rillnet/internal/infrastructure/signal"
	"rillnet/pkg/config"
)

func main() {
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
			log.Printf("Loaded config from: %s", path)
			break
		}
	}

	if err != nil {
		log.Printf("Could not load config from any path, using defaults")
		// Create default configuration
		cfg = &config.Config{}
		cfg.Signal.Address = ":8081"
		cfg.Logging.Level = "info"
	}

	// Initialize repositories
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()

	// Initialize mesh service
	meshService := services.NewMeshService(peerRepo, meshRepo)

	// Initialize WebSocket server
	wsServer := signal.NewWebSocketServer(peerRepo, meshService)

	// Setup HTTP routes
	http.HandleFunc("/ws", wsServer.HandleWebSocket)
	http.HandleFunc("/health", wsServer.HealthCheck)

	log.Printf("Starting RillNet Signaling server on %s", cfg.Signal.Address)
	log.Fatal(http.ListenAndServe(cfg.Signal.Address, nil))
}
