package session_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

func TestSessionManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "session_mgr_test.db")

	st, err := storage.New(storage.Options{SQLitePath: dbPath, EncryptionKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"})
	if err != nil {
		t.Fatalf("storage.New() failed: %v", err)
	}
	defer st.Close()

	// Create manager with store
	_, _ = st.CreateAccurateAccount(t.Context(), storage.AccurateAccount{Name: "test", Provider: "web", Enabled: true})
	mgr := session.NewManager(st, 1, nil)

	// Set email
	testEmail := "persisted@example.com"
	mgr.SetEmail(testEmail)

	// Add cookies to client's jar
	mgr.Client().SetDeviceID("custom-device-id-123")
	mgr.Client().Jar().LoadCookies([]*accurate.Cookie{
		{
			Name:    "ac_token",
			Value:   "secret-token-val",
			Domain:  "account.accurate.id",
			Path:    "/",
			Expires: time.Now().Add(1 * time.Hour),
		},
	})
	// Trigger persistence via SetEmail or SetTenantSession
	mgr.SetEmail(testEmail)

	tenant := &accurate.TenantSession{
		Host: "zeus.accurate.id",
		DSI:  "test-dsi-token",
		Database: accurate.DatabaseInfo{
			ID:    999,
			Alias: "Persisted DB",
		},
	}
	mgr.SetTenantSession(tenant)
	var storedCookies, storedTenant string
	if err := st.DB().QueryRow(`SELECT cookies,tenant FROM accurate_accounts WHERE id=1`).Scan(&storedCookies, &storedTenant); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-token-val", "test-dsi-token"} {
		if strings.Contains(storedCookies, secret) || strings.Contains(storedTenant, secret) {
			t.Fatalf("plaintext session secret persisted: %s", secret)
		}
	}

	// Verify in-memory status
	status := mgr.Status()
	if !status.Connected || status.Database == nil || status.Database.ID != 999 {
		t.Fatalf("unexpected in-memory status: %+v", status)
	}

	// Now simulate restart: create a new Manager pointing to the same store
	mgr2 := session.NewManager(st, 1, nil)

	if mgr2.Email() != testEmail {
		t.Errorf("expected email %q, got %q", testEmail, mgr2.Email())
	}
	if mgr2.Client().DeviceID() != "custom-device-id-123" {
		t.Errorf("expected deviceID %q, got %q", "custom-device-id-123", mgr2.Client().DeviceID())
	}
	cookies := mgr2.Client().Jar().AllCookies()
	if len(cookies) != 1 || cookies[0].Name != "ac_token" || cookies[0].Value != "secret-token-val" {
		t.Errorf("unexpected restored cookies: %+v", cookies)
	}
	restoredTenant, ok := mgr2.TenantSession()
	if !ok || restoredTenant == nil || restoredTenant.DSI != "test-dsi-token" {
		t.Errorf("tenant session not properly restored: ok=%v, tenant=%+v", ok, restoredTenant)
	}
	status2 := mgr2.Status()
	if !status2.Connected || status2.Database == nil || status2.Database.ID != 999 {
		t.Fatalf("unexpected restored status: %+v", status2)
	}

	// Test Logout clears store
	mgr2.Logout()
	statusCleared := mgr2.Status()
	if statusCleared.Connected || statusCleared.Database != nil {
		t.Errorf("expected disconnected after logout, got %+v", statusCleared)
	}

	// Simulate restart after logout
	mgr3 := session.NewManager(st, 1, nil)
	if mgr3.Email() != testEmail {
		t.Errorf("expected account email %q after logout restart, got %q", testEmail, mgr3.Email())
	}
	if len(mgr3.Client().Jar().AllCookies()) != 0 {
		t.Errorf("expected 0 cookies after logout restart, got %d", len(mgr3.Client().Jar().AllCookies()))
	}
	if _, ok := mgr3.TenantSession(); ok {
		t.Errorf("expected no tenant session after logout restart")
	}
}

