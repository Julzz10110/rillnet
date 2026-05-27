package ports

import (
	"context"
	"time"

	"rillnet/internal/core/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User, passwordHash string) error
	GetByUsername(ctx context.Context, username string) (*domain.User, string, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, string, error)
}

type RefreshTokenRepository interface {
	Store(ctx context.Context, userID domain.UserID, tokenHash string, expiresAt time.Time) error
	MarkReplaced(ctx context.Context, tokenHash string, replacedByHash string) error
	Revoke(ctx context.Context, tokenHash string, revokedAt time.Time) error
	IsActive(ctx context.Context, tokenHash string, now time.Time) (bool, error)
	RevokeAllForUser(ctx context.Context, userID domain.UserID, revokedAt time.Time) error
}

