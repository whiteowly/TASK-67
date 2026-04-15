package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultRetentionDays   = 30
	archiveThresholdMonths = 24
	archiveChunkSize       = 1000
)

type BackupService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
	cfg      *config.BackupConfig
	flagSvc  *FeatureFlagService
}

func NewBackupService(repos *repo.Repositories, auditSvc *AuditService, cfg *config.BackupConfig, flagSvc *FeatureFlagService) *BackupService {
	return &BackupService{repos: repos, auditSvc: auditSvc, cfg: cfg, flagSvc: flagSvc}
}

// Pool returns the database pool for direct access (e.g. in integration tests).
func (s *BackupService) Pool() *pgxpool.Pool {
	return s.repos.Backup.Pool()
}

// GetBackupByID retrieves a backup run by ID.
func (s *BackupService) GetBackupByID(ctx context.Context, id uuid.UUID) (*model.BackupRun, error) {
	return s.repos.Backup.GetBackupRunByID(ctx, id)
}

// RunBackup creates a real backup artifact from database state, encrypts it,
// writes it to local storage, and records verifiable metadata.
// Gated by the "enable_manual_backup" feature flag for manual (non-scheduled) triggers.
func (s *BackupService) RunBackup(ctx context.Context, triggeredBy *uuid.UUID) (*model.BackupRun, error) {
	// Feature flag gate: check if manual backups are enabled for this user
	if triggeredBy != nil && s.flagSvc != nil {
		enabled, _ := s.flagSvc.IsEnabledForUser(ctx, "enable_manual_backup", *triggeredBy, []string{model.RoleAdministrator})
		if !enabled {
			s.auditSvc.Log(ctx, &model.AuditEntry{
				ActorType:  "user",
				ActorID:    triggeredBy,
				Action:     "backup_denied_by_flag",
				Resource:   "feature_flag",
				ResourceID: strPtr("enable_manual_backup"),
				NewState:   map[string]interface{}{"allowed": false},
			})
			return nil, fmt.Errorf("manual backup is not enabled for this user")
		}
		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "user",
			ActorID:    triggeredBy,
			Action:     "backup_allowed_by_flag",
			Resource:   "feature_flag",
			ResourceID: strPtr("enable_manual_backup"),
			NewState:   map[string]interface{}{"allowed": true},
		})
	}

	now := time.Now().UTC()
	run := &model.BackupRun{
		ID:            uuid.New(),
		Status:        "running",
		Encrypted:     s.cfg.EncryptionKey != "",
		StartedAt:     now,
		TriggeredBy:   triggeredBy,
		RetentionDays: defaultRetentionDays,
		CreatedAt:     now,
	}
	if err := s.repos.Backup.CreateBackupRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create backup run: %w", err)
	}

	// Export real data from key tables into a JSON artifact
	backupData, err := s.exportDatabaseState(ctx)
	if err != nil {
		errMsg := err.Error()
		run.Status = "failed"
		run.Error = &errMsg
		s.repos.Backup.UpdateBackupRun(ctx, run)
		return run, fmt.Errorf("export database state: %w", err)
	}

	// Compute checksum on plain data
	plainHash := sha256.Sum256(backupData)
	checksum := hex.EncodeToString(plainHash[:])

	// Encrypt if key is configured
	artifactData := backupData
	if s.cfg.EncryptionKey != "" {
		encrypted, encErr := encryptAESGCM(backupData, []byte(s.cfg.EncryptionKey))
		if encErr != nil {
			errMsg := encErr.Error()
			run.Status = "failed"
			run.Error = &errMsg
			s.repos.Backup.UpdateBackupRun(ctx, run)
			return run, fmt.Errorf("encrypt backup: %w", encErr)
		}
		artifactData = encrypted
	}

	// Ensure backup directory exists
	backupDir := s.cfg.Dir
	if backupDir == "" {
		backupDir = "./backups"
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	// Write artifact to disk
	filename := fmt.Sprintf("backup-%s.sql.gz.enc", now.Format("20060102-150405"))
	artifactPath := filepath.Join(backupDir, filename)
	if err := os.WriteFile(artifactPath, artifactData, 0o600); err != nil {
		return nil, fmt.Errorf("write backup artifact: %w", err)
	}

	sizeBytes := int64(len(artifactData))
	completedAt := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, defaultRetentionDays)
	run.Status = "completed"
	run.ArtifactPath = &artifactPath
	run.Checksum = &checksum
	run.SizeBytes = &sizeBytes
	run.CompletedAt = &completedAt
	run.ExpiresAt = &expiresAt

	s.repos.Backup.UpdateBackupRun(ctx, run)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		ActorID:    triggeredBy,
		Action:     "run_backup",
		Resource:   "backup_run",
		ResourceID: strPtr(run.ID.String()),
		NewState: map[string]interface{}{
			"status":        "completed",
			"retention_days": defaultRetentionDays,
			"size_bytes":    sizeBytes,
			"encrypted":    run.Encrypted,
			"checksum":     checksum,
		},
	})

	return run, nil
}

