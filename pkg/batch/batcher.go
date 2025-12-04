package batch

import (
	"context"
	"sync"
	"time"
)

// Batcher batches operations and executes them in batches
type Batcher struct {
	batchSize     int
	batchInterval time.Duration
	mu            sync.Mutex
	pending       []Operation
	flushChan     chan struct{}
	stopChan      chan struct{}
	processor     Processor
}

// Operation represents a single operation to be batched
type Operation interface {
	Execute(ctx context.Context) error
}

// Processor processes a batch of operations
type Processor interface {
	ProcessBatch(ctx context.Context, operations []Operation) error
}

// NewBatcher creates a new batcher
func NewBatcher(batchSize int, batchInterval time.Duration, processor Processor) *Batcher {
	b := &Batcher{
		batchSize:     batchSize,
		batchInterval: batchInterval,
		pending:       make([]Operation, 0, batchSize),
		flushChan:     make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
		processor:     processor,
	}

	go b.run()

	return b
}

// Add adds an operation to the batch
func (b *Batcher) Add(op Operation) error {
	b.mu.Lock()
	b.pending = append(b.pending, op)
	shouldFlush := len(b.pending) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushChan <- struct{}{}:
		default:
		}
	}

	return nil
}

// Flush immediately processes all pending operations
func (b *Batcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return nil
	}

	ops := make([]Operation, len(b.pending))
	copy(ops, b.pending)
	b.pending = b.pending[:0]
	b.mu.Unlock()

	return b.processor.ProcessBatch(ctx, ops)
}

// run processes batches periodically
func (b *Batcher) run() {
	ticker := time.NewTicker(b.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			_ = b.Flush(ctx)
		case <-b.flushChan:
			ctx := context.Background()
			_ = b.Flush(ctx)
		case <-b.stopChan:
			// Final flush on stop
			ctx := context.Background()
			_ = b.Flush(ctx)
			return
		}
	}
}

// Stop stops the batcher and flushes remaining operations
func (b *Batcher) Stop() {
	close(b.stopChan)
}

// PendingCount returns the number of pending operations
func (b *Batcher) PendingCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}

