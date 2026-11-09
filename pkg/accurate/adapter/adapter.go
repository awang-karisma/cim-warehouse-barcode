package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

// SourceAdapter defines the contract for transaction source modules.
type SourceAdapter interface {
	ID() string
	Name() string
	DataPlan() string
	DetailDataPlan() string
	IsAvailable() bool
	UnavailableReason() string
	ListDocuments(ctx context.Context, p accurate.Provider, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error)
	GetDocumentItems(ctx context.Context, p accurate.Provider, documentID int64) ([]accurate.NormalizedItem, error)
}
