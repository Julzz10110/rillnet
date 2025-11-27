package memory

import (
	"context"
	"fmt"
	"sync"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
)

type MemoryStreamRepository struct {
	streams map[domain.StreamID]*domain.Stream
	mu      sync.RWMutex
}

func NewMemoryStreamRepository() ports.StreamRepository {
	return &MemoryStreamRepository{
		streams: make(map[domain.StreamID]*domain.Stream),
	}
}

func (r *MemoryStreamRepository) Create(ctx context.Context, stream *domain.Stream) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[stream.ID]; exists {
		return fmt.Errorf("stream already exists: %s", stream.ID)
	}

	r.streams[stream.ID] = stream
	return nil
}

func (r *MemoryStreamRepository) GetByID(ctx context.Context, id domain.StreamID) (*domain.Stream, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stream, exists := r.streams[id]
	if !exists {
		return nil, domain.ErrStreamNotFound
	}

	return stream, nil
}

func (r *MemoryStreamRepository) Update(ctx context.Context, stream *domain.Stream) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[stream.ID]; !exists {
		return domain.ErrStreamNotFound
	}

	r.streams[stream.ID] = stream
	return nil
}

func (r *MemoryStreamRepository) Delete(ctx context.Context, id domain.StreamID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[id]; !exists {
		return domain.ErrStreamNotFound
	}

	delete(r.streams, id)
	return nil
}

func (r *MemoryStreamRepository) ListActive(ctx context.Context) ([]*domain.Stream, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var activeStreams []*domain.Stream
	for _, stream := range r.streams {
		if stream.Active {
			activeStreams = append(activeStreams, stream)
		}
	}

	return activeStreams, nil
}
