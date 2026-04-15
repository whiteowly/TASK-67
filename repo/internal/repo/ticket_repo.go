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

type TicketRepo struct {
	pool *pgxpool.Pool
}

func NewTicketRepo(pool *pgxpool.Pool) *TicketRepo {
	return &TicketRepo{pool: pool}
}

func (r *TicketRepo) Pool() *pgxpool.Pool { return r.pool }

// CreateTicket inserts a ticket with auto-generated ticket number.
func (r *TicketRepo) CreateTicket(ctx context.Context, ticket *model.Ticket) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Auto-generate ticket number
	if ticket.TicketNumber == "" {
		today := time.Now().UTC().Format("20060102")
		var seq int
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) + 1 FROM tickets
			WHERE ticket_number LIKE $1`, "TKT-"+today+"-%",
		).Scan(&seq)
		if err != nil {
			return fmt.Errorf("generate ticket number: %w", err)
		}
		ticket.TicketNumber = fmt.Sprintf("TKT-%s-%05d", today, seq)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO tickets (id, ticket_number, ticket_type, title, description, priority,
			status, source_type, source_id, assigned_to, resolved_at, resolution_code,
			resolution_summary, closed_at, closed_by, sla_response_due, sla_resolution_due,
			sla_response_met, sla_resolution_met, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		ticket.ID, ticket.TicketNumber, ticket.TicketType, ticket.Title, ticket.Description,
		ticket.Priority, ticket.Status, ticket.SourceType, ticket.SourceID, ticket.AssignedTo,
		ticket.ResolvedAt, ticket.ResolutionCode, ticket.ResolutionSummary, ticket.ClosedAt,
		ticket.ClosedBy, ticket.SLAResponseDue, ticket.SLAResolutionDue, ticket.SLAResponseMet,
		ticket.SLAResolutionMet, ticket.CreatedBy, ticket.CreatedAt, ticket.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert ticket: %w", err)
	}

	// Insert initial status history
	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (id, ticket_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, NULL, $3, $4, $5)`,
		uuid.New(), ticket.ID, ticket.Status, ticket.CreatedBy, ticket.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert ticket status history: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateTicketStatus updates a ticket's status and inserts history.
func (r *TicketRepo) UpdateTicketStatus(ctx context.Context, ticketID uuid.UUID, oldStatus, newStatus string, changedBy *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE tickets SET status = $2, updated_at = $3 WHERE id = $1`,
		ticketID, newStatus, now)
	if err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_status_history (id, ticket_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), ticketID, oldStatus, newStatus, changedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert ticket status history: %w", err)
	}

	return tx.Commit(ctx)
}

// AssignTicket assigns a ticket to a user.
func (r *TicketRepo) AssignTicket(ctx context.Context, ticketID, assigneeID uuid.UUID, assignedBy *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE tickets SET assigned_to = $2, updated_at = $3 WHERE id = $1`,
		ticketID, assigneeID, now)
	if err != nil {
		return fmt.Errorf("update ticket assignment: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ticket_assignments (id, ticket_id, assigned_to, assigned_by, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), ticketID, assigneeID, assignedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert ticket assignment: %w", err)
	}

	return tx.Commit(ctx)
}

// AddComment adds a comment to a ticket.
func (r *TicketRepo) AddComment(ctx context.Context, comment *model.TicketComment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ticket_comments (id, ticket_id, author_id, body, is_internal, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		comment.ID, comment.TicketID, comment.AuthorID, comment.Body,
		comment.IsInternal, comment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("add comment: %w", err)
	}
	return nil
}

// TicketFilter holds filter criteria for listing tickets.
type TicketFilter struct {
	Status     string
	TicketType string
	AssignedTo *uuid.UUID
	CreatedBy  *uuid.UUID
	Limit      int
	Offset     int
}

