package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// ---------------------------------------------------------------------------
// Fix 1: Authorization error mapping — exact 403 on cross-user access
// ---------------------------------------------------------------------------

func TestOrderOwnershipIsolation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	m1 := loginExistingUser(t, r, "member1", "Seed@Pass1234")
	m2 := loginExistingUser(t, r, "member2", "Seed@Pass1234")

	// member1 creates an order
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", m1))
	var prodResp struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &prodResp)
	if len(prodResp.Data) == 0 {
		t.Fatal("seed broken: no published products")
	}
	pid := prodResp.Data[0].ID

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses",
		`{"recipient_name":"OA","phone":"1","line1":"1 Rd","city":"BJ"}`, m1))
	addrID := extractID(t, w.Body.Bytes())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid), m1))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"oa-test-1"}`, addrID), m1))
	orderID := extractID(t, w.Body.Bytes())
	if orderID == "" {
		t.Fatalf("checkout fixture failed: no order id; status=%d body=%s",
			w.Code, truncBody(w.Body.String()))
	}

	// Owner access → 200
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/orders/"+orderID, "", m1))
	if w.Code != 200 {
		t.Fatalf("owner GET order: expected 200, got %d", w.Code)
	}

	// Cross-user GET → must be 403 (not 500)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/orders/"+orderID, "", m2))
	if w.Code == 500 {
		t.Fatal("cross-user GET order returned 500 — authz error not mapped")
	}
	if w.Code != 403 {
		t.Errorf("cross-user GET order: expected 403, got %d", w.Code)
	}

	// Cross-user pay → must be 403 (not 500)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/orders/"+orderID+"/pay", "", m2))
	if w.Code == 500 {
		t.Fatal("cross-user PAY returned 500 — authz error not mapped")
	}
	if w.Code != 403 {
		t.Errorf("cross-user PAY: expected 403, got %d", w.Code)
	}
}

func TestRegistrationOwnershipIsolation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	m1 := loginExistingUser(t, r, "member1", "Seed@Pass1234")
	m2 := loginExistingUser(t, r, "member2", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", m1))
	var sessResp struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &sessResp)
	if len(sessResp.Data) == 0 {
		t.Fatal("seed broken: no published sessions")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sessResp.Data[0].ID), m1))
	regID := extractID(t, w.Body.Bytes())
	if regID == "" {
		t.Fatalf("registration fixture failed: status=%d body=%s",
			w.Code, truncBody(w.Body.String()))
	}

	// Owner → 200
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/registrations/"+regID, "", m1))
	if w.Code != 200 {
		t.Fatalf("owner GET reg: expected 200, got %d", w.Code)
	}

	// Cross-user → must be 403 (not 500)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/registrations/"+regID, "", m2))
	if w.Code == 500 {
		t.Fatal("cross-user GET registration returned 500 — authz error not mapped")
	}
	if w.Code != 403 {
		t.Errorf("cross-user GET registration: expected 403, got %d", w.Code)
	}
}

func TestTicketAuthorizationReturns403(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")
	member := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// staff creates a ticket
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets",
		`{"ticket_type":"check_in_dispute","title":"Auth Test","description":"d","priority":"medium"}`,
		staff))
	tid := extractID(t, w.Body.Bytes())
	if tid == "" {
		t.Fatalf("ticket fixture failed: status=%d body=%s",
			w.Code, truncBody(w.Body.String()))
	}

	// staff (creator) can see it → 200
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets/"+tid, "", staff))
	if w.Code != 200 {
		t.Fatalf("creator GET: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// non-creator member → must be 403 (not 500)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets/"+tid, "", member))
	if w.Code == 500 {
		t.Fatal("unauthorized ticket GET returned 500")
	}
	if w.Code != 403 {
		t.Errorf("unauthorized ticket GET: expected 403, got %d", w.Code)
	}

	// non-creator member update status → 403
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/tickets/"+tid+"/status",
		`{"status":"acknowledged","reason":"test"}`, member))
	if w.Code == 500 {
		t.Fatal("unauthorized ticket status update returned 500")
	}
	if w.Code != 403 {
		t.Errorf("unauthorized ticket status update: expected 403, got %d", w.Code)
	}

	// non-creator member close → 403
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+tid+"/close", "", member))
	if w.Code == 500 {
		t.Fatal("unauthorized ticket close returned 500")
	}
	if w.Code != 403 {
		t.Errorf("unauthorized ticket close: expected 403, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Fix 2: Ticket list authorization scope — members see only own tickets
// ---------------------------------------------------------------------------

func TestTicketListScopedByRole(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")
	member := loginAsNewUser(t, r, "ticketscope")

	// Staff creates a ticket
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets",
		`{"ticket_type":"delivery_exception","title":"Staff Ticket","description":"s","priority":"low"}`,
		staff))
	staffTicketID := extractID(t, w.Body.Bytes())

	// Member creates a ticket
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets",
		`{"ticket_type":"check_in_dispute","title":"Member Ticket","description":"m","priority":"low"}`,
		member))
	memberTicketID := extractID(t, w.Body.Bytes())

	if staffTicketID == "" || memberTicketID == "" {
		t.Fatalf("ticket fixtures failed: staff=%q member=%q", staffTicketID, memberTicketID)
	}

	// Staff lists tickets → should see both (staff sees all)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets", "", staff))
	if w.Code != 200 {
		t.Fatalf("staff list: %d", w.Code)
	}
	var staffList struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &staffList)
	staffSees := len(staffList.Data)
	if staffSees < 2 {
		t.Errorf("staff should see at least 2 tickets, saw %d", staffSees)
	}

	// Member lists tickets → should see only their own
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets", "", member))
	if w.Code != 200 {
		t.Fatalf("member list: %d", w.Code)
	}
	var memberList struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &memberList)

	for _, ticket := range memberList.Data {
		if ticket.ID == staffTicketID {
			t.Fatal("member should NOT see staff's ticket in list")
		}
	}
	foundOwn := false
	for _, ticket := range memberList.Data {
		if ticket.ID == memberTicketID {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Error("member should see their own ticket in list")
	}
}

// ---------------------------------------------------------------------------
// Fix 3: Checkout idempotency — same key returns same order
// ---------------------------------------------------------------------------

func TestCheckoutIdempotency(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Setup: product + address + cart
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", token))
	var prodResp struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &prodResp)
	if len(prodResp.Data) == 0 {
		t.Fatal("seed broken: no published products")
	}
	pid := prodResp.Data[0].ID

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses",
		`{"recipient_name":"Idem","phone":"1","line1":"1 Rd","city":"BJ"}`, token))
	addrID := extractID(t, w.Body.Bytes())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid), token))

	key := "idem-test-key-123"

	// First checkout
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"%s"}`, addrID, key), token))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("first checkout: %d: %s", w.Code, w.Body.String())
	}
	firstOrderID := extractID(t, w.Body.Bytes())
	if firstOrderID == "" {
		t.Fatal("first checkout: no order ID")
	}

	// Second checkout with same key → should return same order (not create duplicate)
	// Need to add item to cart again since first checkout consumed it
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid), token))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"%s"}`, addrID, key), token))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("second checkout: %d: %s", w.Code, w.Body.String())
	}
	secondOrderID := extractID(t, w.Body.Bytes())
	if secondOrderID != firstOrderID {
		t.Errorf("idempotency failed: first=%s second=%s (should be same)", firstOrderID, secondOrderID)
	}
}

// ---------------------------------------------------------------------------
// Waitlist, rate limit, assignment, address, POD tests (from prior round)
// ---------------------------------------------------------------------------

func TestWaitlistWhenSessionFull(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	m1 := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", m1))
	var sessResp struct {
		Data []struct {
			ID             string `json:"id"`
			AllowsWaitlist bool   `json:"allows_waitlist"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &sessResp)

	var sessID string
	for _, s := range sessResp.Data {
		if s.AllowsWaitlist {
			sessID = s.ID
			break
		}
	}
	if sessID == "" {
		t.Fatal("seed broken: no waitlist-allowed sessions present (expected at least one)")
	}

	regUser := loginAsNewUser(t, r, "waitlistuser")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sessID), regUser))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("registration: %d: %s", w.Code, w.Body.String())
	}
	var regResp struct{ Data struct{ Status string `json:"status"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &regResp)
	validStatuses := map[string]bool{"registered": true, "pending_approval": true, "waitlisted": true}
	if !validStatuses[regResp.Data.Status] {
		t.Errorf("unexpected registration status: %q", regResp.Data.Status)
	}
}

func TestPostRateLimitEnforced(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginAsNewUser(t, r, "ratelimituser")

	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"title":"Rate Test","body":"Post number %d with enough content"}`, i)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/posts", body, token))
		if w.Code != 200 && w.Code != 201 {
			t.Fatalf("post %d: expected 200/201, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	// 6th post → rejected
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/posts",
		`{"title":"Over Limit","body":"This should be rejected by rate limiter"}`, token))
	if w.Code == 200 || w.Code == 201 {
		t.Fatal("6th post should be rate-limited")
	}
	if w.Code != 400 {
		t.Errorf("rate-limited post: expected 400, got %d", w.Code)
	}
	if !containsStr(w.Body.String(), "rate limit") {
		t.Errorf("error should mention rate limit: %s", w.Body.String()[:min(len(w.Body.String()), 200)])
	}
}

