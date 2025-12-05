package http

import (
	"net/http"
	"strings"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/services"
	"rillnet/pkg/errors"
	"rillnet/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/auth")
	{
		api.POST("/register", h.Register)
		api.POST("/login", h.Login)
		api.POST("/refresh", h.RefreshToken)
	}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=50"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,max=2048"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.BindJSON(&req); err != nil {
		c.Error(errors.NewInvalidInputError("invalid request format"))
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Validate input
	if err := validation.ValidateUsername(req.Username); err != nil {
		c.Error(errors.NewInvalidInputError(err.Error()))
		return
	}
	if err := validation.ValidateEmail(req.Email); err != nil {
		c.Error(errors.NewInvalidInputError(err.Error()))
		return
	}
	if err := validation.ValidatePassword(req.Password); err != nil {
		c.Error(errors.NewInvalidInputError(err.Error()))
		return
	}

	// TODO: In production, implement proper user storage and password hashing
	// For now, generate a user ID and create tokens
	userID := domain.UserID(uuid.New().String())

	accessToken, err := h.authService.GenerateToken(userID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id":       userID,
		"username":      req.Username,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(time.Minute * 15 / time.Second),
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Username = strings.TrimSpace(req.Username)

	// TODO: In production, validate credentials against user storage
	// For now, generate a user ID and create tokens
	userID := domain.UserID(uuid.New().String())

	accessToken, err := h.authService.GenerateToken(userID, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"username":      req.Username,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(time.Minute * 15 / time.Second),
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	accessToken, err := h.authService.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
		"expires_in":   int(time.Minute * 15 / time.Second),
	})
}



