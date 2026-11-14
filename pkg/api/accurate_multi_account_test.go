package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

const multiAccountTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

type inventoryFixtureProvider struct {
	item       string
	started    chan struct{}
	release    chan struct{}
	requireMFA bool
}

func (p *inventoryFixtureProvider) ID() string { return "official" }
func (p *inventoryFixtureProvider) Login(_ context.Context, challengeID, _ string) (accurate.LoginResult, error) {
	if !p.requireMFA {
		return accurate.LoginResult{Status: accurate.LoginSuccess}, nil
	}
	expected := p.item + "-challenge"
	if challengeID == "" {
		return accurate.LoginResult{Status: accurate.LoginMFARequired, ChallengeID: expected}, nil
	}
	if challengeID != expected {
		return accurate.LoginResult{}, errors.New("challenge does not belong to this account")
	}
	return accurate.LoginResult{Status: accurate.LoginSuccess}, nil
}
func (p *inventoryFixtureProvider) ListDatabases(context.Context) ([]accurate.Database, error) {
	id := int64(1)
	if p.item == "B-ITEM" || p.item == "B" {
		id = 2
	}
	return []accurate.Database{{ID: id, Alias: "Factory " + p.item, Host: "https://fixture.accurate.id", UniqueID: p.item}}, nil
}
func (p *inventoryFixtureProvider) OpenDatabase(_ context.Context, id int64) (accurate.Database, error) {
	return accurate.Database{ID: id, Alias: "Factory " + p.item, Host: "https://fixture.accurate.id", UniqueID: p.item}, nil
}
func (p *inventoryFixtureProvider) SearchItems(ctx context.Context, _ string, _, _ int) (accurate.SearchItemsResult, error) {
	if p.started != nil {
		close(p.started)
		<-p.release
	}
	return accurate.SearchItemsResult{Items: []accurate.InventoryItem{{ID: 1, No: p.item, Name: p.item, CanPrintBarcode: true}}, Pagination: accurate.Pagination{Page: 1, PageCount: 1, PageSize: 50, RowCount: 1}}, nil
}
func (p *inventoryFixtureProvider) ListDocuments(context.Context, string, string, string, string, int, int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	return nil, accurate.Pagination{}, nil
}
func (p *inventoryFixtureProvider) GetDocumentItems(context.Context, string, int64) ([]accurate.NormalizedItem, error) {
	return nil, nil
}
func (p *inventoryFixtureProvider) SupportsSource(string) (bool, string) { return true, "" }

