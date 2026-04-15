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

type OrderRepo struct {
	pool *pgxpool.Pool
}

func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{pool: pool}
}

func (r *OrderRepo) Pool() *pgxpool.Pool { return r.pool }

// GetOrCreateCart returns the active cart for a user, creating one if it does not exist.
func (r *OrderRepo) GetOrCreateCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	cart := &model.Cart{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, is_active, created_at, updated_at
		FROM carts WHERE user_id = $1 AND is_active = true`, userID,
	).Scan(&cart.ID, &cart.UserID, &cart.IsActive, &cart.CreatedAt, &cart.UpdatedAt)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get cart: %w", err)
	}

	now := time.Now().UTC()
	cart = &model.Cart{
		ID:        uuid.New(),
		UserID:    userID,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO carts (id, user_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)`,
		cart.ID, cart.UserID, cart.IsActive, cart.CreatedAt, cart.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create cart: %w", err)
	}
	return cart, nil
}

func (r *OrderRepo) AddCartItem(ctx context.Context, item *model.CartItem) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cart_items (id, cart_id, item_type, item_id, quantity,
			price_snapshot, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		item.ID, item.CartID, item.ItemType, item.ItemID, item.Quantity,
		item.PriceSnapshot, item.Currency, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}
	return nil
}

func (r *OrderRepo) RemoveCartItem(ctx context.Context, cartID, itemID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cart_items WHERE id = $2 AND cart_id = $1`, cartID, itemID)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}
	return nil
}

func (r *OrderRepo) ListCartItems(ctx context.Context, cartID uuid.UUID) ([]model.CartItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, cart_id, item_type, item_id, quantity, price_snapshot,
		       currency, created_at, updated_at
		FROM cart_items WHERE cart_id = $1
		ORDER BY created_at ASC`, cartID)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}
	defer rows.Close()

	var items []model.CartItem
	for rows.Next() {
		var ci model.CartItem
		if err := rows.Scan(
			&ci.ID, &ci.CartID, &ci.ItemType, &ci.ItemID, &ci.Quantity,
			&ci.PriceSnapshot, &ci.Currency, &ci.CreatedAt, &ci.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, ci)
	}
	return items, rows.Err()
}

// CreateOrder creates an order with its items in a transaction.
func (r *OrderRepo) CreateOrder(ctx context.Context, order *model.Order, items []model.OrderItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, subtotal, total, currency,
			delivery_address_id, has_shippable, is_buy_now, close_reason, idempotency_key,
			paid_at, closed_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		order.ID, order.UserID, order.OrderNumber, order.Status, order.Subtotal,
		order.Total, order.Currency, order.DeliveryAddressID, order.HasShippable,
		order.IsBuyNow, order.CloseReason, order.IdempotencyKey, order.PaidAt,
		order.ClosedAt, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, item_type, item_id, item_name,
				quantity, unit_price, line_total, currency, is_shippable, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			item.ID, item.OrderID, item.ItemType, item.ItemID, item.ItemName,
			item.Quantity, item.UnitPrice, item.LineTotal, item.Currency,
			item.IsShippable, item.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}
	}

	// Insert initial status history
	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, NULL, $3, $4, $5)`,
		uuid.New(), order.ID, order.Status, order.UserID, order.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order status history: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateOrderStatus updates an order's status and inserts a history record.
func (r *OrderRepo) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, oldStatus, newStatus string, changedBy uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE orders SET status = $2, updated_at = $3 WHERE id = $1`,
		orderID, newStatus, now)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, old_status, new_status, actor_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), orderID, oldStatus, newStatus, changedBy, now,
	)
	if err != nil {
		return fmt.Errorf("insert order status history: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *OrderRepo) CreatePaymentRequest(ctx context.Context, pr *model.PaymentRequest) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payment_requests (id, order_id, amount, currency, status,
			merchant_order_ref, qr_payload, expires_at, confirmed_at, gateway_tx_id,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		pr.ID, pr.OrderID, pr.Amount, pr.Currency, pr.Status,
		pr.MerchantOrderRef, pr.QRPayload, pr.ExpiresAt, pr.ConfirmedAt,
		pr.GatewayTxID, pr.CreatedAt, pr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create payment request: %w", err)
	}
	return nil
}

// GetActivePaymentRequest returns the active (pending) payment request for an order.
func (r *OrderRepo) GetActivePaymentRequest(ctx context.Context, orderID uuid.UUID) (*model.PaymentRequest, error) {
	pr := &model.PaymentRequest{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, order_id, amount, currency, status, merchant_order_ref,
		       qr_payload, expires_at, confirmed_at, gateway_tx_id, created_at, updated_at
		FROM payment_requests
		WHERE order_id = $1 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`, orderID,
	).Scan(
		&pr.ID, &pr.OrderID, &pr.Amount, &pr.Currency, &pr.Status,
		&pr.MerchantOrderRef, &pr.QRPayload, &pr.ExpiresAt, &pr.ConfirmedAt,
		&pr.GatewayTxID, &pr.CreatedAt, &pr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active payment request: %w", err)
	}
	return pr, nil
}

// ConfirmPayment creates a payment record idempotently by gateway_tx_id.
func (r *OrderRepo) ConfirmPayment(ctx context.Context, payment *model.Payment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, order_id, payment_request_id, amount, currency, status,
			method, gateway_tx_id, verified, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (gateway_tx_id) DO NOTHING`,
		payment.ID, payment.OrderID, payment.PaymentRequestID, payment.Amount,
		payment.Currency, payment.Status, payment.Method, payment.GatewayTxID,
		payment.Verified, payment.CreatedAt, payment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("confirm payment: %w", err)
	}
	return nil
}

