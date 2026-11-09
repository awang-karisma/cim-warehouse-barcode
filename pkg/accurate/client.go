package accurate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultIdPHost = "https://account.accurate.id"
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrMFARequired        = errors.New("mfa verification required")
	ErrChallengeExpired   = errors.New("mfa challenge expired or invalid")
	ErrChallengeUsed      = errors.New("mfa challenge has already been used")
	ErrUntrustedHost      = errors.New("security error: untrusted host or non-HTTPS action")
)

// DatabaseInfo represents a tenant database choice from Accurate.
type DatabaseInfo struct {
	ID             int64  `json:"id"`
	Alias          string `json:"alias"`
	Host           string `json:"host"`
	UniqueID       string `json:"uniqueId"`
	IsPrivateCloud bool   `json:"isPrivateCloud"`
}

// TenantSession represents an established database session with DSI token.
type TenantSession struct {
	Host     string       `json:"host"`
	DSI      string       `json:"dsi"`
	Database DatabaseInfo `json:"database"`
}

// PendingChallenge holds transient MFA state. Raw password and OTP are NEVER stored here.
type PendingChallenge struct {
	ChallengeID string
	Email       string
	Permit      string
	DeviceID    string
	Jar         *MemoryJar
	ExpiresAt   time.Time
}

// ChallengeStore is a thread-safe in-memory store for 5-minute MFA challenges.
type ChallengeStore struct {
	mu         sync.Mutex
	challenges map[string]*PendingChallenge
}

func NewChallengeStore() *ChallengeStore {
	return &ChallengeStore{
		challenges: make(map[string]*PendingChallenge),
	}
}

func (s *ChallengeStore) Put(c *PendingChallenge) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanExpiredLocked()
	s.challenges[c.ChallengeID] = c
}

// Take retrieves and immediately removes the challenge (ensuring single-use).
func (s *ChallengeStore) Take(id string) (*PendingChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanExpiredLocked()

	c, ok := s.challenges[id]
	if !ok {
		return nil, ErrChallengeExpired
	}
	delete(s.challenges, id)
	if time.Now().After(c.ExpiresAt) {
		return nil, ErrChallengeExpired
	}
	return c, nil
}

func (s *ChallengeStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.challenges, id)
}

func (s *ChallengeStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges = make(map[string]*PendingChallenge)
}

func (s *ChallengeStore) cleanExpiredLocked() {
	now := time.Now()
	for k, v := range s.challenges {
		if now.After(v.ExpiresAt) {
			delete(s.challenges, k)
		}
	}
}

// Client represents the web session client for Accurate Online.
type Client struct {
	httpClient *http.Client
	jar        *MemoryJar
	deviceID   string
	idpHost    string
	challenges *ChallengeStore
}

// NewClient creates a new Accurate client instance with its own isolated cookie jar.
func NewClient(transport http.RoundTripper) *Client {
	jar := NewMemoryJar()
	tr := transport
	if tr == nil {
		tr = http.DefaultTransport
	}

	c := &Client{
		jar:        jar,
		deviceID:   GenerateDeviceID(),
		idpHost:    DefaultIdPHost,
		challenges: NewChallengeStore(),
	}

	c.httpClient = &http.Client{
		Jar:       jar,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// We handle redirects manually where required
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	return c
}

// idpRedirectClient follows only redirects that remain on the configured IdP host.
func (c *Client) idpRedirectClient() *http.Client {
	return &http.Client{
		Jar:       c.jar,
		Transport: c.httpClient.Transport,
		Timeout:   c.httpClient.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateIdPHost(req.URL.String(), c.idpHost); err != nil {
				return err
			}
			expected, err := url.Parse(c.idpHost)
			if err != nil || req.URL.Scheme != expected.Scheme || req.URL.User != nil {
				return ErrUntrustedHost
			}
			return nil
		},
	}
}

// SetIdPHost overrides the IdP host (primarily used in testing).
func (c *Client) SetIdPHost(host string) {
	c.idpHost = strings.TrimSuffix(host, "/")
}

// Jar returns the client cookie jar.
func (c *Client) Jar() *MemoryJar {
	return c.jar
}

// SetJar replaces the client cookie jar.
func (c *Client) SetJar(jar *MemoryJar) {
	c.jar = jar
	c.httpClient.Jar = jar
}

// DeviceID returns the client's device identifier.
func (c *Client) DeviceID() string {
	return c.deviceID
}

// SetDeviceID sets the client's device identifier.
func (c *Client) SetDeviceID(id string) {
	c.deviceID = id
}

// Challenges returns the challenge store.
func (c *Client) Challenges() *ChallengeStore {
	return c.challenges
}

