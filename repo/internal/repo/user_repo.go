package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, phone, password_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.Username, user.DisplayName, user.Email, user.Phone,
		user.PasswordHash, user.IsActive, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "idx_users_username_active") {
			return fmt.Errorf("username already exists")
		}
		if strings.Contains(err.Error(), "idx_users_email_active") {
			return fmt.Errorf("email already exists")
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return r.scanOne(ctx, `
		SELECT id, username, display_name, email, phone, password_hash, is_active,
		       failed_attempts, locked_until, last_login_at, deleted_at, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return r.scanOne(ctx, `
		SELECT id, username, display_name, email, phone, password_hash, is_active,
		       failed_attempts, locked_until, last_login_at, deleted_at, created_at, updated_at
		FROM users WHERE lower(username) = lower($1) AND deleted_at IS NULL`, username)
}

func (r *UserRepo) UpdateLoginSuccess(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET failed_attempts = 0, locked_until = NULL,
		       last_login_at = $2, updated_at = $2
		WHERE id = $1`,
		id, time.Now().UTC(),
	)
	return err
}

func (r *UserRepo) IncrementFailedAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		UPDATE users SET failed_attempts = failed_attempts + 1, updated_at = now()
		WHERE id = $1
		RETURNING failed_attempts`,
		id,
	).Scan(&count)
	return count, err
}

func (r *UserRepo) LockAccount(ctx context.Context, id uuid.UUID, until time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET locked_until = $2, updated_at = now()
		WHERE id = $1`,
		id, until,
	)
	return err
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET display_name = $2, email = $3, phone = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL`,
		user.ID, user.DisplayName, user.Email, user.Phone, time.Now().UTC(),
	)
	return err
}

func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET deleted_at = now(), updated_at = now(), is_active = false
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

func (r *UserRepo) scanOne(ctx context.Context, query string, args ...interface{}) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Phone,
		&u.PasswordHash, &u.IsActive, &u.FailedAttempts, &u.LockedUntil,
		&u.LastLoginAt, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
