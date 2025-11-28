package domain

import (
	"time"
)

type StreamID string
type PeerID string
type SessionID string
type TrackID string

type Stream struct {
	ID            StreamID
	Name          string
	Owner         PeerID
	OwnerUserID   UserID // User who owns the stream
	Active        bool
	CreatedAt     time.Time
	MaxPeers      int
	QualityLevels []StreamQuality
	Permissions   []StreamPermission // User permissions for this stream
}

type StreamQuality struct {
	Quality string
	Bitrate int
	Width   int
	Height  int
	Codec   string
}
