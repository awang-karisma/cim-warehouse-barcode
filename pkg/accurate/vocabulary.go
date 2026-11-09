package accurate

// ProviderKind identifies a configured Accurate integration implementation.
type ProviderKind string

const (
	ProviderWeb      ProviderKind = "web"
	ProviderOfficial ProviderKind = "official"
)

// SourceID identifies one built-in Accurate transaction source.
type SourceID string

const (
	SourceReceiveItem       SourceID = "receive-item"
	SourcePurchaseOrder     SourceID = "purchase-order"
	SourceSalesOrder        SourceID = "sales-order"
	SourceDeliveryOrder     SourceID = "delivery-order"
	SourceStockOpnameOrder  SourceID = "stock-opname-order"
	SourceStockOpnameResult SourceID = "stock-opname-result"
)

type LoginStatus string

const (
	LoginSuccess     LoginStatus = "success"
	LoginMFARequired LoginStatus = "mfa_required"
)

type InventoryType string

const InventoryItemType InventoryType = "INVENTORY"
