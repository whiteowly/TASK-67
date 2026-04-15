// rbac_test.go — branch-complete unit tests for the RBAC middleware.
//
// RequireAuth has two branches: authenticated (next) vs unauthenticated (401).
// RequireRole has three branches: unauthenticated (401), authenticated but
// no role match (403), authenticated with role match (next).
//
// We test using a real gin.Engine so that the middleware → next chain
// executes via the production handler-chain machinery (not a synthetic
// stand-in). A "seed" middleware is registered before the middleware
// under test to populate auth context; a sentinel handler proves whether
// next was reached.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() { gin.SetMode(gin.TestMode) }

// seed installs a stand-in for the real auth middleware: it sets the
// authenticated user / roles on the request context. Pass nil user to
// simulate an unauthenticated request.
func seed(user *model.User, roles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if user != nil {
			c.Set(ContextUserKey, user)
			c.Set(ContextRolesKey, roles)
		}
		c.Next()
	}
}

// runRoute mounts seed → mw → sentinel on a fresh engine, sends a GET /,
// and reports whether the sentinel ran and the response status.
func runRoute(seedMW gin.HandlerFunc, mw gin.HandlerFunc) (nextRan bool, status int) {
	r := gin.New()
	r.Use(seedMW, mw)
	r.GET("/", func(c *gin.Context) {
		nextRan = true
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	return nextRan, w.Code
}

// ── RequireAuth ─────────────────────────────────────────────────────────────

func TestRequireAuth_Unauthenticated_401(t *testing.T) {
	ran, code := runRoute(seed(nil, nil), RequireAuth())
	if ran {
		t.Error("next must not run when user is unauthenticated")
	}
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestRequireAuth_Authenticated_NextRuns(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "u1"}
	ran, code := runRoute(seed(user, []string{model.RoleMember}), RequireAuth())
	if !ran {
		t.Error("next must run when user is authenticated")
	}
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
}

// ── RequireRole ─────────────────────────────────────────────────────────────

func TestRequireRole_Unauthenticated_401(t *testing.T) {
	ran, code := runRoute(seed(nil, nil), RequireRole(model.RoleAdministrator))
	if ran {
		t.Error("next must not run when user is unauthenticated")
	}
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestRequireRole_AuthenticatedButNoMatch_403(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "u1"}
	ran, code := runRoute(
		seed(user, []string{model.RoleMember}),
		RequireRole(model.RoleAdministrator, model.RoleStaff),
	)
	if ran {
		t.Error("next must not run when role doesn't match")
	}
	if code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", code)
	}
}

func TestRequireRole_AuthenticatedAndRoleMatches_NextRuns(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "u1"}
	ran, code := runRoute(
		seed(user, []string{model.RoleStaff}),
		RequireRole(model.RoleAdministrator, model.RoleStaff),
	)
	if !ran {
		t.Error("next must run when at least one role matches")
	}
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
}

// Edge case: user has multiple roles, one of them matches.
func TestRequireRole_MultipleUserRoles_AnyMatchPasses(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "u1"}
	ran, _ := runRoute(
		seed(user, []string{model.RoleMember, model.RoleModerator}),
		RequireRole(model.RoleModerator),
	)
	if !ran {
		t.Error("next must run when any user role matches an allowed role")
	}
}

// Edge case: empty role list on the user — must deny.
func TestRequireRole_EmptyUserRoles_403(t *testing.T) {
	user := &model.User{ID: uuid.New(), Username: "u1"}
	ran, code := runRoute(
		seed(user, []string{}),
		RequireRole(model.RoleAdministrator),
	)
	if ran {
		t.Error("next must not run when user has no roles")
	}
	if code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", code)
	}
}
