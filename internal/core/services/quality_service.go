package services

import (
	"time"

	"rillnet/internal/core/domain"
)

type QualityService struct {
	thresholds map[string]domain.NetworkMetrics
}

// GetThresholds returns the quality thresholds (for use by adaptive bitrate service)
func (qs *QualityService) GetThresholds() map[string]domain.NetworkMetrics {
	return qs.thresholds
}

func NewQualityService() *QualityService {
	return &QualityService{
		thresholds: map[string]domain.NetworkMetrics{
			"high": {
				BandwidthDown:    2500,
				BandwidthUp:      1000,
				PacketLoss:       0.01,
				Latency:          100 * time.Millisecond,
				Jitter:           30 * time.Millisecond,
				AvailableBitrate: 2000,
			},
			"medium": {
				BandwidthDown:    1000,
				BandwidthUp:      500,
				PacketLoss:       0.05,
				Latency:          200 * time.Millisecond,
				Jitter:           50 * time.Millisecond,
				AvailableBitrate: 800,
			},
			"low": {
				BandwidthDown:    500,
				BandwidthUp:      256,
				PacketLoss:       0.1,
				Latency:          300 * time.Millisecond,
				Jitter:           100 * time.Millisecond,
				AvailableBitrate: 400,
			},
		},
	}
}

func (qs *QualityService) DetermineOptimalQuality(metrics domain.NetworkMetrics) string {
	if qs.meetsQualityRequirements(metrics, qs.thresholds["high"]) {
		return "high"
	} else if qs.meetsQualityRequirements(metrics, qs.thresholds["medium"]) {
		return "medium"
	} else {
		return "low"
	}
}

func (qs *QualityService) meetsQualityRequirements(metrics, threshold domain.NetworkMetrics) bool {
	return metrics.BandwidthDown >= threshold.BandwidthDown &&
		metrics.BandwidthUp >= threshold.BandwidthUp &&
		metrics.PacketLoss <= threshold.PacketLoss &&
		metrics.Latency <= threshold.Latency &&
		metrics.Jitter <= threshold.Jitter
}

func (qs *QualityService) ShouldDowngrade(currentQuality string, metrics domain.NetworkMetrics) bool {
	threshold := qs.thresholds[currentQuality]
	return float64(metrics.BandwidthDown) < float64(threshold.BandwidthDown)*0.8 ||
		metrics.PacketLoss > threshold.PacketLoss*2 ||
		float64(metrics.Latency) > float64(threshold.Latency)*1.5
}

func (qs *QualityService) ShouldUpgrade(currentQuality string, metrics domain.NetworkMetrics) bool {
	if currentQuality == "high" {
		return false
	}

	nextQuality := "medium"
	if currentQuality == "medium" {
		nextQuality = "high"
	}

	threshold := qs.thresholds[nextQuality]
	return float64(metrics.BandwidthDown) >= float64(threshold.BandwidthDown)*1.2 &&
		metrics.PacketLoss <= threshold.PacketLoss*0.8 &&
		float64(metrics.Latency) <= float64(threshold.Latency)*0.8
}