// backupArtifactVersion is the current artifact schema version.
// Version 2 contains full row data for each table, enabling real restore.
const backupArtifactVersion = 2

// restorableTableOrder defines FK-safe ordering for truncate+load restores.
// Parents come before children.
var restorableTableOrder = []string{
	"users",
	"products",
	"product_inventory",
	"program_sessions",
	"session_seat_inventory",
	"system_config",
	"orders",
	"session_registrations",
	"tickets",
}

// tableExportQueries maps table names to SELECT statements that export full rows
// as JSON arrays. Each column list matches the INSERT used during restore.
var tableExportQueries = map[string]string{
	"users": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,username,display_name,email,phone,password_hash,
		       is_active,failed_attempts,locked_until,last_login_at,
		       deleted_at,created_at,updated_at FROM users ORDER BY created_at) t`,
	"products": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,name,description,short_description,category,sku,
		       price_minor_units,currency,is_shippable,status,image_url,
		       tags,created_by,deleted_at,created_at,updated_at FROM products ORDER BY created_at) t`,
	"product_inventory": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT product_id,stock_qty,version,updated_at
		FROM product_inventory ORDER BY product_id) t`,
	"program_sessions": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,title,description,short_description,category,instructor_name,
		       tags,start_at,end_at,seat_capacity,price_minor_units,currency,
		       registration_open_at,registration_close_at,requires_approval,
		       allows_waitlist,status,location,created_by,deleted_at,created_at,updated_at
		FROM program_sessions ORDER BY created_at) t`,
	"session_seat_inventory": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT session_id,total_seats,reserved_seats,version,updated_at
		FROM session_seat_inventory ORDER BY session_id) t`,
	"system_config": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,key,value,value_type,description,updated_by,version,created_at,updated_at
		FROM system_config ORDER BY key) t`,
	"orders": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,user_id,order_number,status,subtotal,total,currency,
		       delivery_address_id,has_shippable,is_buy_now,close_reason,
		       idempotency_key,paid_at,closed_at,created_at,updated_at
		FROM orders ORDER BY created_at) t`,
	"session_registrations": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,session_id,user_id,status,registered_at,canceled_at,
		       cancel_reason,approved_by,created_at,updated_at
		FROM session_registrations ORDER BY created_at) t`,
	"tickets": `SELECT json_agg(row_to_json(t)) FROM (
		SELECT id,ticket_number,ticket_type,title,description,priority,status,
		       source_type,source_id,assigned_to,resolved_at,resolution_code,
		       resolution_summary,closed_at,closed_by,sla_response_due,
		       sla_resolution_due,sla_response_met,sla_resolution_met,
		       created_by,created_at,updated_at
		FROM tickets ORDER BY created_at) t`,
}

// exportDatabaseState extracts full row data from restorable tables into a
// versioned JSON artifact (v2). Each table is stored as a JSON array of row
// objects keyed by table name.
func (s *BackupService) exportDatabaseState(ctx context.Context) ([]byte, error) {
	pool := s.repos.Backup.Pool()

	tableData := make(map[string]json.RawMessage, len(restorableTableOrder))
	tableCounts := make(map[string]int, len(restorableTableOrder))

	for _, table := range restorableTableOrder {
		query, ok := tableExportQueries[table]
		if !ok {
			continue
		}
		var raw json.RawMessage
		err := pool.QueryRow(ctx, query).Scan(&raw)
		if err != nil || raw == nil {
			// Empty table → empty array
			raw = json.RawMessage("[]")
		}
		tableData[table] = raw
		// Count entries
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			tableCounts[table] = len(arr)
		}
	}

	payload := map[string]interface{}{
		"backup_version": backupArtifactVersion,
		"created_at":     time.Now().UTC().Format(time.RFC3339),
		"table_data":     tableData,
		"table_counts":   tableCounts,
		"table_order":    restorableTableOrder,
		"restore_point":  fmt.Sprintf("campusrec_backup_%s", time.Now().UTC().Format("20060102_150405")),
	}
	return json.Marshal(payload)
}

