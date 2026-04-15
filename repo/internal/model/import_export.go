package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type FileArtifact struct {
	ID           uuid.UUID  `json:"id"`
	Filename     string     `json:"filename"`
	FileType     string     `json:"file_type"`
	MimeType     *string    `json:"mime_type,omitempty"`
	SizeBytes    int64      `json:"size_bytes"`
	Checksum     string     `json:"checksum"`
	StoragePath  string     `json:"storage_path"`
	ArtifactType string     `json:"artifact_type"`
	UploadedBy   *uuid.UUID `json:"uploaded_by,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ImportJob struct {
	ID              uuid.UUID       `json:"id"`
	FileArtifactID  uuid.UUID       `json:"file_artifact_id"`
	TemplateType    string          `json:"template_type"`
	Status          string          `json:"status"`
	UploadedBy      uuid.UUID       `json:"uploaded_by"`
	TotalRows       *int            `json:"total_rows,omitempty"`
	ValidRows       *int            `json:"valid_rows,omitempty"`
	ErrorRows       *int            `json:"error_rows,omitempty"`
	AppliedRows     *int            `json:"applied_rows,omitempty"`
	ForceReprocess  bool            `json:"force_reprocess"`
	ForceReason     *string         `json:"force_reason,omitempty"`
	ErrorSummary    json.RawMessage `json:"error_summary,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ImportRow struct {
	ID          uuid.UUID       `json:"id"`
	ImportJobID uuid.UUID       `json:"import_job_id"`
	RowNumber   int             `json:"row_number"`
	RawData     json.RawMessage `json:"raw_data"`
	IsValid     bool            `json:"is_valid"`
	Errors      json.RawMessage `json:"errors,omitempty"`
	Applied     bool            `json:"applied"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ExportJob struct {
	ID             uuid.UUID       `json:"id"`
	ExportType     string          `json:"export_type"`
	Format         string          `json:"format"`
	Status         string          `json:"status"`
	Filters        json.RawMessage `json:"filters"`
	FileArtifactID *uuid.UUID      `json:"file_artifact_id,omitempty"`
	RequestedBy    uuid.UUID       `json:"requested_by"`
	TotalRows      *int            `json:"total_rows,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Error          *string         `json:"error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
