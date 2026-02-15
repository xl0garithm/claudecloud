package auth

import (
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateToken(secret, 1, "test@example.com", "session", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("user_id = %d, want 1", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", claims.Email)
	}
	if claims.Purpose != "session" {
		t.Errorf("purpose = %s, want session", claims.Purpose)
	}
}

func TestExpiredToken(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateToken(secret, 1, "test@example.com", "session", -time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = ValidateToken(secret, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestTamperedToken(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateToken(secret, 1, "test@example.com", "session", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = ValidateToken(secret, token+"x")
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestWrongSecret(t *testing.T) {
	token, err := GenerateToken("secret1", 1, "test@example.com", "session", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = ValidateToken("secret2", token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestMagicLinkPurpose(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateToken(secret, 1, "test@example.com", "magic_link", 15*time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Purpose != "magic_link" {
		t.Errorf("purpose = %s, want magic_link", claims.Purpose)
	}
}
