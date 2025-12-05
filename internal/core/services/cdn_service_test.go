package services

import (
	"context"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"go.uber.org/zap/zaptest"
)

func TestCDNService_ShouldUseCDN(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	thresholds := DefaultCDNThresholds()
	
	tests := []struct {
		name    string
		enabled bool
		metrics *StreamMetrics
		want    bool
	}{
		{
			name:    "CDN disabled",
			enabled: false,
			metrics: &StreamMetrics{ActivePeers: 1},
			want:    false,
		},
		{
			name:    "insufficient peers",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers: 1,
			},
			want: true,
		},
		{
			name:    "high latency",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers:   5,
				AverageLatency: 600 * time.Millisecond,
			},
			want: true,
		},
		{
			name:    "low bandwidth",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers:       5,
				AverageLatency:    100 * time.Millisecond,
				AvailableBandwidth: 500, // kbps, below threshold
			},
			want: true,
		},
		{
			name:    "high packet loss",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers:       5,
				AverageLatency:    100 * time.Millisecond,
				AvailableBandwidth: 2000,
				PacketLoss:        0.1, // 10%, above threshold
			},
			want: true,
		},
		{
			name:    "low P2P efficiency",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers:       5,
				AverageLatency:    100 * time.Millisecond,
				AvailableBandwidth: 2000,
				PacketLoss:        0.01,
				P2PEfficiency:     0.3, // 30%, below threshold
			},
			want: true,
		},
		{
			name:    "good conditions - use P2P",
			enabled: true,
			metrics: &StreamMetrics{
				ActivePeers:       5,
				AverageLatency:    100 * time.Millisecond,
				AvailableBandwidth: 2000,
				PacketLoss:        0.01,
				P2PEfficiency:     0.8,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewCDNService(tt.enabled, "https://cdn.example.com", thresholds, logger)
			streamID := domain.StreamID("stream-123")
			
			got := service.ShouldUseCDN(context.Background(), streamID, tt.metrics)
			if got != tt.want {
				t.Errorf("ShouldUseCDN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCDNService_GetCDNURL(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	service := NewCDNService(true, "https://cdn.example.com", DefaultCDNThresholds(), logger)
	
	streamID := domain.StreamID("stream-123")
	url := service.GetCDNURL(streamID, "high")
	
	expected := "https://cdn.example.com/streams/stream-123/high/index.m3u8"
	if url != expected {
		t.Errorf("GetCDNURL() = %v, want %v", url, expected)
	}
	
	// Test disabled CDN
	serviceDisabled := NewCDNService(false, "https://cdn.example.com", DefaultCDNThresholds(), logger)
	urlDisabled := serviceDisabled.GetCDNURL(streamID, "high")
	if urlDisabled != "" {
		t.Errorf("GetCDNURL() with disabled CDN = %v, want empty string", urlDisabled)
	}
}

func TestCDNService_GetCDNSegmentURL(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	service := NewCDNService(true, "https://cdn.example.com", DefaultCDNThresholds(), logger)
	
	streamID := domain.StreamID("stream-123")
	url := service.GetCDNSegmentURL(streamID, "high", "segment-001")
	
	expected := "https://cdn.example.com/streams/stream-123/high/segment-001.ts"
	if url != expected {
		t.Errorf("GetCDNSegmentURL() = %v, want %v", url, expected)
	}
}

func TestDefaultCDNThresholds(t *testing.T) {
	thresholds := DefaultCDNThresholds()
	
	if thresholds.MinPeersForP2P != 3 {
		t.Errorf("MinPeersForP2P = %v, want 3", thresholds.MinPeersForP2P)
	}
	if thresholds.MaxLatency != 500*time.Millisecond {
		t.Errorf("MaxLatency = %v, want 500ms", thresholds.MaxLatency)
	}
	if thresholds.MinBandwidth != 1000 {
		t.Errorf("MinBandwidth = %v, want 1000", thresholds.MinBandwidth)
	}
	if thresholds.PacketLossThreshold != 0.05 {
		t.Errorf("PacketLossThreshold = %v, want 0.05", thresholds.PacketLossThreshold)
	}
	if thresholds.P2PEfficiencyThreshold != 0.5 {
		t.Errorf("P2PEfficiencyThreshold = %v, want 0.5", thresholds.P2PEfficiencyThreshold)
	}
}

