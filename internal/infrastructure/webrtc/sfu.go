package webrtc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/internal/core/services"
	"rillnet/pkg/circuitbreaker"
	"rillnet/pkg/retry"
	rlog "rillnet/pkg/logger"

	"github.com/pion/interceptor"
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
	NAT1To1IPs []string
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

	// Reliability features
	retryConfig     retry.Config
	circuitBreaker  *circuitbreaker.CircuitBreaker
	peerBreakers    map[domain.PeerID]*circuitbreaker.CircuitBreaker
	peerBreakersMu  sync.RWMutex
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
	retryConfig retry.Config,
	cbConfig circuitbreaker.Config,
) ports.WebRTCService {
	sfu := &SFUService{
		config:          config,
		qualityService:  qualityService,
		metricsService:  metricsService,
		meshService:     meshService,
		publishers:      make(map[domain.PeerID]*Publisher),
		subscribers:     make(map[domain.PeerID]*Subscriber),
		trackForwarders: make(map[domain.TrackID]*TrackForwarder),
		logger:          rlog.New("info").Sugar(),
		retryConfig:     retryConfig,
		circuitBreaker:  circuitbreaker.New(cbConfig),
		peerBreakers:    make(map[domain.PeerID]*circuitbreaker.CircuitBreaker),
	}

	// Set up state change callback
	sfu.circuitBreaker.OnStateChange(func(from, to circuitbreaker.State) {
		sfu.logger.Infow("SFU circuit breaker state changed",
			"from", from.String(),
			"to", to.String(),
		)
	})

	return sfu
}

// getPeerCircuitBreaker gets or creates a circuit breaker for a specific peer
func (s *SFUService) getPeerCircuitBreaker(peerID domain.PeerID) *circuitbreaker.CircuitBreaker {
	s.peerBreakersMu.RLock()
	cb, exists := s.peerBreakers[peerID]
	s.peerBreakersMu.RUnlock()

	if exists {
		return cb
	}

	// Create new circuit breaker for this peer
	s.peerBreakersMu.Lock()
	defer s.peerBreakersMu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := s.peerBreakers[peerID]; exists {
		return cb
	}

	cb = circuitbreaker.New(circuitbreaker.DefaultConfig())
	cb.OnStateChange(func(from, to circuitbreaker.State) {
		s.logger.Infow("peer circuit breaker state changed",
			"peer_id", peerID,
			"from", from.String(),
			"to", to.String(),
		)
	})

	s.peerBreakers[peerID] = cb
	return cb
}

// CreatePublisherOffer creates an offer for publisher
func (s *SFUService) CreatePublisherOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID) (webrtc.SessionDescription, error) {
	if s.retryConfig.Enabled {
		result, err := retry.RetryWithResult(ctx, s.retryConfig, func() (webrtc.SessionDescription, error) {
			res, err := s.circuitBreaker.ExecuteWithResult(ctx, func() (interface{}, error) {
				return s.createPublisherOfferInternal(ctx, peerID, streamID)
			})
			if err != nil {
				return webrtc.SessionDescription{}, err
			}
			return res.(webrtc.SessionDescription), nil
		})
		return result, err
	}

	return s.createPublisherOfferInternal(ctx, peerID, streamID)
}

// createPublisherOfferInternal is the internal implementation without retry/circuit breaker
func (s *SFUService) createPublisherOfferInternal(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID) (webrtc.SessionDescription, error) {
	// If this peer already has a publisher session, tear it down first.
	// Important: close the old PC BEFORE registering the new one in s.publishers,
	// otherwise the old PC's state callbacks could race and delete the new session.
	var oldPC *webrtc.PeerConnection
	var oldStreamID domain.StreamID
	s.mu.Lock()
	if existing, ok := s.publishers[peerID]; ok {
		oldPC = existing.PC
		oldStreamID = existing.StreamID
		delete(s.publishers, peerID)
	}
	s.mu.Unlock()
	if oldPC != nil {
		_ = oldPC.Close()
		s.metricsService.DecrementPublisherCount(oldStreamID)
	}

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

	videoTracks := make(map[string]*webrtc.TrackLocalStaticRTP)
	if s.config.Simulcast {
		for _, quality := range []string{"low", "medium", "high"} {
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
	} else {
		videoTrack, err := webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
			"video",
			"pion-video",
		)
		if err != nil {
			return webrtc.SessionDescription{}, err
		}
		videoTracks["medium"] = videoTrack
	}

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

	s.metricsService.IncrementPublisherCount(streamID)
	return s.finishLocalOffer(pc)
}

