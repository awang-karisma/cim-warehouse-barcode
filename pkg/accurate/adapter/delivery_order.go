package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type DeliveryOrderAdapter struct {
	prov accurate.Provider
}

func NewDeliveryOrderAdapter(prov accurate.Provider) *DeliveryOrderAdapter {
	return &DeliveryOrderAdapter{prov: prov}
}

func (a *DeliveryOrderAdapter) ID() string {
	return string(accurate.SourceDeliveryOrder)
}

func (a *DeliveryOrderAdapter) Name() string {
	return "Pengiriman Pesanan (Delivery Order)"
}

func (a *DeliveryOrderAdapter) DataPlan() string {
	return "DeliveryOrderDataPlan"
}

func (a *DeliveryOrderAdapter) DetailDataPlan() string {
	return "DeliveryOrderDetailDataPlan"
}

func (a *DeliveryOrderAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *DeliveryOrderAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *DeliveryOrderAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *DeliveryOrderAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
