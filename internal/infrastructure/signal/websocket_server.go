package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	rlog "rillnet/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Should be configured properly for production
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebSocketServer struct {
	peerRepo    ports.PeerRepository
	meshService ports.MeshService

	connections map[domain.PeerID]*websocket.Conn
	mu          sync.RWMutex

	pingInterval time.Duration
	pongTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration

	logger *zap.SugaredLogger
}

type SignalMessage struct {
	Type     string          `json:"type"`
	PeerID   domain.PeerID   `json:"peer_id,omitempty"`
	StreamID domain.StreamID `json:"stream_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type OfferPayload struct {
	SDP        string          `json:"sdp"`
	TargetPeer domain.PeerID   `json:"target_peer,omitempty"`
	StreamID   domain.StreamID `json:"stream_id,omitempty"`
}

type AnswerPayload struct {
	SDP        string          `json:"sdp"`
	TargetPeer domain.PeerID   `json:"target_peer,omitempty"`
	StreamID   domain.StreamID `json:"stream_id,omitempty"`
}

type ICECandidatePayload struct {
	Candidate  string          `json:"candidate"`
	TargetPeer domain.PeerID   `json:"target_peer,omitempty"`
	StreamID   domain.StreamID `json:"stream_id,omitempty"`
}

type MetricsUpdatePayload struct {
	Bandwidth  int     `json:"bandwidth"`
	PacketLoss float64 `json:"packet_loss"`
	Latency    int64   `json:"latency"` // in milliseconds
}

func NewWebSocketServer(peerRepo ports.PeerRepository, meshService ports.MeshService) *WebSocketServer {
	return &WebSocketServer{
		peerRepo:     peerRepo,
		meshService:  meshService,
		connections:  make(map[domain.PeerID]*websocket.Conn),
		pingInterval: 30 * time.Second, // Default ping interval
		pongTimeout:  60 * time.Second, // Default pong timeout
		readTimeout:  60 * time.Second, // Default read timeout
		writeTimeout: 10 * time.Second, // Default write timeout
		logger:       rlog.New("info").Sugar(),
	}
}

// SetPingInterval sets ping interval for WebSocket connections
func (s *WebSocketServer) SetPingInterval(interval time.Duration) {
	s.pingInterval = interval
}

// SetPongTimeout sets pong timeout for WebSocket connections
func (s *WebSocketServer) SetPongTimeout(timeout time.Duration) {
	s.pongTimeout = timeout
}

func (s *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Errorw("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	// Register connection
	peerID := domain.PeerID(r.URL.Query().Get("peer_id"))
	if peerID == "" {
		s.logger.Warn("missing peer_id in query parameters")
		return
	}

	// Check if peer is reconnecting (already exists)
	s.mu.Lock()
	existingConn, isReconnect := s.connections[peerID]
	if isReconnect && existingConn != nil {
		// Close old connection
		existingConn.Close()
		s.logger.Infow("closing old connection for reconnecting peer", "peer_id", peerID)
	}
	s.connections[peerID] = conn
	s.mu.Unlock()

	s.logger.Infow("peer connected via WebSocket", "peer_id", peerID, "reconnect", isReconnect)

	// Set read/write deadlines
	conn.SetReadDeadline(time.Now().Add(s.readTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.readTimeout))
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(s.pingInterval)
	defer pingTicker.Stop()

	// Channel for message processing
	messageChan := make(chan SignalMessage, 10)
	errorChan := make(chan error, 1)

	// Start message reader goroutine
	go func() {
		for {
			var msg SignalMessage
			if err := conn.ReadJSON(&msg); err != nil {
				errorChan <- err
				return
			}
			conn.SetReadDeadline(time.Now().Add(s.readTimeout))
			messageChan <- msg
		}
	}()

	// Process messages and ping
	for {
		select {
		case msg := <-messageChan:
			if err := s.handleMessage(context.Background(), peerID, msg); err != nil {
				s.logger.Infow("error handling message from peer", "peer_id", peerID, "error", err)
				s.sendError(conn, err.Error())
			}

		case <-pingTicker.C:
			// Send ping
			conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.logger.Infow("error sending ping", "peer_id", peerID, "error", err)
				goto cleanup
			}

		case err := <-errorChan:
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Infow("error reading message from peer", "peer_id", peerID, "error", err)
			}
			goto cleanup
		}
	}

cleanup:
	// Clean up on disconnect
	s.mu.Lock()
	delete(s.connections, peerID)
	s.mu.Unlock()

	if err := s.meshService.RemovePeer(context.Background(), peerID); err != nil {
		s.logger.Infow("error removing peer from mesh", "peer_id", peerID, "error", err)
	}

	s.logger.Infow("peer disconnected", "peer_id", peerID)
}

func (s *WebSocketServer) handleMessage(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	// Validate message type
	if msg.Type == "" {
		return fmt.Errorf("message type is required")
	}

	// Validate peer ID matches
	if msg.PeerID != "" && msg.PeerID != peerID {
		return fmt.Errorf("peer_id mismatch: expected %s, got %s", peerID, msg.PeerID)
	}

	switch msg.Type {
	case "join_stream":
		return s.handleJoinStream(ctx, peerID, msg)
	case "offer":
		return s.handleOffer(ctx, peerID, msg)
	case "answer":
		return s.handleAnswer(ctx, peerID, msg)
	case "ice_candidate":
		return s.handleICECandidate(ctx, peerID, msg)
	case "metrics_update":
		return s.handleMetricsUpdate(ctx, peerID, msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

func (s *WebSocketServer) handleJoinStream(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	var payload struct {
		StreamID     domain.StreamID `json:"stream_id"`
		IsPublisher  bool            `json:"is_publisher"`
		Capabilities struct {
			MaxBitrate int      `json:"max_bitrate"`
			Codecs     []string `json:"codecs"`
		} `json:"capabilities"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid join_stream payload: %w", err)
	}

	// Validate stream ID
	if payload.StreamID == "" {
		return fmt.Errorf("stream_id is required")
	}

	// Validate stream ID format (basic check)
	if err := s.validateStreamID(ctx, payload.StreamID); err != nil {
		return fmt.Errorf("invalid stream_id: %w", err)
	}

	// Validate capabilities
	if payload.Capabilities.MaxBitrate < 0 {
		return fmt.Errorf("max_bitrate must be >= 0")
	}

	peer := &domain.Peer{
		ID:        peerID,
		StreamID:  payload.StreamID,
		SessionID: domain.SessionID(generateSessionID()),
		Address:   "dynamic", // In real implementation, actual address should be obtained
		Capabilities: domain.PeerCapabilities{
			MaxBitrate:      payload.Capabilities.MaxBitrate,
			SupportedCodecs: payload.Capabilities.Codecs,
			IsPublisher:     payload.IsPublisher,
			CanRelay:        true,
		},
		Metrics: domain.PeerMetrics{
			Bandwidth:   payload.Capabilities.MaxBitrate,
			PacketLoss:  0.0,
			Latency:     0,
			CPUUsage:    0.0,
			MemoryUsage: 0,
		},
		LastSeen: time.Now(),
	}

	// Add peer to system
	if err := s.meshService.AddPeer(ctx, peer); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	// Find optimal sources for P2P connections
	sources, err := s.meshService.FindOptimalSources(ctx, payload.StreamID, peerID, 4)
	if err != nil {
		// If no sources found, continue anyway
		s.logger.Infow("no optimal sources found for peer", "peer_id", peerID, "error", err)
		sources = []*domain.Peer{}
	}

	var peerList []map[string]interface{}
	for _, source := range sources {
		peerList = append(peerList, map[string]interface{}{
			"peer_id": source.ID,
			"address": source.Address,
			"quality": "auto",
		})
	}

	response := map[string]interface{}{
		"type":  "peers_list",
		"peers": peerList,
	}

	return s.sendToPeer(peerID, response)
}

