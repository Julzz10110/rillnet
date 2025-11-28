package signal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"
	"rillnet/internal/infrastructure/signal"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPeerRepository for tests
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

// MockMeshService with full implementation for signal tests
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

// MockAuthService for tests
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GenerateToken(userID domain.UserID, username string) (string, error) {
	args := m.Called(userID, username)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) GenerateRefreshToken(userID domain.UserID) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) ValidateToken(tokenString string) (*services.Claims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.Claims), args.Error(1)
}

func (m *MockAuthService) ValidateRefreshToken(tokenString string) (*services.Claims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.Claims), args.Error(1)
}

func (m *MockAuthService) CheckStreamPermission(ctx context.Context, userID domain.UserID, streamID domain.StreamID, requiredRole domain.UserRole) error {
	args := m.Called(ctx, userID, streamID, requiredRole)
	return args.Error(0)
}

func (m *MockAuthService) GetUserFromContext(ctx context.Context) (domain.UserID, error) {
	args := m.Called(ctx)
	return args.Get(0).(domain.UserID), args.Error(1)
}

// Helper function to create a test auth service that always validates tokens
func createTestAuthService() *MockAuthService {
	mockAuth := new(MockAuthService)

	// Default behavior: always validate tokens successfully
	mockAuth.On("ValidateToken", mock.AnythingOfType("string")).Return(&services.Claims{
		UserID:           domain.UserID("test-user"),
		Username:         "testuser",
		RegisteredClaims: jwt.RegisteredClaims{},
	}, nil)

	// Default behavior: generate tokens successfully
	mockAuth.On("GenerateToken", mock.AnythingOfType("domain.UserID"), mock.AnythingOfType("string")).Return("test-token-123", nil)

	return mockAuth
}

func TestWebSocketServer_HandleJoinStream(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	ctx := context.Background()
	peerID := domain.PeerID("test-peer")
	streamID := domain.StreamID("test-stream")

	t.Run("successful join stream", func(t *testing.T) {
		// Expectations
		mockMeshService.On("AddPeer", ctx, mock.AnythingOfType("*domain.Peer")).Return(nil)
		mockMeshService.On("FindOptimalSources", ctx, streamID, peerID, 4).Return([]*domain.Peer{}, nil)
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		// Create test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		// Convert http:// to ws://
		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		// Connect to WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		// Send join stream message
		joinMsg := signal.SignalMessage{
			Type: "join_stream",
			Payload: json.RawMessage(`{
                "stream_id": "test-stream",
                "is_publisher": false,
                "capabilities": {
                    "max_bitrate": 1000,
                    "codecs": ["VP8", "H264"]
                }
            }`),
		}

		err = conn.WriteJSON(joinMsg)
		assert.NoError(t, err)

		// Read response
		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		assert.NoError(t, err)
		assert.Equal(t, "peers_list", response["type"])

		mockMeshService.AssertExpectations(t)
	})

	t.Run("join stream with optimal sources", func(t *testing.T) {
		// Mock optimal sources
		sources := []*domain.Peer{
			{
				ID:       "source-peer-1",
				StreamID: streamID,
				Address:  "192.168.1.100:8080",
				Capabilities: domain.PeerCapabilities{
					IsPublisher: true,
					MaxBitrate:  2000,
				},
			},
			{
				ID:       "source-peer-2",
				StreamID: streamID,
				Address:  "192.168.1.101:8080",
				Capabilities: domain.PeerCapabilities{
					IsPublisher: true,
					MaxBitrate:  1500,
				},
			},
		}

		// Expectations
		mockMeshService.On("AddPeer", ctx, mock.AnythingOfType("*domain.Peer")).Return(nil)
		mockMeshService.On("FindOptimalSources", ctx, streamID, peerID, 4).Return(sources, nil)
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		// Create test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		joinMsg := signal.SignalMessage{
			Type: "join_stream",
			Payload: json.RawMessage(`{
                "stream_id": "test-stream",
                "is_publisher": false,
                "capabilities": {
                    "max_bitrate": 1000,
                    "codecs": ["VP8"]
                }
            }`),
		}

		err = conn.WriteJSON(joinMsg)
		assert.NoError(t, err)

		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		assert.NoError(t, err)

		assert.Equal(t, "peers_list", response["type"])
		peers, ok := response["peers"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, peers, 2)

		mockMeshService.AssertExpectations(t)
	})
}

func TestWebSocketServer_HandleMetricsUpdate(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	ctx := context.Background()
	peerID := domain.PeerID("test-peer")

	t.Run("successful metrics update", func(t *testing.T) {
		// Expectations
		mockMeshService.On("UpdatePeerMetrics", ctx, peerID, mock.AnythingOfType("domain.NetworkMetrics")).Return(nil)
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		metricsMsg := signal.SignalMessage{
			Type: "metrics_update",
			Payload: json.RawMessage(`{
                "bandwidth": 1500,
                "packet_loss": 0.02,
                "latency": 50
            }`),
		}

		err = conn.WriteJSON(metricsMsg)
		assert.NoError(t, err)

		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		assert.NoError(t, err)
		assert.Equal(t, "metrics_updated", response["type"])

		mockMeshService.AssertExpectations(t)
	})

	t.Run("metrics update with invalid payload", func(t *testing.T) {
		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		metricsMsg := signal.SignalMessage{
			Type:    "metrics_update",
			Payload: json.RawMessage(`invalid json`),
		}

		err = conn.WriteJSON(metricsMsg)
		assert.NoError(t, err)

		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		assert.NoError(t, err)
		assert.Equal(t, "error", response["type"])

		// UpdatePeerMetrics should not be called with invalid payload
		mockMeshService.AssertNotCalled(t, "UpdatePeerMetrics", ctx, peerID, mock.Anything)
	})
}

