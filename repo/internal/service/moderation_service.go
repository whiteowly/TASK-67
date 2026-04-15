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
	postRateLimitPerHour = 5
)

// BanInput holds the input data for applying a ban.
type BanInput struct {
	UserID       uuid.UUID
	BanType      string
	IsPermanent  bool
	DurationDays int
	Reason       string
}

type ModerationService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewModerationService(repos *repo.Repositories, auditSvc *AuditService) *ModerationService {
	return &ModerationService{repos: repos, auditSvc: auditSvc}
}

// CreatePost creates a new post, enforcing a rate limit of 5 posts per hour
// and checking ban status.
func (s *ModerationService) CreatePost(ctx context.Context, userID uuid.UUID, title, body string) (*model.Post, error) {
	if body == "" {
		return nil, fmt.Errorf("post body is required")
	}

	// Check ban status
	ban, err := s.repos.Moderation.GetActiveBan(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check ban status: %w", err)
	}
	if ban != nil {
		return nil, fmt.Errorf("user is banned from posting")
	}

	// Check posting rate limit
	count, err := s.repos.Moderation.GetPostCountInWindow(ctx, userID, 1*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("check rate limit: %w", err)
	}
	if count >= postRateLimitPerHour {
		return nil, fmt.Errorf("posting rate limit exceeded: maximum %d posts per hour", postRateLimitPerHour)
	}

	now := time.Now().UTC()
	post := &model.Post{
		ID:        uuid.New(),
		UserID:    userID,
		Body:      body,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if title != "" {
		post.Title = &title
	}

	// Create the post (rate limiting already checked above)
	if err := s.repos.Moderation.CreatePost(ctx, post); err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	// Record posting event for rate window tracking
	s.repos.Moderation.RecordPostingEvent(ctx, userID)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "create_post",
		Resource:   "post",
		ResourceID: strPtr(post.ID.String()),
	})

	return post, nil
}

// GetPost retrieves a post by ID.
func (s *ModerationService) GetPost(ctx context.Context, id uuid.UUID) (*model.Post, error) {
	return s.repos.Moderation.GetPostByID(ctx, id)
}

// ReportPost creates a report on a post with deduplication.
func (s *ModerationService) ReportPost(ctx context.Context, postID, reporterID uuid.UUID, reason, description string) (*model.PostReport, error) {
	if reason == "" {
		return nil, fmt.Errorf("report reason is required")
	}

	// Check post exists
	post, err := s.repos.Moderation.GetPostByID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}
	if post == nil {
		return nil, fmt.Errorf("post not found")
	}

	now := time.Now().UTC()
	report := &model.PostReport{
		ID:         uuid.New(),
		PostID:     postID,
		ReporterID: reporterID,
		Reason:     reason,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if description != "" {
		report.Description = &description
	}

	// CreateReport checks for duplicates in the repo
	if err := s.repos.Moderation.CreateReport(ctx, report); err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &reporterID,
		Action:     "report_post",
		Resource:   "post_report",
		ResourceID: strPtr(report.ID.String()),
		NewState:   map[string]interface{}{"post_id": postID, "reason": reason},
	})

	return report, nil
}

// CreateCase creates a moderation case for a post.
func (s *ModerationService) CreateCase(ctx context.Context, postID uuid.UUID) (*model.ModerationCase, error) {
	post, err := s.repos.Moderation.GetPostByID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}
	if post == nil {
		return nil, fmt.Errorf("post not found")
	}

	now := time.Now().UTC()
	mc := &model.ModerationCase{
		ID:        uuid.New(),
		PostID:    &postID,
		UserID:    &post.UserID,
		Status:    "open",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repos.Moderation.CreateModerationCase(ctx, mc); err != nil {
		return nil, fmt.Errorf("create case: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "create_moderation_case",
		Resource:   "moderation_case",
		ResourceID: strPtr(mc.ID.String()),
		NewState:   map[string]interface{}{"post_id": postID, "status": "open"},
	})

	return mc, nil
}

// GetCase retrieves a moderation case by ID.
func (s *ModerationService) GetCase(ctx context.Context, caseID uuid.UUID) (*model.ModerationCase, error) {
	return s.repos.Moderation.GetCaseByID(ctx, caseID)
}

