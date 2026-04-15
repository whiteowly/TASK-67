package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthSession represents an authenticated user session.
type AuthSession struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	IPAddr    *string    `json:"ip_addr,omitempty"`
	UserAgent *string    `json:"user_agent,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
}

type AccountLockout struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Reason    string    `json:"reason"`
	LockedAt  time.Time `json:"locked_at"`
	UnlocksAt time.Time `json:"unlocks_at"`
	CreatedAt time.Time `json:"created_at"`
}
