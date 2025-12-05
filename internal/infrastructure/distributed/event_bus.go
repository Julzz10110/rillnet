package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"go.uber.org/zap"
	"github.com/redis/go-redis/v9"
)

// EventType represents the type of event
type EventType string

const (
	EventPeerJoined    EventType = "peer.joined"
	EventPeerLeft      EventType = "peer.left"
	EventStreamCreated EventType = "stream.created"
	EventStreamEnded   EventType = "stream.ended"
	EventMeshRebalance EventType = "mesh.rebalance"
	EventQualityChange EventType = "quality.changed"
)

// Event represents a distributed event
type Event struct {
	Type      EventType          `json:"type"`
	InstanceID string            `json:"instance_id"`
	Timestamp time.Time          `json:"timestamp"`
	StreamID  domain.StreamID    `json:"stream_id,omitempty"`
	PeerID    domain.PeerID      `json:"peer_id,omitempty"`
	Payload   json.RawMessage   `json:"payload,omitempty"`
}

// EventBus provides event publishing and subscription for coordination
type EventBus struct {
	client     *redis.Client
	instanceID string
	logger     *zap.SugaredLogger
	pubsub     *redis.PubSub
	channels   []string
}

// NewEventBus creates a new event bus
func NewEventBus(
	client *redis.Client,
	instanceID string,
	logger *zap.SugaredLogger,
) *EventBus {
	return &EventBus{
		client:     client,
		instanceID: instanceID,
		logger:     logger,
		channels:   []string{"rillnet:events"},
	}
}

// Publish publishes an event to the event bus
func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
	event.InstanceID = eb.instanceID
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	channel := eb.channels[0]
	if err := eb.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	eb.logger.Debugw("published event",
		"type", event.Type,
		"stream_id", event.StreamID,
		"peer_id", event.PeerID,
	)

	return nil
}

// Subscribe subscribes to events and calls handler for each event
func (eb *EventBus) Subscribe(ctx context.Context, handler func(*Event) error) error {
	if eb.pubsub != nil {
		return fmt.Errorf("already subscribed")
	}

	eb.pubsub = eb.client.Subscribe(ctx, eb.channels...)
	defer eb.pubsub.Close()

	ch := eb.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-ch:
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				eb.logger.Warnw("failed to unmarshal event",
					"error", err,
					"payload", msg.Payload,
				)
				continue
			}

			// Skip events from this instance
			if event.InstanceID == eb.instanceID {
				continue
			}

			// Handle event
			if err := handler(&event); err != nil {
				eb.logger.Warnw("error handling event",
					"type", event.Type,
					"error", err,
				)
			}
		}
	}
}

// PublishPeerJoined publishes a peer joined event
func (eb *EventBus) PublishPeerJoined(ctx context.Context, streamID domain.StreamID, peerID domain.PeerID) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"stream_id": streamID,
		"peer_id":   peerID,
	})

	return eb.Publish(ctx, &Event{
		Type:    EventPeerJoined,
		StreamID: streamID,
		PeerID:  peerID,
		Payload: payload,
	})
}

// PublishPeerLeft publishes a peer left event
func (eb *EventBus) PublishPeerLeft(ctx context.Context, streamID domain.StreamID, peerID domain.PeerID) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"stream_id": streamID,
		"peer_id":   peerID,
	})

	return eb.Publish(ctx, &Event{
		Type:    EventPeerLeft,
		StreamID: streamID,
		PeerID:  peerID,
		Payload: payload,
	})
}

// PublishStreamCreated publishes a stream created event
func (eb *EventBus) PublishStreamCreated(ctx context.Context, streamID domain.StreamID) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"stream_id": streamID,
	})

	return eb.Publish(ctx, &Event{
		Type:    EventStreamCreated,
		StreamID: streamID,
		Payload: payload,
	})
}

// PublishMeshRebalance publishes a mesh rebalance event
func (eb *EventBus) PublishMeshRebalance(ctx context.Context, streamID domain.StreamID) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"stream_id": streamID,
	})

	return eb.Publish(ctx, &Event{
		Type:    EventMeshRebalance,
		StreamID: streamID,
		Payload: payload,
	})
}

// Close closes the event bus
func (eb *EventBus) Close() error {
	if eb.pubsub != nil {
		return eb.pubsub.Close()
	}
	return nil
}

