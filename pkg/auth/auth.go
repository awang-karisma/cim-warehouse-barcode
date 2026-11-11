// Package auth provides JWT session token generation, validation, and Google ID token verification.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
)

// Claims represents the JWT session claims.
type Claims struct {
	jwt.RegisteredClaims
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Picture     string   `json:"picture"`
	Role        string   `json:"role"`
	Locale      string   `json:"locale,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// ErrMissingJWTSecret identifies token operations attempted without a signing secret.
var ErrMissingJWTSecret = errors.New("jwt secret cannot be empty")

// GenerateToken creates an HS256 JWT session token with 24-hour expiry.
// If the user's email matches any entry in superadminEmails, the role in claims
// is set to "superadmin" regardless of the role stored in the database.
func GenerateToken(secret string, user *model.User, superadminEmails []string, permissions []string) (string, error) {
	if secret == "" {
		return "", ErrMissingJWTSecret
	}
	if user == nil {
		return "", fmt.Errorf("user cannot be nil")
	}

	role := user.Role
	for _, admin := range superadminEmails {
		if strings.EqualFold(strings.TrimSpace(admin), strings.TrimSpace(user.Email)) {
			role = string(model.RoleSuperadmin)
			break
		}
	}

	var perms []string
	if role != string(model.RoleSuperadmin) && len(permissions) > 0 {
		perms = permissions
	}

	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		Email:       user.Email,
		Name:        user.Name,
		Picture:     user.Picture,
		Role:        role,
		Locale:      user.Locale,
		Permissions: perms,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenStr, nil
}

// ValidateToken parses and validates an HS256 JWT session token against the secret.
func ValidateToken(secret string, tokenStr string) (*Claims, error) {
	if secret == "" {
		return nil, ErrMissingJWTSecret
	}
	if strings.TrimSpace(tokenStr) == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// VerifyGoogleIDToken validates a Google ID token string using Google's public keys.
func VerifyGoogleIDToken(ctx context.Context, idTokenStr, clientID string) (email, name, picture string, err error) {
	if strings.TrimSpace(idTokenStr) == "" {
		return "", "", "", fmt.Errorf("id token cannot be empty")
	}
	if strings.TrimSpace(clientID) == "" {
		return "", "", "", fmt.Errorf("client id cannot be empty")
	}

	payload, err := idtoken.Validate(ctx, idTokenStr, clientID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to validate google id token: %w", err)
	}

	email, _ = payload.Claims["email"].(string)
	name, _ = payload.Claims["name"].(string)
	picture, _ = payload.Claims["picture"].(string)

	if strings.TrimSpace(email) == "" {
		return "", "", "", fmt.Errorf("email claim missing in google id token")
	}

	return email, name, picture, nil
}
