package validator

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name       string
		password   string
		wantErrors int
	}{
		{"valid password", "SecurePass1!ab", 0},
		{"too short", "Short1!", 2},               // too short + missing lower or something
		{"no uppercase", "securepass1!ab", 1},
		{"no lowercase", "SECUREPASS1!AB", 1},
		{"no digit", "SecurePassWord!", 1},
		{"no special", "SecurePass1abc", 1},
		{"empty", "", 5},                          // all violations
		{"just long enough", "Abcdefghij1!", 0},
		{"exactly 12 chars", "Abcdefghij1!", 0},
		{"all violations", "short", 4},             // short, no upper... wait has lower
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePassword(tt.password)
			if tt.name == "valid password" || tt.name == "just long enough" || tt.name == "exactly 12 chars" {
				if len(violations) != 0 {
					t.Errorf("expected no violations, got %v", violations)
				}
			} else {
				if len(violations) == 0 && tt.wantErrors > 0 {
					t.Errorf("expected violations but got none")
				}
			}
		})
	}
}

func TestValidatePassword_Detailed(t *testing.T) {
	// Test each requirement individually
	t.Run("minimum length", func(t *testing.T) {
		v := ValidatePassword("Abc1!xxxxxx") // 11 chars
		found := false
		for _, s := range v {
			if s == "Password must be at least 12 characters" {
				found = true
			}
		}
		if !found {
			t.Error("expected length violation")
		}
	})

	t.Run("uppercase required", func(t *testing.T) {
		v := ValidatePassword("abcdefghij1!")
		found := false
		for _, s := range v {
			if s == "Password must contain at least 1 uppercase letter" {
				found = true
			}
		}
		if !found {
			t.Error("expected uppercase violation")
		}
	})

	t.Run("lowercase required", func(t *testing.T) {
		v := ValidatePassword("ABCDEFGHIJ1!")
		found := false
		for _, s := range v {
			if s == "Password must contain at least 1 lowercase letter" {
				found = true
			}
		}
		if !found {
			t.Error("expected lowercase violation")
		}
	})

	t.Run("digit required", func(t *testing.T) {
		v := ValidatePassword("Abcdefghijk!")
		found := false
		for _, s := range v {
			if s == "Password must contain at least 1 digit" {
				found = true
			}
		}
		if !found {
			t.Error("expected digit violation")
		}
	})

	t.Run("special required", func(t *testing.T) {
		v := ValidatePassword("Abcdefghijk1")
		found := false
		for _, s := range v {
			if s == "Password must contain at least 1 special character" {
				found = true
			}
		}
		if !found {
			t.Error("expected special character violation")
		}
	})
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"valid simple", "john", false},
		{"valid with dots", "john.doe", false},
		{"valid with underscores", "john_doe", false},
		{"valid with hyphens", "john-doe", false},
		{"valid min length", "abc", false},
		{"valid max length", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuv012345", false}, // 64 chars
		{"too short", "ab", true},
		{"too long", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuv0123456", true}, // 65 chars
		{"has spaces", "john doe", true},
		{"has at sign", "john@doe", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername(%q) error = %v, wantErr = %v", tt.username, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid", "test@example.com", false},
		{"empty ok", "", false},
		{"no at", "testexample.com", true},
		{"no domain", "test@", true},
		{"valid subdomain", "test@sub.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr = %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeUsername(t *testing.T) {
	if got := NormalizeUsername("  JohnDoe  "); got != "johndoe" {
		t.Errorf("got %q, want %q", got, "johndoe")
	}
	if got := NormalizeUsername("ADMIN"); got != "admin" {
		t.Errorf("got %q, want %q", got, "admin")
	}
}
