package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
)

func TestWebProvider_Basics(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/accurate/inventory/search-item.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"sp": {"page": 1, "pageCount": 1, "pageSize": 50, "rowCount": 1},
			"d": [{"id": 1, "no": "W-01", "name": "Web Item", "upcNo": "12345", "unit1Name": "PCS", "unitPrice": 1000, "allQuantity": 10, "availableToSell": 10, "itemType": "INVENTORY", "itemTypeName": "Persediaan"}]
		}`))
	})

	c := accurate.NewClient(server.Client().Transport)
	p := web.New("user@example.com", "pass", c)

	if p.ID() != "web" {
		t.Errorf("expected ID 'web', got %q", p.ID())
	}

	ok, _ := p.SupportsSource("sales-order")
	if !ok {
		t.Errorf("expected sales-order supported")
	}
	ok, _ = p.SupportsSource("unknown-src")
	if ok {
		t.Errorf("expected unknown-src not supported")
	}

	// Setting tenant session
	sess := &accurate.TenantSession{
		Host: server.URL,
		DSI:  "DSI-123",
	}
	p.SetTenantSession(sess)

	res, err := p.SearchItems(context.Background(), "", 1, 50)
	if err != nil {
		t.Fatalf("SearchItems failed: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].No != "W-01" {
		t.Fatalf("unexpected items: %+v", res.Items)
	}
}
