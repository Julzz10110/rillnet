package webrtc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	rlog "rillnet/pkg/logger"

	"github.com/pion/webrtc/v3"
	"go.uber.org/zap"
)

// WebRTCConfig WebRTC configuration
type WebRTCConfig struct {
	ICEServers []webrtc.ICEServer
	PortRange  struct {
		Min uint16
		Max uint16
	}
	Simulcast  bool
	MaxBitrate int
}

// SFUService SFU implementation
type SFUService struct {
	config         WebRTCConfig
	qualityService *services.QualityService
	metricsService *services.MetricsService
	meshService    ports.MeshService

	publishers      map[domain.PeerID]*Publisher
	subscribers     map[domain.PeerID]*Subscriber
	trackForwarders map[domain.TrackID]*TrackForwarder
	mu              sync.RWMutex

	logger *zap.SugaredLogger
}

// Publisher represents a stream publisher
type Publisher struct {
	PeerID      domain.PeerID
	StreamID    domain.StreamID
	PC          *webrtc.PeerConnection
	Tracks      map[domain.TrackID]*webrtc.TrackLocalStaticRTP
	AudioTrack  *webrtc.TrackLocalStaticRTP
	VideoTracks map[string]*webrtc.TrackLocalStaticRTP
	CreatedAt   time.Time
}

// Subscriber represents a stream subscriber
type Subscriber struct {
	PeerID      domain.PeerID
	StreamID    domain.StreamID
	PC          *webrtc.PeerConnection
	Quality     string
	SourcePeers []domain.PeerID
	CreatedAt   time.Time
}

// TrackForwarder manages track forwarding
type TrackForwarder struct {
	TrackID     domain.TrackID
	Publisher   domain.PeerID
	StreamID    domain.StreamID
	Track       *webrtc.TrackLocalStaticRTP
	Subscribers map[domain.PeerID]*webrtc.PeerConnection
	Mu          sync.RWMutex
}

// NewSFUService creates a new SFU service
func NewSFUService(
	config WebRTCConfig,
	qualityService *services.QualityService,
	metricsService *services.MetricsService,
	meshService ports.MeshService,
) ports.WebRTCService {
	return &SFUService{
		config:          config,
		qualityService:  qualityService,
		metricsService:  metricsService,
		meshService:     meshService,
		publishers:      make(map[domain.PeerID]*Publisher),
		subscribers:     make(map[domain.PeerID]*Subscriber),
		trackForwarders: make(map[domain.TrackID]*TrackForwarder),
		logger:          rlog.New("info").Sugar(),
	}
}

// CreatePublisherOffer creates an offer for publisher
func (s *SFUService) CreatePublisherOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID) (webrtc.SessionDescription, error) {
	pc, err := s.createPeerConnection()
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create tracks for publishing
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"pion-audio",
	)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Create video tracks for simulcast
	videoTracks := make(map[string]*webrtc.TrackLocalStaticRTP)
	qualities := []string{"low", "medium", "high"}

	for _, quality := range qualities {
		videoTrack, err := webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
			fmt.Sprintf("video-%s", quality),
			fmt.Sprintf("pion-video-%s", quality),
		)
		if err != nil {
			return webrtc.SessionDescription{}, err
		}
		videoTracks[quality] = videoTrack
	}

	// Add tracks to peer connection
	if _, err := pc.AddTrack(audioTrack); err != nil {
		return webrtc.SessionDescription{}, err
	}

	for _, track := range videoTracks {
		if _, err := pc.AddTrack(track); err != nil {
			return webrtc.SessionDescription{}, err
		}
	}

	// Handle incoming data
	pc.OnTrack(s.handlePublisherTrack(peerID, streamID))
	pc.OnICEConnectionStateChange(s.handleICEConnectionState(peerID))
	pc.OnConnectionStateChange(s.handleConnectionState(peerID))

	publisher := &Publisher{
		PeerID:      peerID,
		StreamID:    streamID,
		PC:          pc,
		AudioTrack:  audioTrack,
		VideoTracks: videoTracks,
		Tracks:      make(map[domain.TrackID]*webrtc.TrackLocalStaticRTP),
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.publishers[peerID] = publisher
	s.mu.Unlock()

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}

	s.metricsService.IncrementPublisherCount(streamID)
	return offer, nil
}

// HandlePublisherAnswer handles answer from publisher
func (s *SFUService) HandlePublisherAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error {
	s.mu.RLock()
	publisher, exists := s.publishers[peerID]
	s.mu.RUnlock()

	if !exists {
		return domain.ErrPeerNotFound
	}

	return publisher.PC.SetRemoteDescription(answer)
}

// CreateSubscriberOffer creates an offer for subscriber
func (s *SFUService) CreateSubscriberOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, sourcePeers []domain.PeerID) (webrtc.SessionDescription, error) {
	pc, err := s.createPeerConnection()
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Find tracks for subscription
	var tracks []*webrtc.TrackLocalStaticRTP
	for _, sourcePeerID := range sourcePeers {
		s.mu.RLock()
		publisher, exists := s.publishers[sourcePeerID]
		s.mu.RUnlock()

		if exists {
			for _, track := range publisher.VideoTracks {
				tracks = append(tracks, track)
			}
			if publisher.AudioTrack != nil {
				tracks = append(tracks, publisher.AudioTrack)
			}
		}
	}

	// Add tracks to peer connection
	for _, track := range tracks {
		if _, err := pc.AddTrack(track); err != nil {
			return webrtc.SessionDescription{}, err
		}
	}

	// Setup handlers
	pc.OnICEConnectionStateChange(s.handleICEConnectionState(peerID))
	pc.OnConnectionStateChange(s.handleConnectionState(peerID))

	subscriber := &Subscriber{
		PeerID:      peerID,
		StreamID:    streamID,
		PC:          pc,
		SourcePeers: sourcePeers,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.subscribers[peerID] = subscriber
	s.mu.Unlock()

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}

	s.metricsService.IncrementSubscriberCount(streamID)
	return offer, nil
}

