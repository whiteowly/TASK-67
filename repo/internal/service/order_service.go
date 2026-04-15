package service

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/google/uuid"
)

const (
	paymentRequestExpiry = 15 * time.Minute
)

type OrderService struct {
	repos    *repo.Repositories
	auditSvc *AuditService
}

func NewOrderService(repos *repo.Repositories, auditSvc *AuditService) *OrderService {
	return &OrderService{repos: repos, auditSvc: auditSvc}
}

// AddToCart adds an item to the user's active cart.
func (s *OrderService) AddToCart(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID, qty int) (*model.CartItem, error) {
	if qty <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	// Get or create active cart
	cart, err := s.repos.Order.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}

	// Look up item price
	price, currency, err := s.resolveItemPrice(ctx, itemType, itemID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	item := &model.CartItem{
		ID:            uuid.New(),
		CartID:        cart.ID,
		ItemType:      itemType,
		ItemID:        itemID,
		Quantity:      qty,
		PriceSnapshot: price,
		Currency:      currency,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repos.Order.AddCartItem(ctx, item); err != nil {
		return nil, fmt.Errorf("add to cart: %w", err)
	}

	return item, nil
}

// RemoveFromCart removes an item from the user's active cart.
func (s *OrderService) RemoveFromCart(ctx context.Context, userID uuid.UUID, cartItemID uuid.UUID) error {
	cart, err := s.repos.Order.GetActiveCart(ctx, userID)
	if err != nil {
		return fmt.Errorf("get cart: %w", err)
	}
	if cart == nil {
		return fmt.Errorf("no active cart")
	}

	if err := s.repos.Order.RemoveCartItem(ctx, cart.ID, cartItemID); err != nil {
		return fmt.Errorf("remove from cart: %w", err)
	}
	return nil
}

// GetCart returns the user's active cart with all items.
func (s *OrderService) GetCart(ctx context.Context, userID uuid.UUID) (*model.Cart, []model.CartItem, error) {
	cart, err := s.repos.Order.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get cart: %w", err)
	}

	items, err := s.repos.Order.ListCartItems(ctx, cart.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list cart items: %w", err)
	}

	return cart, items, nil
}

// Checkout validates the cart, creates an order in AwaitingPayment, and reserves stock.
func (s *OrderService) Checkout(ctx context.Context, userID uuid.UUID, addressID *uuid.UUID, idempotencyKey string) (*model.Order, error) {
	// Idempotency check: if same user submits same key, return existing order
	if idempotencyKey != "" {
		existing, err := s.repos.Order.GetOrderByIdempotencyKey(ctx, idempotencyKey)
		if err == nil && existing != nil && existing.UserID == userID {
			return existing, nil
		}
	}

	// Get active cart with items
	cart, err := s.repos.Order.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}

	items, err := s.repos.Order.ListCartItems(ctx, cart.ID)
	if err != nil || len(items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// Build order items and compute totals
	now := time.Now().UTC()
	orderID := uuid.New()
	var subtotal int64
	var hasShippable bool
	var orderItems []model.OrderItem

	for _, ci := range items {
		itemName, isShippable, err := s.resolveItemDetails(ctx, ci.ItemType, ci.ItemID)
		if err != nil {
			return nil, fmt.Errorf("validate item %s: %w", ci.ItemID, err)
		}
		if isShippable {
			hasShippable = true
		}

		lineTotal := ci.PriceSnapshot * int64(ci.Quantity)
		subtotal += lineTotal

		orderItems = append(orderItems, model.OrderItem{
			ID:          uuid.New(),
			OrderID:     orderID,
			ItemType:    ci.ItemType,
			ItemID:      ci.ItemID,
			ItemName:    itemName,
			Quantity:    ci.Quantity,
			UnitPrice:   ci.PriceSnapshot,
			LineTotal:   lineTotal,
			Currency:    ci.Currency,
			IsShippable: isShippable,
			CreatedAt:   now,
		})
	}

	// Validate address if shippable items exist
	if hasShippable {
		if addressID == nil {
			return nil, fmt.Errorf("delivery address is required for shippable items")
		}
		addr, err := s.repos.Address.GetByIDAndUser(ctx, *addressID, userID)
		if err != nil || addr == nil {
			return nil, fmt.Errorf("delivery address not found")
		}
	}

	orderNumber, err := s.repos.Order.GenerateOrderNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate order number: %w", err)
	}

	order := &model.Order{
		ID:                orderID,
		UserID:            userID,
		OrderNumber:       orderNumber,
		Status:            model.OrderStatusAwaitingPayment,
		Subtotal:          subtotal,
		Total:             subtotal,
		Currency:          items[0].Currency,
		DeliveryAddressID: addressID,
		HasShippable:      hasShippable,
		IsBuyNow:          false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if idempotencyKey != "" {
		order.IdempotencyKey = &idempotencyKey
	}

	// Create order with items in a transaction
	if err := s.repos.Order.CreateOrder(ctx, order, orderItems); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "checkout",
		Resource:   "order",
		ResourceID: strPtr(orderID.String()),
		NewState:   map[string]interface{}{"status": model.OrderStatusAwaitingPayment, "total": subtotal, "order_number": orderNumber},
	})

	return order, nil
}

