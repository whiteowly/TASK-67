package model

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Title     *string    `json:"title,omitempty"`
	Body      string     `json:"body"`
	Status    string     `json:"status"`
	DeletedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type PostReport struct {
	ID          uuid.UUID `json:"id"`
	PostID      uuid.UUID `json:"post_id"`
	ReporterID  uuid.UUID `json:"reporter_id"`
	Reason      string    `json:"reason"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ModerationCase struct {
	ID         uuid.UUID  `json:"id"`
	PostID     *uuid.UUID `json:"post_id,omitempty"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Status     string     `json:"status"`
	AssignedTo *uuid.UUID `json:"assigned_to,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ModerationAction struct {
	ID         uuid.UUID `json:"id"`
	CaseID     uuid.UUID `json:"case_id"`
	ActionType string    `json:"action_type"`
	ActorID    uuid.UUID `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type AccountBan struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	BanType     string     `json:"ban_type"`
	IsPermanent bool       `json:"is_permanent"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	Reason      string     `json:"reason"`
	IssuedBy    uuid.UUID  `json:"issued_by"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RevokedBy   *uuid.UUID `json:"revoked_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
