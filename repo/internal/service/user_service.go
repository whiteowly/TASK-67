package service

import (
	"context"
	"fmt"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type UserService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewUserService(repos *repo.Repositories, auditSvc *AuditService) *UserService {
	return &UserService{repos: repos, auditSvc: auditSvc}
}

func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserPublic, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	roles, err := s.repos.Role.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	pub := user.ToPublic(roles)
	return &pub, nil
}

type UpdateProfileInput struct {
	DisplayName string
	Email       string
	Phone       string
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*model.UserPublic, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if input.DisplayName != "" {
		user.DisplayName = input.DisplayName
	}
	if input.Email != "" {
		user.Email = &input.Email
	}
	if input.Phone != "" {
		user.Phone = &input.Phone
	}

	if err := s.repos.User.Update(ctx, user); err != nil {
		return nil, err
	}

	roles, _ := s.repos.Role.GetUserRoles(ctx, userID)
	pub := user.ToPublic(roles)
	return &pub, nil
}
