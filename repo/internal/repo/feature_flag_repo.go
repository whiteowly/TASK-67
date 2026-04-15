package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FeatureFlagRepo struct {
	pool *pgxpool.Pool
}

func NewFeatureFlagRepo(pool *pgxpool.Pool) *FeatureFlagRepo {
	return &FeatureFlagRepo{pool: pool}
}

func (r *FeatureFlagRepo) ListAll(ctx context.Context) ([]model.FeatureFlag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, key, enabled, description, cohort_percent, target_roles, target_domains,
		       updated_by, version, created_at, updated_at
		FROM feature_flags ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flags []model.FeatureFlag
	for rows.Next() {
		var f model.FeatureFlag
		if err := rows.Scan(&f.ID, &f.Key, &f.Enabled, &f.Description, &f.CohortPercent,
			&f.TargetRoles, &f.TargetDomains, &f.UpdatedBy, &f.Version,
			&f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (r *FeatureFlagRepo) GetByKey(ctx context.Context, key string) (*model.FeatureFlag, error) {
	f := &model.FeatureFlag{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, key, enabled, description, cohort_percent, target_roles, target_domains,
		       updated_by, version, created_at, updated_at
		FROM feature_flags WHERE key = $1`, key,
	).Scan(&f.ID, &f.Key, &f.Enabled, &f.Description, &f.CohortPercent,
		&f.TargetRoles, &f.TargetDomains, &f.UpdatedBy, &f.Version,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

func (r *FeatureFlagRepo) Create(ctx context.Context, f *model.FeatureFlag) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO feature_flags (id, key, enabled, description, cohort_percent,
			target_roles, target_domains, metadata, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		f.ID, f.Key, f.Enabled, f.Description, f.CohortPercent,
		f.TargetRoles, f.TargetDomains, "{}", f.UpdatedBy, f.CreatedAt, f.UpdatedAt)
	return err
}

func (r *FeatureFlagRepo) Update(ctx context.Context, key string, enabled bool, cohortPercent int, updatedBy uuid.UUID, expectedVersion int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE feature_flags SET enabled = $2, cohort_percent = $3, updated_by = $4,
			version = version + 1, updated_at = now()
		WHERE key = $1 AND version = $5`,
		key, enabled, cohortPercent, updatedBy, expectedVersion)
	if err != nil {
		return fmt.Errorf("update feature flag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("feature flag not found or version conflict")
	}
	return nil
}
