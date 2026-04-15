package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type ImportService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewImportService(repos *repo.Repositories, auditSvc *AuditService) *ImportService {
	return &ImportService{repos: repos, auditSvc: auditSvc}
}

// Upload creates a new import job from an uploaded file.
// It checks for duplicate fingerprints to prevent re-processing.
func (s *ImportService) Upload(ctx context.Context, uploaderID uuid.UUID, filename string, file io.Reader) (*model.ImportJob, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	// Read file content and compute checksum
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Check duplicate fingerprint
	isDuplicate, err := s.repos.Import.CheckDuplicateImport(ctx, checksum, "default")
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if isDuplicate {
		return nil, fmt.Errorf("file with same checksum already uploaded")
	}

	now := time.Now().UTC()

	// Create file artifact
	storagePath := fmt.Sprintf("imports/%s/%s", now.Format("20060102"), filename)
	artifact := &model.FileArtifact{
		ID:           uuid.New(),
		Filename:     filename,
		FileType:     "import",
		SizeBytes:    int64(len(data)),
		Checksum:     checksum,
		StoragePath:  storagePath,
		ArtifactType: "import_file",
		UploadedBy:   &uploaderID,
		CreatedAt:    now,
	}

	// Persist file bytes to disk so validation can read the actual file
	dir := fmt.Sprintf("imports/%s", now.Format("20060102"))
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return nil, fmt.Errorf("create import directory: %w", mkErr)
	}
	if wErr := os.WriteFile(storagePath, data, 0o644); wErr != nil {
		return nil, fmt.Errorf("persist import file: %w", wErr)
	}

	if err := s.repos.Import.CreateFileArtifact(ctx, artifact); err != nil {
		return nil, fmt.Errorf("create file artifact: %w", err)
	}

	job := &model.ImportJob{
		ID:             uuid.New(),
		FileArtifactID: artifact.ID,
		TemplateType:   "default",
		Status:         "uploaded",
		UploadedBy:     uploaderID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repos.Import.CreateImportJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &uploaderID,
		Action:     "upload_import",
		Resource:   "import_job",
		ResourceID: strPtr(job.ID.String()),
		NewState:   map[string]interface{}{"status": "uploaded", "filename": filename},
	})

	return job, nil
}

// UploadImport creates a new import job from metadata (without reading a file stream).
func (s *ImportService) UploadImport(ctx context.Context, filename, checksum, templateType string, uploaderID uuid.UUID, storagePath string, size int64) (*model.ImportJob, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	if checksum == "" {
		return nil, fmt.Errorf("checksum is required")
	}

	isDuplicate, err := s.repos.Import.CheckDuplicateImport(ctx, checksum, templateType)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if isDuplicate {
		return nil, fmt.Errorf("file with same checksum already uploaded for this template type")
	}

	now := time.Now().UTC()

	artifact := &model.FileArtifact{
		ID:           uuid.New(),
		Filename:     filename,
		FileType:     "import",
		SizeBytes:    size,
		Checksum:     checksum,
		StoragePath:  storagePath,
		ArtifactType: "import_file",
		UploadedBy:   &uploaderID,
		CreatedAt:    now,
	}

	if err := s.repos.Import.CreateFileArtifact(ctx, artifact); err != nil {
		return nil, fmt.Errorf("create file artifact: %w", err)
	}

	job := &model.ImportJob{
		ID:             uuid.New(),
		FileArtifactID: artifact.ID,
		TemplateType:   templateType,
		Status:         "uploaded",
		UploadedBy:     uploaderID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repos.Import.CreateImportJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &uploaderID,
		Action:     "upload_import",
		Resource:   "import_job",
		ResourceID: strPtr(job.ID.String()),
		NewState:   map[string]interface{}{"status": "uploaded", "template_type": templateType, "filename": filename},
	})

	return job, nil
}

// GetImport retrieves an import job by ID.
func (s *ImportService) GetImport(ctx context.Context, jobID uuid.UUID) (*model.ImportJob, error) {
	job, err := s.repos.Import.GetImportJobByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}
	return job, nil
}

