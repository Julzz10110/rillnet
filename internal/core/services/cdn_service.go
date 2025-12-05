package services

import (
	"context"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"go.uber.org/zap"
)

// CDNService manages CDN fallback logic
type CDNService struct {
	enabled        bool
	fallbackURL    string
	thresholds     CDNThresholds
	logger         *zap.SugaredLogger
}

// CDNThresholds defines when to use CDN fallback
type CDNThresholds struct {
	MinPeersForP2P      int           // Minimum peers needed for efficient P2P
	MaxLatency          time.Duration // Max latency before CDN fallback
	MinBandwidth        int           // Min bandwidth (kbps) for P2P
	PacketLossThreshold float64       // Max packet loss before CDN fallback
	P2PEfficiencyThreshold float64    // Min P2P efficiency before CDN fallback
}

// DefaultCDNThresholds returns default CDN thresholds
func DefaultCDNThresholds() CDNThresholds {
	return CDNThresholds{
		MinPeersForP2P:      3,
		MaxLatency:          500 * time.Millisecond,
		MinBandwidth:        1000, // 1 Mbps
		PacketLossThreshold: 0.05, // 5%
		P2PEfficiencyThreshold: 0.5, // 50%
	}
}

// NewCDNService creates a new CDN service
func NewCDNService(
	enabled bool,
	fallbackURL string,
	thresholds CDNThresholds,
	logger *zap.SugaredLogger,
) *CDNService {
	return &CDNService{
		enabled:     enabled,
		fallbackURL: fallbackURL,
		thresholds:  thresholds,
		logger:      logger,
	}
}

// ShouldUseCDN determines if CDN should be used instead of P2P
func (c *CDNService) ShouldUseCDN(ctx context.Context, streamID domain.StreamID, metrics *StreamMetrics) bool {
	if !c.enabled {
		return false
	}

	// Check if we have enough peers for efficient P2P
	if metrics.ActivePeers < c.thresholds.MinPeersForP2P {
		c.logger.Debugw("using CDN: insufficient peers",
			"stream_id", streamID,
			"peers", metrics.ActivePeers,
			"min_peers", c.thresholds.MinPeersForP2P,
		)
		return true
	}

	// Check latency
	if metrics.AverageLatency > c.thresholds.MaxLatency {
		c.logger.Debugw("using CDN: high latency",
			"stream_id", streamID,
			"latency", metrics.AverageLatency,
			"max_latency", c.thresholds.MaxLatency,
		)
		return true
	}

	// Check bandwidth
	if metrics.AvailableBandwidth < c.thresholds.MinBandwidth {
		c.logger.Debugw("using CDN: low bandwidth",
			"stream_id", streamID,
			"bandwidth", metrics.AvailableBandwidth,
			"min_bandwidth", c.thresholds.MinBandwidth,
		)
		return true
	}

	// Check packet loss
	if metrics.PacketLoss > c.thresholds.PacketLossThreshold {
		c.logger.Debugw("using CDN: high packet loss",
			"stream_id", streamID,
			"packet_loss", metrics.PacketLoss,
			"threshold", c.thresholds.PacketLossThreshold,
		)
		return true
	}

	// Check P2P efficiency
	if metrics.P2PEfficiency < c.thresholds.P2PEfficiencyThreshold {
		c.logger.Debugw("using CDN: low P2P efficiency",
			"stream_id", streamID,
			"efficiency", metrics.P2PEfficiency,
			"threshold", c.thresholds.P2PEfficiencyThreshold,
		)
		return true
	}

	return false
}

// GetCDNURL returns the CDN URL for a stream
func (c *CDNService) GetCDNURL(streamID domain.StreamID, quality string) string {
	if !c.enabled || c.fallbackURL == "" {
		return ""
	}

	// Construct CDN URL with stream ID and quality
	return fmt.Sprintf("%s/streams/%s/%s/index.m3u8", c.fallbackURL, streamID, quality)
}

// GetCDNSegmentURL returns the CDN URL for a specific segment
func (c *CDNService) GetCDNSegmentURL(streamID domain.StreamID, quality string, segmentID string) string {
	if !c.enabled || c.fallbackURL == "" {
		return ""
	}

	return fmt.Sprintf("%s/streams/%s/%s/%s.ts", c.fallbackURL, streamID, quality, segmentID)
}

// StreamMetrics contains metrics for CDN decision making
type StreamMetrics struct {
	ActivePeers        int
	AverageLatency      time.Duration
	AvailableBandwidth  int // kbps
	PacketLoss          float64
	P2PEfficiency       float64 // 0.0-1.0
}

