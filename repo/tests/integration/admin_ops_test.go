package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestAdminBackupEndpoints(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// List backups
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/backups", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("List backups: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// List archives
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/archives", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("List archives: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminKPIDashboard(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/kpis", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("KPI dashboard: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminJobStatus(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/jobs", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("Job status: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestImportExportEndpoints(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	adminToken := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	// List imports
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/imports", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("List imports: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// List exports
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/exports", "", adminToken))
	if w.Code != http.StatusOK {
		t.Fatalf("List exports: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestShipmentEndpointsDeniedForMember(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/shipments", "", token))
	if w.Code != http.StatusForbidden {
		t.Fatalf("Member shipments: expected 403, got %d", w.Code)
	}
}

func TestStaffCanAccessShipments(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/shipments", "", staffToken))
	if w.Code != http.StatusOK {
		t.Fatalf("Staff shipments: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHealthEndpoint(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Health: expected 200, got %d", w.Code)
	}
}