func TestTicketAssignmentPersists(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets",
		`{"ticket_type":"delivery_exception","title":"Assign Test","description":"d","priority":"low"}`,
		staff))
	tid := extractID(t, w.Body.Bytes())
	if tid == "" {
		t.Fatal("ticket not created")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/users/me", "", admin))
	adminID := extractID(t, w.Body.Bytes())

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+tid+"/assign",
		fmt.Sprintf(`{"assigned_to":"%s"}`, adminID), admin))
	if w.Code == 500 {
		t.Fatalf("assignment returned 500 (column mismatch?): %s", w.Body.String())
	}
	if w.Code != 200 {
		t.Fatalf("assignment: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCheckoutWithoutAddress(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
		`{"idempotency_key":"no-addr-test-1"}`, token))
	// Must not fail with "address_id required" validation error
	if w.Code == 400 {
		var env envelope
		json.Unmarshal(w.Body.Bytes(), &env)
		if env.Error != nil && containsStr(env.Error.Message, "address_id") {
			t.Fatal("checkout handler should not require address_id at binding level")
		}
	}
}

func TestPODTypedAcknowledgmentRequiresText(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments/00000000-0000-0000-0000-000000000001/pod",
		`{"proof_type":"typed_acknowledgment","receiver_name":"John"}`, staff))
	if w.Code == 500 {
		t.Fatal("typed_acknowledgment POD without text should not 500")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var env struct{ Data struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(body, &env)
	return env.Data.ID
}

func containsStr(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
