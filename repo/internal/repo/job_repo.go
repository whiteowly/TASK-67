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

type JobRepo struct {
	pool *pgxpool.Pool
}

func NewJobRepo(pool *pgxpool.Pool) *JobRepo {
	return &JobRepo{pool: pool}
}

func (r *JobRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *JobRepo) EnqueueJob(ctx context.Context, job *model.Job) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO job_queue (id, job_type, payload, status, priority, max_retries,
			retry_count, run_at, started_at, completed_at, last_error, lease_token,
			lease_expires, progress, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		job.ID, job.JobType, job.Payload, job.Status, job.Priority, job.MaxRetries,
		job.RetryCount, job.RunAt, job.StartedAt, job.CompletedAt, job.LastError,
		job.LeaseToken, job.LeaseExpires, job.Progress, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}
	return nil
}

// AcquireJob acquires the next available job using SELECT FOR UPDATE SKIP LOCKED.
func (r *JobRepo) AcquireJob(ctx context.Context, leaseDuration time.Duration) (*model.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	leaseToken := uuid.New()
	leaseExpires := now.Add(leaseDuration)

	job := &model.Job{}
	err = tx.QueryRow(ctx, `
		SELECT id, job_type, payload, status, priority, max_retries, retry_count,
		       run_at, started_at, completed_at, last_error, lease_token, lease_expires,
		       progress, created_at, updated_at
		FROM job_queue
		WHERE status = 'pending' AND run_at <= $1
		ORDER BY priority DESC, run_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`, now,
	).Scan(
		&job.ID, &job.JobType, &job.Payload, &job.Status, &job.Priority,
		&job.MaxRetries, &job.RetryCount, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.LastError, &job.LeaseToken, &job.LeaseExpires,
		&job.Progress, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select job for update: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE job_queue SET status = 'running', started_at = $2, lease_token = $3,
			lease_expires = $4, updated_at = $2
		WHERE id = $1`, job.ID, now, leaseToken, leaseExpires)
	if err != nil {
		return nil, fmt.Errorf("update job lease: %w", err)
	}

	// Record attempt
	_, err = tx.Exec(ctx, `
		INSERT INTO job_attempts (id, job_id, attempt, started_at, status, created_at)
		VALUES ($1, $2, $3, $4, 'running', $4)`,
		uuid.New(), job.ID, job.RetryCount+1, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job attempt: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit acquire: %w", err)
	}

	job.Status = "running"
	job.StartedAt = &now
	job.LeaseToken = &leaseToken
	job.LeaseExpires = &leaseExpires
	return job, nil
}

// CompleteJob marks a job as completed.
func (r *JobRepo) CompleteJob(ctx context.Context, jobID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE job_queue SET status = 'completed', completed_at = $2,
			lease_token = NULL, lease_expires = NULL, updated_at = $2
		WHERE id = $1`, jobID, now)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	// Update the latest attempt
	_, err = r.pool.Exec(ctx, `
		UPDATE job_attempts SET status = 'completed', ended_at = $2
		WHERE job_id = $1 AND status = 'running'
		ORDER BY attempt DESC LIMIT 1`, jobID, now)
	if err != nil {
		return fmt.Errorf("update job attempt: %w", err)
	}
	return nil
}

