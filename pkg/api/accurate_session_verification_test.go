package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/auth"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

type verifiedStatus struct {
	Connected         bool               `json:"connected"`
	SessionState      string             `json:"sessionState"`
	Database          *accurate.Database `json:"database"`
	VerificationError string             `json:"verificationError"`
}

func webSessionFixture(t *testing.T, upstream string) (*storage.Storage, *accounts.Pool, *api.Server) {
	t.Helper()
	store, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "sessions.db"), EncryptionKey: multiAccountTestKey})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Selected", "Other"} {
		id, err := store.CreateAccurateAccountSecure(context.Background(), storage.AccurateAccount{Name: name, Provider: "web", Email: "test@accurate.id", Password: "SecretPass123!", Enabled: true}, box.Seal)
		if err != nil {
			t.Fatal(err)
		}
		jar := accurate.NewMemoryJar()
		u, _ := url.Parse(upstream)
		jar.SetCookies(u, []*http.Cookie{{Name: "JSESSIONID", Value: "stored", Path: "/"}})
		tenant := &accurate.TenantSession{Host: upstream, DSI: "stored-dsi", Database: accurate.DatabaseInfo{ID: id, Alias: name, Host: upstream}}
		if err = store.SaveAccurateSession(id, "web", "test@accurate.id", "stored-device", jar, tenant, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.SetDefaultAccurateAccount(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	factory := func(_, email, password, _, _ string) (accurate.Provider, error) {
		client := accurate.NewClient(nil)
		client.SetIdPHost(upstream)
		return web.New(email, password, client), nil
	}
	pool := accounts.NewPool(store, factory)
	server := api.New(api.Config{Store: store, AccuratePool: pool, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey}})
	return store, pool, server
}

func readVerifiedStatus(t *testing.T, server *api.Server, id int) verifiedStatus {
	t.Helper()
	resp, err := server.App().Test(httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/accurate/status?accountId=%d&validate=true", id), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status HTTP %d", resp.StatusCode)
	}
	var result verifiedStatus
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAccurateExplicitValidationClearsUnverifiedSession(t *testing.T) {
	for _, failure := range []string{"unavailable", "malformed", "network"} {
		t.Run(failure, func(t *testing.T) {
			var requests atomic.Int32
			var fail atomic.Bool
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/manage" || r.Method != http.MethodGet {
					t.Errorf("unexpected upstream %s %s", r.Method, r.URL.Path)
				}
				cookie, err := r.Cookie("JSESSIONID")
				if err != nil || cookie.Value != "stored" {
					t.Error("probe did not send stored session")
				}
				if fail.Load() {
					if failure == "unavailable" {
						http.Error(w, "upstream unavailable", 503)
					} else {
						_, _ = w.Write([]byte(`<script>var contentData={};</script>`))
					}
					return
				}
				_, _ = w.Write([]byte(`<script>var contentData={"dbList":[]};</script>`))
			}))
			defer upstream.Close()
			store, pool, server := webSessionFixture(t, upstream.URL)
			for range 2 {
				got := readVerifiedStatus(t, server, 1)
				if got.SessionState != "valid" || !got.Connected || got.Database == nil {
					t.Fatalf("valid status: %+v", got)
				}
			}
			fail.Store(true)
			if failure == "network" {
				upstream.Close()
			}
			got := readVerifiedStatus(t, server, 1)
			if got.SessionState != "unverified" || got.Connected || got.Database != nil || got.VerificationError == "" {
				t.Fatalf("unverified status: %+v", got)
			}
			_, _, _, cookies, tenant, _, err := store.LoadAccurateSession(1)
			if err != nil || len(cookies) != 0 || tenant != nil {
				t.Fatalf("failed verification retained session: %v", err)
			}
			entry, err := pool.Resolve(1, "web", "test@accurate.id", "SecretPass123!", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if entry.Session.Status().Connected || len(entry.Session.Client().Jar().AllCookies()) != 0 {
				t.Fatal("failed verification retained in-memory authentication")
			}
			before := requests.Load()
			fail.Store(false)
			if got = readVerifiedStatus(t, server, 1); got.SessionState != "disconnected" || got.Connected {
				t.Fatalf("invalid session revived without a login: %+v", got)
			}
			if requests.Load() != before {
				t.Fatal("disconnected status automatically retried upstream")
			}
		})
	}
}

