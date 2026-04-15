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

const defaultCloseHoursBefore = 2

type RegistrationService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewRegistrationService(repos *repo.Repositories, auditSvc *AuditService) *RegistrationService {
	return &RegistrationService{repos: repos, auditSvc: auditSvc}
}

// Register creates a registration for a user in a session.
func (s *RegistrationService) Register(ctx context.Context, userID, sessionID uuid.UUID) (*model.SessionRegistration, error) {
	session, err := s.repos.Catalog.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	now := time.Now().UTC()
	if session.RegistrationOpenAt != nil && now.Before(*session.RegistrationOpenAt) {
		return nil, fmt.Errorf("registration is not yet open")
	}
	if session.RegistrationCloseAt != nil && now.After(*session.RegistrationCloseAt) {
		return nil, fmt.Errorf("registration is closed")
	}
	// Enforce default close-hours policy from config when session has no explicit close time
	if session.RegistrationCloseAt == nil {
		closeHours := defaultCloseHoursBefore
		cfg, cfgErr := s.repos.Config.GetByKey(ctx, "registration.default_close_hours_before")
		if cfgErr == nil && cfg != nil {
			if parsed, pErr := strconv.Atoi(cfg.Value); pErr == nil && parsed > 0 {
				closeHours = parsed
			}
		}
		defaultClose := session.StartAt.Add(-time.Duration(closeHours) * time.Hour)
		if now.After(defaultClose) {
			return nil, fmt.Errorf("registration is closed (default policy: %d hours before session start)", closeHours)
		}
	}
	if session.Status != model.SessionStatusPublished {
		return nil, fmt.Errorf("session is not available for registration")
	}

	existing, err := s.repos.Registration.GetActiveRegistration(ctx, userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("check existing registration: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user already has an active registration for this session")
	}

	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, fmt.Errorf("user is not eligible for registration")
	}

	regID := uuid.New()
	reg := &model.SessionRegistration{
		ID:           regID,
		SessionID:    sessionID,
		UserID:       userID,
		RegisteredAt: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		ActorType:      "user",
		ActorID:        &userID,
		CreatedAt:      now,
	}

	if session.AvailableSeats > 0 {
		if session.RequiresApproval {
			reg.Status = model.RegStatusPendingApproval
		} else {
			reg.Status = model.RegStatusRegistered
		}
		history.NewStatus = reg.Status

		if err := s.repos.Registration.CreateRegistration(ctx, reg, history); err != nil {
			return nil, fmt.Errorf("create registration: %w", err)
		}
	} else {
		if !session.AllowsWaitlist {
			return nil, fmt.Errorf("no seats available and waitlist is not allowed")
		}
		reg.Status = model.RegStatusWaitlisted
		history.NewStatus = reg.Status

		if err := s.repos.Registration.CreateWaitlistRegistration(ctx, reg, history); err != nil {
			return nil, fmt.Errorf("create waitlist registration: %w", err)
		}

		// Determine proper position by counting existing waitlist entries
		position, err := s.repos.Registration.CountWaitlistEntries(ctx, sessionID)
		if err != nil {
			position = 0
		}

		s.repos.Registration.CreateWaitlistEntry(ctx, &model.WaitlistEntry{
			ID:             uuid.New(),
			SessionID:      sessionID,
			UserID:         userID,
			RegistrationID: regID,
			Position:       position + 1,
			Status:         "waiting",
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "register",
		Resource:   "session_registration",
		ResourceID: strPtr(regID.String()),
		NewState:   map[string]interface{}{"status": reg.Status, "session_id": sessionID},
	})

	return reg, nil
}

