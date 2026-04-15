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

type RegistrationHandler struct {
	svc *service.RegistrationService
}

func NewRegistrationHandler(svc *service.RegistrationService) *RegistrationHandler {
	return &RegistrationHandler{svc: svc}
}

type registerSessionRequest struct {
	SessionID uuid.UUID `json:"session_id" binding:"required"`
}

func (h *RegistrationHandler) Register(c *gin.Context) {
	var req registerSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	reg, err := h.svc.Register(c.Request.Context(), userID, req.SessionID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REGISTRATION_FAILED")
		return
	}

	response.Created(c, reg)
}

type cancelRegistrationRequest struct {
	Reason string `json:"reason"`
}

func (h *RegistrationHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid registration ID")
		return
	}

	var req cancelRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	reg, err := h.svc.Cancel(c.Request.Context(), id, userID, req.Reason)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "CANCEL_FAILED")
		return
	}

	response.OK(c, reg)
}

func (h *RegistrationHandler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid registration ID")
		return
	}

	approverID := middleware.GetAuthUserID(c)
	reg, err := h.svc.Approve(c.Request.Context(), id, approverID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "APPROVE_FAILED")
		return
	}

	response.OK(c, reg)
}

type rejectRegistrationRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *RegistrationHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid registration ID")
		return
	}

	var req rejectRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	rejectorID := middleware.GetAuthUserID(c)
	reg, err := h.svc.Reject(c.Request.Context(), id, rejectorID, req.Reason)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REJECT_FAILED")
		return
	}

	response.OK(c, reg)
}

func (h *RegistrationHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid registration ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	reg, err := h.svc.GetRegistration(c.Request.Context(), id, userID, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REGISTRATION_ERROR")
		return
	}
	if reg == nil {
		response.NotFound(c, "Registration not found")
		return
	}

	response.OK(c, reg)
}

type adminOverrideRegisterRequest struct {
	UserID    uuid.UUID `json:"user_id" binding:"required"`
	SessionID uuid.UUID `json:"session_id" binding:"required"`
	Reason    string    `json:"reason"`
}

func (h *RegistrationHandler) AdminOverrideRegister(c *gin.Context) {
	var req adminOverrideRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	adminID := middleware.GetAuthUserID(c)
	reg, err := h.svc.AdminOverrideRegister(c.Request.Context(), req.UserID, req.SessionID, adminID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "OVERRIDE_REGISTER_FAILED")
		return
	}

	response.Created(c, reg)
}

func (h *RegistrationHandler) ListMine(c *gin.Context) {
	pg := util.ParsePagination(c)
	userID := middleware.GetAuthUserID(c)

	regs, total, err := h.svc.ListUserRegistrations(c.Request.Context(), userID, pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if regs == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, regs, pg.Page, pg.PerPage, total)
}
