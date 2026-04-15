package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// helper to register, login, and return session cookie
func loginAsNewUser(t *testing.T, r http.Handler, username string) string {
	t.Helper()

	regBody := `{"username":"` + username + `","password":"SecurePass123!","display_name":"Test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d %s", username, w.Code, w.Body.String())
	}

	loginBody := `{"username":"` + username + `","password":"SecurePass123!"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d", username, w.Code)
	}

	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			return c.Value
		}
	}
	t.Fatal("no session cookie after login")
	return ""
}

func authReq(method, path, body, token string) *http.Request {
	var buf *bytes.Buffer
	if body != "" {
		buf = bytes.NewBufferString(body)
	} else {
		buf = &bytes.Buffer{}
	}
	req, _ := http.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	return req
}

func TestAddressCRUD(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginAsNewUser(t, r, "addruser")

	// List (should be empty initially - seeder creates users but no addresses for new users)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/addresses", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("List addresses expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Create
	addrBody := `{
		"label": "Home",
		"recipient_name": "John Doe",
		"phone": "13800138000",
		"line1": "123 Main St",
		"city": "Beijing",
		"postal_code": "100000",
		"is_default": true
	}`
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses", addrBody, token))
	if w.Code != http.StatusCreated {
		t.Fatalf("Create address expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var createEnv struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &createEnv)
	addrID := createEnv.Data.ID

	if addrID == "" {
		t.Fatal("Expected address ID in response")
	}

	// Get
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/addresses/"+addrID, "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("Get address expected 200, got %d", w.Code)
	}

	// Update
	updateBody := `{
		"label": "Work",
		"recipient_name": "John Doe",
		"phone": "13900139000",
		"line1": "456 Office Ave",
		"city": "Shanghai",
		"is_default": true
	}`
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("PATCH", "/api/v1/addresses/"+addrID, updateBody, token))
	if w.Code != http.StatusOK {
		t.Fatalf("Update address expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("DELETE", "/api/v1/addresses/"+addrID, "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("Delete address expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted (should return 404)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/addresses/"+addrID, "", token))
	if w.Code != http.StatusNotFound {
		t.Fatalf("Deleted address should return 404, got %d", w.Code)
	}
}

func TestAddressRequiresAuth(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// No auth
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/addresses", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Addresses without auth should return 401, got %d", w.Code)
	}
}

func TestAddressOwnershipIsolation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	token1 := loginAsNewUser(t, r, "addrowner1")
	token2 := loginAsNewUser(t, r, "addrowner2")

	// User1 creates an address
	addrBody := `{"recipient_name":"Owner1","phone":"111","line1":"addr1","city":"Beijing"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses", addrBody, token1))
	if w.Code != http.StatusCreated {
		t.Fatalf("Create expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &env)

	// User2 should not be able to access user1's address
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/addresses/"+env.Data.ID, "", token2))
	if w.Code != http.StatusNotFound {
		t.Fatalf("Other user's address should return 404, got %d", w.Code)
	}
}

func TestAddressValidation(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginAsNewUser(t, r, "addrvaluser")

	// Missing required fields
	body := `{"label": "Missing fields"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/addresses", body, token))
	if w.Code == http.StatusCreated {
		t.Fatal("Address with missing fields should not be created")
	}
}
