package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/redis/go-redis/v9"
)

type RedisStreamRepository struct {
	client *redis.Client
	prefix string
}

func NewRedisStreamRepository(client *redis.Client) ports.StreamRepository {
	return &RedisStreamRepository{
		client: client,
		prefix: "rillnet:stream:",
	}
}

func (r *RedisStreamRepository) streamKey(id domain.StreamID) string {
	return r.prefix + string(id)
}

func (r *RedisStreamRepository) activeStreamsKey() string {
	return r.prefix + "active"
}

func (r *RedisStreamRepository) Create(ctx context.Context, stream *domain.Stream) error {
	// Serialize stream to JSON
	data, err := json.Marshal(stream)
	if err != nil {
		return fmt.Errorf("failed to marshal stream: %w", err)
	}

	// Store stream data
	key := r.streamKey(stream.ID)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to set stream in Redis: %w", err)
	}

	// Add to active streams set if active
	if stream.Active {
		activeKey := r.activeStreamsKey()
		if err := r.client.SAdd(ctx, activeKey, string(stream.ID)).Err(); err != nil {
			return fmt.Errorf("failed to add stream to active set: %w", err)
		}
	}

	return nil
}

func (r *RedisStreamRepository) GetByID(ctx context.Context, id domain.StreamID) (*domain.Stream, error) {
	key := r.streamKey(id)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, domain.ErrStreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stream from Redis: %w", err)
	}

	var stream domain.Stream
	if err := json.Unmarshal([]byte(data), &stream); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream: %w", err)
	}

	return &stream, nil
}

func (r *RedisStreamRepository) Update(ctx context.Context, stream *domain.Stream) error {
	// Check if stream exists
	_, err := r.GetByID(ctx, stream.ID)
	if err != nil {
		return err
	}

	// Serialize stream to JSON
	data, err := json.Marshal(stream)
	if err != nil {
		return fmt.Errorf("failed to marshal stream: %w", err)
	}

	// Update stream data
	key := r.streamKey(stream.ID)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to update stream in Redis: %w", err)
	}

	// Update active streams set
	activeKey := r.activeStreamsKey()
	if stream.Active {
		if err := r.client.SAdd(ctx, activeKey, string(stream.ID)).Err(); err != nil {
			return fmt.Errorf("failed to add stream to active set: %w", err)
		}
	} else {
		if err := r.client.SRem(ctx, activeKey, string(stream.ID)).Err(); err != nil {
			return fmt.Errorf("failed to remove stream from active set: %w", err)
		}
	}

	return nil
}

func (r *RedisStreamRepository) Delete(ctx context.Context, id domain.StreamID) error {
	// Remove from active streams set
	activeKey := r.activeStreamsKey()
	if err := r.client.SRem(ctx, activeKey, string(id)).Err(); err != nil {
		return fmt.Errorf("failed to remove stream from active set: %w", err)
	}

	// Delete stream data
	key := r.streamKey(id)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete stream from Redis: %w", err)
	}

	return nil
}

func (r *RedisStreamRepository) ListActive(ctx context.Context) ([]*domain.Stream, error) {
	activeKey := r.activeStreamsKey()
	streamIDs, err := r.client.SMembers(ctx, activeKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get active streams from Redis: %w", err)
	}

	var streams []*domain.Stream
	for _, streamIDStr := range streamIDs {
		stream, err := r.GetByID(ctx, domain.StreamID(streamIDStr))
		if err != nil {
			// Skip streams that no longer exist
			continue
		}
		if stream.Active {
			streams = append(streams, stream)
		}
	}

	return streams, nil
}