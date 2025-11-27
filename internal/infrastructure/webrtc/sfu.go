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

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
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

	// Find tracks for subscription from forwarders (these are the actual forwarded tracks)
	var tracks []*webrtc.TrackLocalStaticRTP
	s.mu.RLock()
	for _, forwarder := range s.trackForwarders {
		// Only include tracks from the specified source peers
		for _, sourcePeerID := range sourcePeers {
			if forwarder.Publisher == sourcePeerID && forwarder.Track != nil {
				tracks = append(tracks, forwarder.Track)
			}
		}
	}
	s.mu.RUnlock()

	// Add tracks to peer connection
	for _, track := range tracks {
		if _, err := pc.AddTrack(track); err != nil {
			s.logger.Warnw("failed to add track to subscriber",
				"peer_id", peerID,
				"track_id", track.ID(),
				"error", err,
			)
			continue
		}

		// Register subscriber's peer connection in forwarder
		s.mu.Lock()
		if forwarder, exists := s.trackForwarders[domain.TrackID(track.ID())]; exists {
			forwarder.Mu.Lock()
			forwarder.Subscribers[peerID] = pc
			forwarder.Mu.Unlock()
		}
		s.mu.Unlock()
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
			"codec", track.Codec().MimeType,
		)

		// Create local track for forwarding to subscribers
		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			track.Codec().RTPCodecCapability,
			track.ID(),
			track.StreamID(),
		)
		if err != nil {
			s.logger.Errorw("failed to create local track for forwarding",
				"peer_id", peerID,
				"track_id", track.ID(),
				"error", err,
			)
			return
		}

		// Create forwarder for this track
		forwarder := &TrackForwarder{
			TrackID:     domain.TrackID(track.ID()),
			Publisher:   peerID,
			StreamID:    streamID,
			Track:       localTrack,
			Subscribers: make(map[domain.PeerID]*webrtc.PeerConnection),
		}

		s.mu.Lock()
		s.trackForwarders[domain.TrackID(track.ID())] = forwarder
		s.mu.Unlock()

		// Start RTCP processing for this receiver
		go s.processRTCP(peerID, streamID, receiver, true) // true = publisher

		// Start forwarding packets to subscribers
		go s.forwardTrackToSubscribers(forwarder, track)
	}
}

