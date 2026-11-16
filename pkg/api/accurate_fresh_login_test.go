package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestAccurateInitialConnectClearsSessionBeforePreLogin(t *testing.T) {
	var steps []string
	var pageLoads int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		switch r.URL.Path {
		case "/":
			pageLoads++
			if len(r.Cookies()) != 0 {
				t.Error("fresh Connect sent stored or failed-attempt cookies")
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "fresh", Path: "/"})
		case "/pre-login.do":
			cookie, err := r.Cookie("JSESSIONID")
			if err != nil || cookie.Value != "fresh" {
				t.Error("pre-login did not use fresh initial-page cookie")
			}
			w.Header().Set("Content-Type", "application/json")
			if pageLoads == 1 {
				_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false}}`))
			} else {
				_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true,"permit":"fixture-permit"}}`))
			}
		case "/auth.do":
			http.Redirect(w, r, "/manage", http.StatusFound)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	store, pool, server := webSessionFixture(t, upstream.URL)
	entry, err := pool.Resolve(1, "web", "test@accurate.id", "SecretPass123!", "", "")
	if err != nil {
		t.Fatal(err)
	}
	configuredClient := entry.Session.Client()
	configuredClient.Challenges().Put(&accurate.PendingChallenge{ChallengeID: "stale", ExpiresAt: time.Now().Add(time.Minute)})
	for attempt, body := range []string{`{"challengeId":"stale"}`, `{}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accurate/login?accountId=1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := server.App().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := http.StatusOK
		if attempt == 0 {
			want = http.StatusUnauthorized
		}
		if resp.StatusCode != want {
			t.Fatalf("Connect attempt %d status=%d, want %d", attempt+1, resp.StatusCode, want)
		}
		_, _, _, cookies, tenant, _, err := store.LoadAccurateSession(1)
		if err != nil || tenant != nil || entry.Session.Status().Connected {
			t.Fatalf("Connect retained old tenant: err=%v", err)
		}
		if _, ok := entry.Provider.(interface {
			TenantSession() (*accurate.TenantSession, bool)
		}).TenantSession(); ok {
			t.Fatal("provider retained old tenant")
		}
		if attempt == 0 && (len(cookies) != 0 || len(configuredClient.Jar().AllCookies()) != 0) {
			t.Fatal("rejected Connect retained authentication cookies")
		}
		if entry.Session.Client() != configuredClient {
			t.Fatal("Connect discarded configured transport/IdP host")
		}
	}
	if _, err := configuredClient.Challenges().Take("stale"); !errors.Is(err, accurate.ErrChallengeExpired) {
		t.Fatal("fresh Connect retained old challenge")
	}
	if want := []string{"/", "/pre-login.do", "/", "/pre-login.do", "/auth.do"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("request order=%v, want %v", steps, want)
	}
}
