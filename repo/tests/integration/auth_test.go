package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestRegisterAndLogin(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Register
	body := `{"username":"testuser1","password":"SecurePass123!","display_name":"Test User"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Register: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if !env.Success {
		t.Fatal("Register should succeed")
	}

	// Login
	body = `{"username":"testuser1","password":"SecurePass123!"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Login: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check session cookie is set
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("Login should set session_token cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	body := `{"username":"dupuser","password":"SecurePass123!","display_name":"Dup User"}`

	// First registration
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("First register expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Duplicate registration
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Duplicate register expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterWeakPassword(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	body := `{"username":"weakuser","password":"weak"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Weak password should be rejected, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Register first
	body := `{"username":"logintest","password":"SecurePass123!"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Try wrong password
	body = `{"username":"logintest","password":"WrongPass123!"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Wrong password expected 401, got %d", w.Code)
	}
}

func TestAccountLockout(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Register
	regBody := `{"username":"locktest","password":"SecurePass123!"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Register expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// 5 failed attempts
	wrongBody := `{"username":"locktest","password":"WrongPass123!"}`
	for i := 0; i < 5; i++ {
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(wrongBody))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
	}

	// 6th attempt should indicate locked
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(wrongBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Locked account expected 401, got %d", w.Code)
	}

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if env.Error == nil {
		t.Fatal("Expected error response for locked account")
	}
	// Should mention locked
	if env.Error.Message == "" {
		t.Error("Expected error message for locked account")
	}
}

func TestLogout(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Register and login
	regBody := `{"username":"logouttest","password":"SecurePass123!"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	loginBody := `{"username":"logouttest","password":"SecurePass123!"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "session_token" {
			token = c.Value
		}
	}

	// Logout
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Logout expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session is invalid after logout
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("After logout, /me should return 401, got %d", w.Code)
	}
}

func TestGetMeAuthenticated(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Register and login
	regBody := `{"username":"metest","password":"SecurePass123!","display_name":"Me Test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	loginBody := `{"username":"metest","password":"SecurePass123!"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "session_token" {
			token = c.Value
		}
	}

	// Get /me
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /me expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var env envelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if !env.Success {
		t.Fatal("GET /me should succeed")
	}
}

func TestGetMeUnauthenticated(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Unauthenticated /me expected 401, got %d", w.Code)
	}
}
