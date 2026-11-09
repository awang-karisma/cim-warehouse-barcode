package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type ReceiveItemAdapter struct {
	prov accurate.Provider
}

func NewReceiveItemAdapter(prov accurate.Provider) *ReceiveItemAdapter {
	return &ReceiveItemAdapter{prov: prov}
}

func (a *ReceiveItemAdapter) ID() string {
	return string(accurate.SourceReceiveItem)
}

func (a *ReceiveItemAdapter) Name() string {
	return "Penerimaan Barang (Receive Item)"
}

func (a *ReceiveItemAdapter) DataPlan() string {
	return "ReceiveItemDataPlan"
}

func (a *ReceiveItemAdapter) DetailDataPlan() string {
	return "ReceiveItemDetailDataPlan"
}

func (a *ReceiveItemAdapter) IsAvailable() bool {
	if a.prov == nil {
		return true
	}
	avail, _ := a.prov.SupportsSource(a.ID())
	return avail
}

func (a *ReceiveItemAdapter) UnavailableReason() string {
	if a.prov == nil {
		return ""
	}
	_, reason := a.prov.SupportsSource(a.ID())
	return reason
}

func (a *ReceiveItemAdapter) ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	if p == nil {
		p = a.prov
	}
	return p.ListDocuments(ctx, a.ID(), query, from, to, page, pageSize)
}

func (a *ReceiveItemAdapter) GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error) {
	if p == nil {
		p = a.prov
	}
	return p.GetDocumentItems(ctx, a.ID(), documentID)
}
