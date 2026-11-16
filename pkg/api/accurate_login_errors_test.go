package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	webprovider "github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

const loginErrorTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func startAPITCP(t *testing.T, server *api.Server) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.App().Listener(listener) }()
	t.Cleanup(func() {
		_ = server.App().Shutdown()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("API listener: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("API listener did not stop")
		}
	})
	return "http://" + listener.Addr().String()
}

func postJSON(t *testing.T, baseURL, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/accurate/login?accountId=1", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAccurateLoginMFAApiSmokeUsesAccurateProtocol(t *testing.T) {
	var preloginCalls, totpCalls, authCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "mock-session", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/pre-login.do":
			preloginCalls++
			if r.Method != http.MethodPost || r.FormValue("account") != "mfa@example.invalid" {
				t.Errorf("unexpected pre-login request method=%s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"s":true,"d":{"valid":false,"requireTotp":true,"permit":"mock-permit"}}`)
		case "/totp-login.do":
			totpCalls++
			if r.Method != http.MethodPost || r.FormValue("account") != "mfa@example.invalid" || r.FormValue("password") == "" {
				t.Errorf("unexpected TOTP verification request")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"s":true,"d":{"valid":true}}`)
		case "/auth.do":
			authCalls++
			if r.Method != http.MethodPost || r.FormValue("j_username") != "mfa@example.invalid" || r.FormValue("j_password") == "" {
				t.Errorf("unexpected auth request")
			}
			http.Redirect(w, r, "/manage", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ctx := context.Background()
	st, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "login-mfa.db"), EncryptionKey: loginErrorTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	box, err := accounts.NewSecretBox(loginErrorTestKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "MFA", Provider: "web", Enabled: true, Email: "mfa@example.invalid", Password: "stored-secret"}, box.Seal); err != nil {
		t.Fatal(err)
	}
	client := accurate.NewClient(nil)
	client.SetIdPHost(upstream.URL)
	srv := api.New(api.Config{
		Store:     st,
		AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: loginErrorTestKey, RBACCacheTTL: time.Minute},
		PermCache: rbac.NewMemoryCache(time.Minute),
		AccuratePool: accounts.NewPool(st, func(_, email, password, _, _ string) (accurate.Provider, error) {
			return webprovider.New(email, password, client), nil
		}),
	})
	baseURL := startAPITCP(t, srv)
	first := postJSON(t, baseURL, `{}`)
	var challenge struct {
		Status      string `json:"status"`
		ChallengeID string `json:"challengeId"`
	}
	if first.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(first.Body)
		first.Body.Close()
		t.Fatalf("initial MFA response status=%d body=%s", first.StatusCode, body)
	}
	if err := json.NewDecoder(first.Body).Decode(&challenge); err != nil {
		first.Body.Close()
		t.Fatal(err)
	}
	first.Body.Close()
	if challenge.Status != string(accurate.LoginMFARequired) || challenge.ChallengeID == "" {
		t.Fatalf("initial response=%+v", challenge)
	}
	second := postJSON(t, baseURL, `{"challengeId":"`+challenge.ChallengeID+`","otp":"123456"}`)
	defer second.Body.Close()
	body, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatal(err)
	}
	if second.StatusCode != http.StatusOK {
		t.Fatalf("MFA completion status=%d body=%s", second.StatusCode, body)
	}
	if preloginCalls != 1 || totpCalls != 1 || authCalls != 1 {
		t.Fatalf("upstream calls prelogin=%d totp=%d auth=%d", preloginCalls, totpCalls, authCalls)
	}
}

type loginErrorProvider struct{ err error }

func (p loginErrorProvider) ID() string { return "web" }
func (p loginErrorProvider) Login(context.Context, string, string) (accurate.LoginResult, error) {
	return accurate.LoginResult{}, p.err
}
func (loginErrorProvider) ListDatabases(context.Context) ([]accurate.Database, error) {
	return nil, nil
}
func (loginErrorProvider) OpenDatabase(context.Context, int64) (accurate.Database, error) {
	return accurate.Database{}, nil
}
func (loginErrorProvider) SearchItems(context.Context, string, int, int) (accurate.SearchItemsResult, error) {
	return accurate.SearchItemsResult{}, nil
}
func (loginErrorProvider) ListDocuments(context.Context, string, string, string, string, int, int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	return nil, accurate.Pagination{}, nil
}
func (loginErrorProvider) GetDocumentItems(context.Context, string, int64) ([]accurate.NormalizedItem, error) {
	return nil, nil
}
func (loginErrorProvider) SupportsSource(string) (bool, string) { return true, "" }

func TestAccurateLoginMapsProviderFailuresWithoutLeakingDetails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		providerErr error
		status      int
		message     string
	}{
		{name: "pre-login credentials rejected", providerErr: &accurate.LoginError{Stage: accurate.LoginStagePreLogin, Err: accurate.ErrInvalidCredentials}, status: http.StatusUnauthorized, message: "Accurate credentials were rejected during pre-login verification"},
		{name: "authentication redirect rejected", providerErr: &accurate.LoginError{Stage: accurate.LoginStageAuthentication, StatusCode: http.StatusFound, Err: accurate.ErrInvalidCredentials}, status: http.StatusUnauthorized, message: "Accurate authentication was rejected after credential verification"},
		{name: "challenge expired", providerErr: accurate.ErrChallengeExpired, status: http.StatusUnauthorized, message: "Accurate verification challenge expired or was already used. Please start sign-in again."},
		{name: "challenge used", providerErr: accurate.ErrChallengeUsed, status: http.StatusUnauthorized, message: "Accurate verification challenge expired or was already used. Please start sign-in again."},
		{name: "MFA challenge account mismatch", providerErr: &accurate.LoginError{Stage: accurate.LoginStageMFAVerification, Err: accurate.ErrChallengeAccountMismatch}, status: http.StatusUnauthorized, message: "Accurate verification challenge belongs to a different account. Please start sign-in again."},
		{name: "expired session", providerErr: accurate.ErrSessionExpired, status: http.StatusUnauthorized, message: api.ErrAccurateSessionInvalidatedDetail},
		{name: "invalid OTP", providerErr: &accurate.LoginError{Stage: accurate.LoginStageMFAVerification, StatusCode: http.StatusOK, Err: accurate.ErrInvalidOTP}, status: http.StatusUnauthorized, message: "Accurate verification code was rejected"},
		{name: "typed upstream failure", providerErr: &accurate.LoginError{Stage: accurate.LoginStageAuthentication, StatusCode: http.StatusUnauthorized, Err: errors.New("SECRET_COOKIE_VALUE https://idp.example.invalid/auth?token=SECRET_URL_TOKEN")}, status: http.StatusBadGateway, message: "Accurate login failed at authentication (upstream HTTP 401)"},
		{name: "typed parse failure", providerErr: &accurate.LoginError{Stage: accurate.LoginStagePreLogin, Err: errors.New("SECRET_PASSWORD_AND_BODY")}, status: http.StatusBadGateway, message: "Accurate login failed at pre_login"},
		{name: "untyped provider failure", providerErr: errors.New("SECRET_UPSTREAM_BODY"), status: http.StatusBadGateway, message: "Accurate authentication provider failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "login.db"), EncryptionKey: loginErrorTestKey})
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			box, err := accounts.NewSecretBox(loginErrorTestKey)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = st.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "test", Provider: "web", Enabled: true, Email: "user@example.invalid", Password: "stored-secret"}, box.Seal); err != nil {
				t.Fatal(err)
			}
			srv := api.New(api.Config{
				Store:     st,
				AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: loginErrorTestKey, RBACCacheTTL: time.Minute},
				PermCache: rbac.NewMemoryCache(time.Minute),
				AccuratePool: accounts.NewPool(st, func(string, string, string, string, string) (accurate.Provider, error) {
					return loginErrorProvider{err: tc.providerErr}, nil
				}),
			})
			baseURL := startAPITCP(t, srv)
			resp := postJSON(t, baseURL, `{}`)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d body=%s", resp.StatusCode, body)
			}
			if !strings.Contains(string(body), tc.message) {
				t.Fatalf("response %q does not contain safe message %q", body, tc.message)
			}
			for _, secret := range []string{"SECRET_COOKIE_VALUE", "SECRET_PASSWORD_AND_BODY", "SECRET_UPSTREAM_BODY", "SECRET_URL_TOKEN", "https://idp.example.invalid"} {
				if strings.Contains(string(body), secret) {
					t.Fatalf("response leaked sensitive provider detail %q", secret)
				}
			}
		})
	}
}

func TestAccurateLoginApiReportsTypedUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pre-login.do" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "SECRET_UPSTREAM_BODY")
		}
	}))
	defer upstream.Close()
	ctx := context.Background()
	st, err := storage.New(storage.Options{SQLitePath: filepath.Join(t.TempDir(), "login-error.db"), EncryptionKey: loginErrorTestKey})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	box, err := accounts.NewSecretBox(loginErrorTestKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateAccurateAccountSecure(ctx, storage.AccurateAccount{Name: "failure", Provider: "web", Enabled: true, Email: "failure@example.invalid", Password: "stored-secret"}, box.Seal); err != nil {
		t.Fatal(err)
	}
	client := accurate.NewClient(nil)
	client.SetIdPHost(upstream.URL)
	srv := api.New(api.Config{
		Store:     st,
		AppConfig: &config.Config{JWTSecret: "test", AccurateEncryptionKey: loginErrorTestKey, RBACCacheTTL: time.Minute},
		PermCache: rbac.NewMemoryCache(time.Minute),
		AccuratePool: accounts.NewPool(st, func(_, email, password, _, _ string) (accurate.Provider, error) {
			return webprovider.New(email, password, client), nil
		}),
	})
	baseURL := startAPITCP(t, srv)
	resp := postJSON(t, baseURL, `{}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "Accurate login failed at pre_login (upstream HTTP 503)") {
		t.Fatalf("unexpected safe diagnostic: %s", body)
	}
	if strings.Contains(string(body), "SECRET_UPSTREAM_BODY") {
		t.Fatalf("upstream response body leaked: %s", body)
	}
}
