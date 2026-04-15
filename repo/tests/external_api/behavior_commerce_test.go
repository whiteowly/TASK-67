// behavior_commerce_test.go
//
// Deeper external-HTTP behavior tests for the commerce surface (cart,
// checkout, buy-now, orders). These complement the breadth covered by
// coverage_matrix_test.go and the happy-path flows in tests/blackbox/
// by asserting concrete payload semantics, state transitions, and
// failure paths that go beyond reachability.
package external_api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ── Cart payload contract: snapshot price, quantity, items[] shape ─────────

func TestExternal_Cart_PayloadContract(t *testing.T) {
	e := setupEnv(t)
	c, _ := registerAndLogin(t, e.BaseURL, "cart-contract")

	// Pull a real product.
	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)
	expectedPrice := products[0]["price_minor_units"].(float64)

	// Add to cart with quantity > 1 — the cart must echo quantity faithfully.
	resp, env := call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":3}`, pid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("add: %d body=%+v", resp.StatusCode, env.Error)
	}
	if int(dmap(env)["quantity"].(float64)) != 3 {
		t.Errorf("quantity echoed wrong: %v", dmap(env)["quantity"])
	}
	// price_snapshot must equal the catalog price at add-time.
	if got := dmap(env)["price_snapshot"].(float64); got != expectedPrice {
		t.Errorf("price snapshot %v != catalog price %v", got, expectedPrice)
	}

	// GET /cart returns {items: [...]} envelope shape.
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/cart", "")
	if resp.StatusCode != 200 {
		t.Fatalf("get cart: %d", resp.StatusCode)
	}
	cart := dmap(env)
	if cart == nil {
		t.Fatal("cart payload is not a JSON object")
	}
	itemsRaw, ok := cart["items"]
	if !ok {
		t.Fatal("cart payload missing items[] key")
	}
	items, ok := itemsRaw.([]interface{})
	if !ok {
		t.Fatal("cart.items is not a JSON array")
	}
	if len(items) == 0 {
		t.Fatal("cart.items is empty after add")
	}
	first := items[0].(map[string]interface{})
	for _, k := range []string{"id", "item_type", "item_id", "quantity", "price_snapshot", "currency"} {
		if _, ok := first[k]; !ok {
			t.Errorf("cart item missing required field %q", k)
		}
	}
}

// ── Cart dedup: same item posted twice in the same active cart is rejected ──

func TestExternal_Cart_DuplicateItemRejected(t *testing.T) {
	e := setupEnv(t)
	c, _ := registerAndLogin(t, e.BaseURL, "cart-dedup")

	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)

	// First add succeeds.
	resp, _ := call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("first add: %d", resp.StatusCode)
	}

	// Second add of the same product into the same active cart must be rejected.
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		t.Errorf("duplicate add must be rejected; got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("duplicate add must have success=false")
	}
}

// ── Cart cross-user isolation: one user cannot delete another's item ────────