// FailJob marks a job as failed, incrementing retry count.
func (r *JobRepo) FailJob(ctx context.Context, jobID uuid.UUID, errMsg string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE job_queue SET status = CASE
			WHEN retry_count + 1 >= max_retries THEN 'failed'
			ELSE 'pending'
		END,
		retry_count = retry_count + 1,
		last_error = $2,
		lease_token = NULL, lease_expires = NULL,
		updated_at = $3
		WHERE id = $1`, jobID, errMsg, now)
	if err != nil {
		return fmt.Errorf("fail job: %w", err)
	}

	// Update the latest attempt
	_, err = r.pool.Exec(ctx, `
		UPDATE job_attempts SET status = 'failed', ended_at = $2, error = $3
		WHERE job_id = $1 AND status = 'running'`, jobID, now, errMsg)
	if err != nil {
		return fmt.Errorf("update job attempt: %w", err)
	}
	return nil
}

// ListJobs returns paginated jobs filtered by status.
func (r *JobRepo) ListJobs(ctx context.Context, status string, limit, offset int) ([]model.Job, int, error) {
	where := ""
	var args []interface{}
	argIdx := 1

	if status != "" {
		where = fmt.Sprintf("WHERE status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM job_queue %s", where)
	err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count jobs: %w", err)
	}

	selectQ := fmt.Sprintf(`
		SELECT id, job_type, payload, status, priority, max_retries, retry_count,
		       run_at, started_at, completed_at, last_error, lease_token, lease_expires,
		       progress, created_at, updated_at
		FROM job_queue %s
		ORDER BY priority DESC, run_at ASC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []model.Job
	for rows.Next() {
		var j model.Job
		if err := rows.Scan(
			&j.ID, &j.JobType, &j.Payload, &j.Status, &j.Priority,
			&j.MaxRetries, &j.RetryCount, &j.RunAt, &j.StartedAt,
			&j.CompletedAt, &j.LastError, &j.LeaseToken, &j.LeaseExpires,
			&j.Progress, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

// Create is an alias for EnqueueJob used by services.
func (r *JobRepo) Create(ctx context.Context, job *model.Job) error {
	return r.EnqueueJob(ctx, job)
}

// AcquireNext acquires the next available job with a lease token and expiry.
func (r *JobRepo) AcquireNext(ctx context.Context, leaseToken uuid.UUID, leaseExpires time.Time) (*model.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	job := &model.Job{}
	err = tx.QueryRow(ctx, `
		SELECT id, job_type, payload, status, priority, max_retries, retry_count,
		       run_at, started_at, completed_at, last_error, lease_token, lease_expires,
		       progress, created_at, updated_at
		FROM job_queue
		WHERE status = 'pending' AND run_at <= $1
		ORDER BY priority DESC, run_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`, now,
	).Scan(
		&job.ID, &job.JobType, &job.Payload, &job.Status, &job.Priority,
		&job.MaxRetries, &job.RetryCount, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.LastError, &job.LeaseToken, &job.LeaseExpires,
		&job.Progress, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("acquire next job: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE job_queue SET status = 'running', started_at = $2, lease_token = $3,
			lease_expires = $4, updated_at = $2
		WHERE id = $1`, job.ID, now, leaseToken, leaseExpires)
	if err != nil {
		return nil, fmt.Errorf("update job lease: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit acquire: %w", err)
	}

	job.Status = "running"
	job.StartedAt = &now
	job.LeaseToken = &leaseToken
	job.LeaseExpires = &leaseExpires
	return job, nil
}

// CreateAttempt inserts a job attempt record.
func (r *JobRepo) CreateAttempt(ctx context.Context, attempt *model.JobAttempt) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO job_attempts (id, job_id, attempt, started_at, ended_at, status, error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		attempt.ID, attempt.JobID, attempt.Attempt, attempt.StartedAt,
		attempt.EndedAt, attempt.Status, attempt.Error, attempt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create job attempt: %w", err)
	}
	return nil
}

// UpdateAttempt updates a job attempt record.
func (r *JobRepo) UpdateAttempt(ctx context.Context, attempt *model.JobAttempt) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE job_attempts SET ended_at = $2, status = $3, error = $4
		WHERE id = $1`,
		attempt.ID, attempt.EndedAt, attempt.Status, attempt.Error,
	)
	if err != nil {
		return fmt.Errorf("update job attempt: %w", err)
	}
	return nil
}

// GetByID retrieves a job by ID.
func (r *JobRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Job, error) {
	job := &model.Job{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, job_type, payload, status, priority, max_retries, retry_count,
		       run_at, started_at, completed_at, last_error, lease_token, lease_expires,
		       progress, created_at, updated_at
		FROM job_queue WHERE id = $1`, id,
	).Scan(
		&job.ID, &job.JobType, &job.Payload, &job.Status, &job.Priority,
		&job.MaxRetries, &job.RetryCount, &job.RunAt, &job.StartedAt,
		&job.CompletedAt, &job.LastError, &job.LeaseToken, &job.LeaseExpires,
		&job.Progress, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return job, nil
}

// Update updates a job record.
func (r *JobRepo) Update(ctx context.Context, job *model.Job) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE job_queue SET status = $2, priority = $3, retry_count = $4,
			run_at = $5, started_at = $6, completed_at = $7, last_error = $8,
			lease_token = $9, lease_expires = $10, progress = $11, updated_at = $12
		WHERE id = $1`,
		job.ID, job.Status, job.Priority, job.RetryCount,
		job.RunAt, job.StartedAt, job.CompletedAt, job.LastError,
		job.LeaseToken, job.LeaseExpires, job.Progress, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

// List returns paginated jobs (all statuses).
func (r *JobRepo) List(ctx context.Context, limit, offset int) ([]model.Job, int, error) {
	return r.ListJobs(ctx, "", limit, offset)
}

// GetScheduledJobs returns all enabled scheduled jobs.
func (r *JobRepo) GetScheduledJobs(ctx context.Context) ([]model.ScheduledJob, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, job_type, cron_expr, enabled, last_run, next_run, created_at, updated_at
		FROM scheduled_jobs WHERE enabled = true
		ORDER BY next_run ASC NULLS LAST`)
	if err != nil {
		return nil, fmt.Errorf("get scheduled jobs: %w", err)
	}
	defer rows.Close()

	var jobs []model.ScheduledJob
	for rows.Next() {
		var j model.ScheduledJob
		if err := rows.Scan(
			&j.ID, &j.Name, &j.JobType, &j.CronExpr, &j.Enabled,
			&j.LastRun, &j.NextRun, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// CleanExpiredLeases resets jobs with expired leases back to pending status.
func (r *JobRepo) CleanExpiredLeases(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE job_queue SET status = 'pending', lease_token = NULL,
			lease_expires = NULL, updated_at = $1
		WHERE status = 'running' AND lease_expires < $1`, now)
	if err != nil {
		return 0, fmt.Errorf("clean expired leases: %w", err)
	}
	return tag.RowsAffected(), nil
}
