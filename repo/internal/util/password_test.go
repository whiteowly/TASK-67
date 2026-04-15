package util

import (
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "SecurePass123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == password {
		t.Error("hash should not equal plaintext")
	}

	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	if CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	token1, hash1, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}

	if token1 == "" {
		t.Error("token should not be empty")
	}
	if hash1 == "" {
		t.Error("hash should not be empty")
	}
	if token1 == hash1 {
		t.Error("token and hash should differ")
	}

	// Verify hash is deterministic
	if HashToken(token1) != hash1 {
		t.Error("HashToken should reproduce the same hash")
	}

	// Different tokens
	token2, hash2, _ := GenerateSessionToken()
	if token1 == token2 {
		t.Error("tokens should be unique")
	}
	if hash1 == hash2 {
		t.Error("hashes should be unique")
	}
}

func TestHashToken(t *testing.T) {
	h1 := HashToken("test-token")
	h2 := HashToken("test-token")
	h3 := HashToken("different-token")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}
