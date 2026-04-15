package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

const (
	defaultCheckinLeadMinutes         = 30
	defaultNoshowCancelMinutes        = 10
	defaultLeaveMaxMinutes            = 10
	defaultUnverifiedThresholdMinutes = 15
)

type AttendanceService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewAttendanceService(repos *repo.Repositories, auditSvc *AuditService) *AttendanceService {
	return &AttendanceService{repos: repos, auditSvc: auditSvc}
}

// allowedCheckinMethods defines the valid check-in method values.
// Only kiosk QR (qr_staff) and beacon are supported production methods.
var allowedCheckinMethods = map[string]bool{
	"qr_staff": true,
	"beacon":   true,
}

// CheckIn validates and records a check-in for a registration.
// The registration must be in Registered status and within the check-in window
// (30min before session start by default).
func (s *AttendanceService) CheckIn(ctx context.Context, registrationID uuid.UUID, confirmedBy *uuid.UUID, method string) (*model.CheckInEvent, error) {
	if !allowedCheckinMethods[method] {
		return nil, fmt.Errorf("invalid check-in method %q: allowed methods are qr_staff, beacon", method)
	}

	reg, err := s.repos.Registration.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registration not found")
	}

	if reg.Status != model.RegStatusRegistered {
		return nil, fmt.Errorf("registration is not in a check-in eligible state: %s", reg.Status)
	}

	// Get session to validate check-in window
	session, err := s.repos.Catalog.GetSessionByID(ctx, reg.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Load session policy for dynamic check-in rules
	policy, _ := s.repos.Attendance.GetSessionPolicy(ctx, reg.SessionID)
	checkinLead := defaultCheckinLeadMinutes
	requiresBeacon := false
	if policy != nil {
		checkinLead = policy.CheckinLeadMinutes
		requiresBeacon = policy.RequiresBeacon
	}

	now := time.Now().UTC()
	windowOpen := session.StartAt.Add(-time.Duration(checkinLead) * time.Minute)
	if now.Before(windowOpen) {
		return nil, fmt.Errorf("check-in window is not yet open (opens %s)", windowOpen.Format(time.RFC3339))
	}

	if requiresBeacon && method != "beacon" {
		return nil, fmt.Errorf("beacon check-in required for this session")
	}

	// Create check-in event
	eventID := uuid.New()
	event := &model.CheckInEvent{
		ID:             eventID,
		RegistrationID: registrationID,
		SessionID:      reg.SessionID,
		UserID:         reg.UserID,
		Method:         method,
		ConfirmedBy:    confirmedBy,
		Valid:          true,
		CreatedAt:      now,
	}

	if err := s.repos.Attendance.CreateCheckInEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("create check-in event: %w", err)
	}

	// Update registration status to CheckedIn
	oldStatus := reg.Status
	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusCheckedIn,
		ActorType:      "system",
		ReasonCode:     strPtr("check_in"),
		CreatedAt:      now,
	}
	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, registrationID, model.RegStatusCheckedIn, history); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}

	// Create occupancy session
	occupancy := &model.OccupancySession{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		SessionID:      reg.SessionID,
		UserID:         reg.UserID,
		StartedAt:      now,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.repos.Attendance.CreateOccupancySession(ctx, occupancy)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		ActorID:    confirmedBy,
		Action:     "check_in",
		Resource:   "session_registration",
		ResourceID: strPtr(registrationID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusCheckedIn, "method": method},
	})

	return event, nil
}

