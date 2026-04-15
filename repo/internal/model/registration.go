package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	RegStatusPendingApproval = "pending_approval"
	RegStatusRegistered      = "registered"
	RegStatusWaitlisted      = "waitlisted"
	RegStatusCheckedIn       = "checked_in"
	RegStatusTemporarilyAway = "temporarily_away"
	RegStatusCompleted       = "completed"
	RegStatusCanceled        = "canceled"
	RegStatusRejected        = "rejected"
	RegStatusNoShowCanceled  = "no_show_canceled"
	RegStatusReleased        = "released"
	RegStatusExpired         = "expired"
)

// ActiveRegistrationStatuses are states that occupy an active slot.
var ActiveRegistrationStatuses = []string{
	RegStatusPendingApproval, RegStatusRegistered, RegStatusWaitlisted,
	RegStatusCheckedIn, RegStatusTemporarilyAway,
}

// TerminalStatuses cannot transition further.
var TerminalStatuses = []string{
	RegStatusCompleted, RegStatusCanceled, RegStatusRejected,
	RegStatusNoShowCanceled, RegStatusExpired,
}

type SessionRegistration struct {
	ID           uuid.UUID  `json:"id"`
	SessionID    uuid.UUID  `json:"session_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Status       string     `json:"status"`
	RegisteredAt time.Time  `json:"registered_at"`
	CanceledAt   *time.Time `json:"canceled_at,omitempty"`
	CancelReason *string    `json:"cancel_reason,omitempty"`
	ApprovedBy   *uuid.UUID `json:"approved_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type RegistrationStatusHistory struct {
	ID             uuid.UUID  `json:"id"`
	RegistrationID uuid.UUID  `json:"registration_id"`
	OldStatus      *string    `json:"old_status,omitempty"`
	NewStatus      string     `json:"new_status"`
	ActorType      string     `json:"actor_type"`
	ActorID        *uuid.UUID `json:"actor_id,omitempty"`
	ReasonCode     *string    `json:"reason_code,omitempty"`
	Note           *string    `json:"note,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type WaitlistEntry struct {
	ID                uuid.UUID  `json:"id"`
	SessionID         uuid.UUID  `json:"session_id"`
	UserID            uuid.UUID  `json:"user_id"`
	RegistrationID    uuid.UUID  `json:"registration_id"`
	Position          int        `json:"position"`
	Status            string     `json:"status"`
	PromotedAt        *time.Time `json:"promoted_at,omitempty"`
	PromotionAttempts int        `json:"promotion_attempts"`
	LastAttemptReason *string    `json:"last_attempt_reason,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SessionPolicy struct {
	ID                         uuid.UUID  `json:"id"`
	SessionID                  *uuid.UUID `json:"session_id,omitempty"`
	CheckinLeadMinutes         int        `json:"checkin_lead_minutes"`
	NoshowCancelMinutes        int        `json:"noshow_cancel_minutes"`
	LeaveMaxMinutes            int        `json:"leave_max_minutes"`
	LeavePerHour               int        `json:"leave_per_hour"`
	UnverifiedThresholdMinutes int        `json:"unverified_threshold_minutes"`
	RequiresBeacon             bool       `json:"requires_beacon"`
	Version                    int        `json:"version"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}
