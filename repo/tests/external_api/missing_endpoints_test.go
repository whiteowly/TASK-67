// missing_endpoints_test.go
//
// Every endpoint that had no external HTTP coverage in tests/blackbox/ is
// exercised here via real HTTP. Each test validates at minimum:
//   - status code,
//   - response envelope shape,
//   - (where applicable) auth / RBAC behavior,
//   - (where applicable) a graceful failure mode for invalid input.
package external_api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ── 1. GET /health ──────────────────────────────────────────────────────────

func TestExternal_Health(t *testing.T) {
	e := setupEnv(t)
	c := newClient()

	resp, body := rawCall(c, "GET", e.BaseURL+"/health")
	if resp == nil {
		t.Fatal("no response")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("GET /health: expected 200, got %d", resp.StatusCode)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("GET /health: body not JSON: %s", string(body))
	}
	if doc["status"] != "ok" {
		t.Errorf("GET /health: expected status=ok, got %+v", doc)
	}
}

// ── 2. GET /api/v1/admin/audit-logs ─────────────────────────────────────────

func TestExternal_AdminAuditLogs(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Trigger at least one audited action so the list is non-empty.
	resp, env := call(admin, "GET", e.BaseURL+"/api/v1/admin/config", "")
	if resp.StatusCode != 200 {
		t.Fatalf("prep: GET /admin/config: %d", resp.StatusCode)
	}
	configs := dlist(env)
	var ver float64
	for _, c := range configs {
		if c["key"] == "facility.name" {
			ver = c["version"].(float64)
			break
		}
	}
	if ver > 0 {
		_, _ = call(admin, "PATCH", e.BaseURL+"/api/v1/admin/config/facility.name",
			fmt.Sprintf(`{"value":"AuditLogTrigger","version":%d}`, int(ver)))
	}

	// List audit logs — 200 + success + data is a list
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/admin/audit-logs", "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /admin/audit-logs: expected 200, got %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("GET /admin/audit-logs: expected success=true")
	}
	if env.Data == nil {
		t.Error("GET /admin/audit-logs: expected data field in envelope")
	}
	// Shape check: should unmarshal as list of objects.
	logs := dlist(env)
	_ = logs // empty list is acceptable

	// RBAC: non-admin → 403
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/admin/audit-logs", "")
	if resp.StatusCode != 403 {
		t.Errorf("member /admin/audit-logs: expected 403, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("member 403: expected success=false")
	}
}

// ── 3. POST /api/v1/admin/refunds/:id/reconcile ─────────────────────────────

