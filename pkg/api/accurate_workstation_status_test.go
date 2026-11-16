package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

type workstationStatus struct {
	Enabled     bool               `json:"enabled"`
	Connected   bool               `json:"connected"`
	AccountID   *int64             `json:"accountId"`
	AccountName string             `json:"accountName"`
	Database    *accurate.Database `json:"database"`
}

func readWorkstationStatus(t *testing.T, server *api.Server) workstationStatus {
	t.Helper()
	resp, err := server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/status", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("workstation status HTTP %d", resp.StatusCode)
	}
	var status workstationStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func connectAccountDatabase(t *testing.T, server *api.Server, accountID int64, databaseID int64) {
	t.Helper()
	body, _ := json.Marshal(map[string]int64{"id": databaseID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accurate/database?accountId="+itoa(accountID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.App().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("database selection HTTP %d", resp.StatusCode)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func TestConnectingFirstAccountMakesWorkstationStatusOnline(t *testing.T) {
	ctx := context.Background()
	store, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "workstation.db"), EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "Workstation", Provider: "official", Enabled: true, APIToken: "W-ITEM"}, box.Seal)
	if err != nil {
		t.Fatal(err)
	}
	factory := func(_, _, _, token, _ string) (accurate.Provider, error) {
		return &inventoryFixtureProvider{item: token}, nil
	}
	server := api.New(api.Config{Store: store, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(store, factory)})

	// Before connecting: no active account, so the shared workstation status is offline.
	if status := readWorkstationStatus(t, server); status.Connected || status.AccountID != nil {
		t.Fatalf("pre-connect workstation status: %+v", status)
	}

	// /accurate connect flow: login then database selection for the account.
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/accurate/login?accountId="+itoa(id), bytes.NewReader([]byte(`{}`)))
	loginReq.Header.Set("Content-Type", "application/json")
	login, err := server.App().Test(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login HTTP %d", login.StatusCode)
	}
	connectAccountDatabase(t, server, id, 1)

	// After connecting (and selecting a database) the barcode page's status
	// read must report the account online without any manual activation.
	status := readWorkstationStatus(t, server)
	if !status.Enabled || !status.Connected || status.AccountID == nil || *status.AccountID != id || status.Database == nil || status.Database.ID != 1 {
		t.Fatalf("post-connect workstation status: %+v", status)
	}
}

func TestConnectingSecondAccountKeepsExplicitDefaultActive(t *testing.T) {
	ctx := context.Background()
	store, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "workstation-default.db"), EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	create := func(name, token string) int64 {
		id, e := store.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: name, Provider: "official", Enabled: true, APIToken: token}, box.Seal)
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	first, second := create("First", "A-ITEM"), create("Second", "B-ITEM")
	// First account is already the workstation default with an established session.
	state := &accurate.OfficialSessionState{DatabaseID: first, Host: "https://fixture.accurate.id", Database: accurate.Database{ID: first, Alias: "First", Host: "https://fixture.accurate.id"}}
	if err = store.SaveAccurateSession(first, "official", "", "", nil, nil, state); err != nil {
		t.Fatal(err)
	}
	if err = store.SetDefaultAccurateAccount(ctx, first); err != nil {
		t.Fatal(err)
	}
	factory := func(_, _, _, token, _ string) (accurate.Provider, error) {
		return &inventoryFixtureProvider{item: token}, nil
	}
	server := api.New(api.Config{Store: store, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(store, factory)})

	// Connect the second account; the explicit default must not be hijacked.
	connectAccountDatabase(t, server, second, 2)
	status := readWorkstationStatus(t, server)
	if !status.Connected || status.AccountID == nil || *status.AccountID != first {
		t.Fatalf("connecting second account switched the active workstation account: %+v", status)
	}
}