func TestAccurateAccountsResolveIsolatedActiveInventoryAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "multi.db")
	factory := func(_ string, _ string, _ string, token string, _ string) (accurate.Provider, error) {
		return &inventoryFixtureProvider{item: token}, nil
	}
	store, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	create := func(name, token string) int64 {
		t.Helper()
		id, e := store.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: name, Provider: "official", Enabled: true, APIToken: token}, box.Seal)
		if e != nil {
			t.Fatal(e)
		}
		state := &accurate.OfficialSessionState{DatabaseID: id, Host: "https://fixture.accurate.id", Database: accurate.Database{ID: id, Alias: name, Host: "https://fixture.accurate.id"}}
		if e = store.SaveAccurateSession(id, "official", "", "", nil, nil, state); e != nil {
			t.Fatal(e)
		}
		return id
	}
	idA := create("Entity A", "A-ITEM")
	idB := create("Entity B", "B-ITEM")
	if err = store.SetDefaultAccurateAccount(ctx, idA); err != nil {
		t.Fatal(err)
	}
	newServer := func(st *storage.Storage) *api.Server {
		return api.New(api.Config{Store: st, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey, RBACCacheTTL: time.Minute}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(st, factory)})
	}
	server := newServer(store)
	query := func(srv *api.Server, accountID string) string {
		t.Helper()
		path := "/api/v1/accurate/items"
		if accountID != "" {
			path += "?accountId=" + accountID
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp, err := srv.App().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("inventory status=%d for %s", resp.StatusCode, accountID)
		}
		var body accurate.SearchItemsResult
		if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 {
			t.Fatalf("unexpected inventory response: %+v", body)
		}
		return body.Items[0].No
	}
	for _, invalidID := range []string{"0", "-1", "not-a-number"} {
		resp, e := server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items?accountId="+invalidID, nil))
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("accountId %q status=%d", invalidID, resp.StatusCode)
		}
	}
	if got := query(server, ""); got != "A-ITEM" {
		t.Fatalf("active A returned %q", got)
	}
	if err = store.SetDefaultAccurateAccount(ctx, idB); err != nil {
		t.Fatal(err)
	}
	if got := query(server, ""); got != "B-ITEM" {
		t.Fatalf("active B returned %q", got)
	}
	if got := query(server, "1"); got != "A-ITEM" {
		t.Fatalf("explicit manager preview returned %q", got)
	}
	logout, err := server.App().Test(httptest.NewRequest(http.MethodPost, "/api/v1/accurate/logout?accountId=1", nil))
	if err != nil {
		t.Fatal(err)
	}
	logout.Body.Close()
	if logout.StatusCode != http.StatusOK {
		t.Fatalf("logout A status=%d", logout.StatusCode)
	}
	if got := query(server, ""); got != "B-ITEM" {
		t.Fatalf("logout A affected active B: %q", got)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.New(storage.Options{SQLitePath: path, EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server = newServer(store)
	if got := query(server, ""); got != "B-ITEM" {
		t.Fatalf("restart active B returned %q", got)
	}
	update := []byte(`{"name":"Entity B","enabled":false,"email":""}`)
	disabled, err := server.App().Test(httptest.NewRequest(http.MethodPut, "/api/v1/accurate/accounts/2", bytes.NewReader(update)))
	if err != nil {
		t.Fatal(err)
	}
	disabled.Body.Close()
	if disabled.StatusCode != http.StatusOK {
		t.Fatalf("disable B status=%d", disabled.StatusCode)
	}
	noActive, err := server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	noActive.Body.Close()
	if noActive.StatusCode != http.StatusConflict {
		t.Fatalf("query after disabling active B status=%d", noActive.StatusCode)
	}
	update = []byte(`{"name":"Entity B","enabled":true,"email":""}`)
	enabled, err := server.App().Test(httptest.NewRequest(http.MethodPut, "/api/v1/accurate/accounts/2", bytes.NewReader(update)))
	if err != nil {
		t.Fatal(err)
	}
	enabled.Body.Close()
	if enabled.StatusCode != http.StatusOK {
		t.Fatalf("reenable B status=%d", enabled.StatusCode)
	}
	noActive, err = server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	noActive.Body.Close()
	if noActive.StatusCode != http.StatusConflict {
		t.Fatalf("reenabled account auto-activated: status=%d", noActive.StatusCode)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/accurate/login?accountId=2", bytes.NewBufferString(`{}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := server.App().Test(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("reconnect B status=%d", loginResp.StatusCode)
	}
	databases, err := server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/databases?accountId=2", nil))
	if err != nil {
		t.Fatal(err)
	}
	var dbList []accurate.Database
	if err = json.NewDecoder(databases.Body).Decode(&dbList); err != nil {
		t.Fatal(err)
	}
	databases.Body.Close()
	if len(dbList) != 1 {
		t.Fatalf("reconnect databases=%v", dbList)
	}
	selectBody, _ := json.Marshal(map[string]int64{"id": dbList[0].ID})
	selectReq := httptest.NewRequest(http.MethodPost, "/api/v1/accurate/database?accountId=2", bytes.NewReader(selectBody))
	selectReq.Header.Set("Content-Type", "application/json")
	selected, err := server.App().Test(selectReq)
	if err != nil {
		t.Fatal(err)
	}
	selected.Body.Close()
	if selected.StatusCode != http.StatusOK {
		t.Fatalf("database selection status=%d", selected.StatusCode)
	}
	activated, err := server.App().Test(httptest.NewRequest(http.MethodPost, "/api/v1/accurate/accounts/2/activate", nil))
	if err != nil {
		t.Fatal(err)
	}
	activated.Body.Close()
	if activated.StatusCode != http.StatusOK {
		t.Fatalf("activation status=%d", activated.StatusCode)
	}
	deleted, err := server.App().Test(httptest.NewRequest(http.MethodDelete, "/api/v1/accurate/accounts/2", nil))
	if err != nil {
		t.Fatal(err)
	}
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusOK {
		t.Fatalf("active delete status=%d", deleted.StatusCode)
	}
	noActive, err = server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	noActive.Body.Close()
	if noActive.StatusCode != http.StatusConflict {
		t.Fatalf("query after deleting active account status=%d", noActive.StatusCode)
	}
}

func TestAccurateQueryKeepsResolvedAccountAcrossActivation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "blocked.db")
	store, err := storage.New(storage.Options{SQLitePath: path, EncryptionKey: multiAccountTestKey})
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
		state := &accurate.OfficialSessionState{DatabaseID: id, Host: "https://fixture.accurate.id", Database: accurate.Database{ID: id, Alias: name, Host: "https://fixture.accurate.id"}}
		if e = store.SaveAccurateSession(id, "official", "", "", nil, nil, state); e != nil {
			t.Fatal(e)
		}
		return id
	}
	idA := create("A", "A-ITEM")
	idB := create("B", "B-ITEM")
	if err = store.SetDefaultAccurateAccount(ctx, idA); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	factory := func(_ string, _ string, _ string, token string, _ string) (accurate.Provider, error) {
		p := &inventoryFixtureProvider{item: token}
		if token == "A-ITEM" {
			p.started = started
			p.release = release
		}
		return p, nil
	}
	srv := api.New(api.Config{Store: store, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey, RBACCacheTTL: time.Minute}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(store, factory)})
	response := make(chan *http.Response, 1)
	go func() {
		resp, _ := srv.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil))
		response <- resp
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("account A request did not reach fixture provider")
	}
	if err = store.SetDefaultAccurateAccount(ctx, idB); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	first := <-response
	if first.StatusCode != http.StatusOK {
		t.Fatalf("in-flight A status=%d", first.StatusCode)
	}
	var a accurate.SearchItemsResult
	if err = json.NewDecoder(first.Body).Decode(&a); err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	if len(a.Items) != 1 || a.Items[0].No != "A-ITEM" {
		t.Fatalf("in-flight response switched accounts: %+v", a)
	}
	second, err := srv.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	var b accurate.SearchItemsResult
	if err = json.NewDecoder(second.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	if second.StatusCode != http.StatusOK || len(b.Items) != 1 || b.Items[0].No != "B-ITEM" {
		t.Fatalf("post-activation response=%+v status=%d", b, second.StatusCode)
	}
}

