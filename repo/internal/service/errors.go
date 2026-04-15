package service

import (
	"errors"
	"fmt"
)

// Domain error types for service layer.
// Handlers use errors.Is() to map these to HTTP status codes.

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

// Forbidden wraps a message with ErrForbidden so handlers can detect it via errors.Is.
func Forbidden(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrForbidden)
}

// NotFound wraps a message with ErrNotFound.
func NotFound(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrNotFound)
}
