package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")

	const userID int64 = 42

	token, err := GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken() unexpected error: %v", err)
	}

	gotUserID, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() unexpected error: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("ParseToken() = %d, want %d", gotUserID, userID)
	}
}

func TestGenerateToken_FallsBackToRandomSecretWithoutEnv(t *testing.T) {
	token, err := GenerateToken(1)
	if err != nil {
		t.Fatalf("GenerateToken() unexpected error: %v", err)
	}

	if _, err := ParseToken(token); err != nil {
		t.Fatalf("ParseToken() unexpected error: %v", err)
	}
}

func TestParseTokenExpired(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")

	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	key, err := secretKey()
	if err != nil {
		t.Fatalf("secretKey() unexpected error: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() unexpected error: %v", err)
	}

	if _, err := ParseToken(signed); err == nil {
		t.Error("ParseToken() expected error for expired token, got nil")
	}
}

func TestParseTokenMalformed(t *testing.T) {
	if _, err := ParseToken("not-a-token"); err == nil {
		t.Error("ParseToken() expected error for malformed token, got nil")
	}
}
