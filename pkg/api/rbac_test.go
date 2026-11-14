package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

type testEnv struct {
	server    *api.Server
	store     *storage.Storage
	cfg       *config.Config
	permCache rbac.PermissionCache
}

func setupTestServer(t *testing.T, authEnabled bool) *testEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_api.db")
	store, err := storage.New(storage.Options{SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	cfg := &config.Config{
		Port:             "8080",
		DBPath:           dbPath,
		JWTSecret:        "test-secret-key-12345",
		SuperadminEmails: "super@example.com",
		RBACCacheTTL:     5 * time.Minute,
	}
	if authEnabled {
		cfg.GoogleClientID = "test-client-id.apps.googleusercontent.com"
	}

	permCache := rbac.NewMemoryCache(cfg.RBACCacheTTL)
	srv := api.New(api.Config{
		Store:     store,
		AppConfig: cfg,
		PermCache: permCache,
	})

	return &testEnv{
		server:    srv,
		store:     store,
		cfg:       cfg,
		permCache: permCache,
	}
}

func (e *testEnv) createUserToken(t *testing.T, email, name, role string) string {
	t.Helper()
	ctx := context.Background()
	user, err := e.store.UpsertUser(ctx, email, name, "")
	if err != nil {
		t.Fatalf("failed to upsert user %s: %v", email, err)
	}
	if role != "guest" {
		if err := e.store.SetUserRole(ctx, email, role); err != nil {
			t.Fatalf("failed to set user role %s for %s: %v", role, email, err)
		}
		user.Role = role
	}

	perms, _ := e.store.GetPermissionsByRoleName(ctx, role)
	token, err := auth.GenerateToken(e.cfg.JWTSecret, user, []string{e.cfg.SuperadminEmails}, perms)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func (e *testEnv) makeRequest(method, path string, body []byte, token string) *http.Response {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{
			Name:  "warehouse_session",
			Value: token,
		})
	}

	resp, err := e.server.App().Test(req)
	if err != nil {
		panic(fmt.Sprintf("fiber test request failed: %v", err))
	}
	return resp
}

func TestAuthBypassMode(t *testing.T) {
	env := setupTestServer(t, false)

	// Auth bypass mode (no GOOGLE_CLIENT_ID): all endpoints pass without token
	resp := env.makeRequest(http.MethodGet, "/api/v1/page-designs", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 in auth bypass mode, got %d", resp.StatusCode)
	}

	resp = env.makeRequest(http.MethodGet, "/api/v1/admin/roles", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 in auth bypass mode, got %d", resp.StatusCode)
	}
}

func TestPermissionEnforcement(t *testing.T) {
	env := setupTestServer(t, true)

	// 1. Unauthenticated request to protected endpoint -> 401
	resp := env.makeRequest(http.MethodGet, "/api/v1/page-designs", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthenticated, got %d", resp.StatusCode)
	}

	// 2. Guest user -> 403 (no permissions)
	guestToken := env.createUserToken(t, "guest@example.com", "Guest User", "guest")
	resp = env.makeRequest(http.MethodGet, "/api/v1/page-designs", nil, guestToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for guest, got %d", resp.StatusCode)
	}

	// Guest accessing self-service locale -> 403 (guest blocked from self-service)
	localePayload, _ := json.Marshal(map[string]string{"locale": "en"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/user/locale", localePayload, guestToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for guest updating locale, got %d", resp.StatusCode)
	}

	// 3. User with standard "user" role:
	// Default permissions: design:read, preset:read, accurate:*, system:read
	userToken := env.createUserToken(t, "user@example.com", "Standard User", "user")

	// Has design:read -> 200
	resp = env.makeRequest(http.MethodGet, "/api/v1/page-designs", nil, userToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for user with design:read, got %d", resp.StatusCode)
	}

	// Lacks design:create -> 403
	designPayload, _ := json.Marshal(map[string]any{
		"name":            "My Design",
		"label_width_mm":  50.0,
		"label_height_mm": 30.0,
		"cols":            1,
		"rows":            1,
	})
	resp = env.makeRequest(http.MethodPost, "/api/v1/page-designs", designPayload, userToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for user creating design, got %d", resp.StatusCode)
	}

	// Self-service locale update -> 200
	resp = env.makeRequest(http.MethodPut, "/api/v1/user/locale", localePayload, userToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for user updating locale, got %d", resp.StatusCode)
	}

	// Lacks role:read -> 403
	resp = env.makeRequest(http.MethodGet, "/api/v1/admin/roles", nil, userToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for user accessing admin roles, got %d", resp.StatusCode)
	}

	// 4. Admin role: has all permissions
	adminToken := env.createUserToken(t, "admin@example.com", "Admin User", "admin")

	// Can create page design
	resp = env.makeRequest(http.MethodPost, "/api/v1/page-designs", designPayload, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 200/201 for admin creating design, got %d", resp.StatusCode)
	}

	// Can list admin roles
	resp = env.makeRequest(http.MethodGet, "/api/v1/admin/roles", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for admin listing roles, got %d", resp.StatusCode)
	}

	// 5. Superadmin: bypasses all checks
	superToken := env.createUserToken(t, "super@example.com", "Super Admin", "user") // role in DB is irrelevant
	resp = env.makeRequest(http.MethodPost, "/api/v1/page-designs", designPayload, superToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 200/201 for superadmin creating design, got %d", resp.StatusCode)
	}
	resp = env.makeRequest(http.MethodGet, "/api/v1/admin/roles", nil, superToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for superadmin listing roles, got %d", resp.StatusCode)
	}
}

