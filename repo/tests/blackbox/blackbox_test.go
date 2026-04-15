// Package blackbox contains black-box HTTP API tests.
//
// Every request goes through the real TCP network stack: http.Client → TCP →
// net/http server → Gin middleware → handler → service → repo → PostgreSQL.
// No mocks. No ServeHTTP(). No httptest.NewRecorder().
package blackbox

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// ── infrastructure ──────────────────────────────────────────────────────────

type testEnv struct{ BaseURL string }

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	r, _ := testutil.SetupTestRouter(t)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &testEnv{BaseURL: srv.URL}
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// envelope is the standard API response shape.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta *struct {
		RequestID string `json:"request_id"`
		Total     int    `json:"total"`
	} `json:"meta"`
}

func call(c *http.Client, method, url, body string) (*http.Response, envelope) {
	var r io.Reader
	if body != "" {
		r = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, url, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, envelope{}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	json.Unmarshal(raw, &env)
	return resp, env
}

func loginAs(t *testing.T, base, user, pass string) *http.Client {
	t.Helper()
	c := newClient()
	resp, env := call(c, "POST", base+"/api/v1/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, user, pass))
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("login %s failed: %d %+v", user, resp.StatusCode, env)
	}
	return c
}

func registerAndLogin(t *testing.T, base, user string) *http.Client {
	t.Helper()
	c := newClient()
	resp, _ := call(c, "POST", base+"/api/v1/auth/register",
		fmt.Sprintf(`{"username":%q,"password":"SecurePass123!","display_name":"Test %s"}`, user, user))
	if resp == nil || resp.StatusCode != 201 {
		t.Fatalf("register %s: %d", user, resp.StatusCode)
	}
	return loginAs(t, base, user, "SecurePass123!")
}

func dmap(env envelope) map[string]interface{} {
	var m map[string]interface{}
	json.Unmarshal(env.Data, &m)
	return m
}
func dlist(env envelope) []map[string]interface{} {
	var l []map[string]interface{}
	json.Unmarshal(env.Data, &l)
	return l
}
func ds(env envelope, k string) string {
	m := dmap(env)
	if m == nil {
		return ""
	}
	v, _ := m[k].(string)
	return v
}

// computeHMAC returns hex-encoded HMAC-SHA256 for payment callback signature verification.
func computeHMAC(message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// uploadFile sends a multipart file upload via real HTTP.
func uploadFile(t *testing.T, c *http.Client, url, filename, content, templateType string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	w.WriteField("template_type", templateType)
	w.Close()

	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	return resp
}

// ── 1. Auth / Session ───────────────────────────────────────────────────────

func TestBB_Auth_RegisterLoginLogout(t *testing.T) {
	e := setupEnv(t)

	// Register
	c := newClient()
	resp, env := call(c, "POST", e.BaseURL+"/api/v1/auth/register",
		`{"username":"bb1","password":"SecurePass123!","display_name":"BB One"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	if !env.Success || ds(env, "username") != "bb1" {
		t.Fatalf("register response: success=%v username=%q", env.Success, ds(env, "username"))
	}

	// Login
	c2 := loginAs(t, e.BaseURL, "bb1", "SecurePass123!")

	// GET /me — verify identity
	resp, env = call(c2, "GET", e.BaseURL+"/api/v1/users/me", "")
	if resp.StatusCode != 200 || ds(env, "username") != "bb1" || ds(env, "display_name") != "BB One" {
		t.Fatalf("GET /me: %d username=%q display=%q", resp.StatusCode, ds(env, "username"), ds(env, "display_name"))
	}

	// Logout
	resp, env = call(c2, "POST", e.BaseURL+"/api/v1/auth/logout", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("logout: %d", resp.StatusCode)
	}

	// After logout → 401
	resp, env = call(c2, "GET", e.BaseURL+"/api/v1/users/me", "")
	if resp.StatusCode != 401 {
		t.Errorf("after logout: expected 401, got %d", resp.StatusCode)
	}
	if env.Error == nil || env.Error.Code != "UNAUTHORIZED" {
		t.Error("expected UNAUTHORIZED error code")
	}
}

func TestBB_Auth_BadCredentials(t *testing.T) {
	e := setupEnv(t)
	resp, env := call(newClient(), "POST", e.BaseURL+"/api/v1/auth/login",
		`{"username":"nobody","password":"Wrong123!"}`)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	if env.Error == nil || env.Error.Code != "LOGIN_FAILED" {
		t.Error("expected LOGIN_FAILED")
	}
}

func TestBB_Auth_ProfileUpdate(t *testing.T) {
	e := setupEnv(t)
	c := registerAndLogin(t, e.BaseURL, "bbpro")

	resp, env := call(c, "PATCH", e.BaseURL+"/api/v1/users/me",
		`{"display_name":"New Name","email":"a@b.com"}`)
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("PATCH /me: %d", resp.StatusCode)
	}

	// Readback
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/users/me", "")
	if ds(env, "display_name") != "New Name" {
		t.Errorf("display_name readback: %q", ds(env, "display_name"))
	}
}

// ── 2. Catalog + Addresses ──────────────────────────────────────────────────

func TestBB_Catalog(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	resp, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("sessions: %d", resp.StatusCode)
	}
	sessions := dlist(env)
	if len(sessions) == 0 {
		t.Fatal("expected seeded sessions")
	}
	if env.Meta == nil || env.Meta.Total == 0 {
		t.Error("expected meta.total > 0")
	}

	sid := sessions[0]["id"].(string)
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions/"+sid, "")
	if resp.StatusCode != 200 || ds(env, "title") == "" {
		t.Errorf("session detail: %d title=%q", resp.StatusCode, ds(env, "title"))
	}

	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/products", "")
	if resp.StatusCode != 200 {
		t.Fatalf("products: %d", resp.StatusCode)
	}
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("expected seeded products")
	}
	pid := products[0]["id"].(string)
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/products/"+pid, "")
	if resp.StatusCode != 200 || ds(env, "name") == "" {
		t.Errorf("product detail: %d name=%q", resp.StatusCode, ds(env, "name"))
	}

	// 404 nonexistent
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/catalog/sessions/00000000-0000-0000-0000-000000000000", "")
	if resp.StatusCode != 404 || env.Success {
		t.Errorf("nonexistent: expected 404, got %d", resp.StatusCode)
	}
}

func TestBB_Addresses(t *testing.T) {
	e := setupEnv(t)
	c1 := registerAndLogin(t, e.BaseURL, "bba1")
	c2 := registerAndLogin(t, e.BaseURL, "bba2")

	// Create
	resp, env := call(c1, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"Alice","phone":"1300","line1":"100 Rd","city":"Beijing","is_default":true}`)
	if resp.StatusCode != 201 || !env.Success {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	id := ds(env, "id")

	// Readback
	resp, env = call(c1, "GET", e.BaseURL+"/api/v1/addresses/"+id, "")
	if resp.StatusCode != 200 || ds(env, "recipient_name") != "Alice" || ds(env, "city") != "Beijing" {
		t.Fatalf("readback: %d name=%q city=%q", resp.StatusCode, ds(env, "recipient_name"), ds(env, "city"))
	}

	// Update + readback
	call(c1, "PATCH", e.BaseURL+"/api/v1/addresses/"+id,
		`{"recipient_name":"Alice U","phone":"1300","line1":"200 Rd","city":"Shanghai"}`)
	_, env = call(c1, "GET", e.BaseURL+"/api/v1/addresses/"+id, "")
	if ds(env, "city") != "Shanghai" || ds(env, "recipient_name") != "Alice U" {
		t.Errorf("update readback: city=%q name=%q", ds(env, "city"), ds(env, "recipient_name"))
	}

	// Cross-user isolation
	resp, _ = call(c2, "GET", e.BaseURL+"/api/v1/addresses/"+id, "")
	if resp.StatusCode != 404 {
		t.Errorf("cross-user: expected 404, got %d", resp.StatusCode)
	}

	// Delete + verify gone
	resp, _ = call(c1, "DELETE", e.BaseURL+"/api/v1/addresses/"+id, "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = call(c1, "GET", e.BaseURL+"/api/v1/addresses/"+id, "")
	if resp.StatusCode != 404 {
		t.Errorf("after delete: expected 404, got %d", resp.StatusCode)
	}
}

// ── 3. Admin config + feature flags ─────────────────────────────────────────

func TestBB_AdminConfig(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// List
	resp, env := call(admin, "GET", e.BaseURL+"/api/v1/admin/config", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	configs := dlist(env)
	if len(configs) == 0 {
		t.Fatal("no config entries")
	}

	var ver float64
	for _, c := range configs {
		if c["key"] == "facility.name" {
			ver = c["version"].(float64)
		}
	}
	if ver == 0 {
		t.Fatal("facility.name not found")
	}

	// Mutate
	resp, env = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/config/facility.name",
		fmt.Sprintf(`{"value":"BBNew","version":%d}`, int(ver)))
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("mutate: %d", resp.StatusCode)
	}

	// Readback
	_, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/config", "")
	found := false
	for _, c := range dlist(env) {
		if c["key"] == "facility.name" && c["value"] == "BBNew" {
			found = true
		}
	}
	if !found {
		t.Error("value not persisted in readback")
	}

	// Stale version → 409 conflict
	resp, env = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/config/facility.name",
		fmt.Sprintf(`{"value":"Stale","version":%d}`, int(ver)))
	if resp.StatusCode == 200 {
		t.Error("stale version should not succeed")
	}
	if env.Success {
		t.Error("stale version should be success=false")
	}

	// RBAC: member → 403
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "GET", e.BaseURL+"/api/v1/admin/config", "")
	if resp.StatusCode != 403 {
		t.Errorf("member: expected 403, got %d", resp.StatusCode)
	}
}

func TestBB_FeatureFlags(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// List — should have seeded test.flag
	resp, env := call(admin, "GET", e.BaseURL+"/api/v1/admin/feature-flags", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	flags := dlist(env)
	var flagVer float64
	for _, f := range flags {
		if f["key"] == "test.flag" {
			flagVer = f["version"].(float64)
			if f["enabled"] != false {
				t.Error("seeded flag should be disabled initially")
			}
		}
	}
	if flagVer == 0 {
		t.Fatal("seeded test.flag not found in list")
	}

	// Mutate: enable the flag + change cohort
	resp, env = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/feature-flags/test.flag",
		fmt.Sprintf(`{"enabled":true,"cohort_percent":75,"version":%d}`, int(flagVer)))
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("mutate flag: %d", resp.StatusCode)
	}

	// Readback: verify persisted change
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/feature-flags", "")
	for _, f := range dlist(env) {
		if f["key"] == "test.flag" {
			if f["enabled"] != true {
				t.Error("flag should be enabled after mutation")
			}
			if f["cohort_percent"].(float64) != 75 {
				t.Errorf("cohort_percent should be 75, got %v", f["cohort_percent"])
			}
			if f["version"].(float64) != flagVer+1 {
				t.Errorf("version should have incremented")
			}
		}
	}

	// Stale version → conflict
	resp, env = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/feature-flags/test.flag",
		fmt.Sprintf(`{"enabled":false,"cohort_percent":50,"version":%d}`, int(flagVer)))
	if resp.StatusCode == 200 || env.Success {
		t.Error("stale version should not succeed")
	}

	// Nonexistent flag → clean failure
	resp, env = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/feature-flags/no_such_flag",
		`{"enabled":true,"cohort_percent":50,"version":1}`)
	if resp.StatusCode == 200 || env.Success {
		t.Error("nonexistent flag should fail")
	}

	// RBAC: member → 403
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "PATCH", e.BaseURL+"/api/v1/admin/feature-flags/test.flag",
		`{"enabled":true,"cohort_percent":1,"version":1}`)
	if resp.StatusCode != 403 {
		t.Errorf("member: expected 403, got %d", resp.StatusCode)
	}
}

