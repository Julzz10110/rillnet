package memory

import (
	"context"
	"fmt"

	// "sort"
	"sync"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
)

type MemoryMeshRepository struct {
	connections map[string]*domain.PeerConnection // key: "fromPeer-toPeer"
	mu          sync.RWMutex
}

func NewMemoryMeshRepository() ports.MeshRepository {
	return &MemoryMeshRepository{
		connections: make(map[string]*domain.PeerConnection),
	}
}

func (r *MemoryMeshRepository) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.connectionKey(conn.FromPeer, conn.ToPeer)
	if _, exists := r.connections[key]; exists {
		return fmt.Errorf("connection already exists: %s->%s", conn.FromPeer, conn.ToPeer)
	}

	r.connections[key] = conn
	return nil
}

func (r *MemoryMeshRepository) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.connectionKey(fromPeer, toPeer)
	if _, exists := r.connections[key]; !exists {
		return fmt.Errorf("connection not found: %s->%s", fromPeer, toPeer)
	}

	delete(r.connections, key)
	return nil
}

func (r *MemoryMeshRepository) GetConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.PeerConnection
	for _, conn := range r.connections {
		if conn.FromPeer == peerID || conn.ToPeer == peerID {
			result = append(result, conn)
		}
	}

	return result, nil
}

func (r *MemoryMeshRepository) BuildMesh(ctx context.Context, streamID domain.StreamID, maxConnections int) error {
	// In real implementation, complex mesh network building logic would be here
	// For simplicity, just return success
	return nil
}

func (r *MemoryMeshRepository) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	// Simplified implementation - return direct path
	return []domain.PeerID{sourcePeer, targetPeer}, nil
}

func (r *MemoryMeshRepository) connectionKey(fromPeer, toPeer domain.PeerID) string {
	return string(fromPeer) + "-" + string(toPeer)
}
