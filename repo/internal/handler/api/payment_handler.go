package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

type paymentCallbackRequest struct {
	GatewayTxID      string  `json:"gateway_tx_id" binding:"required"`
	MerchantOrderRef string  `json:"merchant_order_ref" binding:"required"`
	Amount           float64 `json:"amount" binding:"required"`
	Signature        string  `json:"signature" binding:"required"`
}

func (h *PaymentHandler) Callback(c *gin.Context) {
	var req paymentCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	result, err := h.svc.ProcessCallback(c.Request.Context(), service.PaymentCallbackInput{
		GatewayTxID:      req.GatewayTxID,
		MerchantOrderRef: req.MerchantOrderRef,
		Amount:           req.Amount,
		Signature:        req.Signature,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, "CALLBACK_FAILED", "payment callback processing failed")
		return
	}

	response.OK(c, result)
}