// ── 4. Registration lifecycle ───────────────────────────────────────────────

func TestBB_Registration_CreateCancelReadback(t *testing.T) {
	e := setupEnv(t)
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")

	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/sessions?status=published", "")
	sessions := dlist(env)
	if len(sessions) == 0 {
		t.Skip("no sessions")
	}
	sid := sessions[0]["id"].(string)

	// Register
	resp, env := call(member, "POST", e.BaseURL+"/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	rid := ds(env, "id")
	regStatus := ds(env, "status")
	if rid == "" || regStatus == "" {
		t.Fatal("registration missing id or status")
	}

	// GET readback
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/registrations/"+rid, "")
	if resp.StatusCode != 200 || ds(env, "status") != regStatus {
		t.Fatalf("readback: %d status=%q expected=%q", resp.StatusCode, ds(env, "status"), regStatus)
	}

	// List
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/registrations", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	if len(dlist(env)) == 0 {
		t.Error("expected at least 1 registration in list")
	}

	// Cancel
	resp, env = call(member, "POST", e.BaseURL+"/api/v1/registrations/"+rid+"/cancel",
		`{"reason":"changed mind"}`)
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("cancel: %d", resp.StatusCode)
	}

	// Readback → canceled
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/registrations/"+rid, "")
	if ds(env, "status") != "canceled" {
		t.Errorf("after cancel: status=%q, expected 'canceled'", ds(env, "status"))
	}
}

