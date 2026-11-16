package accurate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestGetDatabasesRequiresAuthenticatedResponse(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		location string
		body     string
		valid    bool
		expired  bool
	}{
		{"authenticated", 200, "", `<html><script>var contentData={"dbList":[{"id":1001,"alias":"Demo Store","host":"https://zeus.accurate.id"}]};</script></html>`, true, false},
		{"authenticated_empty", 200, "", `<script>var contentData={"dbList":[]};</script>`, true, false},
		{"unauthorized", 401, "", "", false, true},
		{"forbidden", 403, "", "", false, true},
		{"login_redirect", 302, "/login", "", false, true},
		{"root_redirect", 303, "/", "", false, true},
		{"login_form", 200, "", `<html><form action="/auth.do"><input name="j_username"><input name="j_password" type="password"></form></html>`, false, true},
		{"unrelated_redirect", 302, "/maintenance", "", false, false},
		{"untrusted_login_redirect", 302, "https://other.example/login", "", false, false},
		{"server_error", 503, "", "unavailable", false, false},
		{"unknown_html", 200, "", `<html>Maintenance</html>`, false, false},
		{"missing_dbList", 200, "", `<script>var contentData={};</script>`, false, false},
		{"null_dbList", 200, "", `<script>var contentData={"dbList":null};</script>`, false, false},
		{"wrong_dbList", 200, "", `<script>var contentData={"dbList":{}};</script>`, false, false},
		{"malformed_entry", 200, "", `<script>var contentData={"dbList":[{}]};</script>`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/manage" || r.Method != http.MethodGet {
					t.Errorf("unexpected probe %s %s", r.Method, r.URL.Path)
				}
				if tc.location != "" {
					w.Header().Set("Location", tc.location)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			client := accurate.NewClient(nil)
			client.SetIdPHost(upstream.URL)
			_, err := client.GetDatabases(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if errors.Is(err, accurate.ErrSessionExpired) != tc.expired {
				t.Fatalf("expired=%v, error=%v", tc.expired, err)
			}
			if requests != 1 {
				t.Fatalf("made %d requests; probe must not follow redirects or log in", requests)
			}
		})
	}
}
