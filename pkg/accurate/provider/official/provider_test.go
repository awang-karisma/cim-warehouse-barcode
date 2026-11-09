package official_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
)

func TestOfficialProvider_DatabaseListAndOpen(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var receivedToken string
	mux.HandleFunc("/api/db-list.do", func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": [
				{"id": 1156, "alias": "PT Demo Example", "logo": ""},
				{"id": 1253, "alias": "PT Contoh", "logo": ""}
			]
		}`))
	})

	mux.HandleFunc("/api/open-db.do", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "1156" {
			http.Error(w, "invalid db id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": ["Proses Berhasil Dilakukan"],
			"session": "test-session-token-xyz",
			"host": "` + server.URL + `"
		}`))
	})

	token := "secret-api-token-abc"
	p := official.New(token, official.WithBaseURL(server.URL), official.WithHTTPClient(server.Client()))

	ctx := context.Background()

	// 1. List databases
	dbs, err := p.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("ListDatabases failed: %v", err)
	}
	if len(dbs) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(dbs))
	}
	if dbs[0].ID != 1156 || dbs[0].Alias != "PT Demo Example" {
		t.Errorf("unexpected database 0: %+v", dbs[0])
	}
	if receivedToken != "Bearer "+token {
		t.Errorf("expected Bearer token %q, got %q", "Bearer "+token, receivedToken)
	}

	// 2. Open database
	db, err := p.OpenDatabase(ctx, 1156)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	if db.ID != 1156 || db.Host != server.URL {
		t.Errorf("unexpected opened database: %+v", db)
	}
	if p.SessionID() != "test-session-token-xyz" {
		t.Errorf("expected session token 'test-session-token-xyz', got %q", p.SessionID())
	}
}

func TestOfficialProvider_SearchItemsAndNormalization(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var gotAuth, gotSession string
	var gotFields, gotKeywords, gotPage, gotPageSize, gotItemType string

	mux.HandleFunc("/accurate/api/item/list.do", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotSession = r.Header.Get("X-Session-ID")
		gotFields = r.URL.Query().Get("fields")
		gotKeywords = r.URL.Query().Get("keywords")
		gotPage = r.URL.Query().Get("sp.page")
		gotPageSize = r.URL.Query().Get("sp.pageSize")
		gotItemType = r.URL.Query().Get("filter.itemType")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"sp": {
				"page": 2,
				"pageCount": 5,
				"pageSize": 20,
				"rowCount": 95
			},
			"d": [
				{
					"id": 1001,
					"no": "ITEM-001",
					"name": "Buku Tulis",
					"upcNo": "8991234567890",
					"unit1Name": "PCS",
					"unit2Name": "Box",
					"unitPrice": "5000",
					"allQuantity": 100.5,
					"availableToSell": 95.0,
					"itemType": "INVENTORY",
					"itemTypeName": "Persediaan"
				},
				{
					"id": 1002,
					"no": "SRV-001",
					"name": "Jasa Instalasi",
					"upcNo": null,
					"unit1Name": "JAM",
					"unitPrice": 50000,
					"allQuantity": 0,
					"availableToSell": 0,
					"itemType": "SERVICE",
					"itemTypeName": "Jasa"
				}
			]
		}`))
	})

	token := "token-12345"
	p := official.New(token, official.WithBaseURL(server.URL), official.WithHTTPClient(server.Client()))
	p.SetSession(server.URL, "sess-67890", accurate.Database{ID: 1})

	res, err := p.SearchItems(context.Background(), "buku", 2, 20)
	if err != nil {
		t.Fatalf("SearchItems failed: %v", err)
	}

	// Verify headers
	if gotAuth != "Bearer "+token {
		t.Errorf("expected Bearer header %q, got %q", "Bearer "+token, gotAuth)
	}
	if gotSession != "sess-67890" {
		t.Errorf("expected X-Session-ID header 'sess-67890', got %q", gotSession)
	}

	// Verify query params
	if gotKeywords != "buku" || gotPage != "2" || gotPageSize != "20" || gotItemType != "INVENTORY" {
		t.Errorf("unexpected query params: kw=%q, page=%q, size=%q, type=%q", gotKeywords, gotPage, gotPageSize, gotItemType)
	}
	if !strings.Contains(gotFields, "id") || !strings.Contains(gotFields, "no") {
		t.Errorf("fields parameter missing standard fields: %s", gotFields)
	}

	// Verify pagination
	if res.Pagination.Page != 2 || res.Pagination.PageCount != 5 || res.Pagination.RowCount != 95 {
		t.Errorf("unexpected pagination: %+v", res.Pagination)
	}

	// Verify items normalization
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res.Items))
	}
	it1 := res.Items[0]
	if it1.ID != 1001 || it1.No != "ITEM-001" || it1.UPCNo != "8991234567890" || it1.Unit1Name != "PCS" || it1.Unit2Name != "Box" {
		t.Errorf("unexpected item 1 fields: %+v", it1)
	}
	if it1.UnitPrice != 5000 || it1.AllQuantity != 100.5 || it1.AvailableToSell != 95.0 {
		t.Errorf("unexpected item 1 numeric fields: %+v", it1)
	}
	if !it1.CanPrintBarcode {
		t.Errorf("expected item 1 CanPrintBarcode to be true for INVENTORY type")
	}

	// Barcode fallback to itemNo when upcNo is null
	it2 := res.Items[1]
	if it2.UPCNo != "SRV-001" {
		t.Errorf("expected UPC fallback to itemNo 'SRV-001', got %q", it2.UPCNo)
	}
	if it2.CanPrintBarcode {
		t.Errorf("expected item 2 CanPrintBarcode to be false for SERVICE type")
	}
}

func TestOfficialProvider_HostRedirect308(t *testing.T) {
	// Setup target host
	targetMux := http.NewServeMux()
	targetServer := httptest.NewServer(targetMux)
	defer targetServer.Close()

	var targetAuth, targetSession string
	targetMux.HandleFunc("/accurate/api/item/list.do", func(w http.ResponseWriter, r *http.Request) {
		targetAuth = r.Header.Get("Authorization")
		targetSession = r.Header.Get("X-Session-ID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"sp": {"page": 1, "pageCount": 1, "pageSize": 50, "rowCount": 1},
			"d": [{"id": 55, "no": "ITM-55", "name": "Item 55", "itemType": "INVENTORY"}]
		}`))
	})

	// Setup initial host that returns 308 Permanent Redirect to target host
	initMux := http.NewServeMux()
	initServer := httptest.NewServer(initMux)
	defer initServer.Close()

	initMux.HandleFunc("/accurate/api/item/list.do", func(w http.ResponseWriter, r *http.Request) {
		redirectURL := targetServer.URL + "/accurate/api/item/list.do"
		if r.URL.RawQuery != "" {
			redirectURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, redirectURL, http.StatusPermanentRedirect) // 308
	})

	token := "my-secret-token"
	p := official.New(token, official.WithBaseURL(initServer.URL))
	p.SetSession(initServer.URL, "my-session-id", accurate.Database{ID: 10})

	res, err := p.SearchItems(context.Background(), "", 1, 50)
	if err != nil {
		t.Fatalf("SearchItems through 308 redirect failed: %v", err)
	}

	if len(res.Items) != 1 || res.Items[0].No != "ITM-55" {
		t.Fatalf("unexpected items after redirect: %+v", res.Items)
	}

	// Verify headers were preserved across cross-host redirect
	if targetAuth != "Bearer "+token {
		t.Errorf("expected preserved Bearer auth after redirect, got %q", targetAuth)
	}
	if targetSession != "my-session-id" {
		t.Errorf("expected preserved X-Session-ID after redirect, got %q", targetSession)
	}
}

