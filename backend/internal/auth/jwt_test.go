package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseTokenRejectsNonHS256SigningMethod(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{
		UserID: "user-id",
		Role:   "staff",
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	if _, err := ParseToken(tokenString, secret); err == nil {
		t.Fatal("expected non-HS256 token to be rejected")
	}
}

func TestTokenPreservesCompanyContext(t *testing.T) {
	token, err := GenerateToken("user-id", "superadmin", "test-secret", "company-id")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	claims, err := ParseToken(token, "test-secret")
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.CompanyID != "company-id" {
		t.Fatalf("expected company context, got %q", claims.CompanyID)
	}
}
