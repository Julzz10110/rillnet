package services

import (
	"context"
	"errors"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	ErrUnauthorized = errors.New("unauthorized")
)

type AuthService interface {
	GenerateToken(userID domain.UserID, username string) (string, error)
	GenerateRefreshToken(userID domain.UserID) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
	ValidateRefreshToken(tokenString string) (*Claims, error)
	CheckStreamPermission(ctx context.Context, userID domain.UserID, streamID domain.StreamID, requiredRole domain.UserRole) error
	GetUserFromContext(ctx context.Context) (domain.UserID, error)
}

type Claims struct {
	UserID   domain.UserID `json:"user_id"`
	Username string        `json:"username"`
	jwt.RegisteredClaims
}

type authService struct {
	jwtSecret        []byte
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
	streamService    ports.StreamService // Optional, can be nil
}

func NewAuthService(
	jwtSecret string,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	streamService ports.StreamService, // Can be nil for token-only validation
) AuthService {
	return &authService{
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		streamService:   streamService,
	}
}

func (s *authService) GenerateToken(userID domain.UserID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *authService) GenerateRefreshToken(userID domain.UserID) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *authService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (s *authService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return s.ValidateToken(tokenString)
}

func (s *authService) CheckStreamPermission(ctx context.Context, userID domain.UserID, streamID domain.StreamID, requiredRole domain.UserRole) error {
	if s.streamService == nil {
		// Stream service not available, skip permission check
		return nil
	}

	stream, err := s.streamService.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Owner always has all permissions
	if stream.OwnerUserID == userID {
		return nil
	}

	// Check user's role in stream permissions
	for _, perm := range stream.Permissions {
		if perm.UserID == userID {
			if s.hasRequiredPermission(perm.Role, requiredRole) {
				return nil
			}
		}
	}

	return ErrUnauthorized
}

func (s *authService) hasRequiredPermission(userRole, requiredRole domain.UserRole) bool {
	roleHierarchy := map[domain.UserRole]int{
		domain.RoleViewer:    1,
		domain.RoleModerator: 2,
		domain.RoleOwner:     3,
	}

	userLevel := roleHierarchy[userRole]
	requiredLevel := roleHierarchy[requiredRole]

	return userLevel >= requiredLevel
}

func (s *authService) GetUserFromContext(ctx context.Context) (domain.UserID, error) {
	userID, ok := ctx.Value("user_id").(domain.UserID)
	if !ok {
		return "", ErrUnauthorized
	}
	return userID, nil
}

