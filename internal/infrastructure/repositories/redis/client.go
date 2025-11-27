package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(address, password string, db, poolSize int, logger *zap.SugaredLogger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Run migrations
	if err := Migrate(ctx, client, logger); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	if logger != nil {
		logger.Infow("connected to Redis",
			"address", address,
			"db", db,
			"pool_size", poolSize,
		)
	}

	return client, nil
}

// Close closes the Redis client connection
func CloseRedisClient(client *redis.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}

