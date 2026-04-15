package middleware

import (
	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	ContextUserKey    = "auth_user"
	ContextRolesKey   = "auth_roles"
	ContextSessionKey = "auth_session"
	SessionCookieName = "session_token"
)

// AuthSession extracts and validates the session cookie.
// It populates context but does NOT abort — use RequireAuth for that.
func AuthSession(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil || token == "" {
			c.Next()
			return
		}

		sess, user, roles, err := authSvc.ValidateSession(c.Request.Context(), token)
		if err != nil || sess == nil || user == nil {
			c.Next()
			return
		}

		c.Set(ContextSessionKey, sess)
		c.Set(ContextUserKey, user)
		c.Set(ContextRolesKey, roles)
		c.Next()
	}
}

// GetAuthUser returns the authenticated user from context, or nil.
func GetAuthUser(c *gin.Context) *model.User {
	if v, ok := c.Get(ContextUserKey); ok {
		if user, ok := v.(*model.User); ok {
			return user
		}
	}
	return nil
}

func GetAuthUserID(c *gin.Context) uuid.UUID {
	if u := GetAuthUser(c); u != nil {
		return u.ID
	}
	return uuid.Nil
}

func GetAuthRoles(c *gin.Context) []string {
	if v, ok := c.Get(ContextRolesKey); ok {
		if roles, ok := v.([]string); ok {
			return roles
		}
	}
	return nil
}

func GetAuthSession(c *gin.Context) *model.AuthSession {
	if v, ok := c.Get(ContextSessionKey); ok {
		if sess, ok := v.(*model.AuthSession); ok {
			return sess
		}
	}
	return nil
}
