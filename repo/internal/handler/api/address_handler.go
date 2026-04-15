package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AddressHandler struct {
	addressSvc *service.AddressService
}

func NewAddressHandler(addressSvc *service.AddressService) *AddressHandler {
	return &AddressHandler{addressSvc: addressSvc}
}

func (h *AddressHandler) List(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	addrs, err := h.addressSvc.List(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}
	if addrs == nil {
		response.OK(c, []struct{}{})
		return
	}
	response.OK(c, addrs)
}

type addressRequest struct {
	Label         string `json:"label"`
	RecipientName string `json:"recipient_name" binding:"required"`
	Phone         string `json:"phone" binding:"required"`
	Line1         string `json:"line1" binding:"required"`
	Line2         string `json:"line2"`
	City          string `json:"city" binding:"required"`
	State         string `json:"state"`
	PostalCode    string `json:"postal_code"`
	CountryCode   string `json:"country_code"`
	IsDefault     bool   `json:"is_default"`
}

func (h *AddressHandler) Create(c *gin.Context) {
	var req addressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	addr, err := h.addressSvc.Create(c.Request.Context(), userID, service.AddressInput{
		Label:         req.Label,
		RecipientName: req.RecipientName,
		Phone:         req.Phone,
		Line1:         req.Line1,
		Line2:         req.Line2,
		City:          req.City,
		State:         req.State,
		PostalCode:    req.PostalCode,
		CountryCode:   req.CountryCode,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "CREATE_FAILED")
		return
	}
	response.Created(c, addr)
}

func (h *AddressHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid address ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	addr, err := h.addressSvc.Get(c.Request.Context(), id, userID)
	if err != nil {
		response.NotFound(c, "Address not found")
		return
	}
	response.OK(c, addr)
}

func (h *AddressHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid address ID")
		return
	}

	var req addressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	addr, err := h.addressSvc.Update(c.Request.Context(), id, userID, service.AddressInput{
		Label:         req.Label,
		RecipientName: req.RecipientName,
		Phone:         req.Phone,
		Line1:         req.Line1,
		Line2:         req.Line2,
		City:          req.City,
		State:         req.State,
		PostalCode:    req.PostalCode,
		CountryCode:   req.CountryCode,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "UPDATE_FAILED")
		return
	}
	response.OK(c, addr)
}

func (h *AddressHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid address ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	if err := h.addressSvc.Delete(c.Request.Context(), id, userID); err != nil {
		response.NotFound(c, "Address not found")
		return
	}
	response.OK(c, gin.H{"message": "Address deleted"})
}
