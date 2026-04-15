package middleware

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/response"
	"github.com/gin-gonic/gin"
)

// RequireAuth aborts with 401 if the user is not authenticated.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetAuthUser(c) == nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRole aborts with 403 if the user does not have at least one of the specified roles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetAuthUser(c)
		if user == nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		userRoles := GetAuthRoles(c)
		for _, allowed := range allowedRoles {
			for _, ur := range userRoles {
				if ur == allowed {
					c.Next()
					return
				}
			}
		}

		response.Error(c, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		c.Abort()
	}
}
