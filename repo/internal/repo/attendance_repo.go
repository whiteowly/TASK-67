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

type AttendanceRepo struct {
	pool *pgxpool.Pool
}

func NewAttendanceRepo(pool *pgxpool.Pool) *AttendanceRepo {
	return &AttendanceRepo{pool: pool}
}

func (r *AttendanceRepo) CreateCheckInEvent(ctx context.Context, e *model.CheckInEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO check_in_events (id, registration_id, session_id, user_id, method,
			confirmed_by, device_id, valid, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.RegistrationID, e.SessionID, e.UserID, e.Method,
		e.ConfirmedBy, e.DeviceID, e.Valid, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create check-in event: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) CreateOccupancySession(ctx context.Context, o *model.OccupancySession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO occupancy_sessions (id, registration_id, session_id, user_id,
			started_at, ended_at, end_reason, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		o.ID, o.RegistrationID, o.SessionID, o.UserID,
		o.StartedAt, o.EndedAt, o.EndReason, o.IsActive, o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create occupancy session: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) EndOccupancySession(ctx context.Context, id uuid.UUID, endReason string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE occupancy_sessions
		SET ended_at = $2, end_reason = $3, is_active = false, updated_at = $2
		WHERE id = $1`, id, now, endReason)
	if err != nil {
		return fmt.Errorf("end occupancy session: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) CreateLeaveEvent(ctx context.Context, e *model.TemporaryLeaveEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO temporary_leave_events (id, occupancy_id, registration_id, user_id,
			left_at, returned_at, max_duration_minutes, exceeded, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.OccupancyID, e.RegistrationID, e.UserID,
		e.LeftAt, e.ReturnedAt, e.MaxDurationMinutes, e.Exceeded, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create leave event: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) EndLeaveEvent(ctx context.Context, id uuid.UUID, exceeded bool) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE temporary_leave_events
		SET returned_at = $2, exceeded = $3
		WHERE id = $1`, id, now, exceeded)
	if err != nil {
		return fmt.Errorf("end leave event: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) CreateOccupancyException(ctx context.Context, e *model.OccupancyException) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO occupancy_exceptions (id, registration_id, session_id, user_id,
			exception_type, description, ticket_id, resolved, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.RegistrationID, e.SessionID, e.UserID,
		e.ExceptionType, e.Description, e.TicketID, e.Resolved, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create occupancy exception: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) ListOccupancyExceptions(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]model.OccupancyException, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM occupancy_exceptions WHERE session_id = $1`, sessionID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count occupancy exceptions: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, registration_id, session_id, user_id, exception_type, description,
		       ticket_id, resolved, created_at, updated_at
		FROM occupancy_exceptions
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, sessionID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list occupancy exceptions: %w", err)
	}
	defer rows.Close()

	var exceptions []model.OccupancyException
	for rows.Next() {
		var e model.OccupancyException
		if err := rows.Scan(
			&e.ID, &e.RegistrationID, &e.SessionID, &e.UserID, &e.ExceptionType,
			&e.Description, &e.TicketID, &e.Resolved, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan occupancy exception: %w", err)
		}
		exceptions = append(exceptions, e)
	}
	return exceptions, total, rows.Err()
}

// GetSessionPolicy retrieves the session policy for a given session, or nil if none exists.
func (r *AttendanceRepo) GetSessionPolicy(ctx context.Context, sessionID uuid.UUID) (*model.SessionPolicy, error) {
	p := &model.SessionPolicy{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, checkin_lead_minutes, noshow_cancel_minutes,
		       leave_max_minutes, leave_per_hour, unverified_threshold_minutes,
		       requires_beacon, version, created_at, updated_at
		FROM session_policies WHERE session_id = $1
		ORDER BY version DESC LIMIT 1`, sessionID,
	).Scan(
		&p.ID, &p.SessionID, &p.CheckinLeadMinutes, &p.NoshowCancelMinutes,
		&p.LeaveMaxMinutes, &p.LeavePerHour, &p.UnverifiedThresholdMinutes,
		&p.RequiresBeacon, &p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session policy: %w", err)
	}
	return p, nil
}

// GetActiveOccupancy returns the active occupancy session for a registration.
func (r *AttendanceRepo) GetActiveOccupancy(ctx context.Context, registrationID uuid.UUID) (*model.OccupancySession, error) {
	o := &model.OccupancySession{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, registration_id, session_id, user_id, started_at, ended_at,
		       end_reason, is_active, created_at, updated_at
		FROM occupancy_sessions
		WHERE registration_id = $1 AND is_active = true
		ORDER BY started_at DESC LIMIT 1`, registrationID,
	).Scan(
		&o.ID, &o.RegistrationID, &o.SessionID, &o.UserID, &o.StartedAt,
		&o.EndedAt, &o.EndReason, &o.IsActive, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active occupancy: %w", err)
	}
	return o, nil
}

// CountLeavesSince counts temporary leave events for a registration since a given time.
func (r *AttendanceRepo) CountLeavesSince(ctx context.Context, registrationID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM temporary_leave_events
		WHERE registration_id = $1 AND left_at >= $2`, registrationID, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count leaves since: %w", err)
	}
	return count, nil
}

// GetLeaveByID retrieves a temporary leave event by ID.
func (r *AttendanceRepo) GetLeaveByID(ctx context.Context, id uuid.UUID) (*model.TemporaryLeaveEvent, error) {
	e := &model.TemporaryLeaveEvent{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, occupancy_id, registration_id, user_id, left_at, returned_at,
		       max_duration_minutes, exceeded, created_at
		FROM temporary_leave_events WHERE id = $1`, id,
	).Scan(
		&e.ID, &e.OccupancyID, &e.RegistrationID, &e.UserID,
		&e.LeftAt, &e.ReturnedAt, &e.MaxDurationMinutes, &e.Exceeded, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get leave event: %w", err)
	}
	return e, nil
}

// UpdateLeaveEvent updates a temporary leave event.
func (r *AttendanceRepo) UpdateLeaveEvent(ctx context.Context, e *model.TemporaryLeaveEvent) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE temporary_leave_events
		SET returned_at = $2, exceeded = $3
		WHERE id = $1`, e.ID, e.ReturnedAt, e.Exceeded)
	if err != nil {
		return fmt.Errorf("update leave event: %w", err)
	}
	return nil
}

// FindNoShows returns registrations that are still in 'registered' status but whose
// session started before the given threshold.
func (r *AttendanceRepo) FindNoShows(ctx context.Context, threshold time.Time) ([]model.SessionRegistration, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sr.id, sr.session_id, sr.user_id, sr.status, sr.registered_at,
		       sr.canceled_at, sr.cancel_reason, sr.approved_by, sr.created_at, sr.updated_at
		FROM session_registrations sr
		JOIN program_sessions ps ON ps.id = sr.session_id
		WHERE sr.status = 'registered' AND ps.start_at < $1`, threshold)
	if err != nil {
		return nil, fmt.Errorf("find no shows: %w", err)
	}
	defer rows.Close()

	var regs []model.SessionRegistration
	for rows.Next() {
		var reg model.SessionRegistration
		if err := rows.Scan(
			&reg.ID, &reg.SessionID, &reg.UserID, &reg.Status, &reg.RegisteredAt,
			&reg.CanceledAt, &reg.CancelReason, &reg.ApprovedBy, &reg.CreatedAt, &reg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan no-show registration: %w", err)
		}
		regs = append(regs, reg)
	}
	return regs, rows.Err()
}

// FindStaleOccupancy returns active occupancy sessions that haven't been verified
// since the given threshold.
func (r *AttendanceRepo) FindStaleOccupancy(ctx context.Context, threshold time.Time) ([]model.OccupancySession, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, registration_id, session_id, user_id, started_at, ended_at,
		       end_reason, is_active, created_at, updated_at
		FROM occupancy_sessions
		WHERE is_active = true AND updated_at < $1`, threshold)
	if err != nil {
		return nil, fmt.Errorf("find stale occupancy: %w", err)
	}
	defer rows.Close()

	var sessions []model.OccupancySession
	for rows.Next() {
		var o model.OccupancySession
		if err := rows.Scan(
			&o.ID, &o.RegistrationID, &o.SessionID, &o.UserID, &o.StartedAt,
			&o.EndedAt, &o.EndReason, &o.IsActive, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale occupancy: %w", err)
		}
		sessions = append(sessions, o)
	}
	return sessions, rows.Err()
}

