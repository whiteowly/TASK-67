package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

// PaymentCallbackInput holds the data from a payment gateway callback.
type PaymentCallbackInput struct {
	GatewayTxID      string
	MerchantOrderRef string
	Amount           float64
	Signature        string
}

// PaymentCallbackResult holds the result of processing a payment callback.
type PaymentCallbackResult struct {
	OrderID   uuid.UUID `json:"order_id"`
	Status    string    `json:"status"`
	PaymentID uuid.UUID `json:"payment_id"`
}

type PaymentService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
	// merchantKey is used for verifying callback signatures.
	merchantKey []byte
}

func NewPaymentService(repos *repo.Repositories, auditSvc *AuditService, merchantKey string) *PaymentService {
	return &PaymentService{
		repos:       repos,
		auditSvc:    auditSvc,
		merchantKey: []byte(merchantKey),
	}
}

// ProcessCallback handles a payment gateway callback.
// It verifies the signature, is idempotent by gateway_tx_id, confirms the payment,
// and transitions the order to Paid.
func (s *PaymentService) ProcessCallback(ctx context.Context, input PaymentCallbackInput) (*PaymentCallbackResult, error) {
	amountMinor := int64(input.Amount * 100)

	// Verify signature
	if !s.verifySignature(input.GatewayTxID, input.MerchantOrderRef, amountMinor, input.Signature) {
		return nil, fmt.Errorf("invalid payment signature")
	}

	// Idempotency: check if this gateway tx was already processed
	existing, err := s.repos.Payment.GetByGatewayTxID(ctx, input.GatewayTxID)
	if err != nil {
		return nil, fmt.Errorf("check gateway tx: %w", err)
	}
	if existing != nil {
		return &PaymentCallbackResult{
			OrderID:   existing.OrderID,
			Status:    existing.Status,
			PaymentID: existing.ID,
		}, nil
	}

	// Find the payment request by merchant ref
	pr, err := s.repos.Payment.GetRequestByMerchantRef(ctx, input.MerchantOrderRef)
	if err != nil {
		return nil, fmt.Errorf("get payment request: %w", err)
	}
	if pr == nil {
		return nil, fmt.Errorf("payment request not found for ref: %s", input.MerchantOrderRef)
	}

	// Validate amount matches
	if pr.Amount != amountMinor {
		return nil, fmt.Errorf("payment amount mismatch: expected %d, got %d", pr.Amount, amountMinor)
	}

	now := time.Now().UTC()

	// Create payment record (idempotent via ON CONFLICT in repo)
	paymentID := uuid.New()
	payment := &model.Payment{
		ID:               paymentID,
		OrderID:          pr.OrderID,
		PaymentRequestID: &pr.ID,
		Amount:           amountMinor,
		Currency:         pr.Currency,
		Status:           "confirmed",
		Method:           "qr_payment",
		GatewayTxID:      &input.GatewayTxID,
		Verified:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repos.Payment.CreatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// Update payment request
	pr.Status = "confirmed"
	pr.ConfirmedAt = &now
	pr.GatewayTxID = &input.GatewayTxID
	pr.UpdatedAt = now
	if err := s.repos.Payment.UpdatePaymentRequest(ctx, pr); err != nil {
		return nil, fmt.Errorf("update payment request: %w", err)
	}

	// Transition order to Paid
	order, err := s.repos.Order.GetOrderByID(ctx, pr.OrderID)
	if err != nil || order == nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	oldStatus := order.Status
	if err := s.repos.Order.UpdateOrderStatus(ctx, order.ID, oldStatus, model.OrderStatusPaid, order.UserID); err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "payment_confirmed",
		Resource:   "order",
		ResourceID: strPtr(order.ID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.OrderStatusPaid, "gateway_tx_id": input.GatewayTxID},
	})

	return &PaymentCallbackResult{
		OrderID:   pr.OrderID,
		Status:    model.OrderStatusPaid,
		PaymentID: paymentID,
	}, nil
}

// ExpirePayments finds expired payment requests and closes the associated orders.
func (s *PaymentService) ExpirePayments(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	expired, err := s.repos.Payment.FindExpiredRequests(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("find expired payments: %w", err)
	}

	count := 0
	for _, pr := range expired {
		pr.Status = "expired"
		pr.UpdatedAt = now
		if err := s.repos.Payment.UpdatePaymentRequest(ctx, &pr); err != nil {
			continue
		}

		order, err := s.repos.Order.GetOrderByID(ctx, pr.OrderID)
		if err != nil || order == nil {
			continue
		}
		if order.Status != model.OrderStatusAwaitingPayment {
			continue
		}

		oldStatus := order.Status
		if err := s.repos.Order.UpdateOrderStatus(ctx, order.ID, oldStatus, model.OrderStatusAutoClosed, order.UserID); err != nil {
			continue
		}

		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "system",
			Action:     "expire_payment",
			Resource:   "order",
			ResourceID: strPtr(order.ID.String()),
			OldState:   map[string]interface{}{"status": oldStatus},
			NewState:   map[string]interface{}{"status": model.OrderStatusAutoClosed, "reason": "payment_expired"},
		})

		count++
	}

	return count, nil
}

