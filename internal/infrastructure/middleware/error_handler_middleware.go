package middleware

import (
	"net/http"

	"rillnet/pkg/errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorHandlerMiddleware handles application errors and returns appropriate HTTP responses
func ErrorHandlerMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// Try to extract AppError
			appErr := errors.GetAppError(err)
			if appErr != nil {
				// Log error with context
				logger.Errorw("application error",
					"code", appErr.Code,
					"message", appErr.Message,
					"status", appErr.HTTPStatus,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"context", appErr.Context,
				)

				// Return structured error response
				c.JSON(appErr.HTTPStatus, gin.H{
					"error":   string(appErr.Code),
					"message": appErr.Message,
					"details": appErr.Context,
				})
				return
			}

			// Handle non-AppError errors
			logger.Errorw("unhandled error",
				"error", err.Error(),
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   string(errors.ErrCodeInternal),
				"message": "Internal server error",
			})
		}
	}
}

// RecoveryMiddleware recovers from panics and returns proper error responses
func RecoveryMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorw("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)

				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   string(errors.ErrCodeInternal),
					"message": "Internal server error",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}

