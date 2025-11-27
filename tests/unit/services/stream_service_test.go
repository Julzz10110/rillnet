package services

import (
	"context"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repositories
type MockStreamRepository struct {
	mock.Mock
}

// MockMeshService with full interface implementation
type MockMeshService struct {
	mock.Mock
}

func (m *MockStreamRepository) Create(ctx context.Context, stream *domain.Stream) error {
	args := m.Called(ctx, stream)
	return args.Error(0)
}

func (m *MockStreamRepository) GetByID(ctx context.Context, id domain.StreamID) (*domain.Stream, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Stream), args.Error(1)
}

func (m *MockStreamRepository) Update(ctx context.Context, stream *domain.Stream) error {
	args := m.Called(ctx, stream)
	return args.Error(0)
}

func (m *MockStreamRepository) Delete(ctx context.Context, id domain.StreamID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStreamRepository) ListActive(ctx context.Context) ([]*domain.Stream, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Stream), args.Error(1)
}

type MockPeerRepository struct {
	mock.Mock
}

func (m *MockPeerRepository) Add(ctx context.Context, peer *domain.Peer) error {
	args := m.Called(ctx, peer)
	return args.Error(0)
}

func (m *MockPeerRepository) GetByID(ctx context.Context, id domain.PeerID) (*domain.Peer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Peer), args.Error(1)
}

func (m *MockPeerRepository) Remove(ctx context.Context, id domain.PeerID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPeerRepository) FindByStream(ctx context.Context, streamID domain.StreamID) ([]*domain.Peer, error) {
	args := m.Called(ctx, streamID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Peer), args.Error(1)
}

func (m *MockPeerRepository) FindOptimalSource(ctx context.Context, streamID domain.StreamID, excludePeers []domain.PeerID) (*domain.Peer, error) {
	args := m.Called(ctx, streamID, excludePeers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Peer), args.Error(1)
}

func (m *MockPeerRepository) UpdateMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	args := m.Called(ctx, peerID, metrics)
	return args.Error(0)
}

func (m *MockPeerRepository) UpdatePeerLoad(ctx context.Context, peerID domain.PeerID, load int) error {
	args := m.Called(ctx, peerID, load)
	return args.Error(0)
}

type MockMeshRepository struct {
	mock.Mock
}

func (m *MockMeshRepository) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockMeshRepository) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	args := m.Called(ctx, fromPeer, toPeer)
	return args.Error(0)
}

func (m *MockMeshRepository) GetConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	args := m.Called(ctx, peerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PeerConnection), args.Error(1)
}

func (m *MockMeshRepository) BuildMesh(ctx context.Context, streamID domain.StreamID, maxConnections int) error {
	args := m.Called(ctx, streamID, maxConnections)
	return args.Error(0)
}

func (m *MockMeshRepository) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	args := m.Called(ctx, sourcePeer, targetPeer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.PeerID), args.Error(1)
}

func (m *MockMeshService) AddPeer(ctx context.Context, peer *domain.Peer) error {
	args := m.Called(ctx, peer)
	return args.Error(0)
}

func (m *MockMeshService) RemovePeer(ctx context.Context, peerID domain.PeerID) error {
	args := m.Called(ctx, peerID)
	return args.Error(0)
}

func (m *MockMeshService) UpdatePeerMetrics(ctx context.Context, peerID domain.PeerID, metrics domain.NetworkMetrics) error {
	args := m.Called(ctx, peerID, metrics)
	return args.Error(0)
}

func (m *MockMeshService) FindOptimalSources(ctx context.Context, streamID domain.StreamID, targetPeer domain.PeerID, count int) ([]*domain.Peer, error) {
	args := m.Called(ctx, streamID, targetPeer, count)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Peer), args.Error(1)
}

func (m *MockMeshService) BuildOptimalMesh(ctx context.Context, streamID domain.StreamID) error {
	args := m.Called(ctx, streamID)
	return args.Error(0)
}

func (m *MockMeshService) GetPeerConnections(ctx context.Context, peerID domain.PeerID) ([]*domain.PeerConnection, error) {
	args := m.Called(ctx, peerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PeerConnection), args.Error(1)
}

