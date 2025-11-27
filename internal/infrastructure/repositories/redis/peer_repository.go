package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/redis/go-redis/v9"
)

type RedisPeerRepository struct {
	client *redis.Client
	prefix string
}

func NewRedisPeerRepository(client *redis.Client) ports.PeerRepository {
	return &RedisPeerRepository{
		client: client,
		prefix: "rillnet:peer:",
	}
}

func (r *RedisPeerRepository) peerKey(id domain.PeerID) string {
	return r.prefix + string(id)
}

func (r *RedisPeerRepository) streamPeersKey(streamID domain.StreamID) string {
	return fmt.Sprintf("rillnet:stream:%s:peers", streamID)
}

func (r *RedisPeerRepository) Add(ctx context.Context, peer *domain.Peer) error {
	// Serialize peer to JSON
	data, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("failed to marshal peer: %w", err)
	}

	// Store peer data
	key := r.peerKey(peer.ID)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to set peer in Redis: %w", err)
	}

	// Add to stream peers set
	if peer.StreamID != "" {
		streamKey := r.streamPeersKey(peer.StreamID)
		if err := r.client.SAdd(ctx, streamKey, string(peer.ID)).Err(); err != nil {
			return fmt.Errorf("failed to add peer to stream set: %w", err)
		}
	}

	return nil
}

func (r *RedisPeerRepository) GetByID(ctx context.Context, id domain.PeerID) (*domain.Peer, error) {
	key := r.peerKey(id)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, domain.ErrPeerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get peer from Redis: %w", err)
	}

	var peer domain.Peer
	if err := json.Unmarshal([]byte(data), &peer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peer: %w", err)
	}

	return &peer, nil
}

func (r *RedisPeerRepository) Remove(ctx context.Context, id domain.PeerID) error {
	// Get peer to find stream ID
	peer, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Remove from stream peers set
	if peer.StreamID != "" {
		streamKey := r.streamPeersKey(peer.StreamID)
		if err := r.client.SRem(ctx, streamKey, string(id)).Err(); err != nil {
			return fmt.Errorf("failed to remove peer from stream set: %w", err)
		}
	}

	// Remove peer data
	key := r.peerKey(id)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete peer from Redis: %w", err)
	}

	return nil
}

func (r *RedisPeerRepository) FindByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error) {
	streamKey := r.streamPeersKey(streamID)
	peerIDs, err := r.client.SMembers(ctx, streamKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream peers from Redis: %w", err)
	}

	var peers []*domain.Peer
	for _, peerIDStr := range peerIDs {
		peer, err := r.GetByID(ctx, domain.PeerID(peerIDStr))
		if err != nil {
			// Skip peers that no longer exist
			continue
		}
		peers = append(peers, peer)
	}

	return peers, nil
}

func (r *RedisPeerRepository) FindOptimalSource(ctx context.Context, streamID domain.StreamID, excludePeers []domain.PeerID) (*domain.Peer, error) {
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

func (r *RedisPeerRepository) UpdateMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	peer, err := r.GetByID(ctx, peerID)
	if err != nil {
		return err
	}

	// Update metrics
	peer.Metrics = domain.PeerMetrics{
		Bandwidth:   metrics.BandwidthDown,
		PacketLoss:  metrics.PacketLoss,
		Latency:     metrics.Latency,
		CPUUsage:    peer.Metrics.CPUUsage,
		MemoryUsage: peer.Metrics.MemoryUsage,
	}
	peer.LastSeen = time.Now()

	// Save updated peer
	return r.Add(ctx, peer)
}

func (r *RedisPeerRepository) UpdatePeerLoad(ctx context.Context, peerID domain.PeerID, load int) error {
	// Load is stored as part of peer metrics
	// In a full implementation, this would update a separate load field
	peer, err := r.GetByID(ctx, peerID)
	if err != nil {
		return err
	}

	// Update and save (load can be stored in a separate Redis key or as part of peer)
	// For now, just update LastSeen
	peer.LastSeen = time.Now()
	return r.Add(ctx, peer)
}

func (r *RedisPeerRepository) calculatePeerScore(peer *domain.Peer) float64 {
	score := float64(peer.Metrics.Bandwidth) / 1000.0

	// Account for packet loss (lower = better)
	score += (1.0 - peer.Metrics.PacketLoss) * 10.0

	// Account for latency (lower = better)
	if peer.Metrics.Latency < 50*time.Millisecond {
		score += 5.0
	} else if peer.Metrics.Latency < 100*time.Millisecond {
		score += 3.0
	} else if peer.Metrics.Latency < 200*time.Millisecond {
		score += 1.0
	}

	return score
}