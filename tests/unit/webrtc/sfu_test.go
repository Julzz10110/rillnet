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
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/retry"

	webrtc "github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	// Use default retry and circuit breaker configs for tests (disabled by default)
	retryCfg := retry.Config{
		Enabled:      false, // Disable retry in tests for predictable behavior
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}
	cbCfg := circuitbreaker.Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxRequestsHalfOpen: 3,
	}
	return webRTC.NewSFUService(config, qualityService, metricsService, meshService, retryCfg, cbCfg)
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
		for _, pubID := range sourcePeers {
			_, err := sfuService.CreatePublisherOffer(ctx, pubID, streamID)
			assert.NoError(t, err)
		}

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

	t.Run("subscriber offer with empty source peers uses active publisher", func(t *testing.T) {
		pubPeer := domain.PeerID("publisher-for-empty-sources")
		_, err := sfuService.CreatePublisherOffer(ctx, pubPeer, streamID)
		assert.NoError(t, err)

		offer, err := sfuService.CreateSubscriberOffer(ctx, domain.PeerID("sub-empty-sources"), streamID, []domain.PeerID{})
		assert.NoError(t, err)
		assert.NotNil(t, offer)
		assert.NotEmpty(t, offer.SDP)
	})

	t.Run("subscriber offer without publisher returns ErrNoPublisherMedia", func(t *testing.T) {
		_, err := sfuService.CreateSubscriberOffer(ctx, domain.PeerID("sub-no-publisher"), domain.StreamID("no-publisher-stream"), []domain.PeerID{})
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNoPublisherMedia)
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

	createAnswer := func(t *testing.T, offer webrtc.SessionDescription) webrtc.SessionDescription {
		t.Helper()
		pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc.Close()

		require.NoError(t, pc.SetRemoteDescription(offer))
		answer, err := pc.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc.SetLocalDescription(answer))
		<-webrtc.GatheringCompletePromise(pc)
		return *pc.LocalDescription()
	}

	t.Run("successful publisher answer handling", func(t *testing.T) {
		// First create publisher
		offer, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
		assert.NoError(t, err)
		assert.NotNil(t, offer)

		answer := createAnswer(t, offer)

		// Execution
		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer)

		// Assertions
		assert.NoError(t, err)
	})

	t.Run("publisher answer for non-existent publisher", func(t *testing.T) {
		nonExistentPeerID := domain.PeerID("non-existent-publisher")
		// Valid SDP answer (generated from a real offer), but peer doesn't exist in SFU.
		tmpOffer, err := sfuService.CreatePublisherOffer(ctx, domain.PeerID("tmp-publisher"), streamID)
		require.NoError(t, err)
		answer := createAnswer(t, tmpOffer)

		// Execution
		err = sfuService.HandlePublisherAnswer(ctx, nonExistentPeerID, answer)

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
		_, err := sfuService.CreatePublisherOffer(ctx, sourcePeers[0], streamID)
		assert.NoError(t, err)

		offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, sourcePeers)
		assert.NoError(t, err)
		assert.NotNil(t, offer)

		pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc.Close()
		require.NoError(t, pc.SetRemoteDescription(offer))
		answer, err := pc.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc.SetLocalDescription(answer))
		<-webrtc.GatheringCompletePromise(pc)
		finalAnswer := *pc.LocalDescription()

		// Execution
		err = sfuService.HandleSubscriberAnswer(ctx, peerID, finalAnswer)

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
		pc1, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc1.Close()
		require.NoError(t, pc1.SetRemoteDescription(offer1))
		a1, err := pc1.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc1.SetLocalDescription(a1))
		<-webrtc.GatheringCompletePromise(pc1)
		answer1 := *pc1.LocalDescription()
		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer1)
		assert.NoError(t, err)

		// Second connection (reconnect)
		offer2, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
		assert.NoError(t, err)
		assert.NotNil(t, offer2)

		// Handle second answer
		pc2, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc2.Close()
		require.NoError(t, pc2.SetRemoteDescription(offer2))
		a2, err := pc2.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc2.SetLocalDescription(a2))
		<-webrtc.GatheringCompletePromise(pc2)
		answer2 := *pc2.LocalDescription()
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
		pc1, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc1.Close()
		require.NoError(t, pc1.SetRemoteDescription(offer1))
		a1, err := pc1.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc1.SetLocalDescription(a1))
		<-webrtc.GatheringCompletePromise(pc1)
		answer1 := *pc1.LocalDescription()

		pc2, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc2.Close()
		require.NoError(t, pc2.SetRemoteDescription(offer2))
		a2, err := pc2.CreateAnswer(nil)
		require.NoError(t, err)
		require.NoError(t, pc2.SetLocalDescription(a2))
		<-webrtc.GatheringCompletePromise(pc2)
		answer2 := *pc2.LocalDescription()

		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer1)
		assert.NoError(t, err)

		err = sfuService.HandlePublisherAnswer(ctx, peerID, answer2)
		assert.NoError(t, err)

		// Check metrics for both streams
		metrics1 := metricsService.GetStreamMetrics(stream1)
		metrics2 := metricsService.GetStreamMetrics(stream2)
		// Same peer re-publishes on a different stream => stream1 publisher is replaced.
		assert.Equal(t, 0, metrics1.ActivePublishers)
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
		done := make(chan bool, 10)

		for i := 0; i < 5; i++ {
			go func(index int) {
				peerID := domain.PeerID(fmt.Sprintf("publisher-%d", index))
				offer, err := sfuService.CreatePublisherOffer(ctx, peerID, streamID)
				assert.NoError(t, err)
				assert.NotNil(t, offer)
				done <- true
			}(i)
		}

		for i := 0; i < 5; i++ {
			<-done
		}

		for i := 0; i < 5; i++ {
			go func(index int) {
				peerID := domain.PeerID(fmt.Sprintf("subscriber-%d", index))
				offer, err := sfuService.CreateSubscriberOffer(ctx, peerID, streamID, []domain.PeerID{})
				assert.NoError(t, err)
				assert.NotNil(t, offer)
				done <- true
			}(i)
		}

		for i := 0; i < 5; i++ {
			<-done
		}

		// Check final metrics
		metrics := metricsService.GetStreamMetrics(streamID)
		assert.Equal(t, 5, metrics.ActivePublishers)
		assert.Equal(t, 5, metrics.ActiveSubscribers)
	})
}
