package service

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

// PODInput holds the input data for recording proof of delivery.
type PODInput struct {
	ProofType          string
	SignatureData      string
	AcknowledgmentText string
	ReceiverName       string
}

type ShipmentService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewShipmentService(repos *repo.Repositories, auditSvc *AuditService) *ShipmentService {
	return &ShipmentService{repos: repos, auditSvc: auditSvc}
}

// CreateShipment creates a shipment for a paid order with shippable items.
func (s *ShipmentService) CreateShipment(ctx context.Context, orderID, staffID uuid.UUID) (*model.Shipment, error) {
	order, err := s.repos.Order.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("order not found")
	}

	if order.Status != model.OrderStatusPaid {
		return nil, fmt.Errorf("order must be paid before creating shipment, current status: %s", order.Status)
	}
	if !order.HasShippable {
		return nil, fmt.Errorf("order has no shippable items")
	}

	now := time.Now().UTC()
	shipment := &model.Shipment{
		ID:        uuid.New(),
		OrderID:   orderID,
		Status:    model.ShipmentStatusPendingFulfillment,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repos.Shipment.CreateShipment(ctx, shipment); err != nil {
		return nil, fmt.Errorf("create shipment: %w", err)
	}

	// Update order status to fulfillment pending
	oldStatus := order.Status
	s.repos.Order.UpdateOrderStatus(ctx, order.ID, oldStatus, model.OrderStatusFulfillmentPending, staffID)

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &staffID,
		Action:     "create_shipment",
		Resource:   "shipment",
		ResourceID: strPtr(shipment.ID.String()),
		NewState:   map[string]interface{}{"status": model.ShipmentStatusPendingFulfillment, "order_id": orderID, "order_old_status": oldStatus},
	})

	return shipment, nil
}

// UpdateShipmentStatus transitions a shipment to a new status.
func (s *ShipmentService) UpdateShipmentStatus(ctx context.Context, shipmentID uuid.UUID, newStatus string, staffID uuid.UUID) error {
	shipment, err := s.repos.Shipment.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		return fmt.Errorf("get shipment: %w", err)
	}
	if shipment == nil {
		return fmt.Errorf("shipment not found")
	}

	// Validate transition
	if !isValidShipmentTransition(shipment.Status, newStatus) {
		return fmt.Errorf("invalid shipment transition from %s to %s", shipment.Status, newStatus)
	}

	oldStatus := shipment.Status
	if err := s.repos.Shipment.UpdateShipmentStatus(ctx, shipmentID, oldStatus, newStatus, &staffID); err != nil {
		return fmt.Errorf("update shipment: %w", err)
	}

	// Sync order status
	if newStatus == model.ShipmentStatusShipped || newStatus == model.ShipmentStatusDelivered {
		order, err := s.repos.Order.GetOrderByID(ctx, shipment.OrderID)
		if err == nil && order != nil {
			var orderStatus string
			switch newStatus {
			case model.ShipmentStatusShipped:
				orderStatus = model.OrderStatusShipped
			case model.ShipmentStatusDelivered:
				orderStatus = model.OrderStatusDelivered
			}
			if orderStatus != "" {
				s.repos.Order.UpdateOrderStatus(ctx, order.ID, order.Status, orderStatus, staffID)
			}
		}
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &staffID,
		Action:     "update_shipment_status",
		Resource:   "shipment",
		ResourceID: strPtr(shipmentID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": newStatus},
	})

	return nil
}

// RecordDeliveryProof records proof of delivery for a shipment.
func (s *ShipmentService) RecordDeliveryProof(ctx context.Context, shipmentID uuid.UUID, proofType string, data []byte, acknowledgmentText string, receiverName string, staffID uuid.UUID) (*model.DeliveryProof, error) {
	shipment, err := s.repos.Shipment.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("get shipment: %w", err)
	}
	if shipment == nil {
		return nil, fmt.Errorf("shipment not found")
	}

	// Validate proof requirements
	if proofType == "" {
		return nil, fmt.Errorf("proof type is required")
	}
	if proofType == "signature" && len(data) == 0 {
		return nil, fmt.Errorf("signature data is required for signature proof")
	}
	if proofType == "acknowledgment" && receiverName == "" {
		return nil, fmt.Errorf("receiver name is required for acknowledgment proof")
	}
	if proofType == "typed_acknowledgment" && acknowledgmentText == "" {
		return nil, fmt.Errorf("acknowledgment text is required for typed_acknowledgment proof")
	}

	now := time.Now().UTC()
	proof := &model.DeliveryProof{
		ID:                 uuid.New(),
		ShipmentID:         shipmentID,
		ProofType:          proofType,
		SignatureData:      data,
		AcknowledgmentText: nilIfEmpty(acknowledgmentText),
		ReceiverName:       nilIfEmpty(receiverName),
		DeliveredAt:        now,
		RecordedBy:         staffID,
		CreatedAt:          now,
	}

	if err := s.repos.Shipment.CreateDeliveryProof(ctx, proof); err != nil {
		return nil, fmt.Errorf("create delivery proof: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &staffID,
		Action:     "record_delivery_proof",
		Resource:   "shipment",
		ResourceID: strPtr(shipmentID.String()),
		NewState:   map[string]interface{}{"proof_type": proofType, "receiver_name": receiverName},
	})

	return proof, nil
}

