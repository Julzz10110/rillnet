package postgres

import (
	"context"
	"errors"
	"fmt"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User, passwordHash string) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	_, err := r.pool.Exec(ctx, `
INSERT INTO users(id, username, email, password_hash)
VALUES ($1, $2, $3, $4)`,
		user.ID, user.Username, user.Email, passwordHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, string, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id, username, email, created_at, password_hash
FROM users
WHERE username=$1`, username)

	var u domain.User
	var hash string
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt, &hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", domain.ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("get user by username: %w", err)
	}
	return &u, hash, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id, username, email, created_at, password_hash
FROM users
WHERE email=$1`, email)

	var u domain.User
	var hash string
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt, &hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", domain.ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("get user by email: %w", err)
	}
	return &u, hash, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

