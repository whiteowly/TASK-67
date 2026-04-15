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

type PaymentRepo struct {
	pool *pgxpool.Pool
}

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo {
	return &PaymentRepo{pool: pool}
}

// GetByGatewayTxID retrieves a payment by its gateway transaction ID.
func (r *PaymentRepo) GetByGatewayTxID(ctx context.Context, gatewayTxID string) (*model.Payment, error) {
	p := &model.Payment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, payment_request_id, amount, currency, status,
		       method, gateway_tx_id, verified, created_at, updated_at
		FROM payments WHERE gateway_tx_id = $1`, gatewayTxID,
	).Scan(
		&p.ID, &p.OrderID, &p.PaymentRequestID, &p.Amount, &p.Currency,
		&p.Status, &p.Method, &p.GatewayTxID, &p.Verified, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get payment by gateway tx id: %w", err)
	}
	return p, nil
}

// GetRequestByMerchantRef retrieves a payment request by its merchant order reference.
func (r *PaymentRepo) GetRequestByMerchantRef(ctx context.Context, merchantRef string) (*model.PaymentRequest, error) {
	pr := &model.PaymentRequest{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, amount, currency, status, merchant_order_ref,
		       qr_payload, expires_at, confirmed_at, gateway_tx_id, created_at, updated_at
		FROM payment_requests WHERE merchant_order_ref = $1`, merchantRef,
	).Scan(
		&pr.ID, &pr.OrderID, &pr.Amount, &pr.Currency, &pr.Status,
		&pr.MerchantOrderRef, &pr.QRPayload, &pr.ExpiresAt, &pr.ConfirmedAt,
		&pr.GatewayTxID, &pr.CreatedAt, &pr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get payment request by merchant ref: %w", err)
	}
	return pr, nil
}

// CreatePayment inserts a payment record.
func (r *PaymentRepo) CreatePayment(ctx context.Context, p *model.Payment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, payment_request_id, amount, currency, status,
			method, gateway_tx_id, verified, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.OrderID, p.PaymentRequestID, p.Amount, p.Currency, p.Status,
		p.Method, p.GatewayTxID, p.Verified, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// UpdatePaymentRequest updates a payment request.
func (r *PaymentRepo) UpdatePaymentRequest(ctx context.Context, pr *model.PaymentRequest) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE payment_requests SET status = $2, confirmed_at = $3, gateway_tx_id = $4,
			updated_at = $5
		WHERE id = $1`,
		pr.ID, pr.Status, pr.ConfirmedAt, pr.GatewayTxID, pr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update payment request: %w", err)
	}
	return nil
}

// FindExpiredRequests returns pending payment requests past their expiry time.
func (r *PaymentRepo) FindExpiredRequests(ctx context.Context, now time.Time) ([]model.PaymentRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, amount, currency, status, merchant_order_ref,
		       qr_payload, expires_at, confirmed_at, gateway_tx_id, created_at, updated_at
		FROM payment_requests
		WHERE status = 'pending' AND expires_at < $1`, now)
	if err != nil {
		return nil, fmt.Errorf("find expired requests: %w", err)
	}
	defer rows.Close()

	var requests []model.PaymentRequest
	for rows.Next() {
		var pr model.PaymentRequest
		if err := rows.Scan(
			&pr.ID, &pr.OrderID, &pr.Amount, &pr.Currency, &pr.Status,
			&pr.MerchantOrderRef, &pr.QRPayload, &pr.ExpiresAt, &pr.ConfirmedAt,
			&pr.GatewayTxID, &pr.CreatedAt, &pr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment request: %w", err)
		}
		requests = append(requests, pr)
	}
	return requests, rows.Err()
}

// GetConfirmedByOrderID returns the confirmed payment for an order.
func (r *PaymentRepo) GetConfirmedByOrderID(ctx context.Context, orderID uuid.UUID) (*model.Payment, error) {
	p := &model.Payment{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, payment_request_id, amount, currency, status,
		       method, gateway_tx_id, verified, created_at, updated_at
		FROM payments WHERE order_id = $1 AND status = 'confirmed'
		ORDER BY created_at DESC LIMIT 1`, orderID,
	).Scan(
		&p.ID, &p.OrderID, &p.PaymentRequestID, &p.Amount, &p.Currency,
		&p.Status, &p.Method, &p.GatewayTxID, &p.Verified, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get confirmed payment: %w", err)
	}
	return p, nil
}

// GetRefundByID retrieves a refund by its ID.
func (r *PaymentRepo) GetRefundByID(ctx context.Context, id uuid.UUID) (*model.Refund, error) {
	ref := &model.Refund{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, payment_id, amount, currency, status, reason,
		       initiated_by, gateway_refund_id, completed_at, created_at, updated_at
		FROM refunds WHERE id = $1`, id).Scan(
		&ref.ID, &ref.OrderID, &ref.PaymentID, &ref.Amount, &ref.Currency,
		&ref.Status, &ref.Reason, &ref.InitiatedBy, &ref.GatewayRefundID,
		&ref.CompletedAt, &ref.CreatedAt, &ref.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ref, nil
}

// UpdateRefundStatus updates a refund's status.
func (r *PaymentRepo) UpdateRefundStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refunds SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

// ListRefundsByOrder returns all refunds for a given order.
func (r *PaymentRepo) ListRefundsByOrder(ctx context.Context, orderID uuid.UUID) ([]model.Refund, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, payment_id, amount, currency, status, reason,
		       initiated_by, gateway_refund_id, completed_at, created_at, updated_at
		FROM refunds WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refunds []model.Refund
	for rows.Next() {
		var ref model.Refund
		rows.Scan(&ref.ID, &ref.OrderID, &ref.PaymentID, &ref.Amount, &ref.Currency,
			&ref.Status, &ref.Reason, &ref.InitiatedBy, &ref.GatewayRefundID,
			&ref.CompletedAt, &ref.CreatedAt, &ref.UpdatedAt)
		refunds = append(refunds, ref)
	}
	return refunds, rows.Err()
}

// CreateRefund inserts a refund record.
func (r *PaymentRepo) CreateRefund(ctx context.Context, refund *model.Refund) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refunds (id, order_id, payment_id, amount, currency, status,
			reason, initiated_by, gateway_refund_id, completed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		refund.ID, refund.OrderID, refund.PaymentID, refund.Amount, refund.Currency,
		refund.Status, refund.Reason, refund.InitiatedBy, refund.GatewayRefundID,
		refund.CompletedAt, refund.CreatedAt, refund.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refund: %w", err)
	}
	return nil
}