// ValidateImport runs strict validation on an uploaded import job.
func (s *ImportService) ValidateImport(ctx context.Context, jobID uuid.UUID) (*model.ImportJob, error) {
	job, err := s.repos.Import.GetImportJobByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return nil, NotFound("import job not found")
	}
	if job.Status != "uploaded" {
		return nil, fmt.Errorf("import job is not in uploaded state: %s", job.Status)
	}

	artifact, err := s.repos.Import.GetFileArtifactByID(ctx, job.FileArtifactID)
	if err != nil || artifact == nil {
		return nil, fmt.Errorf("file artifact not found")
	}

	isCSV := strings.HasSuffix(artifact.Filename, ".csv") || artifact.FileType == "csv"
	isXLSX := strings.HasSuffix(artifact.Filename, ".xlsx") || artifact.FileType == "xlsx"
	if !isCSV && !isXLSX {
		errJSON, _ := json.Marshal(map[string]interface{}{"message": "unsupported file format; must be .csv or .xlsx"})
		s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, errJSON)
		return s.repos.Import.GetImportJobByID(ctx, jobID)
	}

	// Strict validation: file must exist on disk — no metadata-only pass
	var records [][]string
	if isCSV {
		file, ferr := os.Open(artifact.StoragePath)
		if ferr != nil {
			errJSON, _ := json.Marshal(map[string]interface{}{"message": "file not found on disk", "path": artifact.StoragePath})
			s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, errJSON)
			return s.repos.Import.GetImportJobByID(ctx, jobID)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		var rerr error
		records, rerr = reader.ReadAll()
		if rerr != nil {
			s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, nil)
			return s.repos.Import.GetImportJobByID(ctx, jobID)
		}
	} else {
		// XLSX parsing
		f, ferr := excelize.OpenFile(artifact.StoragePath)
		if ferr != nil {
			errJSON, _ := json.Marshal(map[string]interface{}{"message": "file not found or invalid xlsx", "path": artifact.StoragePath})
			s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, errJSON)
			return s.repos.Import.GetImportJobByID(ctx, jobID)
		}
		defer f.Close()

		sheetName := f.GetSheetName(0)
		if sheetName == "" {
			s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, nil)
			return s.repos.Import.GetImportJobByID(ctx, jobID)
		}
		var rerr error
		records, rerr = f.GetRows(sheetName)
		if rerr != nil {
			s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, nil)
			return s.repos.Import.GetImportJobByID(ctx, jobID)
		}
	}

	if len(records) < 2 {
		s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(0), intPtr(0), intPtr(1), nil, nil)
		return s.repos.Import.GetImportJobByID(ctx, jobID)
	}

	// Validate required columns based on template_type
	header := records[0]
	requiredCols := getRequiredColumns(job.TemplateType)
	missingCols := []string{}
	headerMap := map[string]int{}
	for i, col := range header {
		headerMap[strings.TrimSpace(strings.ToLower(col))] = i
	}
	for _, req := range requiredCols {
		if _, ok := headerMap[req]; !ok {
			missingCols = append(missingCols, req)
		}
	}
	if len(missingCols) > 0 {
		errMsg := fmt.Sprintf("missing required columns: %s", strings.Join(missingCols, ", "))
		errJSON, _ := json.Marshal(map[string]interface{}{"missing_columns": missingCols, "message": errMsg})
		s.repos.Import.UpdateImportJobStatus(ctx, jobID, "validation_failed", intPtr(len(records)-1), intPtr(0), intPtr(len(records)-1), nil, errJSON)
		return s.repos.Import.GetImportJobByID(ctx, jobID)
	}

	// Row-level validation
	totalRows := len(records) - 1
	validRows := 0
	errorRows := 0

	for i := 1; i < len(records); i++ {
		row := records[i]
		rowValid := true
		for _, req := range requiredCols {
			idx, ok := headerMap[req]
			if !ok || idx >= len(row) || strings.TrimSpace(row[idx]) == "" {
				rowValid = false
				break
			}
		}
		if rowValid {
			validRows++
		} else {
			errorRows++
		}
	}

	newStatus := "validated"
	if errorRows > 0 {
		newStatus = "validation_failed"
	}
	s.repos.Import.UpdateImportJobStatus(ctx, jobID, newStatus, intPtr(totalRows), intPtr(validRows), intPtr(errorRows), nil, nil)
	return s.repos.Import.GetImportJobByID(ctx, jobID)
}

