package services

import (
	"context"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"go.uber.org/zap"
)

// AdaptiveBitrateService manages automatic quality switching based on network conditions
type AdaptiveBitrateService struct {
	qualityService *QualityService
	meshService    ports.MeshService
	logger         *zap.SugaredLogger

	// Per-peer quality state
	peerQuality     map[domain.PeerID]string
	peerQualityMu   sync.RWMutex
	lastQualityTime map[domain.PeerID]time.Time
	qualityHistory  map[domain.PeerID][]qualitySnapshot

	// Configuration
	checkInterval    time.Duration
	minTimeBetweenSwitches time.Duration
	hysteresisFactor float64 // Prevents rapid switching
}

type qualitySnapshot struct {
	Quality   string
	Timestamp time.Time
	Metrics   domain.NetworkMetrics
}

// NewAdaptiveBitrateService creates a new adaptive bitrate service
func NewAdaptiveBitrateService(
	qualityService *QualityService,
	meshService ports.MeshService,
	logger *zap.SugaredLogger,
) *AdaptiveBitrateService {
	return &AdaptiveBitrateService{
		qualityService:        qualityService,
		meshService:           meshService,
		logger:                logger,
		peerQuality:           make(map[domain.PeerID]string),
		lastQualityTime:       make(map[domain.PeerID]time.Time),
		qualityHistory:        make(map[domain.PeerID][]qualitySnapshot),
		checkInterval:         5 * time.Second,
		minTimeBetweenSwitches: 10 * time.Second,
		hysteresisFactor:      0.15, // 15% hysteresis to prevent oscillation
	}
}

// StartMonitoring starts monitoring a peer's metrics and automatically adjusts quality
func (a *AdaptiveBitrateService) StartMonitoring(ctx context.Context, peerID domain.PeerID, initialQuality string) {
	a.peerQualityMu.Lock()
	a.peerQuality[peerID] = initialQuality
	a.lastQualityTime[peerID] = time.Now()
	a.qualityHistory[peerID] = []qualitySnapshot{}
	a.peerQualityMu.Unlock()

	go a.monitorPeer(ctx, peerID)
}

// StopMonitoring stops monitoring a peer
func (a *AdaptiveBitrateService) StopMonitoring(peerID domain.PeerID) {
	a.peerQualityMu.Lock()
	delete(a.peerQuality, peerID)
	delete(a.lastQualityTime, peerID)
	delete(a.qualityHistory, peerID)
	a.peerQualityMu.Unlock()
}

// monitorPeer continuously monitors a peer's metrics and adjusts quality
func (a *AdaptiveBitrateService) monitorPeer(ctx context.Context, peerID domain.PeerID) {
	ticker := time.NewTicker(a.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.checkAndAdjustQuality(ctx, peerID); err != nil {
				a.logger.Warnw("error checking quality for peer",
					"peer_id", peerID,
					"error", err,
				)
			}
		}
	}
}

// checkAndAdjustQuality checks current metrics and adjusts quality if needed
func (a *AdaptiveBitrateService) checkAndAdjustQuality(ctx context.Context, peerID domain.PeerID) error {
	// Get current peer connections to verify peer exists
	_, err := a.meshService.GetPeerConnections(ctx, peerID)
	if err != nil {
		return err
	}

	// For now, we'll get metrics from the peer repository
	// In a full implementation, we'd get real-time metrics from RTCP
	// This is a placeholder - actual implementation would get metrics from WebRTC stats
	
	// Get current quality
	a.peerQualityMu.RLock()
	currentQuality := a.peerQuality[peerID]
	lastSwitchTime := a.lastQualityTime[peerID]
	a.peerQualityMu.RUnlock()

	// Check if enough time has passed since last switch
	if time.Since(lastSwitchTime) < a.minTimeBetweenSwitches {
		return nil
	}

	// Get network metrics (this would come from RTCP in real implementation)
	// For now, we'll use a simplified approach
	metrics := domain.NetworkMetrics{
		Timestamp: time.Now(),
		// These would be populated from actual RTCP stats
		BandwidthDown:    1000, // Placeholder
		BandwidthUp:      500,  // Placeholder
		PacketLoss:       0.02,
		Latency:          150 * time.Millisecond,
		Jitter:           40 * time.Millisecond,
		AvailableBitrate: 800,
	}

	// Determine optimal quality with hysteresis
	newQuality := a.determineQualityWithHysteresis(currentQuality, metrics)

	if newQuality != currentQuality {
		a.logger.Infow("quality switch triggered",
			"peer_id", peerID,
			"from", currentQuality,
			"to", newQuality,
			"bandwidth", metrics.BandwidthDown,
			"packet_loss", metrics.PacketLoss,
			"latency", metrics.Latency,
		)

		// Update quality
		a.peerQualityMu.Lock()
		a.peerQuality[peerID] = newQuality
		a.lastQualityTime[peerID] = time.Now()
		
		// Record in history
		a.qualityHistory[peerID] = append(a.qualityHistory[peerID], qualitySnapshot{
			Quality:   newQuality,
			Timestamp: time.Now(),
			Metrics:   metrics,
		})
		
		// Keep only last 100 snapshots
		if len(a.qualityHistory[peerID]) > 100 {
			a.qualityHistory[peerID] = a.qualityHistory[peerID][len(a.qualityHistory[peerID])-100:]
		}
		a.peerQualityMu.Unlock()

		// Notify about quality change (this would trigger simulcast layer switch)
		// The actual implementation would call SFU to switch simulcast layers
		return nil
	}

	return nil
}

