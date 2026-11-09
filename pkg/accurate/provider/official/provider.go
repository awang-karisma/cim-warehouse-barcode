package official

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

// GenerateSignature computes the HMAC-SHA256 Base64-encoded signature of timestamp using the secret key.
// Reference: accurate-online-api-token-1.0.3.pdf Page 15.
func GenerateSignature(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Option is a functional option for configuring the official Provider.
type Option func(*Provider)

// WithBaseURL overrides the default basic API base URL (https://account.accurate.id).
func WithBaseURL(baseURL string) Option {
	return func(p *Provider) {
		p.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) {
		p.httpClient = client
	}
}

// WithSignatureSecret sets the Signature Secret for generating X-Api-Signature and X-Api-Timestamp.
func WithSignatureSecret(secret string) Option {
	return func(p *Provider) {
		p.signatureSecret = strings.TrimSpace(secret)
	}
}

// WithNow overrides the current time generator (useful for deterministic tests).
func WithNow(fn func() time.Time) Option {
	return func(p *Provider) {
		p.now = fn
	}
}

// WithTimestampFormat overrides the default UTC timestamp format (default: "2006-01-02T15:04:05Z").
func WithTimestampFormat(format string) Option {
	return func(p *Provider) {
		p.timestampFormat = format
	}
}

// WithDisabledSource marks a source as unsupported under official provider (for testing fallback).
func WithDisabledSource(sourceID, reason string) Option {
	return func(p *Provider) {
		p.disabledSrcs[sourceID] = reason
	}
}

// Provider implements accurate.Provider for Accurate Online official Bearer-token API.
type Provider struct {
	mu              sync.RWMutex
	token           string
	signatureSecret string
	baseURL         string
	httpClient      *http.Client
	host            string
	sessionID       string
	database        accurate.Database
	disabledSrcs    map[string]string
	cachedDBs       map[int64]accurate.Database
	now             func() time.Time
	timestampFormat string
}

// New creates an official API provider instance with the given Accurate API token.
func New(token string, opts ...Option) *Provider {
	baseClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Preserve Authorization, X-Session-ID, X-Api-Timestamp, and X-Api-Signature on cross-host redirects (e.g. HTTP 308)
			if len(via) > 0 {
				if auth := via[0].Header.Get("Authorization"); auth != "" && req.Header.Get("Authorization") == "" {
					req.Header.Set("Authorization", auth)
				}
				if sess := via[0].Header.Get("X-Session-ID"); sess != "" && req.Header.Get("X-Session-ID") == "" {
					req.Header.Set("X-Session-ID", sess)
				}
				if ts := via[0].Header.Get("X-Api-Timestamp"); ts != "" && req.Header.Get("X-Api-Timestamp") == "" {
					req.Header.Set("X-Api-Timestamp", ts)
				}
				if sig := via[0].Header.Get("X-Api-Signature"); sig != "" && req.Header.Get("X-Api-Signature") == "" {
					req.Header.Set("X-Api-Signature", sig)
				}
			}
			return nil
		},
	}

	p := &Provider{
		token:           strings.TrimSpace(token),
		baseURL:         "https://account.accurate.id",
		httpClient:      baseClient,
		disabledSrcs:    make(map[string]string),
		cachedDBs:       make(map[int64]accurate.Database),
		now:             func() time.Time { return time.Now().UTC() },
		timestampFormat: "2006-01-02T15:04:05Z",
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Provider) ID() string {
	return string(accurate.ProviderOfficial)
}

// sanitizeError redacts the API token and signature secret from any error message.
func (p *Provider) sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if p.token != "" {
		msg = strings.ReplaceAll(msg, p.token, "[REDACTED]")
	}
	if p.signatureSecret != "" {
		msg = strings.ReplaceAll(msg, p.signatureSecret, "[REDACTED]")
	}
	return errors.New(msg)
}

func (p *Provider) Login(ctx context.Context, challengeID, otp string) (accurate.LoginResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.token == "" {
		return accurate.LoginResult{}, fmt.Errorf("missing ACCURATE_API_TOKEN")
	}
	return accurate.LoginResult{Status: accurate.LoginSuccess}, nil
}

func (p *Provider) SetSession(host, sessionID string, db accurate.Database) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.host = strings.TrimRight(host, "/")
	p.sessionID = sessionID
	p.database = db
}