// HandlePublisherClientOffer lets the browser send the SDP offer (recommended behind Docker/NAT).
func (s *SFUService) HandlePublisherClientOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if s.retryConfig.Enabled {
		result, err := retry.RetryWithResult(ctx, s.retryConfig, func() (webrtc.SessionDescription, error) {
			res, err := s.circuitBreaker.ExecuteWithResult(ctx, func() (interface{}, error) {
				return s.handlePublisherClientOfferInternal(ctx, peerID, streamID, offer)
			})
			if err != nil {
				return webrtc.SessionDescription{}, err
			}
			return res.(webrtc.SessionDescription), nil
		})
		return result, err
	}
	return s.handlePublisherClientOfferInternal(ctx, peerID, streamID, offer)
}

func (s *SFUService) handlePublisherClientOfferInternal(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if offer.Type == 0 {
		offer.Type = webrtc.SDPTypeOffer
	}

	var oldPC *webrtc.PeerConnection
	var oldStreamID domain.StreamID
	s.mu.Lock()
	if existing, ok := s.publishers[peerID]; ok {
		oldPC = existing.PC
		oldStreamID = existing.StreamID
		delete(s.publishers, peerID)
	}
	s.mu.Unlock()
	if oldPC != nil {
		_ = oldPC.Close()
		s.metricsService.DecrementPublisherCount(oldStreamID)
	}

	pc, err := s.createPeerConnection()
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	pc.OnTrack(s.handlePublisherTrack(peerID, streamID))
	pc.OnICEConnectionStateChange(s.handleICEConnectionState(peerID))
	pc.OnConnectionStateChange(s.handleConnectionState(peerID))

	publisher := &Publisher{
		PeerID:      peerID,
		StreamID:    streamID,
		PC:          pc,
		VideoTracks: make(map[string]*webrtc.TrackLocalStaticRTP),
		Tracks:      make(map[domain.TrackID]*webrtc.TrackLocalStaticRTP),
		CreatedAt:   time.Now(),
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, fmt.Errorf("set publisher offer: %w", err)
	}

	s.mu.Lock()
	s.publishers[peerID] = publisher
	s.mu.Unlock()

	answer, err := s.finishLocalAnswer(pc)
	if err != nil {
		_ = pc.Close()
		s.mu.Lock()
		delete(s.publishers, peerID)
		s.mu.Unlock()
		return webrtc.SessionDescription{}, err
	}

	s.metricsService.IncrementPublisherCount(streamID)
	s.logger.Infow("publisher session started from browser offer",
		"peer_id", peerID,
		"stream_id", streamID,
	)
	return answer, nil
}

// HandlePublisherAnswer handles answer from publisher
func (s *SFUService) HandlePublisherAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error {
	s.mu.RLock()
	publisher, exists := s.publishers[peerID]
	s.mu.RUnlock()

	if !exists {
		return domain.ErrPeerNotFound
	}

	return applyRemoteAnswer(publisher.PC, answer)
}

// CreateSubscriberOffer creates an offer for subscriber
func (s *SFUService) CreateSubscriberOffer(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, sourcePeers []domain.PeerID) (webrtc.SessionDescription, error) {
	if s.retryConfig.Enabled {
		result, err := retry.RetryWithResult(ctx, s.retryConfig, func() (webrtc.SessionDescription, error) {
			// Use per-peer circuit breaker for subscriber connections
			peerCB := s.getPeerCircuitBreaker(peerID)
			res, err := peerCB.ExecuteWithResult(ctx, func() (interface{}, error) {
				return s.createSubscriberOfferInternal(ctx, peerID, streamID, sourcePeers)
			})
			if err != nil {
				return webrtc.SessionDescription{}, err
			}
			return res.(webrtc.SessionDescription), nil
		})
		return result, err
	}

	return s.createSubscriberOfferInternal(ctx, peerID, streamID, sourcePeers)
}

