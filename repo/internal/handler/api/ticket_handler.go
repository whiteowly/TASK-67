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

type TicketHandler struct {
	svc *service.TicketService
}

func NewTicketHandler(svc *service.TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

type createTicketRequest struct {
	TicketType  string `json:"ticket_type" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	Priority    string `json:"priority" binding:"required"`
}

func (h *TicketHandler) Create(c *gin.Context) {
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	ticket, err := h.svc.Create(c.Request.Context(), userID, service.CreateTicketInput{
		TicketType:  req.TicketType,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "TICKET_CREATE_FAILED")
		return
	}

	response.Created(c, ticket)
}

func (h *TicketHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	ticket, err := h.svc.Get(c.Request.Context(), id, userID, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "TICKET_ERROR")
		return
	}

	response.OK(c, ticket)
}

func (h *TicketHandler) List(c *gin.Context) {
	pg := util.ParsePagination(c)
	status := c.Query("status")
	priority := c.Query("priority")
	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)

	tickets, total, err := h.svc.ListScoped(c.Request.Context(), userID, roles, status, priority, pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tickets == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, tickets, pg.Page, pg.PerPage, total)
}

type updateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	var req updateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	ticket, err := h.svc.UpdateStatus(c.Request.Context(), id, userID, req.Status, req.Reason, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "STATUS_UPDATE_FAILED")
		return
	}

	response.OK(c, ticket)
}

type assignTicketRequest struct {
	AssignedTo uuid.UUID `json:"assigned_to" binding:"required"`
}

func (h *TicketHandler) Assign(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	var req assignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	assignerID := middleware.GetAuthUserID(c)
	ticket, err := h.svc.Assign(c.Request.Context(), id, assignerID, req.AssignedTo)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "ASSIGN_FAILED")
		return
	}

	response.OK(c, ticket)
}

type addCommentRequest struct {
	Body       string `json:"body" binding:"required"`
	IsInternal bool   `json:"is_internal"`
}

func (h *TicketHandler) AddComment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	var req addCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	comment, err := h.svc.AddComment(c.Request.Context(), id, userID, req.Body, req.IsInternal, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "COMMENT_FAILED")
		return
	}

	response.Created(c, comment)
}

type resolveTicketRequest struct {
	ResolutionCode    string `json:"resolution_code" binding:"required"`
	ResolutionSummary string `json:"resolution_summary" binding:"required"`
}

func (h *TicketHandler) Resolve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	var req resolveTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	ticket, err := h.svc.Resolve(c.Request.Context(), id, userID, req.ResolutionCode, req.ResolutionSummary, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "RESOLVE_FAILED")
		return
	}

	response.OK(c, ticket)
}

func (h *TicketHandler) Close(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid ticket ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	roles := middleware.GetAuthRoles(c)
	ticket, err := h.svc.Close(c.Request.Context(), id, userID, roles)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "CLOSE_FAILED")
		return
	}

	response.OK(c, ticket)
}
