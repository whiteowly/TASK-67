package validator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// ValidatePassword checks the password against policy requirements.
// Returns a list of violations, empty if valid.
func ValidatePassword(password string) []string {
	var violations []string

	if len(password) < 12 {
		violations = append(violations, "Password must be at least 12 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		violations = append(violations, "Password must contain at least 1 uppercase letter")
	}
	if !hasLower {
		violations = append(violations, "Password must contain at least 1 lowercase letter")
	}
	if !hasDigit {
		violations = append(violations, "Password must contain at least 1 digit")
	}
	if !hasSpecial {
		violations = append(violations, "Password must contain at least 1 special character")
	}

	return violations
}

// ValidateUsername checks username format.
func ValidateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username must be 3-64 characters, alphanumeric plus ._-")
	}
	return nil
}

// ValidateEmail checks email format (optional field).
func ValidateEmail(email string) error {
	if email == "" {
		return nil
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// NormalizeUsername returns a lowercase, trimmed username.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
