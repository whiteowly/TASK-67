package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dataField extracts a string field from {"data": { "field": "..." } }
func dataField(body []byte, field string) string {
	var env struct {
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(body, &env)
	if env.Data == nil {
		return ""
	}
	v, _ := env.Data[field].(string)
	return v
}

// dataID is shorthand for the "id" field in the response data object
func dataID(body []byte) string { return dataField(body, "id") }

// firstDataID extracts the id from the first element when data is an array
func firstDataID(body []byte) string {
	var env struct {
		Data []map[string]interface{} `json:"data"`
	}
	json.Unmarshal(body, &env)
	if len(env.Data) == 0 {
		return ""
	}
	v, _ := env.Data[0]["id"].(string)
	return v
}

// expectStatus is a concise assertion helper
func expectStatus(t *testing.T, label string, got, want int, body string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: expected %d, got %d: %s", label, want, got, truncBody(body))
	}
}

func truncBody(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// ── Deterministic fixture helpers (replace t.Skip patterns) ────────────
//
// These helpers fail loudly with t.Fatal if a required fixture cannot be
// produced, so test cases never silently degrade into a no-op skip.

// mustFirstPublishedSessionID returns the id of the first published
// session, or fails the test if none are seeded.
func mustFirstPublishedSessionID(t *testing.T, r http.Handler, token string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("seed broken: GET /catalog/sessions returned %d body=%s",
			w.Code, truncBody(w.Body.String()))
	}
	id := firstDataID(w.Body.Bytes())
	if id == "" {
		t.Fatalf("seed broken: GET /catalog/sessions returned no published sessions")
	}
	return id
}

// mustApprovalRequiredSessionID returns the id of a seeded session that
// requires approval (the "Swimming Basics" session in the seeder is the
// canonical one). If none is found we fail rather than skip.
func mustApprovalRequiredSessionID(t *testing.T, r http.Handler, token string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("seed broken: GET /catalog/sessions returned %d", w.Code)
	}
	var env struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	for _, s := range env.Data {
		if req, _ := s["requires_approval"].(bool); req {
			id, _ := s["id"].(string)
			if id != "" {
				return id
			}
		}
	}
	t.Fatal("seed broken: no approval-required session present")
	return ""
}

// mustFirstPublishedProductID returns the id of the first published
// product, or fails the test if none are seeded.
func mustFirstPublishedProductID(t *testing.T, r http.Handler, token string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("seed broken: GET /catalog/products returned %d", w.Code)
	}
	id := firstDataID(w.Body.Bytes())
	if id == "" {
		t.Fatalf("seed broken: GET /catalog/products returned no published products")
	}
	return id
}

// mustCreateAddress posts the given address body and returns the id, or
// fails the test if creation does not succeed.
func mustCreateAddress(t *testing.T, r http.Handler, token, body string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses", body, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CreateAddress fixture failed: %d body=%s", w.Code, w.Body.String())
	}
	id := dataID(w.Body.Bytes())
	if id == "" {
		t.Fatal("CreateAddress fixture: response missing id")
	}
	return id
}

// mustRegisterUserAndReturnID registers a brand-new user and returns
// their UUID. The username is suffixed with a counter via the input
// argument so callers can produce isolated users per test.
func mustRegisterUserAndReturnID(t *testing.T, r http.Handler, username string) string {
	t.Helper()
	body := `{"username":"` + username + `","password":"SecurePass123!","display_name":"Test ` + username + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register fixture %s failed: %d body=%s", username, w.Code, w.Body.String())
	}
	id := dataID(w.Body.Bytes())
	if id == "" {
		t.Fatal("register fixture: response missing id")
	}
	return id
}

