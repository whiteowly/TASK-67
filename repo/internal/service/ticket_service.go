package service

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

const (
	slaResponseHours  = 4 // 4 business hours
	slaResolutionDays = 3 // 3 calendar days
)

// CreateTicketInput holds the input data for creating a ticket.
type CreateTicketInput struct {
	TicketType  string
	Title       string
	Description string
	Priority    string
	SourceType  *string
	SourceID    *uuid.UUID
}

type TicketService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
	flagSvc  *FeatureFlagService
}

func NewTicketService(repos *repo.Repositories, auditSvc *AuditService, flagSvc *FeatureFlagService) *TicketService {
	return &TicketService{repos: repos, auditSvc: auditSvc, flagSvc: flagSvc}
}

// CreateTicket creates a new support ticket with auto-generated ticket number and SLA due dates.
func (s *TicketService) CreateTicket(ctx context.Context, ticketType, title, description, priority string, sourceType *string, sourceID *uuid.UUID, createdBy uuid.UUID) (*model.Ticket, error) {
	if title == "" {
		return nil, fmt.Errorf("ticket title is required")
	}

	now := time.Now().UTC()

	// Calculate SLA due dates
	slaResponseDue := now.Add(time.Duration(slaResponseHours) * time.Hour)
	slaResolutionDue := now.AddDate(0, 0, slaResolutionDays)

	ticket := &model.Ticket{
		ID:               uuid.New(),
		TicketType:       ticketType,
		Title:            title,
		Priority:         priority,
		Status:           model.TicketStatusOpen,
		SourceType:       sourceType,
		SourceID:         sourceID,
		SLAResponseDue:   &slaResponseDue,
		SLAResolutionDue: &slaResolutionDue,
		CreatedBy:        &createdBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if description != "" {
		ticket.Description = &description
	}

	// TicketNumber is auto-generated in the repo's CreateTicket
	if err := s.repos.Ticket.CreateTicket(ctx, ticket); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &createdBy,
		Action:     "create_ticket",
		Resource:   "ticket",
		ResourceID: strPtr(ticket.ID.String()),
		NewState:   map[string]interface{}{"status": model.TicketStatusOpen, "ticket_number": ticket.TicketNumber, "priority": priority},
	})

	return ticket, nil
}

// UpdateTicketStatus transitions a ticket to a new status.
func (s *TicketService) UpdateTicketStatus(ctx context.Context, ticketID uuid.UUID, newStatus string, actorID uuid.UUID, reason string, callerRoles []string) error {
	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}
	if !isTicketAuthorized(ticket, actorID, callerRoles) {
		return Forbidden("not authorized to access this ticket")
	}

	if !isValidTicketTransition(ticket.Status, newStatus) {
		return fmt.Errorf("invalid ticket transition from %s to %s", ticket.Status, newStatus)
	}

	oldStatus := ticket.Status

	if err := s.repos.Ticket.UpdateTicketStatus(ctx, ticketID, oldStatus, newStatus, &actorID); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "update_ticket_status",
		Resource:   "ticket",
		ResourceID: strPtr(ticketID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": newStatus, "reason": reason},
	})

	return nil
}

// AssignTicket assigns a ticket to a staff member.
// Gated by the "enable_ticket_assignment" feature flag.
func (s *TicketService) AssignTicket(ctx context.Context, ticketID, assigneeID, actorID uuid.UUID) error {
	// Feature flag gate: check if ticket assignment is enabled for this staff user
	if s.flagSvc != nil {
		enabled, _ := s.flagSvc.IsEnabledForUser(ctx, "enable_ticket_assignment", actorID, []string{model.RoleStaff})
		if !enabled {
			s.auditSvc.Log(ctx, &model.AuditEntry{
				ActorType:  "user",
				ActorID:    &actorID,
				Action:     "ticket_assign_denied_by_flag",
				Resource:   "feature_flag",
				ResourceID: strPtr("enable_ticket_assignment"),
				NewState:   map[string]interface{}{"allowed": false, "ticket_id": ticketID},
			})
			return fmt.Errorf("ticket assignment is not enabled for this user")
		}
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "user",
			ActorID:    &actorID,
			Action:     "ticket_assign_allowed_by_flag",
			Resource:   "feature_flag",
			ResourceID: strPtr("enable_ticket_assignment"),
			NewState:   map[string]interface{}{"allowed": true, "ticket_id": ticketID},
		})
	}

	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	if err := s.repos.Ticket.AssignTicket(ctx, ticketID, assigneeID, &actorID); err != nil {
		return fmt.Errorf("assign ticket: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "assign_ticket",
		Resource:   "ticket",
		ResourceID: strPtr(ticketID.String()),
		NewState:   map[string]interface{}{"assigned_to": assigneeID},
	})

	return nil
}