func TestAccurateStatusConfirmedExpiryClearsOnlyProbedAccount(t *testing.T) {
	for _, shape := range []string{"redirect", "login_html", "unauthorized"} {
		t.Run(shape, func(t *testing.T) {
			var requests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch shape {
				case "redirect":
					http.Redirect(w, r, "/login", 302)
				case "login_html":
					_, _ = w.Write([]byte(`<form action="/auth.do"><input name="j_username"><input name="j_password"></form>`))
				default:
					w.WriteHeader(401)
				}
			}))
			defer upstream.Close()
			store, pool, server := webSessionFixture(t, upstream.URL)
			got := readVerifiedStatus(t, server, 1)
			if got.SessionState != "expired" || got.Connected || got.Database != nil {
				t.Fatalf("expired status: %+v", got)
			}
			_, _, _, cookies, tenant, _, err := store.LoadAccurateSession(1)
			if err != nil || len(cookies) != 0 || tenant != nil {
				t.Fatalf("stale persisted session remains: cookies=%v tenant=%v err=%v", cookies, tenant, err)
			}
			entry, err := pool.Resolve(1, "web", "test@accurate.id", "SecretPass123!", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if entry.Session.Status().Connected || len(entry.Session.Client().Jar().AllCookies()) != 0 {
				t.Fatal("stale in-memory session remains")
			}
			_, _, _, otherCookies, otherTenant, _, err := store.LoadAccurateSession(2)
			if err != nil || len(otherCookies) == 0 || otherTenant == nil {
				t.Fatal("probe cleared inactive account")
			}
			if got = readVerifiedStatus(t, server, 1); got.SessionState != "disconnected" {
				t.Fatalf("cleared status: %+v", got)
			}
			if requests.Load() != 1 {
				t.Fatalf("unexpected automatic login/probe after clearing: %d requests", requests.Load())
			}
		})
	}
}

func TestAccurateStatusProbeSerializesSessionReplacement(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		http.Redirect(w, r, "/login", 302)
	}))
	defer upstream.Close()
	store, pool, server := webSessionFixture(t, upstream.URL)
	probeDone := make(chan *http.Response, 1)
	go func() {
		resp, _ := server.App().Test(httptest.NewRequest("GET", "/api/v1/accurate/status?accountId=1&validate=true", nil))
		probeDone <- resp
	}()
	<-started
	replacementDone := make(chan error, 1)
	go func() {
		// Login and credential replacement use this same account operation lock.
		unlock := pool.LockAccount(1)
		defer unlock()
		entry, err := pool.Resolve(1, "web", "test@accurate.id", "SecretPass123!", "", "")
		if err == nil {
			err = entry.Session.SetTenantSession(&accurate.TenantSession{Host: upstream.URL, DSI: "replacement-dsi", Database: accurate.DatabaseInfo{ID: 1, Alias: "Replacement", Host: upstream.URL}})
		}
		replacementDone <- err
	}()
	select {
	case err := <-replacementDone:
		t.Fatalf("session replacement bypassed in-flight probe: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	probe := <-probeDone
	if probe == nil {
		t.Fatal("probe failed")
	}
	probe.Body.Close()
	if err := <-replacementDone; err != nil {
		t.Fatal(err)
	}
	_, _, _, _, tenant, _, err := store.LoadAccurateSession(1)
	if err != nil || tenant == nil || tenant.DSI != "replacement-dsi" {
		t.Fatalf("replacement session lost: tenant=%v err=%v", tenant, err)
	}
}

func TestAccurateStatusPreservesOfficialProviderSemantics(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("official status unexpectedly called upstream %s", r.URL.Path)
		w.WriteHeader(401)
	}))
	defer upstream.Close()
	store, _, _ := webSessionFixture(t, upstream.URL)
	box, err := accounts.NewSecretBox(multiAccountTestKey)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateAccurateAccountSecure(context.Background(), storage.AccurateAccount{Name: "Official", Provider: "official", Enabled: true, APIToken: "aat.fixture"}, box.Seal)
	if err != nil {
		t.Fatal(err)
	}
	state := &accurate.OfficialSessionState{Host: "https://fixture.accurate.id", Database: accurate.Database{ID: id, Host: "https://fixture.accurate.id"}}
	if err = store.SaveAccurateSession(id, "official", "", "", nil, nil, state); err != nil {
		t.Fatal(err)
	}
	if err = store.SetDefaultAccurateAccount(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	pool := accounts.NewPool(store, func(_, _, _, token, _ string) (accurate.Provider, error) {
		return official.New(token, official.WithBaseURL(upstream.URL)), nil
	})
	server := api.New(api.Config{Store: store, AccuratePool: pool, AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: multiAccountTestKey}})
	got := readVerifiedStatus(t, server, int(id))
	if got.SessionState != "not_applicable" || !got.Connected || got.Database == nil {
		t.Fatalf("official status: %+v", got)
	}
}

