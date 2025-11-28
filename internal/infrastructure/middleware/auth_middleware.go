package middleware

import (
	"context"
	"net/http"
	"strings"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func OptionalAuthMiddleware(authService services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			if claims, err := authService.ValidateToken(token); err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
			}
		}

		c.Next()
	}
}

func StreamPermissionMiddleware(authService services.AuthService, requiredRole domain.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by AuthMiddleware)
		userIDVal, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		userID, ok := userIDVal.(domain.UserID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
			c.Abort()
			return
		}

		streamID := domain.StreamID(c.Param("id"))
		if streamID == "" {
			// Try to get from body for POST requests
			var req struct {
				StreamID domain.StreamID `json:"stream_id"`
			}
			if err := c.ShouldBindJSON(&req); err == nil {
				streamID = req.StreamID
			}
		}

		if streamID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stream_id required"})
			c.Abort()
			return
		}

		// Create context with user_id for auth service
		ctx := context.WithValue(c.Request.Context(), "user_id", userID)
		if err := authService.CheckStreamPermission(ctx, userID, streamID, requiredRole); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Set("stream_id", streamID)
		c.Next()
	}
}
