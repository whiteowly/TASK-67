package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ShipmentStatusPendingFulfillment = "pending_fulfillment"
	ShipmentStatusPacked             = "packed"
	ShipmentStatusShipped            = "shipped"
	ShipmentStatusDelivered          = "delivered"
	ShipmentStatusDeliveryException  = "delivery_exception"
	ShipmentStatusReturned           = "returned"
	ShipmentStatusClosedException    = "closed_exception"
	ShipmentStatusCanceled           = "canceled"
)

type Shipment struct {
	ID             uuid.UUID  `json:"id"`
	OrderID        uuid.UUID  `json:"order_id"`
	Status         string     `json:"status"`
	TrackingNumber *string    `json:"tracking_number,omitempty"`
	Carrier        *string    `json:"carrier,omitempty"`
	ShippedBy      *uuid.UUID `json:"shipped_by,omitempty"`
	ShippedAt      *time.Time `json:"shipped_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type DeliveryProof struct {
	ID                uuid.UUID  `json:"id"`
	ShipmentID        uuid.UUID  `json:"shipment_id"`
	ProofType         string     `json:"proof_type"`
	SignatureData     []byte     `json:"-"`
	AcknowledgmentText *string   `json:"acknowledgment_text,omitempty"`
	ReceiverName      *string    `json:"receiver_name,omitempty"`
	DeliveredAt       time.Time  `json:"delivered_at"`
	RecordedBy        uuid.UUID  `json:"recorded_by"`
	CreatedAt         time.Time  `json:"created_at"`
}

type DeliveryException struct {
	ID            uuid.UUID  `json:"id"`
	ShipmentID    uuid.UUID  `json:"shipment_id"`
	ExceptionType string     `json:"exception_type"`
	Description   string     `json:"description"`
	ReportedBy    uuid.UUID  `json:"reported_by"`
	Resolved      bool       `json:"resolved"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
