package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ContextLogger provides context-aware logging
type ContextLogger struct {
	logger *zap.Logger
}

// NewContextLogger creates a new context logger
func NewContextLogger(logger *zap.Logger) *ContextLogger {
	return &ContextLogger{
		logger: logger,
	}
}

// WithContext adds context fields to logger
func (cl *ContextLogger) WithContext(ctx context.Context) *zap.Logger {
	fields := []zapcore.Field{}

	// Extract trace ID from context if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			fields = append(fields, zap.String("trace_id", id))
		}
	}

	// Extract user ID from context if available
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			fields = append(fields, zap.String("user_id", id))
		}
	}

	// Extract request ID from context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", id))
		}
	}

	if len(fields) == 0 {
		return cl.logger
	}

	return cl.logger.With(fields...)
}

// WithFields adds custom fields to logger
func (cl *ContextLogger) WithFields(fields ...zapcore.Field) *zap.Logger {
	return cl.logger.With(fields...)
}

// WithError adds error to logger
func (cl *ContextLogger) WithError(err error) *zap.Logger {
	return cl.logger.With(zap.Error(err))
}

// LogRequest logs an HTTP request with context
func (cl *ContextLogger) LogRequest(ctx context.Context, method, path string, statusCode int, duration int64) {
	logger := cl.WithContext(ctx)
	logger.Info("http_request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Int64("duration_ms", duration),
	)
}

// LogError logs an error with context
func (cl *ContextLogger) LogError(ctx context.Context, err error, message string, fields ...zapcore.Field) {
	logger := cl.WithContext(ctx).With(zap.Error(err))
	allFields := append(fields, zap.String("message", message))
	logger.Error("error_occurred", allFields...)
}

// LogInfo logs info message with context
func (cl *ContextLogger) LogInfo(ctx context.Context, message string, fields ...zapcore.Field) {
	logger := cl.WithContext(ctx)
	logger.Info(message, fields...)
}

// LogDebug logs debug message with context
func (cl *ContextLogger) LogDebug(ctx context.Context, message string, fields ...zapcore.Field) {
	logger := cl.WithContext(ctx)
	logger.Debug(message, fields...)
}

// LogWarn logs warning message with context
func (cl *ContextLogger) LogWarn(ctx context.Context, message string, fields ...zapcore.Field) {
	logger := cl.WithContext(ctx)
	logger.Warn(message, fields...)
}

