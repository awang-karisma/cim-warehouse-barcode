package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type StockOpnameOrderAdapter struct {
	prov accurate.Provider
}

func NewStockOpnameOrderAdapter(prov accurate.Provider) *StockOpnameOrderAdapter {
	return &StockOpnameOrderAdapter{prov: prov}
}

func (a *StockOpnameOrderAdapter) ID() string {
	return string(accurate.SourceStockOpnameOrder)
}

func (a *StockOpnameOrderAdapter) Name() string {
	return "Perintah Stok Opname (Stock Opname Order)"
}

func (a *StockOpnameOrderAdapter) DataPlan() string {
	return "StockOpnameOrderDataPlan"
}

func (a *StockOpnameOrderAdapter) DetailDataPlan() string {
	return "StockOpnameOrderDetailDataPlan"
}

func (a *StockOpnameOrderAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *StockOpnameOrderAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *StockOpnameOrderAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *StockOpnameOrderAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
