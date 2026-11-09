package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

// Provider implements accurate.Provider for Accurate Online web/SAML automation.
type Provider struct {
	mu            sync.RWMutex
	email         string
	password      string
	client        *accurate.Client
	tenantSession *accurate.TenantSession
}

func New(email, password string, client *accurate.Client) *Provider {
	if client == nil {
		client = accurate.NewClient(nil)
	}
	return &Provider{
		email:    email,
		password: password,
		client:   client,
	}
}

func (p *Provider) ID() string {
	return string(accurate.ProviderWeb)
}

func (p *Provider) Login(ctx context.Context, challengeID, otp string) (accurate.LoginResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if challengeID != "" {
		if strings.TrimSpace(otp) == "" {
			return accurate.LoginResult{}, fmt.Errorf("OTP code is required for MFA completion")
		}
		err := p.client.SubmitMFA(ctx, p.email, p.password, challengeID, otp)
		if err != nil {
			return accurate.LoginResult{}, fmt.Errorf("MFA login failed: %w", err)
		}
		return accurate.LoginResult{Status: accurate.LoginSuccess}, nil
	}

	res, err := p.client.Login(ctx, p.email, p.password)
	if err != nil {
		return accurate.LoginResult{}, fmt.Errorf("login failed: %w", err)
	}

	if res.Status == accurate.LoginMFARequired {
		return accurate.LoginResult{
			Status:      accurate.LoginMFARequired,
			ChallengeID: res.ChallengeID,
		}, nil
	}

	return accurate.LoginResult{Status: accurate.LoginSuccess}, nil
}

func (p *Provider) ListDatabases(ctx context.Context) ([]accurate.Database, error) {
	raw, err := p.client.GetDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	res := make([]accurate.Database, 0, len(raw))
	for _, db := range raw {
		res = append(res, accurate.Database{
			ID:             db.ID,
			Alias:          db.Alias,
			Host:           db.Host,
			UniqueID:       db.UniqueID,
			IsPrivateCloud: db.IsPrivateCloud,
		})
	}
	return res, nil
}

func (p *Provider) OpenDatabase(ctx context.Context, databaseID int64) (accurate.Database, error) {
	dbs, err := p.client.GetDatabases(ctx)
	if err != nil {
		return accurate.Database{}, fmt.Errorf("failed to fetch databases: %w", err)
	}
	var target *accurate.DatabaseInfo
	for _, db := range dbs {
		if db.ID == databaseID {
			target = &db
			break
		}
	}
	if target == nil {
		return accurate.Database{}, fmt.Errorf("database ID %d not found in accessible databases", databaseID)
	}

	sess, err := p.client.OpenDatabase(ctx, *target)
	if err != nil {
		return accurate.Database{}, fmt.Errorf("failed to establish database session: %w", err)
	}

	p.SetTenantSession(sess)
	return accurate.Database{
		ID:             target.ID,
		Alias:          target.Alias,
		Host:           target.Host,
		UniqueID:       target.UniqueID,
		IsPrivateCloud: target.IsPrivateCloud,
	}, nil
}

func (p *Provider) Client() *accurate.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

func (p *Provider) SetClient(c *accurate.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = c
}

func (p *Provider) TenantSession() (*accurate.TenantSession, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.tenantSession == nil || p.tenantSession.DSI == "" {
		return nil, false
	}
	return p.tenantSession, true
}

func (p *Provider) SetTenantSession(sess *accurate.TenantSession) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tenantSession = sess
}

func (p *Provider) ClearSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tenantSession = nil
}