// Login initiates the email/password login flow.
func (c *Client) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1. Initial page load to obtain IdP JSESSIONID
	idpClient := c.idpRedirectClient()
	initReq, err := http.NewRequestWithContext(ctx, "GET", c.idpHost+"/", nil)
	if err != nil {
		return nil, err
	}
	initReq.Header.Set("User-Agent", UserAgent)
	initRes, err := idpClient.Do(initReq)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Accurate IdP: %w", err)
	}
	_ = initRes.Body.Close()

	// 2. Pre-login verification
	preHashed, err := NucauthPre(password, c.deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to encode pre-login payload: %w", err)
	}

	preForm := url.Values{
		"account":  {email},
		"password": {preHashed},
	}

	preReq, err := http.NewRequestWithContext(ctx, "POST", c.idpHost+"/pre-login.do", strings.NewReader(preForm.Encode()))
	if err != nil {
		return nil, err
	}
	preReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	preReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	preReq.Header.Set("Origin", c.idpHost)
	preReq.Header.Set("Referer", c.idpHost+"/")
	preReq.Header.Set("User-Agent", UserAgent)

	preRes, err := idpClient.Do(preReq)
	if err != nil {
		return nil, fmt.Errorf("pre-login request failed: %w", err)
	}
	defer preRes.Body.Close()

	var preData struct {
		S bool `json:"s"`
		D struct {
			Valid       bool   `json:"valid"`
			RequireTotp bool   `json:"requireTotp"`
			Permit      string `json:"permit"`
			ErrorMsg    string `json:"errorMsg"`
		} `json:"d"`
	}
	if err := json.NewDecoder(preRes.Body).Decode(&preData); err != nil {
		return nil, fmt.Errorf("failed to parse pre-login response (status %d, content-type %q): %w", preRes.StatusCode, preRes.Header.Get("Content-Type"), err)
	}

	if !preData.S || !preData.D.Valid {
		msg := preData.D.ErrorMsg
		if msg == "" {
			msg = "invalid email or password"
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidCredentials, msg)
	}

	// 3. Handle MFA / TOTP requirement
	if preData.D.RequireTotp {
		challengeID := GenerateChallengeID()
		c.challenges.Put(&PendingChallenge{
			ChallengeID: challengeID,
			Email:       email,
			Permit:      preData.D.Permit,
			DeviceID:    c.deviceID,
			Jar:         c.jar.Clone(),
			ExpiresAt:   time.Now().Add(5 * time.Minute),
		})

		return &LoginResult{
			Status:      LoginMFARequired,
			ChallengeID: challengeID,
		}, nil
	}

	// 4. Proceed directly to auth.do
	if err := c.doAuth(ctx, email, password, preData.D.Permit, nil, c.deviceID, c.jar); err != nil {
		return nil, err
	}

	return &LoginResult{
		Status: LoginSuccess,
	}, nil
}

// SubmitMFA handles resubmission with OTP and challenge ID.
func (c *Client) SubmitMFA(ctx context.Context, email, password, challengeID, otp string) error {
	email = strings.TrimSpace(email)
	challengeID = strings.TrimSpace(challengeID)
	otp = strings.TrimSpace(otp)

	if challengeID == "" || otp == "" {
		return errors.New("challengeId and otp are required")
	}

	challenge, err := c.challenges.Take(challengeID)
	if err != nil {
		return err
	}

	if !strings.EqualFold(challenge.Email, email) {
		return ErrInvalidCredentials
	}

	// Resubmit to auth.do using cloned jar and deviceID from challenge
	if err := c.doAuth(ctx, email, password, challenge.Permit, &otp, challenge.DeviceID, challenge.Jar); err != nil {
		return err
	}

	// On success, adopt the challenge's authenticated jar and device ID
	c.jar = challenge.Jar
	c.httpClient.Jar = c.jar
	c.deviceID = challenge.DeviceID

	return nil
}

func (c *Client) doAuth(ctx context.Context, email, password, permit string, otp *string, deviceID string, jar *MemoryJar) error {
	postHashed, err := NucauthPost(password, permit, otp, deviceID)
	if err != nil {
		return fmt.Errorf("failed to encode post-login payload: %w", err)
	}

	authForm := url.Values{
		"j_username": {email},
		"j_password": {postHashed},
	}

	authReq, err := http.NewRequestWithContext(ctx, "POST", c.idpHost+"/auth.do", strings.NewReader(authForm.Encode()))
	if err != nil {
		return err
	}
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authReq.Header.Set("Origin", c.idpHost)
	authReq.Header.Set("Referer", c.idpHost+"/")
	authReq.Header.Set("User-Agent", UserAgent)

	authClient := &http.Client{
		Jar:       jar,
		Transport: c.httpClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: c.httpClient.Timeout,
	}

	authRes, err := authClient.Do(authReq)
	if err != nil {
		return fmt.Errorf("auth.do request failed: %w", err)
	}
	defer authRes.Body.Close()

	if authRes.StatusCode != http.StatusFound && authRes.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: expected 302 redirect from /auth.do, got status %d", authRes.StatusCode)
	}

	loc := authRes.Header.Get("Location")
	if loc != "" && strings.Contains(loc, "error") {
		return fmt.Errorf("%w: upstream redirected with error", ErrInvalidCredentials)
	}

	return nil
}

