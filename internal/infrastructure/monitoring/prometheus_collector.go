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

	// Bitrate update by quality can be added here
	// Based on real data from peers
}
