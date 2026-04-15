package model

import (
	"time"

	"github.com/google/uuid"
)

type SystemConfig struct {
	ID          uuid.UUID  `json:"id"`
	Key         string     `json:"key"`
	Value       string     `json:"value"`
	ValueType   string     `json:"value_type"`
	Description string     `json:"description"`
	UpdatedBy   *uuid.UUID `json:"updated_by,omitempty"`
	Version     int        `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type FeatureFlag struct {
	ID            uuid.UUID  `json:"id"`
	Key           string     `json:"key"`
	Enabled       bool       `json:"enabled"`
	Description   string     `json:"description"`
	CohortPercent int        `json:"cohort_percent"`
	TargetRoles   []string   `json:"target_roles"`
	TargetDomains []string   `json:"target_domains"`
	UpdatedBy     *uuid.UUID `json:"updated_by,omitempty"`
	Version       int        `json:"version"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
