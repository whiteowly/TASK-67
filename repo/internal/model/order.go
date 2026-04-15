package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	OrderStatusDraft              = "draft"
	OrderStatusAwaitingPayment    = "awaiting_payment"
	OrderStatusPaid               = "paid"
	OrderStatusFulfillmentPending = "fulfillment_pending"
	OrderStatusShipped            = "shipped"
	OrderStatusDelivered          = "delivered"
	OrderStatusAutoClosed         = "auto_closed"
	OrderStatusManuallyCanceled   = "manually_canceled"
	OrderStatusRefundPending      = "refund_pending"
	OrderStatusRefundedPartial    = "refunded_partial"
	OrderStatusRefundedFull       = "refunded_full"
	OrderStatusDeliveryException  = "delivery_exception"
	OrderStatusClosedException    = "closed_exception"
)

type Cart struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CartItem struct {
	ID            uuid.UUID `json:"id"`
	CartID        uuid.UUID `json:"cart_id"`
	ItemType      string    `json:"item_type"`
	ItemID        uuid.UUID `json:"item_id"`
	Quantity      int       `json:"quantity"`
	PriceSnapshot int64     `json:"price_snapshot"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Order struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	OrderNumber       string     `json:"order_number"`
	Status            string     `json:"status"`
	Subtotal          int64      `json:"subtotal"`
	Total             int64      `json:"total"`
	Currency          string     `json:"currency"`
	DeliveryAddressID *uuid.UUID `json:"delivery_address_id,omitempty"`
	HasShippable      bool       `json:"has_shippable"`
	IsBuyNow          bool       `json:"is_buy_now"`
	CloseReason       *string    `json:"close_reason,omitempty"`
	IdempotencyKey    *string    `json:"idempotency_key,omitempty"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type OrderItem struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	ItemType   string    `json:"item_type"`
	ItemID     uuid.UUID `json:"item_id"`
	ItemName   string    `json:"item_name"`
	Quantity   int       `json:"quantity"`
	UnitPrice  int64     `json:"unit_price"`
	LineTotal  int64     `json:"line_total"`
	Currency   string    `json:"currency"`
	IsShippable bool    `json:"is_shippable"`
	CreatedAt  time.Time `json:"created_at"`
}

type PaymentRequest struct {
	ID               uuid.UUID  `json:"id"`
	OrderID          uuid.UUID  `json:"order_id"`
	Amount           int64      `json:"amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	MerchantOrderRef string     `json:"merchant_order_ref"`
	QRPayload        *string    `json:"qr_payload,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty"`
	GatewayTxID      *string    `json:"gateway_tx_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Payment struct {
	ID               uuid.UUID  `json:"id"`
	OrderID          uuid.UUID  `json:"order_id"`
	PaymentRequestID *uuid.UUID `json:"payment_request_id,omitempty"`
	Amount           int64      `json:"amount"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	Method           string     `json:"method"`
	GatewayTxID      *string    `json:"gateway_tx_id,omitempty"`
	Verified         bool       `json:"verified"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Refund struct {
	ID              uuid.UUID  `json:"id"`
	OrderID         uuid.UUID  `json:"order_id"`
	PaymentID       *uuid.UUID `json:"payment_id,omitempty"`
	Amount          int64      `json:"amount"`
	Currency        string     `json:"currency"`
	Status          string     `json:"status"`
	Reason          *string    `json:"reason,omitempty"`
	InitiatedBy     *uuid.UUID `json:"initiated_by,omitempty"`
	GatewayRefundID *string    `json:"gateway_refund_id,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
