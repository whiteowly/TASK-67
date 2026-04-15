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

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) GetCart(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)

	cart, items, err := h.svc.GetCart(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}

	response.OK(c, gin.H{"cart": cart, "items": items})
}

type addToCartRequest struct {
	ItemType string `json:"item_type" binding:"required"`
	ItemID   uuid.UUID `json:"item_id" binding:"required"`
	Quantity int    `json:"quantity" binding:"required"`
}

func (h *OrderHandler) AddToCart(c *gin.Context) {
	var req addToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	cart, err := h.svc.AddToCart(c.Request.Context(), userID, req.ItemType, req.ItemID, req.Quantity)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "ADD_TO_CART_FAILED")
		return
	}

	response.Created(c, cart)
}

func (h *OrderHandler) RemoveFromCart(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid cart item ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	if err := h.svc.RemoveFromCart(c.Request.Context(), userID, id); err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REMOVE_FROM_CART_FAILED")
		return
	}

	response.OK(c, gin.H{"message": "Item removed from cart"})
}

type checkoutRequest struct {
	AddressID      *uuid.UUID `json:"address_id"`
	IdempotencyKey string     `json:"idempotency_key" binding:"required"`
}

func (h *OrderHandler) Checkout(c *gin.Context) {
	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	order, err := h.svc.Checkout(c.Request.Context(), userID, req.AddressID, req.IdempotencyKey)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "CHECKOUT_FAILED")
		return
	}

	response.Created(c, order)
}

type buyNowRequest struct {
	ItemType  string     `json:"item_type" binding:"required"`
	ItemID    uuid.UUID  `json:"item_id" binding:"required"`
	Quantity  int        `json:"quantity" binding:"required"`
	AddressID *uuid.UUID `json:"address_id"`
}

func (h *OrderHandler) BuyNow(c *gin.Context) {
	var req buyNowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	order, err := h.svc.BuyNow(c.Request.Context(), userID, req.ItemType, req.ItemID, req.Quantity, req.AddressID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "BUY_NOW_FAILED")
		return
	}

	response.Created(c, order)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid order ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	order, err := h.svc.GetOrder(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "ORDER_ERROR")
		return
	}

	response.OK(c, order)
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	pg := util.ParsePagination(c)
	userID := middleware.GetAuthUserID(c)

	orders, total, err := h.svc.ListOrders(c.Request.Context(), userID, pg.PerPage, pg.Offset)
	if err != nil {
		response.InternalError(c)
		return
	}
	if orders == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}

	response.Paginated(c, orders, pg.Page, pg.PerPage, total)
}

func (h *OrderHandler) CreatePaymentRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid order ID")
		return
	}

	userID := middleware.GetAuthUserID(c)
	payReq, err := h.svc.CreatePaymentRequest(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "PAYMENT_REQUEST_FAILED")
		return
	}

	response.Created(c, payReq)
}