func (s *WebSocketServer) handleOffer(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	var payload OfferPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid offer payload: %w", err)
	}

	// Validate SDP
	if err := s.validateSDP(payload.SDP); err != nil {
		return fmt.Errorf("invalid SDP in offer: %w", err)
	}

	// Validate stream ID if provided
	if payload.StreamID != "" {
		if err := s.validateStreamID(ctx, payload.StreamID); err != nil {
			return fmt.Errorf("invalid stream_id: %w", err)
		}
	}

	// Determine target peer
	targetPeerID, err := s.determineTargetPeer(ctx, peerID, payload.TargetPeer, payload.StreamID, msg.StreamID)
	if err != nil {
		return fmt.Errorf("failed to determine target peer: %w", err)
	}

	// Validate target peer exists and is connected
	if !s.IsPeerConnected(targetPeerID) {
		return fmt.Errorf("target peer %s is not connected", targetPeerID)
	}

	// Forward offer to target peer
	response := map[string]interface{}{
		"type":      "offer",
		"from_peer": peerID,
		"stream_id": payload.StreamID,
		"payload": map[string]interface{}{
			"sdp": payload.SDP,
		},
	}

	s.logger.Infow("routing offer",
		"from_peer", peerID,
		"to_peer", targetPeerID,
		"stream_id", payload.StreamID,
		"sdp_length", len(payload.SDP),
	)

	return s.sendToPeer(targetPeerID, response)
}

