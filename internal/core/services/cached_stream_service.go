package services

import (
	"context"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/cache"
)

// CachedStreamService wraps StreamService with caching
type CachedStreamService struct {
	baseService ports.StreamService
	cache       *cache.CacheWithFallback
	streamTTL   time.Duration
	peerTTL     time.Duration
}

// NewCachedStreamService creates a new cached stream service
func NewCachedStreamService(
	baseService ports.StreamService,
	streamTTL time.Duration,
	peerTTL time.Duration,
) ports.StreamService {
	return &CachedStreamService{
		baseService: baseService,
		cache:       cache.NewCacheWithFallback(streamTTL),
		streamTTL:   streamTTL,
		peerTTL:     peerTTL,
	}
}

// CreateStream creates a stream and invalidates cache
func (s *CachedStreamService) CreateStream(ctx context.Context, name string, owner domain.PeerID, maxPeers int) (*domain.Stream, error) {
	stream, err := s.baseService.CreateStream(ctx, name, owner, maxPeers)
	if err != nil {
		return nil, err
	}

	// Invalidate streams list cache
	s.cache.Invalidate("streams:list:")

	return stream, nil
}

// GetStream gets a stream with caching
func (s *CachedStreamService) GetStream(ctx context.Context, streamID domain.StreamID) (*domain.Stream, error) {
	cacheKey := fmt.Sprintf("stream:%s", streamID)

	value, err := s.cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return s.baseService.GetStream(ctx, streamID)
	}, s.streamTTL)

	if err != nil {
		return nil, err
	}

	return value.(*domain.Stream), nil
}

// ListStreams lists streams with caching
func (s *CachedStreamService) ListStreams(ctx context.Context) ([]*domain.Stream, error) {
	cacheKey := "streams:list:active"

	value, err := s.cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return s.baseService.ListStreams(ctx)
	}, s.streamTTL)

	if err != nil {
		return nil, err
	}

	return value.([]*domain.Stream), nil
}

// JoinStream joins a stream and invalidates relevant caches
func (s *CachedStreamService) JoinStream(ctx context.Context, streamID domain.StreamID, peer *domain.Peer) error {
	err := s.baseService.JoinStream(ctx, streamID, peer)
	if err != nil {
		return err
	}

	// Invalidate stream cache and peer list cache
	s.cache.Invalidate(fmt.Sprintf("stream:%s", streamID))
	s.cache.Invalidate(fmt.Sprintf("stream:%s:peers", streamID))
	s.cache.Invalidate("streams:list:")

	return nil
}

// LeaveStream leaves a stream and invalidates relevant caches
func (s *CachedStreamService) LeaveStream(ctx context.Context, streamID domain.StreamID, peerID domain.PeerID) error {
	err := s.baseService.LeaveStream(ctx, streamID, peerID)
	if err != nil {
		return err
	}

	// Invalidate stream cache and peer list cache
	s.cache.Invalidate(fmt.Sprintf("stream:%s", streamID))
	s.cache.Invalidate(fmt.Sprintf("stream:%s:peers", streamID))
	s.cache.Invalidate("streams:list:")

	return nil
}

// GetStreamStats gets stream stats with caching (shorter TTL)
func (s *CachedStreamService) GetStreamStats(ctx context.Context, streamID domain.StreamID) (*domain.StreamMetrics, error) {
	cacheKey := fmt.Sprintf("stream:%s:stats", streamID)
	statsTTL := s.streamTTL / 4 // Stats change more frequently, use shorter TTL

	value, err := s.cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (interface{}, error) {
		return s.baseService.GetStreamStats(ctx, streamID)
	}, statsTTL)

	if err != nil {
		return nil, err
	}

	return value.(*domain.StreamMetrics), nil
}

// Stop stops the cache cleanup
func (s *CachedStreamService) Stop() {
	s.cache.Stop()
}

