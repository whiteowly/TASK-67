package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Envelope is the standard API response wrapper.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *ErrorBody  `json:"error"`
	Meta    *Meta       `json:"meta"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	Total     int    `json:"total,omitempty"`
}

func newMeta(c *gin.Context) *Meta {
	reqID, _ := c.Get("request_id")
	rid, ok := reqID.(uuid.UUID)
	if !ok {
		rid = uuid.New()
	}
	return &Meta{
		RequestID: rid.String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Success sends a successful JSON response.
func Success(c *gin.Context, status int, data interface{}) {
	c.JSON(status, Envelope{
		Success: true,
		Data:    data,
		Error:   nil,
		Meta:    newMeta(c),
	})
}

// OK sends a 200 success response.
func OK(c *gin.Context, data interface{}) {
	Success(c, http.StatusOK, data)
}

// Created sends a 201 success response.
func Created(c *gin.Context, data interface{}) {
	Success(c, http.StatusCreated, data)
}

// Paginated sends a successful paginated response.
func Paginated(c *gin.Context, data interface{}, page, perPage, total int) {
	meta := newMeta(c)
	meta.Page = page
	meta.PerPage = perPage
	meta.Total = total
	c.JSON(http.StatusOK, Envelope{
		Success: true,
		Data:    data,
		Error:   nil,
		Meta:    meta,
	})
}

// Error sends an error JSON response.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, Envelope{
		Success: false,
		Data:    nil,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
		},
		Meta: newMeta(c),
	})
}

// ErrorWithDetails sends an error response with additional details.
func ErrorWithDetails(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, Envelope{
		Success: false,
		Data:    nil,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: newMeta(c),
	})
}

// Validation error helpers
func ValidationError(c *gin.Context, details interface{}) {
	ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", details)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

func Conflict(c *gin.Context, code, message string) {
	Error(c, http.StatusConflict, code, message)
}

func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
}