func (s *WebSocketServer) handleAnswer(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	var payload AnswerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid answer payload: %w", err)
	}

	// Validate SDP
	if err := s.validateSDP(payload.SDP); err != nil {
		return fmt.Errorf("invalid SDP in answer: %w", err)
	}

	// Validate stream ID if provided
	if payload.StreamID != "" {
		if err := s.validateStreamID(ctx, payload.StreamID); err != nil {
			return fmt.Errorf("invalid stream_id: %w", err)
		}
	}

	// Determine target peer (usually the publisher who sent the offer)
	targetPeerID, err := s.determineTargetPeer(ctx, peerID, payload.TargetPeer, payload.StreamID, msg.StreamID)
	if err != nil {
		return fmt.Errorf("failed to determine target peer: %w", err)
	}

	// Validate target peer exists and is connected
	if !s.IsPeerConnected(targetPeerID) {
		return fmt.Errorf("target peer %s is not connected", targetPeerID)
	}

	// Forward answer to target peer
	response := map[string]interface{}{
		"type":      "answer",
		"from_peer": peerID,
		"stream_id": payload.StreamID,
		"payload": map[string]interface{}{
			"sdp": payload.SDP,
		},
	}

	s.logger.Infow("routing answer",
		"from_peer", peerID,
		"to_peer", targetPeerID,
		"stream_id", payload.StreamID,
		"sdp_length", len(payload.SDP),
	)

	return s.sendToPeer(targetPeerID, response)
}

func (s *WebSocketServer) handleICECandidate(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	var payload ICECandidatePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid ICE candidate payload: %w", err)
	}

	// Validate candidate
	if payload.Candidate == "" {
		return fmt.Errorf("ICE candidate is required")
	}

	// Determine target peer
	targetPeerID, err := s.determineTargetPeer(ctx, peerID, payload.TargetPeer, payload.StreamID, msg.StreamID)
	if err != nil {
		return fmt.Errorf("failed to determine target peer: %w", err)
	}

	// Validate target peer exists and is connected
	if !s.IsPeerConnected(targetPeerID) {
		return fmt.Errorf("target peer %s is not connected", targetPeerID)
	}

	// Forward ICE candidate to target peer
	response := map[string]interface{}{
		"type":      "ice_candidate",
		"from_peer": peerID,
		"stream_id": payload.StreamID,
		"payload": map[string]interface{}{
			"candidate": payload.Candidate,
		},
	}

	s.logger.Debugw("routing ICE candidate",
		"from_peer", peerID,
		"to_peer", targetPeerID,
		"stream_id", payload.StreamID,
	)

	return s.sendToPeer(targetPeerID, response)
}

func (s *WebSocketServer) handleMetricsUpdate(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	var payload MetricsUpdatePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	// Update peer metrics
	metrics := domain.NetworkMetrics{
		Timestamp:        time.Now(),
		BandwidthDown:    payload.Bandwidth,
		BandwidthUp:      payload.Bandwidth, // In real system, these would be separate values
		PacketLoss:       payload.PacketLoss,
		Latency:          time.Duration(payload.Latency) * time.Millisecond,
		Jitter:           0, // Not supported yet
		AvailableBitrate: payload.Bandwidth,
	}

	if err := s.meshService.UpdatePeerMetrics(ctx, peerID, metrics); err != nil {
		return fmt.Errorf("failed to update peer metrics: %w", err)
	}

	s.logger.Infow("updated peer metrics",
		"peer_id", peerID,
		"bandwidth", payload.Bandwidth,
		"packet_loss", payload.PacketLoss,
		"latency_ms", payload.Latency,
	)

	// Send confirmation
	response := map[string]interface{}{
		"type":      "metrics_updated",
		"timestamp": time.Now().Unix(),
	}

	return s.sendToPeer(peerID, response)
}

