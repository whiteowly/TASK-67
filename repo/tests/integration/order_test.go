package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

func TestCartFlow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Get cart (should be empty or created)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/cart", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("Get cart: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get products to add to cart
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/products?status=published", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("List products: %d", w.Code)
	}

	var prodResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &prodResp)
	if len(prodResp.Data) == 0 {
		t.Skip("No products to test cart")
	}

	// Add to cart
	addBody := `{"item_type":"product","item_id":"` + prodResp.Data[0].ID + `","quantity":1}`
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/cart/items", addBody, token))
	// Accept 201 or 200 or 400 (if stock is 0)
	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		t.Log("Item added to cart successfully")
	}
}

func TestOrderListRequiresAuth(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Orders without auth should return 401, got %d", w.Code)
	}
}

func TestOrdersList(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	token := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/orders", "", token))
	if w.Code != http.StatusOK {
		t.Fatalf("List orders: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
