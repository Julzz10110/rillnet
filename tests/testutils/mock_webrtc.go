package testutils

import (
	"github.com/pion/webrtc/v3"
)

// MockPeerConnection creates a mock WebRTC peer connection for testing
type MockPeerConnection struct {
	OnTrackHandler func(*webrtc.TrackRemote, *webrtc.RTPReceiver)
}

func (m *MockPeerConnection) AddTrack(track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error) {
	return &webrtc.RTPSender{}, nil
}

func (m *MockPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return nil
}

func (m *MockPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	return nil
}

func (m *MockPeerConnection) CreateOffer(options *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "mock-offer-sdp",
	}, nil
}

func (m *MockPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  "mock-answer-sdp",
	}, nil
}

func (m *MockPeerConnection) OnTrack(handler func(*webrtc.TrackRemote, *webrtc.RTPReceiver)) {
	m.OnTrackHandler = handler
}

func (m *MockPeerConnection) OnICEConnectionStateChange(handler func(webrtc.ICEConnectionState)) {}
func (m *MockPeerConnection) OnConnectionStateChange(handler func(webrtc.PeerConnectionState))   {}
func (m *MockPeerConnection) Close() error                                                       { return nil }

// MockAPICreator creates a mock WebRTC API
type MockAPICreator struct{}

func (m *MockAPICreator) NewPeerConnection(configuration webrtc.Configuration) (*webrtc.PeerConnection, error) {
	// In real implementation, mock peer connection should be returned here
	// For simplicity, return nil as it's difficult to fully mock
	return nil, nil
}