// collectSubscriberTracks resolves source peers and gathers tracks for a subscriber offer.
func (s *SFUService) collectSubscriberTracks(streamID domain.StreamID, sourcePeers []domain.PeerID) ([]*webrtc.TrackLocalStaticRTP, []domain.PeerID) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resolved := append([]domain.PeerID(nil), sourcePeers...)
	if len(resolved) == 0 {
		for id, pub := range s.publishers {
			if pub.StreamID == streamID {
				resolved = append(resolved, id)
			}
		}
	}

	seen := make(map[string]struct{})
	var tracks []*webrtc.TrackLocalStaticRTP
	addTrack := func(t *webrtc.TrackLocalStaticRTP) {
		if t == nil {
			return
		}
		if _, ok := seen[t.ID()]; ok {
			return
		}
		seen[t.ID()] = struct{}{}
		tracks = append(tracks, t)
	}

	matchPublisher := func(pub domain.PeerID) bool {
		if len(resolved) == 0 {
			return true
		}
		for _, src := range resolved {
			if src == pub {
				return true
			}
		}
		return false
	}

	for _, forwarder := range s.trackForwarders {
		if forwarder.StreamID != streamID || forwarder.Track == nil {
			continue
		}
		if matchPublisher(forwarder.Publisher) {
			addTrack(forwarder.Track)
		}
	}

	if len(tracks) == 0 {
		for _, src := range resolved {
			pub, ok := s.publishers[src]
			if !ok || pub.StreamID != streamID {
				continue
			}
			addTrack(pub.AudioTrack)
			for _, vt := range pub.VideoTracks {
				addTrack(vt)
			}
		}
	}

	return tracks, resolved
}

// createSubscriberOfferInternal is the internal implementation without retry/circuit breaker
func (s *SFUService) createSubscriberOfferInternal(ctx context.Context, peerID domain.PeerID, streamID domain.StreamID, sourcePeers []domain.PeerID) (webrtc.SessionDescription, error) {
	tracks, sourcePeers := s.collectSubscriberTracks(streamID, sourcePeers)
	// Stale owner/source_peers from the API must not hide an active SFU publisher on this stream.
	if len(tracks) == 0 && len(sourcePeers) > 0 {
		tracks, sourcePeers = s.collectSubscriberTracks(streamID, nil)
	}
	if len(tracks) == 0 {
		return webrtc.SessionDescription{}, fmt.Errorf("%w: start publishing on this stream first", domain.ErrNoPublisherMedia)
	}

	s.mu.Lock()
	if existing, ok := s.subscribers[peerID]; ok {
		_ = existing.PC.Close()
		delete(s.subscribers, peerID)
	}
	s.mu.Unlock()

	pc, err := s.createPeerConnection()
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	for _, track := range tracks {
		if _, err := pc.AddTrack(track); err != nil {
			s.logger.Warnw("failed to add track to subscriber",
				"peer_id", peerID,
				"track_id", track.ID(),
				"error", err,
			)
			continue
		}

		s.mu.RLock()
		if fwd, exists := s.trackForwarders[domain.TrackID(track.ID())]; exists {
			fwd.Mu.Lock()
			fwd.Subscribers[peerID] = pc
			fwd.Mu.Unlock()
		}
		s.mu.RUnlock()
	}

	// Setup handlers
	pc.OnICEConnectionStateChange(s.handleICEConnectionState(peerID))
	pc.OnConnectionStateChange(s.handleConnectionState(peerID))

	// Determine initial quality based on network conditions
	initialQuality := "medium" // Default quality
	if s.qualityService != nil {
		// Get initial metrics (would come from RTCP in real implementation)
		initialMetrics := domain.NetworkMetrics{
			BandwidthDown:    1000,
			BandwidthUp:      500,
			PacketLoss:       0.02,
			Latency:          150 * time.Millisecond,
			Jitter:           40 * time.Millisecond,
			AvailableBitrate: 800,
		}
		initialQuality = s.qualityService.DetermineOptimalQuality(initialMetrics)
	}

	subscriber := &Subscriber{
		PeerID:      peerID,
		StreamID:    streamID,
		PC:          pc,
		Quality:     initialQuality,
		SourcePeers: sourcePeers,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.subscribers[peerID] = subscriber
	s.mu.Unlock()

	s.metricsService.IncrementSubscriberCount(streamID)
	offer, err := s.finishLocalOffer(pc)
	if err != nil {
		_ = pc.Close()
		s.mu.Lock()
		delete(s.subscribers, peerID)
		s.mu.Unlock()
		return webrtc.SessionDescription{}, err
	}
	return offer, nil
}

// finishLocalAnswer creates an answer and waits for ICE gathering.
func (s *SFUService) finishLocalAnswer(pc *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, err
	}
	s.waitICEGathering(pc)
	if ld := pc.LocalDescription(); ld != nil {
		return *ld, nil
	}
	return answer, nil
}

