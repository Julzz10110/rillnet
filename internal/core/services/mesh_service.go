package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
)

type meshService struct {
	peerRepo ports.PeerRepository
	meshRepo ports.MeshRepository
}

func NewMeshService(peerRepo ports.PeerRepository, meshRepo ports.MeshRepository) ports.MeshService {
	return &meshService{
		peerRepo: peerRepo,
		meshRepo: meshRepo,
	}
}

func (m *meshService) AddPeer(ctx context.Context, peer *domain.Peer) error {
	return m.peerRepo.Add(ctx, peer)
}

func (m *meshService) RemovePeer(ctx context.Context, peerID domain.PeerID) error {
	// Get all peer connections
	connections, err := m.meshRepo.GetConnections(ctx, peerID)
	if err != nil {
		return err
	}

	// Remove all connections
	for _, conn := range connections {
		if err := m.meshRepo.RemoveConnection(ctx, conn.FromPeer, conn.ToPeer); err != nil {
			return err
		}
	}

	// Remove peer
	return m.peerRepo.Remove(ctx, peerID)
}

func (m *meshService) UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	return m.peerRepo.UpdateMetrics(ctx, peerID, metrics)
}

func (m *meshService) FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error) {
	// Get all peers in the stream
	allPeers, err := m.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	// Exclude target peer and unsuitable candidates
	var candidates []*domain.Peer
	for _, peer := range allPeers {
		if peer.ID != targetPeer && peer.Capabilities.IsPublisher && peer.Metrics.Bandwidth > 0 {
			candidates = append(candidates, peer)
		}
	}

	if len(candidates) == 0 {
		return nil, domain.ErrPeerNotFound
	}

	// Sort by connection quality
	sort.Slice(candidates, func(i, j int) bool {
		return m.calculatePeerScore(candidates[i]) > m.calculatePeerScore(candidates[j])
	})

	// Return best candidates
	if len(candidates) > count {
		return candidates[:count], nil
	}
	return candidates, nil
}

func (m *meshService) BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error {
	peers, err := m.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Simple mesh building algorithm: each subscriber connects to 3-4 best sources
	for _, peer := range peers {
		if !peer.Capabilities.IsPublisher {
			// Find optimal sources for this subscriber
			sources, err := m.FindOptimalSources(ctx, streamID, peer.ID, 4)
			if err != nil {
				// Skip if no sources found
				continue
			}

			// Create connections with found sources
			for _, source := range sources {
				conn := &domain.PeerConnection{
					FromPeer:  source.ID,
					ToPeer:    peer.ID,
					Direction: domain.DirectionOutbound,
					Quality:   domain.StreamQuality{Quality: "auto"},
					OpenedAt:  time.Now(),
					Bitrate:   source.Metrics.Bandwidth,
				}

				if err := m.meshRepo.AddConnection(ctx, conn); err != nil {
					// Log error but continue with other connections
					fmt.Printf("Failed to add connection from %s to %s: %v\n", source.ID, peer.ID, err)
				}
			}
		}
	}

	return nil
}

func (m *meshService) calculatePeerScore(peer *domain.Peer) float64 {
	score := 0.0

	// High bandwidth = high score
	score += float64(peer.Metrics.Bandwidth) / 100.0

	// Low packet loss = high score
	score += (1.0 - peer.Metrics.PacketLoss) * 50.0

	// Low latency = high score
	if peer.Metrics.Latency < 50*time.Millisecond {
		score += 30.0
	} else if peer.Metrics.Latency < 100*time.Millisecond {
		score += 20.0
	} else if peer.Metrics.Latency < 200*time.Millisecond {
		score += 10.0
	}

	// Publisher gets bonus
	if peer.Capabilities.IsPublisher {
		score += 25.0
	}

	// Peers with relay capability get bonus
	if peer.Capabilities.CanRelay {
		score += 15.0
	}

	return score
}

// Additional methods for mesh network operations
func (m *meshService) GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	return m.meshRepo.GetConnections(ctx, peerID)
}

func (m *meshService) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	return m.meshRepo.AddConnection(ctx, conn)
}

func (m *meshService) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	return m.meshRepo.RemoveConnection(ctx, fromPeer, toPeer)
}

func (m *meshService) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	return m.meshRepo.GetOptimalPath(ctx, sourcePeer, targetPeer)
}