// ExpirePaymentRequests expires all pending payment requests that have passed their expiry time.
func (r *OrderRepo) ExpirePaymentRequests(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE payment_requests SET status = 'expired', updated_at = $1
		WHERE status = 'pending' AND expires_at < $1`, now)
	if err != nil {
		return 0, fmt.Errorf("expire payment requests: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *OrderRepo) CreateRefund(ctx context.Context, refund *model.Refund) error {
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

// GenerateOrderNumber generates a sequential order number in format ORD-YYYYMMDD-XXXXX.
func (r *OrderRepo) GenerateOrderNumber(ctx context.Context) (string, error) {
	today := time.Now().UTC().Format("20060102")
	var seq int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) + 1 FROM orders
		WHERE order_number LIKE $1`, "ORD-"+today+"-%",
	).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}
	return fmt.Sprintf("ORD-%s-%05d", today, seq), nil
}

// ListOrders returns paginated orders for a user.
func (r *OrderRepo) ListOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Order, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, order_number, status, subtotal, total, currency,
		       delivery_address_id, has_shippable, is_buy_now, close_reason,
		       idempotency_key, paid_at, closed_at, created_at, updated_at
		FROM orders WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.OrderNumber, &o.Status, &o.Subtotal, &o.Total,
			&o.Currency, &o.DeliveryAddressID, &o.HasShippable, &o.IsBuyNow,
			&o.CloseReason, &o.IdempotencyKey, &o.PaidAt, &o.ClosedAt,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// GetByID is an alias for GetOrderByID used by services.
func (r *OrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	return r.GetOrderByID(ctx, id)
}

// UpdateStatus updates an order's status fields.
func (r *OrderRepo) UpdateStatus(ctx context.Context, order *model.Order) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE orders SET status = $2, paid_at = $3, closed_at = $4,
			close_reason = $5, updated_at = $6
		WHERE id = $1`,
		order.ID, order.Status, order.PaidAt, order.ClosedAt,
		order.CloseReason, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}

// UpsertCartItem inserts or updates a cart item.
func (r *OrderRepo) UpsertCartItem(ctx context.Context, item *model.CartItem) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cart_items (id, cart_id, item_type, item_id, quantity,
			price_snapshot, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (cart_id, item_type, item_id) DO UPDATE
		SET quantity = EXCLUDED.quantity, price_snapshot = EXCLUDED.price_snapshot,
		    updated_at = EXCLUDED.updated_at`,
		item.ID, item.CartID, item.ItemType, item.ItemID, item.Quantity,
		item.PriceSnapshot, item.Currency, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert cart item: %w", err)
	}
	return nil
}

// GetActiveCart returns the active cart for a user, or nil if none exists.
func (r *OrderRepo) GetActiveCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	cart := &model.Cart{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, is_active, created_at, updated_at
		FROM carts WHERE user_id = $1 AND is_active = true`, userID,
	).Scan(&cart.ID, &cart.UserID, &cart.IsActive, &cart.CreatedAt, &cart.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active cart: %w", err)
	}
	return cart, nil
}

// GetOrderByIdempotencyKey retrieves an order by its idempotency key.
func (r *OrderRepo) GetOrderByIdempotencyKey(ctx context.Context, key string) (*model.Order, error) {
	o := &model.Order{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, order_number, status, subtotal, total, currency,
		       delivery_address_id, has_shippable, is_buy_now, close_reason,
		       idempotency_key, paid_at, closed_at, created_at, updated_at
		FROM orders WHERE idempotency_key = $1`, key,
	).Scan(
		&o.ID, &o.UserID, &o.OrderNumber, &o.Status, &o.Subtotal, &o.Total,
		&o.Currency, &o.DeliveryAddressID, &o.HasShippable, &o.IsBuyNow,
		&o.CloseReason, &o.IdempotencyKey, &o.PaidAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get order by idempotency key: %w", err)
	}
	return o, nil
}

// CreateOrderWithItems is an alias for CreateOrder used by services.
func (r *OrderRepo) CreateOrderWithItems(ctx context.Context, order *model.Order, items []model.OrderItem) error {
	return r.CreateOrder(ctx, order, items)
}

// DeactivateCart marks a cart as inactive.
func (r *OrderRepo) DeactivateCart(ctx context.Context, cartID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE carts SET is_active = false, updated_at = now()
		WHERE id = $1`, cartID)
	if err != nil {
		return fmt.Errorf("deactivate cart: %w", err)
	}
	return nil
}

// ListByUser returns paginated orders for a user (alias for ListOrders).
func (r *OrderRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Order, int, error) {
	return r.ListOrders(ctx, userID, limit, offset)
}

// ListOrderItems returns all items for an order.
func (r *OrderRepo) ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]model.OrderItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, item_type, item_id, item_name, quantity, unit_price,
		       line_total, currency, is_shippable, created_at
		FROM order_items WHERE order_id = $1
		ORDER BY created_at`, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		var i model.OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.ItemType, &i.ItemID, &i.ItemName,
			&i.Quantity, &i.UnitPrice, &i.LineTotal, &i.Currency, &i.IsShippable,
			&i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetOrderByID retrieves a single order by ID.
func (r *OrderRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	o := &model.Order{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, order_number, status, subtotal, total, currency,
		       delivery_address_id, has_shippable, is_buy_now, close_reason,
		       idempotency_key, paid_at, closed_at, created_at, updated_at
		FROM orders WHERE id = $1`, id,
	).Scan(
		&o.ID, &o.UserID, &o.OrderNumber, &o.Status, &o.Subtotal, &o.Total,
		&o.Currency, &o.DeliveryAddressID, &o.HasShippable, &o.IsBuyNow,
		&o.CloseReason, &o.IdempotencyKey, &o.PaidAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	return o, nil
}
