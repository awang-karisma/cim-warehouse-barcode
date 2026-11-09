package adapter

import (
	"context"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

// SearchItems queries items using the given provider.
func SearchItems(ctx context.Context, p accurate.Provider, query string, page, pageSize int) (*accurate.SearchItemsResult, error) {
	res, err := p.SearchItems(ctx, query, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
