package api

import (
	"net/http"
	"time"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BackupHandler struct {
	svc *service.BackupService
}

func NewBackupHandler(svc *service.BackupService) *BackupHandler {
	return &BackupHandler{svc: svc}
}

func (h *BackupHandler) RunBackup(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	backup, err := h.svc.RunBackup(c.Request.Context(), &userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "BACKUP_FAILED")
		return
	}

	response.Created(c, backup)
}

func (h *BackupHandler) ListBackups(c *gin.Context) {
	pg := util.ParsePagination(c)

	backups, total, err := h.svc.ListBackups(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if backups == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, backups, pg.Page, pg.PerPage, total)
}

type initiateRestoreRequest struct {
	BackupID     uuid.UUID  `json:"backup_id" binding:"required"`
	RecoveryMode string     `json:"recovery_mode" binding:"required"` // dry_run, full_restore, point_in_time
	Reason       string     `json:"reason" binding:"required"`
	TargetTime   *time.Time `json:"target_time"`  // required for point_in_time mode
	IsDryRun     bool       `json:"is_dry_run"`   // deprecated: use recovery_mode=dry_run
}

func (h *BackupHandler) InitiateRestore(c *gin.Context) {
	var req initiateRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	// Backwards compat: if recovery_mode is empty but is_dry_run is set, map it
	recoveryMode := req.RecoveryMode
	if recoveryMode == "" {
		if req.IsDryRun {
			recoveryMode = "dry_run"
		} else {
			recoveryMode = "full_restore"
		}
	}

	userID := middleware.GetAuthUserID(c)
	result, err := h.svc.InitiateRestore(c.Request.Context(), req.BackupID, recoveryMode, req.Reason, req.TargetTime, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "RESTORE_FAILED")
		return
	}

	response.Created(c, result)
}

func (h *BackupHandler) ListArchives(c *gin.Context) {
	pg := util.ParsePagination(c)

	archives, total, err := h.svc.ListArchives(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if archives == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, archives, pg.Page, pg.PerPage, total)
}

type runArchiveRequest struct {
	ArchiveType string `json:"archive_type" binding:"required"`
}

func (h *BackupHandler) RunArchive(c *gin.Context) {
	var req runArchiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	archive, err := h.svc.RunArchive(c.Request.Context(), req.ArchiveType)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "ARCHIVE_FAILED")
		return
	}

	response.Created(c, archive)
}
