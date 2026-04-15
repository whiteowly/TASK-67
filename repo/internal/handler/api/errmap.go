package api

import (
	"errors"
	"strings"

	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

// mapServiceError maps domain errors from the service layer to appropriate
// HTTP responses. Returns true if the error was handled, false if caller
// should fall through to generic error handling.
func mapServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrForbidden) {
		response.Forbidden(c, safeDomainMessage(err, "access denied"))
		return true
	}
	if errors.Is(err, service.ErrNotFound) {
		response.NotFound(c, safeDomainMessage(err, "resource not found"))
		return true
	}
	return false
}

// safeDomainMessage extracts the domain prefix from a wrapped error (e.g.
// "order not found: not found" -> "order not found") and falls back to a
// generic message if the error string looks internal.
func safeDomainMessage(err error, fallback string) string {
	msg := err.Error()
	// Domain errors from service layer follow "descriptive prefix: sentinel"
	// pattern. Strip the sentinel suffix for client display.
	for _, sentinel := range []string{": not found", ": forbidden"} {
		if idx := strings.Index(msg, sentinel); idx > 0 {
			return msg[:idx]
		}
	}
	// If the message looks like it could contain internal details, use fallback
	for _, suspect := range []string{"pgx", "sql", "runtime", "goroutine"} {
		if strings.Contains(strings.ToLower(msg), suspect) {
			return fallback
		}
	}
	return msg
}

// handleServiceError maps a service error to the appropriate HTTP response.
// If the error is a domain error (forbidden/not-found), it returns the right
// status. Otherwise it returns the given fallback status with safe message.
func handleServiceError(c *gin.Context, err error, fallbackStatus int, fallbackCode string) {
	if mapServiceError(c, err) {
		return
	}
	response.Error(c, fallbackStatus, fallbackCode, safeErrorMessage(err))
}

// safeErrorMessage returns the error message if it looks like a safe domain
// message (no DB internals, stack traces, etc.), or a generic fallback.
func safeErrorMessage(err error) string {
	msg := err.Error()
	for _, suspect := range []string{"pgx", "sql:", "SQLSTATE", "runtime.", "goroutine", "panic", "connection refused", "broken pipe"} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(suspect)) {
			return "operation failed"
		}
	}
	return msg
}