func TestRoleManagementCRUD(t *testing.T) {
	env := setupTestServer(t, true)
	adminToken := env.createUserToken(t, "admin@example.com", "Admin", "admin")

	// 1. List permissions registry
	resp := env.makeRequest(http.MethodGet, "/api/v1/admin/permissions", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 getting permissions, got %d", resp.StatusCode)
	}
	var permResp struct {
		Permissions []string `json:"permissions"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&permResp)
	if len(permResp.Permissions) != len(model.AllPermissions) {
		t.Errorf("expected %d permissions, got %d", len(model.AllPermissions), len(permResp.Permissions))
	}

	// 2. Create custom role with permissions
	createPayload, _ := json.Marshal(map[string]any{
		"name":        "warehouse_operator",
		"description": "Floor operator with read-only access",
		"permissions": []string{"design:read", "preset:read"},
	})
	resp = env.makeRequest(http.MethodPost, "/api/v1/admin/roles", createPayload, adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating role, got %d", resp.StatusCode)
	}
	var createdRole model.Role
	_ = json.NewDecoder(resp.Body).Decode(&createdRole)
	if createdRole.Name != "warehouse_operator" || len(createdRole.Permissions) != 2 {
		t.Errorf("unexpected created role: %+v", createdRole)
	}

	// 3. Reject invalid permission string on create
	badPermPayload, _ := json.Marshal(map[string]any{
		"name":        "bad_role",
		"description": "Bad",
		"permissions": []string{"invalid:perm"},
	})
	resp = env.makeRequest(http.MethodPost, "/api/v1/admin/roles", badPermPayload, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid permission on create, got %d", resp.StatusCode)
	}

	// 4. Update custom role description and rename
	updatePayload, _ := json.Marshal(map[string]any{
		"name":        "senior_operator",
		"description": "Updated description",
	})
	resp = env.makeRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", createdRole.ID), updatePayload, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 updating custom role, got %d", resp.StatusCode)
	}

	// 5. System role protections
	// Find admin system role
	rolesResp := env.makeRequest(http.MethodGet, "/api/v1/admin/roles", nil, adminToken)
	var listResp struct {
		Roles []model.Role `json:"roles"`
	}
	_ = json.NewDecoder(rolesResp.Body).Decode(&listResp)
	var adminRole model.Role
	for _, r := range listResp.Roles {
		if r.Name == "admin" {
			adminRole = r
			break
		}
	}
	if adminRole.ID == 0 {
		t.Fatal("admin system role not found in list")
	}

	// Attempt rename of system role -> 400
	badRenamePayload, _ := json.Marshal(map[string]any{
		"name":        "super_admin_renamed",
		"description": "Renamed admin",
	})
	resp = env.makeRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", adminRole.ID), badRenamePayload, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 renaming system role, got %d", resp.StatusCode)
	}

	// Attempt delete of system role -> 400
	resp = env.makeRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", adminRole.ID), nil, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 deleting system role, got %d", resp.StatusCode)
	}

	// 6. Set role permissions
	newPermsPayload, _ := json.Marshal(map[string]any{
		"permissions": []string{"design:read", "preset:read", "accurate:read"},
	})
	resp = env.makeRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d/permissions", createdRole.ID), newPermsPayload, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 setting permissions, got %d", resp.StatusCode)
	}

	// Get role permissions
	resp = env.makeRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/roles/%d/permissions", createdRole.ID), nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 getting permissions, got %d", resp.StatusCode)
	}
	var getPermsResp struct {
		Permissions []string `json:"permissions"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&getPermsResp)
	if len(getPermsResp.Permissions) != 3 {
		t.Errorf("expected 3 permissions, got %d: %v", len(getPermsResp.Permissions), getPermsResp.Permissions)
	}

	// 7. Assign user to the custom role and verify permissions
	operatorToken := env.createUserToken(t, "operator@example.com", "Operator", "senior_operator")

	// Operator can read page designs
	resp = env.makeRequest(http.MethodGet, "/api/v1/page-designs", nil, operatorToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for operator reading page designs, got %d", resp.StatusCode)
	}

	// Operator cannot create page designs (403)
	designPayload, _ := json.Marshal(map[string]any{
		"name":            "Operator Design",
		"label_width_mm":  50.0,
		"label_height_mm": 30.0,
		"cols":            1,
		"rows":            1,
	})
	resp = env.makeRequest(http.MethodPost, "/api/v1/page-designs", designPayload, operatorToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for operator creating page design, got %d", resp.StatusCode)
	}

	// 8. Delete role in use -> 409 Conflict
	resp = env.makeRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", createdRole.ID), nil, adminToken)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 deleting assigned role, got %d", resp.StatusCode)
	}

	// Reassign operator to guest
	reassignPayload, _ := json.Marshal(map[string]any{"role": "guest"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/operator@example.com/role", reassignPayload, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 reassigning operator role, got %d", resp.StatusCode)
	}

	// Now delete succeeds
	resp = env.makeRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", createdRole.ID), nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 deleting unused custom role, got %d", resp.StatusCode)
	}
}

