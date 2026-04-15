// coverage_matrix_test.go
//
// Smoke-reachability probe for every endpoint listed in
// docs/api-endpoints-inventory.md. Each subtest confirms the route
// exists and the app handles the request (i.e. it returns a real HTTP
// response — not a 502/504 proxy error and not a closed connection).
//
// In addition, **high-risk endpoints** (money movement, restore, import
// apply) carry strict contract assertions in this file: exact HTTP
// status, exact `success` flag, exact error.code, and 2–3 critical
// response-body fields. These cannot be replaced by a generic liveness
// check because a regression in their failure semantics is a
// production-grade incident.
//
// Behavioral assertions for other endpoints live in the dedicated
// per-endpoint tests in missing_endpoints_test.go, behavior_admin_test.go,
// behavior_commerce_test.go, and tests/blackbox/blackbox_test.go.
// Together they form the 100% external coverage evidence summarized in
// docs/api-coverage-after.md.
package external_api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// endpointProbe describes a single matrix probe.
//
//   - When `wantStatus == 0`: liveness-only fence (no upstream-error
//     status, real response received).
//   - When `wantStatus != 0`: strict contract — assert exact status,
//     `success` flag, error.code (when applicable), and any extra
//     field assertions in `assertFields`.
type endpointProbe struct {
	method string
	path   string
	role   string // "", "member", "staff", "mod", "admin"
	body   string

	// ── strict contract (used iff wantStatus != 0) ──
	wantStatus    int                                 // exact HTTP status to require
	wantSuccess   *bool                               // exact envelope.success
	wantErrorCode string                              // exact envelope.error.code (empty = no check)
	assertFields  func(t *testing.T, env envelope)    // optional deeper assertions
}

// boolPtr is a tiny helper so a probe can express "want success=false"
// distinctly from "don't check success".
func boolPtr(b bool) *bool { return &b }

