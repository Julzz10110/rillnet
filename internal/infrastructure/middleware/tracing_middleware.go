package middleware

import (
	"time"

	"rillnet/pkg/tracing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// TracingMiddleware adds tracing to HTTP requests
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start span
		ctx, span := tracing.TraceHTTPRequest(c.Request.Context(), c.Request.Method, c.FullPath())
		defer span.End()

		// Add request attributes
		span.SetAttributes(
			attribute.String("http.scheme", c.Request.URL.Scheme),
			attribute.String("http.host", c.Request.Host),
			attribute.String("http.user_agent", c.Request.UserAgent()),
			attribute.String("http.remote_addr", c.ClientIP()),
		)

		// Update context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int64("http.response_size", int64(c.Writer.Size())),
			attribute.Int64("http.duration_ms", duration.Milliseconds()),
		)

		// Set status
		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, c.Errors.String())
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

