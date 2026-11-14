package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	_ "modernc.org/sqlite"
)

const storageAccountKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestAccountCiphertextRequiresCorrectKeyOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.db")
	box, err := accounts.NewSecretBox(storageAccountKey)
	if err != nil {
		t.Fatal(err)
	}
	st, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: storageAccountKey})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccurateAccountSecure(context.Background(), storage.AccurateAccount{Name: "A", Provider: "web", Enabled: true, Email: "a@example.com", Password: "secret"}, box.Seal)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.New(storage.Options{SQLitePath: path}); err == nil {
		t.Fatal("startup without key accepted encrypted credentials")
	}
	if _, err = storage.New(storage.Options{SQLitePath: path, EncryptionKey: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}); err == nil {
		t.Fatal("startup with wrong key accepted ciphertext")
	}
	restored, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: storageAccountKey})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
}

func TestFailedAccountEncryptionRollsBackRow(t *testing.T) {
	st, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "rollback.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.CreateAccurateAccountSecure(context.Background(), storage.AccurateAccount{Name: "Broken", Provider: "web", Enabled: true, Password: "secret"}, func(int64, string, string) (string, error) { return "", errors.New("encryption failure") })
	if err == nil {
		t.Fatal("expected encryption error")
	}
	rows, err := st.ListAccurateAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("failed create left %d rows", len(rows))
	}
}

func TestDisableClearsDefaultAndAccountSession(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "disable.db")
	st, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: storageAccountKey})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	box, err := accounts.NewSecretBox(storageAccountKey)
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "Official", Provider: "official", Enabled: true, APIToken: "token"}, box.Seal)
	if err != nil {
		t.Fatal(err)
	}
	session := &accurate.OfficialSessionState{Host: "https://fixture.accurate.id", Database: accurate.Database{ID: 123, Alias: "Main", Host: "https://fixture.accurate.id"}}
	if err = st.SaveAccurateSession(id, "official", "", "", nil, nil, session); err != nil {
		t.Fatal(err)
	}
	if err = st.SetDefaultAccurateAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	account, err := st.GetAccurateAccount(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	account.Enabled = false
	if err = st.UpdateAccurateAccountAndSession(ctx, *account); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetDefaultAccurateAccount(ctx); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("disabled account remained active: %v", err)
	}
	provider, _, _, _, _, official, err := st.LoadAccurateSession(id)
	if err != nil {
		t.Fatal(err)
	}
	if provider != "official" || official != nil {
		t.Fatalf("disabled account session not cleared: provider=%q session=%+v", provider, official)
	}
}

func TestAdminAccountPermissionsMigrateOnceAndRevocationPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE roles(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL UNIQUE COLLATE NOCASE,description TEXT NOT NULL DEFAULT '',is_system INTEGER NOT NULL DEFAULT 0,created_at DATETIME NOT NULL DEFAULT(datetime('now')),updated_at DATETIME NOT NULL DEFAULT(datetime('now'))); CREATE TABLE role_permissions(role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,permission TEXT NOT NULL,PRIMARY KEY(role_id,permission)); INSERT INTO roles(name,is_system) VALUES('admin',1); INSERT INTO role_permissions(role_id,permission) VALUES(1,'design:read');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: storageAccountKey})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	role, err := st.GetRoleByName(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	perms, err := st.GetRolePermissions(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"accurate_account:read", "accurate_account:create", "accurate_account:update", "accurate_account:delete"} {
		found := false
		for _, p := range perms {
			if p == required {
				found = true
			}
		}
		if !found {
			t.Fatalf("existing admin did not receive %s: %v", required, perms)
		}
	}
	if err = st.SetRolePermissions(ctx, role.ID, []string{"design:read"}); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = storage.New(storage.Options{SQLitePath: path, EncryptionKey: storageAccountKey})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	perms, err = st.GetRolePermissions(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 1 || perms[0] != "design:read" {
		t.Fatalf("revoked account permissions were regranted: %v", perms)
	}
}
