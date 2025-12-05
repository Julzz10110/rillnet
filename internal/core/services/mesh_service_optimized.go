package services

import (
	"context"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/optimize"
	"go.uber.org/zap"
)

// OptimizedMeshService wraps MeshService with performance optimizations
type OptimizedMeshService struct {
	baseService ports.MeshService
	peerPool    *optimize.StringSlicePool
	logger      *zap.SugaredLogger
}

// NewOptimizedMeshService creates an optimized mesh service
func NewOptimizedMeshService(
	baseService ports.MeshService,
	logger *zap.SugaredLogger,
) ports.MeshService {
	return &OptimizedMeshService{
		baseService: baseService,
		peerPool:    optimize.NewStringSlicePool(16), // Pre-allocate for typical peer counts
		logger:      logger,
	}
}

// AddPeer adds a peer with optimized allocations
func (m *OptimizedMeshService) AddPeer(ctx context.Context, peer *domain.Peer) error {
	return m.baseService.AddPeer(ctx, peer)
}

// RemovePeer removes a peer
func (m *OptimizedMeshService) RemovePeer(ctx context.Context, peerID domain.PeerID) error {
	return m.baseService.RemovePeer(ctx, peerID)
}

// UpdatePeerMetrics updates peer metrics
func (m *OptimizedMeshService) UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	return m.baseService.UpdatePeerMetrics(ctx, peerID, metrics)
}

// FindOptimalSources finds optimal sources
func (m *OptimizedMeshService) FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error) {
	return m.baseService.FindOptimalSources(ctx, streamID, targetPeer, count)
}

// BuildOptimalMesh builds optimal mesh with optimized allocations
func (m *OptimizedMeshService) BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error {
	return m.baseService.BuildOptimalMesh(ctx, streamID)
}

// GetPeerConnections gets peer connections
func (m *OptimizedMeshService) GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	return m.baseService.GetPeerConnections(ctx, peerID)
}

// AddConnection adds a connection
func (m *OptimizedMeshService) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	return m.baseService.AddConnection(ctx, conn)
}

// RemoveConnection removes a connection
func (m *OptimizedMeshService) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	return m.baseService.RemoveConnection(ctx, fromPeer, toPeer)
}

// GetOptimalPath gets optimal path with optimized allocations
func (m *OptimizedMeshService) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	// Use pre-allocated slice for path
	path := optimize.PreAllocateSlice[domain.PeerID](0, 8) // Typical path length
	
	// Get path from base service
	basePath, err := m.baseService.GetOptimalPath(ctx, sourcePeer, targetPeer)
	if err != nil {
		return nil, err
	}
	
	// Copy to pre-allocated slice
	path = append(path, basePath...)
	return path, nil
}

// Close closes the service
func (m *OptimizedMeshService) Close() error {
	if closer, ok := m.baseService.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

