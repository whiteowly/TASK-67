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

type ConfigRepo struct {
	pool *pgxpool.Pool
}

func NewConfigRepo(pool *pgxpool.Pool) *ConfigRepo {
	return &ConfigRepo{pool: pool}
}

func (r *ConfigRepo) ListAll(ctx context.Context) ([]model.SystemConfig, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, key, value, value_type, description, updated_by, version, created_at, updated_at
		FROM system_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []model.SystemConfig
	for rows.Next() {
		var c model.SystemConfig
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.ValueType, &c.Description,
			&c.UpdatedBy, &c.Version, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *ConfigRepo) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	c := &model.SystemConfig{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, key, value, value_type, description, updated_by, version, created_at, updated_at
		FROM system_config WHERE key = $1`, key,
	).Scan(&c.ID, &c.Key, &c.Value, &c.ValueType, &c.Description,
		&c.UpdatedBy, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// Update uses optimistic locking via version.
func (r *ConfigRepo) Update(ctx context.Context, key, value string, updatedBy uuid.UUID, expectedVersion int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE system_config SET value = $2, updated_by = $3, version = version + 1, updated_at = now()
		WHERE key = $1 AND version = $4`,
		key, value, updatedBy, expectedVersion)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("config key not found or version conflict")
	}
	return nil
}
