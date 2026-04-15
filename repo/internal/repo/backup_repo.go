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

type BackupRepo struct {
	pool *pgxpool.Pool
}

func NewBackupRepo(pool *pgxpool.Pool) *BackupRepo {
	return &BackupRepo{pool: pool}
}

func (r *BackupRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *BackupRepo) CreateBackupRun(ctx context.Context, br *model.BackupRun) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO backup_runs (id, status, artifact_path, checksum, encrypted,
			size_bytes, started_at, completed_at, error, retention_days, expires_at,
			triggered_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		br.ID, br.Status, br.ArtifactPath, br.Checksum, br.Encrypted,
		br.SizeBytes, br.StartedAt, br.CompletedAt, br.Error, br.RetentionDays,
		br.ExpiresAt, br.TriggeredBy, br.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create backup run: %w", err)
	}
	return nil
}

func (r *BackupRepo) CompleteBackupRun(ctx context.Context, id uuid.UUID, artifactPath, checksum string, sizeBytes int64) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE backup_runs SET status = 'completed', artifact_path = $2,
			checksum = $3, size_bytes = $4, completed_at = $5
		WHERE id = $1`, id, artifactPath, checksum, sizeBytes, now)
	if err != nil {
		return fmt.Errorf("complete backup run: %w", err)
	}
	return nil
}

func (r *BackupRepo) CreateRestoreRun(ctx context.Context, rr *model.RestoreRun) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO restore_runs (id, backup_run_id, status, is_dry_run, reason,
			initiated_by, started_at, completed_at, error, validation_result,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		rr.ID, rr.BackupRunID, rr.Status, rr.IsDryRun, rr.Reason,
		rr.InitiatedBy, rr.StartedAt, rr.CompletedAt, rr.Error,
		rr.ValidationResult, rr.CreatedAt, rr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create restore run: %w", err)
	}
	return nil
}

func (r *BackupRepo) UpdateRestoreRun(ctx context.Context, id uuid.UUID, status string, errMsg *string, validationResult []byte) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE restore_runs SET status = $2, error = $3, validation_result = $4,
			completed_at = $5, updated_at = $5
		WHERE id = $1`, id, status, errMsg, validationResult, now)
	if err != nil {
		return fmt.Errorf("update restore run: %w", err)
	}
	return nil
}

func (r *BackupRepo) CreateArchiveRun(ctx context.Context, ar *model.ArchiveRun) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO archive_runs (id, archive_type, status, threshold_date, total_rows,
			archived_rows, last_cursor, chunk_size, started_at, completed_at, error,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		ar.ID, ar.ArchiveType, ar.Status, ar.ThresholdDate, ar.TotalRows,
		ar.ArchivedRows, ar.LastCursor, ar.ChunkSize, ar.StartedAt,
		ar.CompletedAt, ar.Error, ar.CreatedAt, ar.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create archive run: %w", err)
	}
	return nil
}

func (r *BackupRepo) UpdateArchiveRunByFields(ctx context.Context, id uuid.UUID, status string, archivedRows int, lastCursor *string, errMsg *string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE archive_runs SET status = $2, archived_rows = $3, last_cursor = $4,
			error = $5, completed_at = $6, updated_at = $6
		WHERE id = $1`, id, status, archivedRows, lastCursor, errMsg, now)
	if err != nil {
		return fmt.Errorf("update archive run: %w", err)
	}
	return nil
}

// UpdateBackupRun updates a backup run record.
func (r *BackupRepo) UpdateBackupRun(ctx context.Context, br *model.BackupRun) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE backup_runs SET status = $2, artifact_path = $3, checksum = $4,
			size_bytes = $5, completed_at = $6, error = $7
		WHERE id = $1`,
		br.ID, br.Status, br.ArtifactPath, br.Checksum,
		br.SizeBytes, br.CompletedAt, br.Error,
	)
	if err != nil {
		return fmt.Errorf("update backup run: %w", err)
	}
	return nil
}

// UpdateArchiveRun updates an archive run record.
func (r *BackupRepo) UpdateArchiveRun(ctx context.Context, ar *model.ArchiveRun) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE archive_runs SET status = $2, archived_rows = $3, last_cursor = $4,
			completed_at = $5, error = $6, updated_at = $7
		WHERE id = $1`,
		ar.ID, ar.Status, ar.ArchivedRows, ar.LastCursor,
		ar.CompletedAt, ar.Error, ar.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update archive run: %w", err)
	}
	return nil
}

// CreateArchiveLookupProjection inserts into the archive lookup projection table.
func (r *BackupRepo) CreateArchiveLookupProjection(ctx context.Context, archiveRunID uuid.UUID, entityType string, entityID uuid.UUID, archivedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO archive_lookup_projection (id, archive_run_id, entity_type, entity_id, archived_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), archiveRunID, entityType, entityID, archivedAt,
	)
	if err != nil {
		return fmt.Errorf("create archive lookup projection: %w", err)
	}
	return nil
}

func (r *BackupRepo) ListBackupRuns(ctx context.Context, limit, offset int) ([]model.BackupRun, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM backup_runs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count backup runs: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, status, artifact_path, checksum, encrypted, size_bytes,
		       started_at, completed_at, error, retention_days, expires_at,
		       triggered_by, created_at
		FROM backup_runs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list backup runs: %w", err)
	}
	defer rows.Close()

	var runs []model.BackupRun
	for rows.Next() {
		var br model.BackupRun
		if err := rows.Scan(
			&br.ID, &br.Status, &br.ArtifactPath, &br.Checksum, &br.Encrypted,
			&br.SizeBytes, &br.StartedAt, &br.CompletedAt, &br.Error,
			&br.RetentionDays, &br.ExpiresAt, &br.TriggeredBy, &br.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan backup run: %w", err)
		}
		runs = append(runs, br)
	}
	return runs, total, rows.Err()
}

func (r *BackupRepo) ListArchiveRuns(ctx context.Context, limit, offset int) ([]model.ArchiveRun, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM archive_runs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count archive runs: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, archive_type, status, threshold_date, total_rows, archived_rows,
		       last_cursor, chunk_size, started_at, completed_at, error,
		       created_at, updated_at
		FROM archive_runs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list archive runs: %w", err)
	}
	defer rows.Close()

	var runs []model.ArchiveRun
	for rows.Next() {
		var ar model.ArchiveRun
		if err := rows.Scan(
			&ar.ID, &ar.ArchiveType, &ar.Status, &ar.ThresholdDate, &ar.TotalRows,
			&ar.ArchivedRows, &ar.LastCursor, &ar.ChunkSize, &ar.StartedAt,
			&ar.CompletedAt, &ar.Error, &ar.CreatedAt, &ar.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan archive run: %w", err)
		}
		runs = append(runs, ar)
	}
	return runs, total, rows.Err()
}

// GetBackupRunByID retrieves a backup run by ID.
func (r *BackupRepo) GetBackupRunByID(ctx context.Context, id uuid.UUID) (*model.BackupRun, error) {
	br := &model.BackupRun{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, status, artifact_path, checksum, encrypted, size_bytes,
		       started_at, completed_at, error, retention_days, expires_at,
		       triggered_by, created_at
		FROM backup_runs WHERE id = $1`, id,
	).Scan(
		&br.ID, &br.Status, &br.ArtifactPath, &br.Checksum, &br.Encrypted,
		&br.SizeBytes, &br.StartedAt, &br.CompletedAt, &br.Error,
		&br.RetentionDays, &br.ExpiresAt, &br.TriggeredBy, &br.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get backup run: %w", err)
	}
	return br, nil
}
