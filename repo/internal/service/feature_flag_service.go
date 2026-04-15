package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

type FeatureFlagService struct {
	flagRepo *repo.FeatureFlagRepo
	auditSvc *AuditService
}

func NewFeatureFlagService(flagRepo *repo.FeatureFlagRepo, auditSvc *AuditService) *FeatureFlagService {
	return &FeatureFlagService{flagRepo: flagRepo, auditSvc: auditSvc}
}

func (s *FeatureFlagService) ListAll(ctx context.Context) ([]model.FeatureFlag, error) {
	return s.flagRepo.ListAll(ctx)
}

func (s *FeatureFlagService) GetByKey(ctx context.Context, key string) (*model.FeatureFlag, error) {
	return s.flagRepo.GetByKey(ctx, key)
}

func (s *FeatureFlagService) Update(ctx context.Context, key string, enabled bool, cohortPercent int, updatedBy uuid.UUID, expectedVersion int) error {
	old, _ := s.flagRepo.GetByKey(ctx, key)

	err := s.flagRepo.Update(ctx, key, enabled, cohortPercent, updatedBy, expectedVersion)
	if err != nil {
		return err
	}

	var oldState interface{}
	if old != nil {
		oldState = map[string]interface{}{"enabled": old.Enabled, "cohort_percent": old.CohortPercent}
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &updatedBy,
		Action:     "update_feature_flag",
		Resource:   "feature_flag",
		ResourceID: &key,
		OldState:   oldState,
		NewState:   map[string]interface{}{"enabled": enabled, "cohort_percent": cohortPercent},
	})

	return nil
}

// IsEnabledForUser determines if a flag is active for a given user.
// Uses deterministic hashing for stable cohort assignment.
func (s *FeatureFlagService) IsEnabledForUser(ctx context.Context, flagKey string, userID uuid.UUID, userRoles []string) (bool, error) {
	flag, err := s.flagRepo.GetByKey(ctx, flagKey)
	if err != nil {
		return false, err
	}
	if flag == nil || !flag.Enabled {
		return false, nil
	}

	// Check role targeting
	if len(flag.TargetRoles) > 0 {
		matched := false
		for _, tr := range flag.TargetRoles {
			for _, ur := range userRoles {
				if tr == ur {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false, nil
		}
	}

	// Deterministic cohort assignment
	if flag.CohortPercent < 100 {
		if cohortBucket(flagKey, userID, flag.Version) >= flag.CohortPercent {
			return false, nil
		}
	}

	return true, nil
}

// cohortBucket computes the deterministic cohort bucket [0, 100) for a
// given (flagKey, userID, version) triple. The same triple always returns
// the same bucket, so users have stable rollout membership; bumping the
// flag version reshuffles assignments across the user base. The bucket is
// the first 4 bytes of SHA-256({flagKey}:{userID}:{version}) modulo 100.
//
// Extracted so it can be unit-tested independently of the repo dependency.
func cohortBucket(flagKey string, userID uuid.UUID, version int) int {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", flagKey, userID.String(), version)))
	return int(binary.BigEndian.Uint32(hash[:4]) % 100)
}
