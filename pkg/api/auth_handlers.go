package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
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

	effectiveRole := user.Role
	if s.appConfig.IsSuperadmin(user.Email) {
		effectiveRole = string(model.RoleSuperadmin)
	}

	var userPerms []string
	if effectiveRole != string(model.RoleSuperadmin) {
		userPerms, _ = s.resolvePermissions(ctx, effectiveRole)
	}

	token, err := auth.GenerateToken(s.appConfig.JWTSecret, user, superadminEmails, userPerms)
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

	respPerms := userPerms
	if effectiveRole == string(model.RoleSuperadmin) {
		respPerms = []string{"*"}
	}
	if respPerms == nil {
		respPerms = []string{}
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"email":       user.Email,
			"name":        user.Name,
			"picture":     user.Picture,
			"role":        effectiveRole,
			"locale":      user.Locale,
			"permissions": respPerms,
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

	respPerms := refreshedClaims.Permissions
	if model.UserRole(refreshedClaims.Role) == model.RoleSuperadmin || (s.appConfig != nil && s.appConfig.IsSuperadmin(refreshedClaims.Email)) {
		respPerms = []string{"*"}
	}
	if respPerms == nil {
		respPerms = []string{}
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"email":       refreshedClaims.Email,
			"name":        refreshedClaims.Name,
			"picture":     refreshedClaims.Picture,
			"role":        refreshedClaims.Role,
			"locale":      refreshedClaims.Locale,
			"permissions": respPerms,
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
	if err := s.requirePermission(c, string(model.PermUserRead)); err != nil {
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
	hasUpdate := s.requirePermission(c, string(model.PermUserUpdate)) == nil
	hasCreate := false
	if !hasUpdate {
		if err := s.requirePermission(c, string(model.PermUserCreate)); err != nil {
			return err
		}
		hasCreate = true
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
	req.Role = strings.TrimSpace(req.Role)
	if req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role is required"})
	}
	if strings.EqualFold(req.Role, string(model.RoleSuperadmin)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot assign superadmin role"})
	}

	exists, err := s.store.RoleExists(c.Context(), req.Role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if !exists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("role %q does not exist", req.Role)})
	}

	// Callers with only user:create (not user:update) are restricted to guest -> user approval
	if !hasUpdate && hasCreate {
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
	if err := s.requirePermission(c, string(model.PermUserDelete)); err != nil {
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

	// user:delete permission allows deleting any non-superadmin user
	if err := s.store.DeleteUser(c.Context(), targetUser.Email); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": string(ResultOK)})
}

// Role management handlers

func (s *Server) handleListRoles(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleRead)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	roles, err := s.store.GetRoles(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"roles": roles})
}

type createRoleReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (s *Server) handleCreateRole(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleCreate)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	var req createRoleReq
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role name is required"})
	}
	for _, p := range req.Permissions {
		if !model.ValidPermission(strings.TrimSpace(p)) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid permission %q", p)})
		}
	}

	role, err := s.store.CreateRole(c.Context(), trimmedName, req.Description)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if len(req.Permissions) > 0 {
		if err := s.store.SetRolePermissions(c.Context(), role.ID, req.Permissions); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		role.Permissions = req.Permissions
	}
	return c.Status(fiber.StatusCreated).JSON(role)
}

type updateRoleReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleUpdateRole(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleUpdate)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role id"})
	}

	existing, err := s.store.GetRoleByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req updateRoleReq
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := s.store.UpdateRole(c.Context(), id, req.Name, req.Description); err != nil {
		if errors.Is(err, storage.ErrSystemRole) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if s.permCache != nil && existing != nil {
		s.permCache.Invalidate(existing.Name)
	}

	updated, err := s.store.GetRoleByID(c.Context(), id)
	if err != nil {
		return c.JSON(fiber.Map{"status": string(ResultOK)})
	}
	return c.JSON(updated)
}

func (s *Server) handleDeleteRole(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleDelete)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role id"})
	}

	existing, err := s.store.GetRoleByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := s.store.DeleteRole(c.Context(), id); err != nil {
		if errors.Is(err, storage.ErrSystemRole) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, storage.ErrRoleInUse) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if s.permCache != nil && existing != nil {
		s.permCache.Invalidate(existing.Name)
	}
	return c.JSON(fiber.Map{"status": string(ResultOK)})
}

func (s *Server) handleGetRolePermissions(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleRead)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role id"})
	}

	if _, err := s.store.GetRoleByID(c.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	perms, err := s.store.GetRolePermissions(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"permissions": perms})
}

type setRolePermissionsReq struct {
	Permissions []string `json:"permissions"`
}

func (s *Server) handleSetRolePermissions(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleUpdate)); err != nil {
		return err
	}
	if s.store == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "storage service not available"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role id"})
	}

	existing, err := s.store.GetRoleByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req setRolePermissionsReq
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	for _, p := range req.Permissions {
		if !model.ValidPermission(strings.TrimSpace(p)) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid permission %q", p)})
		}
	}

	if err := s.store.SetRolePermissions(c.Context(), id, req.Permissions); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if s.permCache != nil && existing != nil {
		s.permCache.Invalidate(existing.Name)
	}

	return c.JSON(fiber.Map{"status": string(ResultOK), "permissions": req.Permissions})
}

func (s *Server) handleListPermissions(c fiber.Ctx) error {
	if err := s.requirePermission(c, string(model.PermRoleRead)); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"permissions": model.AllPermissions})
}
