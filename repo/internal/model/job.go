package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID           uuid.UUID       `json:"id"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Priority     int             `json:"priority"`
	MaxRetries   int             `json:"max_retries"`
	RetryCount   int             `json:"retry_count"`
	RunAt        time.Time       `json:"run_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	LastError    *string         `json:"last_error,omitempty"`
	LeaseToken   *uuid.UUID      `json:"lease_token,omitempty"`
	LeaseExpires *time.Time      `json:"lease_expires,omitempty"`
	Progress     json.RawMessage `json:"progress,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type JobAttempt struct {
	ID        uuid.UUID  `json:"id"`
	JobID     uuid.UUID  `json:"job_id"`
	Attempt   int        `json:"attempt"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Status    string     `json:"status"`
	Error     *string    `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type ScheduledJob struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	JobType   string     `json:"job_type"`
	CronExpr  string     `json:"cron_expr"`
	Enabled   bool       `json:"enabled"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
