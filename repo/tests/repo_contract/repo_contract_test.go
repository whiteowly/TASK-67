// Package repo_contract holds focused contract tests for
// internal/repo/* primitives that gate critical app behavior:
// idempotency, ownership filtering, and transition / uniqueness
// constraints.
//
// These tests assert exact DB-side behavior — what rows exist after a
// call, what error a duplicate insert returns, that ownership filters
// hide other users' rows — rather than the much weaker "non-nil" or
// "no error" pattern. They run against a real Postgres via the test
// runner's Docker compose harness, so the SQL constraints we depend on
// (unique indexes, FK cascades) are actually exercised.
//
// The package lives outside `internal/repo` to avoid the import cycle
// between the package under test and the testutil that already imports
// `internal/repo` to construct the wired router.
package repo_contract

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/tests/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ── shared fixture ─────────────────────────────────────────────────────────

type fixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	repos    *repo.Repositories
	user1    uuid.UUID
	user2    uuid.UUID
	productA uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	repos := repo.NewRepositories(pool)
	ctx := context.Background()

	// Per-test unique suffix to avoid colliding usernames across tests
	// (cleanTables logs but does not fail on FK-constrained deletes from
	// the users table, so users from prior tests can persist).
	uniq := uuid.NewString()[:8]

	// Create two distinct users for this test run.
	mkUser := func(username string) uuid.UUID {
		username = username + "-" + uniq
		u := &model.User{
			ID:           uuid.New(),
			Username:     username,
			DisplayName:  "Repo Contract " + username,
			PasswordHash: "x", // not used by the repo paths under test
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := repos.User.Create(ctx, u); err != nil {
			t.Fatalf("create user %s: %v", username, err)
		}
		return u.ID
	}

	// Create one published product so cart-item dedup tests have a real FK target.
	prod := &model.Product{
		ID:               uuid.New(),
		Name:             "Repo Contract Product",
		Description:      "x",
		ShortDescription: "x",
		PriceMinorUnits:  1000,
		Currency:         "CNY",
		IsShippable:      true,
		Status:           model.ProductStatusPublished,
		Tags:             []string{"contract"},
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := repos.Catalog.CreateProduct(ctx, prod); err != nil {
		t.Fatalf("create product: %v", err)
	}

	return &fixture{
		t:        t,
		pool:     pool,
		repos:    repos,
		user1:    mkUser("repo-contract-u1"),
		user2:    mkUser("repo-contract-u2"),
		productA: prod.ID,
	}
}

// ── 1. Address ownership filtering ─────────────────────────────────────────

// AddressRepo.GetByIDAndUser must enforce ownership at the SQL level: a
// row belonging to user1 must NOT be returned to user2, and vice versa.
// This is the central authz fence for the address surface.
func TestRepoContract_Address_OwnershipFiltering(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	a := &model.DeliveryAddress{
		ID:            uuid.New(),
		UserID:        f.user1,
		RecipientName: "Owner",
		Phone:         "1",
		Line1:         "1 Rd",
		City:          "BJ",
		CountryCode:   "CN",
		IsDefault:     false,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := f.repos.Address.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Owner sees the row.
	got, err := f.repos.Address.GetByIDAndUser(ctx, a.ID, f.user1)
	if err != nil {
		t.Fatalf("owner read: %v", err)
	}
	if got == nil {
		t.Fatal("owner read returned nil")
	}
	if got.ID != a.ID {
		t.Errorf("owner read: id mismatch")
	}

	// Other user MUST get nil (not the row, not an error — the contract is
	// "either yours or invisible").
	got, err = f.repos.Address.GetByIDAndUser(ctx, a.ID, f.user2)
	if err != nil {
		t.Fatalf("non-owner read returned error %v (must be nil)", err)
	}
	if got != nil {
		t.Errorf("non-owner read MUST return nil; got %+v", got)
	}
}

// SoftDelete must (1) hide the row from subsequent ownership reads and
// (2) return a domain "address not found" error on the second call so
// callers get deterministic idempotent semantics.
func TestRepoContract_Address_SoftDelete_IsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	a := &model.DeliveryAddress{
		ID: uuid.New(), UserID: f.user1, RecipientName: "X", Phone: "1",
		Line1: "1", City: "BJ", CountryCode: "CN",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := f.repos.Address.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First delete must succeed.
	if err := f.repos.Address.SoftDelete(ctx, a.ID, f.user1); err != nil {
		t.Fatalf("first SoftDelete: %v", err)
	}

	// Subsequent owner read must return nil (deleted_at IS NULL filter).
	got, err := f.repos.Address.GetByIDAndUser(ctx, a.ID, f.user1)
	if err != nil {
		t.Fatalf("post-delete read: %v", err)
	}
	if got != nil {
		t.Error("post-delete read MUST return nil")
	}

	// Second delete must return a domain error (not nil), so the handler
	// can map it to 404 / 4xx instead of pretending success.
	err = f.repos.Address.SoftDelete(ctx, a.ID, f.user1)
	if err == nil {
		t.Fatal("second SoftDelete must return an error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("second SoftDelete error must mention 'not found'; got %v", err)
	}
}

// SoftDelete with a non-owner user must NOT remove the row and must
// return a domain error. This prevents authz bypass at the repo layer.
func TestRepoContract_Address_SoftDelete_NonOwnerDenied(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	a := &model.DeliveryAddress{
		ID: uuid.New(), UserID: f.user1, RecipientName: "X", Phone: "1",
		Line1: "1", City: "BJ", CountryCode: "CN",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := f.repos.Address.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	// user2 attempts deletion — must fail.
	err := f.repos.Address.SoftDelete(ctx, a.ID, f.user2)
	if err == nil {
		t.Fatal("non-owner SoftDelete MUST return an error")
	}

	// Row must still exist for user1.
	got, err := f.repos.Address.GetByIDAndUser(ctx, a.ID, f.user1)
	if err != nil {
		t.Fatalf("owner read after attempted non-owner delete: %v", err)
	}
	if got == nil {
		t.Fatal("non-owner SoftDelete MUST NOT remove the row from the owner's view")
	}
}

// ── 2. Order idempotency (lookup + uniqueness) ─────────────────────────────

// GetOrderByIdempotencyKey returns nil for an unknown key (no error).
// After insert, a subsequent lookup returns the exact same row. A
// second insert with the same key violates the UNIQUE constraint,
// which is the SQL-level guarantee the service relies on for safe
// retries.
func TestRepoContract_Order_IdempotencyKey_LookupAndUniqueness(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	idemKey := "repo-contract-idem-" + uuid.New().String()

	// Miss returns nil, nil.
	got, err := f.repos.Order.GetOrderByIdempotencyKey(ctx, idemKey)
	if err != nil {
		t.Fatalf("miss returned error: %v", err)
	}
	if got != nil {
		t.Errorf("miss must return nil; got %+v", got)
	}

	// Insert two orders with the SAME idempotency key — the second must
	// fail with a unique-constraint violation.
	mkOrder := func() *model.Order {
		num, err := f.repos.Order.GenerateOrderNumber(ctx)
		if err != nil {
			t.Fatalf("generate order number: %v", err)
		}
		o := &model.Order{
			ID:             uuid.New(),
			UserID:         f.user1,
			OrderNumber:    num,
			Status:         model.OrderStatusAwaitingPayment,
			Subtotal:       1000,
			Total:          1000,
			Currency:       "CNY",
			HasShippable:   false,
			IsBuyNow:       false,
			IdempotencyKey: &idemKey,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		return o
	}

	first := mkOrder()
	if err := f.repos.Order.CreateOrder(ctx, first, nil); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Lookup returns the inserted row exactly.
	got, err = f.repos.Order.GetOrderByIdempotencyKey(ctx, idemKey)
	if err != nil {
		t.Fatalf("hit returned error: %v", err)
	}
	if got == nil || got.ID != first.ID {
		t.Errorf("hit must return the same order; want id=%s got=%+v", first.ID, got)
	}

	// Second insert with same key must error with a duplicate-key signal.
	second := mkOrder()
	err = f.repos.Order.CreateOrder(ctx, second, nil)
	if err == nil {
		t.Fatal("second CreateOrder with duplicate idempotency_key must fail")
	}
	if !looksLikeUniqueViolation(err) {
		t.Errorf("error must indicate a unique violation; got %v", err)
	}
}

// ── 3. Cart item dedup constraint ──────────────────────────────────────────

// idx_cart_item_dedup is a UNIQUE INDEX on (cart_id, item_type, item_id).
// Inserting the same (cart, type, id) twice MUST be rejected by the DB
// — that uniqueness is the deduplication contract the AddToCart service
// depends on.
func TestRepoContract_Cart_ItemDedupConstraint(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	cart, err := f.repos.Order.GetOrCreateCart(ctx, f.user1)
	if err != nil {
		t.Fatalf("get cart: %v", err)
	}

	mkItem := func() *model.CartItem {
		return &model.CartItem{
			ID:            uuid.New(),
			CartID:        cart.ID,
			ItemType:      "product",
			ItemID:        f.productA,
			Quantity:      1,
			PriceSnapshot: 1000,
			Currency:      "CNY",
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
	}

	if err := f.repos.Order.AddCartItem(ctx, mkItem()); err != nil {
		t.Fatalf("first AddCartItem: %v", err)
	}

	// Second insert of the same product into the same cart must error.
	err = f.repos.Order.AddCartItem(ctx, mkItem())
	if err == nil {
		t.Fatal("duplicate cart-item insert MUST be rejected by the dedup index")
	}
	if !looksLikeUniqueViolation(err) {
		t.Errorf("error must indicate a unique violation; got %v", err)
	}
}

// Cross-cart isolation: the same (item_type, item_id) is allowed in two
// DIFFERENT carts. The dedup is per-cart, not global.
func TestRepoContract_Cart_DedupIsPerCart(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	c1, err := f.repos.Order.GetOrCreateCart(ctx, f.user1)
	if err != nil {
		t.Fatalf("cart 1: %v", err)
	}
	c2, err := f.repos.Order.GetOrCreateCart(ctx, f.user2)
	if err != nil {
		t.Fatalf("cart 2: %v", err)
	}
	if c1.ID == c2.ID {
		t.Fatal("two distinct users must have distinct carts")
	}

	mk := func(cartID uuid.UUID) *model.CartItem {
		return &model.CartItem{
			ID: uuid.New(), CartID: cartID, ItemType: "product",
			ItemID: f.productA, Quantity: 1, PriceSnapshot: 1000, Currency: "CNY",
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
	}

	if err := f.repos.Order.AddCartItem(ctx, mk(c1.ID)); err != nil {
		t.Fatalf("cart 1 add: %v", err)
	}
	if err := f.repos.Order.AddCartItem(ctx, mk(c2.ID)); err != nil {
		t.Fatalf("cart 2 add (same product, different cart) MUST succeed: %v", err)
	}
}

// ── 4. Order status transition: history row is written atomically ──────────

// UpdateOrderStatus must atomically (a) update the order row and
// (b) insert an order_status_history row in the same transaction.
// If the history row is missing on success, audit visibility is broken.
func TestRepoContract_Order_StatusUpdateWritesHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	num, err := f.repos.Order.GenerateOrderNumber(ctx)
	if err != nil {
		t.Fatalf("generate number: %v", err)
	}
	o := &model.Order{
		ID:          uuid.New(),
		UserID:      f.user1,
		OrderNumber: num,
		Status:      model.OrderStatusAwaitingPayment,
		Subtotal:    500,
		Total:       500,
		Currency:    "CNY",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := f.repos.Order.CreateOrder(ctx, o, nil); err != nil {
		t.Fatalf("create order: %v", err)
	}

	if err := f.repos.Order.UpdateOrderStatus(ctx,
		o.ID, model.OrderStatusAwaitingPayment, model.OrderStatusPaid, f.user1); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// Order row must reflect the new status.
	current, err := f.repos.Order.GetOrderByID(ctx, o.ID)
	if err != nil || current == nil {
		t.Fatalf("get order after update: %v %+v", err, current)
	}
	if current.Status != model.OrderStatusPaid {
		t.Errorf("order.status: want %q got %q", model.OrderStatusPaid, current.Status)
	}

	// History row must exist with old+new status.
	var oldS, newS string
	err = f.pool.QueryRow(ctx, `
		SELECT old_status, new_status
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, o.ID).Scan(&oldS, &newS)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("no order_status_history row was written — atomicity contract broken")
	}
	if err != nil {
		t.Fatalf("history query: %v", err)
	}
	if oldS != model.OrderStatusAwaitingPayment {
		t.Errorf("history.old_status: want %q got %q", model.OrderStatusAwaitingPayment, oldS)
	}
	if newS != model.OrderStatusPaid {
		t.Errorf("history.new_status: want %q got %q", model.OrderStatusPaid, newS)
	}
}

// ── 5. Payment idempotency by gateway_tx_id ────────────────────────────────

// GetByGatewayTxID returns nil on miss (not an error) so the service
// can use it as an idempotency probe before a callback insert.
func TestRepoContract_Payment_GatewayTxIDLookup_MissReturnsNil(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	got, err := f.repos.Payment.GetByGatewayTxID(ctx, "no-such-tx-"+uuid.NewString())
	if err != nil {
		t.Fatalf("miss returned error: %v", err)
	}
	if got != nil {
		t.Errorf("miss must return nil; got %+v", got)
	}
}

// ── 6. Registration ownership / status transition ─────────────────────────

// UpdateRegistrationStatus must persist the new status AND write a
// matching status-history row. Missing history breaks the moderation
// + audit pipeline.
func TestRepoContract_Registration_StatusUpdateWritesHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Seed a session and inventory row by hand (bypassing the catalog
	// service so the test stays narrow).
	sessID := uuid.New()
	now := time.Now().UTC()
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO program_sessions
		    (id, title, description, status, seat_capacity, price_minor_units, currency,
		     start_at, end_at, requires_approval, allows_waitlist, created_at, updated_at)
		VALUES ($1, $2, '', 'published', 10, 0, 'CNY',
		        $3, $4, false, false, $5, $5)`,
		sessID, "Repo Contract Session", now.Add(48*time.Hour), now.Add(50*time.Hour), now); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO session_seat_inventory
		    (session_id, total_seats, reserved_seats, version, updated_at)
		VALUES ($1, 10, 0, 1, $2)`, sessID, now); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	regID := uuid.New()
	reg := &model.SessionRegistration{
		ID:           regID,
		UserID:       f.user1,
		SessionID:    sessID,
		Status:       model.RegStatusRegistered,
		RegisteredAt: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	initialReason := "initial"
	hist := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		OldStatus:      nil,
		NewStatus:      model.RegStatusRegistered,
		ActorType:      "user",
		ActorID:        &f.user1,
		ReasonCode:     &initialReason,
		CreatedAt:      now,
	}
	if err := f.repos.Registration.CreateRegistration(ctx, reg, hist); err != nil {
		t.Fatalf("create reg: %v", err)
	}

	// Owner can read.
	got, err := f.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil || got == nil {
		t.Fatalf("get reg: %v %+v", err, got)
	}
	if got.Status != model.RegStatusRegistered {
		t.Errorf("status: want %q got %q", model.RegStatusRegistered, got.Status)
	}

	// Transition to canceled — history row must follow.
	prevStatus := model.RegStatusRegistered
	cancelReason := "user request"
	hist2 := &model.RegistrationStatusHistory{
		ID:             uuid.New(),
		RegistrationID: regID,
		OldStatus:      &prevStatus,
		NewStatus:      model.RegStatusCanceled,
		ActorType:      "user",
		ActorID:        &f.user1,
		ReasonCode:     &cancelReason,
		CreatedAt:      time.Now().UTC(),
	}
	if err := f.repos.Registration.UpdateRegistrationStatus(ctx, regID,
		model.RegStatusCanceled, hist2); err != nil {
		t.Fatalf("update reg status: %v", err)
	}

	got, err = f.repos.Registration.GetRegistrationByID(ctx, regID)
	if err != nil || got == nil {
		t.Fatalf("get reg post-update: %v %+v", err, got)
	}
	if got.Status != model.RegStatusCanceled {
		t.Errorf("post-update status: want %q got %q", model.RegStatusCanceled, got.Status)
	}

	// Verify history row exists with correct old/new.
	var oldS, newS string
	err = f.pool.QueryRow(ctx, `
		SELECT old_status, new_status FROM registration_status_history
		WHERE registration_id = $1 AND new_status = $2
		LIMIT 1`, regID, model.RegStatusCanceled).Scan(&oldS, &newS)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("registration_status_history row for cancellation is missing")
	}
	if err != nil {
		t.Fatalf("history query: %v", err)
	}
	if oldS != model.RegStatusRegistered || newS != model.RegStatusCanceled {
		t.Errorf("history mismatch: old=%q new=%q", oldS, newS)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

// looksLikeUniqueViolation matches both the pgconn-coded error and the
// human-readable wrapper messages used by the repo layer. The exact
// pgconn error code for unique violation is "23505".
func looksLikeUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23505") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate")
}
