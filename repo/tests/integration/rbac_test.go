package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestAdminEndpointsDeniedForMember(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	memberToken := loginAsNewUser(t, r, "memberrbac")

	adminEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/config"},
		{"GET", "/api/v1/admin/feature-flags"},
		{"GET", "/api/v1/admin/audit-logs"},
	}

	for _, ep := range adminEndpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, authReq(ep.method, ep.path, "", memberToken))
			if w.Code != http.StatusForbidden {
				t.Errorf("Member accessing %s %s: expected 403, got %d", ep.method, ep.path, w.Code)
			}
		})
	}
}

func TestAdminEndpointsDeniedUnauthenticated(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	adminEndpoints := []string{
		"/api/v1/admin/config",
		"/api/v1/admin/feature-flags",
		"/api/v1/admin/audit-logs",
	}

	for _, path := range adminEndpoints {
		t.Run("GET "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", path, nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Unauthenticated %s: expected 401, got %d", path, w.Code)
			}
		})
	}
}

func TestAdminEndpointsAllowedForAdmin(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Login as admin (seeded by SetupTestRouter)
	token := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	adminEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/config"},
		{"GET", "/api/v1/admin/feature-flags"},
		{"GET", "/api/v1/admin/audit-logs"},
	}

	for _, ep := range adminEndpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, authReq(ep.method, ep.path, "", token))
			if w.Code != http.StatusOK {
				t.Errorf("Admin accessing %s %s: expected 200, got %d: %s", ep.method, ep.path, w.Code, w.Body.String())
			}
		})
	}
}

func loginExistingUser(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()

	loginBody := `{"username":"` + username + `","password":"` + password + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d: %s", username, w.Code, w.Body.String())
	}

	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			return c.Value
		}
	}
	t.Fatal("no session cookie")
	return ""
}
