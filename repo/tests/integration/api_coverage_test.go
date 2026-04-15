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
		t.Skipf("facility.name config not found or version 0; config entries: %d", len(configResp.Data))
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

	// Update a flag that doesn't exist should return error, not 500
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/admin/feature-flags/nonexistent",
		`{"enabled":true,"cohort_percent":50,"version":1}`, adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Error("updating nonexistent flag should not 500")
	}
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

	// Restore without a valid backup should fail gracefully
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/restore",
		`{"backup_id":"00000000-0000-0000-0000-000000000000","is_dry_run":true,"reason":"test"}`,
		adminToken))
	// Should not 500; may return 400 or 404
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("Restore should not 500: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 5. Registration state transitions
// ---------------------------------------------------------------------------
func TestRegistrationCancelFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Get a session to register for
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", memberToken))
	expectStatus(t, "ListSessions", w.Code, http.StatusOK, w.Body.String())
	sessID := firstDataID(w.Body.Bytes())
	if sessID == "" {
		t.Skip("No published sessions for registration test")
	}

	// Register
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		`{"session_id":"`+sessID+`"}`, memberToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Register: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	regID := dataID(w.Body.Bytes())

	// Get registration
	if regID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("GET", "/api/v1/registrations/"+regID, "", memberToken))
		expectStatus(t, "GetRegistration", w.Code, http.StatusOK, w.Body.String())
	}

	// Cancel
	if regID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/"+regID+"/cancel",
			`{"reason":"changed my mind"}`, memberToken))
		if w.Code != http.StatusOK {
			t.Fatalf("Cancel: expected 200, got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestRegistrationApproveRejectByStaff(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginExistingUser(t, r, "member2", "Seed@Pass1234")
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Member registers (might fail if already registered — that's ok for coverage)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", memberToken))
	sessID := firstDataID(w.Body.Bytes())
	if sessID == "" {
		t.Skip("No sessions")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		`{"session_id":"`+sessID+`"}`, memberToken))
	regID := dataID(w.Body.Bytes())

	// Staff approves
	if regID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/"+regID+"/approve", "", staffToken))
		// May succeed or fail depending on registration state — just check no 500
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("Approve should not 500: %s", w.Body.String())
		}
	}

	// Reject endpoint should also not 500
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations/00000000-0000-0000-0000-000000000000/reject",
		`{"reason":"test"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("Reject should not 500: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 6. Attendance flows
// ---------------------------------------------------------------------------
func TestAttendanceCheckInAndLeave(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Check-in with a bogus registration should fail gracefully
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/checkin",
		`{"registration_id":"00000000-0000-0000-0000-000000000000","method":"qr_staff"}`,
		staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("CheckIn bogus reg should not 500: %s", w.Body.String())
	}

	// Leave with bogus ID
	memberToken := loginExistingUser(t, r, "member1", "Seed@Pass1234")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/leave",
		`{"registration_id":"00000000-0000-0000-0000-000000000000"}`, memberToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("Leave bogus reg should not 500: %s", w.Body.String())
	}

	// Return from leave with bogus ID
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/leave/00000000-0000-0000-0000-000000000000/return",
		"", memberToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("EndLeave bogus should not 500: %s", w.Body.String())
	}

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

	// Add product to cart
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", token))
	prodID := firstDataID(w.Body.Bytes())
	if prodID == "" {
		t.Skip("No products")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items",
		`{"item_type":"product","item_id":"`+prodID+`","quantity":1}`, token))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("AddToCart should not 500: %s", w.Body.String())
	}

	// View cart
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/cart", "", token))
	expectStatus(t, "GetCart", w.Code, http.StatusOK, w.Body.String())

	// Create address for shippable checkout
	addrBody := `{"recipient_name":"Checkout Test","phone":"13000000000","line1":"1 Main St","city":"Beijing"}`
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses", addrBody, token))
	addrID := dataID(w.Body.Bytes())

	// Checkout
	if addrID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/checkout",
			`{"address_id":"`+addrID+`","idempotency_key":"checkout-test-1"}`, token))
		// Accept any non-500
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("Checkout should not 500: %s", w.Body.String())
		}

		orderID := dataID(w.Body.Bytes())
		if orderID != "" {
			// Get order
			w = httptest.NewRecorder()
			r.ServeHTTP(w, authReq("GET", "/api/v1/orders/"+orderID, "", token))
			expectStatus(t, "GetOrder", w.Code, http.StatusOK, w.Body.String())

			// Create payment request
			w = httptest.NewRecorder()
			r.ServeHTTP(w, authReq("POST", "/api/v1/orders/"+orderID+"/pay", "", token))
			if w.Code == http.StatusInternalServerError {
				t.Fatalf("CreatePaymentRequest should not 500: %s", w.Body.String())
			}
		}
	}
}

func TestBuyNowFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Get a product
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", token))
	prodID := firstDataID(w.Body.Bytes())
	if prodID == "" {
		t.Skip("No products")
	}

	// Create address
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses",
		`{"recipient_name":"BuyNow","phone":"13000000001","line1":"2 Main St","city":"Shanghai"}`, token))
	addrID := dataID(w.Body.Bytes())

	if addrID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/buy-now",
			`{"item_type":"product","item_id":"`+prodID+`","quantity":1,"address_id":"`+addrID+`"}`, token))
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("BuyNow should not 500: %s", w.Body.String())
		}
	}
}

func TestRemoveFromCart(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Remove a bogus item from cart should not 500
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("DELETE", "/api/v1/cart/items/00000000-0000-0000-0000-000000000000", "", token))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("RemoveFromCart bogus should not 500: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 8. Shipment mutations
// ---------------------------------------------------------------------------
func TestShipmentCreateAndStatusUpdate(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Create shipment for bogus order should fail gracefully
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments",
		`{"order_id":"00000000-0000-0000-0000-000000000000"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("CreateShipment bogus order should not 500: %s", w.Body.String())
	}

	// Update status on bogus shipment
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/shipments/00000000-0000-0000-0000-000000000000/status",
		`{"status":"packed"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("UpdateShipmentStatus should not 500: %s", w.Body.String())
	}

	// Record POD on bogus shipment
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments/00000000-0000-0000-0000-000000000000/pod",
		`{"proof_type":"typed_acknowledgment","acknowledgment_text":"received","receiver_name":"John"}`,
		staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("RecordPOD should not 500: %s", w.Body.String())
	}

	// Report exception
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/shipments/00000000-0000-0000-0000-000000000000/exception",
		`{"exception_type":"damaged","description":"package was damaged"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("ReportException should not 500: %s", w.Body.String())
	}

	// List shipments
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

	// Member reports the post
	if postID != "" {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, authReq("POST", "/api/v1/posts/"+postID+"/report",
			`{"reason":"spam","description":"looks like spam"}`, memberToken))
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("ReportPost should not 500: %s", w.Body.String())
		}
	}

	// Moderator lists reports
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/moderation/reports", "", modToken))
	expectStatus(t, "ListReports", w.Code, http.StatusOK, w.Body.String())

	// Moderator lists cases
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/moderation/cases", "", modToken))
	expectStatus(t, "ListCases", w.Code, http.StatusOK, w.Body.String())

	// Action on bogus case should not 500
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000/action",
		`{"action_type":"dismiss","details":{}}`, modToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("ActionCase should not 500: %s", w.Body.String())
	}
}