// ReviewCase assigns a reviewer and marks the case as under review.
func (s *ModerationService) ReviewCase(ctx context.Context, caseID, reviewerID uuid.UUID) error {
	if err := s.repos.Moderation.UpdateCaseStatus(ctx, caseID, "under_review"); err != nil {
		return fmt.Errorf("update case: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &reviewerID,
		Action:     "review_case",
		Resource:   "moderation_case",
		ResourceID: strPtr(caseID.String()),
		NewState:   map[string]interface{}{"status": "under_review", "assigned_to": reviewerID},
	})

	return nil
}

// ActionCase records a moderation action on a case and closes it.
// Returns the updated moderation case.
func (s *ModerationService) ActionCase(ctx context.Context, caseID uuid.UUID, actorID uuid.UUID, actionType string, details string) (*model.ModerationCase, error) {
	now := time.Now().UTC()
	action := &model.ModerationAction{
		ID:         uuid.New(),
		CaseID:     caseID,
		ActionType: actionType,
		ActorID:    actorID,
		CreatedAt:  now,
	}

	if err := s.repos.Moderation.CreateModerationAction(ctx, action); err != nil {
		return nil, fmt.Errorf("create action: %w", err)
	}

	if err := s.repos.Moderation.UpdateCaseStatus(ctx, caseID, "actioned"); err != nil {
		return nil, fmt.Errorf("update case: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "action_case",
		Resource:   "moderation_case",
		ResourceID: strPtr(caseID.String()),
		NewState:   map[string]interface{}{"status": "actioned", "action_type": actionType, "details": details},
	})

	return s.repos.Moderation.GetCaseByID(ctx, caseID)
}

// ApplyBan applies a ban to a user account using BanInput.
func (s *ModerationService) ApplyBan(ctx context.Context, issuedBy uuid.UUID, input BanInput) (*model.AccountBan, error) {
	if input.Reason == "" {
		return nil, fmt.Errorf("ban reason is required")
	}

	now := time.Now().UTC()
	ban := &model.AccountBan{
		ID:          uuid.New(),
		UserID:      input.UserID,
		BanType:     input.BanType,
		IsPermanent: input.IsPermanent,
		StartsAt:    now,
		Reason:      input.Reason,
		IssuedBy:    issuedBy,
		CreatedAt:   now,
	}

	if !input.IsPermanent && input.DurationDays > 0 {
		endsAt := now.AddDate(0, 0, input.DurationDays)
		ban.EndsAt = &endsAt
	}

	if err := s.repos.Moderation.CreateBan(ctx, ban); err != nil {
		return nil, fmt.Errorf("create ban: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &issuedBy,
		Action:     "apply_ban",
		Resource:   "account_ban",
		ResourceID: strPtr(ban.ID.String()),
		NewState:   map[string]interface{}{"user_id": input.UserID, "ban_type": input.BanType, "is_permanent": input.IsPermanent, "reason": input.Reason},
	})

	return ban, nil
}

// RevokeBan revokes an existing ban. Returns the updated ban.
func (s *ModerationService) RevokeBan(ctx context.Context, banID, revokedBy uuid.UUID) (*model.AccountBan, error) {
	if err := s.repos.Moderation.RevokeBan(ctx, banID, revokedBy); err != nil {
		return nil, fmt.Errorf("revoke ban: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &revokedBy,
		Action:     "revoke_ban",
		Resource:   "account_ban",
		ResourceID: strPtr(banID.String()),
		NewState:   map[string]interface{}{"revoked": true},
	})

	return s.repos.Moderation.GetBanByID(ctx, banID)
}

// ListPosts returns paginated posts.
func (s *ModerationService) ListPosts(ctx context.Context, limit, offset int) ([]model.Post, int, error) {
	return s.repos.Moderation.ListPosts(ctx, limit, offset)
}

// ListReports returns paginated post reports.
func (s *ModerationService) ListReports(ctx context.Context, limit, offset int) ([]model.PostReport, int, error) {
	return s.repos.Moderation.ListReports(ctx, limit, offset)
}

// ListCases returns paginated moderation cases.
func (s *ModerationService) ListCases(ctx context.Context, limit, offset int) ([]model.ModerationCase, int, error) {
	return s.repos.Moderation.ListCases(ctx, limit, offset)
}
