package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type SalesOrderAdapter struct {
	prov accurate.Provider
}

func NewSalesOrderAdapter(prov accurate.Provider) *SalesOrderAdapter {
	return &SalesOrderAdapter{prov: prov}
}

func (a *SalesOrderAdapter) ID() string {
	return string(accurate.SourceSalesOrder)
}

func (a *SalesOrderAdapter) Name() string {
	return "Pesanan Penjualan (Sales Order)"
}

func (a *SalesOrderAdapter) DataPlan() string {
	return "SalesOrderDataPlan"
}

func (a *SalesOrderAdapter) DetailDataPlan() string {
	return "SalesOrderDetailDataPlan"
}

func (a *SalesOrderAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *SalesOrderAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *SalesOrderAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *SalesOrderAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
