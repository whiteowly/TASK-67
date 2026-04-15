package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	TicketStatusOpen            = "open"
	TicketStatusAcknowledged    = "acknowledged"
	TicketStatusInProgress      = "in_progress"
	TicketStatusWaitingOnMember = "waiting_on_member"
	TicketStatusWaitingOnStaff  = "waiting_on_staff"
	TicketStatusEscalated       = "escalated"
	TicketStatusResolved        = "resolved"
	TicketStatusReopened        = "reopened"
	TicketStatusClosed          = "closed"
)

type Ticket struct {
	ID                 uuid.UUID  `json:"id"`
	TicketNumber       string     `json:"ticket_number"`
	TicketType         string     `json:"ticket_type"`
	Title              string     `json:"title"`
	Description        *string    `json:"description,omitempty"`
	Priority           string     `json:"priority"`
	Status             string     `json:"status"`
	SourceType         *string    `json:"source_type,omitempty"`
	SourceID           *uuid.UUID `json:"source_id,omitempty"`
	AssignedTo         *uuid.UUID `json:"assigned_to,omitempty"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	ResolutionCode     *string    `json:"resolution_code,omitempty"`
	ResolutionSummary  *string    `json:"resolution_summary,omitempty"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
	ClosedBy           *uuid.UUID `json:"closed_by,omitempty"`
	SLAResponseDue     *time.Time `json:"sla_response_due,omitempty"`
	SLAResolutionDue   *time.Time `json:"sla_resolution_due,omitempty"`
	SLAResponseMet     *bool      `json:"sla_response_met,omitempty"`
	SLAResolutionMet   *bool      `json:"sla_resolution_met,omitempty"`
	CreatedBy          *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TicketComment struct {
	ID        uuid.UUID `json:"id"`
	TicketID  uuid.UUID `json:"ticket_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Body      string    `json:"body"`
	IsInternal bool     `json:"is_internal"`
	CreatedAt time.Time `json:"created_at"`
}