func (m *MockMeshService) AddConnection(ctx context.Context, conn *domain.PeerConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockMeshService) RemoveConnection(ctx context.Context, fromPeer, toPeer domain.PeerID) error {
	args := m.Called(ctx, fromPeer, toPeer)
	return args.Error(0)
}

func (m *MockMeshService) GetOptimalPath(ctx context.Context, sourcePeer, targetPeer domain.PeerID) ([]domain.PeerID, error) {
	args := m.Called(ctx, sourcePeer, targetPeer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.PeerID), args.Error(1)
}

func TestStreamService_CreateStream(t *testing.T) {
	// Setup
	mockStreamRepo := new(MockStreamRepository)
	mockPeerRepo := new(MockPeerRepository)
	mockMeshRepo := new(MockMeshRepository)
	mockMeshService := new(MockMeshService)
	metricsService := services.NewMetricsService()

	streamService := services.NewStreamService(
		mockStreamRepo,
		mockPeerRepo,
		mockMeshRepo,
		mockMeshService,
		metricsService,
	)

	ctx := context.Background()
	streamName := "test-stream"
	ownerID := domain.PeerID("owner-123")

	t.Run("successful stream creation", func(t *testing.T) {
		// Expectations
		mockStreamRepo.On("Create", ctx, mock.AnythingOfType("*domain.Stream")).Return(nil)

		// Execution
		stream, err := streamService.CreateStream(ctx, streamName, ownerID, 100)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, stream)
		assert.Equal(t, streamName, stream.Name)
		assert.Equal(t, ownerID, stream.Owner)
		assert.True(t, stream.Active)
		assert.Len(t, stream.QualityLevels, 3)

		mockStreamRepo.AssertExpectations(t)
	})

	t.Run("stream creation with repository error", func(t *testing.T) {
		// Expectations
		mockStreamRepo.On("Create", ctx, mock.AnythingOfType("*domain.Stream")).Return(assert.AnError)

		// Execution
		stream, err := streamService.CreateStream(ctx, streamName, ownerID, 100)

		// Assertions
		assert.Error(t, err)
		assert.Nil(t, stream)

		mockStreamRepo.AssertExpectations(t)
	})
}

func TestStreamService_JoinStream(t *testing.T) {
	// Setup
	mockStreamRepo := new(MockStreamRepository)
	mockPeerRepo := new(MockPeerRepository)
	mockMeshRepo := new(MockMeshRepository)
	mockMeshService := new(MockMeshService)
	metricsService := services.NewMetricsService()

	streamService := services.NewStreamService(
		mockStreamRepo,
		mockPeerRepo,
		mockMeshRepo,
		mockMeshService,
		metricsService,
	)

	ctx := context.Background()
	streamID := domain.StreamID("stream-123")
	peer := &domain.Peer{
		ID:       "peer-123",
		StreamID: streamID,
		Capabilities: domain.PeerCapabilities{
			IsPublisher: false,
			MaxBitrate:  1000,
		},
	}

	t.Run("successful join stream", func(t *testing.T) {
		// Expectations
		existingStream := &domain.Stream{
			ID:       streamID,
			Active:   true,
			MaxPeers: 100,
		}
		currentPeers := []*domain.Peer{} // Empty peer list

		mockStreamRepo.On("GetByID", ctx, streamID).Return(existingStream, nil)
		mockPeerRepo.On("FindByStream", ctx, streamID).Return(currentPeers, nil)
		mockPeerRepo.On("Add", ctx, peer).Return(nil)
		mockMeshService.On("AddPeer", ctx, peer).Return(nil)
		mockMeshRepo.On("BuildMesh", ctx, streamID, 4).Return(nil)

		// Execution
		err := streamService.JoinStream(ctx, streamID, peer)

		// Assertions
		assert.NoError(t, err)
		mockStreamRepo.AssertExpectations(t)
		mockPeerRepo.AssertExpectations(t)
		mockMeshService.AssertExpectations(t)
		mockMeshRepo.AssertExpectations(t)
	})

	t.Run("join non-existent stream", func(t *testing.T) {
		// Expectations
		mockStreamRepo.On("GetByID", ctx, streamID).Return(nil, domain.ErrStreamNotFound)

		// Execution
		err := streamService.JoinStream(ctx, streamID, peer)

		// Assertions
		assert.Error(t, err)
		assert.Equal(t, domain.ErrStreamNotFound, err)
	})

	t.Run("join inactive stream", func(t *testing.T) {
		// Expectations
		inactiveStream := &domain.Stream{
			ID:     streamID,
			Active: false,
		}
		mockStreamRepo.On("GetByID", ctx, streamID).Return(inactiveStream, nil)

		// Execution
		err := streamService.JoinStream(ctx, streamID, peer)

		// Assertions
		assert.Error(t, err)
		assert.Equal(t, domain.ErrStreamNotFound, err)
	})

	t.Run("join full stream", func(t *testing.T) {
		// Expectations
		existingStream := &domain.Stream{
			ID:       streamID,
			Active:   true,
			MaxPeers: 1,
		}
		currentPeers := []*domain.Peer{
			{ID: "existing-peer", StreamID: streamID},
		}

		mockStreamRepo.On("GetByID", ctx, streamID).Return(existingStream, nil)
		mockPeerRepo.On("FindByStream", ctx, streamID).Return(currentPeers, nil)

		// Execution
		err := streamService.JoinStream(ctx, streamID, peer)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "stream is full")
	})
}

