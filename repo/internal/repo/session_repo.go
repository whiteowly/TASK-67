package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

func (r *SessionRepo) Create(ctx context.Context, sess *model.AuthSession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth_sessions (id, user_id, token_hash, ip_addr, user_agent, expires_at, created_at)
		VALUES ($1, $2, $3, $4::inet, $5, $6, $7)`,
		sess.ID, sess.UserID, sess.TokenHash, sess.IPAddr, sess.UserAgent,
		sess.ExpiresAt, sess.CreatedAt,
	)
	return err
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*model.AuthSession, error) {
	sess := &model.AuthSession{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, ip_addr::text, user_agent, expires_at, revoked_at, created_at
		FROM auth_sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2`,
		tokenHash, time.Now().UTC(),
	).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.IPAddr,
		&sess.UserAgent, &sess.ExpiresAt, &sess.RevokedAt, &sess.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

func (r *SessionRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM auth_sessions WHERE expires_at < $1 OR revoked_at IS NOT NULL`,
		time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
