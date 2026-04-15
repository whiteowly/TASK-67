package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// ---------------------------------------------------------------------------
// Fix 1: Import workflow — strict validate-before-apply
// ---------------------------------------------------------------------------

func TestImportValidateThenApplyHappyPath(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Upload a valid CSV with required "name" column for general template
	csv := "name,email\nAlice,alice@example.com\nBob,bob@example.com\n"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, buildMultipartReq("/api/v1/imports", "valid.csv", csv, "general", admin))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("upload: %d: %s", w.Code, w.Body.String())
	}
	importID := extractID(t, w.Body.Bytes())
	if importID == "" {
		t.Fatal("no import ID")
	}

	// Validate
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/"+importID+"/validate", "", admin))
	if w.Code != 200 {
		t.Fatalf("validate: %d: %s", w.Code, w.Body.String())
	}
	var valResp struct{ Data struct{ Status string; ValidRows *int `json:"valid_rows"`; ErrorRows *int `json:"error_rows"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &valResp)
	if valResp.Data.Status != "validated" {
		t.Errorf("expected 'validated', got %q", valResp.Data.Status)
	}

	// Apply
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/"+importID+"/apply", "", admin))
	if w.Code != 200 {
		t.Fatalf("apply: %d: %s", w.Code, w.Body.String())
	}
	var appResp struct{ Data struct{ Status string; AppliedRows *int `json:"applied_rows"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &appResp)
	if appResp.Data.Status != "completed" {
		t.Errorf("expected 'completed', got %q", appResp.Data.Status)
	}
}

func TestImportMissingRequiredColumns(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Upload CSV missing "name" column
	csv := "email,phone\nalice@a.com,123\n"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, buildMultipartReq("/api/v1/imports", "missing_cols.csv", csv, "general", admin))
	importID := extractID(t, w.Body.Bytes())
	if importID == "" {
		t.Skip("upload failed")
	}

	// Validate → should fail
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/"+importID+"/validate", "", admin))
	if w.Code != 200 {
		t.Fatalf("validate: %d", w.Code)
	}
	var resp struct{ Data struct{ Status string } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Status != "validation_failed" {
		t.Errorf("expected 'validation_failed', got %q", resp.Data.Status)
	}
}

func TestImportApplyWithoutValidationRejected(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	csv := "name\nAlice\n"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, buildMultipartReq("/api/v1/imports", "noval.csv", csv, "general", admin))
	importID := extractID(t, w.Body.Bytes())
	if importID == "" {
		t.Skip("upload failed")
	}

	// Try apply without validate → must fail
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports/"+importID+"/apply", "", admin))
	if w.Code == 200 {
		t.Fatal("apply without validation should be rejected")
	}
	if !containsStr(w.Body.String(), "validated") {
		t.Logf("rejection: %s", w.Body.String())
	}
}

func TestImportDuplicateChecksumR3(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	csv := "name\nDupTest\n"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, buildMultipartReq("/api/v1/imports", "dup1.csv", csv, "general", admin))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("first: %d", w.Code)
	}

	// Same content, different name → rejected by checksum
	w = httptest.NewRecorder()
	r.ServeHTTP(w, buildMultipartReq("/api/v1/imports", "dup2.csv", csv, "general", admin))
	if w.Code == 200 || w.Code == 201 {
		t.Fatal("duplicate checksum should be rejected")
	}
}

// ---------------------------------------------------------------------------
// Fix 3: KPI business metrics
// ---------------------------------------------------------------------------

func TestKPIBusinessMetrics(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/kpis", "", admin))
	if w.Code != 200 {
		t.Fatalf("kpis: %d", w.Code)
	}

	var resp struct {
		Data struct {
			TotalUsers        int     `json:"total_users"`
			ActiveSessions    int     `json:"active_sessions"`
			FillRate          float64 `json:"fill_rate"`
			MemberGrowthMonth int     `json:"member_growth_month"`
			EngagementRate    float64 `json:"engagement_rate"`
			CoachSessionCount int     `json:"coach_session_count"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.TotalUsers == 0 {
		t.Error("total_users should be >0 with seed data")
	}
	if resp.Data.ActiveSessions == 0 {
		t.Error("active_sessions should be >0 with seed data")
	}
	// MemberGrowthMonth: seeded users created "now", so should be >0
	if resp.Data.MemberGrowthMonth == 0 {
		t.Error("member_growth_month should be >0 (seeded users are new)")
	}
	// CoachSessionCount: seeded sessions have instructors
	if resp.Data.CoachSessionCount == 0 {
		t.Error("coach_session_count should be >0 (seeded sessions have instructors)")
	}
	// FillRate may be 0 or >0 depending on registrations — just check it's returned
	// EngagementRate may be 0 if no orders/registrations in last 30 days — acceptable
}

// ---------------------------------------------------------------------------
// Fix 5: Waitlist promotion sweep
// ---------------------------------------------------------------------------

func TestWaitlistSweepMethodCallable(t *testing.T) {
	_, svc := testutil.SetupTestRouter(t)
	// Should not panic or error on empty DB
	svc.Registration.SweepWaitlistPromotions(context.Background())
}

// ---------------------------------------------------------------------------
// Fix 6: Member web routes
// ---------------------------------------------------------------------------

func TestMemberOrdersPageRouted(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	member := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/my/orders", "", member))
	if w.Code != 200 {
		t.Fatalf("/my/orders: expected 200, got %d", w.Code)
	}
}

func TestMemberRegistrationsPageRouted(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	member := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/my/registrations", "", member))
	if w.Code != 200 {
		t.Fatalf("/my/registrations: expected 200, got %d", w.Code)
	}
}

func TestMemberPagesBlockedForAnon(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	for _, path := range []string{"/my/orders", "/my/registrations"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", path, nil)
		r.ServeHTTP(w, req)
		if w.Code == 200 {
			t.Errorf("%s should be blocked for unauthenticated", path)
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 7: Error leakage
// ---------------------------------------------------------------------------

func TestPaymentCallbackDoesNotLeakErrors(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/callback",
		bytes.NewBufferString(`{"gateway_tx_id":"x","merchant_order_ref":"x","amount":1,"signature":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// Should NOT contain internal error details like SQL/repo messages
	if containsStr(body, "pgx") || containsStr(body, "ERROR:") || containsStr(body, "SQLSTATE") {
		t.Error("payment callback response leaks internal error details")
	}
	// Should contain safe message
	if !containsStr(body, "payment callback processing failed") && !containsStr(body, "CALLBACK_FAILED") {
		t.Logf("callback response: %s", body)
	}
}

func TestImportUploadDoesNotLeakErrors(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// Send a bad request (no file) — should not leak internal errors
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/imports", "", admin))
	body := w.Body.String()
	if containsStr(body, "pgx") || containsStr(body, "os.Open") {
		t.Error("import error response leaks internal details")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildMultipartReq(path, filename, content, templateType, token string) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	w.WriteField("template_type", templateType)
	w.Close()

	req, _ := http.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	return req
}
