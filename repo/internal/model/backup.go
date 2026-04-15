package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BackupRun struct {
	ID            uuid.UUID  `json:"id"`
	Status        string     `json:"status"`
	ArtifactPath  *string    `json:"artifact_path,omitempty"`
	Checksum      *string    `json:"checksum,omitempty"`
	Encrypted     bool       `json:"encrypted"`
	SizeBytes     *int64     `json:"size_bytes,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Error         *string    `json:"error,omitempty"`
	RetentionDays int        `json:"retention_days"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	TriggeredBy   *uuid.UUID `json:"triggered_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// RestoreRecoveryMode constants define how a restore executes.
const (
	RecoveryModeDryRun          = "dry_run"           // integrity check only
	RecoveryModeFullRestore     = "full_restore"       // restore entire backup
	RecoveryModePointInTime     = "point_in_time"      // restore to a specific timestamp/restore point
)

type RestoreRun struct {
	ID               uuid.UUID       `json:"id"`
	BackupRunID      uuid.UUID       `json:"backup_run_id"`
	Status           string          `json:"status"` // pending, validating, validated, restoring, completed, failed
	RecoveryMode     string          `json:"recovery_mode"`
	IsDryRun         bool            `json:"is_dry_run"`
	Reason           string          `json:"reason"`
	TargetTime       *time.Time      `json:"target_time,omitempty"`       // for point_in_time mode
	RestorePoint     *string         `json:"restore_point,omitempty"`     // WAL restore point name
	ReplayStatus     *string         `json:"replay_status,omitempty"`     // pending_replay, replaying, replayed, skipped
	ValidationStatus *string         `json:"validation_status,omitempty"` // pending, checksum_ok, decryption_ok, schema_ok, failed
	InitiatedBy      uuid.UUID       `json:"initiated_by"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	Error            *string         `json:"error,omitempty"`
	ValidationResult json.RawMessage `json:"validation_result,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type ArchiveRun struct {
	ID            uuid.UUID  `json:"id"`
	ArchiveType   string     `json:"archive_type"`
	Status        string     `json:"status"`
	ThresholdDate time.Time  `json:"threshold_date"`
	TotalRows     int        `json:"total_rows"`
	ArchivedRows  int        `json:"archived_rows"`
	LastCursor    *string    `json:"last_cursor,omitempty"`
	ChunkSize     int        `json:"chunk_size"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Error         *string    `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
