package tracing

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ServiceName != "rillnet" {
		t.Errorf("expected service name 'rillnet', got '%s'", cfg.ServiceName)
	}
	if cfg.JaegerURL != "http://localhost:14268/api/traces" {
		t.Errorf("unexpected Jaeger URL: %s", cfg.JaegerURL)
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("expected sample rate 1.0, got %f", cfg.SampleRate)
	}
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	
	// Test with disabled tracing (no tracer provider)
	ctx, span := StartSpan(ctx, "test.operation")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestAddSpanAttributes(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test")
	defer span.End()

	AddSpanAttributes(ctx,
		attribute.String("test.key", "test.value"),
		attribute.Int("test.number", 42),
	)
}

func TestRecordError(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test")
	defer span.End()

	err := &testError{message: "test error"}
	RecordError(ctx, err)
}

func TestMeasureDuration(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test")
	defer span.End()

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	MeasureDuration(ctx, start, "test.operation")
}

func TestTraceHTTPRequest(t *testing.T) {
	ctx := context.Background()
	ctx, span := TraceHTTPRequest(ctx, "GET", "/api/streams")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestTraceWebSocketMessage(t *testing.T) {
	ctx := context.Background()
	ctx, span := TraceWebSocketMessage(ctx, "join_stream", "peer-123")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestTraceWebRTC(t *testing.T) {
	ctx := context.Background()
	ctx, span := TraceWebRTC(ctx, "create_offer", "peer-123", "stream-456")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestTraceMeshOperation(t *testing.T) {
	ctx := context.Background()
	ctx, span := TraceMeshOperation(ctx, "rebalance", "stream-456")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

func TestTraceDatabaseOperation(t *testing.T) {
	ctx := context.Background()
	ctx, span := TraceDatabaseOperation(ctx, "get", "streams")
	if span == nil {
		t.Error("expected non-nil span")
	}
	span.End()
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

