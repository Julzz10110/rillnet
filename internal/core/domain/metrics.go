package domain

import "time"

type NetworkMetrics struct {
	Timestamp        time.Time
	BandwidthDown    int // kbps
	BandwidthUp      int // kbps
	PacketLoss       float64
	Latency          time.Duration
	Jitter           time.Duration
	AvailableBitrate int // kbps
}

type StreamMetrics struct {
	StreamID          StreamID
	ActivePublishers  int
	ActiveSubscribers int
	TotalBitrate      int
	AverageLatency    time.Duration
	HealthScore       float64 // 0-100
	Timestamp         time.Time
}