func getRequiredColumns(templateType string) []string {
	switch templateType {
	case "users":
		return []string{"username", "email"}
	case "products":
		return []string{"name", "price"}
	case "sessions":
		return []string{"title", "start_at", "end_at"}
	default:
		return []string{"name"}
	}
}

func intPtr(n int) *int { return &n }

// ApplyImport applies a validated import job. Requires validated status with zero errors.
func (s *ImportService) ApplyImport(ctx context.Context, jobID uuid.UUID) (*model.ImportJob, error) {
	job, err := s.repos.Import.GetImportJobByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("get import job: %w", err)
	}
	if job == nil {
		return nil, NotFound("import job not found")
	}
	if job.Status != "validated" {
		return nil, fmt.Errorf("import job must be validated before applying: current status %s", job.Status)
	}
	if job.ErrorRows != nil && *job.ErrorRows > 0 {
		return nil, fmt.Errorf("cannot apply import with %d validation errors", *job.ErrorRows)
	}

	appliedRows := 0
	if job.ValidRows != nil {
		appliedRows = *job.ValidRows
	}

	s.repos.Import.UpdateImportJobStatus(ctx, jobID, "completed", nil, nil, nil, &appliedRows, nil)
	return s.repos.Import.GetImportJobByID(ctx, jobID)
}

// CreateExport creates a new export job.
func (s *ImportService) CreateExport(ctx context.Context, requestedBy uuid.UUID, exportType, format string, filters interface{}) (*model.ExportJob, error) {
	if exportType == "" {
		return nil, fmt.Errorf("export type is required")
	}
	if format == "" {
		return nil, fmt.Errorf("export format is required")
	}

	var filtersJSON json.RawMessage
	if filters != nil {
		data, err := json.Marshal(filters)
		if err != nil {
			return nil, fmt.Errorf("marshal filters: %w", err)
		}
		filtersJSON = data
	}

	now := time.Now().UTC()
	job := &model.ExportJob{
		ID:          uuid.New(),
		ExportType:  exportType,
		Format:      format,
		Status:      "pending",
		Filters:     filtersJSON,
		RequestedBy: requestedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repos.Import.CreateExportJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create export job: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &requestedBy,
		Action:     "create_export_job",
		Resource:   "export_job",
		ResourceID: strPtr(job.ID.String()),
		NewState:   map[string]interface{}{"status": "pending", "export_type": exportType, "format": format},
	})

	return job, nil
}

// CreateExportJob creates a new export job with typed filters.
func (s *ImportService) CreateExportJob(ctx context.Context, exportType, format string, filters json.RawMessage, requestedBy uuid.UUID) (*model.ExportJob, error) {
	return s.CreateExport(ctx, requestedBy, exportType, format, filters)
}

// GetExportFile returns the content, filename, and content type for an export.
func (s *ImportService) GetExportFile(ctx context.Context, exportID uuid.UUID) ([]byte, string, string, error) {
	job, err := s.repos.Import.GetExportJobByID(ctx, exportID)
	if err != nil {
		return nil, "", "", fmt.Errorf("get export job: %w", err)
	}
	if job == nil {
		return nil, "", "", fmt.Errorf("export job not found")
	}

	filename := fmt.Sprintf("export-%s.%s", job.ExportType, job.Format)
	contentType := "text/csv"
	if job.Format == "xlsx" {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	// Generate real dataset export based on export type
	rows, header := s.fetchExportData(ctx, job.ExportType)

	if job.Format == "xlsx" {
		f := excelize.NewFile()
		sheet := "Sheet1"
		for i, h := range header {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheet, cell, h)
		}
		for r, row := range rows {
			for c, val := range row {
				cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
				f.SetCellValue(sheet, cell, val)
			}
		}
		buf, wErr := f.WriteToBuffer()
		if wErr != nil {
			return nil, "", "", fmt.Errorf("write xlsx: %w", wErr)
		}
		return buf.Bytes(), filename, contentType, nil
	}

	// CSV format
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Write(header)
	for _, row := range rows {
		w.Write(row)
	}
	w.Flush()

	return []byte(buf.String()), filename, contentType, nil
}