// GetDatabases discovers databases accessible by the authenticated user.
func (c *Client) GetDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.idpHost+"/manage", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request /manage: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from /manage: %d", res.StatusCode)
	}

	contentData, err := ExtractContentDataFromScripts(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not parse contentData from /manage HTML: %w", err)
	}

	rawList, ok := contentData["dbList"].([]any)
	if !ok {
		return []DatabaseInfo{}, nil
	}

	var dbs []DatabaseInfo
	for _, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		var id int64
		if rawID, ok := m["id"].(float64); ok {
			id = int64(rawID)
		}
		alias, _ := m["alias"].(string)
		host, _ := m["host"].(string)
		uniqueID, _ := m["uniqueId"].(string)
		isPrivateCloud, _ := m["isPrivateCloud"].(bool)

		dbs = append(dbs, DatabaseInfo{
			ID:             id,
			Alias:          alias,
			Host:           strings.TrimSuffix(host, "/"),
			UniqueID:       uniqueID,
			IsPrivateCloud: isPrivateCloud,
		})
	}

	return dbs, nil
}

// OpenDatabase completes SAML SSO and acquires the DSI token for the given database.
func (c *Client) OpenDatabase(ctx context.Context, db DatabaseInfo) (*TenantSession, error) {
	// Validate database host
	if err := validateHost(db.Host); err != nil {
		return nil, fmt.Errorf("invalid database host: %w", err)
	}
	if strings.TrimSpace(db.UniqueID) == "" {
		return nil, errors.New("database uniqueId is required")
	}

	// 0. If host already has an active session (e.g. from prior login), try obtaining DSI directly
	if c.jar.HasCookiesForHost(db.Host) {
		if sess, err := c.requestOpenDatabaseToken(ctx, db); err == nil && sess != nil {
			return sess, nil
		}
	}

	// Clear any stale host cookies so open.do triggers a clean SAML handshake
	c.jar.ClearHost(db.Host)

	openURL := fmt.Sprintf("%s/accurate/open.do?uid=%s&product=aol", db.Host, url.QueryEscape(db.UniqueID))

	// 1. Initial GET to trigger SAML authentication request
	req1, err := http.NewRequestWithContext(ctx, "GET", openURL, nil)
	if err != nil {
		return nil, err
	}
	req1.Header.Set("User-Agent", UserAgent)

	res1, err := c.httpClient.Do(req1)
	if err != nil {
		return nil, fmt.Errorf("failed initial open.do GET: %w", err)
	}
	defer res1.Body.Close()

	// Parse SAMLRequest form
	samlReqForm, err := ExtractForm(res1.Body, "SAMLRequest")
	if err != nil {
		return nil, fmt.Errorf("expected SAMLRequest form from service provider: %w", err)
	}

	// Validate IdP action URL
	idpActionURL := resolveURL(db.Host, samlReqForm.Action)
	if err := validateIdPHost(idpActionURL, c.idpHost); err != nil {
		return nil, fmt.Errorf("invalid SAMLRequest action URL: %w", err)
	}

	// 2. Submit SAMLRequest to IdP using IdP-scoped cookies
	idpValues := url.Values{}
	for k, v := range samlReqForm.Fields {
		idpValues.Set(k, v)
	}

	req2, err := http.NewRequestWithContext(ctx, "POST", idpActionURL, strings.NewReader(idpValues.Encode()))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("User-Agent", UserAgent)

	res2, err := c.httpClient.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("failed IdP SAMLRequest submission: %w", err)
	}
	defer res2.Body.Close()

	// Parse SAMLResponse form from IdP
	samlRespForm, err := ExtractForm(res2.Body, "SAMLResponse")
	if err != nil {
		return nil, fmt.Errorf("expected SAMLResponse form from IdP: %w", err)
	}

	// Validate Assertion Consumer Service (ACS) action URL
	acsURL := resolveURL(c.idpHost, samlRespForm.Action)
	if err := validateHostMatch(acsURL, db.Host); err != nil {
		return nil, fmt.Errorf("invalid SAMLResponse ACS action URL: %w", err)
	}

	// 3. Post SAMLResponse back to Service Provider ACS URL
	spValues := url.Values{}
	for k, v := range samlRespForm.Fields {
		spValues.Set(k, v)
	}

	req3, err := http.NewRequestWithContext(ctx, "POST", acsURL, strings.NewReader(spValues.Encode()))
	if err != nil {
		return nil, err
	}
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.Header.Set("User-Agent", UserAgent)

	res3, err := c.httpClient.Do(req3)
	if err != nil {
		return nil, fmt.Errorf("failed ACS SAMLResponse submission: %w", err)
	}
	defer res3.Body.Close()

	// 4. Follow redirect or location to database open handler
	targetLocation := res3.Header.Get("Location")
	if targetLocation == "" {
		targetLocation = fmt.Sprintf("%s/accurate/open-database.do", db.Host)
	} else {
		targetLocation = resolveURL(db.Host, targetLocation)
	}
	if err := validateHostMatch(targetLocation, db.Host); err != nil {
		return nil, fmt.Errorf("untrusted post-SAML redirect: %w", err)
	}

	req4, err := http.NewRequestWithContext(ctx, "GET", targetLocation, nil)
	if err != nil {
		return nil, err
	}
	req4.Header.Set("User-Agent", UserAgent)

	res4, err := c.httpClient.Do(req4)
	if err != nil {
		return nil, fmt.Errorf("failed to follow open database redirect: %w", err)
	}
	_ = res4.Body.Close()

	// 5. Request open-database.do with uid and product to obtain _dsi token
	return c.requestOpenDatabaseToken(ctx, db)
}

