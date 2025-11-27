package domain

import "errors"

var (
	ErrStreamNotFound      = errors.New("stream not found")
	ErrPeerNotFound        = errors.New("peer not found")
	ErrTrackNotFound       = errors.New("track not found")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrInsufficientQuality = errors.New("insufficient quality")
	ErrPeerCapacityReached = errors.New("peer capacity reached")
)
