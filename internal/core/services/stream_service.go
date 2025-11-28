package services

import (
	"context"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
)

type streamService struct {
	streamRepo     ports.StreamRepository
	peerRepo       ports.PeerRepository
	meshRepo       ports.MeshRepository
	meshService    ports.MeshService
	metricsService *MetricsService
}

func NewStreamService(
	streamRepo ports.StreamRepository,
	peerRepo ports.PeerRepository,
	meshRepo ports.MeshRepository,
	meshService ports.MeshService,
	metricsService *MetricsService,
) ports.StreamService {
	return &streamService{
		streamRepo:     streamRepo,
		peerRepo:       peerRepo,
		meshRepo:       meshRepo,
		meshService:    meshService,
		metricsService: metricsService,
	}
}

func (s *streamService) CreateStream(ctx context.Context, name string, owner domain.PeerID, maxPeers int) (*domain.Stream, error) {
	// Get user ID from context if available
	var ownerUserID domain.UserID
	if userIDVal := ctx.Value("user_id"); userIDVal != nil {
		if userID, ok := userIDVal.(domain.UserID); ok {
			ownerUserID = userID
		}
	}

	stream := &domain.Stream{
		ID:          domain.StreamID(generateStreamID()),
		Name:        name,
		Owner:       owner,
		OwnerUserID: ownerUserID,
		Active:      true,
		CreatedAt:   time.Now(),
		MaxPeers:    maxPeers,
		Permissions: []domain.StreamPermission{}, // Initialize empty permissions
		QualityLevels: []domain.StreamQuality{
			{Quality: "high", Bitrate: 2500, Width: 1280, Height: 720, Codec: "VP8"},
			{Quality: "medium", Bitrate: 1000, Width: 854, Height: 480, Codec: "VP8"},
			{Quality: "low", Bitrate: 500, Width: 640, Height: 360, Codec: "VP8"},
		},
	}

	if err := s.streamRepo.Create(ctx, stream); err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return stream, nil
}

func (s *streamService) GetStream(ctx context.Context, streamID domain.StreamID) (*domain.Stream, error) {
	return s.streamRepo.GetByID(ctx, streamID)
}

func (s *streamService) ListStreams(ctx context.Context) ([]*domain.Stream, error) {
	return s.streamRepo.ListActive(ctx)
}

func (s *streamService) JoinStream(ctx context.Context, streamID domain.StreamID, peer *domain.Peer) error {
	// Check if stream exists
	stream, err := s.streamRepo.GetByID(ctx, streamID)
	if err != nil {
		return err
	}

	if !stream.Active {
		return domain.ErrStreamNotFound
	}

	// Check maximum peer count
	currentPeers, err := s.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return err
	}

	if len(currentPeers) >= stream.MaxPeers {
		return fmt.Errorf("stream is full: %d/%d peers", len(currentPeers), stream.MaxPeers)
	}

	// Add peer to repository
	if err := s.peerRepo.Add(ctx, peer); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	// Add peer to mesh service
	if err := s.meshService.AddPeer(ctx, peer); err != nil {
		return fmt.Errorf("failed to add peer to mesh: %w", err)
	}

	// Update metrics
	if peer.Capabilities.IsPublisher {
		s.metricsService.IncrementPublisherCount(streamID)
	} else {
		s.metricsService.IncrementSubscriberCount(streamID)
	}
	s.metricsService.RecordConnection(streamID)

	// Build mesh network
	if err := s.meshRepo.BuildMesh(ctx, streamID, 4); err != nil {
		return fmt.Errorf("failed to build mesh: %w", err)
	}

	return nil
}

func (s *streamService) LeaveStream(ctx context.Context, streamID domain.StreamID, peerID domain.PeerID) error {
	// Remove peer from mesh
	if err := s.meshService.RemovePeer(ctx, peerID); err != nil {
		return fmt.Errorf("failed to remove peer from mesh: %w", err)
	}

	// Remove peer from repository
	if err := s.peerRepo.Remove(ctx, peerID); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	// Rebuild mesh network
	if err := s.meshRepo.BuildMesh(ctx, streamID, 4); err != nil {
		return fmt.Errorf("failed to rebuild mesh: %w", err)
	}

	return nil
}

func (s *streamService) GetStreamStats(ctx context.Context, streamID domain.StreamID) (*domain.StreamMetrics, error) {
	peers, err := s.peerRepo.FindByStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	var (
		totalBitrate    int
		totalLatency    time.Duration
		publisherCount  int
		subscriberCount int
	)

	for _, peer := range peers {
		if peer.Capabilities.IsPublisher {
			publisherCount++
			totalBitrate += peer.Metrics.Bandwidth
		} else {
			subscriberCount++
		}
		totalLatency += peer.Metrics.Latency
	}

	avgLatency := time.Duration(0)
	if len(peers) > 0 {
		avgLatency = totalLatency / time.Duration(len(peers))
	}

	healthScore := s.calculateHealthScore(publisherCount, subscriberCount, totalBitrate, avgLatency)

	return &domain.StreamMetrics{
		StreamID:          streamID,
		ActivePublishers:  publisherCount,
		ActiveSubscribers: subscriberCount,
		TotalBitrate:      totalBitrate,
		AverageLatency:    avgLatency,
		HealthScore:       healthScore,
	}, nil
}

func (s *streamService) calculateHealthScore(publishers, subscribers, bitrate int, latency time.Duration) float64 {
	// Simplified health score calculation
	publisherScore := float64(publishers) * 20.0
	subscriberScore := float64(subscribers) * 2.0
	bitrateScore := float64(bitrate) / 100.0

	latencyScore := 0.0
	if latency < 100*time.Millisecond {
		latencyScore = 30.0
	} else if latency < 300*time.Millisecond {
		latencyScore = 20.0
	} else if latency < 500*time.Millisecond {
		latencyScore = 10.0
	}

	totalScore := publisherScore + subscriberScore + bitrateScore + latencyScore
	if totalScore > 100.0 {
		return 100.0
	}
	return totalScore
}

func generateStreamID() string {
	return fmt.Sprintf("stream_%d", time.Now().UnixNano())
}