func TestMFAChallengeCannotAuthenticateAnotherAccount(t *testing.T) {
	ctx := context.Background()
	store, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "mfa.db"), EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ name, token string }{{"A", "A"}, {"B", "B"}} {
		if _, err = store.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: entry.name, Provider: "official", Enabled: true, APIToken: entry.token}, box.Seal); err != nil {
			t.Fatal(err)
		}
	}
	factory := func(_ string, _ string, _ string, token string, _ string) (accurate.Provider, error) {
		return &inventoryFixtureProvider{item: token, requireMFA: true}, nil
	}
	srv := api.New(api.Config{Store: store, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(store, factory)})
	call := func(id int, body string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/accurate/login?accountId=%d", id), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, e := srv.App().Test(req)
		if e != nil {
			t.Fatal(e)
		}
		return resp
	}
	first := call(1, `{}`)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("challenge A status=%d", first.StatusCode)
	}
	var challenge struct {
		ChallengeID string `json:"challengeId"`
	}
	if e := json.NewDecoder(first.Body).Decode(&challenge); e != nil {
		t.Fatal(e)
	}
	first.Body.Close()
	if challenge.ChallengeID == "" {
		t.Fatal("account A did not issue a challenge")
	}
	body, _ := json.Marshal(map[string]string{"challengeId": challenge.ChallengeID, "otp": "123456"})
	cross := call(2, string(body))
	defer cross.Body.Close()
	if cross.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-account MFA status=%d", cross.StatusCode)
	}
	second := call(2, `{}`)
	defer second.Body.Close()
	var challengeB struct {
		ChallengeID string `json:"challengeId"`
	}
	if e := json.NewDecoder(second.Body).Decode(&challengeB); e != nil {
		t.Fatal(e)
	}
	if challengeB.ChallengeID == "" || challengeB.ChallengeID == challenge.ChallengeID {
		t.Fatalf("account B challenge was not isolated: A=%q B=%q", challenge.ChallengeID, challengeB.ChallengeID)
	}
}

func TestWarehouseUserUsesActiveAccountWithoutCredentialPermission(t *testing.T) {
	ctx := context.Background()
	key := multiAccountTestKey
	store, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "user-active.db"), EncryptionKey: key})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	box, err := accounts.NewSecretBox(key)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "Active", Provider: "official", Enabled: true, APIToken: "A-ITEM"}, box.Seal)
	if err != nil {
		t.Fatal(err)
	}
	state := &accurate.OfficialSessionState{DatabaseID: id, Host: "https://fixture.accurate.id", Database: accurate.Database{ID: id, Alias: "Active", Host: "https://fixture.accurate.id"}}
	if err = store.SaveAccurateSession(id, "official", "", "", nil, nil, state); err != nil {
		t.Fatal(err)
	}
	if err = store.SetDefaultAccurateAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	user, err := store.UpsertUser(ctx, "operator@example.com", "Operator", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetUserRole(ctx, "operator@example.com", "user"); err != nil {
		t.Fatal(err)
	}
	user.Role = "user"
	permissions, err := store.GetPermissionsByRoleName(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	const secret = "daily-user-test-secret"
	token, err := auth.GenerateToken(secret, user, nil, permissions)
	if err != nil {
		t.Fatal(err)
	}
	factory := func(_ string, _ string, _ string, token string, _ string) (accurate.Provider, error) {
		return &inventoryFixtureProvider{item: token}, nil
	}
	srv := api.New(api.Config{Store: store, AppConfig: &config.Config{JWTSecret: secret, GoogleClientID: "auth-enabled-client", AccurateEncryptionKey: key}, PermCache: rbac.NewMemoryCache(time.Minute), AccuratePool: accounts.NewPool(store, factory)})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accurate/items", nil)
	request.AddCookie(&http.Cookie{Name: "warehouse_session", Value: token})
	response, err := srv.App().Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("active inventory status=%d", response.StatusCode)
	}
	var result accurate.SearchItemsResult
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].No != "A-ITEM" {
		t.Fatalf("wrong active inventory: %+v", result)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/accurate/accounts", nil)
	request.AddCookie(&http.Cookie{Name: "warehouse_session", Value: token})
	response, err = srv.App().Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("account metadata list status=%d", response.StatusCode)
	}
}
