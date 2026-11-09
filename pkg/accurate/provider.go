package accurate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrSessionExpired is returned when the Accurate session is invalid or expired.
var ErrSessionExpired = errors.New("accurate session expired")

// FlexFloat decodes numeric values from JSON numbers, quoted numeric strings, or null (which becomes 0).
// Malformed strings or non-numeric JSON types produce a decode error.
type FlexFloat float64

func (f *FlexFloat) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` || trimmed == "" {
		*f = 0
		return nil
	}

	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		unquoted, err := strconv.Unquote(trimmed)
		if err != nil {
			return fmt.Errorf("invalid quoted string %s: %w", trimmed, err)
		}
		unquoted = strings.TrimSpace(unquoted)
		if unquoted == "" {
			*f = 0
			return nil
		}
		clean := strings.ReplaceAll(unquoted, ",", "")
		val, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as numeric value: %w", unquoted, err)
		}
		*f = FlexFloat(val)
		return nil
	}

	var val float64
	if err := json.Unmarshal(data, &val); err != nil {
		return fmt.Errorf("invalid numeric value %s: %w", trimmed, err)
	}
	*f = FlexFloat(val)
	return nil
}

func (f FlexFloat) Float64() float64 {
	return float64(f)
}

// NormalizedItem represents a single item line normalized across all sources for barcode printing.
type NormalizedItem struct {
	ItemID         int64   `json:"itemId"`
	ItemNo         string  `json:"itemNo"`
	ItemName       string  `json:"itemName"`
	UPCNo          string  `json:"upcNo"`
	Quantity       float64 `json:"quantity"`
	UnitName       string  `json:"unitName"`
	UnitPrice      float64 `json:"unitPrice,omitempty"`
	DocumentType   string  `json:"documentType"`
	DocumentNumber string  `json:"documentNumber"`
}

// DocumentHeader represents a summary document from a transaction list.
type DocumentHeader struct {
	ID          int64  `json:"id"`
	Number      string `json:"number"`
	Date        string `json:"date"`
	Description string `json:"description"`
	StatusName  string `json:"statusName"`
}

// SourceInfo describes a registered transaction source adapter.
type SourceInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DataPlan  string `json:"dataPlan"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// InventoryItem represents a master inventory item from Accurate.
type InventoryItem struct {
	ID              int64   `json:"id"`
	No              string  `json:"no"`
	Name            string  `json:"name"`
	UPCNo           string  `json:"upcNo"`
	Unit1Name       string  `json:"unit1Name"`
	Unit2Name       string  `json:"unit2Name"`
	UnitPrice       float64 `json:"unitPrice"`
	AllQuantity     float64 `json:"allQuantity"`
	AvailableToSell float64 `json:"availableToSell"`
	ItemType        string  `json:"itemType"`
	ItemTypeName    string  `json:"itemTypeName"`
	CanPrintBarcode bool    `json:"canPrintBarcode"`
}

// Pagination represents standard pagination metadata.
type Pagination struct {
	Page      int `json:"page"`
	PageCount int `json:"pageCount"`
	PageSize  int `json:"pageSize"`
	RowCount  int `json:"rowCount"`
}

// Database represents an accessible tenant database.
type Database struct {
	ID             int64  `json:"id"`
	Alias          string `json:"alias"`
	Host           string `json:"host,omitempty"`
	UniqueID       string `json:"uniqueId,omitempty"`
	IsPrivateCloud bool   `json:"isPrivateCloud,omitempty"`
}

// LoginResult contains the result of a login or MFA step.
type LoginResult struct {
	Status      LoginStatus `json:"status"`
	ChallengeID string      `json:"challengeId,omitempty"`
}

// SearchItemsResult contains items and pagination metadata.
type SearchItemsResult struct {
	Items      []InventoryItem `json:"items"`
	Pagination Pagination      `json:"pagination"`
}

// OfficialSessionState stores non-secret connection state for the official API provider.
// Accurate API tokens MUST NEVER be persisted here.
type OfficialSessionState struct {
	DatabaseID int64    `json:"databaseId"`
	Host       string   `json:"host"`
	Session    string   `json:"session"`
	Database   Database `json:"database"`
}

// ResolveBarcode sanitizes UPC / barcode with fallback to item number.
func ResolveBarcode(upc, itemNo string) string {
	upc = strings.TrimSpace(upc)
	if upc != "" {
		return upc
	}
	return strings.TrimSpace(itemNo)
}

// Provider defines the contract for interacting with Accurate Online (Web or Official API).
type Provider interface {
	ID() string
	Login(ctx context.Context, challengeID, otp string) (LoginResult, error)
	ListDatabases(ctx context.Context) ([]Database, error)
	OpenDatabase(ctx context.Context, databaseID int64) (Database, error)
	SearchItems(ctx context.Context, query string, page, pageSize int) (SearchItemsResult, error)
	ListDocuments(ctx context.Context, sourceID, query, from, to string, page, pageSize int) ([]DocumentHeader, Pagination, error)
	GetDocumentItems(ctx context.Context, sourceID string, documentID int64) ([]NormalizedItem, error)
	SupportsSource(sourceID string) (bool, string)
}