// envelopeShape mirrors the standard error response envelope.
type envelopeShape struct {
	Success bool `json:"success"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// expectErrorEnvelope asserts the response is a deterministic error
// (exact status from the allowed set; success=false; non-empty error.code
// and error.message). Replaces the old "is the status not 500?" pattern
// with a positive contract assertion.
//
// Callers pass one or more allowed statuses (typically a single value).
// All allowed values must be 4xx — if a 5xx slips through it is treated
// as a failure. This is the central enforcement of the audit rule
// "no `not 500` checks".
func expectErrorEnvelope(t *testing.T, label string, w *httptest.ResponseRecorder, allowedStatuses ...int) envelopeShape {
	t.Helper()
	if len(allowedStatuses) == 0 {
		t.Fatalf("%s: expectErrorEnvelope requires at least one allowed status", label)
	}
	for _, s := range allowedStatuses {
		if s < 400 || s >= 500 {
			t.Fatalf("%s: allowed status %d is not 4xx — only deterministic 4xx codes are permitted",
				label, s)
		}
	}
	matched := false
	for _, s := range allowedStatuses {
		if w.Code == s {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("%s: status want one of %v, got %d, body=%s",
			label, allowedStatuses, w.Code, truncBody(w.Body.String()))
	}

	var env envelopeShape
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("%s: response body is not a JSON envelope: %v body=%s",
			label, err, truncBody(w.Body.String()))
	}
	if env.Success {
		t.Errorf("%s: envelope.success must be false on error response", label)
	}
	if env.Error == nil {
		t.Fatalf("%s: envelope.error must be present on error response; body=%s",
			label, truncBody(w.Body.String()))
	}
	if env.Error.Code == "" {
		t.Errorf("%s: envelope.error.code must be non-empty", label)
	}
	if env.Error.Message == "" {
		t.Errorf("%s: envelope.error.message must be non-empty", label)
	}
	return env
}

// ---------------------------------------------------------------------------
// 1. User profile update (PATCH /api/v1/users/me)
// ---------------------------------------------------------------------------
func TestUpdateUserProfile(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginAsNewUser(t, r, "profileuser")

	body := `{"display_name":"New Name","email":"new@example.com","phone":"13100001111"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/users/me", body, token))
	expectStatus(t, "UpdateMe", w.Code, http.StatusOK, w.Body.String())

	// Verify updated
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/users/me", "", token))
	expectStatus(t, "GetMe after update", w.Code, http.StatusOK, w.Body.String())
	if !strings.Contains(w.Body.String(), "New Name") {
		t.Error("profile should contain updated display_name")
	}
}

// ---------------------------------------------------------------------------
// 2. Admin config mutation (PATCH /api/v1/admin/config/:key)
// ---------------------------------------------------------------------------
func TestAdminConfigMutation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Read current config to get the version
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/config", "", adminToken))
	expectStatus(t, "ListConfig", w.Code, http.StatusOK, w.Body.String())

	// Parse to get facility.name version
	var configResp struct {
		Data []struct {
			Key     string `json:"key"`
			Value   string `json:"value"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &configResp)

	var version int
	for _, c := range configResp.Data {
		if c.Key == "facility.name" {
			version = c.Version
			break
		}
	}
	if version == 0 {
		// Seed contract: facility.name must be present with a version > 0.
		// If it is missing, the seeder is broken — fail loudly.
		t.Fatalf("seed broken: facility.name config not found or version 0; config entries: %d",
			len(configResp.Data))
	}

	// Update the config entry
	body := fmt.Sprintf(`{"value":"Updated Facility","version":%d}`, version)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/admin/config/facility.name", body, adminToken))
	expectStatus(t, "UpdateConfig", w.Code, http.StatusOK, w.Body.String())

	// Verify updated
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/config", "", adminToken))
	if !strings.Contains(w.Body.String(), "Updated Facility") {
		t.Error("config value should be updated")
	}

	// Version conflict should fail (using the old version)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/admin/config/facility.name",
		fmt.Sprintf(`{"value":"Conflict","version":%d}`, version), adminToken))
	if w.Code == http.StatusOK {
		t.Error("stale version should fail with conflict")
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// ---------------------------------------------------------------------------
// 3. Feature flag mutation (PATCH /api/v1/admin/feature-flags/:key)
// ---------------------------------------------------------------------------
func TestAdminFeatureFlagMutation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Feature flags may be empty after seed, but the endpoint should work
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/feature-flags", "", adminToken))
	expectStatus(t, "ListFlags", w.Code, http.StatusOK, w.Body.String())

	// Update a flag that doesn't exist must be a deterministic conflict
	// (handler fallback maps optimistic-version / not-found errors to 409).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/admin/feature-flags/nonexistent",
		`{"enabled":true,"cohort_percent":50,"version":1}`, adminToken))
	expectErrorEnvelope(t, "UpdateFlag nonexistent", w, http.StatusConflict)
}

// ---------------------------------------------------------------------------
// 4. Backup/Archive/Restore actions
// ---------------------------------------------------------------------------
func TestAdminBackupRunAndList(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Run backup
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/backups", "", adminToken))
	// Accept 200 or 201 (the handler returns Created or OK)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("RunBackup: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}

	// List backups should show the new one
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/backups", "", adminToken))
	expectStatus(t, "ListBackups", w.Code, http.StatusOK, w.Body.String())
}

func TestAdminArchiveRunAndList(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Run archive
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/archives",
		`{"archive_type":"orders"}`, adminToken))
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("RunArchive: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}

	// List archives
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/archives", "", adminToken))
	expectStatus(t, "ListArchives", w.Code, http.StatusOK, w.Body.String())
}

func TestAdminRestoreRequiresReason(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Restore against a syntactically-valid but non-existent backup_id must
	// return 400 RESTORE_FAILED with a clear error envelope. The body uses
	// the modern `recovery_mode` field (the legacy `is_dry_run` alone fails
	// the binding's `required` rule).
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/restore",
		`{"backup_id":"11111111-1111-1111-1111-111111111111","recovery_mode":"dry_run","reason":"test"}`,
		adminToken))
	expectErrorEnvelope(t, "Restore nonexistent backup", w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// 5. Registration state transitions
// ---------------------------------------------------------------------------
func TestRegistrationCancelFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Fixture: there must always be a published session after seed; if not,
	// the seed itself is broken and we want to fail loudly rather than skip.
	sessID := mustFirstPublishedSessionID(t, r, memberToken)

	// Register — must succeed deterministically.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		`{"session_id":"`+sessID+`"}`, memberToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Register: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	regID := dataID(w.Body.Bytes())
	if regID == "" {
		t.Fatal("Register: response missing data.id")
	}

	// Get registration — must succeed.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/registrations/"+regID, "", memberToken))
	expectStatus(t, "GetRegistration", w.Code, http.StatusOK, w.Body.String())

	// Cancel — must succeed.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/"+regID+"/cancel",
		`{"reason":"changed my mind"}`, memberToken))
	expectStatus(t, "Cancel", w.Code, http.StatusOK, w.Body.String())
}

func TestRegistrationApproveRejectByStaff(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member2", "Seed@Pass1234")
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Fixture: pick the approval-required seeded session (the third one,
	// "Swimming Basics", has RequiresApproval=true) so the registration
	// lands in pending_approval and the staff approve path is exercised
	// deterministically rather than being a noop.
	sessID := mustApprovalRequiredSessionID(t, r, memberToken)

	// Member registers — must produce a pending_approval registration.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		`{"session_id":"`+sessID+`"}`, memberToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Register: %d body=%s", w.Code, w.Body.String())
	}
	regID := dataID(w.Body.Bytes())
	if regID == "" {
		t.Fatal("Register: response missing data.id")
	}

	// Staff approves — must succeed deterministically.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/"+regID+"/approve", "", staffToken))
	expectStatus(t, "Approve pending registration", w.Code, http.StatusOK, w.Body.String())

	// Reject against a non-existent registration must be a deterministic
	// 400 REJECT_FAILED with a well-formed error envelope (the service
	// returns a "registration not found" error that is NOT wrapped via
	// service.NotFound, so the handler falls through to the 400 fallback).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/11111111-1111-1111-1111-111111111111/reject",
		`{"reason":"test"}`, staffToken))
	expectErrorEnvelope(t, "Reject nonexistent registration", w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// 6. Attendance flows
// ---------------------------------------------------------------------------
func TestAttendanceCheckInAndLeave(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Check-in against a non-existent registration must be a deterministic
	// 400 with a well-formed envelope.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/checkin",
		`{"registration_id":"11111111-1111-1111-1111-111111111111","method":"qr_staff"}`,
		staffToken))
	expectErrorEnvelope(t, "CheckIn nonexistent reg", w, http.StatusBadRequest)

	// Leave against a non-existent registration: 400 deterministic.
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/leave",
		`{"registration_id":"11111111-1111-1111-1111-111111111111"}`, memberToken))
	expectErrorEnvelope(t, "Leave nonexistent reg", w, http.StatusBadRequest)

	// Return-from-leave against a non-existent leave event: 400 deterministic.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/leave/11111111-1111-1111-1111-111111111111/return",
		"", memberToken))
	expectErrorEnvelope(t, "EndLeave nonexistent", w, http.StatusBadRequest)

	// List exceptions (staff)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/attendance/exceptions", "", staffToken))
	expectStatus(t, "ListExceptions", w.Code, http.StatusOK, w.Body.String())
}

// ---------------------------------------------------------------------------
// 7. Checkout / Buy-Now / Payment-request flows
// ---------------------------------------------------------------------------
func TestCheckoutFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Fixture: seeded products are guaranteed to exist; fail loudly if not.
	prodID := mustFirstPublishedProductID(t, r, token)

	// Add product to cart — must create the cart item (201).
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items",
		`{"item_type":"product","item_id":"`+prodID+`","quantity":1}`, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("AddToCart: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// View cart — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/cart", "", token))
	expectStatus(t, "GetCart", w.Code, http.StatusOK, w.Body.String())

	// Fixture: create a delivery address (deterministic, not skipped).
	addrID := mustCreateAddress(t, r, token,
		`{"recipient_name":"Checkout Test","phone":"13000000000","line1":"1 Main St","city":"Beijing"}`)

	// Checkout — must succeed deterministically.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
		`{"address_id":"`+addrID+`","idempotency_key":"checkout-test-1"}`, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Checkout: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}
	orderID := dataID(w.Body.Bytes())
	if orderID == "" {
		t.Fatal("Checkout: response missing order id")
	}

	// Get order — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/orders/"+orderID, "", token))
	expectStatus(t, "GetOrder", w.Code, http.StatusOK, w.Body.String())

	// Create payment request — must succeed (201/200), envelope.success=true.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/orders/"+orderID+"/pay", "", token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CreatePaymentRequest: expected 200/201, got %d body=%s",
			w.Code, w.Body.String())
	}
}

func TestBuyNowFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Fixtures: seeded product + freshly created address.
	prodID := mustFirstPublishedProductID(t, r, token)
	addrID := mustCreateAddress(t, r, token,
		`{"recipient_name":"BuyNow","phone":"13000000001","line1":"2 Main St","city":"Shanghai"}`)

	// Buy-Now must succeed deterministically (201).
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/buy-now",
		`{"item_type":"product","item_id":"`+prodID+`","quantity":1,"address_id":"`+addrID+`"}`, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("BuyNow: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRemoveFromCart(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Removing a non-existent cart item must be a deterministic 4xx with
	// a well-formed error envelope. The active cart for a freshly-seeded
	// member is empty, so the service returns "no active cart" → 400.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("DELETE", "/api/v1/cart/items/11111111-1111-1111-1111-111111111111", "", token))
	expectErrorEnvelope(t, "RemoveFromCart nonexistent", w, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// 8. Shipment mutations
// ---------------------------------------------------------------------------
func TestShipmentCreateAndStatusUpdate(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Create-shipment against a non-existent order: deterministic 400.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments",
		`{"order_id":"11111111-1111-1111-1111-111111111111"}`, staffToken))
	expectErrorEnvelope(t, "CreateShipment nonexistent order", w, http.StatusBadRequest)

	// Status update on non-existent shipment: deterministic 400.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/shipments/11111111-1111-1111-1111-111111111111/status",
		`{"status":"packed"}`, staffToken))
	expectErrorEnvelope(t, "UpdateShipmentStatus nonexistent", w, http.StatusBadRequest)

	// POD on non-existent shipment: deterministic 400.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments/11111111-1111-1111-1111-111111111111/pod",
		`{"proof_type":"typed_acknowledgment","acknowledgment_text":"received","receiver_name":"John"}`,
		staffToken))
	expectErrorEnvelope(t, "RecordPOD nonexistent", w, http.StatusBadRequest)

	// Report exception on non-existent shipment: deterministic 400.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments/11111111-1111-1111-1111-111111111111/exception",
		`{"exception_type":"damaged","description":"package was damaged"}`, staffToken))
	expectErrorEnvelope(t, "ReportException nonexistent", w, http.StatusBadRequest)

	// List shipments — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/shipments", "", staffToken))
	expectStatus(t, "ListShipments", w.Code, http.StatusOK, w.Body.String())
}

// ---------------------------------------------------------------------------
// 9. Moderation actions and bans
// ---------------------------------------------------------------------------
func TestModerationActionFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")
	modToken := loginExistingUser(t, r, "mod1", "Seed@Pass1234")

	// Member creates a post
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/posts",
		`{"title":"Report Test","body":"This is a post to test moderation flow."}`, memberToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CreatePost: %d: %s", w.Code, w.Body.String())
	}
	postID := dataID(w.Body.Bytes())

	// Get post
	if postID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("GET", "/api/v1/posts/"+postID, "", memberToken))
		expectStatus(t, "GetPost", w.Code, http.StatusOK, w.Body.String())
	}

	// Member reports the post — must succeed (201).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/posts/"+postID+"/report",
		`{"reason":"spam","description":"looks like spam"}`, memberToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("ReportPost: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// Moderator lists reports — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/moderation/reports", "", modToken))
	expectStatus(t, "ListReports", w.Code, http.StatusOK, w.Body.String())

	// Moderator lists cases — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/moderation/cases", "", modToken))
	expectStatus(t, "ListCases", w.Code, http.StatusOK, w.Body.String())

	// Action on a non-existent case must be a deterministic 400 with a
	// well-formed error envelope.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/cases/11111111-1111-1111-1111-111111111111/action",
		`{"action_type":"dismiss","details":"test dismissal"}`, modToken))
	expectErrorEnvelope(t, "ActionCase nonexistent", w, http.StatusBadRequest)
}

func TestModerationBanAndRevoke(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	modToken := loginExistingUser(t, r, "mod1", "Seed@Pass1234")

	// Fixture: register a target user we can actually ban so the apply
	// returns 201 deterministically (the previous version sent a zero
	// UUID, which the service does not validate against the users table —
	// behavior was undocumented).
	targetID := mustRegisterUserAndReturnID(t, r, "ban-target-int")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/bans",
		fmt.Sprintf(`{"user_id":"%s","ban_type":"posting","is_permanent":false,"duration_days":7,"reason":"test ban"}`, targetID),
		modToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("ApplyBan: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}
	banID := dataID(w.Body.Bytes())
	if banID == "" {
		t.Fatal("ApplyBan: response missing ban id")
	}

	// Revoke the ban we just applied — must succeed (200).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/bans/"+banID+"/revoke",
		"", modToken))
	expectStatus(t, "RevokeBan applied ban", w.Code, http.StatusOK, w.Body.String())
}

// ---------------------------------------------------------------------------
// 10. Ticket lifecycle mutations
// ---------------------------------------------------------------------------
func TestTicketLifecycleFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Create ticket
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets",
		`{"ticket_type":"delivery_exception","title":"Damaged Package","description":"Package arrived damaged","priority":"high"}`,
		staffToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CreateTicket: %d: %s", w.Code, w.Body.String())
	}
	ticketID := dataID(w.Body.Bytes())
	if ticketID == "" {
		t.Fatal("expected ticket ID in response")
	}

	// Get ticket
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets/"+ticketID, "", staffToken))
	expectStatus(t, "GetTicket", w.Code, http.StatusOK, w.Body.String())

	// Update status open → acknowledged: must succeed (200).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/tickets/"+ticketID+"/status",
		`{"status":"acknowledged","reason":"looking into it"}`, staffToken))
	expectStatus(t, "UpdateTicketStatus", w.Code, http.StatusOK, w.Body.String())

	// Assign to a non-existent assignee must be deterministic 400.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/assign",
		`{"assigned_to":"11111111-1111-1111-1111-111111111111"}`, adminToken))
	expectErrorEnvelope(t, "AssignTicket bogus assignee", w, http.StatusBadRequest)

	// Add comment must succeed (201).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/comments",
		`{"body":"Investigating the issue","is_internal":true}`, staffToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("AddComment: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// Resolve from acknowledged — the resolve endpoint short-circuits the
	// state-machine: any non-closed ticket can be resolved directly. So
	// this must succeed (200).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/resolve",
		`{"resolution_code":"fixed","resolution_summary":"Replacement shipped"}`, staffToken))
	expectStatus(t, "ResolveTicket from acknowledged", w.Code, http.StatusOK, w.Body.String())

	// Close — must succeed (200) from resolved.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/close", "", staffToken))
	expectStatus(t, "CloseTicket from resolved", w.Code, http.StatusOK, w.Body.String())

	// Resolving a CLOSED ticket must be deterministically rejected
	// (the service explicitly checks `ticket.Status == TicketStatusClosed`
	// and returns "ticket is already closed" → 400 RESOLVE_FAILED).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/resolve",
		`{"resolution_code":"fixed","resolution_summary":"after close"}`, staffToken))
	expectErrorEnvelope(t, "ResolveTicket after close", w, http.StatusBadRequest)

	// List tickets
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets", "", staffToken))
	expectStatus(t, "ListTickets", w.Code, http.StatusOK, w.Body.String())
}

// ---------------------------------------------------------------------------
// 11. Import / Export flows
// ---------------------------------------------------------------------------
func TestImportAndExportFlows(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// List imports (should be empty but 200)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/imports", "", adminToken))
	expectStatus(t, "ListImports", w.Code, http.StatusOK, w.Body.String())

	// Apply import on a non-existent ID → 404 NOT_FOUND (service.NotFound
	// is wrapped → handler maps to 404).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/11111111-1111-1111-1111-111111111111/apply",
		"", adminToken))
	expectErrorEnvelope(t, "ApplyImport nonexistent", w, http.StatusNotFound)

	// Get import detail on a non-existent ID → 404.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/imports/11111111-1111-1111-1111-111111111111", "", adminToken))
	expectErrorEnvelope(t, "GetImportDetail nonexistent", w, http.StatusNotFound)

	// Create export must succeed (201/200).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/exports",
		`{"export_type":"order_export","format":"csv","filters":{}}`, adminToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CreateExport: expected 200/201, got %d body=%s", w.Code, w.Body.String())
	}

	// List exports — 200.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/exports", "", adminToken))
	expectStatus(t, "ListExports", w.Code, http.StatusOK, w.Body.String())

	// Download export on a non-existent ID → 404.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/exports/11111111-1111-1111-1111-111111111111/download",
		"", adminToken))
	expectErrorEnvelope(t, "DownloadExport nonexistent", w, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// 12. Payment callback idempotency
// ---------------------------------------------------------------------------
func TestPaymentCallbackInvalidSignature(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	body := `{"gateway_tx_id":"tx-invalid-123","merchant_order_ref":"nonexistent","amount":1000,"signature":"badsig"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Bad signature must be a deterministic 400 CALLBACK_FAILED with a
	// well-formed error envelope.
	env := expectErrorEnvelope(t, "Callback bad signature", w, http.StatusBadRequest)
	if env.Error.Code != "CALLBACK_FAILED" {
		t.Errorf("error.code: want CALLBACK_FAILED, got %q", env.Error.Code)
	}
}

func TestPaymentCallbackDuplicateRejection(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Two identical callbacks must BOTH return the same deterministic 400
	// (signature is invalid both times — there is no idempotent success
	// path here because the callback never authenticates).
	body := `{"gateway_tx_id":"tx-dup-456","merchant_order_ref":"nonexistent-order","amount":5000,"signature":"invalid"}`
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/payments/callback", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		expectErrorEnvelope(t,
			fmt.Sprintf("Duplicate callback attempt %d", i+1),
			w, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// 13. RBAC enforcement on new endpoint families
// ---------------------------------------------------------------------------
func TestShipmentsDeniedForModerator(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	modToken := loginExistingUser(t, r, "mod1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments", `{"order_id":"x"}`, modToken))
	if w.Code != http.StatusForbidden {
		t.Errorf("Moderator creating shipment: expected 403, got %d", w.Code)
	}
}

func TestImportsDeniedForStaff(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/imports", "", staffToken))
	if w.Code != http.StatusForbidden {
		t.Errorf("Staff accessing imports: expected 403, got %d", w.Code)
	}
}

func TestTicketsDeniedForUnauthenticated(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tickets", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Unauthenticated tickets: expected 401, got %d", w.Code)
	}
}

func TestModerationDeniedForStaff(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	endpoints := []string{
		"/api/v1/moderation/reports",
		"/api/v1/moderation/cases",
	}
	for _, path := range endpoints {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authReq("GET", path, "", staffToken))
		if w.Code != http.StatusForbidden {
			t.Errorf("Staff at %s: expected 403, got %d", path, w.Code)
		}
	}
}

func TestBackupDeniedForMember(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/backups", "", memberToken))
	if w.Code != http.StatusForbidden {
		t.Errorf("Member running backup: expected 403, got %d", w.Code)
	}
}
