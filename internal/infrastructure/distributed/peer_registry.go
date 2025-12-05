package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/pkg/distributed"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SharedPeerRegistry provides shared peer registry across instances
type SharedPeerRegistry struct {
	client     *redis.Client
	lockManager *distributed.LockManager
	instanceID string
	logger     *zap.SugaredLogger
	prefix     string
}

// NewSharedPeerRegistry creates a new shared peer registry
func NewSharedPeerRegistry(
	client *redis.Client,
	instanceID string,
	logger *zap.SugaredLogger,
) *SharedPeerRegistry {
	return &SharedPeerRegistry{
		client:      client,
		lockManager: distributed.NewLockManager(client, "rillnet:lock:"),
		instanceID:  instanceID,
		logger:      logger,
		prefix:      "rillnet:peer:",
	}
}

// RegisterPeer registers a peer in the shared registry
func (r *SharedPeerRegistry) RegisterPeer(ctx context.Context, peer *domain.Peer) error {
	// Serialize peer
	data, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("failed to marshal peer: %w", err)
	}

	key := r.peerKey(peer.ID)
	
	// Set peer data with instance ID
	peerData := map[string]interface{}{
		"peer":       string(data),
		"instance_id": r.instanceID,
		"registered_at": time.Now().Unix(),
	}

	peerDataJSON, err := json.Marshal(peerData)
	if err != nil {
		return fmt.Errorf("failed to marshal peer data: %w", err)
	}

	// Store with TTL (e.g., 5 minutes)
	if err := r.client.Set(ctx, key, peerDataJSON, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	// Add to stream peers set
	if peer.StreamID != "" {
		streamKey := r.streamPeersKey(peer.StreamID)
		if err := r.client.SAdd(ctx, streamKey, string(peer.ID)).Err(); err != nil {
			return fmt.Errorf("failed to add peer to stream set: %w", err)
		}
		// Set expiration on stream set
		r.client.Expire(ctx, streamKey, 10*time.Minute)
	}

	// Add to instance peers set
	instanceKey := r.instancePeersKey(r.instanceID)
	if err := r.client.SAdd(ctx, instanceKey, string(peer.ID)).Err(); err != nil {
		return fmt.Errorf("failed to add peer to instance set: %w", err)
	}
	r.client.Expire(ctx, instanceKey, 10*time.Minute)

	return nil
}

// UnregisterPeer unregisters a peer from the shared registry
func (r *SharedPeerRegistry) UnregisterPeer(ctx context.Context, peerID domain.PeerID) error {
	key := r.peerKey(peerID)

	// Get peer data to find stream ID
	peerDataJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil // Already unregistered
	}
	if err != nil {
		return fmt.Errorf("failed to get peer: %w", err)
	}

	var peerData map[string]interface{}
	if err := json.Unmarshal([]byte(peerDataJSON), &peerData); err != nil {
		return fmt.Errorf("failed to unmarshal peer data: %w", err)
	}

	// Get peer JSON to extract stream ID
	var peer domain.Peer
	if peerJSON, ok := peerData["peer"].(string); ok {
		if err := json.Unmarshal([]byte(peerJSON), &peer); err == nil {
			// Remove from stream peers set
			if peer.StreamID != "" {
				streamKey := r.streamPeersKey(peer.StreamID)
				r.client.SRem(ctx, streamKey, string(peerID))
			}
		}
	}

	// Remove from instance peers set
	instanceID, ok := peerData["instance_id"].(string)
	if ok {
		instanceKey := r.instancePeersKey(instanceID)
		r.client.SRem(ctx, instanceKey, string(peerID))
	}

	// Delete peer data
	return r.client.Del(ctx, key).Err()
}

// GetPeer gets a peer from the shared registry
func (r *SharedPeerRegistry) GetPeer(ctx context.Context, peerID domain.PeerID) (*domain.Peer, error) {
	key := r.peerKey(peerID)
	peerDataJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("peer not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	var peerData map[string]interface{}
	if err := json.Unmarshal([]byte(peerDataJSON), &peerData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peer data: %w", err)
	}

	peerJSON, ok := peerData["peer"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid peer data format")
	}

	var peer domain.Peer
	if err := json.Unmarshal([]byte(peerJSON), &peer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peer: %w", err)
	}

	return &peer, nil
}

// FindPeersByStream finds all peers in a stream across all instances
func (r *SharedPeerRegistry) FindPeersByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error) {
	streamKey := r.streamPeersKey(streamID)
	peerIDs, err := r.client.SMembers(ctx, streamKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream peers: %w", err)
	}

	var peers []*domain.Peer
	for _, peerIDStr := range peerIDs {
		peer, err := r.GetPeer(ctx, domain.PeerID(peerIDStr))
		if err != nil {
			// Skip peers that no longer exist
			continue
		}
		peers = append(peers, peer)
	}

	return peers, nil
}

// GetInstancePeers gets all peers registered on a specific instance
func (r *SharedPeerRegistry) GetInstancePeers(ctx context.Context, instanceID string) ([]domain.PeerID, error) {
	instanceKey := r.instancePeersKey(instanceID)
	peerIDs, err := r.client.SMembers(ctx, instanceKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance peers: %w", err)
	}

	result := make([]domain.PeerID, len(peerIDs))
	for i, id := range peerIDs {
		result[i] = domain.PeerID(id)
	}

	return result, nil
}

// RefreshPeer refreshes the TTL of a peer registration
func (r *SharedPeerRegistry) RefreshPeer(ctx context.Context, peerID domain.PeerID) error {
	key := r.peerKey(peerID)
	return r.client.Expire(ctx, key, 5*time.Minute).Err()
}

// CleanupInstancePeers cleans up peers for a specific instance (e.g., on shutdown)
func (r *SharedPeerRegistry) CleanupInstancePeers(ctx context.Context, instanceID string) error {
	instanceKey := r.instancePeersKey(instanceID)
	peerIDs, err := r.client.SMembers(ctx, instanceKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get instance peers: %w", err)
	}

	// Unregister all peers for this instance
	for _, peerIDStr := range peerIDs {
		if err := r.UnregisterPeer(ctx, domain.PeerID(peerIDStr)); err != nil {
			r.logger.Warnw("failed to unregister peer during cleanup",
				"peer_id", peerIDStr,
				"error", err,
			)
		}
	}

	// Delete instance set
	return r.client.Del(ctx, instanceKey).Err()
}

// AcquireStreamLock acquires a distributed lock for stream operations
func (r *SharedPeerRegistry) AcquireStreamLock(ctx context.Context, streamID domain.StreamID, ttl time.Duration) (*distributed.DistributedLock, error) {
	lockKey := fmt.Sprintf("stream:%s", streamID)
	lock := r.lockManager.AcquireLock(lockKey, ttl)
	
	if err := lock.Lock(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire stream lock: %w", err)
	}

	return lock, nil
}

// Helper methods
func (r *SharedPeerRegistry) peerKey(peerID domain.PeerID) string {
	return r.prefix + string(peerID)
}

func (r *SharedPeerRegistry) streamPeersKey(streamID domain.StreamID) string {
	return fmt.Sprintf("rillnet:stream:%s:peers", streamID)
}

func (r *SharedPeerRegistry) instancePeersKey(instanceID string) string {
	return fmt.Sprintf("rillnet:instance:%s:peers", instanceID)
}