func (c *Client) requestOpenDatabaseToken(ctx context.Context, db DatabaseInfo) (*TenantSession, error) {
	openDbURL := fmt.Sprintf("%s/accurate/open-database.do", db.Host)
	openDbForm := url.Values{
		"uid":     {db.UniqueID},
		"product": {"aol"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openDbURL, strings.NewReader(openDbForm.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed open-database.do request: %w", err)
	}
	defer res.Body.Close()

	var openDbData struct {
		S   bool   `json:"s"`
		DSI string `json:"dsi"`
		D   any    `json:"d"`
	}
	if err := json.NewDecoder(res.Body).Decode(&openDbData); err != nil {
		return nil, fmt.Errorf("failed to decode open-database.do response: %w", err)
	}

	if !openDbData.S || openDbData.DSI == "" {
		return nil, fmt.Errorf("failed to open database: %+v", openDbData.D)
	}

	return &TenantSession{
		Host:     db.Host,
		DSI:      openDbData.DSI,
		Database: db,
	}, nil
}

// PostTenant makes an authenticated POST request to the tenant instance host with DSI and form values.
func (c *Client) PostTenant(ctx context.Context, session *TenantSession, path string, form url.Values) ([]byte, error) {
	if session == nil || session.Host == "" || session.DSI == "" {
		return nil, ErrSessionExpired
	}

	endpoint := session.Host + path
	if form == nil {
		form = url.Values{}
	}
	form.Set("_dsi", session.DSI)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tenant request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusFound {
		location := res.Header.Get("Location")
		if location != "" {
			return nil, fmt.Errorf("tenant redirected to %s: %w", location, ErrSessionExpired)
		}
		return nil, fmt.Errorf("tenant redirected (302): %w", ErrSessionExpired)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenant endpoint returned status %d", res.StatusCode)
	}

	return io.ReadAll(res.Body)
}

func validateHost(hostStr string) error {
	u, err := url.Parse(hostStr)
	if err != nil {
		return err
	}
	// Must be HTTPS (or localhost/http for testing)
	if u.Scheme != "https" && !strings.HasPrefix(u.Host, "127.0.0.1") && !strings.HasPrefix(u.Host, "localhost") {
		return ErrUntrustedHost
	}
	return nil
}

func validateIdPHost(targetURL, expectedIdPHost string) error {
	tu, err := url.Parse(targetURL)
	if err != nil {
		return err
	}
	eu, err := url.Parse(expectedIdPHost)
	if err != nil {
		return err
	}
	if !strings.EqualFold(tu.Host, eu.Host) {
		return ErrUntrustedHost
	}
	return nil
}

func validateHostMatch(targetURL, expectedHost string) error {
	tu, err := url.Parse(targetURL)
	if err != nil {
		return err
	}
	eu, err := url.Parse(expectedHost)
	if err != nil {
		return err
	}
	if !strings.EqualFold(tu.Host, eu.Host) {
		return ErrUntrustedHost
	}
	return nil
}

func resolveURL(base, ref string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}
