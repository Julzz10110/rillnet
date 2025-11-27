package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/gorilla/websocket"
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
}

type SignalMessage struct {
	Type     string          `json:"type"`
	PeerID   domain.PeerID   `json:"peer_id,omitempty"`
	StreamID domain.StreamID `json:"stream_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type OfferPayload struct {
	SDP string `json:"sdp"`
}

type AnswerPayload struct {
	SDP string `json:"sdp"`
}

type ICECandidatePayload struct {
	Candidate string `json:"candidate"`
}

type MetricsUpdatePayload struct {
	Bandwidth  int     `json:"bandwidth"`
	PacketLoss float64 `json:"packet_loss"`
	Latency    int64   `json:"latency"` // in milliseconds
}

func NewWebSocketServer(peerRepo ports.PeerRepository, meshService ports.MeshService) *WebSocketServer {
	return &WebSocketServer{
		peerRepo:    peerRepo,
		meshService: meshService,
		connections: make(map[domain.PeerID]*websocket.Conn),
	}
}

func (s *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Register connection
	peerID := domain.PeerID(r.URL.Query().Get("peer_id"))
	if peerID == "" {
		log.Printf("Missing peer_id in query parameters")
		return
	}

	s.mu.Lock()
	s.connections[peerID] = conn
	s.mu.Unlock()

	log.Printf("Peer %s connected via WebSocket", peerID)

	// Process messages
	for {
		var msg SignalMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Error reading message from peer %s: %v", peerID, err)
			break
		}

		if err := s.handleMessage(context.Background(), peerID, msg); err != nil {
			log.Printf("Error handling message from peer %s: %v", peerID, err)
			s.sendError(conn, err.Error())
		}
	}

	// Clean up on disconnect
	s.mu.Lock()
	delete(s.connections, peerID)
	s.mu.Unlock()

	if err := s.meshService.RemovePeer(context.Background(), peerID); err != nil {
		log.Printf("Error removing peer %s: %v", peerID, err)
	}

	log.Printf("Peer %s disconnected", peerID)
}

func (s *WebSocketServer) handleMessage(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
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
		return err
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
		log.Printf("No optimal sources found for peer %s: %v", peerID, err)
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
	fmt.Print("handleOffer ctx:", ctx)

	var payload OfferPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	// In real implementation, offer routing to target peer would happen here
	// For simplicity, just echo
	response := map[string]interface{}{
		"type":      "offer_forward",
		"from_peer": peerID,
		"payload":   payload,
	}

	fmt.Print("Response:", response)

	log.Printf("Received offer from peer %s: %s", peerID, payload.SDP[:50]+"...")

	// Target peer should be determined here and offer sent to it
	// targetPeerID := determineTargetPeer(peerID, msg)
	// return s.sendToPeer(targetPeerID, response)

	return nil
}

func (s *WebSocketServer) handleAnswer(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	fmt.Print("handleAnswer ctx:", ctx)

	var payload AnswerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	// In real implementation, answer routing to publisher would happen here
	response := map[string]interface{}{
		"type":      "answer_forward",
		"from_peer": peerID,
		"payload":   payload,
	}

	fmt.Print("Response:", response)

	log.Printf("Received answer from peer %s: %s", peerID, payload.SDP[:50]+"...")

	// Target peer should be determined here and answer sent to it
	// targetPeerID := determineTargetPeer(peerID, msg)
	// return s.sendToPeer(targetPeerID, response)

	return nil
}

func (s *WebSocketServer) handleICECandidate(ctx context.Context, peerID domain.PeerID, msg SignalMessage) error {
	fmt.Print("handleICECandidate ctx:", ctx)

	var payload ICECandidatePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	// Forward ICE candidate to target peer
	response := map[string]interface{}{
		"type":      "ice_candidate_forward",
		"from_peer": peerID,
		"payload":   payload,
	}

	fmt.Print("Response:", response)

	log.Printf("Received ICE candidate from peer %s", peerID)

	// Target peer should be determined here and ICE candidate sent to it
	// targetPeerID := determineTargetPeer(peerID, msg)
	// return s.sendToPeer(targetPeerID, response)

	return nil
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

	log.Printf("Updated metrics for peer %s: bandwidth=%d, packet_loss=%.3f, latency=%dms",
		peerID, payload.Bandwidth, payload.PacketLoss, payload.Latency)

	// Send confirmation
	response := map[string]interface{}{
		"type":      "metrics_updated",
		"timestamp": time.Now().Unix(),
	}

	return s.sendToPeer(peerID, response)
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