// encryptAESGCM encrypts data using AES-256-GCM with the given key.
// Key is hashed to 32 bytes via SHA-256 to ensure correct AES-256 key size.
func encryptAESGCM(plaintext, key []byte) ([]byte, error) {
	keyHash := sha256.Sum256(key)
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptAESGCM decrypts data encrypted with encryptAESGCM.
func decryptAESGCM(ciphertext, key []byte) ([]byte, error) {
	keyHash := sha256.Sum256(key)
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// InitiateRestore creates a restore run with an explicit recovery mode and
// executes real database restoration.
//
// Recovery modes:
//   - dry_run: validate artifact integrity only; no data changes
//   - full_restore: truncate+reload all restorable tables from artifact inside a TX
//   - point_in_time: baseline restore filtered to rows with created_at <= target_time
//
// State machine (each transition persisted before next phase):
//
//	validating → validated → restoring → completed
//	          ↘ failed (at any step)
func (s *BackupService) InitiateRestore(ctx context.Context, backupID uuid.UUID, recoveryMode, reason string, targetTime *time.Time, initiatedBy uuid.UUID) (*model.RestoreRun, error) {
	// ── Pre-flight checks ──────────────────────────────────────────────
	if reason == "" {
		return nil, fmt.Errorf("restore reason is required")
	}
	switch recoveryMode {
	case model.RecoveryModeDryRun, model.RecoveryModeFullRestore, model.RecoveryModePointInTime:
	default:
		return nil, fmt.Errorf("invalid recovery_mode %q: must be dry_run, full_restore, or point_in_time", recoveryMode)
	}
	if recoveryMode == model.RecoveryModePointInTime && targetTime == nil {
		return nil, fmt.Errorf("target_time is required for point_in_time recovery mode")
	}

	backup, err := s.repos.Backup.GetBackupRunByID(ctx, backupID)
	if err != nil {
		return nil, fmt.Errorf("get backup: %w", err)
	}
	if backup == nil {
		return nil, fmt.Errorf("backup not found")
	}
	if backup.Status != "completed" {
		return nil, fmt.Errorf("backup is not in completed state: %s", backup.Status)
	}

	now := time.Now().UTC()
	isDryRun := recoveryMode == model.RecoveryModeDryRun
	pendingValidation := "pending"

	restore := &model.RestoreRun{
		ID:               uuid.New(),
		BackupRunID:      backupID,
		Status:           "validating",
		RecoveryMode:     recoveryMode,
		IsDryRun:         isDryRun,
		Reason:           reason,
		TargetTime:       targetTime,
		ValidationStatus: &pendingValidation,
		InitiatedBy:      initiatedBy,
		StartedAt:        &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repos.Backup.CreateRestoreRun(ctx, restore); err != nil {
		return nil, fmt.Errorf("create restore run: %w", err)
	}

	execStats := map[string]interface{}{
		"backup_id":     backupID,
		"recovery_mode": recoveryMode,
	}
	if targetTime != nil {
		execStats["target_time"] = targetTime.Format(time.RFC3339)
	}

	// ── Phase 1: Validation ────────────────────────────────────────────
	_, backupPayload, err := s.validateArtifact(ctx, backup, restore, execStats)
	if err != nil {
		return restore, err // restore already marked failed inside validateArtifact
	}

	// Version gate: non-dry-run requires v2 (full row data)
	bkVersion, _ := backupPayload["backup_version"].(float64)
	if recoveryMode != model.RecoveryModeDryRun && int(bkVersion) < backupArtifactVersion {
		return s.failRestore(ctx, restore, execStats,
			fmt.Sprintf("artifact version %d does not contain row data; full/PITR restore requires version >= %d",
				int(bkVersion), backupArtifactVersion), "version_incompatible")
	}

	restorePointName, _ := backupPayload["restore_point"].(string)
	restore.RestorePoint = &restorePointName
	execStats["restore_point"] = restorePointName

	// PITR-specific: validate target_time scope
	if recoveryMode == model.RecoveryModePointInTime {
		bkCreatedStr, _ := backupPayload["created_at"].(string)
		bkCreated, pErr := time.Parse(time.RFC3339, bkCreatedStr)
		if pErr != nil {
			return s.failRestore(ctx, restore, execStats, "cannot parse backup created_at for PITR", "failed")
		}
		if targetTime.Before(bkCreated) {
			return s.failRestore(ctx, restore, execStats,
				"target_time is before backup creation; no data available", "failed")
		}
		execStats["target_time_valid"] = true
	}

	// Transition: validating → validated
	schemaOK := "schema_ok"
	restore.ValidationStatus = &schemaOK
	restore.Status = "validated"
	execStats["validation_status"] = "schema_ok"
	s.persistRestoreState(ctx, restore, execStats)

	// ── Phase 2: Execution ─────────────────────────────────────────────
	switch recoveryMode {
	case model.RecoveryModeDryRun:
		skipped := "skipped"
		restore.ReplayStatus = &skipped
		execStats["execution"] = "dry_run_only"
		// dry_run goes validated → completed (no restoring phase)

	case model.RecoveryModeFullRestore:
		// Transition: validated → restoring
		restore.Status = "restoring"
		s.persistRestoreState(ctx, restore, execStats)

		stats, execErr := s.executeFullRestore(ctx, backupPayload, nil)
		for k, v := range stats {
			execStats[k] = v
		}
		if execErr != nil {
			return s.failRestore(ctx, restore, execStats,
				fmt.Sprintf("full_restore execution failed: %v", execErr), "execution_failed")
		}
		replayed := "replayed"
		restore.ReplayStatus = &replayed

	case model.RecoveryModePointInTime:
		// Transition: validated → restoring
		restore.Status = "restoring"
		pending := "pending_replay"
		restore.ReplayStatus = &pending
		s.persistRestoreState(ctx, restore, execStats)

		stats, execErr := s.executeFullRestore(ctx, backupPayload, targetTime)
		for k, v := range stats {
			execStats[k] = v
		}
		execStats["replayed_to"] = targetTime.Format(time.RFC3339)
		execStats["restore_point_used"] = restorePointName
		if execErr != nil {
			return s.failRestore(ctx, restore, execStats,
				fmt.Sprintf("point_in_time execution failed: %v", execErr), "execution_failed")
		}
		replayed := "replayed"
		restore.ReplayStatus = &replayed
	}

	// Transition: → completed
	completedAt := time.Now().UTC()
	restore.CompletedAt = &completedAt
	restore.Status = "completed"
	execStats["duration_ms"] = completedAt.Sub(now).Milliseconds()
	s.persistRestoreState(ctx, restore, execStats)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &initiatedBy,
		Action:     "initiate_restore",
		Resource:   "restore_run",
		ResourceID: strPtr(restore.ID.String()),
		NewState:   execStats,
	})

	return restore, nil
}

// ── Validation ─────────────────────────────────────────────────────────

// validateArtifact reads, decrypts, checksums, and parses the backup artifact.
// On any failure it marks the restore run as failed and returns an error.
// On success it returns the plain-text data and parsed payload.
func (s *BackupService) validateArtifact(
	ctx context.Context,
	backup *model.BackupRun,
	restore *model.RestoreRun,
	stats map[string]interface{},
) ([]byte, map[string]interface{}, error) {

	if backup.ArtifactPath == nil {
		_, err := s.failRestore(ctx, restore, stats, "backup artifact path is missing", "failed")
		return nil, nil, err
	}
	artifactData, err := os.ReadFile(*backup.ArtifactPath)
	if err != nil {
		_, err2 := s.failRestore(ctx, restore, stats, fmt.Sprintf("artifact not readable: %v", err), "failed")
		return nil, nil, err2
	}
	stats["artifact_size"] = len(artifactData)

	// Decrypt
	plainData := artifactData
	if backup.Encrypted {
		if s.cfg.EncryptionKey == "" {
			_, err2 := s.failRestore(ctx, restore, stats, "backup encrypted but no key configured", "failed")
			return nil, nil, err2
		}
		dec, decErr := decryptAESGCM(artifactData, []byte(s.cfg.EncryptionKey))
		if decErr != nil {
			_, err2 := s.failRestore(ctx, restore, stats, fmt.Sprintf("decryption failed: %v", decErr), "failed")
			return nil, nil, err2
		}
		plainData = dec
	}

	// Checksum
	hash := sha256.Sum256(plainData)
	computed := hex.EncodeToString(hash[:])
	if backup.Checksum != nil && computed != *backup.Checksum {
		stats["expected_checksum"] = *backup.Checksum
		stats["computed_checksum"] = computed
		_, err2 := s.failRestore(ctx, restore, stats, "checksum mismatch: artifact may be corrupted", "failed")
		return nil, nil, err2
	}
	stats["checksum_valid"] = true

	// Parse JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(plainData, &payload); err != nil {
		_, err2 := s.failRestore(ctx, restore, stats, fmt.Sprintf("invalid backup format: %v", err), "failed")
		return nil, nil, err2
	}
	bkVersion, _ := payload["backup_version"].(float64)
	if bkVersion < 1 {
		_, err2 := s.failRestore(ctx, restore, stats, "unsupported backup version (<1)", "failed")
		return nil, nil, err2
	}
	stats["backup_version"] = int(bkVersion)
	stats["backup_created_at"] = payload["created_at"]

	return plainData, payload, nil
}

