package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

// RequestID injects a unique request ID into the context and response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.New().String()
		}
		reqUUID, err := uuid.Parse(id)
		if err != nil {
			reqUUID = uuid.New()
			id = reqUUID.String()
		}
		c.Set(RequestIDKey, reqUUID)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(RequestIDKey); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}
