package repo

import (
	"context"
	"fmt"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepo struct {
	pool *pgxpool.Pool
}

func NewRoleRepo(pool *pgxpool.Pool) *RoleRepo {
	return &RoleRepo{pool: pool}
}

func (r *RoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	role := &model.Role{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, created_at FROM roles WHERE name = $1`,
		name,
	).Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get role by name: %w", err)
	}
	return role, nil
}

func (r *RoleRepo) AssignRole(ctx context.Context, userID, roleID uuid.UUID, assignedBy *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_role_assignments (user_id, role_id, assigned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role_id) WHERE effective_until IS NULL DO NOTHING`,
		userID, roleID, assignedBy,
	)
	return err
}

func (r *RoleRepo) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.name FROM roles r
		JOIN user_role_assignments ura ON ura.role_id = r.id
		WHERE ura.user_id = $1 AND ura.effective_until IS NULL`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}

func (r *RoleRepo) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_role_assignments SET effective_until = now()
		WHERE user_id = $1 AND role_id = $2 AND effective_until IS NULL`,
		userID, roleID,
	)
	return err
}

func (r *RoleRepo) ListAll(ctx context.Context) ([]model.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, description, created_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
