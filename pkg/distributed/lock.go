package distributed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLock provides distributed locking using Redis
type DistributedLock struct {
	client    *redis.Client
	key       string
	value     string // Unique identifier for this lock holder
	ttl       time.Duration
	renewalCh chan struct{}
	stopRenew chan struct{}
}

// NewDistributedLock creates a new distributed lock
func NewDistributedLock(client *redis.Client, key string, ttl time.Duration) *DistributedLock {
	// Generate unique lock value
	value := generateLockValue()

	return &DistributedLock{
		client:    client,
		key:       key,
		value:     value,
		ttl:       ttl,
		renewalCh: make(chan struct{}),
		stopRenew: make(chan struct{}),
	}
}

// generateLockValue generates a unique value for the lock
func generateLockValue() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Lock acquires the lock, blocking until it's available
func (l *DistributedLock) Lock(ctx context.Context) error {
	return l.LockWithTimeout(ctx, 0)
}

// LockWithTimeout acquires the lock with a timeout
func (l *DistributedLock) LockWithTimeout(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	if timeout == 0 {
		deadline = time.Now().Add(30 * time.Second) // Default timeout
	}

	for {
		// Try to acquire lock using SET NX EX
		acquired, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}

		if acquired {
			// Start renewal goroutine
			go l.renewLock(ctx)
			return nil
		}

		// Check if we've exceeded timeout
		if time.Now().After(deadline) {
			return fmt.Errorf("lock acquisition timeout")
		}

		// Wait a bit before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry
		}
	}
}

// TryLock attempts to acquire the lock without blocking
func (l *DistributedLock) TryLock(ctx context.Context) (bool, error) {
	acquired, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to try lock: %w", err)
	}

	if acquired {
		// Start renewal goroutine
		go l.renewLock(ctx)
		return true, nil
	}

	return false, nil
}

// Unlock releases the lock
func (l *DistributedLock) Unlock(ctx context.Context) error {
	// Stop renewal
	close(l.stopRenew)

	// Use Lua script to ensure we only delete our own lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("failed to unlock: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock was not held by this instance")
	}

	return nil
}

// renewLock periodically renews the lock to prevent expiration
func (l *DistributedLock) renewLock(ctx context.Context) {
	ticker := time.NewTicker(l.ttl / 2) // Renew at half TTL
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if we still hold the lock and renew it
			currentValue, err := l.client.Get(ctx, l.key).Result()
			if err == redis.Nil {
				// Lock was released
				return
			}
			if err != nil {
				// Error getting lock, stop renewal
				return
			}

			// Only renew if we still hold the lock
			if currentValue == l.value {
				l.client.Expire(ctx, l.key, l.ttl)
			} else {
				// Someone else has the lock
				return
			}

		case <-l.stopRenew:
			return
		case <-ctx.Done():
			return
		}
	}
}

// IsLocked checks if the lock is currently held
func (l *DistributedLock) IsLocked(ctx context.Context) (bool, error) {
	exists, err := l.client.Exists(ctx, l.key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// LockManager manages distributed locks
type LockManager struct {
	client *redis.Client
	prefix string
}

// NewLockManager creates a new lock manager
func NewLockManager(client *redis.Client, prefix string) *LockManager {
	return &LockManager{
		client: client,
		prefix: prefix,
	}
}

// AcquireLock acquires a lock with the given key
func (lm *LockManager) AcquireLock(key string, ttl time.Duration) *DistributedLock {
	fullKey := lm.prefix + key
	return NewDistributedLock(lm.client, fullKey, ttl)
}