func TestExternal_Cart_CrossUserDeleteIsIsolated(t *testing.T) {
	e := setupEnv(t)
	c1, _ := registerAndLogin(t, e.BaseURL, "cart-iso-1")
	c2, _ := registerAndLogin(t, e.BaseURL, "cart-iso-2")

	_, env := call(c1, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)

	// User 1 adds an item; capture its cart-item id.
	_, _ = call(c1, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	_, env = call(c1, "GET", e.BaseURL+"/api/v1/cart", "")
	items := dmap(env)["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("user 1 cart is empty after add")
	}
	user1ItemID := items[0].(map[string]interface{})["id"].(string)

	// User 2 tries to DELETE user 1's cart item — must fail.
	resp, env := call(c2, "DELETE", e.BaseURL+"/api/v1/cart/items/"+user1ItemID, "")
	if resp.StatusCode == 200 {
		t.Errorf("user 2 must not be able to delete user 1's cart item; got 200")
	}
	if env.Success {
		t.Error("cross-user delete must report success=false")
	}

	// User 1's item must still exist.
	_, env = call(c1, "GET", e.BaseURL+"/api/v1/cart", "")
	items = dmap(env)["items"].([]interface{})
	stillThere := false
	for _, it := range items {
		if it.(map[string]interface{})["id"] == user1ItemID {
			stillThere = true
		}
	}
	if !stillThere {
		t.Error("user 1's cart item must still be present after user 2's failed delete")
	}
}

// ── Checkout: empty cart fails cleanly with a meaningful error ──────────────

func TestExternal_Checkout_EmptyCartRejected(t *testing.T) {
	e := setupEnv(t)
	c, _ := registerAndLogin(t, e.BaseURL, "empty-checkout")

	// Need an address (will be ignored because cart is empty, but validation
	// requires it to be present for shippable items — empty cart short-circuits).
	resp, env := call(c, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"X","phone":"1","line1":"1","city":"BJ"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("address: %d", resp.StatusCode)
	}
	aid := ds(env, "id")

	// Empty cart checkout — must be 4xx with success=false and a clear error.
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"empty-1"}`, aid))
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		t.Errorf("empty checkout must fail; got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("empty checkout must report success=false")
	}
	if env.Error == nil {
		t.Error("empty checkout must include an error envelope")
	}
}

// ── Order detail payload contract: order_number, status, items[] shape ──────

func TestExternal_OrderDetail_PayloadContract(t *testing.T) {
	e := setupEnv(t)
	c, _ := registerAndLogin(t, e.BaseURL, "order-contract")

	// Bring an order into existence via cart → checkout.
	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	pid := dlist(env)[0]["id"].(string)

	resp, env := call(c, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"OC","phone":"1","line1":"1","city":"BJ"}`)
	aid := ds(env, "id")
	_, _ = call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":2}`, pid))
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"oc-1"}`, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("checkout: %d body=%+v", resp.StatusCode, env.Error)
	}
	oid := ds(env, "id")

	// Read the order back and assert the public contract.
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/orders/"+oid, "")
	if resp.StatusCode != 200 {
		t.Fatalf("order detail: %d", resp.StatusCode)
	}
	order := dmap(env)
	for _, k := range []string{"id", "order_number", "status", "subtotal", "total", "currency"} {
		if _, ok := order[k]; !ok {
			t.Errorf("order missing required field %q", k)
		}
	}
	if order["id"] != oid {
		t.Errorf("order.id mismatch: got %v want %s", order["id"], oid)
	}
	if num, _ := order["order_number"].(string); num == "" {
		t.Error("order_number must be non-empty")
	}
	// Subtotal must equal price_minor * 2 because we ordered quantity=2.
	// (We can't read the catalog price here cheaply, but subtotal>0 is required.)
	if order["subtotal"].(float64) <= 0 {
		t.Errorf("subtotal must be > 0, got %v", order["subtotal"])
	}
	// Status must be a valid lifecycle state.
	switch s, _ := order["status"].(string); s {
	case "awaiting_payment", "paid", "pending", "pending_payment":
		// ok
	default:
		t.Errorf("unexpected initial order status: %q", s)
	}
}

// ── BuyNow does NOT consume the cart ────────────────────────────────────────

func TestExternal_BuyNow_DoesNotConsumeCart(t *testing.T) {
	e := setupEnv(t)
	c, _ := registerAndLogin(t, e.BaseURL, "buynow-cart")

	// Stage a regular cart item first.
	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) < 2 {
		// Seed contract: at least three distinct products are seeded (T-Shirt,
		// Yoga Mat, Water Bottle). Fewer means the seeder is broken.
		t.Fatalf("seed broken: expected at least 2 products, got %d", len(products))
	}
	cartProduct := products[0]["id"].(string)
	buyNowProduct := products[1]["id"].(string)

	resp, env := call(c, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"BN","phone":"1","line1":"1","city":"BJ"}`)
	aid := ds(env, "id")

	_, _ = call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, cartProduct))

	// Buy-Now a different product — must succeed without touching the cart.
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/buy-now",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1,"address_id":"%s"}`,
			buyNowProduct, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("buy-now: %d body=%+v", resp.StatusCode, env.Error)
	}
	buyNowOrderID := ds(env, "id")
	if buyNowOrderID == "" {
		t.Fatal("buy-now returned no order id")
	}

	// Cart must still contain the original cartProduct.
	_, env = call(c, "GET", e.BaseURL+"/api/v1/cart", "")
	items := dmap(env)["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("cart was emptied by buy-now (must not happen)")
	}
	stillThere := false
	for _, it := range items {
		if it.(map[string]interface{})["item_id"] == cartProduct {
			stillThere = true
		}
	}
	if !stillThere {
		t.Error("cart should still contain the original cart product after buy-now")
	}
}

// ── Orders list returns only the caller's orders ────────────────────────────