func TestExternal_CoverageMatrix(t *testing.T) {
	e := setupEnv(t)

	clients := map[string]*http.Client{
		"":              newClient(),
		"member":        loginAs(t, e.BaseURL, "member1", "Seed@Pass1234"),
		"staff":         loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234"),
		"mod":           loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234"),
		"admin":         loginAs(t, e.BaseURL, "admin", "Seed@Pass1234"),
		// Throwaway session used only by the logout probe so the shared
		// "member" cookie remains valid for every other member probe.
		"member-logout": loginAs(t, e.BaseURL, "member1", "Seed@Pass1234"),
	}

	zuid := "00000000-0000-0000-0000-000000000000"

	// fakeButValidUUID is a syntactically-valid UUID that does not exist
	// in any seeded table. Some `binding:"required"` rules on `uuid.UUID`
	// fields treat the all-zero UUID as the zero-value and reject it as
	// missing, which causes a VALIDATION_ERROR before the handler can
	// reach the domain not-found path. Use this for "non-existent but
	// well-formed" probes against required-uuid fields.
	fakeButValidUUID := "11111111-1111-1111-1111-111111111111"

	// ── High-risk strict-contract probes ─────────────────────────────────
	//
	// The four endpoints below must match an exact failure contract when
	// invoked with the matrix probe payload. The payloads are chosen so
	// the failure mode is deterministic (empty cart for checkout, bad
	// signature for callback, nonexistent backup for restore, nonexistent
	// import for apply).

	// 1. POST /api/v1/checkout — member1 has an empty active cart at
	//    matrix time (each test boots a fresh DB), so the service
	//    returns "cart is empty" and the handler maps that to a
	//    400 CHECKOUT_FAILED with envelope.success=false.
	checkoutProbe := endpointProbe{
		method:        "POST",
		path:          "/api/v1/checkout",
		role:          "member",
		body:          fmt.Sprintf(`{"address_id":"%s","idempotency_key":"mx-1"}`, zuid),
		wantStatus:    http.StatusBadRequest,
		wantSuccess:   boolPtr(false),
		wantErrorCode: "CHECKOUT_FAILED",
		assertFields: func(t *testing.T, env envelope) {
			t.Helper()
			if env.Error == nil {
				t.Fatal("expected error envelope, got nil")
			}
			// Critical field 1: human-readable message must be present.
			if env.Error.Message == "" {
				t.Error("error.message must be non-empty")
			}
			// Critical field 2: data must be null on failure.
			if string(env.Data) != "null" && len(env.Data) != 0 {
				t.Errorf("error response must have null data, got %q", string(env.Data))
			}
			// Critical field 3: meta.request_id must be set so callers can correlate.
			if env.Meta == nil || env.Meta.RequestID == "" {
				t.Error("meta.request_id must be set on every response")
			}
		},
	}

	// 2. POST /api/v1/payments/callback — bad signature must yield
	//    400 CALLBACK_FAILED with the exact public message
	//    "payment callback processing failed".
	paymentCallbackProbe := endpointProbe{
		method:        "POST",
		path:          "/api/v1/payments/callback",
		role:          "",
		body:          `{"gateway_tx_id":"mx","merchant_order_ref":"none","amount":100,"signature":"bad"}`,
		wantStatus:    http.StatusBadRequest,
		wantSuccess:   boolPtr(false),
		wantErrorCode: "CALLBACK_FAILED",
		assertFields: func(t *testing.T, env envelope) {
			t.Helper()
			if env.Error == nil {
				t.Fatal("expected error envelope, got nil")
			}
			// Critical field 1: exact public-facing message (no internal leakage).
			if env.Error.Message != "payment callback processing failed" {
				t.Errorf("error.message must be exactly 'payment callback processing failed', got %q",
					env.Error.Message)
			}
			// Critical field 2: must NOT leak verifier internals such as
			// the merchant key, the computed expected signature, or the
			// raw HMAC bytes.
			for _, secret := range []string{"merchant", "hmac", "expected", "secret"} {
				if containsCI(env.Error.Message, secret) {
					t.Errorf("error.message leaks internal token %q: %q", secret, env.Error.Message)
				}
			}
			// Critical field 3: data must be null.
			if string(env.Data) != "null" && len(env.Data) != 0 {
				t.Errorf("error response must have null data, got %q", string(env.Data))
			}
		},
	}

	// 3. POST /api/v1/admin/restore — nonexistent backup_id must yield
	//    400 RESTORE_FAILED (the "backup not found" error from
	//    backup_service is not wrapped with ErrNotFound, so the handler
	//    falls through to its 400 fallback). The body must include the
	//    modern `recovery_mode` field; the legacy `is_dry_run` alias on
	//    its own does not satisfy the request binding.
	restoreProbe := endpointProbe{
		method:        "POST",
		path:          "/api/v1/admin/restore",
		role:          "admin",
		body:          fmt.Sprintf(`{"backup_id":"%s","recovery_mode":"dry_run","reason":"mx"}`, fakeButValidUUID),
		wantStatus:    http.StatusBadRequest,
		wantSuccess:   boolPtr(false),
		wantErrorCode: "RESTORE_FAILED",
		assertFields: func(t *testing.T, env envelope) {
			t.Helper()
			if env.Error == nil {
				t.Fatal("expected error envelope, got nil")
			}
			// Critical field 1: message must mention "backup" so operators
			// can triage without reading code (must NOT be a generic
			// "operation failed").
			if !containsCI(env.Error.Message, "backup") {
				t.Errorf("error.message should mention 'backup', got %q", env.Error.Message)
			}
			// Critical field 2: must NOT leak DB internals (pgx, sql:, SQLSTATE).
			for _, suspect := range []string{"pgx", "sql:", "sqlstate", "panic", "goroutine"} {
				if containsCI(env.Error.Message, suspect) {
					t.Errorf("error.message leaks internal token %q: %q", suspect, env.Error.Message)
				}
			}
			// Critical field 3: data must be null on failure.
			if string(env.Data) != "null" && len(env.Data) != 0 {
				t.Errorf("error response must have null data, got %q", string(env.Data))
			}
		},
	}

	// 4. POST /api/v1/imports/:id/apply — nonexistent import id is
	//    wrapped via service.NotFound, so the handler maps it to
	//    404 NOT_FOUND with the domain message "import job not found".
	importApplyProbe := endpointProbe{
		method:        "POST",
		path:          "/api/v1/imports/" + zuid + "/apply",
		role:          "admin",
		body:          "",
		wantStatus:    http.StatusNotFound,
		wantSuccess:   boolPtr(false),
		wantErrorCode: "NOT_FOUND",
		assertFields: func(t *testing.T, env envelope) {
			t.Helper()
			if env.Error == nil {
				t.Fatal("expected error envelope, got nil")
			}
			// Critical field 1: exact domain message.
			if env.Error.Message != "import job not found" {
				t.Errorf("error.message must be 'import job not found', got %q", env.Error.Message)
			}
			// Critical field 2: data must be null.
			if string(env.Data) != "null" && len(env.Data) != 0 {
				t.Errorf("error response must have null data, got %q", string(env.Data))
			}
			// Critical field 3: meta.request_id must be set.
			if env.Meta == nil || env.Meta.RequestID == "" {
				t.Error("meta.request_id must be set on every response")
			}
		},
	}

	probes := []endpointProbe{
		// Health
		{method: "GET", path: "/health"},

		// Auth
		{method: "POST", path: "/api/v1/auth/register", body: `{"username":"matrix-probe-1","password":"SecurePass123!","display_name":"MP1"}`},
		{method: "POST", path: "/api/v1/auth/login", body: `{"username":"member1","password":"Seed@Pass1234"}`},
		// Logout must use a *throwaway* client so it doesn't invalidate
		// the shared "member" cookie used by every later member probe.
		{method: "POST", path: "/api/v1/auth/logout", role: "member-logout"},

		// Users
		{method: "GET", path: "/api/v1/users/me", role: "admin"},
		{method: "PATCH", path: "/api/v1/users/me", role: "admin", body: `{"display_name":"Matrix Admin"}`},

		// Catalog
		{method: "GET", path: "/api/v1/catalog/sessions"},
		{method: "GET", path: "/api/v1/catalog/sessions/" + zuid},
		{method: "GET", path: "/api/v1/catalog/products"},
		{method: "GET", path: "/api/v1/catalog/products/" + zuid},

		// Addresses
		{method: "GET", path: "/api/v1/addresses", role: "admin"},
		{method: "POST", path: "/api/v1/addresses", role: "admin", body: `{"recipient_name":"MX","phone":"1","line1":"1","city":"X"}`},
		{method: "GET", path: "/api/v1/addresses/" + zuid, role: "admin"},
		{method: "PATCH", path: "/api/v1/addresses/" + zuid, role: "admin", body: `{"recipient_name":"X","phone":"1","line1":"1","city":"X"}`},
		{method: "DELETE", path: "/api/v1/addresses/" + zuid, role: "admin"},

		// Admin
		{method: "GET", path: "/api/v1/admin/config", role: "admin"},
		{method: "PATCH", path: "/api/v1/admin/config/facility.name", role: "admin", body: `{"value":"MX","version":1}`},
		{method: "GET", path: "/api/v1/admin/feature-flags", role: "admin"},
		{method: "PATCH", path: "/api/v1/admin/feature-flags/test.flag", role: "admin", body: `{"enabled":true,"cohort_percent":10,"version":1}`},
		{method: "GET", path: "/api/v1/admin/audit-logs", role: "admin"},
		{method: "POST", path: "/api/v1/admin/backups", role: "admin"},
		{method: "GET", path: "/api/v1/admin/backups", role: "admin"},
		restoreProbe, // ── strict contract ──
		{method: "GET", path: "/api/v1/admin/archives", role: "admin"},
		{method: "POST", path: "/api/v1/admin/archives", role: "admin", body: `{"archive_type":"orders"}`},
		{method: "POST", path: "/api/v1/admin/refunds/" + zuid + "/reconcile", role: "admin", body: `{"status":"reconciled"}`},
		{method: "GET", path: "/api/v1/admin/kpis", role: "admin"},
		{method: "GET", path: "/api/v1/admin/jobs", role: "admin"},
		{method: "POST", path: "/api/v1/admin/registrations/override", role: "admin", body: fmt.Sprintf(`{"user_id":"%s","session_id":"%s"}`, zuid, zuid)},

		// Registrations
		{method: "POST", path: "/api/v1/registrations", role: "member", body: fmt.Sprintf(`{"session_id":"%s"}`, zuid)},
		{method: "GET", path: "/api/v1/registrations", role: "member"},
		{method: "GET", path: "/api/v1/registrations/" + zuid, role: "member"},
		{method: "POST", path: "/api/v1/registrations/" + zuid + "/cancel", role: "member"},
		{method: "POST", path: "/api/v1/registrations/" + zuid + "/approve", role: "staff"},
		{method: "POST", path: "/api/v1/registrations/" + zuid + "/reject", role: "staff", body: `{"reason":"mx"}`},

		// Attendance
		{method: "POST", path: "/api/v1/attendance/checkin", role: "staff", body: fmt.Sprintf(`{"registration_id":"%s","method":"qr_staff"}`, zuid)},
		{method: "POST", path: "/api/v1/attendance/leave", role: "member", body: fmt.Sprintf(`{"registration_id":"%s"}`, zuid)},
		{method: "POST", path: "/api/v1/attendance/leave/" + zuid + "/return", role: "member"},
		{method: "GET", path: "/api/v1/attendance/exceptions", role: "staff"},

		// Cart
		{method: "GET", path: "/api/v1/cart", role: "member"},
		{method: "POST", path: "/api/v1/cart/items", role: "member", body: fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, zuid)},
		{method: "DELETE", path: "/api/v1/cart/items/" + zuid, role: "member"},

		// Checkout / Buy-Now
		checkoutProbe, // ── strict contract ──
		{method: "POST", path: "/api/v1/buy-now", role: "member", body: fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1,"address_id":"%s"}`, zuid, zuid)},

		// Orders
		{method: "GET", path: "/api/v1/orders", role: "member"},
		{method: "GET", path: "/api/v1/orders/" + zuid, role: "member"},
		{method: "POST", path: "/api/v1/orders/" + zuid + "/pay", role: "member"},

		// Payments (public)
		paymentCallbackProbe, // ── strict contract ──

		// Shipments
		{method: "POST", path: "/api/v1/shipments", role: "staff", body: fmt.Sprintf(`{"order_id":"%s"}`, zuid)},
		{method: "GET", path: "/api/v1/shipments", role: "staff"},
		{method: "PATCH", path: "/api/v1/shipments/" + zuid + "/status", role: "staff", body: `{"status":"packed"}`},
		{method: "POST", path: "/api/v1/shipments/" + zuid + "/pod", role: "staff", body: `{"proof_type":"typed_acknowledgment","acknowledgment_text":"ok","receiver_name":"rcv"}`},
		{method: "POST", path: "/api/v1/shipments/" + zuid + "/exception", role: "staff", body: `{"exception_type":"minor_damage","description":"x"}`},

		// Posts
		{method: "GET", path: "/api/v1/posts"},
		{method: "GET", path: "/api/v1/posts/" + zuid},
		{method: "POST", path: "/api/v1/posts", role: "member", body: `{"title":"MX post","body":"Enough body content for MX post."}`},
		{method: "POST", path: "/api/v1/posts/" + zuid + "/report", role: "member", body: `{"reason":"spam","description":"mx"}`},

		// Moderation
		{method: "GET", path: "/api/v1/moderation/reports", role: "mod"},
		{method: "GET", path: "/api/v1/moderation/cases", role: "mod"},
		{method: "GET", path: "/api/v1/moderation/cases/" + zuid, role: "mod"},
		{method: "POST", path: "/api/v1/moderation/cases/" + zuid + "/action", role: "mod", body: `{"action_type":"dismiss","details":"mx"}`},
		{method: "POST", path: "/api/v1/moderation/bans", role: "mod", body: `{"ban_type":"posting","reason":""}`},
		{method: "POST", path: "/api/v1/moderation/bans/" + zuid + "/revoke", role: "mod"},

		// Tickets
		{method: "POST", path: "/api/v1/tickets", role: "staff", body: `{"ticket_type":"delivery_exception","title":"MX","description":"d","priority":"medium"}`},
		{method: "GET", path: "/api/v1/tickets", role: "staff"},
		{method: "GET", path: "/api/v1/tickets/" + zuid, role: "staff"},
		{method: "PATCH", path: "/api/v1/tickets/" + zuid + "/status", role: "staff", body: `{"status":"acknowledged","reason":"mx"}`},
		{method: "POST", path: "/api/v1/tickets/" + zuid + "/assign", role: "staff", body: fmt.Sprintf(`{"assigned_to":"%s"}`, zuid)},
		{method: "POST", path: "/api/v1/tickets/" + zuid + "/comments", role: "staff", body: `{"body":"mx","is_internal":true}`},
		{method: "POST", path: "/api/v1/tickets/" + zuid + "/resolve", role: "staff", body: `{"resolution_code":"fixed","resolution_summary":"mx"}`},
		{method: "POST", path: "/api/v1/tickets/" + zuid + "/close", role: "staff"},

		// Imports (POST /imports is multipart — probed separately below)
		{method: "GET", path: "/api/v1/imports", role: "admin"},
		{method: "GET", path: "/api/v1/imports/" + zuid, role: "admin"},
		{method: "POST", path: "/api/v1/imports/" + zuid + "/validate", role: "admin"},
		importApplyProbe, // ── strict contract ──

		// Exports
		{method: "POST", path: "/api/v1/exports", role: "admin", body: `{"export_type":"order_export","format":"csv","filters":{}}`},
		{method: "GET", path: "/api/v1/exports", role: "admin"},
		{method: "GET", path: "/api/v1/exports/" + zuid + "/download", role: "admin"},
	}

	for _, p := range probes {
		name := p.method + " " + p.path
		t.Run(name, func(t *testing.T) {
			c := clients[p.role]
			resp, env := call(c, p.method, e.BaseURL+p.path, p.body)
			if resp == nil {
				t.Fatal("no response — request failed at transport layer")
			}

			if p.wantStatus == 0 {
				// ── liveness fence (broad endpoints) ──
				if resp.StatusCode == http.StatusBadGateway ||
					resp.StatusCode == http.StatusGatewayTimeout ||
					resp.StatusCode == 0 {
					t.Fatalf("endpoint not reachable: got %d", resp.StatusCode)
				}
				return
			}

			// ── strict contract (high-risk endpoints) ──
			if resp.StatusCode != p.wantStatus {
				body, _ := json.Marshal(env)
				t.Fatalf("status: want %d, got %d; body=%s",
					p.wantStatus, resp.StatusCode, string(body))
			}
			if p.wantSuccess != nil && env.Success != *p.wantSuccess {
				t.Errorf("envelope.success: want %v, got %v", *p.wantSuccess, env.Success)
			}
			if p.wantErrorCode != "" {
				if env.Error == nil {
					t.Fatalf("expected error envelope with code %q, got nil error", p.wantErrorCode)
				}
				if env.Error.Code != p.wantErrorCode {
					t.Errorf("error.code: want %q, got %q", p.wantErrorCode, env.Error.Code)
				}
			}
			if p.assertFields != nil {
				p.assertFields(t, env)
			}
		})
	}

	// POST /api/v1/imports is multipart, probed separately.
	t.Run("POST /api/v1/imports", func(t *testing.T) {
		resp, _ := uploadFile(t, clients["admin"], e.BaseURL+"/api/v1/imports",
			"matrix.csv", "name,email\nMx,m@x.com\n", "general")
		if resp == nil {
			t.Fatal("no response")
		}
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout {
			t.Fatalf("endpoint not reachable: got %d", resp.StatusCode)
		}
	})
}

// containsCI reports whether s contains substr, case-insensitively.
// Tiny helper used by the strict contract assertions to fence against
// secret leakage in error messages.
func containsCI(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Fast path for ASCII.
	ls, lsub := []byte(s), []byte(substr)
	for i := range ls {
		if i+len(lsub) > len(ls) {
			break
		}
		match := true
		for j := range lsub {
			a, b := ls[i+j], lsub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
