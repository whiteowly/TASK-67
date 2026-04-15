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

type ModerationRepo struct {
	pool *pgxpool.Pool
}

func NewModerationRepo(pool *pgxpool.Pool) *ModerationRepo {
	return &ModerationRepo{pool: pool}
}

func (r *ModerationRepo) Pool() *pgxpool.Pool { return r.pool }

// CreatePostWithRateLimit creates a post with a rate limit check: counts posts in last 60 minutes.
func (r *ModerationRepo) CreatePostWithRateLimit(ctx context.Context, post *model.Post, maxPerHour int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var recentCount int
	oneHourAgo := time.Now().UTC().Add(-60 * time.Minute)
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE user_id = $1 AND created_at >= $2 AND deleted_at IS NULL`,
		post.UserID, oneHourAgo,
	).Scan(&recentCount)
	if err != nil {
		return fmt.Errorf("count recent posts: %w", err)
	}
	if recentCount >= maxPerHour {
		return fmt.Errorf("rate limit exceeded: %d posts in last hour", recentCount)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO posts (id, user_id, title, body, status, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		post.ID, post.UserID, post.Title, post.Body, post.Status,
		post.DeletedAt, post.CreatedAt, post.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert post: %w", err)
	}

	return tx.Commit(ctx)
}

// CreatePost inserts a post without rate limit check (service handles rate limiting).
func (r *ModerationRepo) CreatePost(ctx context.Context, post *model.Post) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO posts (id, user_id, title, body, status, deleted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		post.ID, post.UserID, post.Title, post.Body, post.Status,
		post.DeletedAt, post.CreatedAt, post.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert post: %w", err)
	}
	return nil
}

// IsUserBanned checks if a user has an active ban of the given type.
func (r *ModerationRepo) IsUserBanned(ctx context.Context, userID uuid.UUID, banType string) (bool, error) {
	now := time.Now().UTC()
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM account_bans
			WHERE user_id = $1 AND ban_type = $2 AND revoked_at IS NULL
			  AND starts_at <= $3 AND (ends_at IS NULL OR ends_at > $3)
		)`, userID, banType, now,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user banned: %w", err)
	}
	return exists, nil
}

// CountUserPostsSince counts posts by a user since a given time.
func (r *ModerationRepo) CountUserPostsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE user_id = $1 AND created_at >= $2 AND deleted_at IS NULL`,
		userID, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user posts since: %w", err)
	}
	return count, nil
}

// GetReportByUserAndPost checks if a user has already reported a specific post.
func (r *ModerationRepo) GetReportByUserAndPost(ctx context.Context, reporterID, postID uuid.UUID) (*model.PostReport, error) {
	rpt := &model.PostReport{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, post_id, reporter_id, reason, description, status, created_at, updated_at
		FROM post_reports WHERE reporter_id = $1 AND post_id = $2`, reporterID, postID,
	).Scan(
		&rpt.ID, &rpt.PostID, &rpt.ReporterID, &rpt.Reason,
		&rpt.Description, &rpt.Status, &rpt.CreatedAt, &rpt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get report by user and post: %w", err)
	}
	return rpt, nil
}

// CreateCase is an alias for CreateModerationCase.
func (r *ModerationRepo) CreateCase(ctx context.Context, mc *model.ModerationCase) error {
	return r.CreateModerationCase(ctx, mc)
}

// GetCaseByID retrieves a moderation case by ID.
func (r *ModerationRepo) GetCaseByID(ctx context.Context, id uuid.UUID) (*model.ModerationCase, error) {
	mc := &model.ModerationCase{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, post_id, user_id, status, assigned_to, created_at, updated_at
		FROM moderation_cases WHERE id = $1`, id,
	).Scan(
		&mc.ID, &mc.PostID, &mc.UserID, &mc.Status, &mc.AssignedTo, &mc.CreatedAt, &mc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get moderation case: %w", err)
	}
	return mc, nil
}

// UpdateCase updates a moderation case.
func (r *ModerationRepo) UpdateCase(ctx context.Context, mc *model.ModerationCase) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE moderation_cases SET status = $2, assigned_to = $3, updated_at = $4
		WHERE id = $1`,
		mc.ID, mc.Status, mc.AssignedTo, mc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update moderation case: %w", err)
	}
	return nil
}