func TestOfficialProvider_RejectedTokenAndSession(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/accurate/api/item/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"s": false, "d": ["Token atau Session tidak valid"]}`))
	})

	token := "bad-token"
	p := official.New(token, official.WithBaseURL(server.URL))
	p.SetSession(server.URL, "bad-session", accurate.Database{ID: 1})

	_, err := p.SearchItems(context.Background(), "", 1, 50)
	if err == nil {
		t.Fatal("expected error for unauthorized request, got nil")
	}

	if !errors.Is(err, accurate.ErrSessionExpired) {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}

	// Ensure token is NOT leaked in error string
	if strings.Contains(err.Error(), token) {
		t.Errorf("token %q leaked in error message: %v", token, err)
	}
}

func TestOfficialProvider_UnsuccessfulEnvelope(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/open-db.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": false,
			"errorMsg": "Database sedang dalam pemeliharaan"
		}`))
	})

	token := "token-abc"
	p := official.New(token, official.WithBaseURL(server.URL), official.WithHTTPClient(server.Client()))

	_, err := p.OpenDatabase(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error on s=false envelope, got nil")
	}

	if !strings.Contains(err.Error(), "Database sedang dalam pemeliharaan") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOfficialProvider_MalformedJSON(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/db-list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
	})

	token := "token-abc"
	p := official.New(token, official.WithBaseURL(server.URL), official.WithHTTPClient(server.Client()))

	_, err := p.ListDatabases(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}

	if !strings.Contains(err.Error(), "failed to decode db-list response") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOfficialProvider_GenerateSignatureVector(t *testing.T) {
	// Test vector from Accurate Online documentation: accurate-online-api-token-1.0.3.pdf Page 15
	timestamp := "02/11/2023 09:01:01"
	secret := "31d49b3dc632614495ff8071e5be44a1"
	expectedSig := "8NxvylwwMcjGyzVXK0qbwNvFFuzHpwE9tECllVwLkbo="

	sig := official.GenerateSignature(timestamp, secret)
	if sig != expectedSig {
		t.Fatalf("signature mismatch:\nexpected: %s\ngot:      %s", expectedSig, sig)
	}
}

