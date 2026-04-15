package model

import (
	"time"

	"github.com/google/uuid"
)

type CheckInEvent struct {
	ID             uuid.UUID  `json:"id"`
	RegistrationID uuid.UUID  `json:"registration_id"`
	SessionID      uuid.UUID  `json:"session_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Method         string     `json:"method"`
	ConfirmedBy    *uuid.UUID `json:"confirmed_by,omitempty"`
	DeviceID       *uuid.UUID `json:"device_id,omitempty"`
	Valid          bool       `json:"valid"`
	CreatedAt      time.Time  `json:"created_at"`
}

type OccupancySession struct {
	ID             uuid.UUID  `json:"id"`
	RegistrationID uuid.UUID  `json:"registration_id"`
	SessionID      uuid.UUID  `json:"session_id"`
	UserID         uuid.UUID  `json:"user_id"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	EndReason      *string    `json:"end_reason,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type TemporaryLeaveEvent struct {
	ID                 uuid.UUID  `json:"id"`
	OccupancyID        uuid.UUID  `json:"occupancy_id"`
	RegistrationID     uuid.UUID  `json:"registration_id"`
	UserID             uuid.UUID  `json:"user_id"`
	LeftAt             time.Time  `json:"left_at"`
	ReturnedAt         *time.Time `json:"returned_at,omitempty"`
	MaxDurationMinutes int        `json:"max_duration_minutes"`
	Exceeded           bool       `json:"exceeded"`
	CreatedAt          time.Time  `json:"created_at"`
}

type OccupancyException struct {
	ID             uuid.UUID  `json:"id"`
	RegistrationID uuid.UUID  `json:"registration_id"`
	SessionID      uuid.UUID  `json:"session_id"`
	UserID         uuid.UUID  `json:"user_id"`
	ExceptionType  string     `json:"exception_type"`
	Description    *string    `json:"description,omitempty"`
	TicketID       *uuid.UUID `json:"ticket_id,omitempty"`
	Resolved       bool       `json:"resolved"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