func TestStreamService_LeaveStream(t *testing.T) {
	// Setup
	mockStreamRepo := new(MockStreamRepository)
	mockPeerRepo := new(MockPeerRepository)
	mockMeshRepo := new(MockMeshRepository)
	mockMeshService := new(MockMeshService)
	metricsService := services.NewMetricsService()

	streamService := services.NewStreamService(
		mockStreamRepo,
		mockPeerRepo,
		mockMeshRepo,
		mockMeshService,
		metricsService,
	)

	ctx := context.Background()
	streamID := domain.StreamID("stream-123")
	peerID := domain.PeerID("peer-123")
	peer := &domain.Peer{
		ID:       peerID,
		StreamID: streamID,
		Capabilities: domain.PeerCapabilities{
			IsPublisher: false,
		},
	}

	t.Run("successful leave stream", func(t *testing.T) {
		// Expectations
		connections := []*domain.PeerConnection{} // Empty connection list

		mockPeerRepo.On("GetByID", ctx, peerID).Return(peer, nil)
		mockMeshService.On("RemovePeer", ctx, peerID).Return(nil)
		mockPeerRepo.On("Remove", ctx, peerID).Return(nil)
		mockMeshRepo.On("GetConnections", ctx, peerID).Return(connections, nil)
		mockMeshRepo.On("BuildMesh", ctx, streamID, 4).Return(nil)

		// Execution
		err := streamService.LeaveStream(ctx, streamID, peerID)

		// Assertions
		assert.NoError(t, err)
		mockPeerRepo.AssertExpectations(t)
		mockMeshService.AssertExpectations(t)
		mockMeshRepo.AssertExpectations(t)
	})

	t.Run("leave stream with peer not found", func(t *testing.T) {
		// Expectations
		mockPeerRepo.On("GetByID", ctx, peerID).Return(nil, domain.ErrPeerNotFound)

		// Execution
		err := streamService.LeaveStream(ctx, streamID, peerID)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get peer")
	})
}

