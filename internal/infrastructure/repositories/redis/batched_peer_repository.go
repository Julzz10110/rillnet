package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/batch"

	"github.com/redis/go-redis/v9"
)

// RedisOperation represents a batched Redis operation
type RedisOperation struct {
	Type      string // "set", "sadd", "srem", "del"
	Key       string
	Value     interface{}
	TTL       time.Duration
	client    *redis.Client
}

// Execute executes a single Redis operation
func (op *RedisOperation) Execute(ctx context.Context) error {
	switch op.Type {
	case "set":
		data, ok := op.Value.([]byte)
		if !ok {
			return fmt.Errorf("invalid value type for set operation")
		}
		if op.TTL > 0 {
			return op.client.Set(ctx, op.Key, data, op.TTL).Err()
		}
		return op.client.Set(ctx, op.Key, data, 0).Err()
	case "sadd":
		member, ok := op.Value.(string)
		if !ok {
			return fmt.Errorf("invalid value type for sadd operation")
		}
		return op.client.SAdd(ctx, op.Key, member).Err()
	case "srem":
		member, ok := op.Value.(string)
		if !ok {
			return fmt.Errorf("invalid value type for srem operation")
		}
		return op.client.SRem(ctx, op.Key, member).Err()
	case "del":
		return op.client.Del(ctx, op.Key).Err()
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// RedisBatchProcessor processes batches of Redis operations using pipeline
type RedisBatchProcessor struct {
	client *redis.Client
}

// ProcessBatch processes a batch of Redis operations using pipeline
func (p *RedisBatchProcessor) ProcessBatch(ctx context.Context, operations []batch.Operation) error {
	if len(operations) == 0 {
		return nil
	}

	// Group operations by type for better pipeline efficiency
	pipe := p.client.Pipeline()

	for _, op := range operations {
		if redisOp, ok := op.(*RedisOperation); ok {
			switch redisOp.Type {
			case "set":
				data, ok := redisOp.Value.([]byte)
				if ok {
					if redisOp.TTL > 0 {
						pipe.Set(ctx, redisOp.Key, data, redisOp.TTL)
					} else {
						pipe.Set(ctx, redisOp.Key, data, 0)
					}
				}
			case "sadd":
				if member, ok := redisOp.Value.(string); ok {
					pipe.SAdd(ctx, redisOp.Key, member)
				}
			case "srem":
				if member, ok := redisOp.Value.(string); ok {
					pipe.SRem(ctx, redisOp.Key, member)
				}
			case "del":
				pipe.Del(ctx, redisOp.Key)
			}
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	return err
}

// BatchedRedisPeerRepository wraps RedisPeerRepository with batching
type BatchedRedisPeerRepository struct {
	baseRepo *RedisPeerRepository
	batcher  *batch.Batcher
}

// NewBatchedRedisPeerRepository creates a new batched Redis peer repository
func NewBatchedRedisPeerRepository(
	baseRepo *RedisPeerRepository,
	batchSize int,
	batchInterval time.Duration,
) ports.PeerRepository {
	processor := &RedisBatchProcessor{client: baseRepo.client}
	batcher := batch.NewBatcher(batchSize, batchInterval, processor)

	return &BatchedRedisPeerRepository{
		baseRepo: baseRepo,
		batcher:  batcher,
	}
}

// Add batches peer addition
func (r *BatchedRedisPeerRepository) Add(ctx context.Context, peer *domain.Peer) error {
	// Serialize peer to JSON
	data, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("failed to marshal peer: %w", err)
	}

	// Batch SET operation
	key := r.baseRepo.peerKey(peer.ID)
	op := &RedisOperation{
		Type:   "set",
		Key:    key,
		Value:  data,
		TTL:    0,
		client: r.baseRepo.client,
	}
	if err := r.batcher.Add(op); err != nil {
		return err
	}

	// Batch SADD operation for stream peers set
	if peer.StreamID != "" {
		streamKey := r.baseRepo.streamPeersKey(peer.StreamID)
		op := &RedisOperation{
			Type:   "sadd",
			Key:    streamKey,
			Value:  string(peer.ID),
			client: r.baseRepo.client,
		}
		_ = r.batcher.Add(op)
	}

	return nil
}

// GetByID gets peer by ID (not batched, immediate)
func (r *BatchedRedisPeerRepository) GetByID(ctx context.Context, id domain.PeerID) (*domain.Peer, error) {
	return r.baseRepo.GetByID(ctx, id)
}

// Remove batches peer removal
func (r *BatchedRedisPeerRepository) Remove(ctx context.Context, id domain.PeerID) error {
	// Get peer first to get stream ID
	peer, err := r.baseRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Batch DEL operation
	key := r.baseRepo.peerKey(id)
	op := &RedisOperation{
		Type:   "del",
		Key:    key,
		client: r.baseRepo.client,
	}
	if err := r.batcher.Add(op); err != nil {
		return err
	}

	// Batch SREM operation for stream peers set
	if peer.StreamID != "" {
		streamKey := r.baseRepo.streamPeersKey(peer.StreamID)
		op := &RedisOperation{
			Type:   "srem",
			Key:    streamKey,
			Value:  string(id),
			client: r.baseRepo.client,
		}
		_ = r.batcher.Add(op)
	}

	return nil
}

// FindByStream finds peers by stream (not batched, immediate)
func (r *BatchedRedisPeerRepository) FindByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error) {
	return r.baseRepo.FindByStream(ctx, streamID)
}

// FindOptimalSource finds optimal source (not batched, immediate)
func (r *BatchedRedisPeerRepository) FindOptimalSource(ctx context.Context, streamID domain.StreamID, excludePeers []domain.PeerID) (*domain.Peer, error) {
	return r.baseRepo.FindOptimalSource(ctx, streamID, excludePeers)
}

// UpdateMetrics batches metrics update
func (r *BatchedRedisPeerRepository) UpdateMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	// Get peer first
	peer, err := r.baseRepo.GetByID(ctx, peerID)
	if err != nil {
		return err
	}

	// Update metrics in memory
	peer.Metrics = domain.PeerMetrics{
		Bandwidth:   metrics.BandwidthDown,
		PacketLoss:  metrics.PacketLoss,
		Latency:     metrics.Latency,
		CPUUsage:    peer.Metrics.CPUUsage,
		MemoryUsage: peer.Metrics.MemoryUsage,
	}

	// Batch SET operation with updated peer
	data, err := json.Marshal(peer)
	if err != nil {
		return fmt.Errorf("failed to marshal peer: %w", err)
	}

	key := r.baseRepo.peerKey(peerID)
	op := &RedisOperation{
		Type:   "set",
		Key:    key,
		Value:  data,
		TTL:    0,
		client: r.baseRepo.client,
	}
	return r.batcher.Add(op)
}

// UpdatePeerLoad batches peer load update
func (r *BatchedRedisPeerRepository) UpdatePeerLoad(ctx context.Context, peerID domain.PeerID, load int) error {
	// Similar to UpdateMetrics
	return r.UpdateMetrics(ctx, peerID, domain.NetworkMetrics{})
}

// Flush flushes all pending operations
func (r *BatchedRedisPeerRepository) Flush(ctx context.Context) error {
	return r.batcher.Flush(ctx)
}

// Stop stops the batcher
func (r *BatchedRedisPeerRepository) Stop() {
	r.batcher.Stop()
}

