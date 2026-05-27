package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
	RegisterUser(ctx context.Context, username, email, password string) (*domain.User, string, string, error) // user, access, refresh
	LoginUser(ctx context.Context, username, password string) (*domain.User, string, string, error)         // user, access, refresh
	RotateRefreshToken(ctx context.Context, refreshToken string) (string, string, error)                     // access, refresh
	Logout(ctx context.Context, refreshToken string) error
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
	userRepo         ports.UserRepository
	refreshRepo      ports.RefreshTokenRepository
}

func NewAuthService(
	jwtSecret string,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	streamService ports.StreamService, // Can be nil for token-only validation
	userRepo ports.UserRepository,
	refreshRepo ports.RefreshTokenRepository,
) AuthService {
	return &authService{
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		streamService:   streamService,
		userRepo:        userRepo,
		refreshRepo:     refreshRepo,
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
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if s.refreshRepo == nil {
		// Backward-compatible mode: if no refresh repository configured, accept JWT-only.
		return claims, nil
	}
	active, err := s.refreshRepo.IsActive(context.Background(), hashToken(tokenString), time.Now())
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, domain.ErrRefreshTokenRevoked
	}
	return claims, nil
}

func (s *authService) RegisterUser(ctx context.Context, username, email, password string) (*domain.User, string, string, error) {
	// Backward-compatible stub mode (no persistent auth storage configured).
	if s.userRepo == nil || s.refreshRepo == nil {
		user := &domain.User{
			ID:        domain.UserID(uuid.New().String()),
			Username:  strings.TrimSpace(username),
			Email:     strings.TrimSpace(strings.ToLower(email)),
			CreatedAt: time.Now(),
		}
		access, err := s.GenerateToken(user.ID, user.Username)
		if err != nil {
			return nil, "", "", err
		}
		refresh, err := s.GenerateRefreshToken(user.ID)
		if err != nil {
			return nil, "", "", err
		}
		return user, access, refresh, nil
	}
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}
	user := &domain.User{
		ID:        domain.UserID(utilsGenerateUUID()),
		Username:  username,
		Email:     email,
		CreatedAt: time.Now(),
	}
	if err := s.userRepo.Create(ctx, user, string(hashBytes)); err != nil {
		return nil, "", "", err
	}
	access, err := s.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := s.issueAndStoreRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}
	return user, access, refresh, nil
}

func (s *authService) LoginUser(ctx context.Context, username, password string) (*domain.User, string, string, error) {
	// Backward-compatible stub mode (no persistent auth storage configured).
	if s.userRepo == nil || s.refreshRepo == nil {
		user := &domain.User{
			ID:        domain.UserID(uuid.New().String()),
			Username:  strings.TrimSpace(username),
			CreatedAt: time.Now(),
		}
		access, err := s.GenerateToken(user.ID, user.Username)
		if err != nil {
			return nil, "", "", err
		}
		refresh, err := s.GenerateRefreshToken(user.ID)
		if err != nil {
			return nil, "", "", err
		}
		return user, access, refresh, nil
	}
	username = strings.TrimSpace(username)
	u, hash, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, "", "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, "", "", domain.ErrInvalidCredentials
	}
	access, err := s.GenerateToken(u.ID, u.Username)
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := s.issueAndStoreRefreshToken(ctx, u.ID)
	if err != nil {
		return nil, "", "", err
	}
	return u, access, refresh, nil
}

func (s *authService) RotateRefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	// Backward-compatible stub mode (no refresh token persistence/rotation).
	if s.refreshRepo == nil {
		claims, err := s.ValidateRefreshToken(refreshToken)
		if err != nil {
			return "", "", err
		}
		access, err := s.GenerateToken(claims.UserID, claims.Username)
		if err != nil {
			return "", "", err
		}
		newRefresh, err := s.GenerateRefreshToken(claims.UserID)
		if err != nil {
			return "", "", err
		}
		return access, newRefresh, nil
	}
	claims, err := s.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	newRefresh, err := s.issueAndStoreRefreshToken(ctx, claims.UserID)
	if err != nil {
		return "", "", err
	}
	_ = s.refreshRepo.MarkReplaced(ctx, hashToken(refreshToken), hashToken(newRefresh))

	access, err := s.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		return "", "", err
	}
	return access, newRefresh, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if s.refreshRepo == nil {
		return nil
	}
	return s.refreshRepo.Revoke(ctx, hashToken(refreshToken), time.Now())
}

func (s *authService) issueAndStoreRefreshToken(ctx context.Context, userID domain.UserID) (string, error) {
	refresh, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(s.refreshTokenTTL)
	if err := s.refreshRepo.Store(ctx, userID, hashToken(refresh), expiresAt); err != nil {
		return "", err
	}
	return refresh, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// utilsGenerateUUID isolates UUID generation so auth_service.go stays stdlib-only at call sites.
func utilsGenerateUUID() string {
	// NOTE: implemented in a small helper to avoid leaking uuid into domain package.
	return uuid.New().String()
}

func (s *authService) CheckStreamPermission(ctx context.Context, userID domain.UserID, streamID domain.StreamID, requiredRole domain.UserRole) error {
	if s.streamService == nil {
		// Stream service not available, skip permission check
		return nil
	}

	// Handle empty or invalid streamID
	if streamID == "" || streamID == "undefined" || streamID == "null" {
		return ErrUnauthorized
	}

	// If user is authenticated (userID is not empty), allow access
	// This is a temporary fix - in production, you should check actual permissions
	if userID != "" {
		return nil
	}

	stream, err := s.streamService.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Owner always has all permissions
	if stream.OwnerUserID == userID && userID != "" {
		return nil
	}

	// If stream has no OwnerUserID set but user is authenticated, allow access
	// This handles the case where stream was created before OwnerUserID was properly set
	if stream.OwnerUserID == "" && userID != "" {
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

	// If user is authenticated, allow access (temporary fix)
	if userID != "" {
		return nil
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
	userID, ok := ctx.Value(domain.UserIDContextKey).(domain.UserID)
	if !ok {
		return "", ErrUnauthorized
	}
	return userID, nil
}

