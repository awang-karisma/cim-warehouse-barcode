package storage_test

import (
	"context"
	"os"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

func postgresStorage(t *testing.T) *storage.Storage {
	t.Helper()
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL is not configured")
	}
	store, err := storage.New(storage.Options{DatabaseURL: url})
	if err != nil {
		t.Fatalf("open PostgreSQL storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPostgresSmoke(t *testing.T) {
	store := postgresStorage(t)
	stats, err := store.GetTableStats(context.Background())
	if err != nil {
		t.Fatalf("GetTableStats: %v", err)
	}
	for _, table := range []string{"users", "accurate_accounts", "page_designs", "label_layout_presets", "roles", "role_permissions"} {
		if _, ok := stats[table]; !ok {
			t.Fatalf("missing %s stat: %#v", table, stats)
		}
	}
}

func TestPostgresCRUDSmoke(t *testing.T) {
	store := postgresStorage(t)
	ctx := context.Background()
	const email = "postgres-smoke@example.test"
	if _, err := store.UpsertUser(ctx, email, "Postgres Smoke", ""); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteUser(ctx, email) })

	presetID, err := store.CreateLabelLayoutPreset(ctx, model.LabelLayoutPreset{Name: "postgres-smoke-preset", LabelLayout: "[]"})
	if err != nil {
		t.Fatalf("CreateLabelLayoutPreset: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteLabelLayoutPreset(ctx, presetID) })
	presets, err := store.GetLabelLayoutPresets(ctx)
	if err != nil {
		t.Fatalf("GetLabelLayoutPresets: %v", err)
	}
	if len(presets) == 0 {
		t.Fatal("expected preset")
	}

	designID, err := store.CreatePageDesign(ctx, model.PageDesign{Name: "postgres-smoke-design", LabelWidthMM: 10, LabelHeightMM: 20})
	if err != nil {
		t.Fatalf("CreatePageDesign: %v", err)
	}
	t.Cleanup(func() { _ = store.DeletePageDesign(ctx, designID) })

	role, err := store.CreateRole(ctx, "postgres-smoke-role", "PostgreSQL smoke role")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() {
		_ = store.SetUserRole(ctx, email, "guest")
		_ = store.DeleteRole(ctx, role.ID)
	})
	if err := store.SetRolePermissions(ctx, role.ID, []string{string(model.PermDesignRead), string(model.PermPresetRead)}); err != nil {
		t.Fatalf("SetRolePermissions: %v", err)
	}
	if err := store.SetUserRole(ctx, email, role.Name); err != nil {
		t.Fatalf("SetUserRole: %v", err)
	}
	perms, err := store.GetPermissionsByRoleName(ctx, role.Name)
	if err != nil {
		t.Fatalf("GetPermissionsByRoleName: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("role permissions = %v, want two", perms)
	}

	accountID, err := store.CreateAccurateAccount(ctx, storage.AccurateAccount{Name: "postgres-smoke-account", Provider: "web", Enabled: true})
	if err != nil {
		t.Fatalf("CreateAccurateAccount: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteAccurateAccount(ctx, accountID) })
	if _, err := store.GetAccurateAccount(ctx, accountID); err != nil {
		t.Fatalf("GetAccurateAccount: %v", err)
	}
	if _, err := store.GetPageDesignByID(ctx, designID); err != nil {
		t.Fatalf("GetPageDesignByID: %v", err)
	}
}
