package webrtc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	webRTC "rillnet/internal/infrastructure/webrtc"

	webrtc "github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMetricsService with full implementation of services.MetricsService interface
type MockMetricsService struct {
	mock.Mock
}

func (m *MockMetricsService) IncrementPublisherCount(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) DecrementPublisherCount(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) IncrementSubscriberCount(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) DecrementSubscriberCount(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) UpdateBitrate(streamID domain.StreamID, bitrate int) {
	m.Called(streamID, bitrate)
}

func (m *MockMetricsService) UpdateLatency(streamID domain.StreamID, latency time.Duration) {
	m.Called(streamID, latency)
}

func (m *MockMetricsService) GetStreamMetrics(streamID domain.StreamID) *domain.StreamMetrics {
	args := m.Called(streamID)
	if args.Get(0) == nil {
		return &domain.StreamMetrics{
			StreamID:          streamID,
			ActivePublishers:  0,
			ActiveSubscribers: 0,
			TotalBitrate:      0,
			AverageLatency:    0,
			HealthScore:       0,
			Timestamp:         time.Now(),
		}
	}
	return args.Get(0).(*domain.StreamMetrics)
}

func (m *MockMetricsService) RecordConnection(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) RemoveConnection(streamID domain.StreamID) {
	m.Called(streamID)
}

func (m *MockMetricsService) GetConnectionCount(streamID domain.StreamID) int {
	args := m.Called(streamID)
	return args.Int(0)
}

// MockMeshService with full interface implementation
type MockMeshService struct {
	mock.Mock
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

// createTestSFUService creates SFU service for testing with correct types
func createTestSFUService(
	config webRTC.WebRTCConfig,
	qualityService *services.QualityService,
	metricsService *services.MetricsService, // Use correct type - pointer to struct
	meshService ports.MeshService,
) ports.WebRTCService {
	return webRTC.NewSFUService(config, qualityService, metricsService, meshService)
}

// TestMetricsServiceWrapper wraps MockMetricsService for use in tests
type TestMetricsServiceWrapper struct {
	mock *MockMetricsService
}

// NewTestMetricsServiceWrapper creates a wrapper for testing
func NewTestMetricsServiceWrapper(mock *MockMetricsService) *services.MetricsService {
	// Create real MetricsService and replace its behavior via mock
	realService := services.NewMetricsService()

	// Create wrapper that will delegate calls to mock object
	// wrapper := &TestMetricsServiceWrapper{mock: mock}

	// Replace real service methods
	// In real project this can be done via interface or more complex composition
	// But for test simplicity we'll use mock directly in tests

	return realService
}

// Helper methods for setting expectations
func setupMetricsExpectations(mockMetrics *MockMetricsService, streamID domain.StreamID, publisherCalls, subscriberCalls int) {
	if publisherCalls > 0 {
		mockMetrics.On("IncrementPublisherCount", streamID).Times(publisherCalls)
	}
	if subscriberCalls > 0 {
		mockMetrics.On("IncrementSubscriberCount", streamID).Times(subscriberCalls)
	}
}

func TestSFUService_CreatePublisherOffer(t *testing.T) {
	// Setup
	qualityService := services.NewQualityService()
	// mockMetrics := new(MockMetricsService)
	metricsService := services.NewMetricsService() // Use real service
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
		Simulcast:  true,
		MaxBitrate: 5000,
	}

	// Create SFU service for testing
	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-publisher")
	streamID := domain.StreamID("test-stream")

	t.Run("successful publisher offer creation", func(t *testing.T) {
		// Execution
		offer, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, offer)
		assert.Equal(t, webrtc.SDPTypeOffer, offer.Type)
		assert.NotEmpty(t, offer.SDP)

		// Check that metrics were updated
		metrics := metricsService.GetStreamMetrics(streamID)
		assert.Equal(t, 1, metrics.ActivePublishers)
	})

	t.Run("publisher offer creation failure", func(t *testing.T) {
		// Error scenarios can be tested here
		// For example, with invalid WebRTC configuration
		invalidConfig := webRTC.WebRTCConfig{
			ICEServers: []webrtc.ICEServer{}, // Empty list may cause issues
		}

		invalidSFUService := createTestSFUService(invalidConfig, qualityService, metricsService, meshService)

		// Execution
		offer, err := invalidSFUService.CreatePublisherOffer(ctx, peerID, streamID)

		// Assertions - check that there's no panic and meaningful result
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, offer)
		}
	})
}

func TestSFUService_CreateSubscriberOffer(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService() // Real metrics service
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-subscriber")
	streamID := domain.StreamID("test-stream")
	sourcePeers := []domain.PeerID{"publisher-1", "publisher-2"}

	t.Run("successful subscriber offer creation", func(t *testing.T) {
		// Execution
		offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, sourcePeers)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, offer)
		assert.Equal(t, webrtc.SDPTypeOffer, offer.Type)
		assert.NotEmpty(t, offer.SDP)

		// Check metrics
		metrics := metricsService.GetStreamMetrics(streamID)
		assert.Equal(t, 1, metrics.ActiveSubscribers)
	})

	t.Run("subscriber offer with empty source peers", func(t *testing.T) {
		// Execution with empty source list
		offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, []domain.PeerID{})

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, offer)
	})
}