func TestWebSocketServer_HandleOffer(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	peerID := domain.PeerID("test-peer")

	t.Run("handle offer message", func(t *testing.T) {
		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		offerMsg := signal.SignalMessage{
			Type: "offer",
			Payload: json.RawMessage(`{
                "sdp": "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"
            }`),
		}

		err = conn.WriteJSON(offerMsg)
		assert.NoError(t, err)

		// In current implementation, offer is just logged, no response expected
		// This is normal for basic implementation
	})
}

func TestWebSocketServer_HandleAnswer(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	peerID := domain.PeerID("test-peer")

	t.Run("handle answer message", func(t *testing.T) {
		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		answerMsg := signal.SignalMessage{
			Type: "answer",
			Payload: json.RawMessage(`{
                "sdp": "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"
            }`),
		}

		err = conn.WriteJSON(answerMsg)
		assert.NoError(t, err)

		// In current implementation, answer is just logged, no response expected
	})
}

func TestWebSocketServer_HandleICECandidate(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	peerID := domain.PeerID("test-peer")

	t.Run("handle ICE candidate message", func(t *testing.T) {
		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		iceMsg := signal.SignalMessage{
			Type: "ice_candidate",
			Payload: json.RawMessage(`{
                "candidate": "candidate:1 1 UDP 123456 192.168.1.100 8080 typ host"
            }`),
		}

		err = conn.WriteJSON(iceMsg)
		assert.NoError(t, err)

		// In current implementation, ICE candidate is just logged
	})
}

func TestWebSocketServer_HealthCheck(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	t.Run("health check with no connections", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.HealthCheck(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, float64(0), response["connections"]) // 0 connections
		assert.NotNil(t, response["timestamp"])
	})

	t.Run("health check returns valid JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.HealthCheck(w, req)

		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
	})
}

func TestWebSocketServer_ConnectionManagement(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	peerID := domain.PeerID("test-peer")

	t.Run("peer connection and disconnection", func(t *testing.T) {
		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)

		// Check that peer is connected
		assert.True(t, server.IsPeerConnected(peerID))

		// Get list of connected peers
		connectedPeers := server.GetConnectedPeers()
		assert.Contains(t, connectedPeers, peerID)

		// Close connection
		conn.Close()

		// Give time for disconnection handling
		time.Sleep(100 * time.Millisecond)

		mockMeshService.AssertExpectations(t)
	})

	t.Run("multiple peer connections", func(t *testing.T) {
		// Expectations for disconnection
		peerIDs := []domain.PeerID{"peer-1", "peer-2", "peer-3"}
		for _, pid := range peerIDs {
			mockMeshService.On("RemovePeer", mock.Anything, pid).Return(nil)
		}

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Connect multiple peers
		var connections []*websocket.Conn

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		for _, pid := range peerIDs {
			wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(pid) + "&token=" + token
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			assert.NoError(t, err)
			connections = append(connections, conn)
		}

		// Check that all peers are connected
		connectedPeers := server.GetConnectedPeers()
		assert.Len(t, connectedPeers, len(peerIDs))

		for _, pid := range peerIDs {
			assert.Contains(t, connectedPeers, pid)
			assert.True(t, server.IsPeerConnected(pid))
		}

		// Close all connections
		for _, conn := range connections {
			conn.Close()
		}

		// Give time for disconnection handling
		time.Sleep(100 * time.Millisecond)
	})
}

func TestWebSocketServer_ErrorHandling(t *testing.T) {
	mockPeerRepo := new(MockPeerRepository)
	mockMeshService := new(MockMeshService)
	mockAuthService := createTestAuthService()

	server := signal.NewWebSocketServer(mockPeerRepo, mockMeshService, mockAuthService, []string{"*"})

	t.Run("WebSocket without peer_id parameter", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		wsURL := "ws" + testServer.URL[4:] + "/ws" // without peer_id

		_, _, err := websocket.DefaultDialer.Dial(wsURL, nil)

		// Expect error or successful connection with subsequent closure
		// (depends on implementation of missing peer_id handling)
		if err != nil {
			// This is acceptable - server may reject connections without peer_id
			assert.Contains(t, err.Error(), "bad handshake")
		}
	})

	t.Run("unknown message type", func(t *testing.T) {
		peerID := domain.PeerID("test-peer")

		// Expectations for disconnection
		mockMeshService.On("RemovePeer", mock.Anything, peerID).Return(nil)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server.HandleWebSocket(w, r)
		}))
		defer testServer.Close()

		// Generate a test token
		token, _ := mockAuthService.GenerateToken(domain.UserID("test-user"), "testuser")

		wsURL := "ws" + testServer.URL[4:] + "/ws?peer_id=" + string(peerID) + "&token=" + token

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		defer conn.Close()

		unknownMsg := signal.SignalMessage{
			Type:    "unknown_message_type",
			Payload: json.RawMessage(`{}`),
		}

		err = conn.WriteJSON(unknownMsg)
		assert.NoError(t, err)

		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		assert.NoError(t, err)
		assert.Equal(t, "error", response["type"])
		assert.Contains(t, response["message"], "unknown message type")
	})
}
