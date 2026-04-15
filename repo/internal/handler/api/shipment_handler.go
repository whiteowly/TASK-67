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

type ShipmentHandler struct {
	svc *service.ShipmentService
}

func NewShipmentHandler(svc *service.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{svc: svc}
}

type createShipmentRequest struct {
	OrderID uuid.UUID `json:"order_id" binding:"required"`
}

func (h *ShipmentHandler) Create(c *gin.Context) {
	var req createShipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	staffID := middleware.GetAuthUserID(c)
	shipment, err := h.svc.Create(c.Request.Context(), req.OrderID, staffID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "SHIPMENT_CREATE_FAILED")
		return
	}

	response.Created(c, shipment)
}

type updateShipmentStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *ShipmentHandler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid shipment ID")
		return
	}

	var req updateShipmentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	staffID := middleware.GetAuthUserID(c)
	shipment, err := h.svc.UpdateStatus(c.Request.Context(), id, staffID, req.Status)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "STATUS_UPDATE_FAILED")
		return
	}

	response.OK(c, shipment)
}

type recordPODRequest struct {
	ProofType         string `json:"proof_type" binding:"required"`
	SignatureData     string `json:"signature_data"`
	AcknowledgmentText string `json:"acknowledgment_text"`
	ReceiverName      string `json:"receiver_name" binding:"required"`
}

func (h *ShipmentHandler) RecordPOD(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid shipment ID")
		return
	}

	var req recordPODRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	staffID := middleware.GetAuthUserID(c)
	pod, err := h.svc.RecordPOD(c.Request.Context(), id, staffID, service.PODInput{
		ProofType:          req.ProofType,
		SignatureData:      req.SignatureData,
		AcknowledgmentText: req.AcknowledgmentText,
		ReceiverName:       req.ReceiverName,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "POD_FAILED")
		return
	}

	response.Created(c, pod)
}

type shipmentExceptionRequest struct {
	ExceptionType string `json:"exception_type" binding:"required"`
	Description   string `json:"description" binding:"required"`
}

func (h *ShipmentHandler) ReportException(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid shipment ID")
		return
	}

	var req shipmentExceptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	staffID := middleware.GetAuthUserID(c)
	exc, err := h.svc.ReportException(c.Request.Context(), id, staffID, req.ExceptionType, req.Description)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "EXCEPTION_FAILED")
		return
	}

	response.Created(c, exc)
}

func (h *ShipmentHandler) List(c *gin.Context) {
	pg := util.ParsePagination(c)
	status := c.Query("status")

	shipments, total, err := h.svc.List(c.Request.Context(), status, pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if shipments == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, shipments, pg.Page, pg.PerPage, total)
}
