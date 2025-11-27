package ports

import (
	"context"

	"rillnet/internal/core/domain"
)

type StreamRepository interface {
	Create(ctx context.Context, stream *domain.Stream) error
	GetByID(ctx context.Context, id domain.StreamID) (*domain.Stream, error)
	Update(ctx context.Context, stream *domain.Stream) error
	Delete(ctx context.Context, id domain.StreamID) error
	ListActive(ctx context.Context) ([]*domain.Stream, error)
}

type PeerRepository interface {
	Add(ctx context.Context, peer *domain.Peer) error
	GetByID(ctx context.Context, id domain.PeerID) (*domain.Peer, error)
	Remove(ctx context.Context, id domain.PeerID) error
	FindByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error)
	FindOptimalSource(ctx context.Context, streamID domain.StreamID, excludePeers []domain.PeerID) (*domain.Peer, error)
	UpdateMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error
	UpdatePeerLoad(ctx context.Context, peerID domain.PeerID, load int) error
}

type MeshRepository interface {
	AddConnection(ctx context.Context, conn *domain.PeerConnection) error
	RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error
	GetConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error)
	BuildMesh(ctx context.Context, streamID domain.StreamID, maxConnections int) error
	GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error)
}
