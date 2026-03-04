package auth

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken("testuser", "secret-key", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	secret := "test-secret-key-2026"
	token, _ := GenerateToken("admin", secret, time.Hour)

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("Username = %q, want admin", claims.Username)
	}
	if claims.Subject != "admin" {
		t.Errorf("Subject = %q, want admin", claims.Subject)
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	token, _ := GenerateToken("admin", "secret1", time.Hour)

	_, err := ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Error("ValidateToken() should fail with wrong secret")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// Generate a token that's already expired
	token, _ := GenerateToken("admin", "secret", -time.Hour)

	_, err := ValidateToken(token, "secret")
	if err == nil {
		t.Error("ValidateToken() should fail for expired token")
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	_, err := ValidateToken("not.a.valid.jwt", "secret")
	if err == nil {
		t.Error("ValidateToken() should fail for malformed token")
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	_, err := ValidateToken("", "secret")
	if err == nil {
		t.Error("ValidateToken() should fail for empty token")
	}
}