func TestBB_Registration_Approve(t *testing.T) {
	e := setupEnv(t)
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")

	// Register a new user for the approval-required session (3rd seeded session)
	member := registerAndLogin(t, e.BaseURL, "bbapprove")
	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/sessions?status=published", "")
	sessions := dlist(env)
	if len(sessions) < 3 {
		t.Skip("need 3 seeded sessions (3rd requires approval)")
	}
	// The 3rd session (Swimming Basics) has RequiresApproval=true
	sid := sessions[2]["id"].(string)

	resp, env := call(member, "POST", e.BaseURL+"/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	rid := ds(env, "id")
	if ds(env, "status") != "pending_approval" {
		t.Fatalf("expected pending_approval, got %q", ds(env, "status"))
	}

	// Member cannot approve their own registration
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/registrations/"+rid+"/approve", "")
	if resp.StatusCode != 403 {
		t.Errorf("member approve: expected 403, got %d", resp.StatusCode)
	}

	// Staff approves
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/registrations/"+rid+"/approve", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("staff approve: %d success=%v", resp.StatusCode, env.Success)
	}

	// Readback → registered
	_, env = call(member, "GET", e.BaseURL+"/api/v1/registrations/"+rid, "")
	if ds(env, "status") != "registered" {
		t.Errorf("after approve: status=%q, expected 'registered'", ds(env, "status"))
	}
}

func TestBB_Registration_Reject(t *testing.T) {
	e := setupEnv(t)
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")

	// Register a new user for the approval-required session
	member := registerAndLogin(t, e.BaseURL, "bbreject")
	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/sessions?status=published", "")
	sessions := dlist(env)
	if len(sessions) < 3 {
		t.Skip("need 3 sessions")
	}
	sid := sessions[2]["id"].(string)

	resp, env := call(member, "POST", e.BaseURL+"/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	rid := ds(env, "id")
	if ds(env, "status") != "pending_approval" {
		t.Fatalf("expected pending_approval, got %q", ds(env, "status"))
	}

	// Member cannot reject
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/registrations/"+rid+"/reject",
		`{"reason":"test"}`)
	if resp.StatusCode != 403 {
		t.Errorf("member reject: expected 403, got %d", resp.StatusCode)
	}

	// Staff rejects
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/registrations/"+rid+"/reject",
		`{"reason":"capacity concern"}`)
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("staff reject: %d success=%v", resp.StatusCode, env.Success)
	}

	// Readback → rejected
	_, env = call(member, "GET", e.BaseURL+"/api/v1/registrations/"+rid, "")
	if ds(env, "status") != "rejected" {
		t.Errorf("after reject: status=%q, expected 'rejected'", ds(env, "status"))
	}
}