// finishLocalOffer creates an offer and waits for ICE gathering so the SDP includes host candidates.
func (s *SFUService) finishLocalOffer(pc *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}
	s.waitICEGathering(pc)
	if ld := pc.LocalDescription(); ld != nil {
		return *ld, nil
	}
	return offer, nil
}

func (s *SFUService) waitICEGathering(pc *webrtc.PeerConnection) {
	gatherDone := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherDone:
	case <-time.After(10 * time.Second):
		s.logger.Warnw("ICE gathering timed out, returning partial local description")
	}
}

// HandleSubscriberAnswer handles answer from subscriber
func (s *SFUService) HandleSubscriberAnswer(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error {
	if s.retryConfig.Enabled {
		return retry.Retry(ctx, s.retryConfig, func() error {
			peerCB := s.getPeerCircuitBreaker(peerID)
			return peerCB.Execute(ctx, func() error {
				return s.handleSubscriberAnswerInternal(ctx, peerID, answer)
			})
		})
	}

	return s.handleSubscriberAnswerInternal(ctx, peerID, answer)
}

// handleSubscriberAnswerInternal is the internal implementation without retry/circuit breaker
func (s *SFUService) handleSubscriberAnswerInternal(ctx context.Context, peerID domain.PeerID, answer webrtc.SessionDescription) error {
	s.mu.RLock()
	subscriber, exists := s.subscribers[peerID]
	s.mu.RUnlock()

	if !exists {
		return domain.ErrPeerNotFound
	}

	return applyRemoteAnswer(subscriber.PC, answer)
}

// applyRemoteAnswer sets the browser's answer on an SFU peer that created the offer.
func applyRemoteAnswer(pc *webrtc.PeerConnection, answer webrtc.SessionDescription) error {
	if answer.Type == 0 {
		answer.Type = webrtc.SDPTypeAnswer
	}

	switch pc.SignalingState() {
	case webrtc.SignalingStateStable:
		// Duplicate answer POST (double Join, retries) — already negotiated.
		return nil
	case webrtc.SignalingStateHaveLocalOffer:
		return pc.SetRemoteDescription(answer)
	default:
		return fmt.Errorf("unexpected signaling state %s when applying answer", pc.SignalingState())
	}
}

// createPeerConnection creates a new WebRTC connection
func (s *SFUService) createPeerConnection() (*webrtc.PeerConnection, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("register default codecs: %w", err)
	}

	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		return nil, fmt.Errorf("register default interceptors: %w", err)
	}

	config := webrtc.Configuration{
		ICEServers:   s.config.ICEServers,
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	}

	settingEngine := webrtc.SettingEngine{}
	if s.config.PortRange.Min > 0 && s.config.PortRange.Max > 0 {
		_ = settingEngine.SetEphemeralUDPPortRange(s.config.PortRange.Min, s.config.PortRange.Max)
	}
	if len(s.config.NAT1To1IPs) > 0 {
		settingEngine.SetNAT1To1IPs(s.config.NAT1To1IPs, webrtc.ICECandidateTypeHost)
	}
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeTCP4,
	})

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
		webrtc.WithSettingEngine(settingEngine),
	)
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

