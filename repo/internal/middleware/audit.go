package middleware

import (
	"encoding/json"
	"log"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

// AuditLog logs mutating requests to the audit trail after they complete.
// Uses structured logging format with request_id, action, status, and error class.
func AuditLog(auditSvc *service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		method := c.Request.Method
		reqID := GetRequestID(c)
		duration := time.Since(start)

		// Structured request log for all requests
		logEntry := map[string]interface{}{
			"level":       "info",
			"component":   "http",
			"action":      method + " " + c.FullPath(),
			"request_id":  reqID.String(),
			"method":      method,
			"path":        c.Request.URL.Path,
			"status":      c.Writer.Status(),
			"duration_ms": duration.Milliseconds(),
		}

		status := c.Writer.Status()
		if status >= 400 {
			logEntry["level"] = "warn"
			if status >= 500 {
				logEntry["level"] = "error"
				logEntry["error_class"] = "server_error"
			} else {
				logEntry["error_class"] = "client_error"
			}
		}

		logJSON, _ := json.Marshal(logEntry)
		log.Printf("%s", logJSON)

		// Only persist audit entries for mutating requests that succeeded
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return
		}
		if status >= 500 {
			return
		}

		userID := GetAuthUserID(c)
		ip := c.ClientIP()

		entry := &model.AuditEntry{
			ActorType: "user",
			Action:    method + " " + c.FullPath(),
			Resource:  "http",
			IPAddr:    &ip,
			RequestID: &reqID,
			Metadata: map[string]interface{}{
				"status":      status,
				"duration_ms": duration.Milliseconds(),
				"path":        c.Request.URL.Path,
			},
		}

		if userID != [16]byte{} {
			entry.ActorID = &userID
		}

		auditSvc.Log(c.Request.Context(), entry)
	}
}