func (p *Provider) SearchItems(ctx context.Context, query string, page, pageSize int) (accurate.SearchItemsResult, error) {
	sess, ok := p.TenantSession()
	if !ok {
		return accurate.SearchItemsResult{}, accurate.ErrSessionExpired
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	form := url.Values{
		"_dsi":            {sess.DSI},
		"fields":          {"id,no,name,upcNo,unit1Name,unit2Name,unitPrice,allQuantity,availableToSell,itemType,itemTypeName"},
		"filter.itemType": {string(accurate.InventoryItemType)},
		"keywords":        {strings.TrimSpace(query)},
		"sp.page":         {strconv.Itoa(page)},
		"sp.pageSize":     {strconv.Itoa(pageSize)},
	}

	respBytes, err := p.client.PostTenant(ctx, sess, "/accurate/inventory/search-item.do", form)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			return accurate.SearchItemsResult{}, accurate.ErrSessionExpired
		}
		return accurate.SearchItemsResult{}, fmt.Errorf("failed to search items: %w", err)
	}

	var rawResp struct {
		S  bool `json:"s"`
		Sp struct {
			Page      int `json:"page"`
			PageCount int `json:"pageCount"`
			PageSize  int `json:"pageSize"`
			RowCount  int `json:"rowCount"`
		} `json:"sp"`
		D []struct {
			ID              int64              `json:"id"`
			No              string             `json:"no"`
			Name            string             `json:"name"`
			UPCNo           *string            `json:"upcNo"`
			Unit1Name       string             `json:"unit1Name"`
			Unit2Name       *string            `json:"unit2Name"`
			UnitPrice       accurate.FlexFloat `json:"unitPrice"`
			AllQuantity     accurate.FlexFloat `json:"allQuantity"`
			AvailableToSell accurate.FlexFloat `json:"availableToSell"`
			ItemType        string             `json:"itemType"`
			ItemTypeName    string             `json:"itemTypeName"`
		} `json:"d"`
	}

	if err := json.Unmarshal(respBytes, &rawResp); err != nil {
		return accurate.SearchItemsResult{}, fmt.Errorf("failed to decode search-item response: %w", err)
	}

	if !rawResp.S {
		return accurate.SearchItemsResult{}, fmt.Errorf("search-item error from Accurate")
	}

	items := make([]accurate.InventoryItem, 0, len(rawResp.D))
	for _, it := range rawResp.D {
		upc := ""
		if it.UPCNo != nil {
			upc = strings.TrimSpace(*it.UPCNo)
		}
		unit2 := ""
		if it.Unit2Name != nil {
			unit2 = strings.TrimSpace(*it.Unit2Name)
		}

		barcode := accurate.ResolveBarcode(upc, it.No)
		canPrint := accurate.InventoryType(it.ItemType) == accurate.InventoryItemType

		items = append(items, accurate.InventoryItem{
			ID:              it.ID,
			No:              it.No,
			Name:            it.Name,
			UPCNo:           barcode,
			Unit1Name:       it.Unit1Name,
			Unit2Name:       unit2,
			UnitPrice:       it.UnitPrice.Float64(),
			AllQuantity:     it.AllQuantity.Float64(),
			AvailableToSell: it.AvailableToSell.Float64(),
			ItemType:        it.ItemType,
			ItemTypeName:    it.ItemTypeName,
			CanPrintBarcode: canPrint,
		})
	}

	return accurate.SearchItemsResult{
		Items: items,
		Pagination: accurate.Pagination{
			Page:      rawResp.Sp.Page,
			PageCount: rawResp.Sp.PageCount,
			PageSize:  rawResp.Sp.PageSize,
			RowCount:  rawResp.Sp.RowCount,
		},
	}, nil
}

func (p *Provider) SupportsSource(sourceID string) (bool, string) {
	switch accurate.SourceID(sourceID) {
	case accurate.SourceSalesOrder, accurate.SourceDeliveryOrder, accurate.SourcePurchaseOrder, accurate.SourceReceiveItem, accurate.SourceStockOpnameOrder, accurate.SourceStockOpnameResult:
		return true, ""
	default:
		return false, fmt.Sprintf("unsupported source: %s", sourceID)
	}
}

