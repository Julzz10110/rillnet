package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider wraps OpenTelemetry tracer provider
type TracerProvider struct {
	tp *tracesdk.TracerProvider
}

// Config contains tracing configuration
type Config struct {
	Enabled     bool
	ServiceName string
	JaegerURL   string
	Environment string
	SampleRate  float64
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		ServiceName: "rillnet",
		JaegerURL:   "http://localhost:14268/api/traces",
		Environment: "development",
		SampleRate:  1.0, // 100% sampling by default
	}
}

// Init initializes tracing
func Init(cfg Config) (*TracerProvider, error) {
	if !cfg.Enabled {
		return &TracerProvider{}, nil
	}

	// Create Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerURL)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{tp: tp}, nil
}

// Shutdown shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.tp != nil {
		return tp.tp.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("rillnet")
	return tracer.Start(ctx, name, opts...)
}

// SpanFromContext gets span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// RecordError records an error in the current span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(code, description)
	}
}

// Common span attributes
var (
	StreamIDKey    = attribute.Key("stream.id")
	PeerIDKey      = attribute.Key("peer.id")
	UserIDKey      = attribute.Key("user.id")
	QualityKey     = attribute.Key("quality")
	BitrateKey     = attribute.Key("bitrate")
	LatencyKey     = attribute.Key("latency")
	PacketLossKey  = attribute.Key("packet_loss")
	ErrorKey       = attribute.Key("error")
	DurationKey    = attribute.Key("duration")
)

// TraceHTTPRequest traces an HTTP request
func TraceHTTPRequest(ctx context.Context, method, path string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("http.%s", method),
		trace.WithAttributes(
			semconv.HTTPMethodKey.String(method),
			semconv.HTTPRouteKey.String(path),
		),
	)
}

// TraceWebSocketMessage traces a WebSocket message
func TraceWebSocketMessage(ctx context.Context, messageType string, peerID string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("websocket.%s", messageType),
		trace.WithAttributes(
			attribute.String("websocket.message_type", messageType),
			PeerIDKey.String(peerID),
		),
	)
}

// TraceWebRTC traces a WebRTC operation
func TraceWebRTC(ctx context.Context, operation string, peerID, streamID string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("webrtc.%s", operation),
		trace.WithAttributes(
			attribute.String("webrtc.operation", operation),
			PeerIDKey.String(peerID),
			StreamIDKey.String(streamID),
		),
	)
}

// TraceMeshOperation traces a mesh network operation
func TraceMeshOperation(ctx context.Context, operation string, streamID string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("mesh.%s", operation),
		trace.WithAttributes(
			attribute.String("mesh.operation", operation),
			StreamIDKey.String(streamID),
		),
	)
}

// TraceDatabaseOperation traces a database operation
func TraceDatabaseOperation(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("db.%s", operation),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.table", table),
		),
	)
}

// MeasureDuration measures the duration of an operation
func MeasureDuration(ctx context.Context, start time.Time, operation string) {
	duration := time.Since(start)
	AddSpanAttributes(ctx,
		attribute.String("operation", operation),
		DurationKey.Int64(duration.Milliseconds()),
	)
}