// BuyNow creates an ephemeral checkout for a single item without mutating the cart.
func (s *OrderService) BuyNow(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID, qty int, addressID *uuid.UUID) (*model.Order, error) {
	if qty <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	price, currency, err := s.resolveItemPrice(ctx, itemType, itemID)
	if err != nil {
		return nil, err
	}

	itemName, isShippable, err := s.resolveItemDetails(ctx, itemType, itemID)
	if err != nil {
		return nil, err
	}

	if isShippable {
		if addressID == nil {
			return nil, fmt.Errorf("delivery address is required for shippable items")
		}
		addr, err := s.repos.Address.GetByIDAndUser(ctx, *addressID, userID)
		if err != nil || addr == nil {
			return nil, fmt.Errorf("delivery address not found")
		}
	}

	now := time.Now().UTC()
	orderID := uuid.New()
	lineTotal := price * int64(qty)

	orderNumber, err := s.repos.Order.GenerateOrderNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate order number: %w", err)
	}

	order := &model.Order{
		ID:                orderID,
		UserID:            userID,
		OrderNumber:       orderNumber,
		Status:            model.OrderStatusAwaitingPayment,
		Subtotal:          lineTotal,
		Total:             lineTotal,
		Currency:          currency,
		DeliveryAddressID: addressID,
		HasShippable:      isShippable,
		IsBuyNow:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	orderItems := []model.OrderItem{
		{
			ID:          uuid.New(),
			OrderID:     orderID,
			ItemType:    itemType,
			ItemID:      itemID,
			ItemName:    itemName,
			Quantity:    qty,
			UnitPrice:   price,
			LineTotal:   lineTotal,
			Currency:    currency,
			IsShippable: isShippable,
			CreatedAt:   now,
		},
	}

	if err := s.repos.Order.CreateOrder(ctx, order, orderItems); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &userID,
		Action:     "buy_now",
		Resource:   "order",
		ResourceID: strPtr(orderID.String()),
		NewState:   map[string]interface{}{"status": model.OrderStatusAwaitingPayment, "total": lineTotal, "order_number": orderNumber},
	})

	return order, nil
}

