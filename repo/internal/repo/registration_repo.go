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

type RegistrationRepo struct {
	pool *pgxpool.Pool
}

func NewRegistrationRepo(pool *pgxpool.Pool) *RegistrationRepo {
	return &RegistrationRepo{pool: pool}
}

// CreateRegistration inserts a registration and deducts a seat from inventory
// using SELECT FOR UPDATE to prevent overselling.
func (r *RegistrationRepo) CreateRegistration(ctx context.Context, reg *model.SessionRegistration, history *model.RegistrationStatusHistory) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock and deduct seat
	var available int
	err = tx.QueryRow(ctx, `
		SELECT available_seats FROM session_seat_inventory
		WHERE session_id = $1 FOR UPDATE`, reg.SessionID).Scan(&available)
	if err != nil {
		return fmt.Errorf("lock seat inventory: %w", err)
	}
	if available <= 0 {
		return fmt.Errorf("no seats available")
	}

	_, err = tx.Exec(ctx, `
		UPDATE session_seat_inventory
		SET reserved_seats = reserved_seats + 1,
		    version = version + 1, updated_at = now()
		WHERE session_id = $1`, reg.SessionID)
	if err != nil {
		return fmt.Errorf("deduct seat: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO session_registrations (id, session_id, user_id, status, registered_at,
			canceled_at, cancel_reason, approved_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		reg.ID, reg.SessionID, reg.UserID, reg.Status, reg.RegisteredAt,
		reg.CanceledAt, reg.CancelReason, reg.ApprovedBy, reg.CreatedAt, reg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert registration: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO registration_status_history (id, registration_id, old_status, new_status,
			actor_type, actor_id, reason_code, note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		history.ID, history.RegistrationID, history.OldStatus, history.NewStatus,
		history.ActorType, history.ActorID, history.ReasonCode, history.Note, history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}

	return tx.Commit(ctx)
}

// CreateWaitlistRegistration inserts a registration and history record WITHOUT
// touching seat_inventory (used when no seats are available and waitlist is allowed).
func (r *RegistrationRepo) CreateWaitlistRegistration(ctx context.Context, reg *model.SessionRegistration, history *model.RegistrationStatusHistory) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO session_registrations (id, session_id, user_id, status, registered_at,
			canceled_at, cancel_reason, approved_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		reg.ID, reg.SessionID, reg.UserID, reg.Status, reg.RegisteredAt,
		reg.CanceledAt, reg.CancelReason, reg.ApprovedBy, reg.CreatedAt, reg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert registration: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO registration_status_history (id, registration_id, old_status, new_status,
			actor_type, actor_id, reason_code, note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		history.ID, history.RegistrationID, history.OldStatus, history.NewStatus,
		history.ActorType, history.ActorID, history.ReasonCode, history.Note, history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}

	return tx.Commit(ctx)
}

// CountWaitlistEntries returns the number of waiting entries for a session.
func (r *RegistrationRepo) CountWaitlistEntries(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM session_waitlist_entries
		WHERE session_id = $1 AND status = 'waiting'`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count waitlist entries: %w", err)
	}
	return count, nil
}

func (r *RegistrationRepo) GetRegistrationByID(ctx context.Context, id uuid.UUID) (*model.SessionRegistration, error) {
	reg := &model.SessionRegistration{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, user_id, status, registered_at, canceled_at,
		       cancel_reason, approved_by, created_at, updated_at
		FROM session_registrations WHERE id = $1`, id,
	).Scan(
		&reg.ID, &reg.SessionID, &reg.UserID, &reg.Status, &reg.RegisteredAt,
		&reg.CanceledAt, &reg.CancelReason, &reg.ApprovedBy, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get registration: %w", err)
	}
	return reg, nil
}

