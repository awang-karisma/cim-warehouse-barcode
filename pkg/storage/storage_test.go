package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := storage.New(storage.Options{SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

func TestMigrate(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Verify table stats include roles and role_permissions
	stats, err := s.GetTableStats(ctx)
	if err != nil {
		t.Fatalf("GetTableStats failed: %v", err)
	}
	if _, ok := stats["roles"]; !ok {
		t.Errorf("expected stats to include 'roles'")
	}
	if _, ok := stats["role_permissions"]; !ok {
		t.Errorf("expected stats to include 'role_permissions'")
	}
	if stats["roles"].RowCount < 4 {
		t.Errorf("expected at least 4 seeded roles, got %d", stats["roles"].RowCount)
	}

	// Verify all system roles seeded
	for _, name := range []string{"guest", "user", "admin", "superadmin"} {
		role, err := s.GetRoleByName(ctx, name)
		if err != nil {
			t.Fatalf("system role %s not found: %v", name, err)
		}
		if !role.IsSystem {
			t.Errorf("role %s expected IsSystem = true", name)
		}
	}

	// Verify user permissions
	userPerms, err := s.GetPermissionsByRoleName(ctx, "user")
	if err != nil {
		t.Fatalf("failed to get user permissions: %v", err)
	}
	expectedUserPerms := []string{
		string(model.PermAccurateCreate),
		string(model.PermAccurateDelete),
		string(model.PermAccurateRead),
		string(model.PermAccurateUpdate),
		string(model.PermDesignRead),
		string(model.PermPresetRead),
		string(model.PermSystemRead),
	}
	if len(userPerms) != len(expectedUserPerms) {
		t.Errorf("expected %d user permissions, got %d: %v", len(expectedUserPerms), len(userPerms), userPerms)
	}

	// Verify admin has all permissions
	adminPerms, err := s.GetPermissionsByRoleName(ctx, "admin")
	if err != nil {
		t.Fatalf("failed to get admin permissions: %v", err)
	}
	if len(adminPerms) != len(model.AllPermissions) {
		t.Errorf("expected admin to have %d permissions, got %d", len(model.AllPermissions), len(adminPerms))
	}

	// Verify superadmin and guest have 0 permissions stored
	guestPerms, err := s.GetPermissionsByRoleName(ctx, "guest")
	if err != nil {
		t.Fatalf("failed to get guest permissions: %v", err)
	}
	if len(guestPerms) != 0 {
		t.Errorf("expected guest to have 0 permissions, got %d", len(guestPerms))
	}
}

func TestRoleCRUD(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// 1. Create custom role
	customRole, err := s.CreateRole(ctx, "operator", "Warehouse floor operator")
	if err != nil {
		t.Fatalf("failed to create custom role: %v", err)
	}
	if customRole.Name != "operator" || customRole.IsSystem {
		t.Errorf("unexpected custom role properties: %+v", customRole)
	}

	// Reject duplicate creation
	_, err = s.CreateRole(ctx, "operator", "duplicate")
	if err == nil {
		t.Errorf("expected error creating duplicate role, got nil")
	}

	// Reject system role name creation
	_, err = s.CreateRole(ctx, "admin", "fake admin")
	if err == nil {
		t.Errorf("expected error creating role with system name, got nil")
	}

	// 2. Set and Get permissions
	perms := []string{string(model.PermDesignRead), string(model.PermPresetRead)}
	if err := s.SetRolePermissions(ctx, customRole.ID, perms); err != nil {
		t.Fatalf("failed to set role permissions: %v", err)
	}

	gotPerms, err := s.GetRolePermissions(ctx, customRole.ID)
	if err != nil {
		t.Fatalf("failed to get role permissions: %v", err)
	}
	if len(gotPerms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(gotPerms))
	}

	// Reject invalid permission
	err = s.SetRolePermissions(ctx, customRole.ID, []string{"invalid:perm"})
	if !errors.Is(err, storage.ErrInvalidPermission) {
		t.Errorf("expected ErrInvalidPermission, got %v", err)
	}

	// Reject setting permissions on guest or superadmin
	guestRole, _ := s.GetRoleByName(ctx, "guest")
	if err := s.SetRolePermissions(ctx, guestRole.ID, perms); err == nil {
		t.Errorf("expected error setting permissions on guest role, got nil")
	}
	superRole, _ := s.GetRoleByName(ctx, "superadmin")
	if err := s.SetRolePermissions(ctx, superRole.ID, perms); err == nil {
		t.Errorf("expected error setting permissions on superadmin role, got nil")
	}

	// 3. Update role
	// System role: cannot rename
	adminRole, _ := s.GetRoleByName(ctx, "admin")
	err = s.UpdateRole(ctx, adminRole.ID, "renamed_admin", "New desc")
	if !errors.Is(err, storage.ErrSystemRole) {
		t.Errorf("expected ErrSystemRole when renaming system role, got %v", err)
	}
	// System role: can update description
	if err := s.UpdateRole(ctx, adminRole.ID, "admin", "Updated admin desc"); err != nil {
		t.Fatalf("failed to update system role description: %v", err)
	}

	// Custom role: can rename
	if err := s.UpdateRole(ctx, customRole.ID, "senior_operator", "Senior operator"); err != nil {
		t.Fatalf("failed to update custom role: %v", err)
	}

	// 4. Delete role guardrails
	// System role cannot be deleted
	if err := s.DeleteRole(ctx, adminRole.ID); !errors.Is(err, storage.ErrSystemRole) {
		t.Errorf("expected ErrSystemRole deleting system role, got %v", err)
	}

	// Assign user to custom role
	_, err = s.UpsertUser(ctx, "user1@example.com", "User One", "")
	if err != nil {
		t.Fatalf("failed to upsert user: %v", err)
	}
	if err := s.SetUserRole(ctx, "user1@example.com", "senior_operator"); err != nil {
		t.Fatalf("failed to set user role: %v", err)
	}

	// Cannot delete role in use
	if err := s.DeleteRole(ctx, customRole.ID); !errors.Is(err, storage.ErrRoleInUse) {
		t.Errorf("expected ErrRoleInUse when deleting assigned role, got %v", err)
	}

	// Reassign user back to guest
	if err := s.SetUserRole(ctx, "user1@example.com", "guest"); err != nil {
		t.Fatalf("failed to reassign user: %v", err)
	}

	// Now delete custom role succeeds
	if err := s.DeleteRole(ctx, customRole.ID); err != nil {
		t.Fatalf("failed to delete unused custom role: %v", err)
	}

	// Confirm role is gone
	_, err = s.GetRoleByID(ctx, customRole.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted role, got %v", err)
	}
}

func TestSetUserRole_Validation(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.UpsertUser(ctx, "tester@example.com", "Tester", "")
	if err != nil {
		t.Fatalf("failed to upsert user: %v", err)
	}

	// Reject assigning superadmin role
	err = s.SetUserRole(ctx, "tester@example.com", "superadmin")
	if err == nil {
		t.Errorf("expected error assigning superadmin role, got nil")
	}

	// Reject assigning non-existent role
	err = s.SetUserRole(ctx, "tester@example.com", "non_existent_role")
	if err == nil {
		t.Errorf("expected error assigning non-existent role, got nil")
	}

	// Accept assigning valid system role
	if err := s.SetUserRole(ctx, "tester@example.com", "admin"); err != nil {
		t.Errorf("expected success assigning admin role, got %v", err)
	}
}
