package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShipmentRepo struct {
	pool *pgxpool.Pool
}

func NewShipmentRepo(pool *pgxpool.Pool) *ShipmentRepo {
	return &ShipmentRepo{pool: pool}
}

func (r *ShipmentRepo) CreateShipment(ctx context.Context, s *model.Shipment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO shipments (id, order_id, status, tracking_number, carrier,
			shipped_by, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		s.ID, s.OrderID, s.Status, s.TrackingNumber, s.Carrier,
		s.ShippedBy, s.ShippedAt, s.DeliveredAt, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert shipment: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO shipment_status_history (id, shipment_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, NULL, $3, $4, $5)`,
		uuid.New(), s.ID, s.Status, s.ShippedBy, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert shipment status history: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateShipmentStatus updates the shipment status and records history.
func (r *ShipmentRepo) UpdateShipmentStatus(ctx context.Context, shipmentID uuid.UUID, oldStatus, newStatus string, changedBy *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE shipments SET status = $2, updated_at = $3 WHERE id = $1`,
		shipmentID, newStatus, now)
	if err != nil {
		return fmt.Errorf("update shipment status: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO shipment_status_history (id, shipment_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), shipmentID, oldStatus, newStatus, changedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert shipment status history: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *ShipmentRepo) CreateDeliveryProof(ctx context.Context, dp *model.DeliveryProof) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO delivery_proofs (id, shipment_id, proof_type, signature_data,
			acknowledgment_text, receiver_name, delivered_at, recorded_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		dp.ID, dp.ShipmentID, dp.ProofType, dp.SignatureData,
		dp.AcknowledgmentText, dp.ReceiverName, dp.DeliveredAt, dp.RecordedBy, dp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create delivery proof: %w", err)
	}
	return nil
}

func (r *ShipmentRepo) CreateDeliveryException(ctx context.Context, de *model.DeliveryException) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO delivery_exceptions (id, shipment_id, exception_type, description,
			reported_by, resolved, resolved_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		de.ID, de.ShipmentID, de.ExceptionType, de.Description,
		de.ReportedBy, de.Resolved, de.ResolvedAt, de.CreatedAt, de.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create delivery exception: %w", err)
	}
	return nil
}

// ListShipments returns paginated shipments filtered by order or by status.
func (r *ShipmentRepo) ListShipments(ctx context.Context, orderID *uuid.UUID, status *string, limit, offset int) ([]model.Shipment, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if orderID != nil {
		where += fmt.Sprintf(" AND order_id = $%d", argIdx)
		args = append(args, *orderID)
		argIdx++
	}
	if status != nil {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM shipments %s", where)
	err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count shipments: %w", err)
	}

	selectQ := fmt.Sprintf(`
		SELECT id, order_id, status, tracking_number, carrier, shipped_by,
		       shipped_at, delivered_at, created_at, updated_at
		FROM shipments %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list shipments: %w", err)
	}
	defer rows.Close()

	var shipments []model.Shipment
	for rows.Next() {
		var s model.Shipment
		if err := rows.Scan(
			&s.ID, &s.OrderID, &s.Status, &s.TrackingNumber, &s.Carrier,
			&s.ShippedBy, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan shipment: %w", err)
		}
		shipments = append(shipments, s)
	}
	return shipments, total, rows.Err()
}

// Create is an alias for CreateShipment used by services.
func (r *ShipmentRepo) Create(ctx context.Context, s *model.Shipment) error {
	return r.CreateShipment(ctx, s)
}

// GetByID is an alias for GetShipmentByID used by services.
func (r *ShipmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Shipment, error) {
	return r.GetShipmentByID(ctx, id)
}

// Update updates a shipment record.
func (r *ShipmentRepo) Update(ctx context.Context, s *model.Shipment) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE shipments SET status = $2, tracking_number = $3, carrier = $4,
			shipped_by = $5, shipped_at = $6, delivered_at = $7, updated_at = $8
		WHERE id = $1`,
		s.ID, s.Status, s.TrackingNumber, s.Carrier,
		s.ShippedBy, s.ShippedAt, s.DeliveredAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update shipment: %w", err)
	}
	return nil
}

// GetShipmentByID retrieves a shipment by ID.
func (r *ShipmentRepo) GetShipmentByID(ctx context.Context, id uuid.UUID) (*model.Shipment, error) {
	s := &model.Shipment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, status, tracking_number, carrier, shipped_by,
		       shipped_at, delivered_at, created_at, updated_at
		FROM shipments WHERE id = $1`, id,
	).Scan(
		&s.ID, &s.OrderID, &s.Status, &s.TrackingNumber, &s.Carrier,
		&s.ShippedBy, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get shipment: %w", err)
	}
	return s, nil
}
