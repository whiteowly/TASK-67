package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type AuditService struct {
	repo *repo.AuditRepo
}

func NewAuditService(auditRepo *repo.AuditRepo) *AuditService {
	return &AuditService{repo: auditRepo}
}

// Log creates an audit log entry. Failures are logged as structured JSON
// but do not propagate to caller.
func (s *AuditService) Log(ctx context.Context, entry *model.AuditEntry) {
	if err := s.repo.Create(ctx, entry); err != nil {
		logEntry := map[string]interface{}{
			"level":       "error",
			"component":   "audit",
			"action":      "audit_persist_failure",
			"error_class": "database",
			"error":       err.Error(),
			"audit_action":   entry.Action,
			"audit_resource": entry.Resource,
		}
		if entry.RequestID != nil {
			logEntry["request_id"] = entry.RequestID.String()
		}
		b, _ := json.Marshal(logEntry)
		log.Printf("%s", b)
	}
}

// LogWithRequestID creates an audit log entry with request context.
func (s *AuditService) LogWithRequestID(ctx context.Context, entry *model.AuditEntry, requestID uuid.UUID) {
	entry.RequestID = &requestID
	s.Log(ctx, entry)
}

// List returns paginated audit logs.
func (s *AuditService) List(ctx context.Context, filter repo.AuditFilter) ([]model.AuditLog, int, error) {
	return s.repo.List(ctx, filter)
}
