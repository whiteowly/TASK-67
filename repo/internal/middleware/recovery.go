package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/campusrec/campusrec/internal/response"
	"github.com/gin-gonic/gin"
)

// isProduction returns true when GIN_MODE is "release".
func isProduction() bool {
	return strings.ToLower(os.Getenv("GIN_MODE")) == "release"
}

// Recovery recovers from panics and returns a 500 error.
// In production mode, stack traces are not logged to prevent information leakage.
// Structured log output includes request_id, path, method, and error class.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				reqID := GetRequestID(c)

				// Structured log entry
				logEntry := map[string]interface{}{
					"level":       "error",
					"component":   "recovery",
					"action":      "panic_recovery",
					"request_id":  reqID.String(),
					"method":      c.Request.Method,
					"path":        c.Request.URL.Path,
					"error_class": "panic",
					"error":       formatPanicError(err),
				}

				// Only include stack trace in non-production environments
				if !isProduction() {
					logEntry["stack"] = string(debug.Stack())
				}

				logJSON, _ := json.Marshal(logEntry)
				log.Printf("%s", logJSON)

				// Always return a generic error to the client — never leak internals
				response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// formatPanicError safely converts a panic value to a string error class
// without exposing sensitive details.
func formatPanicError(err interface{}) string {
	switch v := err.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return "unknown panic"
	}
}