// StartLeave records a temporary leave for a checked-in user.
// Validates the user is currently checked in and leave allowance is not exceeded.
func (s *AttendanceService) StartLeave(ctx context.Context, registrationID, userID uuid.UUID) (*model.TemporaryLeaveEvent, error) {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registration not found")
	}
	if reg.UserID != userID {
		return nil, fmt.Errorf("not authorized")
	}
	if reg.Status != model.RegStatusCheckedIn {
		return nil, fmt.Errorf("must be checked in to take a leave")
	}

	// Load active occupancy session — required for valid FK
	occupancy, err := s.repos.Attendance.GetActiveOccupancy(ctx, registrationID)
	if err != nil {
		return nil, fmt.Errorf("get active occupancy: %w", err)
	}
	if occupancy == nil {
		return nil, fmt.Errorf("no active occupancy session found")
	}

	// Load session policy for leave rules
	policy, _ := s.repos.Attendance.GetSessionPolicy(ctx, reg.SessionID)
	leaveMax := defaultLeaveMaxMinutes
	leavePerHour := 1
	if policy != nil {
		leaveMax = policy.LeaveMaxMinutes
		leavePerHour = policy.LeavePerHour
	}

	now := time.Now().UTC()

	// Check leave count in last hour
	count, _ := s.repos.Attendance.CountLeavesSince(ctx, reg.ID, now.Add(-1*time.Hour))
	if count >= leavePerHour {
		return nil, fmt.Errorf("leave allowance exceeded")
	}

	// Create temporary leave event with real occupancy ID
	leave := &model.TemporaryLeaveEvent{
		ID:                 uuid.New(),
		OccupancyID:        occupancy.ID,
		RegistrationID:     registrationID,
		UserID:             userID,
		LeftAt:             now,
		MaxDurationMinutes: leaveMax,
		Exceeded:           false,
		CreatedAt:          now,
	}

	if err := s.repos.Attendance.CreateLeaveEvent(ctx, leave); err != nil {
		return nil, fmt.Errorf("create leave event: %w", err)
	}

	// Update registration status
	oldStatus := reg.Status
	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusTemporarilyAway,
		ActorType:      "user",
		ActorID:        &userID,
		ReasonCode:     strPtr("temporary_leave"),
		CreatedAt:      now,
	}
	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, registrationID, model.RegStatusTemporarilyAway, history); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "start_leave",
		Resource:   "session_registration",
		ResourceID: strPtr(registrationID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusTemporarilyAway, "occupancy_id": occupancy.ID},
	})

	return leave, nil
}

