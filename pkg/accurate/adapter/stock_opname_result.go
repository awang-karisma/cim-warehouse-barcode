package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type StockOpnameResultAdapter struct {
	prov accurate.Provider
}

func NewStockOpnameResultAdapter(prov accurate.Provider) *StockOpnameResultAdapter {
	return &StockOpnameResultAdapter{prov: prov}
}

func (a *StockOpnameResultAdapter) ID() string {
	return string(accurate.SourceStockOpnameResult)
}

func (a *StockOpnameResultAdapter) Name() string {
	return "Hasil Stok Opname (Stock Opname Result)"
}

func (a *StockOpnameResultAdapter) DataPlan() string {
	return "StockOpnameResultDataPlan"
}

func (a *StockOpnameResultAdapter) DetailDataPlan() string {
	return "StockOpnameResultDetailDataPlan"
}

func (a *StockOpnameResultAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *StockOpnameResultAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *StockOpnameResultAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *StockOpnameResultAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
