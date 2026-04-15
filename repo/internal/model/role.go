package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleMember        = "member"
	RoleStaff         = "staff"
	RoleModerator     = "moderator"
	RoleAdministrator = "administrator"
)

type Role struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserRoleAssignment struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	RoleID         uuid.UUID  `json:"role_id"`
	AssignedBy     *uuid.UUID `json:"assigned_by,omitempty"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EffectiveUntil *time.Time `json:"effective_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