func TestExternal_AdminRefundReconcile(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Missing body → validation error (400)
	resp, env := call(admin, "POST",
		e.BaseURL+"/api/v1/admin/refunds/00000000-0000-0000-0000-000000000000/reconcile", "")
	if resp.StatusCode != 400 {
		t.Errorf("no body: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("no body: expected success=false")
	}

	// Invalid refund ID path param → 404 via the wired handler
	resp, env = call(admin, "POST",
		e.BaseURL+"/api/v1/admin/refunds/not-a-uuid/reconcile", `{"status":"reconciled"}`)
	if resp.StatusCode != 404 {
		t.Errorf("bad uuid: expected 404, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("bad uuid: expected success=false")
	}

	// Well-formed UUID but nonexistent refund → endpoint reachable, clean error,
	// should NOT 500.
	resp, env = call(admin, "POST",
		e.BaseURL+"/api/v1/admin/refunds/00000000-0000-0000-0000-000000000000/reconcile",
		`{"status":"reconciled"}`)
	if resp.StatusCode == 500 {
		t.Fatalf("nonexistent refund should not 500: body=%s", env.Error)
	}
	if resp.StatusCode < 400 {
		t.Errorf("nonexistent refund: expected 4xx, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("nonexistent refund: expected success=false")
	}

	// RBAC: member → 403
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "POST",
		e.BaseURL+"/api/v1/admin/refunds/00000000-0000-0000-0000-000000000000/reconcile",
		`{"status":"reconciled"}`)
	if resp.StatusCode != 403 {
		t.Errorf("member refund reconcile: expected 403, got %d", resp.StatusCode)
	}
}

// ── 4. POST /api/v1/admin/registrations/override ────────────────────────────

func TestExternal_AdminRegistrationOverride(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Register a fresh user so we have a user_id to override for.
	_, newUserID := registerAndLogin(t, e.BaseURL, "override-target")
	if newUserID == "" {
		t.Fatal("could not obtain target user id")
	}

	// Get a real session id.
	_, env := call(admin, "GET", e.BaseURL+"/api/v1/catalog/sessions", "")
	sessions := dlist(env)
	if len(sessions) == 0 {
		t.Fatal("seed broken: no published sessions present")
	}
	sid := sessions[0]["id"].(string)

	// Happy path: admin overrides-registers the member.
	resp, env := call(admin, "POST", e.BaseURL+"/api/v1/admin/registrations/override",
		fmt.Sprintf(`{"user_id":"%s","session_id":"%s","reason":"external-test"}`,
			newUserID, sid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		// Some sessions may already have a registration for that user from other
		// tests; accept a clean 4xx rejection as valid coverage of the path.
		if resp.StatusCode >= 500 {
			t.Fatalf("override register should not 500, got %d", resp.StatusCode)
		}
		if env.Success {
			t.Error("failure path: expected success=false")
		}
	} else {
		if !env.Success {
			t.Error("happy path: expected success=true")
		}
		if ds(env, "id") == "" {
			t.Error("happy path: expected returned registration id")
		}
	}

	// Missing body → 400 validation error.
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/admin/registrations/override", "")
	if resp.StatusCode != 400 {
		t.Errorf("no body: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("no body: expected success=false")
	}

	// RBAC: member → 403
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/admin/registrations/override",
		fmt.Sprintf(`{"user_id":"%s","session_id":"%s"}`, newUserID, sid))
	if resp.StatusCode != 403 {
		t.Errorf("member override: expected 403, got %d", resp.StatusCode)
	}
}

// ── 5. DELETE /api/v1/cart/items/:id ────────────────────────────────────────

func TestExternal_CartItemDelete(t *testing.T) {
	e := setupEnv(t)
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")

	// Get a product.
	_, env := call(member, "GET", e.BaseURL+"/api/v1/catalog/products?status=published", "")
	products := dlist(env)
	if len(products) == 0 {
		t.Fatal("no seeded products")
	}
	pid := products[0]["id"].(string)

	// Add to cart.
	resp, env := call(member, "POST", e.BaseURL+"/api/v1/cart/items",
		fmt.Sprintf(`{"item_type":"product","item_id":"%s","quantity":1}`, pid))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("add to cart: %d", resp.StatusCode)
	}

	// Get cart to find the cart_item id.
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/cart", "")
	if resp.StatusCode != 200 {
		t.Fatalf("get cart: %d", resp.StatusCode)
	}
	cart := dmap(env)
	items, _ := cart["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("cart empty after add")
	}
	firstItem := items[0].(map[string]interface{})
	itemID, _ := firstItem["id"].(string)
	if itemID == "" {
		t.Fatal("no cart item id")
	}

	// DELETE the cart item — success.
	resp, env = call(member, "DELETE", e.BaseURL+"/api/v1/cart/items/"+itemID, "")
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE /cart/items/:id: expected 200, got %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("DELETE: expected success=true")
	}

	// Readback: cart should no longer contain the item.
	resp, env = call(member, "GET", e.BaseURL+"/api/v1/cart", "")
	if resp.StatusCode != 200 {
		t.Fatalf("readback cart: %d", resp.StatusCode)
	}
	cart = dmap(env)
	items, _ = cart["items"].([]interface{})
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["id"] == itemID {
			t.Error("cart item should be gone after DELETE")
		}
	}

	// Bad UUID path → 404 / 4xx, not 500.
	resp, env = call(member, "DELETE", e.BaseURL+"/api/v1/cart/items/not-a-uuid", "")
	if resp.StatusCode == 500 {
		t.Error("bad uuid: should not 500")
	}
	if env.Success {
		t.Error("bad uuid: expected success=false")
	}

	// Unauthenticated → 401.
	resp, _ = call(newClient(), "DELETE", e.BaseURL+"/api/v1/cart/items/"+itemID, "")
	if resp.StatusCode != 401 {
		t.Errorf("unauth: expected 401, got %d", resp.StatusCode)
	}
}

