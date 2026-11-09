package adapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/adapter"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
)

func TestSearchItems(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/accurate/inventory/search-item.do", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("_dsi") != "TEST_DSI" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"s": true,
			"sp": {
				"page": 1,
				"pageCount": 1,
				"pageSize": 50,
				"rowCount": 2
			},
			"d": [
				{
					"id": 101,
					"no": "ITEM-001",
					"name": "Buku Tulis",
					"upcNo": "8991234567890",
					"unit1Name": "PCS",
					"unitPrice": 5000,
					"allQuantity": 100,
					"availableToSell": 95,
					"itemType": "INVENTORY",
					"itemTypeName": "Persediaan"
				},
				{
					"id": 102,
					"no": "SRV-001",
					"name": "Jasa Servis",
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

	client := accurate.NewClient(server.Client().Transport)
	webProv := web.New("test@example.com", "pass", client)
	session := &accurate.TenantSession{
		Host: server.URL,
		DSI:  "TEST_DSI",
	}
	webProv.SetTenantSession(session)

	res, err := adapter.SearchItems(context.Background(), webProv, "buku", 1, 50)
	if err != nil {
		t.Fatalf("SearchItems failed: %v", err)
	}

	if len(res.Items) != 2 {
		t.Fatalf("Expected 2 items, got: %d", len(res.Items))
	}

	item1 := res.Items[0]
	if item1.No != "ITEM-001" || item1.UPCNo != "8991234567890" || !item1.CanPrintBarcode {
		t.Fatalf("Item 1 mismatch: %+v", item1)
	}

	item2 := res.Items[1]
	if item2.CanPrintBarcode {
		t.Fatalf("SERVICE item should not be allowed for barcode printing: %+v", item2)
	}
}

func TestAdapters_AllSources_Web(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Receive Item
	mux.HandleFunc("/accurate/vendor/search-receive-item.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":1,"receiveNumber":"RI-001","transDate":"01/01/2026","statusName":"Selesai","description":"Penerimaan 1"}]}`))
	})
	mux.HandleFunc("/accurate/vendor/view-receive-item.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"RI-001","detailItem":[{"quantity":10,"unitPrice":15000,"item":{"id":11,"no":"RI-ITM","name":"Barang RI","upcNo":"111222"},"itemUnit":{"name":"Box"}}]}}`))
	})

	// 2. Purchase Order
	mux.HandleFunc("/accurate/vendor/search-purchase-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":2,"number":"PO-001","transDate":"02/01/2026","statusName":"Disetujui","description":"PO 1"}]}`))
	})
	mux.HandleFunc("/accurate/vendor/view-purchase-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"PO-001","detailItem":[{"quantity":5,"unitPrice":20000,"item":{"id":22,"no":"PO-ITM","name":"Barang PO","upcNo":null},"itemUnit":{"name":"Unit"}}]}}`))
	})

	// 3. Sales Order
	mux.HandleFunc("/accurate/customer/search-sales-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":3,"number":"SO-001","transDate":"03/01/2026","statusName":"Proses","description":"SO 1"}]}`))
	})
	mux.HandleFunc("/accurate/customer/view-sales-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SO-001","detailItem":[{"quantity":2,"unitPrice":50000,"item":{"id":33,"no":"SO-ITM","name":"Barang SO","upcNo":"333444"},"itemUnit":{"name":"Pcs"}}]}}`))
	})

	// 4. Delivery Order
	mux.HandleFunc("/accurate/customer/search-delivery-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":4,"number":"DO-001","transDate":"04/01/2026","statusName":"Terkirim","description":"DO 1"}]}`))
	})
	mux.HandleFunc("/accurate/customer/view-delivery-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"DO-001","detailItem":[{"quantity":8,"unitPrice":7500,"item":{"id":44,"no":"DO-ITM","name":"Barang DO","upcNo":null},"itemUnit":{"name":"Pcs"}}]}}`))
	})

	// 5. Stock Opname Order
	mux.HandleFunc("/accurate/inventory/search-stock-opname-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":5,"number":"SOO-001","transDate":"05/01/2026","statusName":"Menunggu","description":"Opname 1"}]}`))
	})
	mux.HandleFunc("/accurate/inventory/detail-stock-opname-order.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SOO-001","detailItem":[{"quantity":0,"unitPrice":0,"item":{"id":55,"no":"SOO-ITM","name":"Barang SOO","upcNo":"555666"},"itemUnit":{"name":"Pcs"}}]}}`))
	})

	// 6. Stock Opname Result
	mux.HandleFunc("/accurate/inventory/search-stock-opname-result.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":6,"number":"SOR-001","transDate":"06/01/2026","statusName":"Disetujui","description":"Hasil 1"}]}`))
	})
	mux.HandleFunc("/accurate/inventory/view-stock-opname-result.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SOR-001","detailItem":[{"quantity":12,"unitPrice":0,"item":{"id":66,"no":"SOR-ITM","name":"Barang SOR","upcNo":null},"itemUnit":{"name":"Lusin"}}]}}`))
	})

	client := accurate.NewClient(server.Client().Transport)
	webProv := web.New("test@example.com", "pass", client)
	session := &accurate.TenantSession{
		Host: server.URL,
		DSI:  "TEST_DSI",
	}
	webProv.SetTenantSession(session)

	registry := adapter.NewRegistry(webProv)
	sources := registry.List()
	if len(sources) != 6 {
		t.Fatalf("Expected 6 registered sources, got: %d", len(sources))
	}

	ctx := context.Background()

	// Test each source
	for _, s := range sources {
		src, ok := registry.Get(s.ID)
		if !ok || !src.IsAvailable() {
			t.Fatalf("Source %s not available", s.ID)
		}

		docs, p, err := src.ListDocuments(ctx, webProv, "", "", "", 1, 20)
		if err != nil {
			t.Fatalf("Source %s ListDocuments error: %v", s.ID, err)
		}
		if len(docs) != 1 || p.RowCount != 1 {
			t.Fatalf("Source %s expected 1 doc, got: %d", s.ID, len(docs))
		}

		items, err := src.GetDocumentItems(ctx, webProv, docs[0].ID)
		if err != nil {
			t.Fatalf("Source %s GetDocumentItems error: %v", s.ID, err)
		}
		if len(items) != 1 {
			t.Fatalf("Source %s expected 1 item, got: %d", s.ID, len(items))
		}

		item := items[0]
		if item.ItemNo == "PO-ITM" && item.UPCNo != "PO-ITM" {
			t.Fatalf("Expected UPC fallback to itemNo 'PO-ITM', got: %s", item.UPCNo)
		}
		if item.ItemNo == "SOO-ITM" && item.Quantity != 1 {
			t.Fatalf("Expected stock opname order quantity to default to 1, got: %f", item.Quantity)
		}
		if item.ItemNo == "SOR-ITM" && item.Quantity != 12 {
			t.Fatalf("Expected stock opname result quantity 12, got: %f", item.Quantity)
		}
	}
}

