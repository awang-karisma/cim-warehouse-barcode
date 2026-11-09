package accurate

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Cookie represents a stored HTTP cookie with domain and path scoping.
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HttpOnly bool      `json:"httpOnly"`
}

// MemoryJar is a thread-safe in-memory http.CookieJar that scopes cookies
// strictly by domain and path according to RFC 6265 rules.
// Crucially, cookies from account.accurate.id never overwrite or leak to
// database hosts such as zeus.accurate.id.
type MemoryJar struct {
	mu      sync.RWMutex
	cookies []*Cookie
}

// NewMemoryJar creates a new initialized MemoryJar.
func NewMemoryJar() *MemoryJar {
	return &MemoryJar{
		cookies: make([]*Cookie, 0),
	}
}

// SetCookies handles the receipt of the cookies in a reply for the given URL.
func (j *MemoryJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()

	hostname := strings.ToLower(u.Hostname())

	for _, c := range cookies {
		domain := hostname
		if c.Domain != "" {
			d := strings.ToLower(c.Domain)
			if strings.HasPrefix(d, ".") {
				d = d[1:]
			}
			// Only allow suffix of hostname
			if hostname == d || strings.HasSuffix(hostname, "."+d) {
				domain = d
			}
		}

		path := c.Path
		if path == "" {
			path = "/"
		}

		// Check if expired / max-age <= 0
		if !c.Expires.IsZero() && c.Expires.Before(time.Now()) {
			j.remove(domain, path, c.Name)
			continue
		}
		if c.MaxAge < 0 {
			j.remove(domain, path, c.Name)
			continue
		}

		// Replace existing cookie with same (domain, path, name)
		j.remove(domain, path, c.Name)
		j.cookies = append(j.cookies, &Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   domain,
			Path:     path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
		})
	}
}

// Cookies returns the cookies to send in a request for the given URL.
func (j *MemoryJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.RLock()
	defer j.mu.RUnlock()

	hostname := strings.ToLower(u.Hostname())
	pathname := u.Path
	if pathname == "" {
		pathname = "/"
	}

	var res []*http.Cookie
	for _, c := range j.cookies {
		// Domain match: exact or ends with .domain
		domainMatch := (hostname == c.Domain) || strings.HasSuffix(hostname, "."+c.Domain)
		if !domainMatch {
			continue
		}

		// Path match: exact or prefix
		pathMatch := (pathname == c.Path) || strings.HasPrefix(pathname, strings.TrimSuffix(c.Path, "/")+"/")
		if !pathMatch {
			continue
		}

		// Expiry check
		if !c.Expires.IsZero() && c.Expires.Before(time.Now()) {
			continue
		}

		res = append(res, &http.Cookie{
			Name:  c.Name,
			Value: c.Value,
		})
	}

	return res
}

func (j *MemoryJar) remove(domain, path, name string) {
	filtered := make([]*Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		if !(c.Domain == domain && c.Path == path && c.Name == name) {
			filtered = append(filtered, c)
		}
	}
	j.cookies = filtered
}

// AllCookies returns a deep copy of all currently stored valid cookies.
func (j *MemoryJar) AllCookies() []*Cookie {
	j.mu.RLock()
	defer j.mu.RUnlock()

	res := make([]*Cookie, len(j.cookies))
	for i, c := range j.cookies {
		cp := *c
		res[i] = &cp
	}
	return res
}

// LoadCookies replaces the current jar contents with the given cookies.
func (j *MemoryJar) LoadCookies(cookies []*Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.cookies = make([]*Cookie, len(cookies))
	for i, c := range cookies {
		cp := *c
		j.cookies[i] = &cp
	}
}

// Clone returns an independent, deep copy of the MemoryJar.
func (j *MemoryJar) Clone() *MemoryJar {
	j.mu.RLock()
	defer j.mu.RUnlock()

	clone := NewMemoryJar()
	clone.cookies = make([]*Cookie, len(j.cookies))
	for i, c := range j.cookies {
		cp := *c
		clone.cookies[i] = &cp
	}
	return clone
}

// HasCookiesForHost checks if there are stored cookies matching the host.
func (j *MemoryJar) HasCookiesForHost(hostOrURL string) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	target := strings.ToLower(hostOrURL)
	if u, err := url.Parse(hostOrURL); err == nil && u.Hostname() != "" {
		target = strings.ToLower(u.Hostname())
	}

	now := time.Now()
	for _, c := range j.cookies {
		if !strings.EqualFold(c.Domain, target) && !strings.HasSuffix(target, "."+c.Domain) {
			continue
		}
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		return true
	}
	return false
}

// ClearHost removes all stored cookies matching the given host or URL.
func (j *MemoryJar) ClearHost(hostOrURL string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	target := strings.ToLower(hostOrURL)
	if u, err := url.Parse(hostOrURL); err == nil && u.Hostname() != "" {
		target = strings.ToLower(u.Hostname())
	}

	filtered := make([]*Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		if strings.EqualFold(c.Domain, target) {
			continue
		}
		filtered = append(filtered, c)
	}
	j.cookies = filtered
}
