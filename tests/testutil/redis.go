package testutil

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAddr returns the Redis address for integration tests.
func RedisAddr() string {
	if addr := os.Getenv("RILLNET_REDIS_ADDRESS"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

// RedisAvailable reports whether Redis accepts connections.
func RedisAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: RedisAddr()})
	defer client.Close()

	return client.Ping(ctx).Err() == nil
}