// ListTickets returns paginated tickets with optional filters.
func (r *TicketRepo) ListTickets(ctx context.Context, f TicketFilter) ([]model.Ticket, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if f.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.TicketType != "" {
		where += fmt.Sprintf(" AND ticket_type = $%d", argIdx)
		args = append(args, f.TicketType)
		argIdx++
	}
	if f.AssignedTo != nil {
		where += fmt.Sprintf(" AND assigned_to = $%d", argIdx)
		args = append(args, *f.AssignedTo)
		argIdx++
	}
	if f.CreatedBy != nil {
		where += fmt.Sprintf(" AND created_by = $%d", argIdx)
		args = append(args, *f.CreatedBy)
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM tickets %s", where)
	err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	selectQ := fmt.Sprintf(`
		SELECT id, ticket_number, ticket_type, title, description, priority, status,
		       source_type, source_id, assigned_to, resolved_at, resolution_code,
		       resolution_summary, closed_at, closed_by, sla_response_due, sla_resolution_due,
		       sla_response_met, sla_resolution_met, created_by, created_at, updated_at
		FROM tickets %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	var tickets []model.Ticket
	for rows.Next() {
		var t model.Ticket
		if err := rows.Scan(
			&t.ID, &t.TicketNumber, &t.TicketType, &t.Title, &t.Description,
			&t.Priority, &t.Status, &t.SourceType, &t.SourceID, &t.AssignedTo,
			&t.ResolvedAt, &t.ResolutionCode, &t.ResolutionSummary, &t.ClosedAt,
			&t.ClosedBy, &t.SLAResponseDue, &t.SLAResolutionDue, &t.SLAResponseMet,
			&t.SLAResolutionMet, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, total, rows.Err()
}

func (r *TicketRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (*model.Ticket, error) {
	t := &model.Ticket{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, ticket_number, ticket_type, title, description, priority, status,
		       source_type, source_id, assigned_to, resolved_at, resolution_code,
		       resolution_summary, closed_at, closed_by, sla_response_due, sla_resolution_due,
		       sla_response_met, sla_resolution_met, created_by, created_at, updated_at
		FROM tickets WHERE id = $1`, id,
	).Scan(
		&t.ID, &t.TicketNumber, &t.TicketType, &t.Title, &t.Description,
		&t.Priority, &t.Status, &t.SourceType, &t.SourceID, &t.AssignedTo,
		&t.ResolvedAt, &t.ResolutionCode, &t.ResolutionSummary, &t.ClosedAt,
		&t.ClosedBy, &t.SLAResponseDue, &t.SLAResolutionDue, &t.SLAResponseMet,
		&t.SLAResolutionMet, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	return t, nil
}

// GenerateTicketNumber generates a sequential ticket number.
func (r *TicketRepo) GenerateTicketNumber(ctx context.Context, prefix string) (string, error) {
	today := time.Now().UTC().Format("20060102")
	var seq int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) + 1 FROM tickets
		WHERE ticket_number LIKE $1`, prefix+"-"+today+"-%",
	).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate ticket number: %w", err)
	}
	return fmt.Sprintf("%s-%s-%05d", prefix, today, seq), nil
}

// Create is an alias for CreateTicket used by services.
func (r *TicketRepo) Create(ctx context.Context, ticket *model.Ticket) error {
	return r.CreateTicket(ctx, ticket)
}

// GetByID is an alias for GetTicketByID used by services.
func (r *TicketRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Ticket, error) {
	return r.GetTicketByID(ctx, id)
}

// Update updates a ticket record.
func (r *TicketRepo) Update(ctx context.Context, ticket *model.Ticket) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tickets SET ticket_type = $2, title = $3, description = $4, priority = $5,
			status = $6, assigned_to = $7, resolved_at = $8, resolution_code = $9,
			resolution_summary = $10, closed_at = $11, closed_by = $12,
			sla_response_due = $13, sla_resolution_due = $14,
			sla_response_met = $15, sla_resolution_met = $16, updated_at = $17
		WHERE id = $1`,
		ticket.ID, ticket.TicketType, ticket.Title, ticket.Description, ticket.Priority,
		ticket.Status, ticket.AssignedTo, ticket.ResolvedAt, ticket.ResolutionCode,
		ticket.ResolutionSummary, ticket.ClosedAt, ticket.ClosedBy,
		ticket.SLAResponseDue, ticket.SLAResolutionDue,
		ticket.SLAResponseMet, ticket.SLAResolutionMet, ticket.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	return nil
}

// CreateComment is an alias for AddComment used by services.
func (r *TicketRepo) CreateComment(ctx context.Context, comment *model.TicketComment) error {
	return r.AddComment(ctx, comment)
}

// FindSLABreaches returns tickets with breached SLA deadlines.
func (r *TicketRepo) FindSLABreaches(ctx context.Context, now time.Time) ([]model.Ticket, error) {
	return r.ListOverdueSLATickets(ctx)
}

// List returns paginated tickets.
func (r *TicketRepo) List(ctx context.Context, limit, offset int) ([]model.Ticket, int, error) {
	return r.ListTickets(ctx, TicketFilter{Limit: limit, Offset: offset})
}

// CreateSLAEvent records an SLA event for a ticket.
func (r *TicketRepo) CreateSLAEvent(ctx context.Context, ticketID uuid.UUID, eventType string, dueAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ticket_sla_events (id, ticket_id, event_type, due_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), ticketID, eventType, dueAt, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("create SLA event: %w", err)
	}
	return nil
}

// ListOverdueSLATickets returns tickets that have overdue SLA deadlines.
func (r *TicketRepo) ListOverdueSLATickets(ctx context.Context) ([]model.Ticket, error) {
	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, `
		SELECT id, ticket_number, ticket_type, title, description, priority, status,
		       source_type, source_id, assigned_to, resolved_at, resolution_code,
		       resolution_summary, closed_at, closed_by, sla_response_due, sla_resolution_due,
		       sla_response_met, sla_resolution_met, created_by, created_at, updated_at
		FROM tickets
		WHERE status NOT IN ('resolved', 'closed')
		  AND (
			(sla_response_due IS NOT NULL AND sla_response_due < $1 AND (sla_response_met IS NULL OR sla_response_met = false))
			OR
			(sla_resolution_due IS NOT NULL AND sla_resolution_due < $1 AND (sla_resolution_met IS NULL OR sla_resolution_met = false))
		  )
		ORDER BY LEAST(COALESCE(sla_response_due, $1), COALESCE(sla_resolution_due, $1)) ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list overdue SLA tickets: %w", err)
	}
	defer rows.Close()

	var tickets []model.Ticket
	for rows.Next() {
		var t model.Ticket
		if err := rows.Scan(
			&t.ID, &t.TicketNumber, &t.TicketType, &t.Title, &t.Description,
			&t.Priority, &t.Status, &t.SourceType, &t.SourceID, &t.AssignedTo,
			&t.ResolvedAt, &t.ResolutionCode, &t.ResolutionSummary, &t.ClosedAt,
			&t.ClosedBy, &t.SLAResponseDue, &t.SLAResolutionDue, &t.SLAResponseMet,
			&t.SLAResolutionMet, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}