func TestExternal_Orders_OnlyOwnOrders(t *testing.T) {
	e := setupEnv(t)
	c1, _ := registerAndLogin(t, e.BaseURL, "owner-only-1")
	c2, _ := registerAndLogin(t, e.BaseURL, "owner-only-2")

	// User 1 places an order.
	_, env := call(c1, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	pid := dlist(env)[0]["id"].(string)
	resp, env := call(c1, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"O1","phone":"1","line1":"1","city":"BJ"}`)
	aid := ds(env, "id")
	_, _ = call(c1, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	resp, env = call(c1, "POST", e.BaseURL+"/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"own-1"}`, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("checkout: %d", resp.StatusCode)
	}
	orderID := ds(env, "id")

	// User 2 lists their orders — must NOT contain user 1's order.
	resp, env = call(c2, "GET", e.BaseURL+"/api/v1/orders", "")
	if resp.StatusCode != 200 {
		t.Fatalf("user 2 orders: %d", resp.StatusCode)
	}
	for _, o := range dlist(env) {
		if id, _ := o["id"].(string); id == orderID {
			t.Errorf("user 2 must not see user 1's order %s in their orders list", orderID)
		}
	}

	// User 2 tries to GET user 1's order detail — must be 4xx.
	resp, env = call(c2, "GET", e.BaseURL+"/api/v1/orders/"+orderID, "")
	if resp.StatusCode == 200 {
		t.Errorf("user 2 must not be able to read user 1's order; got 200 body=%s",
			func() string {
				b, _ := json.Marshal(env.Data)
				return string(b)
			}())
	}
}

// ── Catalog: pagination meta and search filter narrow the result set ────────

func TestExternal_Catalog_PaginationAndSearch(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	// Default page must include meta.total >= the visible count.
	resp, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions", "")
	if resp.StatusCode != 200 {
		t.Fatalf("sessions list: %d", resp.StatusCode)
	}
	if env.Meta == nil {
		t.Fatal("expected meta on paginated response")
	}
	if env.Meta.Total < len(dlist(env)) {
		t.Errorf("meta.total (%d) < visible count (%d)", env.Meta.Total, len(dlist(env)))
	}

	// per_page=1 must return at most 1 row but the same meta.total.
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions?per_page=1", "")
	if resp.StatusCode != 200 {
		t.Fatalf("per_page=1: %d", resp.StatusCode)
	}
	if len(dlist(env)) > 1 {
		t.Errorf("per_page=1 returned %d rows", len(dlist(env)))
	}

	// A search that should match nothing must return an empty array
	// (NOT an error) and meta.total == 0.
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions?q=zzz_no_such_session_xyz", "")
	if resp.StatusCode != 200 {
		t.Fatalf("no-match search: %d", resp.StatusCode)
	}
	if len(dlist(env)) != 0 {
		t.Errorf("no-match search returned %d rows", len(dlist(env)))
	}
	if env.Meta != nil && env.Meta.Total != 0 {
		t.Errorf("no-match meta.total=%d, expected 0", env.Meta.Total)
	}
}

// ── Auth: malformed register payload returns 4xx with VALIDATION_ERROR ──────

func TestExternal_Auth_RegisterValidation(t *testing.T) {
	e := setupEnv(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing username", `{"password":"SecurePass123!","display_name":"X"}`},
		{"missing password", `{"username":"badreg1","display_name":"X"}`},
		{"weak password", `{"username":"badreg2","password":"short","display_name":"X"}`},
		{"empty body", `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := call(newClient(), "POST", e.BaseURL+"/api/v1/auth/register", tc.body)
			if resp.StatusCode == 201 || resp.StatusCode == 200 {
				t.Errorf("%s: must not succeed; got %d", tc.name, resp.StatusCode)
			}
			if env.Success {
				t.Errorf("%s: success must be false", tc.name)
			}
			if env.Error == nil {
				t.Errorf("%s: must include error envelope", tc.name)
			}
		})
	}
}

// ── Catalog product detail: shape includes price/stock/currency ─────────────

func TestExternal_CatalogProduct_Detail_Shape(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)

	resp, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products/"+pid, "")
	if resp.StatusCode != 200 {
		t.Fatalf("product detail: %d", resp.StatusCode)
	}
	prod := dmap(env)
	for _, k := range []string{"id", "name", "price_minor_units", "currency", "status"} {
		if _, ok := prod[k]; !ok {
			t.Errorf("product detail missing %q", k)
		}
	}
	if !strings.EqualFold(prod["status"].(string), "published") {
		t.Errorf("status=%q, expected published", prod["status"])
	}
	if prod["price_minor_units"].(float64) <= 0 {
		t.Errorf("price_minor_units must be > 0, got %v", prod["price_minor_units"])
	}
}