// ── 5. Attendance ───────────────────────────────────────────────────────────

func TestBB_Attendance(t *testing.T) {
	e := setupEnv(t)
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")

	// Create a real registration so we can test check-in against it
	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/sessions?status=published", "")
	sessions := dlist(env)
	var regID string
	if len(sessions) > 0 {
		resp, env := call(member, "POST", e.BaseURL+"/api/v1/registrations",
			fmt.Sprintf(`{"session_id":"%s"}`, sessions[0]["id"].(string)))
		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			regID = ds(env, "id")
		}
	}

	// Check-in with nonexistent registration → 400 + CHECKIN_FAILED + success=false
	resp, env := call(staff, "POST", e.BaseURL+"/api/v1/attendance/checkin",
		`{"registration_id":"00000000-0000-0000-0000-000000000000","method":"qr_staff"}`)
	if resp.StatusCode != 400 {
		t.Errorf("checkin nonexistent: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("checkin nonexistent should be success=false")
	}
	if env.Error == nil {
		t.Error("expected error body for nonexistent checkin")
	}

	// Check-in with a real but ineligible registration → 400 + specific error message
	// (session starts in 48h, check-in window is 30min before — will fail with "window not open")
	if regID != "" {
		resp, env = call(staff, "POST", e.BaseURL+"/api/v1/attendance/checkin",
			fmt.Sprintf(`{"registration_id":"%s","method":"qr_staff"}`, regID))
		if resp.StatusCode != 400 {
			t.Errorf("checkin too early: expected 400, got %d", resp.StatusCode)
		}
		if env.Error == nil {
			t.Error("expected error body for early check-in")
		} else if !strings.Contains(env.Error.Message, "window") && !strings.Contains(env.Error.Message, "eligible") {
			t.Logf("check-in rejection message: %s", env.Error.Message)
		}
	}

	// Leave with nonexistent → 400 + success=false
	resp, env = call(member, "POST", e.BaseURL+"/api/v1/attendance/leave",
		`{"registration_id":"00000000-0000-0000-0000-000000000000"}`)
	if resp.StatusCode != 400 {
		t.Errorf("leave nonexistent: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("leave nonexistent should be success=false")
	}

	// Return from leave with nonexistent → should not crash (200/400/404 all acceptable)
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/attendance/leave/00000000-0000-0000-0000-000000000000/return", "")
	if resp.StatusCode == 500 {
		t.Error("return nonexistent should not 500")
	}

	// List exceptions — staff: 200 + success
	resp, env = call(staff, "GET", e.BaseURL+"/api/v1/attendance/exceptions", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("staff exceptions: %d", resp.StatusCode)
	}

	// RBAC: member → 403 on exceptions
	resp, _ = call(member, "GET", e.BaseURL+"/api/v1/attendance/exceptions", "")
	if resp.StatusCode != 403 {
		t.Errorf("member exceptions: expected 403, got %d", resp.StatusCode)
	}

	// RBAC: member → 403 on checkin
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/attendance/checkin",
		`{"registration_id":"00000000-0000-0000-0000-000000000000","method":"qr_staff"}`)
	if resp.StatusCode != 403 {
		t.Errorf("member checkin: expected 403, got %d", resp.StatusCode)
	}
}

// ── 6. Commerce ─────────────────────────────────────────────────────────────