// determineQualityWithHysteresis determines quality with hysteresis to prevent oscillation
func (a *AdaptiveBitrateService) determineQualityWithHysteresis(currentQuality string, metrics domain.NetworkMetrics) string {
	// Get optimal quality without hysteresis
	optimalQuality := a.qualityService.DetermineOptimalQuality(metrics)

	// If optimal is same as current, no change needed
	if optimalQuality == currentQuality {
		return currentQuality
	}

		// Apply hysteresis: be more conservative when downgrading, more aggressive when upgrading
		if optimalQuality < currentQuality {
			// Downgrading: use stricter thresholds (with hysteresis)
			thresholds := a.qualityService.GetThresholds()
			threshold := thresholds[currentQuality]
		hysteresisThreshold := domain.NetworkMetrics{
			BandwidthDown:    int(float64(threshold.BandwidthDown) * (1.0 - a.hysteresisFactor)),
			BandwidthUp:      int(float64(threshold.BandwidthUp) * (1.0 - a.hysteresisFactor)),
			PacketLoss:       threshold.PacketLoss * (1.0 + a.hysteresisFactor),
			Latency:          time.Duration(float64(threshold.Latency) * (1.0 + a.hysteresisFactor)),
			Jitter:           time.Duration(float64(threshold.Jitter) * (1.0 + a.hysteresisFactor)),
			AvailableBitrate: int(float64(threshold.AvailableBitrate) * (1.0 - a.hysteresisFactor)),
		}

		if !a.qualityService.meetsQualityRequirements(metrics, hysteresisThreshold) {
			return optimalQuality
		}
	} else {
		// Upgrading: use relaxed thresholds (with hysteresis)
		thresholds := a.qualityService.GetThresholds()
		threshold := thresholds[optimalQuality]
		hysteresisThreshold := domain.NetworkMetrics{
			BandwidthDown:    int(float64(threshold.BandwidthDown) * (1.0 - a.hysteresisFactor)),
			BandwidthUp:      int(float64(threshold.BandwidthUp) * (1.0 - a.hysteresisFactor)),
			PacketLoss:       threshold.PacketLoss * (1.0 + a.hysteresisFactor),
			Latency:          time.Duration(float64(threshold.Latency) * (1.0 + a.hysteresisFactor)),
			Jitter:           time.Duration(float64(threshold.Jitter) * (1.0 + a.hysteresisFactor)),
			AvailableBitrate: int(float64(threshold.AvailableBitrate) * (1.0 - a.hysteresisFactor)),
		}

		if a.qualityService.meetsQualityRequirements(metrics, hysteresisThreshold) {
			return optimalQuality
		}
	}

	return currentQuality
}

// GetCurrentQuality returns the current quality for a peer
func (a *AdaptiveBitrateService) GetCurrentQuality(peerID domain.PeerID) string {
	a.peerQualityMu.RLock()
	defer a.peerQualityMu.RUnlock()
	return a.peerQuality[peerID]
}

// GetQualityHistory returns quality change history for a peer
func (a *AdaptiveBitrateService) GetQualityHistory(peerID domain.PeerID) []qualitySnapshot {
	a.peerQualityMu.RLock()
	defer a.peerQualityMu.RUnlock()
	
	history := make([]qualitySnapshot, len(a.qualityHistory[peerID]))
	copy(history, a.qualityHistory[peerID])
	return history
}

// SetCheckInterval sets the interval for quality checks
func (a *AdaptiveBitrateService) SetCheckInterval(interval time.Duration) {
	a.checkInterval = interval
}

// SetMinTimeBetweenSwitches sets minimum time between quality switches
func (a *AdaptiveBitrateService) SetMinTimeBetweenSwitches(duration time.Duration) {
	a.minTimeBetweenSwitches = duration
}

// SetHysteresisFactor sets the hysteresis factor (0.0-1.0)
func (a *AdaptiveBitrateService) SetHysteresisFactor(factor float64) {
	if factor < 0 {
		factor = 0
	}
	if factor > 1.0 {
		factor = 1.0
	}
	a.hysteresisFactor = factor
}