// HandleSubscriberAnswer handles answer from subscriber
func (s *SFUService) HandleSubscriberAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error {
	s.mu.RLock()
	subscriber, exists := s.subscribers[peerID]
	s.mu.RUnlock()

	if !exists {
		return domain.ErrPeerNotFound
	}

	return subscriber.PC.SetRemoteDescription(answer)
}

// createPeerConnection creates a new WebRTC connection
func (s *SFUService) createPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers:   s.config.ICEServers,
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	}

	settingEngine := webrtc.SettingEngine{}
	if s.config.PortRange.Min > 0 && s.config.PortRange.Max > 0 {
		settingEngine.SetEphemeralUDPPortRange(s.config.PortRange.Min, s.config.PortRange.Max)
	}

	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))
	return api.NewPeerConnection(config)
}

// handlePublisherTrack handles incoming tracks from publisher
func (s *SFUService) handlePublisherTrack(peerID domain.PeerID, streamID domain.StreamID) func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
	return func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		s.logger.Infow("publisher started streaming track",
			"peer_id", peerID,
			"stream_id", streamID,
			"track_id", track.ID(),
		)

		// Create forwarder for this track
		forwarder := &TrackForwarder{
			TrackID:     domain.TrackID(track.ID()),
			Publisher:   peerID,
			StreamID:    streamID,
			Subscribers: make(map[domain.PeerID]*webrtc.PeerConnection),
		}

		s.mu.Lock()
		s.trackForwarders[domain.TrackID(track.ID())] = forwarder
		s.mu.Unlock()

		// Start forwarding packets to subscribers
		go s.forwardTrackToSubscribers(forwarder, track)
	}
}

// forwardTrackToSubscribers forwards track to all subscribers
func (s *SFUService) forwardTrackToSubscribers(forwarder *TrackForwarder, track *webrtc.TrackRemote) {
	packetBuffer := make([]byte, 1500) // MTU size

	for {
		// Read RTP packet from publisher
		_, _, err := track.Read(packetBuffer)
		if err != nil {
			s.logger.Warnw("error reading track",
				"track_id", forwarder.TrackID,
				"error", err,
			)
			break
		}

		// In real implementation, packet sending logic to subscribers would be here
		// This is a simplified version for demonstration
		forwarder.Mu.RLock()
		subscriberCount := len(forwarder.Subscribers)
		forwarder.Mu.RUnlock()

		if subscriberCount > 0 {
			// Packet forwarding logic
		}
	}
}

// handleICEConnectionState handles ICE connection state changes
func (s *SFUService) handleICEConnectionState(peerID domain.PeerID) func(webrtc.ICEConnectionState) {
	return func(state webrtc.ICEConnectionState) {
		s.logger.Infow("peer ICE connection state changed",
			"peer_id", peerID,
			"ice_state", state,
		)

		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			s.handlePeerDisconnect(peerID)
		}
	}
}

// handleConnectionState handles connection state changes
func (s *SFUService) handleConnectionState(peerID domain.PeerID) func(webrtc.PeerConnectionState) {
	return func(state webrtc.PeerConnectionState) {
		s.logger.Infow("peer connection state changed",
			"peer_id", peerID,
			"connection_state", state,
		)

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			s.handlePeerDisconnect(peerID)
		}
	}
}

// handlePeerDisconnect handles peer disconnection
func (s *SFUService) handlePeerDisconnect(peerID domain.PeerID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up publisher
	if publisher, exists := s.publishers[peerID]; exists {
		if publisher.PC != nil {
			publisher.PC.Close()
		}
		delete(s.publishers, peerID)
		s.metricsService.DecrementPublisherCount(publisher.StreamID)
	}

	// Clean up subscriber
	if subscriber, exists := s.subscribers[peerID]; exists {
		if subscriber.PC != nil {
			subscriber.PC.Close()
		}
		delete(s.subscribers, peerID)
		s.metricsService.DecrementSubscriberCount(subscriber.StreamID)
	}

	// Clean up forwarders
	for trackID, forwarder := range s.trackForwarders {
		if forwarder.Publisher == peerID {
			forwarder.Mu.Lock()
			for subPeerID := range forwarder.Subscribers {
				delete(forwarder.Subscribers, subPeerID)
			}
			forwarder.Mu.Unlock()
			delete(s.trackForwarders, trackID)
		}
	}
}

// GetPublisher returns publisher by ID
func (s *SFUService) GetPublisher(peerID domain.PeerID) (*Publisher, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	publisher, exists := s.publishers[peerID]
	return publisher, exists
}

// GetSubscriber returns subscriber by ID
func (s *SFUService) GetSubscriber(peerID domain.PeerID) (*Subscriber, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	subscriber, exists := s.subscribers[peerID]
	return subscriber, exists
}
