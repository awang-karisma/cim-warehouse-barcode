package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

const testAccountKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestAccurateAccountCreateEncryptsAndRedactsSecrets(t *testing.T) {
	env := setupTestServer(t, true)
	env.cfg.AccurateEncryptionKey = testAccountKey
	token := env.createUserToken(t, "admin@example.com", "Admin", "admin")
	payload := []byte(`{"name":"Main","provider":"web","email":"operator@example.com","password":"DoNotExposeThis"}`)
	resp := env.makeRequest(http.MethodPost, "/api/v1/accurate/accounts", payload, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"has_password", "has_api_token", "has_signature_secret"} {
		if _, ok := body[key].(bool); !ok {
			t.Fatalf("response missing presence flag %q", key)
		}
	}
	if body["has_password"] != true || body["has_api_token"] != false || body["has_signature_secret"] != false {
		t.Fatalf("wrong secret presence flags: %v", body)
	}
	encoded, _ := json.Marshal(body)
	if strings.Contains(string(encoded), "DoNotExposeThis") {
		t.Fatal("API response exposed plaintext secret")
	}
	rows, err := env.store.ListAccurateAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("accounts=%d", len(rows))
	}
	if rows[0].Password == "DoNotExposeThis" || rows[0].Password == "" {
		t.Fatal("password was not stored as ciphertext")
	}
	box, err := accounts.NewSecretBox(testAccountKey)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Open(rows[0].ID, "password", rows[0].Password)
	if err != nil || plain != "DoNotExposeThis" {
		t.Fatalf("decrypt=%q err=%v", plain, err)
	}
}

func TestAccurateAccountWriteRequiresEncryptionKey(t *testing.T) {
	env := setupTestServer(t, true)
	admin := env.createUserToken(t, "admin@example.com", "Admin", "admin")
	payload := []byte(`{"name":"No Key","provider":"web","email":"operator@example.com","password":"secret"}`)
	resp := env.makeRequest(http.MethodPost, "/api/v1/accurate/accounts", payload, admin)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("create without key status=%d", resp.StatusCode)
	}
	rows, err := env.store.ListAccurateAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("failed encryption left %d rows", len(rows))
	}
}

func TestAccurateAccountPermissionsAreSeparate(t *testing.T) {
	env := setupTestServer(t, true)
	env.cfg.AccurateEncryptionKey = testAccountKey
	admin := env.createUserToken(t, "admin@example.com", "Admin", "admin")
	create := []byte(`{"name":"Main","provider":"official","email":"","api_token":"aat.sentinel"}`)
	resp := env.makeRequest(http.MethodPost, "/api/v1/accurate/accounts", create, admin)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin create status=%d", resp.StatusCode)
	}
	user := env.createUserToken(t, "operator@example.com", "Operator", "user")
	for _, tc := range []struct {
		method, path string
		body         []byte
	}{{http.MethodGet, "/api/v1/accurate/accounts", nil}, {http.MethodPost, "/api/v1/accurate/accounts", create}, {http.MethodGet, "/api/v1/accurate/status?accountId=1", nil}} {
		resp = env.makeRequest(tc.method, tc.path, tc.body, user)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("user %s %s status=%d", tc.method, tc.path, resp.StatusCode)
		}
	}
	role, err := env.store.CreateRole(context.Background(), "account-reader", "Read account metadata only")
	if err != nil {
		t.Fatal(err)
	}
	if err = env.store.SetRolePermissions(context.Background(), role.ID, []string{string(model.PermAccurateAccountRead)}); err != nil {
		t.Fatal(err)
	}
	if _, err = env.store.UpsertUser(context.Background(), "reader@example.com", "Reader", ""); err != nil {
		t.Fatal(err)
	}
	if err = env.store.SetUserRole(context.Background(), "reader@example.com", "account-reader"); err != nil {
		t.Fatal(err)
	}
	reader := env.createUserToken(t, "reader@example.com", "Reader", "account-reader")
	resp = env.makeRequest(http.MethodGet, "/api/v1/accurate/accounts", nil, reader)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read-only list status=%d", resp.StatusCode)
	}
	resp = env.makeRequest(http.MethodDelete, "/api/v1/accurate/accounts/1", nil, reader)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only delete status=%d", resp.StatusCode)
	}
}

func TestAccurateAccountDuplicateNamesAreCaseInsensitive(t *testing.T) {
	store, err := storage.New(filepath.Join(t.TempDir(), "dupe.db"), testAccountKey)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	box, err := accounts.NewSecretBox(testAccountKey)
	if err != nil {
		t.Fatal(err)
	}
	input := storage.AccurateAccount{Name: "Primary", Provider: "web", Enabled: true, Email: "e@example.com", Password: "first"}
	if _, err = store.CreateAccurateAccountSecure(context.Background(), input, box.Seal); err != nil {
		t.Fatal(err)
	}
	input.Name = "primary"
	if _, err = store.CreateAccurateAccountSecure(context.Background(), input, box.Seal); err == nil {
		t.Fatal("case-insensitive duplicate name accepted")
	}
	all, err := store.ListAccurateAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("duplicate create left %d rows", len(all))
	}
}
