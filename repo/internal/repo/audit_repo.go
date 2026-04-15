package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Create(ctx context.Context, entry *model.AuditEntry) error {
	oldState, _ := marshalJSON(entry.OldState)
	newState, _ := marshalJSON(entry.NewState)
	metadata, _ := marshalJSON(entry.Metadata)

	// Normalize empty IP to nil (PostgreSQL INET doesn't accept empty string)
	var ipAddr *string
	if entry.IPAddr != nil && *entry.IPAddr != "" {
		ipAddr = entry.IPAddr
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_type, actor_id, action, resource, resource_id,
		                        old_state, new_state, reason_code, note, request_id, ip_addr, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::inet, $12)`,
		entry.ActorType, entry.ActorID, entry.Action, entry.Resource, entry.ResourceID,
		oldState, newState, entry.ReasonCode, entry.Note, entry.RequestID, ipAddr, metadata,
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *AuditRepo) List(ctx context.Context, filter AuditFilter) ([]model.AuditLog, int, error) {
	baseQuery := `FROM audit_logs WHERE 1=1`
	var args []interface{}
	argIdx := 1

	if filter.ActorID != nil {
		baseQuery += fmt.Sprintf(` AND actor_id = $%d`, argIdx)
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.Resource != "" {
		baseQuery += fmt.Sprintf(` AND resource = $%d`, argIdx)
		args = append(args, filter.Resource)
		argIdx++
	}
	if filter.Action != "" {
		baseQuery += fmt.Sprintf(` AND action = $%d`, argIdx)
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ResourceID != "" {
		baseQuery += fmt.Sprintf(` AND resource_id = $%d`, argIdx)
		args = append(args, filter.ResourceID)
		argIdx++
	}

	// Count
	var total int
	countQuery := "SELECT count(*) " + baseQuery
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Fetch — cast inet to text because pgx's default codec does not scan
	// PostgreSQL INET directly into *string; host() yields plain text.
	query := "SELECT id, actor_type, actor_id, action, resource, resource_id, old_state, new_state, reason_code, note, request_id, host(ip_addr) AS ip_addr, metadata, created_at " +
		baseQuery + fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var l model.AuditLog
		if err := rows.Scan(&l.ID, &l.ActorType, &l.ActorID, &l.Action, &l.Resource,
			&l.ResourceID, &l.OldState, &l.NewState, &l.ReasonCode, &l.Note,
			&l.RequestID, &l.IPAddr, &l.Metadata, &l.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

type AuditFilter struct {
	ActorID    *string
	Resource   string
	Action     string
	ResourceID string
	Limit      int
	Offset     int
}

func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
