package monitoring

import (
	"time"

	"rillnet/internal/core/domain"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusCollector struct {
	// Counters
	peersConnectedTotal prometheus.Gauge
	streamsActiveTotal  prometheus.Gauge
	dataExchangedBytes  prometheus.Counter
	connectionsTotal    prometheus.Counter

	// Histograms
	webrtcConnectionDuration prometheus.Histogram
	videoSegmentDuration     prometheus.Histogram
	networkLatency           prometheus.Histogram

	// Stream metrics
	streamBitrate     *prometheus.GaugeVec
	streamPeerCount   *prometheus.GaugeVec
	streamHealthScore *prometheus.GaugeVec

	// Business metrics
	streamViewerCount      *prometheus.GaugeVec
	streamWatchDuration    *prometheus.HistogramVec
	p2pEfficiencyPercent   *prometheus.GaugeVec
	p2pDataTransferred     prometheus.Counter
	serverDataTransferred  prometheus.Counter
}

func NewPrometheusCollector() *PrometheusCollector {
	return &PrometheusCollector{
		peersConnectedTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rillnet_peers_connected_total",
			Help: "Total number of connected peers",
		}),

		streamsActiveTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rillnet_streams_active_total",
			Help: "Total number of active streams",
		}),

		dataExchangedBytes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rillnet_data_exchanged_bytes_total",
			Help: "Total amount of data exchanged in bytes",
		}),

		connectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rillnet_connections_total",
			Help: "Total number of WebRTC connections established",
		}),

		webrtcConnectionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rillnet_webrtc_connection_duration_seconds",
			Help:    "Duration of WebRTC connections",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}),

		videoSegmentDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rillnet_video_segment_download_duration_seconds",
			Help:    "Duration of video segment downloads",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		}),

		networkLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rillnet_network_latency_seconds",
			Help:    "Network latency between peers",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}),

		streamBitrate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rillnet_stream_bitrate_bps",
			Help: "Current bitrate of streams in bits per second",
		}, []string{"stream_id", "quality"}),

		streamPeerCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rillnet_stream_peer_count",
			Help: "Number of peers in each stream",
		}, []string{"stream_id", "peer_type"}),

		streamHealthScore: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rillnet_stream_health_score",
			Help: "Health score of streams (0-100)",
		}, []string{"stream_id"}),

		// Business metrics
		streamViewerCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rillnet_stream_viewer_count",
			Help: "Number of viewers (subscribers) per stream",
		}, []string{"stream_id"}),

		streamWatchDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "rillnet_stream_watch_duration_seconds",
			Help:    "Duration of stream viewing sessions",
			Buckets: []float64{60, 300, 600, 1800, 3600, 7200, 14400}, // 1min, 5min, 10min, 30min, 1h, 2h, 4h
		}, []string{"stream_id"}),

		p2pEfficiencyPercent: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rillnet_p2p_efficiency_percent",
			Help: "Percentage of traffic served through P2P (0-100)",
		}, []string{"stream_id"}),

		p2pDataTransferred: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rillnet_p2p_data_transferred_bytes_total",
			Help: "Total amount of data transferred through P2P connections in bytes",
		}),

		serverDataTransferred: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rillnet_server_data_transferred_bytes_total",
			Help: "Total amount of data transferred directly from server in bytes",
		}),
	}
}

func (p *PrometheusCollector) RecordPeerConnected(streamID domain.StreamID, isPublisher bool) {
	p.peersConnectedTotal.Inc()

	peerType := "subscriber"
	if isPublisher {
		peerType = "publisher"
	}

	p.streamPeerCount.WithLabelValues(string(streamID), peerType).Inc()
}

func (p *PrometheusCollector) RecordPeerDisconnected(streamID domain.StreamID, isPublisher bool) {
	p.peersConnectedTotal.Dec()

	peerType := "subscriber"
	if isPublisher {
		peerType = "publisher"
	}

	p.streamPeerCount.WithLabelValues(string(streamID), peerType).Dec()
}

func (p *PrometheusCollector) RecordStreamCreated(streamID domain.StreamID) {
	p.streamsActiveTotal.Inc()
}

func (p *PrometheusCollector) RecordStreamEnded(streamID domain.StreamID) {
	p.streamsActiveTotal.Dec()

	// Очищаем метрики для этого стрима
	p.streamBitrate.DeleteLabelValues(string(streamID), "high")
	p.streamBitrate.DeleteLabelValues(string(streamID), "medium")
	p.streamBitrate.DeleteLabelValues(string(streamID), "low")
	p.streamPeerCount.DeleteLabelValues(string(streamID), "publisher")
	p.streamPeerCount.DeleteLabelValues(string(streamID), "subscriber")
	p.streamHealthScore.DeleteLabelValues(string(streamID))
}

func (p *PrometheusCollector) RecordDataTransferred(bytes int64) {
	p.dataExchangedBytes.Add(float64(bytes))
}

func (p *PrometheusCollector) RecordWebRTCConnection(duration time.Duration) {
	p.webrtcConnectionDuration.Observe(duration.Seconds())
	p.connectionsTotal.Inc()
}

func (p *PrometheusCollector) RecordVideoSegmentDownload(duration time.Duration) {
	p.videoSegmentDuration.Observe(duration.Seconds())
}

func (p *PrometheusCollector) RecordNetworkLatency(latency time.Duration) {
	p.networkLatency.Observe(latency.Seconds())
}

func (p *PrometheusCollector) UpdateStreamMetrics(metrics *domain.StreamMetrics) {
	p.streamHealthScore.WithLabelValues(string(metrics.StreamID)).Set(metrics.HealthScore)

	// Update viewer count (subscribers)
	p.streamViewerCount.WithLabelValues(string(metrics.StreamID)).Set(float64(metrics.ActiveSubscribers))

	// Bitrate update by quality can be added here
	// Based on real data from peers
}

// RecordViewerSession records a viewer session duration
func (p *PrometheusCollector) RecordViewerSession(streamID domain.StreamID, duration time.Duration) {
	p.streamWatchDuration.WithLabelValues(string(streamID)).Observe(duration.Seconds())
}

// RecordP2PDataTransferred records data transferred through P2P connections
func (p *PrometheusCollector) RecordP2PDataTransferred(bytes int64) {
	p.p2pDataTransferred.Add(float64(bytes))
}

// RecordServerDataTransferred records data transferred directly from server
func (p *PrometheusCollector) RecordServerDataTransferred(bytes int64) {
	p.serverDataTransferred.Add(float64(bytes))
}

// UpdateP2PEfficiency updates P2P efficiency percentage for a stream
// efficiency should be between 0 and 100
func (p *PrometheusCollector) UpdateP2PEfficiency(streamID domain.StreamID, efficiency float64) {
	if efficiency < 0 {
		efficiency = 0
	}
	if efficiency > 100 {
		efficiency = 100
	}
	p.p2pEfficiencyPercent.WithLabelValues(string(streamID)).Set(efficiency)
}

// CalculateAndUpdateP2PEfficiency calculates P2P efficiency based on transferred data
// and updates the metric for a stream
func (p *PrometheusCollector) CalculateAndUpdateP2PEfficiency(streamID domain.StreamID, p2pBytes, totalBytes int64) {
	if totalBytes == 0 {
		return
	}
	efficiency := (float64(p2pBytes) / float64(totalBytes)) * 100.0
	p.UpdateP2PEfficiency(streamID, efficiency)
}