// AdminOverrideRegister allows an administrator to register a user for a session,
// bypassing the default close-hours policy. The override is audited.
func (s *RegistrationService) AdminOverrideRegister(ctx context.Context, userID, sessionID, adminID uuid.UUID) (*model.SessionRegistration, error) {
	session, err := s.repos.Catalog.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.Status != model.SessionStatusPublished {
		return nil, fmt.Errorf("session is not available for registration")
	}

	existing, err := s.repos.Registration.GetActiveRegistration(ctx, userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("check existing registration: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user already has an active registration for this session")
	}

	now := time.Now().UTC()
	regID := uuid.New()
	reg := &model.SessionRegistration{
		ID:           regID,
		SessionID:    sessionID,
		UserID:       userID,
		Status:       model.RegStatusRegistered,
		RegisteredAt: now,
		ApprovedBy:   &adminID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		NewStatus:      model.RegStatusRegistered,
		ActorType:      "admin_override",
		ActorID:        &adminID,
		ReasonCode:     strPtr("admin_override_close_policy"),
		CreatedAt:      now,
	}
	if err := s.repos.Registration.CreateRegistration(ctx, reg, history); err != nil {
		return nil, fmt.Errorf("create registration: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &adminID,
		Action:     "admin_override_register",
		Resource:   "session_registration",
		ResourceID: strPtr(regID.String()),
		NewState:   map[string]interface{}{"status": reg.Status, "session_id": sessionID, "override": true},
	})

	return reg, nil
}

// Cancel cancels a registration if it is in a cancellable state.
func (s *RegistrationService) Cancel(ctx context.Context, regID, userID uuid.UUID, reason string) (*model.SessionRegistration, error) {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registration not found")
	}
	if reg.UserID != userID {
		return nil, fmt.Errorf("not authorized to cancel this registration")
	}

	cancellable := map[string]bool{
		model.RegStatusPendingApproval: true,
		model.RegStatusRegistered:      true,
		model.RegStatusWaitlisted:      true,
	}
	if !cancellable[reg.Status] {
		return nil, fmt.Errorf("registration cannot be canceled in current state: %s", reg.Status)
	}

	oldStatus := reg.Status
	now := time.Now().UTC()

	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusCanceled,
		ActorType:      "user",
		ActorID:        &userID,
		ReasonCode:     &reason,
		CreatedAt:      now,
	}

	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, regID, model.RegStatusCanceled, history); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}

	if oldStatus == model.RegStatusRegistered || oldStatus == model.RegStatusPendingApproval {
		s.PromoteNextWaitlist(ctx, reg.SessionID)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "cancel_registration",
		Resource:   "session_registration",
		ResourceID: strPtr(regID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusCanceled, "reason": reason},
	})

	return s.repos.Registration.GetRegistrationByID(ctx, regID)
}

// Approve approves a pending registration (staff/admin action).
func (s *RegistrationService) Approve(ctx context.Context, regID, approverID uuid.UUID) (*model.SessionRegistration, error) {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registration not found")
	}
	if reg.Status != model.RegStatusPendingApproval {
		return nil, fmt.Errorf("registration is not pending approval")
	}

	oldStatus := reg.Status
	now := time.Now().UTC()

	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusRegistered,
		ActorType:      "staff",
		ActorID:        &approverID,
		CreatedAt:      now,
	}

	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, regID, model.RegStatusRegistered, history); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &approverID,
		Action:     "approve_registration",
		Resource:   "session_registration",
		ResourceID: strPtr(regID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusRegistered},
	})

	return s.repos.Registration.GetRegistrationByID(ctx, regID)
}

// Reject rejects a pending registration (staff/admin action).
func (s *RegistrationService) Reject(ctx context.Context, regID, rejecterID uuid.UUID, reason string) (*model.SessionRegistration, error) {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil {
		return nil, fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, fmt.Errorf("registration not found")
	}
	if reg.Status != model.RegStatusPendingApproval {
		return nil, fmt.Errorf("registration is not pending approval")
	}

	oldStatus := reg.Status
	now := time.Now().UTC()

	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusRejected,
		ActorType:      "staff",
		ActorID:        &rejecterID,
		ReasonCode:     &reason,
		CreatedAt:      now,
	}

	if err := s.repos.Registration.UpdateRegistrationStatus(ctx, regID, model.RegStatusRejected, history); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}

	s.PromoteNextWaitlist(ctx, reg.SessionID)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &rejecterID,
		Action:     "reject_registration",
		Resource:   "session_registration",
		ResourceID: strPtr(regID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusRejected, "reason": reason},
	})

	return s.repos.Registration.GetRegistrationByID(ctx, regID)
}

