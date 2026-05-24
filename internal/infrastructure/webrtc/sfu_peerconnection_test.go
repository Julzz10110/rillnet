package webrtc

import (
	"context"
	"testing"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/retry"

	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/require"
)

func TestSFU_CreatePublisherOfferRegistersCodecs(t *testing.T) {
	sfu := NewSFUService(
		WebRTCConfig{
			ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
		},
		services.NewQualityService(),
		services.NewMetricsService(),
		nil,
		retry.DefaultConfig(),
		circuitbreaker.DefaultConfig(),
	).(*SFUService)

	offer, err := sfu.CreatePublisherOffer(
		context.Background(),
		domain.PeerID("test-publisher"),
		domain.StreamID("test-stream"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, offer.SDP)
	require.Contains(t, offer.SDP, "m=audio")
	require.Contains(t, offer.SDP, "m=video")
}
