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

type ImportRepo struct {
	pool *pgxpool.Pool
}

func NewImportRepo(pool *pgxpool.Pool) *ImportRepo {
	return &ImportRepo{pool: pool}
}

func (r *ImportRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *ImportRepo) CreateFileArtifact(ctx context.Context, fa *model.FileArtifact) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO file_artifacts (id, filename, file_type, mime_type, size_bytes,
			checksum, storage_path, artifact_type, uploaded_by, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		fa.ID, fa.Filename, fa.FileType, fa.MimeType, fa.SizeBytes,
		fa.Checksum, fa.StoragePath, fa.ArtifactType, fa.UploadedBy,
		fa.ExpiresAt, fa.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create file artifact: %w", err)
	}
	return nil
}

func (r *ImportRepo) GetFileArtifactByID(ctx context.Context, id uuid.UUID) (*model.FileArtifact, error) {
	a := &model.FileArtifact{}
	err := r.pool.QueryRow(ctx, `SELECT id, filename, file_type, mime_type, size_bytes, checksum, storage_path, artifact_type, uploaded_by, expires_at, created_at FROM file_artifacts WHERE id = $1`, id).Scan(&a.ID, &a.Filename, &a.FileType, &a.MimeType, &a.SizeBytes, &a.Checksum, &a.StoragePath, &a.ArtifactType, &a.UploadedBy, &a.ExpiresAt, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// CheckDuplicateImport checks if an import with the same checksum and template type exists.
func (r *ImportRepo) CheckDuplicateImport(ctx context.Context, checksum, templateType string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM import_jobs ij
			JOIN file_artifacts fa ON fa.id = ij.file_artifact_id
			WHERE fa.checksum = $1 AND ij.template_type = $2
		)`, checksum, templateType,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check duplicate import: %w", err)
	}
	return exists, nil
}

func (r *ImportRepo) CreateImportJob(ctx context.Context, job *model.ImportJob) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO import_jobs (id, file_artifact_id, template_type, status, uploaded_by,
			total_rows, valid_rows, error_rows, applied_rows, force_reprocess, force_reason,
			error_summary, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		job.ID, job.FileArtifactID, job.TemplateType, job.Status, job.UploadedBy,
		job.TotalRows, job.ValidRows, job.ErrorRows, job.AppliedRows,
		job.ForceReprocess, job.ForceReason, job.ErrorSummary,
		job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create import job: %w", err)
	}
	return nil
}

func (r *ImportRepo) UpdateImportJobStatus(ctx context.Context, jobID uuid.UUID, status string, totalRows, validRows, errorRows, appliedRows *int, errorSummary []byte) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE import_jobs SET status = $2, total_rows = $3, valid_rows = $4,
			error_rows = $5, applied_rows = $6, error_summary = $7, updated_at = $8
		WHERE id = $1`,
		jobID, status, totalRows, validRows, errorRows, appliedRows, errorSummary, now,
	)
	if err != nil {
		return fmt.Errorf("update import job status: %w", err)
	}
	return nil
}

// CreateImportRows bulk-inserts import rows.
func (r *ImportRepo) CreateImportRows(ctx context.Context, rows []model.ImportRow) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, row := range rows {
		_, err = tx.Exec(ctx, `
			INSERT INTO import_rows (id, import_job_id, row_number, raw_data, is_valid, errors, applied, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			row.ID, row.ImportJobID, row.RowNumber, row.RawData,
			row.IsValid, row.Errors, row.Applied, row.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert import row %d: %w", row.RowNumber, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *ImportRepo) CreateExportJob(ctx context.Context, job *model.ExportJob) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO export_jobs (id, export_type, format, status, filters,
			file_artifact_id, requested_by, total_rows, started_at, completed_at,
			error, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		job.ID, job.ExportType, job.Format, job.Status, job.Filters,
		job.FileArtifactID, job.RequestedBy, job.TotalRows, job.StartedAt,
		job.CompletedAt, job.Error, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create export job: %w", err)
	}
	return nil
}

func (r *ImportRepo) UpdateExportJobStatus(ctx context.Context, jobID uuid.UUID, status string, fileArtifactID *uuid.UUID, totalRows *int, errMsg *string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE export_jobs SET status = $2, file_artifact_id = $3, total_rows = $4,
			error = $5, completed_at = $6, updated_at = $6
		WHERE id = $1`,
		jobID, status, fileArtifactID, totalRows, errMsg, now,
	)
	if err != nil {
		return fmt.Errorf("update export job status: %w", err)
	}
	return nil
}

