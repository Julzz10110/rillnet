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
	Active        bool
	CreatedAt     time.Time
	MaxPeers      int
	QualityLevels []StreamQuality
}

type StreamQuality struct {
	Quality string
	Bitrate int
	Width   int
	Height  int
	Codec   string
}
