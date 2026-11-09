package accurate_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestClient_LoginSuccess(t *testing.T) {
	// Setup Mock IdP Server
	idpMux := http.NewServeMux()
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	idpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: "MOCK_IDP_JSESSIONID",
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>IdP Login Page</html>"))
	})

	idpMux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": {
				"valid": true,
				"requireTotp": false,
				"permit": "PERMIT_TOKEN_ABC",
				"errorCode": 0
			}
		}`))
	})

	idpMux.HandleFunc("/auth.do", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		username := r.FormValue("j_username")
		password := r.FormValue("j_password")
		if username != "test@accurate.id" || !strings.HasPrefix(password, "ua") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/manage", http.StatusFound)
	})

	idpMux.HandleFunc("/manage", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("JSESSIONID")
		if err != nil || cookie.Value != "MOCK_IDP_JSESSIONID" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
			<html>
			<script>
				var contentData = {
					"dbList": [
						{
							"id": 1001,
							"alias": "Demo Store",
							"host": "` + idpServer.URL + `",
							"uniqueId": "uid-1001",
							"isPrivateCloud": false
						}
					]
				};
			</script>
			</html>
		`))
	})

	client := accurate.NewClient(idpServer.Client().Transport)
	client.SetIdPHost(idpServer.URL)

	ctx := context.Background()
	res, err := client.Login(ctx, "test@accurate.id", "SecretPass123!")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if res.Status != accurate.LoginSuccess {
		t.Fatalf("Expected status success, got: %s", res.Status)
	}

	// Fetch databases
	dbs, err := client.GetDatabases(ctx)
	if err != nil {
		t.Fatalf("GetDatabases failed: %v", err)
	}
	if len(dbs) != 1 || dbs[0].Alias != "Demo Store" {
		t.Fatalf("Unexpected databases: %+v", dbs)
	}
}

func TestClient_LoginWithTOTP(t *testing.T) {
	idpMux := http.NewServeMux()
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	idpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: "MOCK_TOTP_JSESSIONID",
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
	})

	idpMux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": {
				"valid": true,
				"requireTotp": true,
				"permit": "PERMIT_TOTP_123",
				"errorCode": 0
			}
		}`))
	})

	idpMux.HandleFunc("/auth.do", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		pw := r.FormValue("j_password")
		if !strings.HasPrefix(pw, "ua") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/manage", http.StatusFound)
	})

	client := accurate.NewClient(idpServer.Client().Transport)
	client.SetIdPHost(idpServer.URL)

	ctx := context.Background()
	res, err := client.Login(ctx, "totp@accurate.id", "SecretPass123!")
	if err != nil {
		t.Fatalf("Initial login failed: %v", err)
	}

	if res.Status != accurate.LoginMFARequired || res.ChallengeID == "" {
		t.Fatalf("Expected mfa_required with challengeId, got: %+v", res)
	}

	// Submit correct OTP with challenge ID
	err = client.SubmitMFA(ctx, "totp@accurate.id", "SecretPass123!", res.ChallengeID, "123456")
	if err != nil {
		t.Fatalf("SubmitMFA failed: %v", err)
	}

	// Challenge must be single-use: second attempt must fail
	err2 := client.SubmitMFA(ctx, "totp@accurate.id", "SecretPass123!", res.ChallengeID, "123456")
	if err2 == nil {
		t.Fatalf("Expected error when reusing challengeId")
	}
}

func TestClient_OpenDatabaseAndSAML(t *testing.T) {
	// We need two servers: IdP and SP (database host)
	idpMux := http.NewServeMux()
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	spMux := http.NewServeMux()
	spServer := httptest.NewServer(spMux)
	defer spServer.Close()

	// 1. Initial SP open.do triggers SAML
	spMux.HandleFunc("/accurate/open.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(`
			<html>
			<form action="%s/idp/sso" method="post">
				<input type="hidden" name="SAMLRequest" value="MOCK_SAML_REQUEST" />
				<input type="hidden" name="RelayState" value="%s/accurate" />
			</form>
			</html>
		`, idpServer.URL, spServer.URL)))
	})

	// 2. IdP accepts SAMLRequest and returns SAMLResponse form
	idpMux.HandleFunc("/idp/sso", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		req := r.FormValue("SAMLRequest")
		if req != "MOCK_SAML_REQUEST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(`
			<html>
			<form action="%s/accurate/saml/SSO" method="post">
				<input type="hidden" name="SAMLResponse" value="MOCK_SAML_RESPONSE" />
				<input type="hidden" name="RelayState" value="%s/accurate" />
			</form>
			</html>
		`, spServer.URL, spServer.URL)))
	})

	// 3. SP receives SAMLResponse and issues host JSESSIONID
	spMux.HandleFunc("/accurate/saml/SSO", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		resp := r.FormValue("SAMLResponse")
		if resp != "MOCK_SAML_RESPONSE" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: "SP_HOST_SESSION_TOKEN",
			Path:  "/accurate",
		})
		http.Redirect(w, r, spServer.URL+"/accurate/open.do?uid=uid-1001&product=aol", http.StatusFound)
	})

	// 4. open-database.do returns DSI
	spMux.HandleFunc("/accurate/open-database.do", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("JSESSIONID")
		if err != nil || cookie.Value != "SP_HOST_SESSION_TOKEN" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"dsi": "MOCK_DSI_TOKEN_XYZ"
		}`))
	})

	client := accurate.NewClient(http.DefaultTransport)
	client.SetIdPHost(idpServer.URL)

	dbInfo := accurate.DatabaseInfo{
		ID:       1001,
		Alias:    "Demo Store",
		Host:     spServer.URL,
		UniqueID: "uid-1001",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := client.OpenDatabase(ctx, dbInfo)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}

	if sess.DSI != "MOCK_DSI_TOKEN_XYZ" || sess.Host != spServer.URL {
		t.Fatalf("Unexpected session: %+v", sess)
	}

	// Second OpenDatabase call on same client must succeed via direct open-database.do without failing on SAML form
	sess2, err := client.OpenDatabase(ctx, dbInfo)
	if err != nil {
		t.Fatalf("Second OpenDatabase call failed: %v", err)
	}
	if sess2.DSI != "MOCK_DSI_TOKEN_XYZ" {
		t.Fatalf("Unexpected DSI in second call: %s", sess2.DSI)
	}
}