func TestAdapters_AllSources_Official(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Receive Item
	mux.HandleFunc("/accurate/api/receive-item/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":10,"number":"RI-OFF-1","transDate":"10/01/2026","statusName":"Diterima","description":"RI Official"}]}`))
	})
	mux.HandleFunc("/accurate/api/receive-item/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"RI-OFF-1","detailItem":[{"id":1,"itemNo":"RI-01","itemName":"Baut M4","itemUnitName":"Bungkus","quantity":100,"unitPrice":250}]}}`))
	})

	// 2. Purchase Order
	mux.HandleFunc("/accurate/api/purchase-order/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":20,"number":"PO-OFF-1","transDate":"11/01/2026","statusName":"Disetujui"}]}`))
	})
	mux.HandleFunc("/accurate/api/purchase-order/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"PO-OFF-1","detailItem":[{"id":2,"itemNo":"PO-01","itemName":"Cat Tembok","quantity":10,"unitPrice":85000}]}}`))
	})

	// 3. Sales Order
	mux.HandleFunc("/accurate/api/sales-order/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":30,"number":"SO-OFF-1","transDate":"12/01/2026","statusName":"Menunggu"}]}`))
	})
	mux.HandleFunc("/accurate/api/sales-order/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SO-OFF-1","detailItem":[{"id":3,"itemNo":"SO-01","itemName":"Kertas A4","quantity":5,"unitPrice":45000}]}}`))
	})

	// 4. Delivery Order
	mux.HandleFunc("/accurate/api/delivery-order/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":40,"number":"DO-OFF-1","transDate":"13/01/2026","statusName":"Terkirim"}]}`))
	})
	mux.HandleFunc("/accurate/api/delivery-order/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"DO-OFF-1","detailItem":[{"id":4,"itemNo":"DO-01","itemName":"Monitor LED","quantity":2,"unitPrice":1250000}]}}`))
	})

	// 5. Stock Opname Order
	mux.HandleFunc("/accurate/api/stock-opname-order/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":50,"number":"SOO-OFF-1","transDate":"14/01/2026","statusName":"Proses"}]}`))
	})
	mux.HandleFunc("/accurate/api/stock-opname-order/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SOO-OFF-1","detailItem":[{"id":5,"itemNo":"SOO-01","itemName":"Tinta Printer","quantity":0,"unitPrice":0}]}}`))
	})

	// 6. Stock Opname Result
	mux.HandleFunc("/accurate/api/stock-opname-result/list.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"sp":{"page":1,"pageCount":1,"pageSize":20,"rowCount":1},"d":[{"id":60,"number":"SOR-OFF-1","transDate":"15/01/2026","statusName":"Selesai"}]}`))
	})
	mux.HandleFunc("/accurate/api/stock-opname-result/detail.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":true,"d":{"number":"SOR-OFF-1","detailItem":[{"id":6,"itemNo":"SOR-01","itemName":"Stapler","actualQuantity":15,"bookQuantity":10,"unitPrice":0}]}}`))
	})

	offProv := official.New("token-xyz", official.WithBaseURL(server.URL), official.WithHTTPClient(server.Client()))
	offProv.SetSession(server.URL, "sess-xyz", accurate.Database{ID: 1})

	registry := adapter.NewRegistry(offProv)
	sources := registry.List()
	if len(sources) != 6 {
		t.Fatalf("Expected 6 registered sources, got: %d", len(sources))
	}

	ctx := context.Background()

	for _, s := range sources {
		src, ok := registry.Get(s.ID)
		if !ok || !src.IsAvailable() {
			t.Fatalf("Source %s should be available under official provider", s.ID)
		}

		docs, p, err := src.ListDocuments(ctx, offProv, "", "", "", 1, 20)
		if err != nil {
			t.Fatalf("Source %s ListDocuments error: %v", s.ID, err)
		}
		if len(docs) != 1 || p.RowCount != 1 {
			t.Fatalf("Source %s expected 1 doc, got %d", s.ID, len(docs))
		}

		items, err := src.GetDocumentItems(ctx, offProv, docs[0].ID)
		if err != nil {
			t.Fatalf("Source %s GetDocumentItems error: %v", s.ID, err)
		}
		if len(items) != 1 {
			t.Fatalf("Source %s expected 1 item, got %d", s.ID, len(items))
		}

		item := items[0]
		// Barcode fallback check: when UPC is null, fallback to itemNo
		if item.UPCNo != item.ItemNo {
			t.Errorf("Source %s expected UPC fallback to %q, got %q", s.ID, item.ItemNo, item.UPCNo)
		}

		// Quantity semantics
		if s.ID == "stock-opname-order" && item.Quantity != 1 {
			t.Errorf("SOO expected quantity to default to 1, got %f", item.Quantity)
		}
		if s.ID == "stock-opname-result" && item.Quantity != 15 {
			t.Errorf("SOR expected actualQuantity 15, got %f", item.Quantity)
		}
	}
}

func TestAdapters_Official_UnavailableFallback(t *testing.T) {
	// If a source is absent from the official reference, it reports unavailable under official mode
	// while remaining available under web mode.
	offProv := official.New("token-xyz", official.WithDisabledSource("stock-opname-order", "resource not exposed in official developer reference"))
	webProv := web.New("email@example.com", "pass", nil)

	offRegistry := adapter.NewRegistry(offProv)
	webRegistry := adapter.NewRegistry(webProv)

	// Check official mode
	sooOff, ok := offRegistry.Get("stock-opname-order")
	if !ok {
		t.Fatal("stock-opname-order not found in official registry")
	}
	if sooOff.IsAvailable() {
		t.Fatal("expected stock-opname-order to be UNAVAILABLE in official mode")
	}
	if !strings.Contains(sooOff.UnavailableReason(), "not exposed") {
		t.Errorf("unexpected unavailable reason: %q", sooOff.UnavailableReason())
	}

	// Other official sources remain usable
	soOff, ok := offRegistry.Get("sales-order")
	if !ok || !soOff.IsAvailable() {
		t.Fatal("sales-order should remain available in official mode")
	}

	// Check web mode: all sources remain available
	sooWeb, ok := webRegistry.Get("stock-opname-order")
	if !ok || !sooWeb.IsAvailable() {
		t.Fatal("stock-opname-order should be available under web mode")
	}
}

func TestFlexFloat_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		expected  float64
		shouldErr bool
	}{
		{"plain number", `15000`, 15000, false},
		{"decimal number", `15000.5`, 15000.5, false},
		{"quoted number", `"15000.00"`, 15000, false},
		{"quoted integer", `"25"`, 25, false},
		{"null", `null`, 0, false},
		{"empty quoted string", `""`, 0, false},
		{"malformed text", `"not-a-number"`, 0, true},
		{"boolean", `true`, 0, true},
		{"object", `{"val": 10}`, 0, true},
		{"array", `[10]`, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f accurate.FlexFloat
			err := json.Unmarshal([]byte(tc.input), &f)
			if tc.shouldErr {
				if err == nil {
					t.Fatalf("expected error for input %s, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %s: %v", tc.input, err)
				}
				if f.Float64() != tc.expected {
					t.Fatalf("expected %f, got %f", tc.expected, f.Float64())
				}
			}
		})
	}
}
