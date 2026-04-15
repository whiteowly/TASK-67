package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestCatalogListSessions(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List sessions expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if !env.Success {
		t.Fatal("List sessions should succeed")
	}
}

func TestCatalogListProducts(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List products expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if !env.Success {
		t.Fatal("List products should succeed")
	}
}

func TestCatalogSearchSessions(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions?q=yoga", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Search sessions expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCatalogSessionNotFound(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions/00000000-0000-0000-0000-000000000000", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Not found session expected 404, got %d", w.Code)
	}
}

func TestCatalogInvalidSessionID(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Invalid UUID should return 404, got %d", w.Code)
	}
}

func TestCatalogPagination(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions?page=1&per_page=2", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Paginated sessions expected 200, got %d", w.Code)
	}

	var env struct {
		Success bool `json:"success"`
		Meta    struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}
	json.Unmarshal(w.Body.Bytes(), &env)

	if env.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", env.Meta.Page)
	}
	if env.Meta.PerPage != 2 {
		t.Errorf("expected per_page 2, got %d", env.Meta.PerPage)
	}
}

func TestCatalogFilterByCategory(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/catalog/sessions?category=Yoga", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Filter by category expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
