package cache

import (
	"context"
	"sync"
	"time"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Value      interface{}
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// IsExpired checks if the cache item has expired
func (item *CacheItem) IsExpired() bool {
	return time.Now().After(item.ExpiresAt)
}

// Cache is a thread-safe in-memory cache with TTL support
type Cache struct {
	items      map[string]*CacheItem
	mu         sync.RWMutex
	defaultTTL time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewCache creates a new cache with default TTL
func NewCache(defaultTTL time.Duration) *Cache {
	c := &Cache{
		items:           make(map[string]*CacheItem),
		defaultTTL:      defaultTTL,
		cleanupInterval: defaultTTL / 2,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a value from cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if item.IsExpired() {
		// Item expired, but we'll let cleanup remove it
		return nil, false
	}

	return item.Value, true
}

// Set stores a value in cache with default TTL
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value in cache with custom TTL
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

// Delete removes a key from cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all items from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*CacheItem)
}

// Invalidate removes expired items (can be called manually)
func (c *Cache) Invalidate(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pattern == "" {
		// Remove all expired items
		for key, item := range c.items {
			if item.IsExpired() {
				delete(c.items, key)
			}
		}
		return
	}

	// Simple prefix matching for pattern invalidation
	for key := range c.items {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(c.items, key)
		}
	}
}

// cleanup periodically removes expired items
func (c *Cache) cleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Invalidate("")
		case <-c.stopCleanup:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (c *Cache) Stop() {
	close(c.stopCleanup)
}

// Size returns the number of items in cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics
type Stats struct {
	Size      int
	Expired   int
	TotalKeys int
}

// GetStats returns cache statistics
func (c *Cache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := Stats{
		TotalKeys: len(c.items),
	}

	for _, item := range c.items {
		if item.IsExpired() {
			stats.Expired++
		}
	}

	stats.Size = stats.TotalKeys - stats.Expired
	return stats
}

// CacheWithFallback is a cache wrapper that falls back to a function if cache miss
type CacheWithFallback struct {
	cache *Cache
}

// NewCacheWithFallback creates a cache with fallback function support
func NewCacheWithFallback(defaultTTL time.Duration) *CacheWithFallback {
	return &CacheWithFallback{
		cache: NewCache(defaultTTL),
	}
}

// GetOrSet retrieves from cache or calls fallback function and caches result
func (c *CacheWithFallback) GetOrSet(ctx context.Context, key string, fallback func(context.Context) (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get from cache
	if value, found := c.cache.Get(key); found {
		return value, nil
	}

	// Cache miss, call fallback
	value, err := fallback(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if ttl > 0 {
		c.cache.SetWithTTL(key, value, ttl)
	} else {
		c.cache.Set(key, value)
	}

	return value, nil
}

// Invalidate invalidates cache entries matching pattern
func (c *CacheWithFallback) Invalidate(pattern string) {
	c.cache.Invalidate(pattern)
}

// Stop stops the cache cleanup
func (c *CacheWithFallback) Stop() {
	c.cache.Stop()
}