func (p *Provider) Host() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.host
}

func (p *Provider) SessionID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionID
}

func (p *Provider) Database() accurate.Database {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.database
}

func (p *Provider) ClearSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.host = ""
	p.sessionID = ""
	p.database = accurate.Database{}
}

// executeRequest performs an HTTP request with Bearer authorization and optional X-Session-ID / X-Api-Signature.
func (p *Provider) executeRequest(ctx context.Context, method, fullURL string, query url.Values, sessionID string) ([]byte, error) {
	if len(query) > 0 {
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + query.Encode()
		} else {
			fullURL += "?" + query.Encode()
		}
	}

	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, p.sanitizeError(err)
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}

	if p.signatureSecret != "" {
		timestamp := p.now().UTC().Format(p.timestampFormat)
		sig := GenerateSignature(timestamp, p.signatureSecret)
		req.Header.Set("X-Api-Timestamp", timestamp)
		req.Header.Set("X-Api-Signature", sig)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, p.sanitizeError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, p.sanitizeError(err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: status %d unauthorized", accurate.ErrSessionExpired, resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.sanitizeError(fmt.Errorf("API request to %s failed with status %d: %s", fullURL, resp.StatusCode, string(body)))
	}

	return body, nil
}

func parseAPITokenResponse(body []byte) (*accurate.Database, error) {
	var raw struct {
		S bool            `json:"s"`
		D json.RawMessage `json:"d"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode api-token json: %w", err)
	}
	if !raw.S || len(raw.D) == 0 {
		return nil, errors.New("api-token returned unsuccessful response")
	}

	var dMap map[string]interface{}
	if err := json.Unmarshal(raw.D, &dMap); err != nil {
		return nil, fmt.Errorf("failed to decode api-token 'd' map: %w", err)
	}

	checkObj := func(obj map[string]interface{}) *accurate.Database {
		host, _ := obj["host"].(string)
		host = strings.TrimRight(strings.TrimSpace(host), "/")
		if host == "" || !strings.HasPrefix(host, "http") {
			return nil
		}
		var id int64
		switch v := obj["id"].(type) {
		case float64:
			id = int64(v)
		case string:
			id, _ = strconv.ParseInt(v, 10, 64)
		}
		alias, _ := obj["alias"].(string)
		if alias == "" {
			alias, _ = obj["name"].(string)
		}
		isPrivateCloud, _ := obj["isPrivateCloud"].(bool)
		return &accurate.Database{
			ID:             id,
			Alias:          alias,
			Host:           host,
			IsPrivateCloud: isPrivateCloud,
		}
	}

	// 1. Direct check in dMap
	if db := checkObj(dMap); db != nil {
		return db, nil
	}

	// 2. Check known sub-keys (handling formatting variations)
	knownKeys := []string{"data usaha", "dataUsaha", "data_usaha", "database", "db", "data"}
	for _, k := range knownKeys {
		if sub, ok := dMap[k].(map[string]interface{}); ok {
			if db := checkObj(sub); db != nil {
				return db, nil
			}
		}
	}

	// 3. Scan any nested map in dMap
	for _, v := range dMap {
		if sub, ok := v.(map[string]interface{}); ok {
			if db := checkObj(sub); db != nil {
				return db, nil
			}
		}
	}

	return nil, errors.New("host not found in api-token response")
}

func (p *Provider) resolveDatabaseHostFromAPIToken(ctx context.Context) (*accurate.Database, error) {
	p.mu.RLock()
	baseURL := p.baseURL
	p.mu.RUnlock()

	// Try POST first (documented in accurate-online-api-token-1.0.3.pdf page 16)
	body, err := p.executeRequest(ctx, http.MethodPost, baseURL+"/api/api-token.do", nil, "")
	if err != nil {
		// If POST fails, try GET
		body, err = p.executeRequest(ctx, http.MethodGet, baseURL+"/api/api-token.do", nil, "")
		if err != nil {
			return nil, err
		}
	}

	db, err := parseAPITokenResponse(body)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.cachedDBs[db.ID] = *db
	p.mu.Unlock()

	return db, nil
}

func (p *Provider) ListDatabases(ctx context.Context) ([]accurate.Database, error) {
	p.mu.RLock()
	baseURL := p.baseURL
	sigSecret := p.signatureSecret
	token := p.token
	p.mu.RUnlock()

	// If using an API Token (has signature secret or starts with aat.), try /api/api-token.do first
	if sigSecret != "" || strings.HasPrefix(token, "aat.") {
		if db, err := p.resolveDatabaseHostFromAPIToken(ctx); err == nil && db != nil && db.Host != "" {
			return []accurate.Database{*db}, nil
		}
	}

	// Fallback to /api/db-list.do
	body, err := p.executeRequest(ctx, http.MethodGet, baseURL+"/api/db-list.do", nil, "")
	if err != nil {
		return nil, err
	}

	var rawResp struct {
		S bool `json:"s"`
		D []struct {
			ID             int64  `json:"id"`
			Alias          string `json:"alias"`
			Host           string `json:"host"`
			Logo           string `json:"logo"`
			IsPrivateCloud bool   `json:"isPrivateCloud"`
		} `json:"d"`
		ErrorMsg string `json:"errorMsg"`
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, p.sanitizeError(fmt.Errorf("failed to decode db-list response: %w", err))
	}

	if !rawResp.S {
		msg := rawResp.ErrorMsg
		if msg == "" {
			msg = "db-list returned unsuccessful response"
		}
		return nil, p.sanitizeError(errors.New(msg))
	}

	result := make([]accurate.Database, 0, len(rawResp.D))
	p.mu.Lock()
	for _, db := range rawResp.D {
		d := accurate.Database{
			ID:             db.ID,
			Alias:          db.Alias,
			Host:           db.Host,
			IsPrivateCloud: db.IsPrivateCloud,
		}
		result = append(result, d)
		p.cachedDBs[db.ID] = d
	}
	p.mu.Unlock()

	return result, nil
}

func (p *Provider) OpenDatabase(ctx context.Context, databaseID int64) (accurate.Database, error) {
	p.mu.RLock()
	baseURL := p.baseURL
	cached, hasCached := p.cachedDBs[databaseID]
	sigSecret := p.signatureSecret
	token := p.token
	p.mu.RUnlock()

	// 1. If already resolved with a valid host, use it directly (API token already includes database session)
	if hasCached && cached.Host != "" {
		p.SetSession(cached.Host, "", cached)
		return cached, nil
	}

	// 2. If this is an API Token, resolve host via /api/api-token.do (API token already includes session, avoid open-db.do)
	if sigSecret != "" || strings.HasPrefix(token, "aat.") {
		if tokenDB, tokenErr := p.resolveDatabaseHostFromAPIToken(ctx); tokenErr == nil && tokenDB != nil && tokenDB.Host != "" {
			if hasCached && tokenDB.Alias == "" {
				tokenDB.Alias = cached.Alias
			}
			if databaseID != 0 && tokenDB.ID == 0 {
				tokenDB.ID = databaseID
			}
			p.SetSession(tokenDB.Host, "", *tokenDB)
			return *tokenDB, nil
		}
	}

	// 3. Otherwise call /api/open-db.do (standard OAuth flow)
	q := url.Values{"id": {strconv.FormatInt(databaseID, 10)}}
	body, err := p.executeRequest(ctx, http.MethodGet, baseURL+"/api/open-db.do", q, "")
	if err != nil {
		errMsg := err.Error()
		// Check if Accurate returned the known "API Token ini sudah termasuk Sesi Database" message
		if strings.Contains(errMsg, "API Token ini sudah termasuk Sesi Database") || strings.Contains(errMsg, "tidak perlu melakukan open-db") {
			tokenDB, tokenErr := p.resolveDatabaseHostFromAPIToken(ctx)
			if tokenErr == nil && tokenDB != nil && tokenDB.Host != "" {
				if hasCached && tokenDB.Alias == "" {
					tokenDB.Alias = cached.Alias
				}
				if databaseID != 0 && tokenDB.ID == 0 {
					tokenDB.ID = databaseID
				}
				p.SetSession(tokenDB.Host, "", *tokenDB)
				return *tokenDB, nil
			}
		}
		return accurate.Database{}, err
	}

	var rawResp struct {
		S           bool     `json:"s"`
		D           []string `json:"d"`
		Session     string   `json:"session"`
		Host        string   `json:"host"`
		ErrorMsg    string   `json:"errorMsg"`
		DataVersion int64    `json:"dataVersion"`
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return accurate.Database{}, p.sanitizeError(fmt.Errorf("failed to decode open-db response: %w", err))
	}

	if !rawResp.S {
		msg := rawResp.ErrorMsg
		if msg == "" && len(rawResp.D) > 0 {
			msg = strings.Join(rawResp.D, "; ")
		}
		if msg == "" {
			msg = "open-db returned unsuccessful response"
		}
		// Also check if rawResp has the "API Token ini sudah termasuk Sesi Database" message
		if strings.Contains(msg, "API Token ini sudah termasuk Sesi Database") || strings.Contains(msg, "tidak perlu melakukan open-db") {
			tokenDB, tokenErr := p.resolveDatabaseHostFromAPIToken(ctx)
			if tokenErr == nil && tokenDB != nil && tokenDB.Host != "" {
				if hasCached && tokenDB.Alias == "" {
					tokenDB.Alias = cached.Alias
				}
				if databaseID != 0 && tokenDB.ID == 0 {
					tokenDB.ID = databaseID
				}
				p.SetSession(tokenDB.Host, "", *tokenDB)
				return *tokenDB, nil
			}
		}
		return accurate.Database{}, p.sanitizeError(errors.New(msg))
	}

	if rawResp.Session == "" || rawResp.Host == "" {
		return accurate.Database{}, errors.New("open-db response missing session or host")
	}

	db := accurate.Database{
		ID:   databaseID,
		Host: rawResp.Host,
	}
	if hasCached && cached.Alias != "" {
		db.Alias = cached.Alias
	}

	p.SetSession(rawResp.Host, rawResp.Session, db)
	return db, nil
}

func (p *Provider) SearchItems(ctx context.Context, query string, page, pageSize int) (accurate.SearchItemsResult, error) {
	p.mu.RLock()
	host := p.host
	sessionID := p.sessionID
	p.mu.RUnlock()

	if host == "" {
		return accurate.SearchItemsResult{}, accurate.ErrSessionExpired
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	q := url.Values{
		"fields":          {"id,no,name,upcNo,unit1Name,unit2Name,unitPrice,allQuantity,availableToSell,itemType,itemTypeName"},
		"filter.itemType": {string(accurate.InventoryItemType)},
		"sp.page":         {strconv.Itoa(page)},
		"sp.pageSize":     {strconv.Itoa(pageSize)},
	}
	if strings.TrimSpace(query) != "" {
		q.Set("keywords", strings.TrimSpace(query))
	}

	apiURL := host + "/accurate/api/item/list.do"
	body, err := p.executeRequest(ctx, http.MethodGet, apiURL, q, sessionID)
	if err != nil {
		return accurate.SearchItemsResult{}, err
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
			Unit1Name       *string            `json:"unit1Name"`
			Unit2Name       *string            `json:"unit2Name"`
			UnitPrice       accurate.FlexFloat `json:"unitPrice"`
			AllQuantity     accurate.FlexFloat `json:"allQuantity"`
			AvailableToSell accurate.FlexFloat `json:"availableToSell"`
			ItemType        string             `json:"itemType"`
			ItemTypeName    string             `json:"itemTypeName"`
		} `json:"d"`
		ErrorMsg string `json:"errorMsg"`
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return accurate.SearchItemsResult{}, p.sanitizeError(fmt.Errorf("failed to decode item list: %w", err))
	}

	if !rawResp.S {
		msg := rawResp.ErrorMsg
		if msg == "" {
			msg = "item list returned unsuccessful response"
		}
		if strings.Contains(strings.ToLower(msg), "session") || strings.Contains(strings.ToLower(msg), "token") {
			return accurate.SearchItemsResult{}, fmt.Errorf("%w: %s", accurate.ErrSessionExpired, msg)
		}
		return accurate.SearchItemsResult{}, p.sanitizeError(errors.New(msg))
	}

	items := make([]accurate.InventoryItem, 0, len(rawResp.D))
	for _, it := range rawResp.D {
		upc := ""
		if it.UPCNo != nil {
			upc = strings.TrimSpace(*it.UPCNo)
		}
		barcode := accurate.ResolveBarcode(upc, it.No)
		unit1 := "PCS"
		if it.Unit1Name != nil && *it.Unit1Name != "" {
			unit1 = *it.Unit1Name
		}
		unit2 := ""
		if it.Unit2Name != nil {
			unit2 = strings.TrimSpace(*it.Unit2Name)
		}
		canPrint := accurate.InventoryType(it.ItemType) == accurate.InventoryItemType

		items = append(items, accurate.InventoryItem{
			ID:              it.ID,
			No:              it.No,
			Name:            it.Name,
			UPCNo:           barcode,
			Unit1Name:       unit1,
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	if reason, disabled := p.disabledSrcs[sourceID]; disabled {
		return false, reason
	}

	switch accurate.SourceID(sourceID) {
	case accurate.SourceSalesOrder, accurate.SourceDeliveryOrder, accurate.SourcePurchaseOrder, accurate.SourceReceiveItem, accurate.SourceStockOpnameOrder, accurate.SourceStockOpnameResult:
		return true, ""
	default:
		return false, fmt.Sprintf("unsupported source: %s", sourceID)
	}
}

func (p *Provider) ListDocuments(ctx context.Context, sourceID, query, from, to string, page, pageSize int) ([]accurate.DocumentHeader, accurate.Pagination, error) {
	ok, reason := p.SupportsSource(sourceID)
	if !ok {
		return nil, accurate.Pagination{}, fmt.Errorf("source %s unavailable: %s", sourceID, reason)
	}

	p.mu.RLock()
	host := p.host
	sessionID := p.sessionID
	p.mu.RUnlock()

	if host == "" {
		return nil, accurate.Pagination{}, accurate.ErrSessionExpired
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := url.Values{
		"fields":      {"id,number,transDate,description,statusName"},
		"sp.page":     {strconv.Itoa(page)},
		"sp.pageSize": {strconv.Itoa(pageSize)},
	}
	if strings.TrimSpace(query) != "" {
		q.Set("keywords", strings.TrimSpace(query))
	}
	if from != "" && to != "" {
		q.Set("filter.transDate.op", "BETWEEN")
		q.Add("filter.transDate.val[0]", from)
		q.Add("filter.transDate.val[1]", to)
	} else if from != "" {
		q.Set("filter.transDate.op", "GREATER_EQUAL_THAN")
		q.Set("filter.transDate.val", from)
	} else if to != "" {
		q.Set("filter.transDate.op", "LESS_EQUAL_THAN")
		q.Set("filter.transDate.val", to)
	}

	apiURL := fmt.Sprintf("%s/accurate/api/%s/list.do", host, sourceID)
	body, err := p.executeRequest(ctx, http.MethodGet, apiURL, q, sessionID)
	if err != nil {
		return nil, accurate.Pagination{}, err
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
		ErrorMsg string `json:"errorMsg"`
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, accurate.Pagination{}, p.sanitizeError(fmt.Errorf("failed to decode documents response: %w", err))
	}

	if !rawResp.S {
		msg := rawResp.ErrorMsg
		if msg == "" {
			msg = "documents list returned unsuccessful response"
		}
		if strings.Contains(strings.ToLower(msg), "session") || strings.Contains(strings.ToLower(msg), "token") {
			return nil, accurate.Pagination{}, fmt.Errorf("%w: %s", accurate.ErrSessionExpired, msg)
		}
		return nil, accurate.Pagination{}, p.sanitizeError(errors.New(msg))
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
	ok, reason := p.SupportsSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("source %s unavailable: %s", sourceID, reason)
	}

	p.mu.RLock()
	host := p.host
	sessionID := p.sessionID
	p.mu.RUnlock()

	if host == "" {
		return nil, accurate.ErrSessionExpired
	}

	q := url.Values{"id": {strconv.FormatInt(documentID, 10)}}
	apiURL := fmt.Sprintf("%s/accurate/api/%s/detail.do", host, sourceID)
	body, err := p.executeRequest(ctx, http.MethodGet, apiURL, q, sessionID)
	if err != nil {
		return nil, err
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
		ErrorMsg string `json:"errorMsg"`
	}

	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, p.sanitizeError(fmt.Errorf("failed to decode document detail: %w", err))
	}

	if !rawResp.S {
		msg := rawResp.ErrorMsg
		if msg == "" {
			msg = "document detail returned unsuccessful response"
		}
		if strings.Contains(strings.ToLower(msg), "session") || strings.Contains(strings.ToLower(msg), "token") {
			return nil, fmt.Errorf("%w: %s", accurate.ErrSessionExpired, msg)
		}
		return nil, p.sanitizeError(errors.New(msg))
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