func TestStreamService_GetStreamStats(t *testing.T) {
	// Setup
	mockStreamRepo := new(MockStreamRepository)
	mockPeerRepo := new(MockPeerRepository)
	mockMeshRepo := new(MockMeshRepository)
	mockMeshService := new(MockMeshService)
	metricsService := services.NewMetricsService()

	streamService := services.NewStreamService(
		mockStreamRepo,
		mockPeerRepo,
		mockMeshRepo,
		mockMeshService,
		metricsService,
	)

	ctx := context.Background()
	streamID := domain.StreamID("test-stream")

	t.Run("successful stream stats retrieval", func(t *testing.T) {
		// Add metrics via metrics service
		metricsService.IncrementPublisherCount(streamID)
		metricsService.IncrementSubscriberCount(streamID)
		metricsService.IncrementSubscriberCount(streamID)
		metricsService.UpdateBitrate(streamID, 1500)
		metricsService.UpdateLatency(streamID, 100*time.Millisecond)

		// Execution
		stats, err := streamService.GetStreamStats(ctx, streamID)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, 1, stats.ActivePublishers)
		assert.Equal(t, 2, stats.ActiveSubscribers)
		assert.Equal(t, 1500, stats.TotalBitrate)
		assert.Equal(t, 100*time.Millisecond, stats.AverageLatency)
		assert.True(t, stats.HealthScore > 0)
		assert.NotZero(t, stats.Timestamp)
	})

	t.Run("stream stats for non-existent stream", func(t *testing.T) {
		// Execution for non-existent stream
		nonExistentStreamID := domain.StreamID("non-existent")
		stats, err := streamService.GetStreamStats(ctx, nonExistentStreamID)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, nonExistentStreamID, stats.StreamID)
		assert.Equal(t, 0, stats.ActivePublishers)
		assert.Equal(t, 0, stats.ActiveSubscribers)
		assert.Equal(t, 0, stats.TotalBitrate)
		assert.Equal(t, 0.0, stats.HealthScore)
	})
}

func TestStreamService_ListStreams(t *testing.T) {
	// Setup
	mockStreamRepo := new(MockStreamRepository)
	mockPeerRepo := new(MockPeerRepository)
	mockMeshRepo := new(MockMeshRepository)
	mockMeshService := new(MockMeshService)
	metricsService := services.NewMetricsService()

	streamService := services.NewStreamService(
		mockStreamRepo,
		mockPeerRepo,
		mockMeshRepo,
		mockMeshService,
		metricsService,
	)

	ctx := context.Background()

	t.Run("successful list streams", func(t *testing.T) {
		// Mock data
		streams := []*domain.Stream{
			{
				ID:        "stream-1",
				Name:      "Test Stream 1",
				Active:    true,
				CreatedAt: time.Now(),
			},
			{
				ID:        "stream-2",
				Name:      "Test Stream 2",
				Active:    true,
				CreatedAt: time.Now(),
			},
		}

		// Expectations
		mockStreamRepo.On("ListActive", ctx).Return(streams, nil)

		// Execution
		result, err := streamService.ListStreams(ctx)

		// Assertions
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "stream-1", string(result[0].ID))
		assert.Equal(t, "stream-2", string(result[1].ID))

		mockStreamRepo.AssertExpectations(t)
	})

	t.Run("list streams with error", func(t *testing.T) {
		// Expectations
		mockStreamRepo.On("ListActive", ctx).Return(nil, assert.AnError)

		// Execution
		result, err := streamService.ListStreams(ctx)

		// Assertions
		assert.Error(t, err)
		assert.Nil(t, result)

		mockStreamRepo.AssertExpectations(t)
	})
}

func TestQualityService(t *testing.T) {
	qualityService := services.NewQualityService()

	tests := []struct {
		name     string
		metrics  domain.NetworkMetrics
		expected string
	}{
		{
			name: "high quality conditions",
			metrics: domain.NetworkMetrics{
				BandwidthDown: 3000,
				BandwidthUp:   1500,
				PacketLoss:    0.005,
				Latency:       50 * time.Millisecond,
				Jitter:        20 * time.Millisecond,
			},
			expected: "high",
		},
		{
			name: "medium quality conditions",
			metrics: domain.NetworkMetrics{
				BandwidthDown: 1500,
				BandwidthUp:   800,
				PacketLoss:    0.03,
				Latency:       150 * time.Millisecond,
				Jitter:        40 * time.Millisecond,
			},
			expected: "medium",
		},
		{
			name: "low quality conditions",
			metrics: domain.NetworkMetrics{
				BandwidthDown: 400,
				BandwidthUp:   200,
				PacketLoss:    0.1,
				Latency:       400 * time.Millisecond,
				Jitter:        100 * time.Millisecond,
			},
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qualityService.DetermineOptimalQuality(tt.metrics)
			assert.Equal(t, tt.expected, result)
		})
	}
}
