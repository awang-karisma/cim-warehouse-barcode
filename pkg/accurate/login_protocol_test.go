package accurate_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	webprovider "github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
)

func TestClient_LoginMFAChallengePrecedesValid(t *testing.T) {
	var steps []string
	var challengeDevice string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "fixture-session", Path: "/"})
	})
	mux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// The public browser protocol checks requireTotp before valid. The
		// password alone need not be a valid completed MFA authentication.
		_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false,"requireTotp":true,"permit":"fixture-permit"}}`))
	})
	mux.HandleFunc("/totp-login.do", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		if r.Method != http.MethodPost || r.FormValue("account") != "fixture@example.test" || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Error("unexpected MFA verification request")
		}
		cookie, err := r.Cookie("JSESSIONID")
		if err != nil || cookie.Value != "fixture-session" {
			t.Error("MFA verification did not retain challenge cookies")
		}
		encoded := strings.TrimPrefix(r.FormValue("password"), "ua")
		payload, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Error("invalid fixture MFA encoding")
			return
		}
		var data struct {
			Password string `json:"p"`
			Permit   string `json:"c"`
			OTP      string `json:"t"`
			Device   string `json:"d"`
		}
		if json.Unmarshal(payload, &data) != nil || data.Password != "fixture-password" || data.Permit != "fixture-permit" || data.OTP != "123456" || data.Device != challengeDevice {
			t.Error("MFA verification payload does not match the challenge")
		}
		http.SetCookie(w, &http.Cookie{Name: "MFA_VERIFIED", Value: "fixture-verified", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true}}`))
	})
	mux.HandleFunc("/auth.do", func(w http.ResponseWriter, r *http.Request) {
		steps = append(steps, r.URL.Path)
		if _, err := r.Cookie("MFA_VERIFIED"); err != nil {
			t.Error("authentication did not use verified challenge cookies")
		}
		http.Redirect(w, r, "/manage", http.StatusFound)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()
	client := accurate.NewClient(upstream.Client().Transport)
	client.SetIdPHost(upstream.URL)
	client.SetDeviceID("fixture-device")
	result, err := client.Login(context.Background(), "fixture@example.test", "fixture-password")
	if err != nil {
		t.Fatalf("valid MFA challenge rejected before prompting: %v", err)
	}
	if result.Status != accurate.LoginMFARequired || result.ChallengeID == "" {
		t.Fatal("expected an MFA challenge without premature authentication")
	}
	challengeDevice = client.DeviceID()
	if challengeDevice == "fixture-device" {
		t.Fatal("initial login did not discard previous device state")
	}
	if !reflect.DeepEqual(steps, []string{"/", "/pre-login.do"}) {
		t.Fatalf("unexpected initial login stages: %v", steps)
	}
	if err := client.SubmitMFA(context.Background(), "fixture@example.test", "fixture-password", result.ChallengeID, "123456"); err != nil {
		t.Fatalf("MFA completion failed: %v", err)
	}
	if !reflect.DeepEqual(steps, []string{"/", "/pre-login.do", "/totp-login.do", "/auth.do"}) {
		t.Fatalf("incorrect MFA protocol order: %v", steps)
	}
}

func TestClient_LoginFailuresHaveSafeStageDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		status  int
		body    string
		stage   accurate.LoginStage
		invalid bool
	}{
		{"initial_unavailable", "/", 503, "fixture-secret", accurate.LoginStageInitialPage, false},
		{"prelogin_unavailable", "/pre-login.do", 503, "fixture-secret", accurate.LoginStagePreLogin, false},
		{"prelogin_non_json", "/pre-login.do", 200, "fixture-secret", accurate.LoginStagePreLogin, false},
		{"prelogin_envelope_error", "/pre-login.do", 200, `{"s":false,"d":"fixture-secret"}`, accurate.LoginStagePreLogin, false},
		{"credential_rejection", "/pre-login.do", 200, `{"s":true,"d":{"valid":false,"errorMsg":"fixture-secret"}}`, accurate.LoginStagePreLogin, true},
		{"auth_unavailable", "/auth.do", 503, "fixture-secret", accurate.LoginStageAuthentication, false},
		{"auth_unauthorized", "/auth.do", 401, "fixture-secret", accurate.LoginStageAuthentication, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tc.path {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				if r.URL.Path == "/pre-login.do" {
					_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true,"permit":"fixture-permit"}}`))
				}
			}))
			defer upstream.Close()
			client := accurate.NewClient(upstream.Client().Transport)
			client.SetIdPHost(upstream.URL)
			_, err := client.Login(context.Background(), "fixture@example.test", "fixture-secret")
			var loginErr *accurate.LoginError
			if !errors.As(err, &loginErr) || loginErr.Stage != tc.stage || loginErr.StatusCode != tc.status {
				t.Fatalf("expected stage %s and status %d, got %v", tc.stage, tc.status, err)
			}
			if strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), upstream.URL) {
				t.Fatal("login diagnostic exposed upstream data")
			}
			if errors.Is(err, accurate.ErrInvalidCredentials) != tc.invalid {
				t.Fatal("incorrect credential rejection classification")
			}
		})
	}
}

func TestClient_MFAVerificationFailureStopsAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		invalid bool
	}{
		{"rejected_otp", 200, `{"s":true,"d":{"valid":false}}`, true},
		{"upstream_failure", 503, "fixture-secret", false},
		{"malformed_response", 200, "fixture-secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authCalled := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/pre-login.do":
					_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false,"requireTotp":true,"permit":"fixture-permit"}}`))
				case "/totp-login.do":
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
				case "/auth.do":
					authCalled = true
				}
			}))
			defer upstream.Close()
			client := accurate.NewClient(upstream.Client().Transport)
			client.SetIdPHost(upstream.URL)
			result, err := client.Login(context.Background(), "fixture@example.test", "fixture-secret")
			if err != nil {
				t.Fatal(err)
			}
			err = client.SubmitMFA(context.Background(), "fixture@example.test", "fixture-secret", result.ChallengeID, "123456")
			var loginErr *accurate.LoginError
			if !errors.As(err, &loginErr) || loginErr.Stage != accurate.LoginStageMFAVerification || loginErr.StatusCode != tc.status || errors.Is(err, accurate.ErrInvalidOTP) != tc.invalid {
				t.Fatalf("unexpected MFA verification diagnostic: %v", err)
			}
			if errors.Is(err, accurate.ErrInvalidCredentials) {
				t.Fatal("OTP verification failure was attributed to the account password")
			}
			if authCalled {
				t.Fatal("auth.do called after failed OTP verification")
			}
			if err := client.SubmitMFA(context.Background(), "fixture@example.test", "fixture-secret", result.ChallengeID, "123456"); !errors.Is(err, accurate.ErrChallengeExpired) {
				t.Fatal("failed MFA verification did not consume its single-use challenge")
			}
		})
	}
}

