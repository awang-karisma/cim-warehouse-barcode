package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/golang-jwt/jwt/v5"
)

func TestTokenGenerationAndValidation(t *testing.T) {
	secret := "my-secret-key-12345"
	user := &model.User{
		ID:      1,
		Email:   "user@example.com",
		Name:    "Normal User",
		Picture: "https://example.com/pic.png",
		Role:    "user",
	}

	perms := []string{"design:read", "preset:read"}
	tokenStr, err := auth.GenerateToken(secret, user, nil, perms)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	claims, err := auth.ValidateToken(secret, tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, claims.Email)
	}
	if claims.Name != user.Name {
		t.Errorf("expected name %q, got %q", user.Name, claims.Name)
	}
	if claims.Picture != user.Picture {
		t.Errorf("expected picture %q, got %q", user.Picture, claims.Picture)
	}
	if claims.Role != "user" {
		t.Errorf("expected role 'user', got %q", claims.Role)
	}
	if len(claims.Permissions) != 2 || claims.Permissions[0] != "design:read" {
		t.Errorf("expected permissions %v, got %v", perms, claims.Permissions)
	}
}
func TestMissingJWTSecretHasStableIdentity(t *testing.T) {
	user := &model.User{Email: "user@example.com", Role: "user"}
	if _, err := auth.GenerateToken("", user, nil, nil); !errors.Is(err, auth.ErrMissingJWTSecret) {
		t.Fatalf("GenerateToken missing secret should return ErrMissingJWTSecret, got: %v", err)
	}
	if _, err := auth.ValidateToken("", "token"); !errors.Is(err, auth.ErrMissingJWTSecret) {
		t.Fatalf("ValidateToken missing secret should return ErrMissingJWTSecret, got: %v", err)
	}
}

func TestSuperadminElevation(t *testing.T) {
	secret := "my-secret-key-12345"
	user := &model.User{
		ID:    2,
		Email: "Admin@Example.COM",
		Name:  "Admin User",
		Role:  "guest", // DB role is guest, but superadminEmails will elevate
	}

	superadminEmails := []string{"other@example.com", "admin@example.com"}
	tokenStr, err := auth.GenerateToken(secret, user, superadminEmails, []string{"design:read"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := auth.ValidateToken(secret, tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.Role != "superadmin" {
		t.Errorf("expected role 'superadmin', got %q", claims.Role)
	}
	if claims.Permissions != nil {
		t.Errorf("expected nil permissions for superadmin in token, got %v", claims.Permissions)
	}
}
func TestValidateToken_InvalidSignature(t *testing.T) {
	secret := "secret-one"
	user := &model.User{
		Email: "user@example.com",
		Role:  "user",
	}

	tokenStr, err := auth.GenerateToken(secret, user, nil, nil)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = auth.ValidateToken("secret-two", tokenStr)
	if err == nil {
		t.Error("expected error validating with wrong secret, got nil")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := "test-secret"
	claims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired@example.com",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
		Email: "expired@example.com",
		Role:  "user",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = auth.ValidateToken(secret, tokenStr)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	secret := "test-secret"
	_, err := auth.ValidateToken(secret, "not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for malformed token, got nil")
	}
	_, err = auth.ValidateToken(secret, "")
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
}

func TestVerifyGoogleIDToken_EmptyInputs(t *testing.T) {
	ctx := context.Background()
	_, _, _, err := auth.VerifyGoogleIDToken(ctx, "", "client-id")
	if err == nil {
		t.Error("expected error for empty id token")
	}

	_, _, _, err = auth.VerifyGoogleIDToken(ctx, "some-token", "")
	if err == nil {
		t.Error("expected error for empty client id")
	}

	_, _, _, err = auth.VerifyGoogleIDToken(ctx, "invalid-token", "client-id")
	if err == nil {
		t.Error("expected error for invalid id token")
	}
}
