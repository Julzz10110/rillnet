package streaming

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"go.uber.org/zap"
)

// Segmenter handles video segmentation for HLS/DASH
type Segmenter struct {
	segmentDuration time.Duration
	outputPath      string
	logger          *zap.SugaredLogger
}

// Segment represents a video segment
type Segment struct {
	ID          string
	StreamID    domain.StreamID
	Quality     string
	Index       int
	StartTime   time.Time
	Duration    time.Duration
	FilePath    string
	URL         string
	Size        int64
}

// NewSegmenter creates a new segmenter
func NewSegmenter(segmentDuration time.Duration, outputPath string, logger *zap.SugaredLogger) *Segmenter {
	return &Segmenter{
		segmentDuration: segmentDuration,
		outputPath:      outputPath,
		logger:          logger,
	}
}

// CreateSegment creates a new video segment
func (s *Segmenter) CreateSegment(ctx context.Context, streamID domain.StreamID, quality string, index int, data []byte) (*Segment, error) {
	segmentID := fmt.Sprintf("segment-%d", index)
	fileName := fmt.Sprintf("%s-%s-%d.ts", streamID, quality, index)
	filePath := fmt.Sprintf("%s/%s/%s/%s", s.outputPath, streamID, quality, fileName)

	// In a real implementation, this would:
	// 1. Write segment data to file
	// 2. Generate segment metadata
	// 3. Update playlist files

	segment := &Segment{
		ID:        segmentID,
		StreamID:  streamID,
		Quality:   quality,
		Index:     index,
		StartTime: time.Now(),
		Duration:  s.segmentDuration,
		FilePath:  filePath,
		URL:       fmt.Sprintf("/segments/%s/%s/%s", streamID, quality, fileName),
		Size:      int64(len(data)),
	}

	s.logger.Debugw("created segment",
		"stream_id", streamID,
		"quality", quality,
		"index", index,
		"size", segment.Size,
	)

	return segment, nil
}

// GeneratePlaylist generates HLS playlist (M3U8)
func (s *Segmenter) GeneratePlaylist(ctx context.Context, streamID domain.StreamID, quality string, segments []*Segment) (string, error) {
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:3\n"
	playlist += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(s.segmentDuration.Seconds()))
	playlist += "#EXT-X-MEDIA-SEQUENCE:0\n"

	for _, segment := range segments {
		playlist += fmt.Sprintf("#EXTINF:%.3f,\n", segment.Duration.Seconds())
		playlist += fmt.Sprintf("%s\n", segment.URL)
	}

	playlist += "#EXT-X-ENDLIST\n"

	return playlist, nil
}

// GenerateMasterPlaylist generates HLS master playlist with multiple qualities
func (s *Segmenter) GenerateMasterPlaylist(ctx context.Context, streamID domain.StreamID, qualities []string) (string, error) {
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:3\n"

	for i, quality := range qualities {
		bandwidth := s.getBandwidthForQuality(quality)
		playlist += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d\n", bandwidth)
		playlist += fmt.Sprintf("/streams/%s/%s/index.m3u8\n", streamID, quality)
		
		if i < len(qualities)-1 {
			playlist += "\n"
		}
	}

	return playlist, nil
}

// getBandwidthForQuality returns bandwidth for a quality level
func (s *Segmenter) getBandwidthForQuality(quality string) int {
	switch quality {
	case "high":
		return 2500000 // 2.5 Mbps
	case "medium":
		return 1000000 // 1 Mbps
	case "low":
		return 500000 // 500 kbps
	default:
		return 1000000
	}
}

// SegmentCache manages segment caching for P2P sharing
type SegmentCache struct {
	segments map[string]*Segment
	mu       sync.RWMutex
	maxSize  int
}

// NewSegmentCache creates a new segment cache
func NewSegmentCache(maxSize int) *SegmentCache {
	return &SegmentCache{
		segments: make(map[string]*Segment),
		maxSize:  maxSize,
	}
}

// Add adds a segment to cache
func (sc *SegmentCache) Add(segment *Segment) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Remove oldest if cache is full
	if len(sc.segments) >= sc.maxSize {
		// Simple FIFO eviction
		var oldestKey string
		var oldestTime time.Time
		for key, seg := range sc.segments {
			if oldestTime.IsZero() || seg.StartTime.Before(oldestTime) {
				oldestTime = seg.StartTime
				oldestKey = key
			}
		}
		if oldestKey != "" {
			delete(sc.segments, oldestKey)
		}
	}

	key := fmt.Sprintf("%s-%s-%d", segment.StreamID, segment.Quality, segment.Index)
	sc.segments[key] = segment
}

// Get retrieves a segment from cache
func (sc *SegmentCache) Get(streamID domain.StreamID, quality string, index int) (*Segment, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	key := fmt.Sprintf("%s-%s-%d", streamID, quality, index)
	segment, exists := sc.segments[key]
	return segment, exists
}

// ListSegments lists all segments for a stream/quality
func (sc *SegmentCache) ListSegments(streamID domain.StreamID, quality string) []*Segment {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var result []*Segment
	prefix := fmt.Sprintf("%s-%s-", streamID, quality)

	for key, segment := range sc.segments {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, segment)
		}
	}

	return result
}