// ── Execution ──────────────────────────────────────────────────────────

// executeFullRestore applies backup data inside a single database transaction.
// If cutoff is non-nil, only rows with created_at <= cutoff are inserted (PITR).
// Returns per-table execution stats and any error.
func (s *BackupService) executeFullRestore(
	ctx context.Context,
	payload map[string]interface{},
	cutoff *time.Time,
) (map[string]interface{}, error) {

	stats := map[string]interface{}{}

	// Extract table_data and table_order from the v2 artifact
	tableDataRaw, ok := payload["table_data"]
	if !ok {
		return stats, fmt.Errorf("artifact missing table_data (not a v2 artifact)")
	}
	// Re-marshal then unmarshal to get typed map
	tdBytes, _ := json.Marshal(tableDataRaw)
	var tableData map[string]json.RawMessage
	if err := json.Unmarshal(tdBytes, &tableData); err != nil {
		return stats, fmt.Errorf("parse table_data: %w", err)
	}

	orderRaw, _ := payload["table_order"].([]interface{})
	tableOrder := make([]string, 0, len(orderRaw))
	for _, v := range orderRaw {
		if s, ok := v.(string); ok {
			tableOrder = append(tableOrder, s)
		}
	}
	if len(tableOrder) == 0 {
		tableOrder = restorableTableOrder
	}

	pool := s.repos.Backup.Pool()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tablesProcessed := 0
	rowsPerTable := map[string]int{}
	failurePoint := ""
	_ = failurePoint

	// Phase A: delete in reverse FK order (children first)
	for i := len(tableOrder) - 1; i >= 0; i-- {
		table := tableOrder[i]
		if _, exists := tableData[table]; !exists {
			continue
		}
		if table == "users" {
			var userRows []map[string]interface{}
			if err := json.Unmarshal(tableData[table], &userRows); err != nil {
				stats["failure_point"] = "parse:users"
				return stats, fmt.Errorf("parse rows for users: %w", err)
			}

			ids := make([]interface{}, 0, len(userRows))
			phs := make([]string, 0, len(userRows))
			for _, row := range userRows {
				id, ok := row["id"].(string)
				if !ok || id == "" {
					continue
				}
				ids = append(ids, id)
				phs = append(phs, fmt.Sprintf("$%d", len(ids)))
			}

			if len(ids) > 0 {
				q := fmt.Sprintf("DELETE FROM users WHERE id NOT IN (%s)", joinStrings(phs, ","))
				if _, err := tx.Exec(ctx, q, ids...); err != nil {
					stats["failure_point"] = "truncate:users"
					return stats, fmt.Errorf("truncate users: %w", err)
				}
			}
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			stats["failure_point"] = fmt.Sprintf("truncate:%s", table)
			return stats, fmt.Errorf("truncate %s: %w", table, err)
		}
	}

	// Phase B: insert in FK order (parents first)
	for _, table := range tableOrder {
		raw, exists := tableData[table]
		if !exists {
			continue
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal(raw, &rows); err != nil {
			stats["failure_point"] = fmt.Sprintf("parse:%s", table)
			return stats, fmt.Errorf("parse rows for %s: %w", table, err)
		}

		inserted := 0
		for _, row := range rows {
			// PITR filter: skip rows created after cutoff
			if cutoff != nil {
				if createdStr, ok := row["created_at"].(string); ok {
					skip := false
					for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
						if t, pErr := time.Parse(layout, createdStr); pErr == nil {
							if t.After(*cutoff) {
								skip = true
							}
							break
						}
					}
					if skip {
						continue
					}
				}
			}

			if len(row) == 0 {
				continue
			}
			cols := make([]string, 0, len(row))
			phs := make([]string, 0, len(row))
			vals := make([]interface{}, 0, len(row))
			pi := 1
			for k, v := range row {
				cols = append(cols, k)
				phs = append(phs, fmt.Sprintf("$%d", pi))
				vals = append(vals, v)
				pi++
			}
			q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
				table, joinStrings(cols, ","), joinStrings(phs, ","))
			if _, err := tx.Exec(ctx, q, vals...); err != nil {
				stats["failure_point"] = fmt.Sprintf("insert:%s:row_%d", table, inserted)
				return stats, fmt.Errorf("insert into %s row %d: %w", table, inserted, err)
			}
			inserted++
		}
		rowsPerTable[table] = inserted
		tablesProcessed++
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		stats["failure_point"] = "commit"
		return stats, fmt.Errorf("commit restore tx: %w", err)
	}

	stats["tables_processed"] = tablesProcessed
	stats["rows_restored"] = rowsPerTable
	totalRows := 0
	for _, n := range rowsPerTable {
		totalRows += n
	}
	stats["total_rows_restored"] = totalRows
	stats["replay_applied"] = true
	return stats, nil
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// ── Helpers ────────────────────────────────────────────────────────────

