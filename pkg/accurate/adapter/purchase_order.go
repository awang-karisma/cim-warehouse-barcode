package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type PurchaseOrderAdapter struct {
	prov accurate.Provider
}

func NewPurchaseOrderAdapter(prov accurate.Provider) *PurchaseOrderAdapter {
	return &PurchaseOrderAdapter{prov: prov}
}

func (a *PurchaseOrderAdapter) ID() string {
	return string(accurate.SourcePurchaseOrder)
}

func (a *PurchaseOrderAdapter) Name() string {
	return "Pesanan Pembelian (Purchase Order)"
}

func (a *PurchaseOrderAdapter) DataPlan() string {
	return "PurchaseOrderDataPlan"
}

func (a *PurchaseOrderAdapter) DetailDataPlan() string {
	return "PurchaseOrderDetailDataPlan"
}

func (a *PurchaseOrderAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *PurchaseOrderAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *PurchaseOrderAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *PurchaseOrderAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