func TestAdminUserManagement(t *testing.T) {
	env := setupTestServer(t, true)
	adminToken := env.createUserToken(t, "admin@example.com", "Admin", "admin")

	// Create a target guest user
	_ = env.createUserToken(t, "target@example.com", "Target", "guest")

	// 1. Admin can list users
	resp := env.makeRequest(http.MethodGet, "/api/v1/admin/users", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing users, got %d", resp.StatusCode)
	}

	// 2. Admin can change target user role to "user"
	setRolePayload, _ := json.Marshal(map[string]any{"role": "user"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/target@example.com/role", setRolePayload, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 changing user role, got %d", resp.StatusCode)
	}

	// 3. Admin can change target user role to "admin"
	setAdminPayload, _ := json.Marshal(map[string]any{"role": "admin"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/target@example.com/role", setAdminPayload, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 elevating user to admin, got %d", resp.StatusCode)
	}

	// 4. Reject assigning superadmin role
	setSuperPayload, _ := json.Marshal(map[string]any{"role": "superadmin"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/target@example.com/role", setSuperPayload, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 assigning superadmin role, got %d", resp.StatusCode)
	}

	// 5. Admin cannot target superadmin email
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/super@example.com/role", setRolePayload, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 modifying superadmin, got %d", resp.StatusCode)
	}

	// 6. Admin cannot delete self
	resp = env.makeRequest(http.MethodDelete, "/api/v1/admin/users/admin@example.com", nil, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 deleting self, got %d", resp.StatusCode)
	}

	// 7. Admin cannot delete superadmin
	resp = env.makeRequest(http.MethodDelete, "/api/v1/admin/users/super@example.com", nil, adminToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 deleting superadmin, got %d", resp.StatusCode)
	}

	// 8. Admin can delete other user
	resp = env.makeRequest(http.MethodDelete, "/api/v1/admin/users/target@example.com", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 deleting user, got %d", resp.StatusCode)
	}
}

func TestAuthMePermissions(t *testing.T) {
	env := setupTestServer(t, true)

	// User role gets resolved permissions in /auth/me
	userToken := env.createUserToken(t, "user@example.com", "User", "user")
	resp := env.makeRequest(http.MethodGet, "/auth/me", nil, userToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /auth/me, got %d", resp.StatusCode)
	}
	var userMeResp struct {
		User struct {
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&userMeResp)
	if userMeResp.User.Role != "user" || len(userMeResp.User.Permissions) == 0 {
		t.Errorf("expected user role with non-empty permissions: %+v", userMeResp)
	}

	// Superadmin gets ["*"] wildcard in /auth/me
	superToken := env.createUserToken(t, "super@example.com", "Super", "user")
	resp = env.makeRequest(http.MethodGet, "/auth/me", nil, superToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /auth/me, got %d", resp.StatusCode)
	}
	var superMeResp struct {
		User struct {
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&superMeResp)
	if superMeResp.User.Role != "superadmin" {
		t.Errorf("expected superadmin role, got %s", superMeResp.User.Role)
	}
	if len(superMeResp.User.Permissions) != 1 || superMeResp.User.Permissions[0] != "*" {
		t.Errorf("expected ['*'] permissions for superadmin, got %v", superMeResp.User.Permissions)
	}
}

func TestUserCreateOnlyPermission(t *testing.T) {
	env := setupTestServer(t, true)
	adminToken := env.createUserToken(t, "admin@example.com", "Admin", "admin")

	// Create custom role with user:create and user:read only
	createRolePayload, _ := json.Marshal(map[string]any{
		"name":        "user_approver",
		"description": "Can only approve guest to user",
		"permissions": []string{"user:read", "user:create"},
	})
	resp := env.makeRequest(http.MethodPost, "/api/v1/admin/roles", createRolePayload, adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating role, got %d", resp.StatusCode)
	}

	approverToken := env.createUserToken(t, "approver@example.com", "Approver", "user_approver")
	_ = env.createUserToken(t, "newbie@example.com", "Newbie", "guest")

	// Approver can transition guest -> user
	toUserPayload, _ := json.Marshal(map[string]any{"role": "user"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/newbie@example.com/role", toUserPayload, approverToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for user:create approving guest to user, got %d", resp.StatusCode)
	}

	// Approver CANNOT transition user -> admin (403 forbidden)
	toAdminPayload, _ := json.Marshal(map[string]any{"role": "admin"})
	resp = env.makeRequest(http.MethodPut, "/api/v1/admin/users/newbie@example.com/role", toAdminPayload, approverToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for user:create attempting user to admin elevation, got %d", resp.StatusCode)
	}
}
