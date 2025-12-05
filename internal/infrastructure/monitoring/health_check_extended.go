package monitoring

import (
	"context"
	"time"

	"rillnet/internal/core/ports"
	"github.com/redis/go-redis/v9"
)

// AddRedisCheck adds a Redis health check
func (h *HealthChecker) AddRedisCheck(client *redis.Client, interval, timeout time.Duration) {
	h.AddCheck("redis", func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			return false, err
		}
		return true, nil
	}, interval, timeout)
}

// AddRepositoryCheck adds a repository health check
func (h *HealthChecker) AddRepositoryCheck(repo ports.StreamRepository, interval, timeout time.Duration) {
	h.AddCheck("repository", func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Try to list streams as a health check
		_, err := repo.ListActive(ctx)
		if err != nil {
			return false, err
		}
		return true, nil
	}, interval, timeout)
}

// AddReadinessCheck creates a readiness check that verifies all dependencies
func (h *HealthChecker) AddReadinessCheck(
	redisClient *redis.Client,
	repo ports.StreamRepository,
	interval, timeout time.Duration,
) {
	h.AddCheck("readiness", func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Check Redis
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				return false, err
			}
		}

		// Check repository
		if repo != nil {
			if _, err := repo.ListActive(ctx); err != nil {
				return false, err
			}
		}

		return true, nil
	}, interval, timeout)
}

// GetReadinessStatus returns readiness status for load balancer
func (h *HealthChecker) GetReadinessStatus(ctx context.Context) HealthStatus {
	return h.CheckAll(ctx)
}

// IsReady checks if the service is ready to accept traffic
func (h *HealthChecker) IsReady(ctx context.Context) bool {
	status := h.CheckAll(ctx)
	return status.Status == "healthy"
}