func TestClient_PostTenant_RedirectIsSessionExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer server.Close()

	client := accurate.NewClient(server.Client().Transport)
	sess := &accurate.TenantSession{
		Host: server.URL,
		DSI:  "TEST_DSI",
	}

	_, err := client.PostTenant(context.Background(), sess, "/accurate/api/item/list.do", url.Values{})
	if err == nil {
		t.Fatal("expected error for 302 redirect, got nil")
	}
	if !errors.Is(err, accurate.ErrSessionExpired) {
		t.Fatalf("expected errors.Is(err, ErrSessionExpired) to be true, got err: %v", err)
	}
}

func TestClient_PostTenant_Non302FailureDoesNotWrapSessionExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := accurate.NewClient(server.Client().Transport)
	sess := &accurate.TenantSession{
		Host: server.URL,
		DSI:  "TEST_DSI",
	}

	_, err := client.PostTenant(context.Background(), sess, "/accurate/api/item/list.do", url.Values{})
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
	if errors.Is(err, accurate.ErrSessionExpired) {
		t.Fatalf("expected errors.Is(err, ErrSessionExpired) to be false for 500, got err: %v", err)
	}
}

func TestClient_LoginEmptyPreLoginResponseIncludesHTTPContext(t *testing.T) {
	idpMux := http.NewServeMux()
	idpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	idpMux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	client := accurate.NewClient(idpServer.Client().Transport)
	client.SetIdPHost(idpServer.URL)
	_, err := client.Login(context.Background(), "person@example.com", "secret")
	if err == nil {
		t.Fatal("expected empty pre-login response to fail")
	}
	if !strings.Contains(err.Error(), "status 503") || !strings.Contains(err.Error(), `content-type "text/html"`) {
		t.Fatalf("expected status and content type in error, got: %v", err)
	}
}

func TestClient_LoginFollowsSameHostRedirects(t *testing.T) {
	idpMux := http.NewServeMux()
	idpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login-page", http.StatusFound)
	})
	idpMux.HandleFunc("/login-page", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("initial redirect method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	idpMux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("pre-login method = %s, want POST", r.Method)
		}
		http.Redirect(w, r, "/pre-login-result", http.StatusFound)
	})
	idpMux.HandleFunc("/pre-login-result", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("302 pre-login redirect method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"valid":true,"permit":"permit"}}`))
	})
	idpMux.HandleFunc("/auth.do", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	client := accurate.NewClient(idpServer.Client().Transport)
	client.SetIdPHost(idpServer.URL)
	if _, err := client.Login(context.Background(), "person@example.com", "secret"); err != nil {
		t.Fatalf("Login() after same-host redirects failed: %v", err)
	}
}

func TestClient_LoginRejectsCrossHostRedirect(t *testing.T) {
	crossHostCalled := false
	crossHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		crossHostCalled = true
	}))
	defer crossHost.Close()

	idpMux := http.NewServeMux()
	idpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	idpMux.HandleFunc("/pre-login.do", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, crossHost.URL+"/capture", http.StatusFound)
	})
	idpServer := httptest.NewServer(idpMux)
	defer idpServer.Close()

	client := accurate.NewClient(idpServer.Client().Transport)
	client.SetIdPHost(idpServer.URL)
	_, err := client.Login(context.Background(), "person@example.com", "secret")
	if !errors.Is(err, accurate.ErrUntrustedHost) {
		t.Fatalf("expected untrusted-host redirect error, got: %v", err)
	}
	if crossHostCalled {
		t.Fatal("client followed a redirect to a different host")
	}
}