func TestOfficialProvider_SignatureWithUTCTimestamp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var gotTimestamp, gotSignature string
	mux.HandleFunc("/accurate/api/item/list.do", func(w http.ResponseWriter, r *http.Request) {
		gotTimestamp = r.Header.Get("X-Api-Timestamp")
		gotSignature = r.Header.Get("X-Api-Signature")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"sp": {"page": 1, "pageCount": 1, "pageSize": 50, "rowCount": 1},
			"d": [{"id": 1, "no": "ITM-1", "name": "Test Item", "itemType": "INVENTORY"}]
		}`))
	})

	fixedTime := time.Date(2026, 10, 24, 15, 30, 45, 0, time.UTC)
	secretKey := "31d49b3dc632614495ff8071e5be44a1"
	p := official.New("test-token",
		official.WithBaseURL(server.URL),
		official.WithHTTPClient(server.Client()),
		official.WithSignatureSecret(secretKey),
		official.WithNow(func() time.Time { return fixedTime }),
	)
	p.SetSession(server.URL, "", accurate.Database{ID: 1})

	_, err := p.SearchItems(context.Background(), "", 1, 50)
	if err != nil {
		t.Fatalf("SearchItems failed: %v", err)
	}

	// Assert timestamp is in UTC (ends with 'Z')
	expectedTimestamp := "2026-10-24T15:30:45Z"
	if gotTimestamp != expectedTimestamp {
		t.Errorf("expected UTC timestamp %q, got %q", expectedTimestamp, gotTimestamp)
	}
	if !strings.HasSuffix(gotTimestamp, "Z") {
		t.Errorf("expected UTC timestamp ending in 'Z', got %q", gotTimestamp)
	}

	// Assert signature matches HMAC-SHA256 of the UTC timestamp
	expectedSig := official.GenerateSignature(expectedTimestamp, secretKey)
	if gotSignature != expectedSig {
		t.Errorf("expected signature %q, got %q", expectedSig, gotSignature)
	}
}

func TestOfficialProvider_ApiTokenDiscovery(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var gotAuth, gotTimestamp, gotSignature string
	mux.HandleFunc("/api/api-token.do", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotTimestamp = r.Header.Get("X-Api-Timestamp")
		gotSignature = r.Header.Get("X-Api-Signature")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": {
				"data usaha": {
					"id": 96400,
					"alias": "PT AOL User",
					"host": "` + server.URL + `"
				}
			}
		}`))
	})

	secretKey := "sec-token-xyz"
	fixedTime := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	p := official.New("aat.MTUw.test",
		official.WithBaseURL(server.URL),
		official.WithHTTPClient(server.Client()),
		official.WithSignatureSecret(secretKey),
		official.WithNow(func() time.Time { return fixedTime }),
	)

	dbs, err := p.ListDatabases(context.Background())
	if err != nil {
		t.Fatalf("ListDatabases via /api/api-token.do failed: %v", err)
	}
	if len(dbs) != 1 || dbs[0].ID != 96400 || dbs[0].Alias != "PT AOL User" || dbs[0].Host != server.URL {
		t.Fatalf("unexpected databases: %+v", dbs)
	}

	// Verify request headers
	if gotAuth != "Bearer aat.MTUw.test" {
		t.Errorf("expected Bearer token, got %q", gotAuth)
	}
	if gotTimestamp != "2026-10-24T12:00:00Z" {
		t.Errorf("expected UTC timestamp, got %q", gotTimestamp)
	}
	expectedSig := official.GenerateSignature("2026-10-24T12:00:00Z", secretKey)
	if gotSignature != expectedSig {
		t.Errorf("expected signature %q, got %q", expectedSig, gotSignature)
	}

	// OpenDatabase uses cached host from api-token.do directly
	opened, err := p.OpenDatabase(context.Background(), 96400)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	if opened.Host != server.URL || p.Host() != server.URL {
		t.Errorf("expected host %q, got opened=%q, provider=%q", server.URL, opened.Host, p.Host())
	}
}

func TestOfficialProvider_OpenDatabase_AlreadyIncludesSessionErrorFallback(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// open-db.do returns status 500 with Accurate's exact message:
	// "API Token ini sudah termasuk Sesi Database. Anda tidak perlu melakukan open-db.do lagi."
	mux.HandleFunc("/api/open-db.do", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"s":false,"d":["API Token ini sudah termasuk Sesi Database. Anda tidak perlu melakukan open-db.do lagi."]}`))
	})

	// api-token.do returns the database host with camelCase dataUsaha
	mux.HandleFunc("/api/api-token.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"d": {
				"dataUsaha": {
					"id": 3026472,
					"alias": "Toko Utama",
					"host": "` + server.URL + `"
				}
			}
		}`))
	})

	p := official.New("test-token",
		official.WithBaseURL(server.URL),
		official.WithHTTPClient(server.Client()),
	)

	// Calling OpenDatabase should catch the error and resolve the host via api-token.do
	opened, err := p.OpenDatabase(context.Background(), 3026472)
	if err != nil {
		t.Fatalf("OpenDatabase failed unexpectedly: %v", err)
	}
	if opened.ID != 3026472 || opened.Host != server.URL || p.Host() != server.URL {
		t.Fatalf("unexpected opened database: %+v, host=%s", opened, p.Host())
	}
}