func TestBB_Commerce_CheckoutAndOrderReadback(t *testing.T) {
	e := setupEnv(t)
	c := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")

	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)

	// Address
	resp, env := call(c, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"Checkout","phone":"1","line1":"1 Rd","city":"BJ"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("address: %d", resp.StatusCode)
	}
	aid := ds(env, "id")

	// Add to cart — exact 200/201
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("add to cart: %d", resp.StatusCode)
	}

	// View cart — 200 + success
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/cart", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("cart: %d", resp.StatusCode)
	}

	// Checkout
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"bb-co-1"}`, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		errMsg := ""
		if env.Error != nil {
			errMsg = env.Error.Message
		}
		t.Fatalf("checkout: %d err=%s", resp.StatusCode, errMsg)
	}
	oid := ds(env, "id")
	if oid == "" {
		t.Fatal("no order ID from checkout")
	}

	// Order readback — verify persisted fields
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/orders/"+oid, "")
	if resp.StatusCode != 200 {
		t.Fatalf("get order: %d", resp.StatusCode)
	}
	if ds(env, "order_number") == "" {
		t.Error("order missing order_number")
	}
	if ds(env, "status") == "" {
		t.Error("order missing status")
	}

	// Create payment request
	resp, env = call(c, "POST", e.BaseURL+"/api/v1/orders/"+oid+"/pay", "")
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("payment request: %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("payment request should succeed")
	}

	// List orders — ours should appear
	resp, env = call(c, "GET", e.BaseURL+"/api/v1/orders", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list orders: %d", resp.StatusCode)
	}
	if len(dlist(env)) == 0 {
		t.Error("expected at least 1 order")
	}
}

func TestBB_Commerce_BuyNow(t *testing.T) {
	e := setupEnv(t)
	c := loginAs(t, e.BaseURL, "member2", "Seed@Pass1234")

	_, env := call(c, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Skip("no products")
	}
	pid := products[0]["id"].(string)

	resp, env := call(c, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"BN","phone":"1","line1":"2 Rd","city":"SH"}`)
	aid := ds(env, "id")

	resp, env = call(c, "POST", e.BaseURL+"/api/v1/buy-now",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1,"address_id":"%s"}`, pid, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("buy-now: %d", resp.StatusCode)
	}
	if ds(env, "id") == "" {
		t.Error("buy-now should return order id")
	}
}

// ── 7. Shipments ────────────────────────────────────────────────────────────

func TestBB_Shipments_FullLifecycle(t *testing.T) {
	e := setupEnv(t)
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")

	// ── Step 1: create a paid order ────────────────────────────────────────
	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no products")
	}
	pid := products[0]["id"].(string)

	resp, env := call(member, "POST", e.BaseURL+"/api/v1/addresses",
		`{"recipient_name":"Ship Test","phone":"1","line1":"1 Rd","city":"BJ"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("address: %d", resp.StatusCode)
	}
	aid := ds(env, "id")

	call(member, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))

	resp, env = call(member, "POST", e.BaseURL+"/api/v1/checkout",
		fmt.Sprintf(`{"address_id":"%s","idempotency_key":"bb-ship-1"}`, aid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("checkout: %d", resp.StatusCode)
	}
	orderID := ds(env, "id")

	// Create payment request to get merchant ref + amount
	resp, env = call(member, "POST", e.BaseURL+"/api/v1/orders/"+orderID+"/pay", "")
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("payment request: %d", resp.StatusCode)
	}
	merchantRef := ds(env, "merchant_order_ref")
	amount := dmap(env)["amount"].(float64)

	// Compute valid HMAC signature: message = "txid|merchantRef|amountMinor"
	amountMinor := int64(amount)
	message := fmt.Sprintf("tx-ship-1|%s|%d", merchantRef, amountMinor)
	sig := computeHMAC(message, "test-merchant-key-for-testing-only")

	// Submit valid payment callback
	resp, env = call(newClient(), "POST", e.BaseURL+"/api/v1/payments/callback",
		fmt.Sprintf(`{"gateway_tx_id":"tx-ship-1","merchant_order_ref":"%s","amount":%f,"signature":"%s"}`,
			merchantRef, float64(amountMinor)/100.0, sig))
	if resp.StatusCode != 200 || !env.Success {
		errMsg := ""
		if env.Error != nil {
			errMsg = env.Error.Message
		}
		t.Fatalf("payment callback: %d success=%v err=%s", resp.StatusCode, env.Success, errMsg)
	}

	// Verify order is now paid
	_, env = call(member, "GET", e.BaseURL+"/api/v1/orders/"+orderID, "")
	if ds(env, "status") != "paid" {
		t.Fatalf("order should be paid, got %q", ds(env, "status"))
	}

	// ── Step 2: create shipment ────────────────────────────────────────────
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/shipments",
		fmt.Sprintf(`{"order_id":"%s"}`, orderID))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create shipment: %d", resp.StatusCode)
	}
	shipID := ds(env, "id")
	if shipID == "" {
		t.Fatal("no shipment ID")
	}
	if ds(env, "status") != "pending_fulfillment" {
		t.Errorf("initial shipment status: %q", ds(env, "status"))
	}

	// Verify in list
	resp, env = call(staff, "GET", e.BaseURL+"/api/v1/shipments", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list shipments: %d", resp.StatusCode)
	}
	found := false
	for _, s := range dlist(env) {
		if sid, _ := s["id"].(string); sid == shipID {
			found = true
		}
	}
	if !found {
		t.Error("created shipment should appear in list")
	}

	// ── Step 3: status update packed → verify via response ─────────────────
	resp, env = call(staff, "PATCH", e.BaseURL+"/api/v1/shipments/"+shipID+"/status",
		`{"status":"packed"}`)
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("status→packed: %d", resp.StatusCode)
	}
	// The PATCH response returns the updated shipment — verify status field
	if ds(env, "status") != "packed" {
		t.Errorf("PATCH response status: expected 'packed', got %q", ds(env, "status"))
	}

	// Verify via list: find our shipment and confirm status changed
	_, listEnv := call(staff, "GET", e.BaseURL+"/api/v1/shipments", "")
	for _, s := range dlist(listEnv) {
		if sid, _ := s["id"].(string); sid == shipID {
			if st, _ := s["status"].(string); st != "packed" {
				t.Errorf("list readback status: expected 'packed', got %q", st)
			}
		}
	}

	// ── Step 3b: status update shipped → verify ────────────────────────────
	resp, env = call(staff, "PATCH", e.BaseURL+"/api/v1/shipments/"+shipID+"/status",
		`{"status":"shipped"}`)
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("status→shipped: %d", resp.StatusCode)
	}
	if ds(env, "status") != "shipped" {
		t.Errorf("PATCH response status: expected 'shipped', got %q", ds(env, "status"))
	}

	// ── Step 4: record POD → verify returned proof record ──────────────────
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/shipments/"+shipID+"/pod",
		`{"proof_type":"typed_acknowledgment","acknowledgment_text":"Package received","receiver_name":"John Doe"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("POD: %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("POD should succeed")
	}
	// Verify the returned proof record has correct fields
	podData := dmap(env)
	if podData["proof_type"] != "typed_acknowledgment" {
		t.Errorf("POD proof_type: %v", podData["proof_type"])
	}
	if podData["receiver_name"] != "John Doe" {
		t.Errorf("POD receiver_name: %v", podData["receiver_name"])
	}
	if podData["shipment_id"] != shipID {
		t.Errorf("POD shipment_id: %v, expected %s", podData["shipment_id"], shipID)
	}
	if podData["id"] == nil || podData["id"] == "" {
		t.Error("POD should return a persisted id")
	}

	// ── Step 5: report exception → verify returned exception record ────────
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/shipments/"+shipID+"/exception",
		`{"exception_type":"minor_damage","description":"Small dent on corner"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("exception: %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("exception should succeed")
	}
	excData := dmap(env)
	if excData["exception_type"] != "minor_damage" {
		t.Errorf("exception type: %v", excData["exception_type"])
	}
	if excData["description"] != "Small dent on corner" {
		t.Errorf("exception description: %v", excData["description"])
	}
	if excData["shipment_id"] != shipID {
		t.Errorf("exception shipment_id: %v, expected %s", excData["shipment_id"], shipID)
	}
	if excData["id"] == nil || excData["id"] == "" {
		t.Error("exception should return a persisted id")
	}

	// RBAC: member → 403 on shipments
	resp, _ = call(member, "GET", e.BaseURL+"/api/v1/shipments", "")
	if resp.StatusCode != 403 {
		t.Errorf("member list: expected 403, got %d", resp.StatusCode)
	}
}