// GetActiveRegistration returns the active registration for a user+session pair.
func (r *RegistrationRepo) GetActiveRegistration(ctx context.Context, userID, sessionID uuid.UUID) (*model.SessionRegistration, error) {
	reg := &model.SessionRegistration{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, user_id, status, registered_at, canceled_at,
		       cancel_reason, approved_by, created_at, updated_at
		FROM session_registrations
		WHERE user_id = $1 AND session_id = $2
		  AND status IN ('pending_approval','registered','waitlisted','checked_in','temporarily_away')`,
		userID, sessionID,
	).Scan(
		&reg.ID, &reg.SessionID, &reg.UserID, &reg.Status, &reg.RegisteredAt,
		&reg.CanceledAt, &reg.CancelReason, &reg.ApprovedBy, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active registration: %w", err)
	}
	return reg, nil
}

// UpdateRegistrationStatus updates a registration's status and inserts a history record.
func (r *RegistrationRepo) UpdateRegistrationStatus(ctx context.Context, regID uuid.UUID, newStatus string, history *model.RegistrationStatusHistory) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE session_registrations SET status = $2, updated_at = $3
		WHERE id = $1`, regID, newStatus, now)
	if err != nil {
		return fmt.Errorf("update registration status: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO registration_status_history (id, registration_id, old_status, new_status,
			actor_type, actor_id, reason_code, note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		history.ID, history.RegistrationID, history.OldStatus, history.NewStatus,
		history.ActorType, history.ActorID, history.ReasonCode, history.Note, history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *RegistrationRepo) CreateWaitlistEntry(ctx context.Context, entry *model.WaitlistEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO session_waitlist_entries (id, session_id, user_id, registration_id,
			position, status, promoted_at, promotion_attempts, last_attempt_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		entry.ID, entry.SessionID, entry.UserID, entry.RegistrationID,
		entry.Position, entry.Status, entry.PromotedAt, entry.PromotionAttempts,
		entry.LastAttemptReason, entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create waitlist entry: %w", err)
	}
	return nil
}

// GetNextWaitlistEntry returns the next waiting entry for a session ordered by position.
func (r *RegistrationRepo) GetNextWaitlistEntry(ctx context.Context, sessionID uuid.UUID) (*model.WaitlistEntry, error) {
	entry := &model.WaitlistEntry{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, user_id, registration_id, position, status,
		       promoted_at, promotion_attempts, last_attempt_reason, created_at, updated_at
		FROM session_waitlist_entries
		WHERE session_id = $1 AND status = 'waiting'
		ORDER BY position ASC
		LIMIT 1`, sessionID,
	).Scan(
		&entry.ID, &entry.SessionID, &entry.UserID, &entry.RegistrationID,
		&entry.Position, &entry.Status, &entry.PromotedAt, &entry.PromotionAttempts,
		&entry.LastAttemptReason, &entry.CreatedAt, &entry.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get next waitlist entry: %w", err)
	}
	return entry, nil
}

// PromoteWaitlistEntry marks a waitlist entry as promoted.
func (r *RegistrationRepo) PromoteWaitlistEntry(ctx context.Context, entryID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE session_waitlist_entries
		SET status = 'promoted', promoted_at = $2, promotion_attempts = promotion_attempts + 1,
		    updated_at = $2
		WHERE id = $1`, entryID, now)
	if err != nil {
		return fmt.Errorf("promote waitlist entry: %w", err)
	}
	return nil
}

// CountActiveRegistrations returns the count of active registrations for a session.
func (r *RegistrationRepo) CountActiveRegistrations(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM session_registrations
		WHERE session_id = $1
		  AND status IN ('pending_approval','registered','waitlisted','checked_in','temporarily_away')`,
		sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active registrations: %w", err)
	}
	return count, nil
}

// GetByID is an alias for GetRegistrationByID used by services.
func (r *RegistrationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.SessionRegistration, error) {
	return r.GetRegistrationByID(ctx, id)
}

// GetActiveByUserAndSession is an alias for GetActiveRegistration used by services.
func (r *RegistrationRepo) GetActiveByUserAndSession(ctx context.Context, userID, sessionID uuid.UUID) (*model.SessionRegistration, error) {
	return r.GetActiveRegistration(ctx, userID, sessionID)
}

// Update updates a registration record.
func (r *RegistrationRepo) Update(ctx context.Context, reg *model.SessionRegistration) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE session_registrations
		SET status = $2, canceled_at = $3, cancel_reason = $4, approved_by = $5, updated_at = $6
		WHERE id = $1`,
		reg.ID, reg.Status, reg.CanceledAt, reg.CancelReason, reg.ApprovedBy, reg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update registration: %w", err)
	}
	return nil
}

// Create inserts a new registration record.
func (r *RegistrationRepo) Create(ctx context.Context, reg *model.SessionRegistration) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO session_registrations (id, session_id, user_id, status, registered_at,
			canceled_at, cancel_reason, approved_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		reg.ID, reg.SessionID, reg.UserID, reg.Status, reg.RegisteredAt,
		reg.CanceledAt, reg.CancelReason, reg.ApprovedBy, reg.CreatedAt, reg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create registration: %w", err)
	}
	return nil
}

// CreateStatusHistory inserts a registration status history record.
func (r *RegistrationRepo) CreateStatusHistory(ctx context.Context, h *model.RegistrationStatusHistory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO registration_status_history (id, registration_id, old_status, new_status,
			actor_type, actor_id, reason_code, note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		h.ID, h.RegistrationID, h.OldStatus, h.NewStatus,
		h.ActorType, h.ActorID, h.ReasonCode, h.Note, h.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create status history: %w", err)
	}
	return nil
}

