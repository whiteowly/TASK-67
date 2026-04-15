package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ImportHandler struct {
	svc *service.ImportService
}

func NewImportHandler(svc *service.ImportService) *ImportHandler {
	return &ImportHandler{svc: svc}
}

func (h *ImportHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ValidationError(c, "File is required")
		return
	}
	defer file.Close()

	// Read file content for checksum computation
	content, err := io.ReadAll(file)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "UPLOAD_FAILED", "failed to read file")
		return
	}

	// Compute SHA-256 checksum from content
	hash := sha256.Sum256(content)
	checksum := hex.EncodeToString(hash[:])

	userID := middleware.GetAuthUserID(c)
	templateType := c.DefaultPostForm("template_type", "general")
	storagePath := fmt.Sprintf("uploads/%s-%s", checksum[:16], header.Filename)

	// Persist file bytes to storage_path so validation can read the actual file
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		response.Error(c, http.StatusInternalServerError, "UPLOAD_FAILED", "failed to create upload directory")
		return
	}
	if err := os.WriteFile(storagePath, content, 0o644); err != nil {
		response.Error(c, http.StatusInternalServerError, "UPLOAD_FAILED", "failed to persist file")
		return
	}

	imp, err := h.svc.UploadImport(c.Request.Context(), header.Filename, checksum, templateType, userID, storagePath, int64(len(content)))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "UPLOAD_FAILED", "import upload failed")
		return
	}

	response.Created(c, imp)
}

func (h *ImportHandler) ListImports(c *gin.Context) {
	pg := util.ParsePagination(c)

	imports, total, err := h.svc.ListImports(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if imports == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, imports, pg.Page, pg.PerPage, total)
}

func (h *ImportHandler) GetImportDetail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid import ID")
		return
	}

	imp, err := h.svc.GetImport(c.Request.Context(), id)
	if err != nil || imp == nil {
		response.NotFound(c, "Import not found")
		return
	}

	response.OK(c, imp)
}

func (h *ImportHandler) ValidateImport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid import ID")
		return
	}

	result, err := h.svc.ValidateImport(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "VALIDATION_FAILED")
		return
	}

	response.OK(c, result)
}

func (h *ImportHandler) ApplyImport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid import ID")
		return
	}

	result, err := h.svc.ApplyImport(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "APPLY_FAILED")
		return
	}

	response.OK(c, result)
}

type createExportRequest struct {
	ExportType string      `json:"export_type" binding:"required"`
	Format     string      `json:"format" binding:"required"`
	Filters    interface{} `json:"filters"`
}

func (h *ImportHandler) CreateExport(c *gin.Context) {
	var req createExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	filtersJSON, _ := json.Marshal(req.Filters)
	export, err := h.svc.CreateExportJob(c.Request.Context(), req.ExportType, req.Format, filtersJSON, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "EXPORT_CREATE_FAILED")
		return
	}

	response.Created(c, export)
}

func (h *ImportHandler) ListExports(c *gin.Context) {
	pg := util.ParsePagination(c)

	exports, total, err := h.svc.ListExports(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if exports == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, exports, pg.Page, pg.PerPage, total)
}

func (h *ImportHandler) DownloadExport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid export ID")
		return
	}

	content, filename, contentType, err := h.svc.GetExportFile(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err, http.StatusNotFound, "EXPORT_NOT_FOUND")
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, content)
}