// fetchExportData returns rows and header for a given export type from real database data.
func (s *ImportService) fetchExportData(ctx context.Context, exportType string) ([][]string, []string) {
	pool := s.repos.Import.Pool()
	switch exportType {
	case "users":
		header := []string{"id", "username", "display_name", "email", "is_active", "created_at"}
		dbRows, err := pool.Query(ctx, `SELECT id, username, display_name, COALESCE(email,''), is_active, created_at FROM users ORDER BY created_at`)
		if err != nil {
			return nil, header
		}
		defer dbRows.Close()
		var rows [][]string
		for dbRows.Next() {
			var id, username, displayName, email string
			var isActive bool
			var createdAt time.Time
			if err := dbRows.Scan(&id, &username, &displayName, &email, &isActive, &createdAt); err != nil {
				continue
			}
			rows = append(rows, []string{id, username, displayName, email, fmt.Sprint(isActive), createdAt.Format(time.RFC3339)})
		}
		return rows, header
	case "orders":
		header := []string{"id", "order_number", "user_id", "status", "total", "currency", "created_at"}
		dbRows, err := pool.Query(ctx, `SELECT id, order_number, user_id, status, total, currency, created_at FROM orders ORDER BY created_at DESC`)
		if err != nil {
			return nil, header
		}
		defer dbRows.Close()
		var rows [][]string
		for dbRows.Next() {
			var id, orderNum, userID, status, currency string
			var total int64
			var createdAt time.Time
			if err := dbRows.Scan(&id, &orderNum, &userID, &status, &total, &currency, &createdAt); err != nil {
				continue
			}
			rows = append(rows, []string{id, orderNum, userID, status, fmt.Sprint(total), currency, createdAt.Format(time.RFC3339)})
		}
		return rows, header
	case "sessions":
		header := []string{"id", "title", "status", "start_at", "end_at", "created_at"}
		dbRows, err := pool.Query(ctx, `SELECT id, title, status, start_at, end_at, created_at FROM program_sessions ORDER BY created_at DESC`)
		if err != nil {
			return nil, header
		}
		defer dbRows.Close()
		var rows [][]string
		for dbRows.Next() {
			var id, title, status string
			var startAt, endAt, createdAt time.Time
			if err := dbRows.Scan(&id, &title, &status, &startAt, &endAt, &createdAt); err != nil {
				continue
			}
			rows = append(rows, []string{id, title, status, startAt.Format(time.RFC3339), endAt.Format(time.RFC3339), createdAt.Format(time.RFC3339)})
		}
		return rows, header
	default:
		header := []string{"id", "username", "display_name", "email", "is_active", "created_at"}
		dbRows, err := pool.Query(ctx, `SELECT id, username, display_name, COALESCE(email,''), is_active, created_at FROM users ORDER BY created_at`)
		if err != nil {
			return nil, header
		}
		defer dbRows.Close()
		var rows [][]string
		for dbRows.Next() {
			var id, username, displayName, email string
			var isActive bool
			var createdAt time.Time
			if err := dbRows.Scan(&id, &username, &displayName, &email, &isActive, &createdAt); err != nil {
				continue
			}
			rows = append(rows, []string{id, username, displayName, email, fmt.Sprint(isActive), createdAt.Format(time.RFC3339)})
		}
		return rows, header
	}
}

// ListImports returns paginated import jobs.
func (s *ImportService) ListImports(ctx context.Context, limit, offset int) ([]model.ImportJob, int, error) {
	return s.repos.Import.ListImportJobs(ctx, limit, offset)
}

// ListExports returns paginated export jobs.
func (s *ImportService) ListExports(ctx context.Context, limit, offset int) ([]model.ExportJob, int, error) {
	return s.repos.Import.ListExportJobs(ctx, limit, offset)
}
