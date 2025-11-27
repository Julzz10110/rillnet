package domain

import "time"

type Peer struct {
	ID           PeerID
	SessionID    SessionID
	StreamID     StreamID
	Address      string
	Capabilities PeerCapabilities
	Connections  []PeerConnection
	Metrics      PeerMetrics
	LastSeen     time.Time
}

type PeerCapabilities struct {
	MaxBitrate      int // kbps
	SupportedCodecs []string
	IsPublisher     bool
	CanRelay        bool
}

type PeerMetrics struct {
	Bandwidth   int // kbps
	PacketLoss  float64
	Latency     time.Duration
	CPUUsage    float64
	MemoryUsage int64
}

type PeerConnection struct {
	FromPeer  PeerID
	ToPeer    PeerID
	Direction ConnectionDirection
	Quality   StreamQuality
	OpenedAt  time.Time
	Bitrate   int
}

type ConnectionDirection string

const (
	DirectionInbound  ConnectionDirection = "inbound"
	DirectionOutbound ConnectionDirection = "outbound"
)
