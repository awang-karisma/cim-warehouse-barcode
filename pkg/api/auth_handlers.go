package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	"github.com/gofiber/fiber/v3"
)

type googleCallbackReq struct {
	Credential string `json:"credential"`
}

// handleAuthConfig returns public authentication configuration (Google Client ID and status).
func (s *Server) handleAuthConfig(c fiber.Ctx) error {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		return c.JSON(fiber.Map{
			"auth_enabled": false,
		})
	}
	return c.JSON(fiber.Map{
		"auth_enabled":     true,
		"google_client_id": s.appConfig.GoogleClientID,
	})
}

// handleGoogleCallback verifies the Google ID token and issues a session cookie.
func (s *Server) handleGoogleCallback(c fiber.Ctx) error {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "authentication is not enabled"})
	}

	var req googleCallbackReq
	if err := json.Unmarshal(c.Body(), &req); err != nil || strings.TrimSpace(req.Credential) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body: credential is required"})
	}

	ctx := c.Context()
	email, name, picture, err := auth.VerifyGoogleIDToken(ctx, req.Credential, s.appConfig.GoogleClientID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": fmt.Sprintf("invalid google credential: %v", err)})
	}

	user, err := s.store.UpsertUser(ctx, email, name, picture)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to store user: %v", err)})
	}

	var superadminEmails []string
	if s.appConfig.SuperadminEmails != "" {
		superadminEmails = strings.Split(s.appConfig.SuperadminEmails, ",")
	}

	token, err := auth.GenerateToken(s.appConfig.JWTSecret, user, superadminEmails)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to generate session token: %v", err)})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "warehouse_session",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax",
		MaxAge:   86400,
	})

	role := user.Role
	if s.appConfig.IsSuperadmin(user.Email) {
		role = string(model.RoleSuperadmin)
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"email":   user.Email,
			"name":    user.Name,
			"picture": user.Picture,
			"role":    role,
			"locale":  user.Locale,
		},
	})
}

// handleAuthMe returns the current authenticated user from session cookie claims.
func (s *Server) handleAuthMe(c fiber.Ctx) error {
	if s.appConfig == nil || s.appConfig.GoogleClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "authentication is not enabled"})
	}

	cookie := c.Cookies("warehouse_session")
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	claims, err := auth.ValidateToken(s.appConfig.JWTSecret, cookie)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	refreshedClaims, found, err := s.resolveCurrentUser(c.Context(), claims)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to resolve user: %v", err)})
	}
	if !found {
		clearSessionCookie(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"email":   refreshedClaims.Email,
			"name":    refreshedClaims.Name,
			"picture": refreshedClaims.Picture,
			"role":    refreshedClaims.Role,
			"locale":  refreshedClaims.Locale,
		},
	})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(c fiber.Ctx) error {
	clearSessionCookie(c)
	return c.JSON(fiber.Map{"status": string(ResultOK)})
}

// handleListUsers returns all users in the system.
func (s *Server) handleListUsers(c fiber.Ctx) error {
	if err := s.requireRole(c, string(model.RoleAdmin)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	users, err := s.store.GetUsers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for i := range users {
		if s.appConfig != nil && s.appConfig.IsSuperadmin(users[i].Email) {
			users[i].Role = string(model.RoleSuperadmin)
		}
	}

	return c.JSON(fiber.Map{
		"users": users,
	})
}

type setUserRoleReq struct {
	Role string `json:"role"`
}

// handleSetUserRole changes a user's role to 'user' or 'admin'.
func (s *Server) handleSetUserRole(c fiber.Ctx) error {
	if err := s.requireRole(c, string(model.RoleAdmin)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	targetEmail := c.Params("email")
	if unescaped, err := url.PathUnescape(targetEmail); err == nil && unescaped != "" {
		targetEmail = unescaped
	}
	targetEmail = strings.TrimSpace(targetEmail)
	if targetEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	if s.appConfig != nil && s.appConfig.IsSuperadmin(targetEmail) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot change role of superadmin"})
	}

	targetUser, err := s.store.GetUserByEmail(c.Context(), targetEmail)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if targetUser == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if s.appConfig != nil && s.appConfig.IsSuperadmin(targetUser.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot change role of superadmin"})
	}

	var req setUserRoleReq
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if model.UserRole(req.Role) != model.RoleUser && model.UserRole(req.Role) != model.RoleAdmin {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be 'user' or 'admin'"})
	}

	isCallerSuperadmin := false
	if claims, ok := c.Locals("user").(*auth.Claims); ok && claims != nil {
		if model.UserRole(claims.Role) == model.RoleSuperadmin || (s.appConfig != nil && s.appConfig.IsSuperadmin(claims.Email)) {
			isCallerSuperadmin = true
		}
	}

	if !isCallerSuperadmin {
		// Admin requests are allowed ONLY for the exact transition guest -> user
		if model.UserRole(targetUser.Role) != model.RoleGuest || model.UserRole(req.Role) != model.RoleUser {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient permissions"})
		}
	}

	if err := s.store.SetUserRole(c.Context(), targetUser.Email, req.Role); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": string(ResultOK)})
}

// handleDeleteUser deletes a user by email.
func (s *Server) handleDeleteUser(c fiber.Ctx) error {
	if err := s.requireRole(c, string(model.RoleAdmin)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	targetEmail := c.Params("email")
	if unescaped, err := url.PathUnescape(targetEmail); err == nil && unescaped != "" {
		targetEmail = unescaped
	}
	targetEmail = strings.TrimSpace(targetEmail)
	if targetEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	if s.appConfig != nil && s.appConfig.IsSuperadmin(targetEmail) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete superadmin"})
	}

	if claims, ok := c.Locals("user").(*auth.Claims); ok && claims != nil {
		if strings.EqualFold(claims.Email, targetEmail) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete self"})
		}
	}

	targetUser, err := s.store.GetUserByEmail(c.Context(), targetEmail)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if targetUser == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if s.appConfig != nil && s.appConfig.IsSuperadmin(targetUser.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete superadmin"})
	}

	isCallerSuperadmin := false
	if claims, ok := c.Locals("user").(*auth.Claims); ok && claims != nil {
		if model.UserRole(claims.Role) == model.RoleSuperadmin || (s.appConfig != nil && s.appConfig.IsSuperadmin(claims.Email)) {
			isCallerSuperadmin = true
		}
	}

	if !isCallerSuperadmin {
		// Admin can only delete targets whose current DB role is "guest" or "user"
		if model.UserRole(targetUser.Role) != model.RoleGuest && model.UserRole(targetUser.Role) != model.RoleUser {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient permissions"})
		}
	}

	if err := s.store.DeleteUser(c.Context(), targetUser.Email); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": string(ResultOK)})
}
