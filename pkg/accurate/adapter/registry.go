package adapter

import (
	"sync"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

type Registry struct {
	mu       sync.RWMutex
	adapters map[string]SourceAdapter
	order    []string
	prov     accurate.Provider
}

func NewRegistry(prov accurate.Provider) *Registry {
	r := &Registry{
		adapters: make(map[string]SourceAdapter),
		order:    make([]string, 0),
		prov:     prov,
	}

	// Register all 6 standard adapters
	r.Register(NewReceiveItemAdapter(prov))
	r.Register(NewPurchaseOrderAdapter(prov))
	r.Register(NewSalesOrderAdapter(prov))
	r.Register(NewDeliveryOrderAdapter(prov))
	r.Register(NewStockOpnameOrderAdapter(prov))
	r.Register(NewStockOpnameResultAdapter(prov))

	return r
}

func (r *Registry) SetProvider(prov accurate.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prov = prov
	r.adapters = make(map[string]SourceAdapter)
	r.order = make([]string, 0)
	r.Register(NewReceiveItemAdapter(prov))
	r.Register(NewPurchaseOrderAdapter(prov))
	r.Register(NewSalesOrderAdapter(prov))
	r.Register(NewDeliveryOrderAdapter(prov))
	r.Register(NewStockOpnameOrderAdapter(prov))
	r.Register(NewStockOpnameResultAdapter(prov))
}

func (r *Registry) Register(adapter SourceAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := adapter.ID()
	if _, exists := r.adapters[id]; !exists {
		r.order = append(r.order, id)
	}
	r.adapters[id] = adapter
}

func (r *Registry) Get(id string) (SourceAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.adapters[id]
	return a, ok
}

func (r *Registry) List() []accurate.SourceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]accurate.SourceInfo, 0, len(r.order))
	for _, id := range r.order {
		a := r.adapters[id]
		result = append(result, accurate.SourceInfo{
			ID:        a.ID(),
			Name:      a.Name(),
			DataPlan:  a.DataPlan(),
			Available: a.IsAvailable(),
			Reason:    a.UnavailableReason(),
		})
	}
	return result
}