// InitiateRefund creates a refund request for an order.
func (s *PaymentService) InitiateRefund(ctx context.Context, orderID uuid.UUID, amount int64, reason string, initiatedBy uuid.UUID) (*model.Refund, error) {
	order, err := s.repos.Order.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("order not found")
	}

	if order.Status != model.OrderStatusPaid && order.Status != model.OrderStatusDelivered {
		return nil, fmt.Errorf("order is not eligible for refund in status: %s", order.Status)
	}

	payment, err := s.repos.Payment.GetConfirmedByOrderID(ctx, orderID)
	if err != nil || payment == nil {
		return nil, fmt.Errorf("no confirmed payment found for order")
	}

	if amount > payment.Amount {
		return nil, fmt.Errorf("refund amount exceeds payment amount")
	}

	now := time.Now().UTC()
	refund := &model.Refund{
		ID:          uuid.New(),
		OrderID:     orderID,
		PaymentID:   &payment.ID,
		Amount:      amount,
		Currency:    order.Currency,
		Status:      "pending",
		Reason:      &reason,
		InitiatedBy: &initiatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repos.Payment.CreateRefund(ctx, refund); err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	oldStatus := order.Status
	s.repos.Order.UpdateOrderStatus(ctx, order.ID, oldStatus, model.OrderStatusRefundPending, initiatedBy)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &initiatedBy,
		Action:     "initiate_refund",
		Resource:   "order",
		ResourceID: strPtr(orderID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.OrderStatusRefundPending, "refund_amount": amount, "reason": reason},
	})

	return refund, nil
}

// ReconcileRefund updates a refund status and syncs the order state.
func (s *PaymentService) ReconcileRefund(ctx context.Context, refundID uuid.UUID, newStatus string) error {
	refund, err := s.repos.Payment.GetRefundByID(ctx, refundID)
	if err != nil || refund == nil {
		return fmt.Errorf("refund not found")
	}

	if refund.Status == newStatus {
		return nil // idempotent
	}

	validTransitions := map[string][]string{
		"pending":    {"processing", "completed", "failed"},
		"processing": {"completed", "failed"},
	}
	allowed := validTransitions[refund.Status]
	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid refund transition from %s to %s", refund.Status, newStatus)
	}

	if err := s.repos.Payment.UpdateRefundStatus(ctx, refundID, newStatus); err != nil {
		return fmt.Errorf("update refund: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "system",
		Action:     "reconcile_refund",
		Resource:   "refund",
		ResourceID: strPtr(refundID.String()),
		NewState:   map[string]interface{}{"status": newStatus},
	})

	// Sync order status based on aggregate refund state
	if newStatus == "completed" || newStatus == "failed" {
		return s.syncOrderRefundState(ctx, refund.OrderID)
	}
	return nil
}

func (s *PaymentService) syncOrderRefundState(ctx context.Context, orderID uuid.UUID) error {
	order, err := s.repos.Order.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return nil
	}

	refunds, err := s.repos.Payment.ListRefundsByOrder(ctx, orderID)
	if err != nil {
		return nil
	}

	var totalRefunded int64
	allCompleted := true
	for _, r := range refunds {
		if r.Status == "completed" {
			totalRefunded += r.Amount
		}
		if r.Status != "completed" && r.Status != "failed" {
			allCompleted = false
		}
	}

	if !allCompleted {
		return nil
	}

	var newOrderStatus string
	if totalRefunded >= order.Total {
		newOrderStatus = model.OrderStatusRefundedFull
	} else if totalRefunded > 0 {
		newOrderStatus = model.OrderStatusRefundedPartial
	}

	if newOrderStatus != "" && newOrderStatus != order.Status {
		s.repos.Order.UpdateOrderStatus(ctx, orderID, order.Status, newOrderStatus, order.UserID)

		s.auditSvc.Log(ctx, &model.AuditEntry{
			ActorType:  "system",
			Action:     "refund_order_sync",
			Resource:   "order",
			ResourceID: strPtr(orderID.String()),
			OldState:   map[string]interface{}{"status": order.Status},
			NewState:   map[string]interface{}{"status": newOrderStatus, "refunded_amount": totalRefunded},
		})
	}
	return nil
}

// verifySignature verifies the HMAC-SHA256 signature of a payment callback.
func (s *PaymentService) verifySignature(gatewayTxID, merchantRef string, amount int64, signature string) bool {
	if len(s.merchantKey) == 0 {
		return false
	}

	message := fmt.Sprintf("%s|%s|%d", gatewayTxID, merchantRef, amount)
	mac := hmac.New(sha256.New, s.merchantKey)
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}