func (p *Provider) ListDocuments(ctx context.Context, sourceID, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	sess, ok := p.TenantSession()
	if !ok {
		return nil, accurate.Pagination{}, accurate.ErrSessionExpired
	}

	var endpoint string
	switch accurate.SourceID(sourceID) {
	case accurate.SourceSalesOrder:
		endpoint = "/accurate/customer/search-sales-order.do"
	case accurate.SourceDeliveryOrder:
		endpoint = "/accurate/customer/search-delivery-order.do"
	case accurate.SourcePurchaseOrder:
		endpoint = "/accurate/vendor/search-purchase-order.do"
	case accurate.SourceReceiveItem:
		endpoint = "/accurate/vendor/search-receive-item.do"
	case accurate.SourceStockOpnameOrder:
		endpoint = "/accurate/inventory/search-stock-opname-order.do"
	case accurate.SourceStockOpnameResult:
		endpoint = "/accurate/inventory/search-stock-opname-result.do"
	default:
		return nil, accurate.Pagination{}, fmt.Errorf("unsupported source: %s", sourceID)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	form := url.Values{
		"_dsi":        {sess.DSI},
		"keywords":    {strings.TrimSpace(query)},
		"sp.page":     {strconv.Itoa(page)},
		"sp.pageSize": {strconv.Itoa(pageSize)},
	}
	if from != "" {
		form.Set("from", from)
	}
	if to != "" {
		form.Set("to", to)
	}

	respBytes, err := p.client.PostTenant(ctx, sess, endpoint, form)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			return nil, accurate.Pagination{}, accurate.ErrSessionExpired
		}
		return nil, accurate.Pagination{}, fmt.Errorf("failed to search documents for %s: %w", sourceID, err)
	}

	var rawResp struct {
		S  bool `json:"s"`
		Sp struct {
			Page      int `json:"page"`
			PageCount int `json:"pageCount"`
			PageSize  int `json:"pageSize"`
			RowCount  int `json:"rowCount"`
		} `json:"sp"`
		D []struct {
			ID          int64   `json:"id"`
			Number      string  `json:"number"`
			TransDate   string  `json:"transDate"`
			Date        *string `json:"date"`
			Description *string `json:"description"`
			StatusName  *string `json:"statusName"`
			Status      *string `json:"status"`
		} `json:"d"`
	}

	if err := json.Unmarshal(respBytes, &rawResp); err != nil {
		return nil, accurate.Pagination{}, fmt.Errorf("failed to decode documents response: %w", err)
	}

	if !rawResp.S {
		return nil, accurate.Pagination{}, fmt.Errorf("error returned from Accurate for %s", sourceID)
	}

	docs := make([]accurate.DocumentHeader, 0, len(rawResp.D))
	for _, it := range rawResp.D {
		date := it.TransDate
		if date == "" && it.Date != nil {
			date = *it.Date
		}
		desc := ""
		if it.Description != nil {
			desc = strings.TrimSpace(*it.Description)
		}
		status := ""
		if it.StatusName != nil {
			status = strings.TrimSpace(*it.StatusName)
		} else if it.Status != nil {
			status = strings.TrimSpace(*it.Status)
		}

		docs = append(docs, accurate.DocumentHeader{
			ID:          it.ID,
			Number:      it.Number,
			Date:        date,
			Description: desc,
			StatusName:  status,
		})
	}

	return docs, accurate.Pagination{
		Page:      rawResp.Sp.Page,
		PageCount: rawResp.Sp.PageCount,
		PageSize:  rawResp.Sp.PageSize,
		RowCount:  rawResp.Sp.RowCount,
	}, nil
}

