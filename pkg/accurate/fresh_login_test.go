package accurate_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestClient_InitialLoginDiscardsStaleAuthentication(t *testing.T) {
	var steps []string
	var attempts int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		switch r.URL.Path {
		case "/":
			attempts++
			if len(r.Cookies()) != 0 {
				t.Error("initial page received stale authentication cookies")
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: fmt.Sprintf("attempt-%d", attempts), Path: "/"})
		case "/pre-login.do":
			cookie, err := r.Cookie("JSESSIONID")
			if err != nil || cookie.Value != fmt.Sprintf("attempt-%d", attempts) {
				t.Error("pre-login did not use the current page's session")
			}
			w.Header().Set("Content-Type", "application/json")
			if attempts == 1 {
				_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false}}`))
			} else {
				_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true,"permit":"fixture-permit"}}`))
			}
		case "/auth.do":
			http.Redirect(w, r, "/manage", http.StatusFound)
		default:
			t.Errorf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	client := accurate.NewClient(upstream.Client().Transport)
	client.SetIdPHost(upstream.URL)
	client.SetDeviceID("stale-device")
	u, _ := url.Parse(upstream.URL)
	client.Jar().SetCookies(u, []*http.Cookie{{Name: "JSESSIONID", Value: "stale", Path: "/"}, {Name: "TENANT", Value: "stale", Path: "/"}})
	client.Challenges().Put(&accurate.PendingChallenge{ChallengeID: "stale-challenge", ExpiresAt: time.Now().Add(time.Minute)})
	if _, err := client.Login(context.Background(), "fixture@example.test", "fixture-password"); !errors.Is(err, accurate.ErrInvalidCredentials) {
		t.Fatalf("expected fixture pre-login rejection, got %v", err)
	}
	if len(client.Jar().AllCookies()) != 0 {
		t.Fatal("failed login retained authentication cookies")
	}
	if client.DeviceID() == "stale-device" {
		t.Fatal("fresh login reused stale device state")
	}
	if _, err := client.Challenges().Take("stale-challenge"); !errors.Is(err, accurate.ErrChallengeExpired) {
		t.Fatal("fresh login retained a stale MFA challenge")
	}
	result, err := client.Login(context.Background(), "fixture@example.test", "fixture-password")
	if err != nil || result.Status != accurate.LoginSuccess {
		t.Fatalf("fresh retry failed: result=%v err=%v", result, err)
	}
	if want := []string{"/", "/pre-login.do", "/", "/pre-login.do", "/auth.do"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("request order=%v, want %v", steps, want)
	}
}
