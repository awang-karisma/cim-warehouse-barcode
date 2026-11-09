package session

import (
	"log"
	"sync"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
)

// SessionStore defines the interface for persisting Accurate session state.
type SessionStore interface {
	SaveAccurateSession(providerID, email, deviceID string, jar *accurate.MemoryJar, tenant *accurate.TenantSession, official *accurate.OfficialSessionState) error
	LoadAccurateSession() (providerID, email, deviceID string, cookies []*accurate.Cookie, tenant *accurate.TenantSession, official *accurate.OfficialSessionState, err error)
	ClearAccurateSession() error
}

// Status represents the current connection and database session status.
type Status struct {
	Connected bool               `json:"connected"`
	Database  *accurate.Database `json:"database,omitempty"`
}

// Manager manages in-memory session state and coordinates persistence.
// All credentials and tokens exist ONLY in process memory.
type Manager struct {
	mu              sync.RWMutex
	store           SessionStore
	prov            accurate.Provider
	client          *accurate.Client
	tenantSession   *accurate.TenantSession
	officialSession *accurate.OfficialSessionState
	email           string
}

// NewManager creates a Manager with optional persistence and configured Provider.
func NewManager(st SessionStore, prov ...accurate.Provider) *Manager {
	var p accurate.Provider
	if len(prov) > 0 && prov[0] != nil {
		p = prov[0]
	}

	m := &Manager{
		store: st,
		prov:  p,
	}

	if wp, ok := p.(*web.Provider); ok {
		m.client = wp.Client()
	} else {
		m.client = accurate.NewClient(nil)
	}

	if st != nil {
		providerID, email, deviceID, cookies, tenant, officialState, err := st.LoadAccurateSession()
		if err != nil {
			log.Printf("session: failed to load session from store: %v", err)
		} else {
			if p == nil || p.ID() == providerID {
				if accurate.ProviderKind(providerID) == accurate.ProviderOfficial {
					if officialState != nil && officialState.Host != "" {
						m.officialSession = officialState
						if op, ok := p.(*official.Provider); ok {
							op.SetSession(officialState.Host, officialState.Session, officialState.Database)
						}
					}
				} else {
					if deviceID != "" {
						m.client.SetDeviceID(deviceID)
					}
					if len(cookies) > 0 {
						m.client.Jar().LoadCookies(cookies)
					}
					if email != "" {
						m.email = email
					}
					if tenant != nil && tenant.DSI != "" {
						m.tenantSession = tenant
						if wp, ok := p.(*web.Provider); ok {
							wp.SetTenantSession(tenant)
						}
					}
				}
			}
		}
	}

	return m
}

func (m *Manager) Provider() accurate.Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.prov
}

func (m *Manager) SetProvider(p accurate.Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prov = p
	if wp, ok := p.(*web.Provider); ok {
		m.client = wp.Client()
	}
}

func (m *Manager) Client() *accurate.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

func (m *Manager) SetClient(c *accurate.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = c
	if wp, ok := m.prov.(*web.Provider); ok {
		wp.SetClient(c)
	}
	m.persist()
}

func (m *Manager) SetEmail(email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.email = email
	m.persist()
}

func (m *Manager) Email() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.email
}

func (m *Manager) SetTenantSession(s *accurate.TenantSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenantSession = s
	if wp, ok := m.prov.(*web.Provider); ok {
		wp.SetTenantSession(s)
	}
	m.persist()
}

func (m *Manager) TenantSession() (*accurate.TenantSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tenantSession == nil || m.tenantSession.DSI == "" {
		return nil, false
	}
	return m.tenantSession, true
}

func (m *Manager) SetOfficialSession(s *accurate.OfficialSessionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.officialSession = s
	if op, ok := m.prov.(*official.Provider); ok && s != nil {
		op.SetSession(s.Host, s.Session, s.Database)
	}
	m.persist()
}

func (m *Manager) OfficialSession() (*accurate.OfficialSessionState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.officialSession == nil || (m.officialSession.Host == "" && m.officialSession.Session == "") {
		return nil, false
	}
	return m.officialSession, true
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.prov != nil && accurate.ProviderKind(m.prov.ID()) == accurate.ProviderOfficial {
		if m.officialSession != nil && (m.officialSession.Host != "" || m.officialSession.Session != "") {
			db := m.officialSession.Database
			return Status{
				Connected: true,
				Database:  &db,
			}
		}
		if op, ok := m.prov.(*official.Provider); ok && op.Host() != "" {
			db := op.Database()
			return Status{
				Connected: true,
				Database:  &db,
			}
		}
		return Status{
			Connected: false,
			Database:  nil,
		}
	}

	if m.tenantSession != nil && m.tenantSession.DSI != "" {
		db := accurate.Database{
			ID:             m.tenantSession.Database.ID,
			Alias:          m.tenantSession.Database.Alias,
			Host:           m.tenantSession.Database.Host,
			UniqueID:       m.tenantSession.Database.UniqueID,
			IsPrivateCloud: m.tenantSession.Database.IsPrivateCloud,
		}
		return Status{
			Connected: true,
			Database:  &db,
		}
	}

	return Status{
		Connected: false,
		Database:  nil,
	}
}

func (m *Manager) resetLocked() {
	m.tenantSession = nil
	m.officialSession = nil
	m.email = ""
	webProv, isWeb := m.prov.(*web.Provider)
	if isWeb {
		webProv.ClearSession()
	}
	if op, ok := m.prov.(*official.Provider); ok {
		op.ClearSession()
	}
	m.client = accurate.NewClient(nil)
	if isWeb {
		webProv.SetClient(m.client)
	}
	if m.store != nil {
		if err := m.store.ClearAccurateSession(); err != nil {
			log.Printf("session: failed to clear session in store: %v", err)
		}
	}
}

func (m *Manager) Logout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetLocked()
}

func (m *Manager) InvalidateSession() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetLocked()
	return true
}

func (m *Manager) InvalidateTenantSession(expected *accurate.TenantSession) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if expected == nil || m.tenantSession != expected {
		return false
	}
	m.resetLocked()
	return true
}

func (m *Manager) persist() {
	if m.store == nil {
		return
	}
	provID := string(accurate.ProviderWeb)
	if m.prov != nil {
		provID = m.prov.ID()
	}

	var jar *accurate.MemoryJar
	var deviceID string
	if m.client != nil {
		jar = m.client.Jar()
		deviceID = m.client.DeviceID()
	}

	if err := m.store.SaveAccurateSession(provID, m.email, deviceID, jar, m.tenantSession, m.officialSession); err != nil {
		log.Printf("session: failed to persist session: %v", err)
	}
}
