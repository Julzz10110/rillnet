package webrtc

import (
	"sync"

	"rillnet/internal/core/domain"
	"github.com/pion/rtp"
)

// TrackPriority represents the priority of a track
type TrackPriority int

const (
	PriorityAudio TrackPriority = iota // Highest priority - audio tracks
	PriorityVideoKeyframe               // High priority - video keyframes
	PriorityVideoNormal                 // Normal priority - regular video frames
	PriorityVideoLow                    // Low priority - low quality video
)

// TrackPrioritizer manages track prioritization for forwarding
type TrackPrioritizer struct {
	mu sync.RWMutex
	
	// Track priorities
	trackPriorities map[domain.TrackID]TrackPriority
	
	// Audio track IDs (highest priority)
	audioTracks map[domain.TrackID]bool
	
	// Keyframe detection state
	keyframeState map[domain.TrackID]bool
}

// NewTrackPrioritizer creates a new track prioritizer
func NewTrackPrioritizer() *TrackPrioritizer {
	return &TrackPrioritizer{
		trackPriorities: make(map[domain.TrackID]TrackPriority),
		audioTracks:     make(map[domain.TrackID]bool),
		keyframeState:   make(map[domain.TrackID]bool),
	}
}

// RegisterTrack registers a track with its priority
func (tp *TrackPrioritizer) RegisterTrack(trackID domain.TrackID, isAudio bool, quality string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if isAudio {
		tp.trackPriorities[trackID] = PriorityAudio
		tp.audioTracks[trackID] = true
	} else {
		// Video track priority based on quality
		switch quality {
		case "high":
			tp.trackPriorities[trackID] = PriorityVideoNormal
		case "medium":
			tp.trackPriorities[trackID] = PriorityVideoNormal
		case "low":
			tp.trackPriorities[trackID] = PriorityVideoLow
		default:
			tp.trackPriorities[trackID] = PriorityVideoNormal
		}
	}
}

// GetPriority returns the priority of a track
func (tp *TrackPrioritizer) GetPriority(trackID domain.TrackID) TrackPriority {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	priority, exists := tp.trackPriorities[trackID]
	if !exists {
		return PriorityVideoNormal // Default priority
	}

	// Check if this is a keyframe
	if tp.isKeyframe(trackID) {
		return PriorityVideoKeyframe
	}

	return priority
}

// isKeyframe checks if the last packet for a track was a keyframe
func (tp *TrackPrioritizer) isKeyframe(trackID domain.TrackID) bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return tp.keyframeState[trackID]
}

// ProcessPacket processes an RTP packet and updates keyframe state
func (tp *TrackPrioritizer) ProcessPacket(trackID domain.TrackID, packet *rtp.Packet) {
	// Check if this is a keyframe (VP8/VP9/H264)
	isKeyframe := tp.detectKeyframe(packet)

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if isKeyframe {
		tp.keyframeState[trackID] = true
	} else {
		// Reset keyframe state after a few non-keyframe packets
		// This ensures we prioritize keyframes when they arrive
		tp.keyframeState[trackID] = false
	}
}

// detectKeyframe detects if an RTP packet contains a keyframe
func (tp *TrackPrioritizer) detectKeyframe(packet *rtp.Packet) bool {
	if len(packet.Payload) == 0 {
		return false
	}

	// VP8 keyframe detection
	// VP8 payload starts with a 1-byte header
	// Bit 0 (X bit) indicates if extended control bits are present
	// For keyframes, we check the I bit in the extended control bits
	if len(packet.Payload) >= 1 {
		firstByte := packet.Payload[0]
		
		// Check if extended control bits are present (X bit)
		if firstByte&0x80 != 0 && len(packet.Payload) >= 2 {
			// Extended control bits present
			// I bit (bit 4) indicates if this is a keyframe
			extendedByte := packet.Payload[1]
			if extendedByte&0x10 != 0 {
				return true // Keyframe
			}
		}
	}

	// H.264 keyframe detection (NAL unit type 5 = IDR frame)
	if len(packet.Payload) >= 1 {
		nalType := packet.Payload[0] & 0x1F
		if nalType == 5 {
			return true // IDR frame (keyframe)
		}
	}

	return false
}

// ShouldForward determines if a packet should be forwarded based on priority and current load
func (tp *TrackPrioritizer) ShouldForward(trackID domain.TrackID, currentLoad float64, maxLoad float64) bool {
	priority := tp.GetPriority(trackID)

	// Always forward audio
	if priority == PriorityAudio {
		return true
	}

	// Always forward keyframes
	if priority == PriorityVideoKeyframe {
		return true
	}

	// Under low load, forward everything
	if currentLoad < maxLoad*0.7 {
		return true
	}

	// Under medium load, drop low priority video
	if currentLoad < maxLoad*0.9 {
		return priority != PriorityVideoLow
	}

	// Under high load, only forward audio and keyframes
	return priority == PriorityAudio || priority == PriorityVideoKeyframe
}

// GetForwardOrder returns tracks in priority order for forwarding
func (tp *TrackPrioritizer) GetForwardOrder(trackIDs []domain.TrackID) []domain.TrackID {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	// Sort by priority
	priorities := make(map[domain.TrackID]TrackPriority)
	for _, trackID := range trackIDs {
		priorities[trackID] = tp.GetPriority(trackID)
	}

	// Simple insertion sort by priority
	result := make([]domain.TrackID, len(trackIDs))
	copy(result, trackIDs)

	for i := 1; i < len(result); i++ {
		key := result[i]
		j := i - 1

		for j >= 0 && priorities[result[j]] > priorities[key] {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = key
	}

	return result
}

// UnregisterTrack removes a track from prioritization
func (tp *TrackPrioritizer) UnregisterTrack(trackID domain.TrackID) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	delete(tp.trackPriorities, trackID)
	delete(tp.audioTracks, trackID)
	delete(tp.keyframeState, trackID)
}