func TestAccurateStatusProbePreservesAccountReadPermission(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`<script>var contentData={"dbList":[]};</script>`))
	}))
	defer upstream.Close()
	store, pool, _ := webSessionFixture(t, upstream.URL)
	user, err := store.UpsertUser(context.Background(), "operator@example.com", "Operator", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetUserRole(context.Background(), user.Email, "user"); err != nil {
		t.Fatal(err)
	}
	user.Role = "user"
	permissions, err := store.GetPermissionsByRoleName(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateToken("test", user, nil, permissions)
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(api.Config{Store: store, AccuratePool: pool, AppConfig: &config.Config{JWTSecret: "test", GoogleClientID: "auth-enabled-client", AccurateEncryptionKey: multiAccountTestKey}})
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/accurate/status?validate=true", 200}, {"/api/v1/accurate/status?accountId=2&validate=true", 403}} {
		req := httptest.NewRequest("GET", tc.path, nil)
		req.AddCookie(&http.Cookie{Name: "warehouse_session", Value: token})
		resp, err := server.App().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatalf("%s HTTP=%d want %d", tc.path, resp.StatusCode, tc.status)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("unauthorized account triggered upstream probe: %d requests", requests.Load())
	}
}

func TestAccurateValidationOnlyProbesActiveEstablishedSession(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/manage" {
			t.Errorf("unexpected upstream path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<script>var contentData={"dbList":[]};</script>`))
	}))
	defer upstream.Close()
	_, pool, server := webSessionFixture(t, upstream.URL)
	for _, path := range []string{"/api/v1/accurate/status", "/api/v1/accurate/status?accountId=1", "/api/v1/accurate/status?accountId=2"} {
		resp, err := server.App().Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		var got verifiedStatus
		if err = json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || !got.Connected || got.SessionState != "unverified" {
			t.Fatalf("cheap metadata %s: HTTP=%d %+v", path, resp.StatusCode, got)
		}
	}
	resp, err := server.App().Test(httptest.NewRequest("GET", "/api/v1/accurate/status?accountId=2&validate=true", nil))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("inactive validation HTTP=%d", resp.StatusCode)
	}
	if requests.Load() != 0 {
		t.Fatalf("ordinary/inactive status made %d upstream requests", requests.Load())
	}
	if got := readVerifiedStatus(t, server, 1); got.SessionState != "valid" || !got.Connected {
		t.Fatalf("active validation: %+v", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("active validation made %d upstream requests, want exactly 1", requests.Load())
	}
	entry, err := pool.Resolve(1, "web", "test@accurate.id", "SecretPass123!", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = entry.Session.SetTenantSession(nil); err != nil {
		t.Fatal(err)
	}
	if len(entry.Session.Client().Jar().AllCookies()) == 0 {
		t.Fatal("fixture must retain cookies-only login")
	}
	if got := readVerifiedStatus(t, server, 1); got.SessionState != "disconnected" || got.Connected {
		t.Fatalf("cookies-only validation: %+v", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("cookies-only validation probed upstream: %d", requests.Load())
	}
	if err = entry.Session.Logout(); err != nil {
		t.Fatal(err)
	}
	if got := readVerifiedStatus(t, server, 1); got.SessionState != "disconnected" || got.Connected {
		t.Fatalf("disconnected validation: %+v", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("disconnected validation probed upstream: %d", requests.Load())
	}
}