// EndLeave records the return from a temporary leave.
// Computes whether leave duration was exceeded. On breach: ends occupancy,
// releases seat, updates registration, creates exception + ticket, and audits.
func (s *AttendanceService) EndLeave(ctx context.Context, leaveID uuid.UUID, userID uuid.UUID) error {
	leave, err := s.repos.Attendance.GetLeaveByID(ctx, leaveID)
	if err != nil {
		return fmt.Errorf("get leave event: %w", err)
	}
	if leave == nil {
		return fmt.Errorf("leave event not found")
	}
	if leave.UserID != userID {
		return fmt.Errorf("not authorized to end this leave")
	}
	if leave.ReturnedAt != nil {
		return fmt.Errorf("leave already ended")
	}

	now := time.Now().UTC()

	// Compute whether the leave exceeded max duration
	elapsed := now.Sub(leave.LeftAt)
	exceeded := elapsed > time.Duration(leave.MaxDurationMinutes)*time.Minute

	if err := s.repos.Attendance.EndLeaveEvent(ctx, leaveID, exceeded); err != nil {
		return fmt.Errorf("end leave event: %w", err)
	}

	if exceeded {
		// Breach: end occupancy session, release seat, update registration
		if err := s.repos.Attendance.EndOccupancy(ctx, leave.RegistrationID, "leave_exceeded", now); err != nil {
			return fmt.Errorf("end occupancy on breach: %w", err)
		}

		// Update registration status to released and release seat
		reg, _ := s.repos.Registration.GetRegistrationByID(ctx, leave.RegistrationID)
		if reg != nil {
			oldStatus := reg.Status
			history := &model.RegistrationStatusHistory{
				ID:             uuid.New(),
				RegistrationID: leave.RegistrationID,
				OldStatus:      &oldStatus,
				NewStatus:      model.RegStatusReleased,
				ActorType:      "system",
				ReasonCode:     strPtr("leave_exceeded"),
				CreatedAt:      now,
			}
			s.repos.Registration.UpdateRegistrationStatus(ctx, leave.RegistrationID, model.RegStatusReleased, history)

			// Release the seat back to inventory
			s.repos.Registration.ReleaseSeat(ctx, reg.SessionID)

			// Create occupancy exception
			exc := &model.OccupancyException{
				ID:             uuid.New(),
				RegistrationID: leave.RegistrationID,
				SessionID:      reg.SessionID,
				UserID:         userID,
				ExceptionType:  "leave_exceeded",
				Description:    strPtr(fmt.Sprintf("Leave exceeded max %d minutes by %s", leave.MaxDurationMinutes, elapsed.Round(time.Second))),
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := s.repos.Attendance.CreateOccupancyException(ctx, exc); err == nil {
				// Create linked ticket
				ticketNum := fmt.Sprintf("LEV-%s", uuid.New().String()[:8])
				ticket := &model.Ticket{
					ID:           uuid.New(),
					TicketNumber: ticketNum,
					TicketType:   "occupancy_exception",
					Title:        fmt.Sprintf("Leave exceeded: registration %s", leave.RegistrationID.String()[:8]),
					Priority:     "high",
					Status:       model.TicketStatusOpen,
					SourceType:   strPtr("occupancy_exception"),
					SourceID:     &exc.ID,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				ticket.SLAResponseDue = timePtr(now.Add(4 * time.Hour))
				ticket.SLAResolutionDue = timePtr(now.Add(72 * time.Hour))

				if terr := s.repos.Ticket.Create(ctx, ticket); terr == nil {
					exc.TicketID = &ticket.ID
					s.repos.Attendance.UpdateOccupancyException(ctx, exc)
				}
			}

			s.auditSvc.Log(ctx, &model.AuditEntry{
				ActorType:  "system",
				Action:     "leave_exceeded_release",
				Resource:   "session_registration",
				ResourceID: strPtr(leave.RegistrationID.String()),
				OldState:   map[string]interface{}{"status": oldStatus},
				NewState: map[string]interface{}{
					"status":           model.RegStatusReleased,
					"exceeded_minutes": int(elapsed.Minutes()),
					"max_minutes":      leave.MaxDurationMinutes,
				},
			})
		}
	} else {
		// Normal return: restore registration to checked_in
		reg, _ := s.repos.Registration.GetRegistrationByID(ctx, leave.RegistrationID)
		if reg != nil && reg.Status == model.RegStatusTemporarilyAway {
			oldStatus := reg.Status
			history := &model.RegistrationStatusHistory{
				ID:             uuid.New(),
				RegistrationID: leave.RegistrationID,
				OldStatus:      &oldStatus,
				NewStatus:      model.RegStatusCheckedIn,
				ActorType:      "user",
				ActorID:        &userID,
				ReasonCode:     strPtr("leave_return"),
				CreatedAt:      now,
			}
			s.repos.Registration.UpdateRegistrationStatus(ctx, leave.RegistrationID, model.RegStatusCheckedIn, history)
		}
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "end_leave",
		Resource:   "temporary_leave_event",
		ResourceID: strPtr(leaveID.String()),
		NewState:   map[string]interface{}{"returned_at": now, "exceeded": exceeded},
	})

	return nil
}

// configInt resolves an integer config key from the system_config table, falling back to the given default.
func (s *AttendanceService) configInt(ctx context.Context, key string, fallback int) int {
	cfg, err := s.repos.Config.GetByKey(ctx, key)
	if err != nil || cfg == nil {
		return fallback
	}
	if parsed, pErr := strconv.Atoi(cfg.Value); pErr == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

// DetectNoShows finds registrations that are in Registered status but the session
// started more than N minutes ago (configurable), and auto-cancels them.
func (s *AttendanceService) DetectNoShows(ctx context.Context) (int, error) {
	noshowMinutes := s.configInt(ctx, "attendance.noshow_cancel_minutes", defaultNoshowCancelMinutes)
	threshold := time.Now().UTC().Add(-time.Duration(noshowMinutes) * time.Minute)
	noShows, err := s.repos.Attendance.FindNoShows(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("find no-shows: %w", err)
	}
	count := 0
	for _, reg := range noShows {
		oldStatus := reg.Status
		now := time.Now().UTC()
		history := &model.RegistrationStatusHistory{
			ID:             uuid.New(),
			RegistrationID: reg.ID,
			OldStatus:      &oldStatus,
			NewStatus:      model.RegStatusNoShowCanceled,
			ActorType:      "system",
			CreatedAt:      now,
		}
		if err := s.repos.Registration.UpdateRegistrationStatus(ctx, reg.ID, model.RegStatusNoShowCanceled, history); err != nil {
			continue
		}
		s.repos.Registration.ReleaseSeat(ctx, reg.SessionID)
		count++
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "system",
			Action:     "no_show_cancel",
			Resource:   "session_registration",
			ResourceID: strPtr(reg.ID.String()),
			OldState:   map[string]interface{}{"status": oldStatus},
			NewState:   map[string]interface{}{"status": model.RegStatusNoShowCanceled},
		})
	}
	return count, nil
}

