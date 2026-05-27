package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) ports.RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Store(ctx context.Context, userID domain.UserID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO refresh_tokens(id, user_id, token_hash, expires_at)
VALUES (gen_random_uuid(), $1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) MarkReplaced(ctx context.Context, tokenHash string, replacedByHash string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE refresh_tokens
SET replaced_by_hash=$2, revoked_at=COALESCE(revoked_at, now())
WHERE token_hash=$1`, tokenHash, replacedByHash)
	if err != nil {
		return fmt.Errorf("mark refresh token replaced: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE refresh_tokens
SET revoked_at=$2
WHERE token_hash=$1`, tokenHash, revokedAt)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) IsActive(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	row := r.pool.QueryRow(ctx, `
SELECT expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash=$1`, tokenHash)
	var expiresAt time.Time
	var revokedAt *time.Time
	if err := row.Scan(&expiresAt, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("get refresh token: %w", err)
	}
	if revokedAt != nil {
		return false, nil
	}
	if !expiresAt.After(now) {
		return false, nil
	}
	return true, nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID domain.UserID, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE refresh_tokens
SET revoked_at=$2
WHERE user_id=$1 AND revoked_at IS NULL`, userID, revokedAt)
	if err != nil {
		return fmt.Errorf("revoke refresh tokens for user: %w", err)
	}
	return nil
}

