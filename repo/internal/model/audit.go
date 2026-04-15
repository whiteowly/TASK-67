package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         int64            `json:"id"`
	ActorType  string           `json:"actor_type"`
	ActorID    *uuid.UUID       `json:"actor_id,omitempty"`
	Action     string           `json:"action"`
	Resource   string           `json:"resource"`
	ResourceID *string          `json:"resource_id,omitempty"`
	OldState   json.RawMessage  `json:"old_state,omitempty"`
	NewState   json.RawMessage  `json:"new_state,omitempty"`
	ReasonCode *string          `json:"reason_code,omitempty"`
	Note       *string          `json:"note,omitempty"`
	RequestID  *uuid.UUID       `json:"request_id,omitempty"`
	IPAddr     *string          `json:"ip_addr,omitempty"`
	Metadata   json.RawMessage  `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// AuditEntry is used to create new audit log entries.
type AuditEntry struct {
	ActorType  string
	ActorID    *uuid.UUID
	Action     string
	Resource   string
	ResourceID *string
	OldState   interface{}
	NewState   interface{}
	ReasonCode *string
	Note       *string
	RequestID  *uuid.UUID
	IPAddr     *string
	Metadata   interface{}
}
