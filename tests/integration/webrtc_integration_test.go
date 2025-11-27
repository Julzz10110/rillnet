package integration

import (
	"context"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"
	"rillnet/internal/infrastructure/repositories/memory"

	"github.com/stretchr/testify/assert"
)

func TestStreamLifecycleIntegration(t *testing.T) {
	// Setup real repositories and services
	streamRepo := memory.NewMemoryStreamRepository()
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()
	meshService := services.NewMeshService(peerRepo, meshRepo)
	metricsService := services.NewMetricsService()
	streamService := services.NewStreamService(streamRepo, peerRepo, meshRepo, meshService, metricsService)

	ctx := context.Background()

	t.Run("complete stream lifecycle", func(t *testing.T) {
		// Create stream
		stream, err := streamService.CreateStream(ctx, "integration-test", "owner-123", 50)
		assert.NoError(t, err)
		assert.NotNil(t, stream)

		// Join multiple peers with unique IDs
		peers := []*domain.Peer{
			{
				ID:       domain.PeerID("publisher-1-" + time.Now().Format("150405")), // Unique ID
				StreamID: stream.ID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: true,
					MaxBitrate:  2500,
				},
			},
			{
				ID:       domain.PeerID("subscriber-1-" + time.Now().Format("150405")), // Unique ID
				StreamID: stream.ID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: false,
					MaxBitrate:  1000,
				},
			},
			{
				ID:       domain.PeerID("subscriber-2-" + time.Now().Format("150405")), // Unique ID
				StreamID: stream.ID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: false,
					MaxBitrate:  1500,
				},
			},
		}

		// Store peer IDs for cleanup
		peerIDs := make([]domain.PeerID, len(peers))
		for i, peer := range peers {
			peerIDs[i] = peer.ID
			err := streamService.JoinStream(ctx, stream.ID, peer)
			assert.NoError(t, err, "Failed to join stream for peer %s", peer.ID)
		}

		// Verify stream stats
		stats, err := streamService.GetStreamStats(ctx, stream.ID)
		assert.NoError(t, err)
		assert.Equal(t, 1, stats.ActivePublishers)
		assert.Equal(t, 2, stats.ActiveSubscribers)
		assert.True(t, stats.HealthScore > 0)

		// Verify peers are in repository
		streamPeers, err := peerRepo.FindByStream(ctx, stream.ID)
		assert.NoError(t, err)
		assert.Len(t, streamPeers, 3)

		// Leave stream
		for _, peerID := range peerIDs {
			err := streamService.LeaveStream(ctx, stream.ID, peerID)
			assert.NoError(t, err, "Failed to leave stream for peer %s", peerID)
		}

		// Verify no peers left
		streamPeers, err = peerRepo.FindByStream(ctx, stream.ID)
		assert.NoError(t, err)
		assert.Len(t, streamPeers, 0)
	})
}

func TestMeshServiceIntegration(t *testing.T) {
	peerRepo := memory.NewMemoryPeerRepository()
	meshRepo := memory.NewMemoryMeshRepository()
	meshService := services.NewMeshService(peerRepo, meshRepo)

	ctx := context.Background()
	streamID := domain.StreamID("mesh-test-stream")

	t.Run("mesh building with multiple peers", func(t *testing.T) {
		// Add multiple peers with unique IDs
		timestamp := time.Now().Format("150405")
		peers := []*domain.Peer{
			{
				ID:       domain.PeerID("publisher-1-" + timestamp),
				StreamID: streamID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: true,
					MaxBitrate:  3000,
				},
				Metrics: domain.PeerMetrics{
					Bandwidth:  3000,
					Latency:    50 * time.Millisecond,
					PacketLoss: 0.01,
				},
			},
			{
				ID:       domain.PeerID("subscriber-1-" + timestamp),
				StreamID: streamID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: false,
					MaxBitrate:  2000,
				},
				Metrics: domain.PeerMetrics{
					Bandwidth:  2000,
					Latency:    100 * time.Millisecond,
					PacketLoss: 0.02,
				},
			},
			{
				ID:       domain.PeerID("subscriber-2-" + timestamp),
				StreamID: streamID,
				Capabilities: domain.PeerCapabilities{
					IsPublisher: false,
					MaxBitrate:  1500,
				},
				Metrics: domain.PeerMetrics{
					Bandwidth:  1500,
					Latency:    150 * time.Millisecond,
					PacketLoss: 0.05,
				},
			},
		}

		for _, peer := range peers {
			err := meshService.AddPeer(ctx, peer)
			assert.NoError(t, err)
		}

		// Find optimal sources for subscriber
		targetPeer := domain.PeerID("subscriber-2-" + timestamp)
		sources, err := meshService.FindOptimalSources(ctx, streamID, targetPeer, 2)
		assert.NoError(t, err)
		assert.NotEmpty(t, sources)

		// Should prefer publisher as source
		if len(sources) > 0 {
			assert.Equal(t, "publisher-1-"+timestamp, string(sources[0].ID))
		}

		// Build mesh
		err = meshService.BuildOptimalMesh(ctx, streamID)
		assert.NoError(t, err)
	})
}
