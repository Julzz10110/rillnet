package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rillnet/internal/core/services"
	"rillnet/internal/handlers/http"
	"rillnet/internal/infrastructure/monitoring"
	"rillnet/internal/infrastructure/repositories/memory"
	webrtcinfra "rillnet/internal/infrastructure/webrtc"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
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
		cfg.Server.Address = ":8080"
		cfg.Logging.Level = "info"
	}

	// Initialize logger
	zapLogger := logger.New(cfg.Logging.Level)
	defer zapLogger.Sync()

	log := zapLogger.Sugar()

	// Initialize repositories
	streamRepo := memory.NewMemoryStreamRepository()
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()

	// Initialize services
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := services.NewMeshService(peerRepo, meshRepo)
	streamService := services.NewStreamService(streamRepo, peerRepo, meshRepo, meshService, metricsService)

	// WebRTC configuration
	webrtcConfig := webrtcinfra.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
		Simulcast:  cfg.WebRTC.Simulcast,
		MaxBitrate: cfg.WebRTC.MaxBitrate,
	}

	// Initialize SFU
	sfuService := webrtcinfra.NewSFUService(webrtcConfig, qualityService, metricsService, meshService)

	// Initialize monitoring
	prometheusCollector := monitoring.NewPrometheusCollector()

	fmt.Print("Prometheus Collector:", prometheusCollector)

	// Initialize HTTP handlers
	streamHandler := http.NewStreamHandler(streamService, sfuService)

	// Configure Gin
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// Setup routes
	streamHandler.SetupRoutes(router)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "timestamp": time.Now()})
	})

	// Prometheus metrics endpoint
	if cfg.Monitoring.PrometheusEnabled {
		// Prometheus handler should be added here
		// router.GET("/metrics", gin.WrapH(promhttp.Handler()))
		log.Info("Prometheus metrics enabled")
	}

	// Start server
	go func() {
		log.Infof("Starting RillNet Ingest server on %s", cfg.Server.Address)
		if err := router.Run(cfg.Server.Address); err != nil {
			log.Fatal("Server failed:", err)
		}
	}()

	// Wait for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down RillNet Ingest server...")
}
