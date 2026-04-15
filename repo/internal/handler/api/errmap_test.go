// errmap_test.go — unit tests for the handler error-mapping primitives.
//
// These primitives are the single funnel through which every handler in
// internal/handler/api/* maps a service-layer error into an HTTP
// response. A regression in mapping silently breaks the entire API
// contract (400 / 401 / 403 / 404 / 409 semantics, error envelopes,
// safe message redaction). The functions are pure-ish (only need a
// gin.Context for the response writer) so they unit-test cleanly.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newCtx builds a gin context with a response recorder and a real
// http.Request so middleware-style helpers (request id, etc.) can run.
func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, w
}

// decode unmarshals the recorded body into the standard envelope shape.
func decode(t *testing.T, w *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var env response.Envelope
	// Unwrap the typed Error (response.ErrorBody) for assertion access.
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not a valid envelope: %v body=%s", err, w.Body.String())
	}
	return env
}

// ── mapServiceError ────────────────────────────────────────────────────────

func TestMapServiceError_NilError_ReturnsFalse(t *testing.T) {
	c, w := newCtx()
	if mapServiceError(c, nil) {
		t.Error("nil error must not be mapped")
	}
	if w.Code != http.StatusOK {
		t.Errorf("nil error must not write a response body; got status %d", w.Code)
	}
}