// EndOccupancy ends all active occupancy sessions for a registration.
func (r *AttendanceRepo) EndOccupancy(ctx context.Context, registrationID uuid.UUID, endReason string, endedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE occupancy_sessions
		SET ended_at = $2, end_reason = $3, is_active = false, updated_at = $2
		WHERE registration_id = $1 AND is_active = true`, registrationID, endedAt, endReason)
	if err != nil {
		return fmt.Errorf("end occupancy: %w", err)
	}
	return nil
}

// GetOccupancySessionByID retrieves an occupancy session by its ID.
func (r *AttendanceRepo) GetOccupancySessionByID(ctx context.Context, id uuid.UUID) (*model.OccupancySession, error) {
	o := &model.OccupancySession{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, registration_id, session_id, user_id, started_at, ended_at,
		       end_reason, is_active, created_at, updated_at
		FROM occupancy_sessions WHERE id = $1`, id,
	).Scan(
		&o.ID, &o.RegistrationID, &o.SessionID, &o.UserID, &o.StartedAt,
		&o.EndedAt, &o.EndReason, &o.IsActive, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get occupancy session: %w", err)
	}
	return o, nil
}

func (r *AttendanceRepo) UpdateOccupancyException(ctx context.Context, exc *model.OccupancyException) error {
	_, err := r.pool.Exec(ctx, `UPDATE occupancy_exceptions SET ticket_id = $2, updated_at = now() WHERE id = $1`, exc.ID, exc.TicketID)
	return err
}

// ListAllExceptions returns paginated occupancy exceptions across all sessions.
func (r *AttendanceRepo) ListAllExceptions(ctx context.Context, limit, offset int) ([]model.OccupancyException, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM occupancy_exceptions`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count occupancy exceptions: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, registration_id, session_id, user_id, exception_type, description,
		       ticket_id, resolved, created_at, updated_at
		FROM occupancy_exceptions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all occupancy exceptions: %w", err)
	}
	defer rows.Close()

	var exceptions []model.OccupancyException
	for rows.Next() {
		var e model.OccupancyException
		if err := rows.Scan(
			&e.ID, &e.RegistrationID, &e.SessionID, &e.UserID, &e.ExceptionType,
			&e.Description, &e.TicketID, &e.Resolved, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan occupancy exception: %w", err)
		}
		exceptions = append(exceptions, e)
	}
	return exceptions, total, rows.Err()
}