// CreateAction is an alias for CreateModerationAction.
func (r *ModerationRepo) CreateAction(ctx context.Context, action *model.ModerationAction) error {
	return r.CreateModerationAction(ctx, action)
}

// GetBanByID retrieves an account ban by ID.
func (r *ModerationRepo) GetBanByID(ctx context.Context, id uuid.UUID) (*model.AccountBan, error) {
	ban := &model.AccountBan{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, ban_type, is_permanent, starts_at, ends_at,
		       reason, issued_by, revoked_at, revoked_by, created_at
		FROM account_bans WHERE id = $1`, id,
	).Scan(
		&ban.ID, &ban.UserID, &ban.BanType, &ban.IsPermanent, &ban.StartsAt,
		&ban.EndsAt, &ban.Reason, &ban.IssuedBy, &ban.RevokedAt, &ban.RevokedBy,
		&ban.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ban: %w", err)
	}
	return ban, nil
}

// UpdateBan updates an account ban record.
func (r *ModerationRepo) UpdateBan(ctx context.Context, ban *model.AccountBan) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE account_bans SET revoked_at = $2, revoked_by = $3
		WHERE id = $1`,
		ban.ID, ban.RevokedAt, ban.RevokedBy,
	)
	if err != nil {
		return fmt.Errorf("update ban: %w", err)
	}
	return nil
}

// ListReports returns paginated post reports.
func (r *ModerationRepo) ListReports(ctx context.Context, limit, offset int) ([]model.PostReport, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM post_reports`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, post_id, reporter_id, reason, description, status, created_at, updated_at
		FROM post_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []model.PostReport
	for rows.Next() {
		var r model.PostReport
		if err := rows.Scan(
			&r.ID, &r.PostID, &r.ReporterID, &r.Reason,
			&r.Description, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, total, rows.Err()
}

// ListCases returns paginated moderation cases.
func (r *ModerationRepo) ListCases(ctx context.Context, limit, offset int) ([]model.ModerationCase, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM moderation_cases`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cases: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, post_id, user_id, status, assigned_to, created_at, updated_at
		FROM moderation_cases
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()

	var cases []model.ModerationCase
	for rows.Next() {
		var mc model.ModerationCase
		if err := rows.Scan(
			&mc.ID, &mc.PostID, &mc.UserID, &mc.Status, &mc.AssignedTo, &mc.CreatedAt, &mc.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan case: %w", err)
		}
		cases = append(cases, mc)
	}
	return cases, total, rows.Err()
}

func (r *ModerationRepo) ListPosts(ctx context.Context, limit, offset int) ([]model.Post, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, title, body, status, deleted_at, created_at, updated_at
		FROM posts WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Title, &p.Body, &p.Status,
			&p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, total, rows.Err()
}

func (r *ModerationRepo) GetPostByID(ctx context.Context, id uuid.UUID) (*model.Post, error) {
	p := &model.Post{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, title, body, status, deleted_at, created_at, updated_at
		FROM posts WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Body, &p.Status,
		&p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	return p, nil
}

// CreateReport creates a post report with dedup check (one report per user per post).
func (r *ModerationRepo) CreateReport(ctx context.Context, report *model.PostReport) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM post_reports WHERE post_id = $1 AND reporter_id = $2)`,
		report.PostID, report.ReporterID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check duplicate report: %w", err)
	}
	if exists {
		return fmt.Errorf("duplicate report: user already reported this post")
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO post_reports (id, post_id, reporter_id, reason, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		report.ID, report.PostID, report.ReporterID, report.Reason,
		report.Description, report.Status, report.CreatedAt, report.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	return nil
}

func (r *ModerationRepo) CreateModerationCase(ctx context.Context, mc *model.ModerationCase) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO moderation_cases (id, post_id, user_id, status, assigned_to, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		mc.ID, mc.PostID, mc.UserID, mc.Status, mc.AssignedTo, mc.CreatedAt, mc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create moderation case: %w", err)
	}
	return nil
}

func (r *ModerationRepo) UpdateCaseStatus(ctx context.Context, caseID uuid.UUID, newStatus string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE moderation_cases SET status = $2, updated_at = $3 WHERE id = $1`,
		caseID, newStatus, now)
	if err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	return nil
}