func TestLoginErrorDoesNotRenderUnderlyingSecrets(t *testing.T) {
	cause := errors.New("fixture-password fixture-cookie https://fixture.test/?permit=fixture-permit")
	err := &accurate.LoginError{Stage: accurate.LoginStagePreLogin, StatusCode: 503, Err: cause}
	if err.Error() != "Accurate login failed at pre_login (upstream HTTP 503)" {
		t.Fatal("login error rendered unexpected diagnostic content")
	}
	if !errors.Is(err, cause) {
		t.Fatal("login error lost its internal cause")
	}
}

func TestWebProviderLoginMatchesPublicBrowserCredentials(t *testing.T) {
	// Exact public JavaScript output for the synthetic password café£ÿ.
	const pre = "upeyJ2IjoxLCJwIjoiY2Fm6aP/IiwiZCI6ImZpeHR1cmUtZGV2aWNlIn0="
	const direct = "uaeyJ2IjoxLCJwIjoiY2Fm6aP/IiwiYyI6ImZpeHR1cmUtcGVybWl0IiwidCI6bnVsbCwiZCI6ImZpeHR1cmUtZGV2aWNlIn0="
	const verified = "uaeyJ2IjoxLCJwIjoiY2Fm6aP/IiwiYyI6ImZpeHR1cmUtcGVybWl0IiwidCI6IjEyMzQ1NiIsImQiOiJmaXh0dXJlLWRldmljZSJ9"
	for _, mfa := range []bool{false, true} {
		name := "direct"
		if mfa {
			name = "MFA"
		}
		t.Run(name, func(t *testing.T) {
			var steps []string
			var client *accurate.Client
			// Keep the public JavaScript fixture bytes, substituting only the
			// newly generated browser device identity for each initial Connect.
			expected := func(encoded string) string {
				payload, err := base64.StdEncoding.DecodeString(encoded[2:])
				if err != nil {
					t.Fatal(err)
				}
				payload = []byte(strings.ReplaceAll(string(payload), "fixture-device", client.DeviceID()))
				return encoded[:2] + base64.StdEncoding.EncodeToString(payload)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				steps = append(steps, r.URL.Path)
				if r.URL.Path != "/" {
					if cookie, err := r.Cookie("JSESSIONID"); err != nil || cookie.Value != "fixture-session" {
						t.Error("login did not preserve initial browser session")
					}
				}
				switch r.URL.Path {
				case "/":
					http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "fixture-session", Path: "/"})
				case "/pre-login.do":
					if r.FormValue("account") != "fixture@example.test" || r.FormValue("password") != expected(pre) {
						t.Error("pre-login request differs from public JavaScript output")
						_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false}}`))
						return
					}
					if mfa {
						_, _ = w.Write([]byte(`{"s":true,"d":{"valid":false,"requireTotp":true,"permit":"fixture-permit"}}`))
					} else {
						_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true,"permit":"fixture-permit"}}`))
					}
				case "/totp-login.do":
					if !mfa || r.FormValue("account") != "fixture@example.test" || r.FormValue("password") != expected(verified) {
						t.Error("MFA verification differs from public JavaScript output")
					}
					http.SetCookie(w, &http.Cookie{Name: "MFA_VERIFIED", Value: "fixture-verified", Path: "/"})
					_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true}}`))
				case "/auth.do":
					want := direct
					if mfa {
						want = verified
						if _, err := r.Cookie("MFA_VERIFIED"); err != nil {
							t.Error("authentication lost verified challenge cookie")
						}
					}
					if r.FormValue("j_username") != "fixture@example.test" || r.FormValue("j_password") != expected(want) {
						t.Error("authentication request differs from public JavaScript output")
					}
					http.SetCookie(w, &http.Cookie{Name: "AUTHENTICATED", Value: "fixture-authenticated", Path: "/"})
					http.Redirect(w, r, "/manage", http.StatusFound)
				case "/manage":
					if _, err := r.Cookie("AUTHENTICATED"); err != nil {
						t.Error("database discovery did not retain authenticated session")
					}
					_, _ = w.Write([]byte(`<script>var contentData = {"dbList":[{"id":1,"alias":"fixture","host":"https://fixture.accurate.id"}]};</script>`))
				default:
					t.Error("unexpected upstream request")
				}
			}))
			defer upstream.Close()
			client = accurate.NewClient(upstream.Client().Transport)
			client.SetIdPHost(upstream.URL)
			client.SetDeviceID("fixture-device")
			provider := webprovider.New("fixture@example.test", "caf\u00e9\u00a3\u00ff", client)
			result, err := provider.Login(context.Background(), "", "")
			if err != nil {
				t.Fatalf("provider login failed: %v", err)
			}
			if mfa {
				if result.Status != accurate.LoginMFARequired || result.ChallengeID == "" {
					t.Fatal("expected pending MFA challenge")
				}
				result, err = provider.Login(context.Background(), result.ChallengeID, "123456")
				if err != nil {
					t.Fatalf("provider MFA completion failed: %v", err)
				}
			}
			if result.Status != accurate.LoginSuccess {
				t.Fatal("provider did not authenticate")
			}
			dbs, err := provider.ListDatabases(context.Background())
			if err != nil || len(dbs) != 1 || dbs[0].ID != 1 {
				t.Fatalf("authenticated database discovery failed: %v", err)
			}
			wantSteps := []string{"/", "/pre-login.do", "/auth.do", "/manage"}
			if mfa {
				wantSteps = []string{"/", "/pre-login.do", "/totp-login.do", "/auth.do", "/manage"}
			}
			if !reflect.DeepEqual(steps, wantSteps) {
				t.Fatalf("unexpected runtime protocol: %v", steps)
			}
		})
	}
}

func TestClientMFAChallengeAccountMismatchIsNotPasswordRejection(t *testing.T) {
	client := accurate.NewClient(nil)
	client.Challenges().Put(&accurate.PendingChallenge{
		ChallengeID: "fixture-challenge", Email: "owner@example.test",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	err := client.SubmitMFA(context.Background(), "other@example.test", "fixture-password", "fixture-challenge", "123456")
	var loginErr *accurate.LoginError
	if !errors.Is(err, accurate.ErrChallengeAccountMismatch) || errors.Is(err, accurate.ErrInvalidCredentials) || !errors.As(err, &loginErr) || loginErr.Stage != accurate.LoginStageMFAVerification {
		t.Fatalf("incorrect challenge account diagnostic: %v", err)
	}
	if _, err := client.Challenges().Take("fixture-challenge"); !errors.Is(err, accurate.ErrChallengeExpired) {
		t.Fatal("mismatched challenge was not consumed")
	}
}