// CreatePaymentRequest generates a payment request with QR payload and 15min expiry.
func (s *OrderService) CreatePaymentRequest(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) (*model.PaymentRequest, error) {
	order, err := s.repos.Order.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	if order == nil {
		return nil, NotFound("order not found")
	}
	if order.UserID != userID {
		return nil, Forbidden("not authorized to access this order")
	}
	if order.Status != model.OrderStatusAwaitingPayment {
		return nil, fmt.Errorf("order is not awaiting payment")
	}

	now := time.Now().UTC()
	reqID := uuid.New()
	merchantRef := fmt.Sprintf("CR-%s-%s", order.OrderNumber, reqID.String()[:8])
	qrPayload := fmt.Sprintf("campusrec://pay?ref=%s&amount=%d&currency=%s", merchantRef, order.Total, order.Currency)

	pr := &model.PaymentRequest{
		ID:               reqID,
		OrderID:          orderID,
		Amount:           order.Total,
		Currency:         order.Currency,
		Status:           "pending",
		MerchantOrderRef: merchantRef,
		QRPayload:        &qrPayload,
		ExpiresAt:        now.Add(paymentRequestExpiry),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repos.Order.CreatePaymentRequest(ctx, pr); err != nil {
		return nil, fmt.Errorf("create payment request: %w", err)
	}

	s.auditSvc.Log(ctx, &model.AuditEntry{
		ActorType:  "user",
		ActorID:    &order.UserID,
		Action:     "create_payment_request",
		Resource:   "payment_request",
		ResourceID: strPtr(reqID.String()),
		NewState:   map[string]interface{}{"order_id": orderID, "amount": order.Total, "expires_at": pr.ExpiresAt},
	})

	return pr, nil
}

// GetOrder returns an order by ID.
func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) (*model.Order, error) {
	order, err := s.repos.Order.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	if order == nil {
		return nil, NotFound("order not found")
	}
	if order.UserID != userID {
		return nil, Forbidden("not authorized to access this order")
	}
	return order, nil
}

// ListOrders returns paginated orders for a user.
func (s *OrderService) ListOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Order, int, error) {
	return s.repos.Order.ListOrders(ctx, userID, limit, offset)
}

// GetOrderItems returns all items for an order.
func (s *OrderService) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]model.OrderItem, error) {
	return s.repos.Order.ListOrderItems(ctx, orderID)
}

// GetActivePaymentRequest returns the pending payment request for an order, if any.
func (s *OrderService) GetActivePaymentRequest(ctx context.Context, orderID uuid.UUID) (*model.PaymentRequest, error) {
	return s.repos.Order.GetActivePaymentRequest(ctx, orderID)
}

// resolveItemPrice looks up the price for an item by type.
func (s *OrderService) resolveItemPrice(ctx context.Context, itemType string, itemID uuid.UUID) (int64, string, error) {
	switch itemType {
	case "session":
		sess, err := s.repos.Catalog.GetSessionByID(ctx, itemID)
		if err != nil || sess == nil {
			return 0, "", fmt.Errorf("session not found")
		}
		return sess.PriceMinorUnits, sess.Currency, nil
	case "product":
		prod, err := s.repos.Catalog.GetProductByID(ctx, itemID)
		if err != nil || prod == nil {
			return 0, "", fmt.Errorf("product not found")
		}
		return prod.PriceMinorUnits, prod.Currency, nil
	default:
		return 0, "", fmt.Errorf("unsupported item type: %s", itemType)
	}
}

// resolveItemDetails returns the name and shippable flag for an item.
func (s *OrderService) resolveItemDetails(ctx context.Context, itemType string, itemID uuid.UUID) (string, bool, error) {
	switch itemType {
	case "session":
		sess, err := s.repos.Catalog.GetSessionByID(ctx, itemID)
		if err != nil || sess == nil {
			return "", false, fmt.Errorf("session not found")
		}
		return sess.Title, false, nil
	case "product":
		prod, err := s.repos.Catalog.GetProductByID(ctx, itemID)
		if err != nil || prod == nil {
			return "", false, fmt.Errorf("product not found")
		}
		return prod.Name, prod.IsShippable, nil
	default:
		return "", false, fmt.Errorf("unsupported item type: %s", itemType)
	}
}