// AddComment adds a comment to a ticket.
func (s *TicketService) AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string, isInternal bool, callerRoles []string) (*model.TicketComment, error) {
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}

	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}
	if !isTicketAuthorized(ticket, authorID, callerRoles) {
		return nil, Forbidden("not authorized to access this ticket")
	}

	now := time.Now().UTC()
	comment := &model.TicketComment{
		ID:         uuid.New(),
		TicketID:   ticketID,
		AuthorID:   authorID,
		Body:       body,
		IsInternal: isInternal,
		CreatedAt:  now,
	}

	if err := s.repos.Ticket.AddComment(ctx, comment); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &authorID,
		Action:     "add_ticket_comment",
		Resource:   "ticket",
		ResourceID: strPtr(ticketID.String()),
		NewState:   map[string]interface{}{"comment_id": comment.ID, "is_internal": isInternal},
	})

	return comment, nil
}

// ResolveTicket marks a ticket as resolved with resolution data.
func (s *TicketService) ResolveTicket(ctx context.Context, ticketID uuid.UUID, resolutionCode, summary string, actorID uuid.UUID, callerRoles []string) error {
	if resolutionCode == "" {
		return fmt.Errorf("resolution code is required")
	}
	if summary == "" {
		return fmt.Errorf("resolution summary is required")
	}

	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}
	if !isTicketAuthorized(ticket, actorID, callerRoles) {
		return Forbidden("not authorized to access this ticket")
	}

	if ticket.Status == model.TicketStatusClosed {
		return fmt.Errorf("ticket is already closed")
	}

	oldStatus := ticket.Status

	if err := s.repos.Ticket.UpdateTicketStatus(ctx, ticketID, oldStatus, model.TicketStatusResolved, &actorID); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "resolve_ticket",
		Resource:   "ticket",
		ResourceID: strPtr(ticketID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.TicketStatusResolved, "resolution_code": resolutionCode},
	})

	return nil
}

// CloseTicket closes a resolved or open ticket.
func (s *TicketService) CloseTicket(ctx context.Context, ticketID, actorID uuid.UUID, callerRoles []string) error {
	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}
	if !isTicketAuthorized(ticket, actorID, callerRoles) {
		return Forbidden("not authorized to access this ticket")
	}

	if ticket.Status == model.TicketStatusClosed {
		return fmt.Errorf("ticket is already closed")
	}

	oldStatus := ticket.Status

	if err := s.repos.Ticket.UpdateTicketStatus(ctx, ticketID, oldStatus, model.TicketStatusClosed, &actorID); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "close_ticket",
		Resource:   "ticket",
		ResourceID: strPtr(ticketID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.TicketStatusClosed},
	})

	return nil
}

// CheckSLABreaches finds tickets past their SLA deadlines and creates escalation events.
func (s *TicketService) CheckSLABreaches(ctx context.Context) (int, error) {
	tickets, err := s.repos.Ticket.ListOverdueSLATickets(ctx)
	if err != nil {
		return 0, fmt.Errorf("find SLA breaches: %w", err)
	}

	count := 0
	for _, ticket := range tickets {
		// Escalate if not already escalated
		if ticket.Status != model.TicketStatusEscalated {
			oldStatus := ticket.Status
			if err := s.repos.Ticket.UpdateTicketStatus(ctx, ticket.ID, oldStatus, model.TicketStatusEscalated, nil); err != nil {
				continue
			}

			s.auditSvc.Log(ctx, &model.AuditEntry{
				ActorType:  "system",
				Action:     "sla_breach_escalation",
				Resource:   "ticket",
				ResourceID: strPtr(ticket.ID.String()),
				OldState:   map[string]interface{}{"status": oldStatus},
				NewState:   map[string]interface{}{"status": model.TicketStatusEscalated},
			})
		}

		count++
	}

	return count, nil
}

// GetTicket returns a ticket by ID.
func (s *TicketService) GetTicket(ctx context.Context, ticketID uuid.UUID, callerID uuid.UUID, callerRoles []string) (*model.Ticket, error) {
	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return nil, NotFound("ticket not found")
	}
	if !isTicketAuthorized(ticket, callerID, callerRoles) {
		return nil, Forbidden("not authorized to access this ticket")
	}
	return ticket, nil
}

// isTicketAuthorized checks if the caller is the ticket creator or has staff/admin role.
func isTicketAuthorized(ticket *model.Ticket, callerID uuid.UUID, callerRoles []string) bool {
	if ticket.CreatedBy != nil && *ticket.CreatedBy == callerID {
		return true
	}
	for _, r := range callerRoles {
		if r == model.RoleStaff || r == model.RoleAdministrator {
			return true
		}
	}
	return false
}

// ListTickets returns paginated tickets.
func (s *TicketService) ListTickets(ctx context.Context, limit, offset int) ([]model.Ticket, int, error) {
	return s.repos.Ticket.ListTickets(ctx, repo.TicketFilter{Limit: limit, Offset: offset})
}