// Get retrieves a single registration by ID.
func (s *RegistrationService) Get(ctx context.Context, regID uuid.UUID, userID uuid.UUID, roles []string) (*model.SessionRegistration, error) {
	reg, err := s.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil {
		return nil, err
	}
	if reg == nil {
		return nil, NotFound("registration not found")
	}
	if reg.UserID != userID && !hasRole(roles, model.RoleStaff) && !hasRole(roles, model.RoleAdministrator) {
		return nil, Forbidden("not authorized to access this registration")
	}
	return reg, nil
}

// GetRegistration retrieves a single registration by ID.
func (s *RegistrationService) GetRegistration(ctx context.Context, regID uuid.UUID, userID uuid.UUID, roles []string) (*model.SessionRegistration, error) {
	return s.Get(ctx, regID, userID, roles)
}

// hasRole checks if a role exists in the roles slice.
func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// ListByUser returns all registrations for a user with pagination.
func (s *RegistrationService) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.SessionRegistration, int, error) {
	return s.repos.Registration.ListByUser(ctx, userID, limit, offset)
}

// ListUserRegistrations returns all registrations for a user with pagination.
func (s *RegistrationService) ListUserRegistrations(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.SessionRegistration, int, error) {
	return s.ListByUser(ctx, userID, limit, offset)
}

// SweepWaitlistPromotions checks all sessions with available seats and waiting entries.
func (s *RegistrationService) SweepWaitlistPromotions(ctx context.Context) {
	sessions, err := s.repos.Catalog.GetSessionsWithAvailableSeats(ctx)
	if err != nil {
		return
	}
	for _, sessID := range sessions {
		s.PromoteNextWaitlist(ctx, sessID)
	}
}

// PromoteNextWaitlist finds the next eligible waitlist entry and promotes it.
func (s *RegistrationService) PromoteNextWaitlist(ctx context.Context, sessionID uuid.UUID) error {
	entry, err := s.repos.Registration.GetNextWaitlistEntry(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get next waitlist entry: %w", err)
	}
	if entry == nil {
		return nil
	}

	user, err := s.repos.User.GetByID(ctx, entry.UserID)
	if err != nil {
		return fmt.Errorf("get waitlist user: %w", err)
	}
	if user == nil || !user.IsActive {
		return s.PromoteNextWaitlist(ctx, sessionID)
	}

	now := time.Now().UTC()
	oldStatus := model.RegStatusWaitlisted
	history := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: entry.RegistrationID,
		OldStatus:      &oldStatus,
		NewStatus:      model.RegStatusRegistered,
		ActorType:      "system",
		ReasonCode:     strPtr("waitlist_promotion"),
		CreatedAt:      now,
	}

	promoted, err := s.repos.Registration.PromoteWaitlistAtomic(ctx, entry.ID, entry.RegistrationID, sessionID, history)
	if err != nil {
		return fmt.Errorf("promote registration: %w", err)
	}
	if !promoted {
		// No seat available — record failed attempt on waitlist entry and leave waiting
		failReason := "no_seats_available"
		s.repos.Registration.UpdateWaitlistEntry(ctx, &model.WaitlistEntry{
			ID:                entry.ID,
			Status:            "waiting",
			PromotionAttempts: entry.PromotionAttempts + 1,
			LastAttemptReason: &failReason,
			UpdatedAt:         now,
		})
		return nil
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "promote_waitlist",
		Resource:   "session_registration",
		ResourceID: strPtr(entry.RegistrationID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.RegStatusRegistered},
	})

	return nil
}