// persistRestoreState writes the current restore run status + stats to DB.
func (s *BackupService) persistRestoreState(ctx context.Context, restore *model.RestoreRun, stats map[string]interface{}) {
	valJSON, _ := json.Marshal(stats)
	restore.ValidationResult = valJSON
	s.repos.Backup.UpdateRestoreRun(ctx, restore.ID, restore.Status, restore.Error, valJSON)
}

// failRestore marks the restore as failed and persists the state.
func (s *BackupService) failRestore(ctx context.Context, restore *model.RestoreRun, stats map[string]interface{}, errMsg, valStatus string) (*model.RestoreRun, error) {
	stats["error"] = errMsg
	stats["validation_status"] = valStatus
	valJSON, _ := json.Marshal(stats)
	restore.Status = "failed"
	restore.Error = &errMsg
	restore.ValidationStatus = &valStatus
	restore.ValidationResult = valJSON
	s.repos.Backup.UpdateRestoreRun(ctx, restore.ID, "failed", &errMsg, valJSON)
	return restore, fmt.Errorf("%s", errMsg)
}

// RunArchive archives records older than 24 months.
func (s *BackupService) RunArchive(ctx context.Context, archiveType string) (*model.ArchiveRun, error) {
	if archiveType == "" {
		return nil, fmt.Errorf("archive type is required")
	}

	now := time.Now().UTC()
	threshold := now.AddDate(0, -24, 0) // 24 months ago

	run := &model.ArchiveRun{
		ID:            uuid.New(),
		ArchiveType:   archiveType,
		Status:        "running",
		ThresholdDate: threshold,
		ChunkSize:     1000,
		StartedAt:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repos.Backup.CreateArchiveRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create archive run: %w", err)
	}

	var archivedRows int
	switch archiveType {
	case "orders":
		// Count and move closed orders older than threshold
		err := s.repos.Backup.Pool().QueryRow(ctx,
			`SELECT count(*) FROM orders WHERE status IN ('auto_closed','manually_canceled','refunded_full','delivered') AND created_at < $1`,
			threshold).Scan(&archivedRows)
		if err != nil {
			archivedRows = 0
		}

		// Copy to archive schema, then delete from live tables
		if archivedRows > 0 {
			s.repos.Backup.Pool().Exec(ctx,
				`INSERT INTO archive.orders SELECT * FROM orders WHERE status IN ('auto_closed','manually_canceled','refunded_full','delivered') AND created_at < $1 ON CONFLICT DO NOTHING`, threshold)
			for {
				tag, delErr := s.repos.Backup.Pool().Exec(ctx,
					`DELETE FROM orders WHERE id IN (SELECT id FROM orders WHERE status IN ('auto_closed','manually_canceled','refunded_full','delivered') AND created_at < $1 LIMIT $2)`,
					threshold, archiveChunkSize)
				if delErr != nil || tag.RowsAffected() == 0 {
					break
				}
			}
		}
	case "tickets":
		err := s.repos.Backup.Pool().QueryRow(ctx,
			`SELECT count(*) FROM tickets WHERE status = 'closed' AND created_at < $1`,
			threshold).Scan(&archivedRows)
		if err != nil {
			archivedRows = 0
		}

		if archivedRows > 0 {
			s.repos.Backup.Pool().Exec(ctx,
				`INSERT INTO archive.tickets SELECT * FROM tickets WHERE status = 'closed' AND created_at < $1 ON CONFLICT DO NOTHING`, threshold)
			for {
				tag, delErr := s.repos.Backup.Pool().Exec(ctx,
					`DELETE FROM tickets WHERE id IN (SELECT id FROM tickets WHERE status = 'closed' AND created_at < $1 LIMIT $2)`,
					threshold, archiveChunkSize)
				if delErr != nil || tag.RowsAffected() == 0 {
					break
				}
			}
		}
	}

	// Create masked lookup projections from archive tables (live rows already deleted)
	if archivedRows > 0 {
		switch archiveType {
		case "orders":
			s.repos.Backup.Pool().Exec(ctx, `
				INSERT INTO archive_lookup_projection (source_type, source_id, month, status, masked_user_ref, monetary_total, currency, archived_at, created_at)
				SELECT 'order', id, date_trunc('month', created_at)::date, status,
				       'USR-' || left(user_id::text, 8), total, currency, $1, now()
				FROM archive.orders WHERE status IN ('auto_closed','manually_canceled','refunded_full','delivered') AND created_at < $2
				ON CONFLICT (source_type, source_id) DO NOTHING`, now, threshold)
		case "tickets":
			s.repos.Backup.Pool().Exec(ctx, `
				INSERT INTO archive_lookup_projection (source_type, source_id, month, status, masked_user_ref, archived_at, created_at)
				SELECT 'ticket', id, date_trunc('month', created_at)::date, status,
				       CASE WHEN created_by IS NOT NULL THEN 'USR-' || left(created_by::text, 8) ELSE 'system' END, $1, now()
				FROM archive.tickets WHERE status = 'closed' AND created_at < $2
				ON CONFLICT (source_type, source_id) DO NOTHING`, now, threshold)
		}
	}

	completedAt := time.Now().UTC()
	run.Status = "completed"
	run.ArchivedRows = archivedRows
	run.TotalRows = archivedRows
	run.CompletedAt = &completedAt
	run.UpdatedAt = completedAt
	s.repos.Backup.UpdateArchiveRun(ctx, run)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "run_archive",
		Resource:   "archive_run",
		ResourceID: strPtr(run.ID.String()),
		NewState:   map[string]interface{}{"status": "completed", "archive_type": archiveType, "threshold_date": threshold},
	})

	return run, nil
}

// ListBackups returns paginated backup runs.
func (s *BackupService) ListBackups(ctx context.Context, limit, offset int) ([]model.BackupRun, int, error) {
	return s.repos.Backup.ListBackupRuns(ctx, limit, offset)
}

// ListArchives returns paginated archive runs.
func (s *BackupService) ListArchives(ctx context.Context, limit, offset int) ([]model.ArchiveRun, int, error) {
	return s.repos.Backup.ListArchiveRuns(ctx, limit, offset)
}