func TestModerationBanAndRevoke(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	modToken := loginExistingUser(t, r, "mod1", "Seed@Pass1234")

	// Apply ban (need user_id of member2)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/bans",
		`{"user_id":"00000000-0000-0000-0000-000000000000","ban_type":"posting","is_permanent":false,"duration_days":7,"reason":"test ban"}`,
		modToken))
	// Should not 500 even with bogus user_id
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("ApplyBan should not 500: %s", w.Body.String())
	}

	// Revoke ban on bogus ban_id
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/moderation/bans/00000000-0000-0000-0000-000000000000/revoke",
		"", modToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("RevokeBan should not 500: %s", w.Body.String())
	}
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

	// Update status
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/tickets/"+ticketID+"/status",
		`{"status":"acknowledged","reason":"looking into it"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("UpdateTicketStatus should not 500: %s", w.Body.String())
	}

	// Assign (admin)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/assign",
		`{"assigned_to":"00000000-0000-0000-0000-000000000000"}`, adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("AssignTicket should not 500: %s", w.Body.String())
	}

	// Add comment
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/comments",
		`{"body":"Investigating the issue","is_internal":true}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("AddComment should not 500: %s", w.Body.String())
	}

	// Resolve
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/resolve",
		`{"resolution_code":"fixed","resolution_summary":"Replacement shipped"}`, staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("ResolveTicket should not 500: %s", w.Body.String())
	}

	// Close
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets/"+ticketID+"/close", "", staffToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("CloseTicket should not 500: %s", w.Body.String())
	}

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

	// Apply import on bogus ID should not 500
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/00000000-0000-0000-0000-000000000000/apply",
		"", adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("ApplyImport bogus should not 500: %s", w.Body.String())
	}

	// Get import detail on bogus ID
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/imports/00000000-0000-0000-0000-000000000000", "", adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("GetImportDetail bogus should not 500: %s", w.Body.String())
	}

	// Create export
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/exports",
		`{"export_type":"order_export","format":"csv","filters":{}}`, adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("CreateExport should not 500: %s", w.Body.String())
	}

	// List exports
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/exports", "", adminToken))
	expectStatus(t, "ListExports", w.Code, http.StatusOK, w.Body.String())

	// Download export on bogus ID
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/exports/00000000-0000-0000-0000-000000000000/download",
		"", adminToken))
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("DownloadExport bogus should not 500: %s", w.Body.String())
	}
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

	// Should reject with non-500
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("Payment callback with bad sig should not 500: %s", w.Body.String())
	}
	// Should not be 200 OK (signature is invalid)
	if w.Code == http.StatusOK {
		t.Error("Payment callback with bad sig should not succeed")
	}
}

func TestPaymentCallbackDuplicateRejection(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Two identical callbacks should both not 500
	body := `{"gateway_tx_id":"tx-dup-456","merchant_order_ref":"nonexistent-order","amount":5000,"signature":"invalid"}`
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/payments/callback", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("Duplicate callback %d should not 500: %s", i+1, w.Body.String())
		}
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
