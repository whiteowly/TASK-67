package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ModerationHandler struct {
	svc *service.ModerationService
}

func NewModerationHandler(svc *service.ModerationService) *ModerationHandler {
	return &ModerationHandler{svc: svc}
}

type createPostRequest struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

func (h *ModerationHandler) CreatePost(c *gin.Context) {
	var req createPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	post, err := h.svc.CreatePost(c.Request.Context(), userID, req.Title, req.Body)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "POST_CREATE_FAILED")
		return
	}

	response.Created(c, post)
}

func (h *ModerationHandler) ListPosts(c *gin.Context) {
	pg := util.ParsePagination(c)

	posts, total, err := h.svc.ListPosts(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if posts == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, posts, pg.Page, pg.PerPage, total)
}

func (h *ModerationHandler) GetPost(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid post ID")
		return
	}

	post, err := h.svc.GetPost(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if post == nil {
		response.NotFound(c, "Post not found")
		return
	}

	response.OK(c, post)
}

type reportPostRequest struct {
	Reason      string `json:"reason" binding:"required"`
	Description string `json:"description"`
}

func (h *ModerationHandler) ReportPost(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid post ID")
		return
	}

	var req reportPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	report, err := h.svc.ReportPost(c.Request.Context(), id, userID, req.Reason, req.Description)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REPORT_FAILED")
		return
	}

	response.Created(c, report)
}

func (h *ModerationHandler) ListReports(c *gin.Context) {
	pg := util.ParsePagination(c)

	reports, total, err := h.svc.ListReports(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if reports == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, reports, pg.Page, pg.PerPage, total)
}

func (h *ModerationHandler) ListCases(c *gin.Context) {
	pg := util.ParsePagination(c)

	cases, total, err := h.svc.ListCases(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if cases == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, cases, pg.Page, pg.PerPage, total)
}

func (h *ModerationHandler) GetCase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid case ID")
		return
	}

	modCase, err := h.svc.GetCase(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if modCase == nil {
		response.NotFound(c, "Case not found")
		return
	}

	response.OK(c, modCase)
}

type actionCaseRequest struct {
	ActionType string `json:"action_type" binding:"required"`
	Details    string `json:"details"`
}

func (h *ModerationHandler) ActionCase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid case ID")
		return
	}

	var req actionCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	modID := middleware.GetAuthUserID(c)
	modCase, err := h.svc.ActionCase(c.Request.Context(), id, modID, req.ActionType, req.Details)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "ACTION_FAILED")
		return
	}

	response.OK(c, modCase)
}

type applyBanRequest struct {
	UserID       uuid.UUID `json:"user_id" binding:"required"`
	BanType      string    `json:"ban_type" binding:"required"`
	IsPermanent  bool      `json:"is_permanent"`
	DurationDays int       `json:"duration_days"`
	Reason       string    `json:"reason" binding:"required"`
}

func (h *ModerationHandler) ApplyBan(c *gin.Context) {
	var req applyBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	modID := middleware.GetAuthUserID(c)
	ban, err := h.svc.ApplyBan(c.Request.Context(), modID, service.BanInput{
		UserID:       req.UserID,
		BanType:      req.BanType,
		IsPermanent:  req.IsPermanent,
		DurationDays: req.DurationDays,
		Reason:       req.Reason,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "BAN_FAILED")
		return
	}

	response.Created(c, ban)
}

func (h *ModerationHandler) RevokeBan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ban ID")
		return
	}

	modID := middleware.GetAuthUserID(c)
	ban, err := h.svc.RevokeBan(c.Request.Context(), id, modID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REVOKE_BAN_FAILED")
		return
	}

	response.OK(c, ban)
}
