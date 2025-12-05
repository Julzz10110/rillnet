package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
)

type MemoryPeerRepository struct {
	peers map[domain.PeerID]*domain.Peer
	mu    sync.RWMutex
}

func NewMemoryPeerRepository() ports.PeerRepository {
	return &MemoryPeerRepository{
		peers: make(map[domain.PeerID]*domain.Peer),
	}
}

func (r *MemoryPeerRepository) Add(ctx context.Context, peer *domain.Peer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.peers[peer.ID]; exists {
		return fmt.Errorf("peer already exists: %s", peer.ID)
	}

	r.peers[peer.ID] = peer
	return nil
}

func (r *MemoryPeerRepository) GetByID(ctx context.Context, id domain.PeerID) (*domain.Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peer, exists := r.peers[id]
	if !exists {
		return nil, domain.ErrPeerNotFound
	}

	return peer, nil
}

func (r *MemoryPeerRepository) Remove(ctx context.Context, id domain.PeerID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.peers[id]; !exists {
		return domain.ErrPeerNotFound
	}

	delete(r.peers, id)
	return nil
}

func (r *MemoryPeerRepository) FindByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var streamPeers []*domain.Peer
	for _, peer := range r.peers {
		if peer.StreamID == streamID {
			streamPeers = append(streamPeers, peer)
		}
	}

	return streamPeers, nil
}

func (r *MemoryPeerRepository) FindOptimalSource(ctx context.Context, streamID domain.StreamID, excludePeers []domain.PeerID) (*domain.Peer, error) {
	peers, err := r.FindByStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	// Create map of excluded peers for fast lookup
	excludeMap := make(map[domain.PeerID]bool)
	for _, peerID := range excludePeers {
		excludeMap[peerID] = true
	}

	// Filter and sort peers
	var candidates []*domain.Peer
	for _, peer := range peers {
		if !excludeMap[peer.ID] && peer.Capabilities.IsPublisher {
			candidates = append(candidates, peer)
		}
	}

	if len(candidates) == 0 {
		return nil, domain.ErrPeerNotFound
	}

	// Sort by connection quality
	sort.Slice(candidates, func(i, j int) bool {
		return r.calculatePeerScore(candidates[i]) > r.calculatePeerScore(candidates[j])
	})

	return candidates[0], nil
}

func (r *MemoryPeerRepository) UpdateMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	peer, exists := r.peers[peerID]
	if !exists {
		return domain.ErrPeerNotFound
	}

	peer.Metrics = domain.PeerMetrics{
		Bandwidth:   metrics.BandwidthDown,
		PacketLoss:  metrics.PacketLoss,
		Latency:     metrics.Latency,
		CPUUsage:    peer.Metrics.CPUUsage,
		MemoryUsage: peer.Metrics.MemoryUsage,
	}

	return nil
}

func (r *MemoryPeerRepository) UpdatePeerLoad(ctx context.Context, peerID domain.PeerID, load int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.peers[peerID]
	if !exists {
		return domain.ErrPeerNotFound
	}

	// Update peer load
	// In real implementation, more complex logic would be here
	// For now, just verify peer exists
	return nil
}

func (r *MemoryPeerRepository) calculatePeerScore(peer *domain.Peer) float64 {
	score := float64(peer.Metrics.Bandwidth) / 1000.0

	// Consider packet loss (less = better)
	score += (1.0 - peer.Metrics.PacketLoss) * 10.0

	// Consider latency (less = better)
	if peer.Metrics.Latency < 50*time.Millisecond {
		score += 5.0
	} else if peer.Metrics.Latency < 100*time.Millisecond {
		score += 3.0
	} else if peer.Metrics.Latency < 200*time.Millisecond {
		score += 1.0
	}

	return score
}
