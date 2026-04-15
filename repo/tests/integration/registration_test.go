package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestRegistrationFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// List sessions to get a session ID
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("List sessions: %d %s", w.Code, w.Body.String())
	}

	var sessResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &sessResp)
	if len(sessResp.Data) == 0 {
		t.Skip("No published sessions to test registration")
	}

	sessionID := sessResp.Data[0].ID

	// Register for session
	regBody := `{"session_id":"` + sessionID + `"}`
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations", regBody, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Register: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}

	// List my registrations
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/registrations", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("List registrations: expected 200, got %d", w.Code)
	}
}

func TestRegistrationRequiresAuth(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/registrations", bytes.NewBufferString(`{"session_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Registration without auth should return 401, got %d", w.Code)
	}
}
