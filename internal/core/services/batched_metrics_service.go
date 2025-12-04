package services

import (
	"context"
	"sync"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/pkg/batch"
)

// BatchedMetricsService wraps MetricsService with batching support
type BatchedMetricsService struct {
	baseService *MetricsService
	batcher     *batch.Batcher
	mu          sync.RWMutex
}

// MetricsOperation represents a batched metrics update operation
type MetricsOperation struct {
	StreamID    domain.StreamID
	Type        string // "publisher", "subscriber", "bitrate", "latency"
	Value       interface{}
	baseService *MetricsService
}

// Execute executes a single metrics operation
func (op *MetricsOperation) Execute(ctx context.Context) error {
	switch op.Type {
	case "publisher_inc":
		op.baseService.IncrementPublisherCount(op.StreamID)
	case "publisher_dec":
		op.baseService.DecrementPublisherCount(op.StreamID)
	case "subscriber_inc":
		op.baseService.IncrementSubscriberCount(op.StreamID)
	case "subscriber_dec":
		op.baseService.DecrementSubscriberCount(op.StreamID)
	case "bitrate":
		if bitrate, ok := op.Value.(int); ok {
			op.baseService.UpdateBitrate(op.StreamID, bitrate)
		}
	case "latency":
		if latency, ok := op.Value.(time.Duration); ok {
			op.baseService.UpdateLatency(op.StreamID, latency)
		}
	}
	return nil
}

// MetricsBatchProcessor processes batches of metrics operations
type MetricsBatchProcessor struct {
	baseService *MetricsService
}

// ProcessBatch processes a batch of operations
func (p *MetricsBatchProcessor) ProcessBatch(ctx context.Context, operations []batch.Operation) error {
	// Group operations by stream ID for efficiency
	streamOps := make(map[domain.StreamID][]*MetricsOperation)

	for _, op := range operations {
		if metricsOp, ok := op.(*MetricsOperation); ok {
			streamOps[metricsOp.StreamID] = append(streamOps[metricsOp.StreamID], metricsOp)
		}
	}

	// Process each stream's operations
	for streamID, ops := range streamOps {
		for _, op := range ops {
			_ = op.Execute(ctx)
		}
		// Update stream metrics once per stream after all operations
		p.baseService.mu.Lock()
		p.baseService.updateStreamMetrics(streamID)
		p.baseService.mu.Unlock()
	}

	return nil
}

// NewBatchedMetricsService creates a new batched metrics service
func NewBatchedMetricsService(baseService *MetricsService, batchSize int, batchInterval time.Duration) *BatchedMetricsService {
	processor := &MetricsBatchProcessor{baseService: baseService}
	batcher := batch.NewBatcher(batchSize, batchInterval, processor)

	return &BatchedMetricsService{
		baseService: baseService,
		batcher:     batcher,
	}
}

// IncrementPublisherCount batches publisher count increment
func (b *BatchedMetricsService) IncrementPublisherCount(streamID domain.StreamID) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "publisher_inc",
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// DecrementPublisherCount batches publisher count decrement
func (b *BatchedMetricsService) DecrementPublisherCount(streamID domain.StreamID) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "publisher_dec",
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// IncrementSubscriberCount batches subscriber count increment
func (b *BatchedMetricsService) IncrementSubscriberCount(streamID domain.StreamID) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "subscriber_inc",
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// DecrementSubscriberCount batches subscriber count decrement
func (b *BatchedMetricsService) DecrementSubscriberCount(streamID domain.StreamID) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "subscriber_dec",
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// UpdateBitrate batches bitrate update
func (b *BatchedMetricsService) UpdateBitrate(streamID domain.StreamID, bitrate int) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "bitrate",
		Value:     bitrate,
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// UpdateLatency batches latency update
func (b *BatchedMetricsService) UpdateLatency(streamID domain.StreamID, latency time.Duration) {
	op := &MetricsOperation{
		StreamID: streamID,
		Type:     "latency",
		Value:     latency,
		baseService: b.baseService,
	}
	_ = b.batcher.Add(op)
}

// GetStreamMetrics gets stream metrics (not batched, immediate)
func (b *BatchedMetricsService) GetStreamMetrics(streamID domain.StreamID) *domain.StreamMetrics {
	return b.baseService.GetStreamMetrics(streamID)
}

// RecordConnection batches connection record
func (b *BatchedMetricsService) RecordConnection(streamID domain.StreamID) {
	b.baseService.RecordConnection(streamID)
}

// RemoveConnection batches connection removal
func (b *BatchedMetricsService) RemoveConnection(streamID domain.StreamID) {
	b.baseService.RemoveConnection(streamID)
}

// GetConnectionCount gets connection count (not batched, immediate)
func (b *BatchedMetricsService) GetConnectionCount(streamID domain.StreamID) int {
	return b.baseService.GetConnectionCount(streamID)
}

// Flush flushes all pending operations
func (b *BatchedMetricsService) Flush(ctx context.Context) error {
	return b.batcher.Flush(ctx)
}

// Stop stops the batcher
func (b *BatchedMetricsService) Stop() {
	b.batcher.Stop()
}

