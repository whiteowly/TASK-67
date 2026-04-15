package service

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/campusrec/campusrec/internal/validator"
	"github.com/google/uuid"
)

const (
	maxFailedAttempts   = 5
	lockoutDuration     = 15 * time.Minute
	lockoutWindowNotUsed = 15 * time.Minute // rolling window tracked via failed_attempts + locked_until
)

type AuthService struct {
	repos    *repo.Repositories
	cfg      *config.Config
	auditSvc *AuditService
}

func NewAuthService(repos *repo.Repositories, cfg *config.Config, auditSvc *AuditService) *AuthService {
	return &AuthService{repos: repos, cfg: cfg, auditSvc: auditSvc}
}

type RegisterInput struct {
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Password    string
}

type LoginInput struct {
	Username string
	Password string
	IPAddr   string
	UserAgent string
}

type LoginResult struct {
	Token string
	User  model.UserPublic
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*model.UserPublic, error) {
	// Validate username
	if err := validator.ValidateUsername(input.Username); err != nil {
		return nil, err
	}

	// Validate email
	if err := validator.ValidateEmail(input.Email); err != nil {
		return nil, err
	}

	// Validate password
	if violations := validator.ValidatePassword(input.Password); len(violations) > 0 {
		return nil, fmt.Errorf("%s", violations[0])
	}

	// Check username uniqueness
	existing, err := s.repos.User.GetByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("username already exists")
	}

	// Hash password
	hash, err := util.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &model.User{
		ID:           uuid.New(),
		Username:     validator.NormalizeUsername(input.Username),
		DisplayName:  input.DisplayName,
		PasswordHash: hash,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if input.Email != "" {
		user.Email = &input.Email
	}
	if input.Phone != "" {
		user.Phone = &input.Phone
	}
	if user.DisplayName == "" {
		user.DisplayName = input.Username
	}

	if err := s.repos.User.Create(ctx, user); err != nil {
		return nil, err
	}

	// Assign member role
	memberRole, err := s.repos.Role.GetByName(ctx, model.RoleMember)
	if err != nil {
		return nil, fmt.Errorf("get member role: %w", err)
	}
	if err := s.repos.Role.AssignRole(ctx, user.ID, memberRole.ID, nil); err != nil {
		return nil, fmt.Errorf("assign member role: %w", err)
	}

	// Audit
	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType: "user",
		ActorID:   &user.ID,
		Action:    "register",
		Resource:  "user",
		ResourceID: strPtr(user.ID.String()),
	})

	pub := user.ToPublic([]string{model.RoleMember})
	return &pub, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.repos.User.GetByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check lockout
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now().UTC()) {
		remaining := time.Until(*user.LockedUntil).Round(time.Second)
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "user",
			ActorID:    &user.ID,
			Action:     "login_locked",
			Resource:   "user",
			ResourceID: strPtr(user.ID.String()),
		})
		return nil, fmt.Errorf("account is locked, try again in %s", remaining)
	}

	// Verify password
	if !util.CheckPassword(input.Password, user.PasswordHash) {
		count, _ := s.repos.User.IncrementFailedAttempts(ctx, user.ID)
		if count >= maxFailedAttempts {
			lockUntil := time.Now().UTC().Add(lockoutDuration)
			_ = s.repos.User.LockAccount(ctx, user.ID, lockUntil)
			s.auditSvc.Log(ctx, &model.AuditEntry{
				ActorType:  "user",
				ActorID:    &user.ID,
				Action:     "account_locked",
				Resource:   "user",
				ResourceID: strPtr(user.ID.String()),
				NewState:   map[string]interface{}{"locked_until": lockUntil, "failed_attempts": count},
			})
		}
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "user",
			ActorID:    &user.ID,
			Action:     "login_failed",
			Resource:   "user",
			ResourceID: strPtr(user.ID.String()),
			IPAddr:     &input.IPAddr,
		})
		return nil, fmt.Errorf("invalid credentials")
	}

	// Success: reset failed attempts
	if err := s.repos.User.UpdateLoginSuccess(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("update login: %w", err)
	}

	// Create session
	token, tokenHash, err := util.GenerateSessionToken()
	if err != nil {
		return nil, err
	}

	sess := &model.AuthSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		IPAddr:    nilIfEmpty(input.IPAddr),
		UserAgent: nilIfEmpty(input.UserAgent),
		ExpiresAt: time.Now().UTC().Add(s.cfg.Session.MaxAge),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repos.Session.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	roles, _ := s.repos.Role.GetUserRoles(ctx, user.ID)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &user.ID,
		Action:     "login_success",
		Resource:   "user",
		ResourceID: strPtr(user.ID.String()),
		IPAddr:     &input.IPAddr,
	})

	pub := user.ToPublic(roles)
	return &LoginResult{Token: token, User: pub}, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	if err := s.repos.Session.Revoke(ctx, sessionID); err != nil {
		return err
	}
	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "logout",
		Resource:   "auth_session",
		ResourceID: strPtr(sessionID.String()),
	})
	return nil
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (*model.AuthSession, *model.User, []string, error) {
	tokenHash := util.HashToken(token)
	sess, err := s.repos.Session.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, nil, err
	}
	if sess == nil {
		return nil, nil, nil, nil
	}

	user, err := s.repos.User.GetByID(ctx, sess.UserID)
	if err != nil || user == nil || !user.IsActive {
		return nil, nil, nil, nil
	}

	roles, err := s.repos.Role.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	return sess, user, roles, nil
}

// RevokeAllSessions forcefully revokes all sessions for a user (admin action).
func (s *AuthService) RevokeAllSessions(ctx context.Context, targetUserID uuid.UUID, actorID uuid.UUID) error {
	if err := s.repos.Session.RevokeAllForUser(ctx, targetUserID); err != nil {
		return err
	}
	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &actorID,
		Action:     "revoke_all_sessions",
		Resource:   "user",
		ResourceID: strPtr(targetUserID.String()),
	})
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strPtr(s string) *string {
	return &s
}
