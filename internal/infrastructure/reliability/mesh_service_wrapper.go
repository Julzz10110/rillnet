package reliability

import (
	"context"
	"sync"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/retry"

	"go.uber.org/zap"
)

// MeshServiceWrapper wraps a MeshService with retry logic and circuit breaker
type MeshServiceWrapper struct {
	service ports.MeshService
	logger  *zap.SugaredLogger

	retryConfig       retry.Config
	circuitBreaker    *circuitbreaker.CircuitBreaker
	peerBreakers      map[domain.PeerID]*circuitbreaker.CircuitBreaker
	peerBreakersMu    sync.RWMutex
}

// NewMeshServiceWrapper creates a new wrapper with retry and circuit breaker
func NewMeshServiceWrapper(
	service ports.MeshService,
	retryConfig retry.Config,
	cbConfig circuitbreaker.Config,
	logger *zap.SugaredLogger,
) *MeshServiceWrapper {
	wrapper := &MeshServiceWrapper{
		service:      service,
		logger:       logger,
		retryConfig:  retryConfig,
		circuitBreaker: circuitbreaker.New(cbConfig),
		peerBreakers:  make(map[domain.PeerID]*circuitbreaker.CircuitBreaker),
	}

	// Set up state change callback for global circuit breaker
	wrapper.circuitBreaker.OnStateChange(func(from, to circuitbreaker.State) {
		logger.Infow("circuit breaker state changed",
			"from", from.String(),
			"to", to.String(),
		)
	})

	return wrapper
}

// getPeerCircuitBreaker gets or creates a circuit breaker for a specific peer
func (w *MeshServiceWrapper) getPeerCircuitBreaker(peerID domain.PeerID) *circuitbreaker.CircuitBreaker {
	w.peerBreakersMu.RLock()
	cb, exists := w.peerBreakers[peerID]
	w.peerBreakersMu.RUnlock()

	if exists {
		return cb
	}

	// Create new circuit breaker for this peer
	w.peerBreakersMu.Lock()
	defer w.peerBreakersMu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := w.peerBreakers[peerID]; exists {
		return cb
	}

	cb = circuitbreaker.New(circuitbreaker.DefaultConfig())
	cb.OnStateChange(func(from, to circuitbreaker.State) {
		w.logger.Infow("peer circuit breaker state changed",
			"peer_id", peerID,
			"from", from.String(),
			"to", to.String(),
		)
	})

	w.peerBreakers[peerID] = cb
	return cb
}

// AddPeer adds a peer with retry logic
func (w *MeshServiceWrapper) AddPeer(ctx context.Context, peer *domain.Peer) error {
	if !w.retryConfig.Enabled {
		return w.service.AddPeer(ctx, peer)
	}

	return retry.Retry(ctx, w.retryConfig, func() error {
		return w.circuitBreaker.Execute(ctx, func() error {
			return w.service.AddPeer(ctx, peer)
		})
	})
}

// RemovePeer removes a peer with retry logic
func (w *MeshServiceWrapper) RemovePeer(ctx context.Context, peerID domain.PeerID) error {
	if !w.retryConfig.Enabled {
		return w.service.RemovePeer(ctx, peerID)
	}

	return retry.Retry(ctx, w.retryConfig, func() error {
		return w.circuitBreaker.Execute(ctx, func() error {
			return w.service.RemovePeer(ctx, peerID)
		})
	})
}

// UpdatePeerMetrics updates peer metrics with retry logic
func (w *MeshServiceWrapper) UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	if !w.retryConfig.Enabled {
		return w.service.UpdatePeerMetrics(ctx, peerID, metrics)
	}

	return retry.Retry(ctx, w.retryConfig, func() error {
		return w.circuitBreaker.Execute(ctx, func() error {
			return w.service.UpdatePeerMetrics(ctx, peerID, metrics)
		})
	})
}

// FindOptimalSources finds optimal sources with retry logic
func (w *MeshServiceWrapper) FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error) {
	if !w.retryConfig.Enabled {
		return w.service.FindOptimalSources(ctx, streamID, targetPeer, count)
	}

	result, err := retry.RetryWithResult(ctx, w.retryConfig, func() ([]*domain.Peer, error) {
		res, err := w.circuitBreaker.ExecuteWithResult(ctx, func() (interface{}, error) {
			return w.service.FindOptimalSources(ctx, streamID, targetPeer, count)
		})
		if err != nil {
			return nil, err
		}
		return res.([]*domain.Peer), nil
	})
	return result, err
}

// BuildOptimalMesh builds optimal mesh with retry logic
func (w *MeshServiceWrapper) BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error {
	if !w.retryConfig.Enabled {
		return w.service.BuildOptimalMesh(ctx, streamID)
	}

	return retry.Retry(ctx, w.retryConfig, func() error {
		return w.circuitBreaker.Execute(ctx, func() error {
			return w.service.BuildOptimalMesh(ctx, streamID)
		})
	})
}

// AddConnection adds a connection with retry logic and per-peer circuit breaker
func (w *MeshServiceWrapper) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	if !w.retryConfig.Enabled {
		return w.service.AddConnection(ctx, conn)
	}

	// Use per-peer circuit breaker for connections
	peerCB := w.getPeerCircuitBreaker(conn.FromPeer)

	return retry.Retry(ctx, w.retryConfig, func() error {
		return peerCB.Execute(ctx, func() error {
			return w.service.AddConnection(ctx, conn)
		})
	})
}

// RemoveConnection removes a connection with retry logic
func (w *MeshServiceWrapper) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	if !w.retryConfig.Enabled {
		return w.service.RemoveConnection(ctx, fromPeer, toPeer)
	}

	peerCB := w.getPeerCircuitBreaker(fromPeer)

	return retry.Retry(ctx, w.retryConfig, func() error {
		return peerCB.Execute(ctx, func() error {
			return w.service.RemoveConnection(ctx, fromPeer, toPeer)
		})
	})
}

// GetPeerConnections gets peer connections (no retry needed for read operations)
func (w *MeshServiceWrapper) GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	return w.service.GetPeerConnections(ctx, peerID)
}

// GetOptimalPath gets optimal path (no retry needed for read operations)
func (w *MeshServiceWrapper) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	return w.service.GetOptimalPath(ctx, sourcePeer, targetPeer)
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (w *MeshServiceWrapper) GetCircuitBreakerStats() circuitbreaker.Stats {
	return w.circuitBreaker.GetStats()
}

// GetPeerCircuitBreakerStats returns circuit breaker statistics for a specific peer
func (w *MeshServiceWrapper) GetPeerCircuitBreakerStats(peerID domain.PeerID) (circuitbreaker.Stats, bool) {
	w.peerBreakersMu.RLock()
	defer w.peerBreakersMu.RUnlock()

	cb, exists := w.peerBreakers[peerID]
	if !exists {
		return circuitbreaker.Stats{}, false
	}

	return cb.GetStats(), true
}