// DetectStaleOccupancy finds active occupancy sessions that have been unverified
// for more than N minutes (configurable) and creates exceptions.
func (s *AttendanceService) DetectStaleOccupancy(ctx context.Context) (int, error) {
	staleMinutes := s.configInt(ctx, "attendance.unverified_occupancy_threshold_minutes", defaultUnverifiedThresholdMinutes)
	threshold := time.Now().UTC().Add(-time.Duration(staleMinutes) * time.Minute)
	stale, err := s.repos.Attendance.FindStaleOccupancy(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("find stale occupancy: %w", err)
	}
	count := 0
	now := time.Now().UTC()
	for _, occ := range stale {
		exc := &model.OccupancyException{
			ID:             uuid.New(),
			RegistrationID: occ.RegistrationID,
			SessionID:      occ.SessionID,
			UserID:         occ.UserID,
			ExceptionType:  "unverified_occupancy",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.repos.Attendance.CreateOccupancyException(ctx, exc); err != nil {
			continue
		}

		// Create linked ticket
		ticketNum := fmt.Sprintf("OCC-%s", uuid.New().String()[:8])
		ticket := &model.Ticket{
			ID:           uuid.New(),
			TicketNumber: ticketNum,
			TicketType:   "occupancy_exception",
			Title:        fmt.Sprintf("Stale occupancy: session %s", occ.SessionID.String()[:8]),
			Priority:     "medium",
			Status:       model.TicketStatusOpen,
			SourceType:   strPtr("occupancy_exception"),
			SourceID:     &exc.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		// SLA defaults: 4h response, 3 days resolution
		ticket.SLAResponseDue = timePtr(now.Add(4 * time.Hour))
		ticket.SLAResolutionDue = timePtr(now.Add(72 * time.Hour))

		if terr := s.repos.Ticket.Create(ctx, ticket); terr == nil {
			exc.TicketID = &ticket.ID
			// Update the exception with ticket link
			s.repos.Attendance.UpdateOccupancyException(ctx, exc)
		}

		count++
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "system",
			Action:     "stale_occupancy_exception",
			Resource:   "occupancy_session",
			ResourceID: strPtr(occ.ID.String()),
		})
	}
	return count, nil
}

// CompleteAttendance marks a registration as completed when the session ends.
func (s *AttendanceService) CompleteAttendance(ctx context.Context, registrationID uuid.UUID) error {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return fmt.Errorf("registration not found")
	}

	if reg.Status != model.RegStatusCheckedIn && reg.Status != model.RegStatusTemporarilyAway {
		return fmt.Errorf("registration is not in an active attendance state: %s", reg.Status)
	}

	now := time.Now().UTC()
	oldStatus := reg.Status

	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusCompleted,
		ActorType:      "system",
		ReasonCode:     strPtr("session_completed"),
		CreatedAt:      now,
	}

	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, registrationID, model.RegStatusCompleted, history); err != nil {
		return fmt.Errorf("update registration: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "complete_attendance",
		Resource:   "session_registration",
		ResourceID: strPtr(registrationID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusCompleted},
	})

	return nil
}

// ListExceptions returns paginated occupancy exceptions across all sessions.
func (s *AttendanceService) ListExceptions(ctx context.Context, limit, offset int) ([]model.OccupancyException, int, error) {
	return s.repos.Attendance.ListAllExceptions(ctx, limit, offset)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