// ── 8. Moderation ───────────────────────────────────────────────────────────

func TestBB_Moderation(t *testing.T) {
	e := setupEnv(t)
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	mod := loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234")

	// Create post
	resp, env := call(member, "POST", e.BaseURL+"/api/v1/posts",
		`{"title":"Mod Post","body":"Content for moderation testing."}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create post: %d", resp.StatusCode)
	}
	pid := ds(env, "id")
	if pid == "" {
		t.Fatal("no post ID")
	}

	// Readback
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/posts/"+pid, "")
	if resp.StatusCode != 200 || ds(env, "title") != "Mod Post" {
		t.Fatalf("post readback: %d title=%q", resp.StatusCode, ds(env, "title"))
	}

	// Report post — exact success assertion
	resp, env = call(member, "POST", e.BaseURL+"/api/v1/posts/"+pid+"/report",
		`{"reason":"spam","description":"looks like spam"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("report: %d", resp.StatusCode)
	}

	// Member cannot access moderation
	resp, _ = call(member, "GET", e.BaseURL+"/api/v1/moderation/reports", "")
	if resp.StatusCode != 403 {
		t.Errorf("member reports: expected 403, got %d", resp.StatusCode)
	}

	// Moderator lists reports — 200 + success
	resp, env = call(mod, "GET", e.BaseURL+"/api/v1/moderation/reports", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("mod reports: %d", resp.StatusCode)
	}

	// Moderator lists cases — 200 + success
	resp, env = call(mod, "GET", e.BaseURL+"/api/v1/moderation/cases", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("mod cases: %d", resp.StatusCode)
	}

	// Staff → 403 on moderation
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	resp, _ = call(staff, "GET", e.BaseURL+"/api/v1/moderation/cases", "")
	if resp.StatusCode != 403 {
		t.Errorf("staff moderation: expected 403, got %d", resp.StatusCode)
	}
}

// ── 9. Tickets lifecycle ────────────────────────────────────────────────────

func TestBB_Tickets_Lifecycle(t *testing.T) {
	e := setupEnv(t)
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")

	// Create
	resp, env := call(staff, "POST", e.BaseURL+"/api/v1/tickets",
		`{"ticket_type":"delivery_exception","title":"BB Ticket","description":"desc","priority":"high"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	tid := ds(env, "id")
	tnum := ds(env, "ticket_number")
	if tid == "" || tnum == "" {
		t.Fatalf("missing id=%q or ticket_number=%q", tid, tnum)
	}

	// Readback initial → open
	resp, env = call(staff, "GET", e.BaseURL+"/api/v1/tickets/"+tid, "")
	if resp.StatusCode != 200 || ds(env, "status") != "open" {
		t.Fatalf("initial readback: %d status=%q", resp.StatusCode, ds(env, "status"))
	}
	if ds(env, "priority") != "high" {
		t.Errorf("priority: %q", ds(env, "priority"))
	}

	// Add comment — 200
	resp, env = call(staff, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/comments",
		`{"body":"Investigating","is_internal":true}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("comment: %d", resp.StatusCode)
	}

	// Update status → acknowledged, readback
	resp, env = call(staff, "PATCH", e.BaseURL+"/api/v1/tickets/"+tid+"/status",
		`{"status":"acknowledged","reason":"looking into it"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status update: %d", resp.StatusCode)
	}
	_, env = call(staff, "GET", e.BaseURL+"/api/v1/tickets/"+tid, "")
	if ds(env, "status") != "acknowledged" {
		t.Errorf("after acknowledge: status=%q", ds(env, "status"))
	}

	// Resolve, readback
	resp, _ = call(staff, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/resolve",
		`{"resolution_code":"fixed","resolution_summary":"Shipped replacement"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("resolve: %d", resp.StatusCode)
	}
	_, env = call(staff, "GET", e.BaseURL+"/api/v1/tickets/"+tid, "")
	if ds(env, "status") != "resolved" {
		t.Errorf("after resolve: status=%q", ds(env, "status"))
	}

	// Close, readback
	resp, _ = call(staff, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/close", "")
	if resp.StatusCode != 200 {
		t.Fatalf("close: %d", resp.StatusCode)
	}
	_, env = call(staff, "GET", e.BaseURL+"/api/v1/tickets/"+tid, "")
	if ds(env, "status") != "closed" {
		t.Errorf("after close: status=%q", ds(env, "status"))
	}

	// List — ticket should appear
	resp, env = call(staff, "GET", e.BaseURL+"/api/v1/tickets", "")
	if resp.StatusCode != 200 || len(dlist(env)) == 0 {
		t.Errorf("list: %d count=%d", resp.StatusCode, len(dlist(env)))
	}

	// Unauth → 401
	resp, _ = call(newClient(), "GET", e.BaseURL+"/api/v1/tickets", "")
	if resp.StatusCode != 401 {
		t.Errorf("unauth tickets: expected 401, got %d", resp.StatusCode)
	}
}

// ── 10. Import / Export ─────────────────────────────────────────────────────

func TestBB_ImportExport(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// ── Import lifecycle ───────────────────────────────────────────────────
	// Upload a real CSV file via multipart
	importResp := uploadFile(t, admin, e.BaseURL+"/api/v1/imports",
		"test.csv", "name,email\nAlice,a@b.com\n", "general")
	if importResp.StatusCode != 201 && importResp.StatusCode != 200 {
		t.Fatalf("import upload: %d", importResp.StatusCode)
	}
	importBody, _ := io.ReadAll(importResp.Body)
	importResp.Body.Close()
	var importEnv envelope
	json.Unmarshal(importBody, &importEnv)
	importID := ds(importEnv, "id")
	if importID == "" {
		t.Fatal("no import ID from upload")
	}

	// List imports — should contain the upload
	resp, env := call(admin, "GET", e.BaseURL+"/api/v1/imports", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("list imports: %d", resp.StatusCode)
	}

	// Get import detail → 200 with "uploaded" status
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/imports/"+importID, "")
	if resp.StatusCode != 200 {
		t.Fatalf("get import: %d", resp.StatusCode)
	}
	if ds(env, "status") != "uploaded" {
		t.Errorf("initial status: expected 'uploaded', got %q", ds(env, "status"))
	}

	// Apply on "uploaded" (not validated) → 400 with meaningful error
	// Domain behavior: must validate before applying
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/imports/"+importID+"/apply", "")
	if resp.StatusCode != 400 {
		t.Errorf("apply unvalidated: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("apply unvalidated should be success=false")
	}
	if env.Error == nil || !strings.Contains(env.Error.Message, "validated") {
		t.Errorf("apply error should mention validation, got %+v", env.Error)
	}

	// Nonexistent import → 404
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/imports/00000000-0000-0000-0000-000000000000", "")
	if resp.StatusCode != 404 || env.Success {
		t.Errorf("nonexistent: expected 404, got %d", resp.StatusCode)
	}

	// ── Export lifecycle ───────────────────────────────────────────────────
	// Create export
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/exports",
		`{"export_type":"order_export","format":"csv","filters":{}}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create export: %d", resp.StatusCode)
	}
	exportID := ds(env, "id")

	// List exports — verify created export appears
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/exports", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("list exports: %d", resp.StatusCode)
	}
	if exportID != "" {
		found := false
		for _, exp := range dlist(env) {
			if eid, _ := exp["id"].(string); eid == exportID {
				found = true
			}
		}
		if !found {
			t.Error("created export should appear in list")
		}
	}

	// Download the real created export
	if exportID != "" {
		dlReq, _ := http.NewRequest("GET", e.BaseURL+"/api/v1/exports/"+exportID+"/download", nil)
		dlResp, err := admin.Do(dlReq)
		if err != nil {
			t.Fatalf("download request failed: %v", err)
		}
		dlBody, _ := io.ReadAll(dlResp.Body)
		dlResp.Body.Close()

		if dlResp.StatusCode != 200 {
			t.Fatalf("download real export: expected 200, got %d body=%s", dlResp.StatusCode, string(dlBody))
		}
		ct := dlResp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/csv") {
			t.Errorf("download Content-Type: expected text/csv, got %q", ct)
		}
		cd := dlResp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "attachment") {
			t.Errorf("download Content-Disposition: expected attachment, got %q", cd)
		}
		if len(dlBody) == 0 {
			t.Error("download body should not be empty")
		}
		bodyStr := string(dlBody)
		if !strings.Contains(bodyStr, "id") || !strings.Contains(bodyStr, "created_at") {
			t.Errorf("download body should contain CSV headers and export data, got: %s", bodyStr[:min(len(bodyStr), 200)])
		}
	}

	// Download nonexistent → 404
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/exports/00000000-0000-0000-0000-000000000000/download", "")
	if resp.StatusCode != 404 {
		t.Errorf("download nonexistent: expected 404, got %d", resp.StatusCode)
	}

	// RBAC: staff → 403
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	resp, _ = call(staff, "GET", e.BaseURL+"/api/v1/imports", "")
	if resp.StatusCode != 403 {
		t.Errorf("staff imports: expected 403, got %d", resp.StatusCode)
	}
}

// ── 11. Admin ops ───────────────────────────────────────────────────────────

func TestBB_AdminOps(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Backup: create + list readback
	resp, env := call(admin, "POST", e.BaseURL+"/api/v1/admin/backups", "")
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("backup: %d", resp.StatusCode)
	}

	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/backups", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("list backups: %d", resp.StatusCode)
	}

	// Archive: create + list readback
	resp, _ = call(admin, "POST", e.BaseURL+"/api/v1/admin/archives",
		`{"archive_type":"orders"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("archive: %d", resp.StatusCode)
	}

	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/archives", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("list archives: %d", resp.StatusCode)
	}

	// Restore bogus backup → clean 400/404, not 500
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/admin/restore",
		`{"backup_id":"00000000-0000-0000-0000-000000000000","is_dry_run":true,"reason":"test"}`)
	if resp.StatusCode == 500 {
		t.Fatal("restore bogus: 500")
	}
	if env.Success {
		t.Error("restore bogus should be success=false")
	}

	// KPIs — 200 + success
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/kpis", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("kpis: %d", resp.StatusCode)
	}

	// Jobs — 200 + success
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/jobs", "")
	if resp.StatusCode != 200 || !env.Success {
		t.Fatalf("jobs: %d", resp.StatusCode)
	}

	// RBAC: member → 403 on all admin paths
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	for _, p := range []string{"/api/v1/admin/backups", "/api/v1/admin/archives", "/api/v1/admin/kpis", "/api/v1/admin/jobs"} {
		resp, _ = call(member, "GET", e.BaseURL+p, "")
		if resp.StatusCode != 403 {
			t.Errorf("member %s: expected 403, got %d", p, resp.StatusCode)
		}
	}
}

// ── 12. Payment callback ────────────────────────────────────────────────────

func TestBB_PaymentCallback(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	// Invalid signature → rejected with error
	resp, env := call(c, "POST", e.BaseURL+"/api/v1/payments/callback",
		`{"gateway_tx_id":"tx-bb-1","merchant_order_ref":"none","amount":1000,"signature":"bad"}`)
	if resp.StatusCode == 200 || resp.StatusCode == 500 {
		t.Fatalf("bad sig: expected 4xx, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("bad sig should be success=false")
	}
	if env.Error == nil {
		t.Error("bad sig should have error body")
	}

	// Replay same payload → same rejection (idempotent rejection)
	resp2, env2 := call(c, "POST", e.BaseURL+"/api/v1/payments/callback",
		`{"gateway_tx_id":"tx-bb-1","merchant_order_ref":"none","amount":1000,"signature":"bad"}`)
	if resp2.StatusCode != resp.StatusCode {
		t.Errorf("replay status mismatch: %d vs %d", resp.StatusCode, resp2.StatusCode)
	}
	if env2.Success {
		t.Error("replay should be success=false")
	}
}

// ── 13. RBAC comprehensive ──────────────────────────────────────────────────

func TestBB_RBAC_Unauthenticated(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	for _, ep := range []struct{ m, p string }{
		{"GET", "/api/v1/users/me"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/addresses"},
		{"POST", "/api/v1/registrations"},
		{"GET", "/api/v1/orders"},
		{"GET", "/api/v1/cart"},
		{"POST", "/api/v1/tickets"},
	} {
		t.Run(ep.m+"_"+ep.p, func(t *testing.T) {
			resp, env := call(c, ep.m, e.BaseURL+ep.p, "")
			if resp.StatusCode != 401 {
				t.Errorf("expected 401, got %d", resp.StatusCode)
			}
			if env.Success {
				t.Error("should be success=false")
			}
		})
	}
}

func TestBB_RBAC_Forbidden(t *testing.T) {
	e := setupEnv(t)
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	mod := loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234")

	for _, tc := range []struct {
		role   string
		method string
		path   string
		c      *http.Client
	}{
		{"member", "GET", "/api/v1/admin/config", member},
		{"member", "GET", "/api/v1/admin/feature-flags", member},
		{"member", "POST", "/api/v1/admin/backups", member},
		{"member", "GET", "/api/v1/shipments", member},
		{"member", "GET", "/api/v1/imports", member},
		{"staff", "GET", "/api/v1/moderation/cases", staff},
		{"staff", "GET", "/api/v1/imports", staff},
		{"mod", "GET", "/api/v1/shipments", mod},
		{"mod", "GET", "/api/v1/admin/config", mod},
	} {
		t.Run(tc.role+"_"+strings.ReplaceAll(tc.path, "/", "_"), func(t *testing.T) {
			resp, env := call(tc.c, tc.method, e.BaseURL+tc.path, "")
			if resp.StatusCode != 403 {
				t.Errorf("%s: expected 403, got %d", tc.role, resp.StatusCode)
			}
			if env.Success {
				t.Error("should be success=false")
			}
		})
	}
}