func TestSessionManager_InvalidateTenantSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "session_mgr_invalidate_test.db")

	st, err := storage.New(storage.Options{SQLitePath: dbPath, EncryptionKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"})
	if err != nil {
		t.Fatalf("storage.New() failed: %v", err)
	}
	defer st.Close()

	_, _ = st.CreateAccurateAccount(t.Context(), storage.AccurateAccount{Name: "test", Provider: "web", Enabled: true})
	mgr := session.NewManager(st, 1, nil)
	mgr.SetEmail("active@example.com")

	activeTenant := &accurate.TenantSession{
		Host: "zeus.accurate.id",
		DSI:  "active-dsi-token",
		Database: accurate.DatabaseInfo{
			ID:    101,
			Alias: "Active DB",
		},
	}
	mgr.SetTenantSession(activeTenant)

	staleTenant := &accurate.TenantSession{
		Host: "zeus.accurate.id",
		DSI:  "stale-dsi-token",
		Database: accurate.DatabaseInfo{
			ID:    99,
			Alias: "Old DB",
		},
	}

	// 1. Attempt invalidation with nil expected -> should return false and keep active state
	if mgr.InvalidateTenantSession(nil) {
		t.Fatal("expected InvalidateTenantSession(nil) to return false")
	}
	if s := mgr.Status(); !s.Connected || s.Database == nil || s.Database.ID != 101 {
		t.Fatalf("state was modified on nil invalidation: %+v", s)
	}

	// 2. Attempt invalidation with stale tenant pointer -> should return false and keep active state
	if mgr.InvalidateTenantSession(staleTenant) {
		t.Fatal("expected InvalidateTenantSession(staleTenant) to return false")
	}
	if s := mgr.Status(); !s.Connected || s.Database == nil || s.Database.ID != 101 {
		t.Fatalf("state was modified on stale invalidation: %+v", s)
	}
	if mgr.Email() != "active@example.com" {
		t.Fatalf("email was cleared on stale invalidation: %q", mgr.Email())
	}

	// 3. Attempt invalidation with matching active tenant pointer -> should return true and clear state
	if !mgr.InvalidateTenantSession(activeTenant) {
		t.Fatal("expected InvalidateTenantSession(activeTenant) to return true")
	}
	if s := mgr.Status(); s.Connected || s.Database != nil {
		t.Fatalf("expected disconnected status after invalidation, got: %+v", s)
	}
	if mgr.Email() != "active@example.com" {
		t.Fatalf("expected account email to remain after invalidation, got: %q", mgr.Email())
	}
	if _, ok := mgr.TenantSession(); ok {
		t.Fatal("expected no tenant session after invalidation")
	}

	// Verify persistent store was cleared
	mgrRestart := session.NewManager(st, 1, nil)
	if mgrRestart.Status().Connected {
		t.Fatal("expected store to be cleared after invalidation on restart")
	}
	if mgrRestart.Email() != "active@example.com" {
		t.Fatalf("expected account email on restart, got: %q", mgrRestart.Email())
	}
}

func TestSessionManager_OfficialPersistenceWithoutSessionID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "session_mgr_official_test.db")

	st, err := storage.New(storage.Options{SQLitePath: dbPath, EncryptionKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"})
	if err != nil {
		t.Fatalf("storage.New() failed: %v", err)
	}
	defer st.Close()

	offProv := official.New("test-token")
	_, _ = st.CreateAccurateAccount(t.Context(), storage.AccurateAccount{Name: "test", Provider: "official", Enabled: true})
	mgr := session.NewManager(st, 1, offProv)

	// In API Token mode, Session is empty, but Host and Database are populated
	state := &accurate.OfficialSessionState{
		DatabaseID: 3026472,
		Host:       "https://zeus.accurate.id",
		Session:    "", // Empty in API token mode
		Database: accurate.Database{
			ID:    3026472,
			Alias: "Toko Utama",
			Host:  "https://zeus.accurate.id",
		},
	}
	mgr.SetOfficialSession(state)

	// Verify Status().Connected is true even when Session is empty
	s := mgr.Status()
	if !s.Connected || s.Database == nil || s.Database.ID != 3026472 {
		t.Fatalf("expected connected status with empty session ID in API token mode, got: %+v", s)
	}

	// Simulate restart
	offProv2 := official.New("test-token")
	mgr2 := session.NewManager(st, 1, offProv2)

	s2 := mgr2.Status()
	if !s2.Connected || s2.Database == nil || s2.Database.ID != 3026472 {
		t.Fatalf("expected restored connected status on restart, got: %+v", s2)
	}
	if offProv2.Host() != "https://zeus.accurate.id" {
		t.Fatalf("expected provider host to be restored, got %q", offProv2.Host())
	}
}

func TestSessionManager_LogoutReplacesWebProviderClient(t *testing.T) {
	client := accurate.NewClient(nil)
	client.Jar().LoadCookies([]*accurate.Cookie{{
		Name:    "JSESSIONID",
		Value:   "stale-session",
		Domain:  "account.accurate.id",
		Path:    "/",
		Expires: time.Now().Add(time.Hour),
	}})
	provider := web.New("user@example.com", "password", client)
	manager := session.NewManager(nil, 0, provider)

	if manager.Client() != provider.Client() {
		t.Fatal("manager and provider should initially share the web client")
	}
	manager.Logout()

	if manager.Client() != provider.Client() {
		t.Fatal("logout should replace the provider client with the cleared client")
	}
	if manager.Client() == client {
		t.Fatal("logout should not retain the old web client")
	}
	if cookies := provider.Client().Jar().AllCookies(); len(cookies) != 0 {
		t.Fatalf("expected provider client to have no cookies after logout; got %d", len(cookies))
	}
}