func TestMapServiceError_Forbidden_Maps403(t *testing.T) {
	c, w := newCtx()
	err := service.Forbidden("not authorized to access this ticket")
	if !mapServiceError(c, err) {
		t.Fatal("Forbidden(...) must be mapped")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status: want 403, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Success {
		t.Error("envelope.success must be false")
	}
	if env.Error == nil || env.Error.Code != "FORBIDDEN" {
		t.Errorf("error.code: want FORBIDDEN, got %+v", env.Error)
	}
	// Domain prefix must be preserved (the ": forbidden" sentinel suffix stripped).
	if env.Error.Message != "not authorized to access this ticket" {
		t.Errorf("error.message: want clean prefix, got %q", env.Error.Message)
	}
}

func TestMapServiceError_NotFound_Maps404(t *testing.T) {
	c, w := newCtx()
	err := service.NotFound("import job not found")
	if !mapServiceError(c, err) {
		t.Fatal("NotFound(...) must be mapped")
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Success {
		t.Error("envelope.success must be false")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code: want NOT_FOUND, got %+v", env.Error)
	}
	if env.Error.Message != "import job not found" {
		t.Errorf("error.message: want clean prefix, got %q", env.Error.Message)
	}
}

func TestMapServiceError_UnknownError_ReturnsFalse(t *testing.T) {
	c, w := newCtx()
	err := errors.New("some random non-domain error")
	if mapServiceError(c, err) {
		t.Error("non-domain error must NOT be mapped (caller falls through)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("non-domain error must not write a response; got %d", w.Code)
	}
}

// errors.Is must work transitively through fmt.Errorf %w chains so that
// service code can wrap context around domain errors without breaking
// handler mapping. This is the contract the rest of the codebase relies on.
func TestMapServiceError_WrappedNotFound_StillMaps404(t *testing.T) {
	c, w := newCtx()
	wrapped := fmt.Errorf("get import job: %w", service.NotFound("import job not found"))
	if !mapServiceError(c, wrapped) {
		t.Fatal("wrapped NotFound must still be detected via errors.Is")
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("wrapped NotFound: want 404, got %d", w.Code)
	}
}

// ── safeDomainMessage ──────────────────────────────────────────────────────

func TestSafeDomainMessage_StripsForbiddenSentinel(t *testing.T) {
	got := safeDomainMessage(service.Forbidden("you cannot do that"), "fallback")
	if got != "you cannot do that" {
		t.Errorf("got %q", got)
	}
}

func TestSafeDomainMessage_StripsNotFoundSentinel(t *testing.T) {
	got := safeDomainMessage(service.NotFound("widget not found"), "fallback")
	if got != "widget not found" {
		t.Errorf("got %q", got)
	}
}

func TestSafeDomainMessage_FallsBackOnDBLeakage(t *testing.T) {
	// Any error message that mentions "pgx", "sql", "runtime", "goroutine"
	// must be replaced by the supplied fallback to avoid leaking DB
	// internals to API consumers.
	cases := []string{
		"pgx connection refused",
		"sql: no rows in result set",
		"runtime panic: bad map access",
		"goroutine 5 [running]",
	}
	for _, msg := range cases {
		got := safeDomainMessage(errors.New(msg), "internal-fallback")
		if got != "internal-fallback" {
			t.Errorf("for %q: got %q, want fallback", msg, got)
		}
	}
}

func TestSafeDomainMessage_PassesThroughCleanMessages(t *testing.T) {
	got := safeDomainMessage(errors.New("address validation failed: postal_code required"),
		"fallback")
	if got != "address validation failed: postal_code required" {
		t.Errorf("clean message must pass through; got %q", got)
	}
}

// ── safeErrorMessage ───────────────────────────────────────────────────────

func TestSafeErrorMessage_RedactsDBInternals(t *testing.T) {
	cases := []string{
		"pgx connection refused",
		"sql: no rows in result set",
		"SQLSTATE 23505 unique violation",
		"runtime.gopanic at ...",
		"goroutine 17 stack",
		"panic: nil dereference",
		"connection refused",
		"broken pipe",
	}
	for _, msg := range cases {
		got := safeErrorMessage(errors.New(msg))
		if got != "operation failed" {
			t.Errorf("for %q: got %q, want 'operation failed'", msg, got)
		}
	}
}

func TestSafeErrorMessage_PassesThroughDomainErrors(t *testing.T) {
	got := safeErrorMessage(errors.New("amount must be positive"))
	if got != "amount must be positive" {
		t.Errorf("got %q", got)
	}
}

// ── handleServiceError ─────────────────────────────────────────────────────

// Domain Forbidden error → 403 FORBIDDEN regardless of fallback.
func TestHandleServiceError_DomainForbidden_Maps403(t *testing.T) {
	c, w := newCtx()
	handleServiceError(c, service.Forbidden("nope"), http.StatusBadRequest, "FALLBACK_CODE")
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Error.Code != "FORBIDDEN" {
		t.Errorf("error.code: want FORBIDDEN, got %q", env.Error.Code)
	}
}

// Domain NotFound error → 404 NOT_FOUND regardless of fallback.
func TestHandleServiceError_DomainNotFound_Maps404(t *testing.T) {
	c, w := newCtx()
	handleServiceError(c, service.NotFound("widget"), http.StatusBadRequest, "FALLBACK_CODE")
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code: want NOT_FOUND, got %q", env.Error.Code)
	}
}

// Non-domain error → falls back to the caller's status + code.
func TestHandleServiceError_FallbackPath(t *testing.T) {
	c, w := newCtx()
	handleServiceError(c, errors.New("checkout: cart is empty"), http.StatusBadRequest, "CHECKOUT_FAILED")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Error.Code != "CHECKOUT_FAILED" {
		t.Errorf("error.code: want CHECKOUT_FAILED, got %q", env.Error.Code)
	}
	if env.Error.Message != "checkout: cart is empty" {
		t.Errorf("error.message: want clean message, got %q", env.Error.Message)
	}
}

// Non-domain error with a different fallback status (e.g. config-conflict) →
// uses that status. This is the path PATCH /admin/config/:key takes.
func TestHandleServiceError_FallbackUsesGivenStatus(t *testing.T) {
	c, w := newCtx()
	handleServiceError(c, errors.New("version mismatch"), http.StatusConflict, "CONFIG_UPDATE_FAILED")
	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Error.Code != "CONFIG_UPDATE_FAILED" {
		t.Errorf("error.code: want CONFIG_UPDATE_FAILED, got %q", env.Error.Code)
	}
}

// Fallback path with a DB-internal error message → message must be redacted.
func TestHandleServiceError_FallbackRedactsDBInternals(t *testing.T) {
	c, w := newCtx()
	handleServiceError(c, errors.New("pgx: connection refused"), http.StatusBadRequest, "X_FAILED")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	env := decode(t, w)
	if env.Error.Code != "X_FAILED" {
		t.Errorf("error.code: want X_FAILED, got %q", env.Error.Code)
	}
	if env.Error.Message != "operation failed" {
		t.Errorf("error.message must be redacted to 'operation failed', got %q",
			env.Error.Message)
	}
}

// Envelope shape invariants: every error response written through
// handleServiceError must have success=false and a non-nil error.
func TestHandleServiceError_EnvelopeShapeInvariants(t *testing.T) {
	cases := []struct {
		label string
		err   error
	}{
		{"forbidden", service.Forbidden("a")},
		{"not_found", service.NotFound("b")},
		{"fallback", errors.New("c")},
		{"redacted", errors.New("pgx: x")},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			c, w := newCtx()
			handleServiceError(c, tc.err, http.StatusBadRequest, "ANY_CODE")
			env := decode(t, w)
			if env.Success {
				t.Error("envelope.success must be false on every error path")
			}
			if env.Error == nil {
				t.Fatal("envelope.error must be present on every error path")
			}
			if env.Error.Code == "" {
				t.Error("envelope.error.code must be non-empty")
			}
			if env.Error.Message == "" {
				t.Error("envelope.error.message must be non-empty")
			}
			if w.Code < 400 {
				t.Errorf("status must be 4xx; got %d", w.Code)
			}
		})
	}
}
