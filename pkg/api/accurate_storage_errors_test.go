package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
)

type failingSessionStore struct {
	session.SessionStore
	loadErr, clearErr error
}

func (s *failingSessionStore) LoadAccurateSession(id int64) (string, string, string, []*accurate.Cookie, *accurate.TenantSession, *accurate.OfficialSessionState, error) {
	if s.loadErr != nil {
		return "", "", "", nil, nil, nil, s.loadErr
	}
	return s.SessionStore.LoadAccurateSession(id)
}
func (s *failingSessionStore) ClearAccurateSession(id int64) error {
	if s.clearErr != nil {
		return s.clearErr
	}
	return s.SessionStore.ClearAccurateSession(id)
}

type expiredStorageFixtureProvider struct{ inventoryFixtureProvider }

func (*expiredStorageFixtureProvider) ID() string { return "web" }
func (*expiredStorageFixtureProvider) Login(context.Context, string, string) (accurate.LoginResult, error) {
	return accurate.LoginResult{}, accurate.ErrInvalidOTP
}
func (*expiredStorageFixtureProvider) ListDatabases(context.Context) ([]accurate.Database, error) {
	return nil, accurate.ErrSessionExpired
}
func (*expiredStorageFixtureProvider) SearchItems(context.Context, string, int, int) (accurate.SearchItemsResult, error) {
	return accurate.SearchItemsResult{}, accurate.ErrSessionExpired
}
func (*expiredStorageFixtureProvider) ListDocuments(context.Context, string, string, string, string, int, int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	return nil, accurate.Pagination{}, accurate.ErrSessionExpired
}
func (*expiredStorageFixtureProvider) GetDocumentItems(context.Context, string, int64) ([]accurate.NormalizedItem, error) {
	return nil, accurate.ErrSessionExpired
}

func TestAccurateSessionLoadFailureFailsClosedAndCanRecover(t *testing.T) {
	store, _, _ := webSessionFixture(t, "http://127.0.0.1:1")
	failure := errors.New("postgres://private-user:private-password@private-host/private-db")
	faults := &failingSessionStore{SessionStore: store, loadErr: failure}
	pool := accounts.NewPool(faults, nil)
	server := api.New(api.Config{Store: store, AccuratePool: pool, AppConfig: &config.Config{AccurateEncryptionKey: multiAccountTestKey}})
	for _, path := range []string{"/api/v1/accurate/status", "/api/v1/accurate/items"} {
		resp, err := server.App().Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 500 || !strings.Contains(string(body), "could not be initialized") || strings.Contains(string(body), "private-") {
			t.Fatalf("load failure HTTP %d: %s", resp.StatusCode, body)
		}
	}
	entry, err := pool.Resolve(1, "web", "", "", "", "")
	if entry != nil || !errors.Is(err, failure) {
		t.Fatalf("load failure returned entry=%v error=%v", entry != nil, err)
	}
	faults.loadErr = nil
	entry, err = pool.Resolve(1, "web", "", "", "", "")
	if err != nil || entry == nil || !entry.Session.Status().Connected {
		t.Fatalf("failed initialization was cached instead of restoring stored session: %v", err)
	}
}

func TestAccurateInvalidationStorageFailureReturnsServerError(t *testing.T) {
	for _, tc := range []struct{ name, method, path, body string }{
		{"validation", "GET", "/api/v1/accurate/status?validate=true", ""},
		{"items", "GET", "/api/v1/accurate/items", ""},
		{"documents", "GET", "/api/v1/accurate/sources/receive-item/documents", ""},
		{"document_items", "GET", "/api/v1/accurate/sources/receive-item/documents/1/items", ""},
		{"mfa_failure", "POST", "/api/v1/accurate/login", `{"challengeId":"pending","otp":"123456"}`},
		{"fresh_login", "POST", "/api/v1/accurate/login", `{}`},
		{"logout", "POST", "/api/v1/accurate/logout", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _, _ := webSessionFixture(t, "http://127.0.0.1:1")
			failure := errors.New("postgres://private-user:private-password@private-host/private-db")
			faults := &failingSessionStore{SessionStore: store, clearErr: failure}
			pool := accounts.NewPool(faults, func(string, string, string, string, string) (accurate.Provider, error) {
				return &expiredStorageFixtureProvider{}, nil
			})
			server := api.New(api.Config{Store: store, AccuratePool: pool, AppConfig: &config.Config{AccurateEncryptionKey: multiAccountTestKey}})
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := server.App().Test(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 500 || !strings.Contains(string(body), "Failed to clear Accurate session") || strings.Contains(string(body), "private-") {
				t.Fatalf("clear failure HTTP %d: %s", resp.StatusCode, body)
			}
			entry, err := pool.Resolve(1, "web", "", "", "", "")
			if err != nil || entry.Session.Status().Connected || len(entry.Session.Client().Jar().AllCookies()) != 0 {
				t.Fatalf("failed clear retained runtime authentication: %v", err)
			}
			_, _, _, _, persisted, _, err := store.LoadAccurateSession(1)
			if err != nil || persisted == nil {
				t.Fatal("fixture must retain stale disk state when clear fails")
			}
			status, err := server.App().Test(httptest.NewRequest(http.MethodGet, "/api/v1/accurate/status", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer status.Body.Close()
			statusBody, _ := io.ReadAll(status.Body)
			if status.StatusCode != 200 || !strings.Contains(string(statusBody), `"connected":false`) {
				t.Fatalf("cached runtime session resurrected: HTTP %d %s", status.StatusCode, statusBody)
			}
		})
	}
}

func TestAccurateAccountMetadataLoadFailureReturnsServerError(t *testing.T) {
	for _, tc := range []struct{ name, method, path, body, fault string }{
		{"list", "GET", "/api/v1/accurate/accounts", "", `UPDATE accurate_accounts SET cookies='invalid-ciphertext' WHERE id=1`},
		{"create", "POST", "/api/v1/accurate/accounts", `{"name":"New","provider":"web","email":"test@accurate.id","password":"fixture-password"}`, `CREATE TRIGGER corrupt_session AFTER INSERT ON accurate_accounts BEGIN UPDATE accurate_accounts SET cookies='invalid-ciphertext' WHERE id=NEW.id; END`},
		{"update", "PUT", "/api/v1/accurate/accounts/1", `{"name":"Renamed","enabled":true,"email":"test@accurate.id"}`, `CREATE TRIGGER corrupt_session AFTER UPDATE ON accurate_accounts BEGIN UPDATE accurate_accounts SET cookies='invalid-ciphertext' WHERE id=NEW.id; END`},
		{"activate_metadata", "POST", "/api/v1/accurate/accounts/1/activate", "", `CREATE TRIGGER corrupt_session AFTER UPDATE ON accurate_accounts BEGIN UPDATE accurate_accounts SET cookies='invalid-ciphertext' WHERE id=NEW.id; END`},
		{"activate_storage", "POST", "/api/v1/accurate/accounts/1/activate", "", `UPDATE accurate_accounts SET cookies='invalid-ciphertext' WHERE id=1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _, server := webSessionFixture(t, "http://127.0.0.1:1")
			if _, err := store.DB().Exec(tc.fault); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := server.App().Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 500 || strings.Contains(string(body), "invalid-ciphertext") || strings.Contains(string(body), `"connected"`) {
				t.Fatalf("metadata failure HTTP %d: %s", resp.StatusCode, body)
			}
		})
	}
}