func (p *Provider) GetDocumentItems(ctx context.Context, sourceID string, documentID int64) ([]accurate.NormalizedItem, error) {
	sess, ok := p.TenantSession()
	if !ok {
		return nil, accurate.ErrSessionExpired
	}

	var endpoint string
	switch accurate.SourceID(sourceID) {
	case accurate.SourceSalesOrder:
		endpoint = "/accurate/customer/view-sales-order.do"
	case accurate.SourceDeliveryOrder:
		endpoint = "/accurate/customer/view-delivery-order.do"
	case accurate.SourcePurchaseOrder:
		endpoint = "/accurate/vendor/view-purchase-order.do"
	case accurate.SourceReceiveItem:
		endpoint = "/accurate/vendor/view-receive-item.do"
	case accurate.SourceStockOpnameOrder:
		endpoint = "/accurate/inventory/detail-stock-opname-order.do"
	case accurate.SourceStockOpnameResult:
		endpoint = "/accurate/inventory/view-stock-opname-result.do"
	default:
		return nil, fmt.Errorf("unsupported source: %s", sourceID)
	}

	form := url.Values{
		"_dsi": {sess.DSI},
		"id":   {strconv.FormatInt(documentID, 10)},
	}

	respBytes, err := p.client.PostTenant(ctx, sess, endpoint, form)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			return nil, accurate.ErrSessionExpired
		}
		return nil, fmt.Errorf("failed to fetch document detail: %w", err)
	}

	var rawResp struct {
		S bool `json:"s"`
		D struct {
			ID         int64  `json:"id"`
			Number     string `json:"number"`
			DetailItem []struct {
				ID             int64              `json:"id"`
				Quantity       accurate.FlexFloat `json:"quantity"`
				ActualQuantity accurate.FlexFloat `json:"actualQuantity"`
				BookQuantity   accurate.FlexFloat `json:"bookQuantity"`
				UnitPrice      accurate.FlexFloat `json:"unitPrice"`
				Item           struct {
					ID    int64   `json:"id"`
					No    string  `json:"no"`
					Name  string  `json:"name"`
					UPCNo *string `json:"upcNo"`
				} `json:"item"`
				ItemNo       *string `json:"itemNo"`
				DetailName   *string `json:"detailName"`
				ItemUnitName *string `json:"itemUnitName"`
				ItemUnit     struct {
					Name string `json:"name"`
				} `json:"itemUnit"`
			} `json:"detailItem"`
		} `json:"d"`
	}

	if err := json.Unmarshal(respBytes, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to decode document detail: %w", err)
	}

	if !rawResp.S {
		return nil, fmt.Errorf("document not found or error returned for %s", sourceID)
	}

	docNum := rawResp.D.Number
	items := make([]accurate.NormalizedItem, 0, len(rawResp.D.DetailItem))
	for _, it := range rawResp.D.DetailItem {
		itemNo := it.Item.No
		if itemNo == "" && it.ItemNo != nil {
			itemNo = *it.ItemNo
		}
		itemName := it.Item.Name
		if itemName == "" && it.DetailName != nil {
			itemName = *it.DetailName
		}

		upc := ""
		if it.Item.UPCNo != nil {
			upc = strings.TrimSpace(*it.Item.UPCNo)
		}
		barcode := accurate.ResolveBarcode(upc, itemNo)

		qty := float64(it.Quantity)
		if accurate.SourceID(sourceID) == accurate.SourceStockOpnameResult {
			if float64(it.ActualQuantity) > 0 {
				qty = float64(it.ActualQuantity)
			} else if float64(it.BookQuantity) > 0 {
				qty = float64(it.BookQuantity)
			}
		}
		if qty <= 0 {
			qty = 1
		}

		unit := it.ItemUnit.Name
		if unit == "" && it.ItemUnitName != nil {
			unit = *it.ItemUnitName
		}
		if unit == "" {
			unit = "PCS"
		}

		items = append(items, accurate.NormalizedItem{
			ItemID:         it.Item.ID,
			ItemNo:         itemNo,
			ItemName:       itemName,
			UPCNo:          barcode,
			Quantity:       qty,
			UnitName:       unit,
			UnitPrice:      float64(it.UnitPrice),
			DocumentType:   sourceID,
			DocumentNumber: docNum,
		})
	}

	return items, nil
}