// ReportDeliveryException reports a delivery exception on a shipment.
func (s *ShipmentService) ReportDeliveryException(ctx context.Context, shipmentID uuid.UUID, exType, description string, staffID uuid.UUID) (*model.DeliveryException, error) {
	shipment, err := s.repos.Shipment.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("get shipment: %w", err)
	}
	if shipment == nil {
		return nil, fmt.Errorf("shipment not found")
	}

	if exType == "" {
		return nil, fmt.Errorf("exception type is required")
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}

	now := time.Now().UTC()
	exception := &model.DeliveryException{
		ID:            uuid.New(),
		ShipmentID:    shipmentID,
		ExceptionType: exType,
		Description:   description,
		ReportedBy:    staffID,
		Resolved:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repos.Shipment.CreateDeliveryException(ctx, exception); err != nil {
		return nil, fmt.Errorf("create delivery exception: %w", err)
	}

	// Update shipment status
	oldStatus := shipment.Status
	s.repos.Shipment.UpdateShipmentStatus(ctx, shipmentID, oldStatus, model.ShipmentStatusDeliveryException, &staffID)

	// Update order status
	order, err := s.repos.Order.GetOrderByID(ctx, shipment.OrderID)
	if err == nil && order != nil {
		s.repos.Order.UpdateOrderStatus(ctx, order.ID, order.Status, model.OrderStatusDeliveryException, staffID)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &staffID,
		Action:     "report_delivery_exception",
		Resource:   "shipment",
		ResourceID: strPtr(shipmentID.String()),
		OldState:   map[string]interface{}{"status": oldStatus},
		NewState:   map[string]interface{}{"status": model.ShipmentStatusDeliveryException, "exception_type": exType},
	})

	return exception, nil
}

// Create is an alias for CreateShipment used by handlers.
func (s *ShipmentService) Create(ctx context.Context, orderID, staffID uuid.UUID) (*model.Shipment, error) {
	return s.CreateShipment(ctx, orderID, staffID)
}

// UpdateStatus transitions a shipment to a new status. Returns the updated shipment.
func (s *ShipmentService) UpdateStatus(ctx context.Context, shipmentID, staffID uuid.UUID, newStatus string) (*model.Shipment, error) {
	if err := s.UpdateShipmentStatus(ctx, shipmentID, newStatus, staffID); err != nil {
		return nil, err
	}
	return s.repos.Shipment.GetShipmentByID(ctx, shipmentID)
}

// RecordPOD records proof of delivery using PODInput.
func (s *ShipmentService) RecordPOD(ctx context.Context, shipmentID, staffID uuid.UUID, input PODInput) (*model.DeliveryProof, error) {
	return s.RecordDeliveryProof(ctx, shipmentID, input.ProofType, []byte(input.SignatureData), input.AcknowledgmentText, input.ReceiverName, staffID)
}

// ReportException reports a delivery exception.
func (s *ShipmentService) ReportException(ctx context.Context, shipmentID, staffID uuid.UUID, exType, description string) (*model.DeliveryException, error) {
	return s.ReportDeliveryException(ctx, shipmentID, exType, description, staffID)
}

// List returns paginated shipments filtered by status.
func (s *ShipmentService) List(ctx context.Context, status string, limit, offset int) ([]model.Shipment, int, error) {
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	return s.repos.Shipment.ListShipments(ctx, nil, statusPtr, limit, offset)
}

// ListShipments returns paginated shipments filtered by status.
func (s *ShipmentService) ListShipments(ctx context.Context, status *string, limit, offset int) ([]model.Shipment, int, error) {
	return s.repos.Shipment.ListShipments(ctx, nil, status, limit, offset)
}

// isValidShipmentTransition checks if a shipment status transition is allowed.
func isValidShipmentTransition(from, to string) bool {
	allowed := map[string][]string{
		model.ShipmentStatusPendingFulfillment: {model.ShipmentStatusPacked, model.ShipmentStatusCanceled},
		model.ShipmentStatusPacked:             {model.ShipmentStatusShipped, model.ShipmentStatusCanceled},
		model.ShipmentStatusShipped:            {model.ShipmentStatusDelivered, model.ShipmentStatusDeliveryException},
		model.ShipmentStatusDeliveryException:  {model.ShipmentStatusReturned, model.ShipmentStatusClosedException},
	}

	targets, ok := allowed[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