// ── 6. GET /api/v1/posts ────────────────────────────────────────────────────

func TestExternal_ListPosts(t *testing.T) {
	e := setupEnv(t)

	// Create a post to ensure the list is non-empty.
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, env := call(member, "POST", e.BaseURL+"/api/v1/posts",
		`{"title":"External test post","body":"Body content for external_api test."}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("prep create post: %d", resp.StatusCode)
	}
	newPostID := ds(env, "id")
	if newPostID == "" {
		t.Fatal("prep: no new post id")
	}

	// Public GET — no auth required.
	pub := newClient()
	resp, env = call(pub, "GET", e.BaseURL+"/api/v1/posts", "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /posts: expected 200, got %d", resp.StatusCode)
	}
	if !env.Success {
		t.Error("GET /posts: expected success=true")
	}
	// Envelope shape: .meta.total should be >= 1.
	if env.Meta == nil || env.Meta.Total < 1 {
		t.Errorf("GET /posts: expected meta.total >= 1, got %+v", env.Meta)
	}

	// Data should be a JSON array and contain our post.
	posts := dlist(env)
	found := false
	for _, p := range posts {
		if pid, _ := p["id"].(string); pid == newPostID {
			found = true
			if p["title"] == nil {
				t.Error("post entry missing title field")
			}
			break
		}
	}
	if !found {
		t.Error("newly created post not found in list")
	}
}

// ── 7. GET /api/v1/moderation/cases/:id ─────────────────────────────────────

func TestExternal_ModerationGetCase(t *testing.T) {
	e := setupEnv(t)
	mod := loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234")

	// Nonexistent case → 404, clean error envelope, not 500.
	resp, env := call(mod, "GET",
		e.BaseURL+"/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000", "")
	if resp.StatusCode != 404 {
		t.Errorf("nonexistent case: expected 404, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("nonexistent case: expected success=false")
	}
	if env.Error == nil || env.Error.Code == "" {
		t.Error("nonexistent case: expected error envelope with code")
	}

	// Bad UUID → 404 / 4xx, not 500.
	resp, env = call(mod, "GET", e.BaseURL+"/api/v1/moderation/cases/not-a-uuid", "")
	if resp.StatusCode == 500 {
		t.Error("bad uuid: should not 500")
	}
	if env.Success {
		t.Error("bad uuid: expected success=false")
	}

	// RBAC: staff lacks moderation permission.
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	resp, _ = call(staff, "GET",
		e.BaseURL+"/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000", "")
	if resp.StatusCode != 403 {
		t.Errorf("staff moderation: expected 403, got %d", resp.StatusCode)
	}
}

// ── 8. POST /api/v1/moderation/cases/:id/action ─────────────────────────────

func TestExternal_ModerationActionCase(t *testing.T) {
	e := setupEnv(t)
	mod := loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234")

	// Missing body → validation 400.
	resp, env := call(mod, "POST",
		e.BaseURL+"/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000/action",
		"")
	if resp.StatusCode != 400 {
		t.Errorf("no body: expected 400, got %d", resp.StatusCode)
	}
	if env.Success {
		t.Error("no body: expected success=false")
	}

	// Nonexistent case + valid body → endpoint reachable, graceful error, not 500.
	resp, env = call(mod, "POST",
		e.BaseURL+"/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000/action",
		`{"action_type":"dismiss","details":"test dismissal"}`)
	if resp.StatusCode == 500 {
		t.Fatalf("nonexistent case action: should not 500")
	}
	if env.Success {
		t.Error("nonexistent case: expected success=false")
	}

	// RBAC: member → 403.
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "POST",
		e.BaseURL+"/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000/action",
		`{"action_type":"dismiss"}`)
	if resp.StatusCode != 403 {
		t.Errorf("member action: expected 403, got %d", resp.StatusCode)
	}
}

// ── 9 & 10. Moderation bans: apply + revoke ────────────────────────────────

func TestExternal_ModerationBanAndRevoke(t *testing.T) {
	e := setupEnv(t)
	mod := loginAs(t, e.BaseURL, "mod1", "Seed@Pass1234")

	// Register a fresh user (so we get a known user_id).
	_, targetID := registerAndLogin(t, e.BaseURL, "ban-target")
	if targetID == "" {
		t.Fatal("could not obtain target user id")
	}

	// 9. Apply ban — happy path.
	resp, env := call(mod, "POST", e.BaseURL+"/api/v1/moderation/bans",
		fmt.Sprintf(`{"user_id":"%s","ban_type":"posting","is_permanent":false,"duration_days":7,"reason":"external-test ban"}`,
			targetID))
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("apply ban: expected 200/201, got %d body=%+v", resp.StatusCode, env.Error)
	}
	if !env.Success {
		t.Error("apply ban: expected success=true")
	}
	banID := ds(env, "id")
	if banID == "" {
		t.Fatal("apply ban: no returned ban id")
	}
	if banType, _ := dmap(env)["ban_type"].(string); banType != "posting" {
		t.Errorf("apply ban: ban_type=%q, want posting", banType)
	}

	// Missing body → validation 400.
	resp, env = call(mod, "POST", e.BaseURL+"/api/v1/moderation/bans", "")
	if resp.StatusCode != 400 {
		t.Errorf("no body: expected 400, got %d", resp.StatusCode)
	}

	// 10. Revoke the ban we just applied.
	resp, env = call(mod, "POST",
		e.BaseURL+"/api/v1/moderation/bans/"+banID+"/revoke", "")
	if resp.StatusCode != 200 {
		t.Fatalf("revoke ban: expected 200, got %d body=%+v", resp.StatusCode, env.Error)
	}
	if !env.Success {
		t.Error("revoke ban: expected success=true")
	}

	// Revoke nonexistent → endpoint is reachable and does not 500. (The
	// service treats revoke as idempotent and returns 200; we only assert
	// that the endpoint is alive and returns a well-formed envelope.)
	resp, _ = call(mod, "POST",
		e.BaseURL+"/api/v1/moderation/bans/00000000-0000-0000-0000-000000000000/revoke", "")
	if resp.StatusCode == 500 {
		t.Error("revoke nonexistent: should not 500")
	}

	// RBAC: member → 403 on apply.
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/moderation/bans",
		fmt.Sprintf(`{"user_id":"%s","ban_type":"posting","is_permanent":true,"reason":"x"}`, targetID))
	if resp.StatusCode != 403 {
		t.Errorf("member apply ban: expected 403, got %d", resp.StatusCode)
	}

	// RBAC: member → 403 on revoke.
	resp, _ = call(member, "POST",
		e.BaseURL+"/api/v1/moderation/bans/"+banID+"/revoke", "")
	if resp.StatusCode != 403 {
		t.Errorf("member revoke: expected 403, got %d", resp.StatusCode)
	}
}

// ── 11. POST /api/v1/tickets/:id/assign ─────────────────────────────────────

func TestExternal_TicketAssign(t *testing.T) {
	e := setupEnv(t)
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Staff's own user id (assignee) — read from GET /me.
	_, meEnv := call(staff, "GET", e.BaseURL+"/api/v1/users/me", "")
	staffID := ds(meEnv, "id")
	if staffID == "" {
		t.Fatal("could not obtain staff user id")
	}

	// Create a ticket (admin creates so staff can be assigned).
	resp, env := call(admin, "POST", e.BaseURL+"/api/v1/tickets",
		`{"ticket_type":"delivery_exception","title":"External Assign","description":"desc","priority":"medium"}`)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create ticket: %d", resp.StatusCode)
	}
	tid := ds(env, "id")
	if tid == "" {
		t.Fatal("no ticket id")
	}

	// Happy path: assign to staff.
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/assign",
		fmt.Sprintf(`{"assigned_to":"%s"}`, staffID))
	if resp.StatusCode != 200 {
		t.Fatalf("assign: expected 200, got %d body=%+v", resp.StatusCode, env.Error)
	}
	if !env.Success {
		t.Error("assign: expected success=true")
	}

	// Readback: assigned ticket should show an assignee.
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/tickets/"+tid, "")
	if resp.StatusCode != 200 {
		t.Fatalf("readback: %d", resp.StatusCode)
	}
	// Ticket body is expected to contain assigned_to — shape check.
	body, _ := json.Marshal(env.Data)
	if !strings.Contains(string(body), staffID) {
		t.Errorf("ticket readback should mention staffID=%s; body=%s", staffID, string(body))
	}

	// Missing body → 400.
	resp, env = call(admin, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/assign", "")
	if resp.StatusCode != 400 {
		t.Errorf("no body: expected 400, got %d", resp.StatusCode)
	}

	// RBAC: member → 403.
	member := loginAs(t, e.BaseURL, "member1", "Seed@Pass1234")
	resp, _ = call(member, "POST", e.BaseURL+"/api/v1/tickets/"+tid+"/assign",
		fmt.Sprintf(`{"assigned_to":"%s"}`, staffID))
	if resp.StatusCode != 403 {
		t.Errorf("member assign: expected 403, got %d", resp.StatusCode)
	}
}

// ── 12. POST /api/v1/imports/:id/validate ───────────────────────────────────

func TestExternal_ImportValidate(t *testing.T) {
	e := setupEnv(t)
	admin := loginAs(t, e.BaseURL, "admin", "Seed@Pass1234")

	// Upload a CSV import.
	resp, env := uploadFile(t, admin, e.BaseURL+"/api/v1/imports",
		"validate.csv", "name,email\nBob,b@c.com\n", "general")
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("upload: %d", resp.StatusCode)
	}
	importID := ds(env, "id")
	if importID == "" {
		t.Fatal("no import id")
	}

	// Validate the import — happy path.
	resp, env = call(admin, "POST",
		e.BaseURL+"/api/v1/imports/"+importID+"/validate", "")
	if resp.StatusCode != 200 {
		t.Fatalf("validate: expected 200, got %d body=%+v", resp.StatusCode, env.Error)
	}
	if !env.Success {
		t.Error("validate: expected success=true")
	}

	// Readback the import — status should advance past "uploaded".
	resp, env = call(admin, "GET", e.BaseURL+"/api/v1/imports/"+importID, "")
	if resp.StatusCode != 200 {
		t.Fatalf("readback: %d", resp.StatusCode)
	}
	if s := ds(env, "status"); s == "uploaded" || s == "" {
		t.Errorf("after validate, status=%q; expected != 'uploaded'", s)
	}

	// Validate a nonexistent import → not 500; success=false.
	resp, env = call(admin, "POST",
		e.BaseURL+"/api/v1/imports/00000000-0000-0000-0000-000000000000/validate", "")
	if resp.StatusCode == 500 {
		t.Error("validate nonexistent: should not 500")
	}
	if env.Success {
		t.Error("validate nonexistent: expected success=false")
	}

	// RBAC: staff → 403.
	staff := loginAs(t, e.BaseURL, "staff1", "Seed@Pass1234")
	resp, _ = call(staff, "POST",
		e.BaseURL+"/api/v1/imports/"+importID+"/validate", "")
	if resp.StatusCode != 403 {
		t.Errorf("staff validate: expected 403, got %d", resp.StatusCode)
	}
}
