package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rillnet/internal/core/services"
	signalserver "rillnet/internal/infrastructure/signal"
	repositories "rillnet/internal/infrastructure/repositories"
	"rillnet/pkg/config"
	"rillnet/pkg/logger"
)

// SignalTestServer wraps an httptest server for the signal service.
type SignalTestServer struct {
	Server  *httptest.Server
	Factory *repositories.RepositoryFactory
}

// Close shuts down the server and Redis connection.
func (s *SignalTestServer) Close() {
	if s.Server != nil {
		s.Server.Close()
	}
	if s.Factory != nil {
		_ = s.Factory.Close()
	}
}

// NewSignalTestServer starts signal /ws and /ready endpoints.
func NewSignalTestServer(t *testing.T, cfg *config.Config) *SignalTestServer {
	t.Helper()

	log := logger.New("error").Sugar()
	factory, err := repositories.NewRepositoryFactory(cfg, log)
	if err != nil {
		t.Fatalf("repository factory: %v", err)
	}

	peerRepo := factory.CreatePeerRepository()
	meshRepo := factory.CreateMeshRepository()
	meshService := services.NewMeshService(peerRepo, meshRepo, cfg.Mesh, log)
	authService := services.NewAuthService(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
		nil,
	)

	wsServer := signalserver.NewWebSocketServer(peerRepo, meshService, authService, cfg.Auth.AllowedOrigins)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.HandleWebSocket)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := factory.HealthCheck(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return &SignalTestServer{
		Server:  httptest.NewServer(mux),
		Factory: factory,
	}
}