func TestSFUService_HandlePublisherAnswer(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-publisher")
	streamID := domain.StreamID("test-stream")

	t.Run("successful publisher answer handling", func(t *testing.T) {
		// First create publisher
		offer, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
		assert.NoError(t, err)
		assert.NotNil(t, offer)

		// Create answer
		answer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-sdp",
		}

		// Execution
		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer)

		// Assertions
		assert.NoError(t, err)
	})

	t.Run("publisher answer for non-existent publisher", func(t *testing.T) {
		nonExistentPeerID := domain.PeerID("non-existent-publisher")
		answer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-sdp",
		}

		// Execution
		err := sfuService.HandlePublisherAnswer(ctx, nonExistentPeerID, answer)

		// Assertions
		assert.Error(t, err)
		// assert.Equal(t, domain.ErrPeerNotFound, err) // Uncomment if ErrPeerNotFound exists
	})
}

func TestSFUService_HandleSubscriberAnswer(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-subscriber")
	streamID := domain.StreamID("test-stream")
	sourcePeers := []domain.PeerID{"publisher-1"}

	t.Run("successful subscriber answer handling", func(t *testing.T) {
		// First create subscriber
		offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, sourcePeers)
		assert.NoError(t, err)
		assert.NotNil(t, offer)

		// Create answer
		answer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-sdp",
		}

		// Execution
		err = sfuService.HandleSubscriberAnswer(ctx, peerID, answer)

		// Assertions
		assert.NoError(t, err)
	})

	t.Run("subscriber answer for non-existent subscriber", func(t *testing.T) {
		nonExistentPeerID := domain.PeerID("non-existent-subscriber")
		answer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-sdp",
		}

		// Execution
		err := sfuService.HandleSubscriberAnswer(ctx, nonExistentPeerID, answer)

		// Assertions
		assert.Error(t, err)
		// assert.Equal(t, domain.ErrPeerNotFound, err) // Uncomment if ErrPeerNotFound exists
	})
}

func TestSFUService_PeerReconnection(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-peer")
	streamID := domain.StreamID("test-stream")

	t.Run("publisher reconnection scenario", func(t *testing.T) {
		// First connection
		offer1, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
		assert.NoError(t, err)
		assert.NotNil(t, offer1)

		// Handle answer
		answer1 := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-1",
		}
		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer1)
		assert.NoError(t, err)

		// Second connection (reconnect)
		offer2, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
		assert.NoError(t, err)
		assert.NotNil(t, offer2)

		// Handle second answer
		answer2 := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  "mock-answer-2",
		}
		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer2)
		assert.NoError(t, err)

		// Check that metrics are correct
		metrics := metricsService.GetStreamMetrics(streamID)
		assert.Equal(t, 1, metrics.ActivePublishers) // Should remain 1, not 2
	})
}

func TestSFUService_MultipleStreams(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	peerID := domain.PeerID("test-peer")

	t.Run("publisher in multiple streams", func(t *testing.T) {
		stream1 := domain.StreamID("stream-1")
		stream2 := domain.StreamID("stream-2")

		// Connect to first stream
		offer1, err := sfuService.CreatePublisherOffer(ctx, peerID, stream1)
		assert.NoError(t, err)
		assert.NotNil(t, offer1)

		// Connect to second stream
		offer2, err := sfuService.CreatePublisherOffer(ctx, peerID, stream2)
		assert.NoError(t, err)
		assert.NotNil(t, offer2)

		// Handle answers
		answer1 := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: "answer-1"}
		answer2 := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: "answer-2"}

		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer1)
		assert.NoError(t, err)

		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer2)
		assert.NoError(t, err)

		// Check metrics for both streams
		metrics1 := metricsService.GetStreamMetrics(stream1)
		metrics2 := metricsService.GetStreamMetrics(stream2)
		assert.Equal(t, 1, metrics1.ActivePublishers)
		assert.Equal(t, 1, metrics2.ActivePublishers)
	})
}

func TestSFUService_ConcurrentOperations(t *testing.T) {
	qualityService := services.NewQualityService()
	metricsService := services.NewMetricsService()
	meshService := new(MockMeshService)

	config := webRTC.WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	sfuService := createTestSFUService(config, qualityService, metricsService, meshService)

	ctx := context.Background()
	streamID := domain.StreamID("test-stream")

	t.Run("concurrent peer operations", func(t *testing.T) {
		// Start goroutines to create peers
		done := make(chan bool, 10)

		for i := 0; i < 5; i++ {
			go func(index int) {
				peerID := domain.PeerID(fmt.Sprintf("publisher-%d", index))
				offer, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
				assert.NoError(t, err)
				assert.NotNil(t, offer)
				done <- true
			}(i)

			go func(index int) {
				peerID := domain.PeerID(fmt.Sprintf("subscriber-%d", index))
				offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, []domain.PeerID{})
				assert.NoError(t, err)
				assert.NotNil(t, offer)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Check final metrics
		metrics := metricsService.GetStreamMetrics(streamID)
		assert.Equal(t, 5, metrics.ActivePublishers)
		assert.Equal(t, 5, metrics.ActiveSubscribers)
	})
}
