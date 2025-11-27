package services

import (
	"sync"
	"time"

	"rillnet/internal/core/domain"
)

type MetricsService struct {
	mu sync.RWMutex

	// Stream metrics
	streamMetrics map[domain.StreamID]*domain.StreamMetrics

	// Counters
	publisherCount  map[domain.StreamID]int
	subscriberCount map[domain.StreamID]int
	totalBitrate    map[domain.StreamID]int
	connectionCount map[domain.StreamID]int
	averageLatency  map[domain.StreamID]time.Duration
}

func NewMetricsService() *MetricsService {
	return &MetricsService{
		streamMetrics:   make(map[domain.StreamID]*domain.StreamMetrics),
		publisherCount:  make(map[domain.StreamID]int),
		subscriberCount: make(map[domain.StreamID]int),
		totalBitrate:    make(map[domain.StreamID]int),
		connectionCount: make(map[domain.StreamID]int),
		averageLatency:  make(map[domain.StreamID]time.Duration),
	}
}

func (m *MetricsService) IncrementPublisherCount(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publisherCount[streamID]++
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) DecrementPublisherCount(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publisherCount[streamID] > 0 {
		m.publisherCount[streamID]--
	}
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) IncrementSubscriberCount(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriberCount[streamID]++
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) DecrementSubscriberCount(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscriberCount[streamID] > 0 {
		m.subscriberCount[streamID]--
	}
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) UpdateBitrate(streamID domain.StreamID, bitrate int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalBitrate[streamID] = bitrate
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) UpdateLatency(streamID domain.StreamID, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.averageLatency[streamID] = latency
	m.updateStreamMetrics(streamID)
}

func (m *MetricsService) GetStreamMetrics(streamID domain.StreamID) *domain.StreamMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.streamMetrics[streamID]; exists {
		return metrics
	}

	// Return default metrics if stream not found
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

func (m *MetricsService) updateStreamMetrics(streamID domain.StreamID) {
	publishers := m.publisherCount[streamID]
	subscribers := m.subscriberCount[streamID]
	bitrate := m.totalBitrate[streamID]
	latency := m.averageLatency[streamID]

	m.streamMetrics[streamID] = &domain.StreamMetrics{
		StreamID:          streamID,
		ActivePublishers:  publishers,
		ActiveSubscribers: subscribers,
		TotalBitrate:      bitrate,
		AverageLatency:    latency,
		HealthScore:       m.calculateHealthScore(publishers, subscribers, bitrate, latency),
		Timestamp:         time.Now(),
	}
}

func (m *MetricsService) calculateHealthScore(publishers, subscribers, bitrate int, latency time.Duration) float64 {
	// Simplified health score calculation
	publisherScore := float64(publishers) * 20.0
	subscriberScore := float64(subscribers) * 2.0
	bitrateScore := float64(bitrate) / 100.0

	latencyScore := 0.0
	if latency < 100*time.Millisecond {
		latencyScore = 30.0
	} else if latency < 300*time.Millisecond {
		latencyScore = 20.0
	} else if latency < 500*time.Millisecond {
		latencyScore = 10.0
	}

	totalScore := publisherScore + subscriberScore + bitrateScore + latencyScore
	if totalScore > 100.0 {
		return 100.0
	}
	return totalScore
}

// Additional methods for updating metrics
func (m *MetricsService) RecordConnection(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionCount[streamID]++
}

func (m *MetricsService) RemoveConnection(streamID domain.StreamID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectionCount[streamID] > 0 {
		m.connectionCount[streamID]--
	}
}

func (m *MetricsService) GetConnectionCount(streamID domain.StreamID) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionCount[streamID]
}
