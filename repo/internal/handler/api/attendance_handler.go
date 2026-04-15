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

type AttendanceHandler struct {
	svc *service.AttendanceService
}

func NewAttendanceHandler(svc *service.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{svc: svc}
}

type checkInRequest struct {
	RegistrationID uuid.UUID `json:"registration_id" binding:"required"`
	Method         string    `json:"method" binding:"required"`
}

func (h *AttendanceHandler) CheckIn(c *gin.Context) {
	var req checkInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	staffID := middleware.GetAuthUserID(c)
	att, err := h.svc.CheckIn(c.Request.Context(), req.RegistrationID, &staffID, req.Method)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "CHECKIN_FAILED")
		return
	}

	response.Created(c, att)
}

type startLeaveRequest struct {
	RegistrationID uuid.UUID `json:"registration_id" binding:"required"`
}

func (h *AttendanceHandler) StartLeave(c *gin.Context) {
	var req startLeaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	leave, err := h.svc.StartLeave(c.Request.Context(), req.RegistrationID, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "LEAVE_FAILED")
		return
	}

	response.Created(c, leave)
}

func (h *AttendanceHandler) EndLeave(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid leave ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	err = h.svc.EndLeave(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "END_LEAVE_FAILED")
		return
	}

	response.OK(c, gin.H{"status": "ok"})
}

func (h *AttendanceHandler) ListExceptions(c *gin.Context) {
	pg := util.ParsePagination(c)

	exceptions, total, err := h.svc.ListExceptions(c.Request.Context(), pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if exceptions == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, exceptions, pg.Page, pg.PerPage, total)
}