// ReserveSeat atomically reserves a seat for a session. Returns true if successful.
func (r *RegistrationRepo) ReserveSeat(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE session_seat_inventory
		SET reserved_seats = reserved_seats + 1,
		    version = version + 1, updated_at = now()
		WHERE session_id = $1 AND available_seats > 0`, sessionID)
	if err != nil {
		return false, fmt.Errorf("reserve seat: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ReleaseSeat releases a reserved seat back to available.
func (r *RegistrationRepo) ReleaseSeat(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE session_seat_inventory
		SET reserved_seats = reserved_seats - 1,
		    version = version + 1, updated_at = now()
		WHERE session_id = $1 AND reserved_seats > 0`, sessionID)
	if err != nil {
		return fmt.Errorf("release seat: %w", err)
	}
	return nil
}

// PromoteWaitlistAtomic atomically reserves a seat, updates registration status to registered,
// marks the waitlist entry as promoted, and records status history — all in one transaction.
// Returns false if no seat is available (promotion skipped without error).
func (r *RegistrationRepo) PromoteWaitlistAtomic(ctx context.Context, entryID, registrationID, sessionID uuid.UUID, history *model.RegistrationStatusHistory) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock and check seat availability
	var available int
	err = tx.QueryRow(ctx, `
		SELECT available_seats FROM session_seat_inventory
		WHERE session_id = $1 FOR UPDATE`, sessionID).Scan(&available)
	if err != nil {
		return false, fmt.Errorf("lock seat inventory: %w", err)
	}
	if available <= 0 {
		return false, nil // No seat available, skip promotion
	}

	// Reserve seat
	_, err = tx.Exec(ctx, `
		UPDATE session_seat_inventory
		SET reserved_seats = reserved_seats + 1,
		    version = version + 1, updated_at = now()
		WHERE session_id = $1`, sessionID)
	if err != nil {
		return false, fmt.Errorf("reserve seat: %w", err)
	}

	// Update registration status to registered
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE session_registrations SET status = $2, updated_at = $3
		WHERE id = $1`, registrationID, "registered", now)
	if err != nil {
		return false, fmt.Errorf("update registration status: %w", err)
	}

	// Record status history
	_, err = tx.Exec(ctx, `
		INSERT INTO registration_status_history (id, registration_id, old_status, new_status,
			actor_type, actor_id, reason_code, note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		history.ID, history.RegistrationID, history.OldStatus, history.NewStatus,
		history.ActorType, history.ActorID, history.ReasonCode, history.Note, history.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert status history: %w", err)
	}

	// Mark waitlist entry as promoted
	_, err = tx.Exec(ctx, `
		UPDATE session_waitlist_entries
		SET status = 'promoted', promoted_at = $2, promotion_attempts = promotion_attempts + 1,
		    updated_at = $2
		WHERE id = $1`, entryID, now)
	if err != nil {
		return false, fmt.Errorf("promote waitlist entry: %w", err)
	}

	return true, tx.Commit(ctx)
}

// UpdateWaitlistEntry updates a waitlist entry.
func (r *RegistrationRepo) UpdateWaitlistEntry(ctx context.Context, entry *model.WaitlistEntry) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE session_waitlist_entries
		SET status = $2, promoted_at = $3, promotion_attempts = $4,
		    last_attempt_reason = $5, updated_at = $6
		WHERE id = $1`,
		entry.ID, entry.Status, entry.PromotedAt, entry.PromotionAttempts,
		entry.LastAttemptReason, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update waitlist entry: %w", err)
	}
	return nil
}

// ListByUser returns paginated registrations for a user.
func (r *RegistrationRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.SessionRegistration, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM session_registrations WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count registrations: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, session_id, user_id, status, registered_at, canceled_at,
		       cancel_reason, approved_by, created_at, updated_at
		FROM session_registrations WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list registrations: %w", err)
	}
	defer rows.Close()

	var regs []model.SessionRegistration
	for rows.Next() {
		var reg model.SessionRegistration
		if err := rows.Scan(
			&reg.ID, &reg.SessionID, &reg.UserID, &reg.Status, &reg.RegisteredAt,
			&reg.CanceledAt, &reg.CancelReason, &reg.ApprovedBy, &reg.CreatedAt, &reg.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan registration: %w", err)
		}
		regs = append(regs, reg)
	}
	return regs, total, rows.Err()
}
