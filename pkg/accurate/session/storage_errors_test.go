package session_test

import (
	"errors"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
)

type clearFailureStore struct {
	tenant  *accurate.TenantSession
	failure error
	clears  int
}

func (s *clearFailureStore) LoadAccurateSession(int64) (string, string, string, []*accurate.Cookie, *accurate.TenantSession, *accurate.OfficialSessionState, error) {
	return "web", "fixture@example.test", "device", []*accurate.Cookie{{Name: "JSESSIONID", Value: "fixture", Domain: "account.accurate.id", Path: "/"}}, s.tenant, nil, nil
}
func (*clearFailureStore) SaveAccurateSession(int64, string, string, string, *accurate.MemoryJar, *accurate.TenantSession, *accurate.OfficialSessionState) error {
	return nil
}
func (s *clearFailureStore) ClearAccurateSession(int64) error {
	s.clears++
	return s.failure
}

func TestInvalidateTenantSessionPersistenceFailureStillClearsRuntime(t *testing.T) {
	failure := errors.New("storage unavailable")
	store := &clearFailureStore{tenant: &accurate.TenantSession{DSI: "fixture-dsi"}, failure: failure}
	provider := web.New("fixture@example.test", "fixture-password", nil)
	manager, err := session.NewManager(store, 1, provider)
	if err != nil {
		t.Fatal(err)
	}
	if invalidated, err := manager.InvalidateTenantSession(&accurate.TenantSession{DSI: "stale"}); invalidated || err != nil || store.clears != 0 {
		t.Fatalf("stale invalidation touched persistence: invalidated=%v err=%v clears=%d", invalidated, err, store.clears)
	}
	if invalidated, err := manager.InvalidateTenantSession(store.tenant); !invalidated || !errors.Is(err, failure) {
		t.Fatalf("storage failure lost: invalidated=%v err=%v", invalidated, err)
	}
	if manager.Status().Connected || len(manager.Client().Jar().AllCookies()) != 0 {
		t.Fatal("clear failure retained runtime authentication")
	}
	if _, ok := provider.TenantSession(); ok {
		t.Fatal("clear failure retained provider tenant")
	}
	if invalidated, err := manager.InvalidateTenantSession(store.tenant); invalidated || err != nil || store.clears != 1 {
		t.Fatalf("cleared tenant was revived: invalidated=%v err=%v clears=%d", invalidated, err, store.clears)
	}
	if err := manager.Logout(); !errors.Is(err, failure) || store.clears != 2 {
		t.Fatalf("explicit logout could not retry persistence: %v", err)
	}
}