func (r *ImportRepo) ListImportJobs(ctx context.Context, limit, offset int) ([]model.ImportJob, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM import_jobs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count import jobs: %w", err)
	}

	dbRows, err := r.pool.Query(ctx, `
		SELECT id, file_artifact_id, template_type, status, uploaded_by,
		       total_rows, valid_rows, error_rows, applied_rows, force_reprocess,
		       force_reason, error_summary, created_at, updated_at
		FROM import_jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list import jobs: %w", err)
	}
	defer dbRows.Close()

	var jobs []model.ImportJob
	for dbRows.Next() {
		var j model.ImportJob
		if err := dbRows.Scan(
			&j.ID, &j.FileArtifactID, &j.TemplateType, &j.Status, &j.UploadedBy,
			&j.TotalRows, &j.ValidRows, &j.ErrorRows, &j.AppliedRows,
			&j.ForceReprocess, &j.ForceReason, &j.ErrorSummary,
			&j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan import job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, total, dbRows.Err()
}

func (r *ImportRepo) GetExportJobByID(ctx context.Context, id uuid.UUID) (*model.ExportJob, error) {
	j := &model.ExportJob{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, export_type, format, status, filters, file_artifact_id,
		       requested_by, total_rows, started_at, completed_at, error, created_at, updated_at
		FROM export_jobs WHERE id = $1`, id,
	).Scan(
		&j.ID, &j.ExportType, &j.Format, &j.Status, &j.Filters,
		&j.FileArtifactID, &j.RequestedBy, &j.TotalRows,
		&j.StartedAt, &j.CompletedAt, &j.Error, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get export job: %w", err)
	}
	return j, nil
}

func (r *ImportRepo) ListExportJobs(ctx context.Context, limit, offset int) ([]model.ExportJob, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM export_jobs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count export jobs: %w", err)
	}

	dbRows, err := r.pool.Query(ctx, `
		SELECT id, export_type, format, status, filters, file_artifact_id,
		       requested_by, total_rows, started_at, completed_at, error,
		       created_at, updated_at
		FROM export_jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list export jobs: %w", err)
	}
	defer dbRows.Close()

	var jobs []model.ExportJob
	for dbRows.Next() {
		var j model.ExportJob
		if err := dbRows.Scan(
			&j.ID, &j.ExportType, &j.Format, &j.Status, &j.Filters,
			&j.FileArtifactID, &j.RequestedBy, &j.TotalRows, &j.StartedAt,
			&j.CompletedAt, &j.Error, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan export job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, total, dbRows.Err()
}

// GetByChecksum returns an import job matching the given file checksum, or nil.
func (r *ImportRepo) GetByChecksum(ctx context.Context, checksum string) (*model.ImportJob, error) {
	j := &model.ImportJob{}
	err := r.pool.QueryRow(ctx, `
		SELECT ij.id, ij.file_artifact_id, ij.template_type, ij.status, ij.uploaded_by,
		       ij.total_rows, ij.valid_rows, ij.error_rows, ij.applied_rows,
		       ij.force_reprocess, ij.force_reason, ij.error_summary,
		       ij.created_at, ij.updated_at
		FROM import_jobs ij
		JOIN file_artifacts fa ON fa.id = ij.file_artifact_id
		WHERE fa.checksum = $1
		ORDER BY ij.created_at DESC LIMIT 1`, checksum,
	).Scan(
		&j.ID, &j.FileArtifactID, &j.TemplateType, &j.Status, &j.UploadedBy,
		&j.TotalRows, &j.ValidRows, &j.ErrorRows, &j.AppliedRows,
		&j.ForceReprocess, &j.ForceReason, &j.ErrorSummary,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get import by checksum: %w", err)
	}
	return j, nil
}

// UpdateImportJob updates an import job record.
func (r *ImportRepo) UpdateImportJob(ctx context.Context, job *model.ImportJob) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE import_jobs SET status = $2, total_rows = $3, valid_rows = $4,
			error_rows = $5, applied_rows = $6, error_summary = $7, updated_at = $8
		WHERE id = $1`,
		job.ID, job.Status, job.TotalRows, job.ValidRows,
		job.ErrorRows, job.AppliedRows, job.ErrorSummary, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update import job: %w", err)
	}
	return nil
}

// GetImportJobByID retrieves an import job by ID.
func (r *ImportRepo) GetImportJobByID(ctx context.Context, id uuid.UUID) (*model.ImportJob, error) {
	j := &model.ImportJob{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, file_artifact_id, template_type, status, uploaded_by,
		       total_rows, valid_rows, error_rows, applied_rows, force_reprocess,
		       force_reason, error_summary, created_at, updated_at
		FROM import_jobs WHERE id = $1`, id,
	).Scan(
		&j.ID, &j.FileArtifactID, &j.TemplateType, &j.Status, &j.UploadedBy,
		&j.TotalRows, &j.ValidRows, &j.ErrorRows, &j.AppliedRows,
		&j.ForceReprocess, &j.ForceReason, &j.ErrorSummary,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	return j, nil
}