// Create creates a new ticket using CreateTicketInput.
func (s *TicketService) Create(ctx context.Context, createdBy uuid.UUID, input CreateTicketInput) (*model.Ticket, error) {
	return s.CreateTicket(ctx, input.TicketType, input.Title, input.Description, input.Priority, input.SourceType, input.SourceID, createdBy)
}

// Get retrieves a ticket by ID.
func (s *TicketService) Get(ctx context.Context, ticketID uuid.UUID, callerID uuid.UUID, callerRoles []string) (*model.Ticket, error) {
	ticket, err := s.repos.Ticket.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, NotFound("ticket not found")
	}
	if !isTicketAuthorized(ticket, callerID, callerRoles) {
		return nil, Forbidden("not authorized to access this ticket")
	}
	return ticket, nil
}

// List returns paginated tickets with optional status and priority filters.
func (s *TicketService) List(ctx context.Context, status, priority string, limit, offset int) ([]model.Ticket, int, error) {
	return s.repos.Ticket.ListTickets(ctx, repo.TicketFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
}

// ListScoped returns tickets scoped by caller role:
// - staff/admin see all tickets
// - members see only tickets they created
func (s *TicketService) ListScoped(ctx context.Context, callerID uuid.UUID, callerRoles []string, status, priority string, limit, offset int) ([]model.Ticket, int, error) {
	filter := repo.TicketFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	}
	// If caller is not staff or administrator, scope to their own tickets
	isPrivileged := false
	for _, r := range callerRoles {
		if r == model.RoleStaff || r == model.RoleAdministrator {
			isPrivileged = true
			break
		}
	}
	if !isPrivileged {
		filter.CreatedBy = &callerID
	}
	return s.repos.Ticket.ListTickets(ctx, filter)
}

// UpdateStatus transitions a ticket to a new status. Returns the updated ticket.
func (s *TicketService) UpdateStatus(ctx context.Context, ticketID, actorID uuid.UUID, newStatus, reason string, callerRoles []string) (*model.Ticket, error) {
	if err := s.UpdateTicketStatus(ctx, ticketID, newStatus, actorID, reason, callerRoles); err != nil {
		return nil, err
	}
	return s.repos.Ticket.GetTicketByID(ctx, ticketID)
}

// Assign assigns a ticket to a staff member. Returns the updated ticket.
func (s *TicketService) Assign(ctx context.Context, ticketID, assignerID, assigneeID uuid.UUID) (*model.Ticket, error) {
	if err := s.AssignTicket(ctx, ticketID, assigneeID, assignerID); err != nil {
		return nil, err
	}
	return s.repos.Ticket.GetTicketByID(ctx, ticketID)
}

// Resolve resolves a ticket with resolution data. Returns the updated ticket.
func (s *TicketService) Resolve(ctx context.Context, ticketID, actorID uuid.UUID, resolutionCode, summary string, callerRoles []string) (*model.Ticket, error) {
	if err := s.ResolveTicket(ctx, ticketID, resolutionCode, summary, actorID, callerRoles); err != nil {
		return nil, err
	}
	return s.repos.Ticket.GetTicketByID(ctx, ticketID)
}

// Close closes a ticket. Returns the updated ticket.
func (s *TicketService) Close(ctx context.Context, ticketID, actorID uuid.UUID, callerRoles []string) (*model.Ticket, error) {
	if err := s.CloseTicket(ctx, ticketID, actorID, callerRoles); err != nil {
		return nil, err
	}
	return s.repos.Ticket.GetTicketByID(ctx, ticketID)
}

// isValidTicketTransition checks if a ticket status transition is allowed.
func isValidTicketTransition(from, to string) bool {
	allowed := map[string][]string{
		model.TicketStatusOpen:            {model.TicketStatusAcknowledged, model.TicketStatusInProgress, model.TicketStatusClosed},
		model.TicketStatusAcknowledged:    {model.TicketStatusInProgress, model.TicketStatusWaitingOnMember, model.TicketStatusClosed},
		model.TicketStatusInProgress:      {model.TicketStatusWaitingOnMember, model.TicketStatusWaitingOnStaff, model.TicketStatusResolved, model.TicketStatusEscalated},
		model.TicketStatusWaitingOnMember: {model.TicketStatusInProgress, model.TicketStatusResolved, model.TicketStatusClosed},
		model.TicketStatusWaitingOnStaff:  {model.TicketStatusInProgress, model.TicketStatusEscalated},
		model.TicketStatusEscalated:       {model.TicketStatusInProgress, model.TicketStatusResolved},
		model.TicketStatusResolved:        {model.TicketStatusClosed, model.TicketStatusReopened},
		model.TicketStatusReopened:        {model.TicketStatusInProgress, model.TicketStatusClosed},
	}

	targets, ok := allowed[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