// forwardTrackToSubscribers forwards track to all subscribers
func (s *SFUService) forwardTrackToSubscribers(forwarder *TrackForwarder, track *webrtc.TrackRemote) {
	packetBuffer := make([]byte, 1500) // MTU size
	rtpPacket := &rtp.Packet{}
	packetCount := uint16(0)

	for {
		// Read RTP packet from publisher
		n, _, err := track.Read(packetBuffer)
		if err != nil {
			s.logger.Warnw("error reading track",
				"track_id", forwarder.TrackID,
				"publisher", forwarder.Publisher,
				"error", err,
			)
			break
		}

		// Parse RTP packet
		if err := rtpPacket.Unmarshal(packetBuffer[:n]); err != nil {
			s.logger.Warnw("error unmarshaling RTP packet",
				"track_id", forwarder.TrackID,
				"error", err,
			)
			continue
		}

		// Write packet to local track, which will forward to all subscribers
		if forwarder.Track != nil {
			if err := forwarder.Track.WriteRTP(rtpPacket); err != nil {
				s.logger.Warnw("error writing RTP packet to local track",
					"track_id", forwarder.TrackID,
					"error", err,
				)
				// Continue processing even if one write fails
			}
		}

		packetCount++

		// Log forwarding stats periodically
		forwarder.Mu.RLock()
		subscriberCount := len(forwarder.Subscribers)
		forwarder.Mu.RUnlock()

		// Update metrics periodically (every 100 packets or so)
		if packetCount%100 == 0 && subscriberCount > 0 {
			s.logger.Debugw("forwarding RTP packet",
				"track_id", forwarder.TrackID,
				"subscribers", subscriberCount,
				"sequence", rtpPacket.SequenceNumber,
				"packets_forwarded", packetCount,
			)
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

// processRTCP processes RTCP packets from RTPReceiver to extract quality metrics
func (s *SFUService) processRTCP(peerID domain.PeerID, streamID domain.StreamID, receiver *webrtc.RTPReceiver, isPublisher bool) {
	// Read RTCP packets from receiver
	for {
		packets, _, err := receiver.ReadRTCP()
		if err != nil {
			s.logger.Warnw("error reading RTCP packets",
				"peer_id", peerID,
				"stream_id", streamID,
				"error", err,
			)
			break
		}

		s.processRTCPPackets(peerID, streamID, packets, isPublisher)
	}
}

// processRTCPPackets processes RTCP packets to extract quality metrics
func (s *SFUService) processRTCPPackets(peerID domain.PeerID, streamID domain.StreamID, packets []rtcp.Packet, isPublisher bool) {
	var totalPacketLoss uint8
	var totalJitter uint32
	var totalLatency time.Duration
	packetCount := 0

	for _, packet := range packets {
		switch p := packet.(type) {
		case *rtcp.ReceiverReport:
			// Receiver Report contains quality metrics from receiver perspective
			for _, report := range p.Reports {
				totalPacketLoss += report.FractionLost
				totalJitter += report.Jitter
				packetCount++

				// Calculate RTT if available (LSR and DLSR are present)
				if report.LastSenderReport != 0 && report.Delay != 0 {
					// RTT estimation (simplified)
					rtt := time.Duration(report.Delay) * time.Second / 65536
					totalLatency += rtt
				}
			}

		case *rtcp.SenderReport:
			// Sender Report contains transmission statistics
			// Can be used to calculate bandwidth
			s.logger.Debugw("received sender report",
				"peer_id", peerID,
				"stream_id", streamID,
				"packet_count", p.PacketCount,
				"octet_count", p.OctetCount,
			)

		case *rtcp.TransportLayerNack:
			// NACK indicates packet loss
			s.logger.Debugw("received NACK",
				"peer_id", peerID,
				"stream_id", streamID,
				"nacks", len(p.Nacks),
			)
			// NACK indicates packet loss, increment loss counter
			totalPacketLoss += uint8(len(p.Nacks))
			packetCount++

		case *rtcp.PictureLossIndication:
			// PLI indicates picture loss (keyframe request)
			s.logger.Debugw("received PLI",
				"peer_id", peerID,
				"stream_id", streamID,
			)
		}
	}

	// Calculate average metrics if we have reports
	if packetCount > 0 {
		avgPacketLoss := float64(totalPacketLoss) / float64(packetCount) / 255.0 // Normalize to 0-1
		avgJitter := time.Duration(totalJitter/uint32(packetCount)) * time.Millisecond
		avgLatency := totalLatency / time.Duration(packetCount)

		// Update peer metrics
		metrics := domain.NetworkMetrics{
			Timestamp:        time.Now(),
			PacketLoss:       avgPacketLoss,
			Jitter:           avgJitter,
			Latency:          avgLatency,
			BandwidthDown:    0, // Will be calculated from other sources
			BandwidthUp:      0, // Will be calculated from other sources
			AvailableBitrate: 0, // Will be calculated from other sources
		}

		// Update metrics through mesh service
		ctx := context.Background()
		if err := s.meshService.UpdatePeerMetrics(ctx, peerID, metrics); err != nil {
			s.logger.Warnw("failed to update peer metrics from RTCP",
				"peer_id", peerID,
				"stream_id", streamID,
				"error", err,
			)
		} else {
			s.logger.Debugw("updated peer metrics from RTCP",
				"peer_id", peerID,
				"stream_id", streamID,
				"packet_loss", avgPacketLoss,
				"jitter", avgJitter,
				"latency", avgLatency,
				"is_publisher", isPublisher,
			)
		}

		// Update stream latency metrics
		s.metricsService.UpdateLatency(streamID, avgLatency)
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

		// Remove subscriber from all forwarders
		for _, forwarder := range s.trackForwarders {
			forwarder.Mu.Lock()
			delete(forwarder.Subscribers, peerID)
			forwarder.Mu.Unlock()
		}
	}

	// Clean up forwarders when publisher disconnects
	for trackID, forwarder := range s.trackForwarders {
		if forwarder.Publisher == peerID {
			forwarder.Mu.Lock()
			// Close all subscriber connections for this forwarder
			for subPeerID, subPC := range forwarder.Subscribers {
				if subPC != nil {
					subPC.Close()
				}
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
