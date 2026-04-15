package service

import (
	"context"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type ConfigService struct {
	configRepo *repo.ConfigRepo
	auditSvc   *AuditService
}

func NewConfigService(configRepo *repo.ConfigRepo, auditSvc *AuditService) *ConfigService {
	return &ConfigService{configRepo: configRepo, auditSvc: auditSvc}
}

func (s *ConfigService) ListAll(ctx context.Context) ([]model.SystemConfig, error) {
	return s.configRepo.ListAll(ctx)
}

func (s *ConfigService) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	return s.configRepo.GetByKey(ctx, key)
}

func (s *ConfigService) Update(ctx context.Context, key, value string, updatedBy uuid.UUID, expectedVersion int) error {
	// Get current for audit
	old, _ := s.configRepo.GetByKey(ctx, key)

	err := s.configRepo.Update(ctx, key, value, updatedBy, expectedVersion)
	if err != nil {
		return err
	}

	var oldVal interface{}
	if old != nil {
		oldVal = map[string]string{"value": old.Value}
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &updatedBy,
		Action:     "update_config",
		Resource:   "system_config",
		ResourceID: &key,
		OldState:   oldVal,
		NewState:   map[string]string{"value": value},
	})

	return nil
}