// validateSDP validates SDP format
func (s *WebSocketServer) validateSDP(sdp string) error {
	if sdp == "" {
		return fmt.Errorf("SDP cannot be empty")
	}

	// Basic SDP format validation
	// SDP should start with "v=" (version)
	if len(sdp) < 2 || sdp[:2] != "v=" {
		return fmt.Errorf("invalid SDP format: must start with 'v='")
	}

	// Check for required SDP fields
	requiredFields := []string{"v=", "o=", "s=", "t="}
	for _, field := range requiredFields {
		if !strings.Contains(sdp, field) {
			return fmt.Errorf("invalid SDP format: missing required field '%s'", field)
		}
	}

	return nil
}

// validateStreamID validates stream ID format and existence
func (s *WebSocketServer) validateStreamID(ctx context.Context, streamID domain.StreamID) error {
	if streamID == "" {
		return fmt.Errorf("stream_id cannot be empty")
	}

	// Basic format validation (alphanumeric, dash, underscore)
	if len(string(streamID)) < 1 || len(string(streamID)) > 100 {
		return fmt.Errorf("stream_id must be between 1 and 100 characters")
	}

	// Note: In a full implementation, we would check if stream exists in repository
	// For now, we just validate format
	return nil
}

// validatePeerID validates peer ID format and existence
func (s *WebSocketServer) validatePeerID(ctx context.Context, peerID domain.PeerID) error {
	if peerID == "" {
		return fmt.Errorf("peer_id cannot be empty")
	}

	// Check if peer exists
	_, err := s.peerRepo.GetByID(ctx, peerID)
	if err != nil {
		return fmt.Errorf("peer %s not found: %w", peerID, err)
	}

	return nil
}

// determineTargetPeer determines the target peer for message routing
func (s *WebSocketServer) determineTargetPeer(ctx context.Context, fromPeer domain.PeerID, explicitTarget domain.PeerID, payloadStreamID domain.StreamID, messageStreamID domain.StreamID) (domain.PeerID, error) {
	// Priority 1: Explicit target peer in payload
	if explicitTarget != "" {
		// Validate that target peer exists
		_, err := s.peerRepo.GetByID(ctx, explicitTarget)
		if err != nil {
			return "", fmt.Errorf("target peer %s not found: %w", explicitTarget, err)
		}
		return explicitTarget, nil
	}

	// Priority 2: Find publisher in the stream
	streamID := payloadStreamID
	if streamID == "" {
		streamID = messageStreamID
	}

	if streamID != "" {
		// Find publisher in this stream
		peers, err := s.peerRepo.FindByStream(ctx, streamID)
		if err != nil {
			return "", fmt.Errorf("failed to find peers in stream: %w", err)
		}

		// Find first publisher (excluding the sender)
		for _, peer := range peers {
			if peer.ID != fromPeer && peer.Capabilities.IsPublisher {
				return peer.ID, nil
			}
		}

		// If no publisher found, find any peer in the stream (for P2P mesh)
		for _, peer := range peers {
			if peer.ID != fromPeer {
				return peer.ID, nil
			}
		}

		return "", fmt.Errorf("no target peer found in stream %s", streamID)
	}

	return "", fmt.Errorf("cannot determine target peer: no target_peer or stream_id provided")
}

func (s *WebSocketServer) sendToPeer(peerID domain.PeerID, data interface{}) error {
	s.mu.RLock()
	conn, exists := s.connections[peerID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("peer %s not connected", peerID)
	}

	return conn.WriteJSON(data)
}

func (s *WebSocketServer) sendError(conn *websocket.Conn, message string) {
	errorMsg := map[string]interface{}{
		"type":    "error",
		"message": message,
	}
	conn.WriteJSON(errorMsg)
}

func (s *WebSocketServer) HealthCheck(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	connectionCount := len(s.connections)
	s.mu.RUnlock()

	response := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Unix(),
		"connections": connectionCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *WebSocketServer) BroadcastToStream(streamID domain.StreamID, message interface{}) error {
	// In real implementation, all peers in stream would be found and message broadcasted
	// This is a simplified version
	s.mu.RLock()
	defer s.mu.RUnlock()

	var errors []error
	for peerID, conn := range s.connections {
		if err := conn.WriteJSON(message); err != nil {
			errors = append(errors, fmt.Errorf("failed to send to peer %s: %w", peerID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("broadcast completed with %d errors", len(errors))
	}

	return nil
}

// Additional methods for connection management

func (s *WebSocketServer) GetConnectedPeers() []domain.PeerID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]domain.PeerID, 0, len(s.connections))
	for peerID := range s.connections {
		peers = append(peers, peerID)
	}

	return peers
}

func (s *WebSocketServer) IsPeerConnected(peerID domain.PeerID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.connections[peerID]
	return exists
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
