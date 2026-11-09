package accurate_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestCookieJar_DomainAndPathScoping(t *testing.T) {
	jar := accurate.NewMemoryJar()

	idpURL, _ := url.Parse("https://account.accurate.id/")
	spURL, _ := url.Parse("https://zeus.accurate.id/accurate/open.do")

	// Set cookie on IdP
	jar.SetCookies(idpURL, []*http.Cookie{
		{
			Name:  "JSESSIONID",
			Value: "IDP_SESSION_123",
			Path:  "/",
		},
	})

	// Set cookie on SP
	jar.SetCookies(spURL, []*http.Cookie{
		{
			Name:  "JSESSIONID",
			Value: "SP_SESSION_456",
			Path:  "/accurate",
		},
	})

	// 1. Verify IdP cookies
	idpCookies := jar.Cookies(idpURL)
	if len(idpCookies) != 1 || idpCookies[0].Value != "IDP_SESSION_123" {
		t.Fatalf("Expected IDP_SESSION_123, got: %+v", idpCookies)
	}

	// 2. Verify SP cookies - IdP cookie must NEVER overwrite or leak to SP
	spCookies := jar.Cookies(spURL)
	if len(spCookies) != 1 || spCookies[0].Value != "SP_SESSION_456" {
		t.Fatalf("Expected SP_SESSION_456, got: %+v", spCookies)
	}

	// 3. Verify SP root does not get /accurate scoped cookie
	spRootURL, _ := url.Parse("https://zeus.accurate.id/")
	spRootCookies := jar.Cookies(spRootURL)
	if len(spRootCookies) != 0 {
		t.Fatalf("Expected 0 cookies on SP root, got: %+v", spRootCookies)
	}
}

func TestCookieJar_Expiry(t *testing.T) {
	jar := accurate.NewMemoryJar()
	u, _ := url.Parse("https://account.accurate.id/")

	// Set expired cookie
	jar.SetCookies(u, []*http.Cookie{
		{
			Name:    "EXPIRED_COOKIE",
			Value:   "VAL",
			Path:    "/",
			Expires: time.Now().Add(-1 * time.Hour),
		},
		{
			Name:    "VALID_COOKIE",
			Value:   "VAL",
			Path:    "/",
			Expires: time.Now().Add(1 * time.Hour),
		},
	})

	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "VALID_COOKIE" {
		t.Fatalf("Expected only VALID_COOKIE, got: %+v", cookies)
	}
}

func TestCookieJar_CloneAndLoad(t *testing.T) {
	jar := accurate.NewMemoryJar()
	u, _ := url.Parse("https://account.accurate.id/")

	jar.SetCookies(u, []*http.Cookie{
		{Name: "C1", Value: "V1", Path: "/"},
	})

	clone := jar.Clone()

	// Mutate clone
	clone.SetCookies(u, []*http.Cookie{
		{Name: "C2", Value: "V2", Path: "/"},
	})

	// Original must not have C2
	if len(jar.Cookies(u)) != 1 {
		t.Fatalf("Original jar was mutated by clone modification")
	}

	// Clone must have both
	if len(clone.Cookies(u)) != 2 {
		t.Fatalf("Clone did not retain original + new cookies")
	}
}

func TestCookieJar_HasAndClearHost(t *testing.T) {
	jar := accurate.NewMemoryJar()
	idpURL, _ := url.Parse("https://account.accurate.id/")
	spURL, _ := url.Parse("https://zeus.accurate.id/accurate")

	jar.SetCookies(idpURL, []*http.Cookie{
		{Name: "JSESSIONID", Value: "IDP_VAL", Path: "/"},
	})
	jar.SetCookies(spURL, []*http.Cookie{
		{Name: "JSESSIONID", Value: "SP_VAL", Path: "/accurate"},
	})

	if !jar.HasCookiesForHost("https://zeus.accurate.id") {
		t.Fatalf("expected HasCookiesForHost to be true for zeus.accurate.id")
	}
	if !jar.HasCookiesForHost("https://account.accurate.id") {
		t.Fatalf("expected HasCookiesForHost to be true for account.accurate.id")
	}
	if jar.HasCookiesForHost("https://other.accurate.id") {
		t.Fatalf("expected HasCookiesForHost to be false for other.accurate.id")
	}

	// Clear zeus.accurate.id
	jar.ClearHost("https://zeus.accurate.id")

	if jar.HasCookiesForHost("https://zeus.accurate.id") {
		t.Fatalf("expected HasCookiesForHost to be false after ClearHost")
	}
	// account.accurate.id must still remain
	if !jar.HasCookiesForHost("https://account.accurate.id") {
		t.Fatalf("expected account.accurate.id cookies to remain untouched")
	}
}
