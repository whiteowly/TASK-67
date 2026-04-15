package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestPostCRUD(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Create post
	body := `{"title":"Test Post","body":"This is a test post body with enough content."}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/posts", body, token))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Create post: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List posts
	w = httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/posts", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("List posts: expected 200, got %d", w.Code)
	}
}

func TestModerationEndpointsDeniedForMember(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	endpoints := []string{
		"/api/v1/moderation/reports",
		"/api/v1/moderation/cases",
	}

	for _, path := range endpoints {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authReq("GET", path, "", token))
		if w.Code != http.StatusForbidden {
			t.Errorf("Member accessing %s: expected 403, got %d", path, w.Code)
		}
	}
}

func TestModerationAccessForModerator(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "mod1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/moderation/cases", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("Moderator accessing cases: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTicketCRUD(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staffToken := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	// Create ticket
	body := `{"ticket_type":"occupancy_exception","title":"Test Exception","description":"A test ticket","priority":"medium"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/tickets", body, staffToken))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("Create ticket: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var ticketResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &ticketResp)

	// List tickets
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets", "", staffToken))
	if w.Code != http.StatusOK {
		t.Fatalf("List tickets: expected 200, got %d", w.Code)
	}
}

func TestPaymentCallback(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	// Test invalid callback (should be rejected but not crash)
	body := `{"gateway_tx_id":"test-tx-123","merchant_order_ref":"nonexistent","amount":1000,"signature":"invalid"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should return error but not 500
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("Payment callback should not return 500: %s", w.Body.String())
	}
}
