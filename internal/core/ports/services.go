package ports

import (
	"context"

	"rillnet/internal/core/domain"

	"github.com/pion/webrtc/v3"
)

type StreamService interface {
	CreateStream(ctx context.Context, name string, owner domain.PeerID, maxPeers int) (*domain.Stream, error)
	GetStream(ctx context.Context, streamID domain.StreamID) (*domain.Stream, error)
	JoinStream(ctx context.Context, streamID domain.StreamID, peer *domain.Peer) error
	LeaveStream(ctx context.Context, streamID domain.StreamID, peerID domain.PeerID) error
	GetStreamStats(ctx context.Context, streamID domain.StreamID) (*domain.StreamMetrics, error)
	ListStreams(ctx context.Context) ([]*domain.Stream, error)
}

type MeshService interface {
	AddPeer(ctx context.Context, peer *domain.Peer) error
	RemovePeer(ctx context.Context, peerID domain.PeerID) error
	UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error
	FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error)
	BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error
	GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error)
	AddConnection(ctx context.Context, conn *domain.PeerConnection) error
	RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error
	GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error)
}

type WebRTCService interface {
	CreatePublisherOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID) (webrtc.SessionDescription, error)
	HandlePublisherAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error
	CreateSubscriberOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, sourcePeers []domain.PeerID) (webrtc.SessionDescription, error)
	HandleSubscriberAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error
}