func (r *ModerationRepo) CreateModerationAction(ctx context.Context, action *model.ModerationAction) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO moderation_actions (id, case_id, action_type, actor_id, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		action.ID, action.CaseID, action.ActionType, action.ActorID, action.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create moderation action: %w", err)
	}
	return nil
}

func (r *ModerationRepo) CreateBan(ctx context.Context, ban *model.AccountBan) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO account_bans (id, user_id, ban_type, is_permanent, starts_at, ends_at,
			reason, issued_by, revoked_at, revoked_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		ban.ID, ban.UserID, ban.BanType, ban.IsPermanent, ban.StartsAt, ban.EndsAt,
		ban.Reason, ban.IssuedBy, ban.RevokedAt, ban.RevokedBy, ban.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create ban: %w", err)
	}
	return nil
}

// GetActiveBan returns the currently active ban for a user, if any.
func (r *ModerationRepo) GetActiveBan(ctx context.Context, userID uuid.UUID) (*model.AccountBan, error) {
	ban := &model.AccountBan{}
	now := time.Now().UTC()
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, ban_type, is_permanent, starts_at, ends_at,
		       reason, issued_by, revoked_at, revoked_by, created_at
		FROM account_bans
		WHERE user_id = $1 AND revoked_at IS NULL
		  AND starts_at <= $2 AND (ends_at IS NULL OR ends_at > $2)
		ORDER BY created_at DESC LIMIT 1`, userID, now,
	).Scan(
		&ban.ID, &ban.UserID, &ban.BanType, &ban.IsPermanent, &ban.StartsAt,
		&ban.EndsAt, &ban.Reason, &ban.IssuedBy, &ban.RevokedAt, &ban.RevokedBy,
		&ban.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active ban: %w", err)
	}
	return ban, nil
}

// RevokeBan revokes a ban.
func (r *ModerationRepo) RevokeBan(ctx context.Context, banID uuid.UUID, revokedBy uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE account_bans SET revoked_at = $2, revoked_by = $3
		WHERE id = $1`, banID, now, revokedBy)
	if err != nil {
		return fmt.Errorf("revoke ban: %w", err)
	}
	return nil
}

// CountUpheldViolations counts moderation cases with upheld status for a user in the last N days.
func (r *ModerationRepo) CountUpheldViolations(ctx context.Context, userID uuid.UUID, days int) (int, error) {
	var count int
	since := time.Now().UTC().AddDate(0, 0, -days)
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM moderation_cases
		WHERE user_id = $1 AND status = 'upheld' AND created_at >= $2`,
		userID, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count upheld violations: %w", err)
	}
	return count, nil
}

// RecordPostingEvent records a posting event in the rate window table.
func (r *ModerationRepo) RecordPostingEvent(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO posting_rate_windows (id, user_id, posted_at)
		VALUES ($1, $2, $3)`,
		uuid.New(), userID, now,
	)
	if err != nil {
		return fmt.Errorf("record posting event: %w", err)
	}
	return nil
}

// GetPostCountInWindow returns the number of posts by a user in a given time window.
func (r *ModerationRepo) GetPostCountInWindow(ctx context.Context, userID uuid.UUID, window time.Duration) (int, error) {
	var count int
	since := time.Now().UTC().Add(-window)
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posting_rate_windows
		WHERE user_id = $1 AND posted_at >= $2`,
		userID, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get post count in window: %w", err)
	}
	return count, nil
}