// Global packet buffer pool to reduce allocations
var packetBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 1500) // MTU size
		return &b
	},
}

// forwardTrackToSubscribers forwards track to all subscribers
func (s *SFUService) forwardTrackToSubscribers(forwarder *TrackForwarder, track *webrtc.TrackRemote) {
	packetBufferPtr := packetBufferPool.Get().(*[]byte)
	packetBuffer := *packetBufferPtr
	defer packetBufferPool.Put(packetBufferPtr)

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

		switch state {
		case webrtc.ICEConnectionStateFailed:
			s.logger.Warnw("peer ICE failed (session kept for retry)", "peer_id", peerID)
		case webrtc.ICEConnectionStateClosed:
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

		switch state {
		case webrtc.PeerConnectionStateFailed:
			s.logger.Warnw("peer connection failed (session kept; ICE may recover)", "peer_id", peerID)
		case webrtc.PeerConnectionStateClosed:
			s.handlePeerDisconnect(peerID)
		}
	}
}

// HasActiveMedia reports whether real publisher media is being forwarded (not placeholder tracks).
func (s *SFUService) HasActiveMedia(_ context.Context, streamID domain.StreamID) bool {
	return s.GetStreamWebRTCStatus(context.Background(), streamID).MediaReady
}

// GetStreamWebRTCStatus returns SFU WebRTC state for a stream (single ingest process).
func (s *SFUService) GetStreamWebRTCStatus(_ context.Context, streamID domain.StreamID) ports.StreamWebRTCStatus {
	status := ports.StreamWebRTCStatus{}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, pub := range s.publishers {
		if pub.StreamID != streamID {
			continue
		}
		status.PublisherRegistered = true
		if pub.PC != nil {
			status.PublisherICEState = pub.PC.ICEConnectionState().String()
			status.PublisherConnState = pub.PC.ConnectionState().String()
		}
		break
	}

	for _, fwd := range s.trackForwarders {
		if fwd.StreamID == streamID && fwd.Track != nil {
			status.ForwarderTracks++
		}
	}
	status.MediaReady = status.ForwarderTracks > 0
	return status
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
			// NACK indicates packet loss, increment loss counter (saturate at 255)
			for range min(len(p.Nacks), 255-int(totalPacketLoss)) {
				totalPacketLoss++
			}
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
			_ = publisher.PC.Close()
		}
		delete(s.publishers, peerID)
		s.metricsService.DecrementPublisherCount(publisher.StreamID)
	}

	// Clean up subscriber
	if subscriber, exists := s.subscribers[peerID]; exists {
		if subscriber.PC != nil {
			_ = subscriber.PC.Close()
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
					_ = subPC.Close()
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

// SwitchSubscriberQuality switches the quality layer for a subscriber (simulcast)
func (s *SFUService) SwitchSubscriberQuality(ctx context.Context, peerID domain.PeerID, quality string) error {
	s.mu.RLock()
	subscriber, exists := s.subscribers[peerID]
	s.mu.RUnlock()

	if !exists {
		return domain.ErrPeerNotFound
	}

	// Validate quality
	validQualities := map[string]bool{"low": true, "medium": true, "high": true}
	if !validQualities[quality] {
		return fmt.Errorf("invalid quality: %s", quality)
	}

	// Update subscriber quality
	s.mu.Lock()
	subscriber.Quality = quality
	s.mu.Unlock()

	s.logger.Infow("switched subscriber quality",
		"peer_id", peerID,
		"quality", quality,
	)

	// In a full implementation, this would:
	// 1. Get the RTPSender for the video track
	// 2. Use SetRTPParameters to switch simulcast layers
	// 3. Or use SetTrack to replace the track with the desired quality layer
	// 
	// For now, we just update the quality field. The actual simulcast switching
	// would require access to RTPSender and RTPParameters, which needs to be
	// stored when tracks are added.

	return nil
}
