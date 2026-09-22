package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
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

func TestParseTokenExpired(t *testing.T) {
	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secretKey())
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
