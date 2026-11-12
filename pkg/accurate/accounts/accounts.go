package accounts

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/adapter"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
)

var ErrEncryptionNotConfigured = errors.New("Accurate account encryption is not configured")

// SecretBox encrypts values with AES-256-GCM and field-specific associated data.
type SecretBox struct{ aead cipher.AEAD }

func NewSecretBox(keyHex string) (*SecretBox, error) {
	if keyHex == "" {
		return nil, ErrEncryptionNotConfigured
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 || len(keyHex) != 64 {
		return nil, errors.New("invalid Accurate encryption key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}
func (b *SecretBox) Seal(accountID int64, field, value string) (string, error) {
	if b == nil {
		return "", ErrEncryptionNotConfigured
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(value), []byte(fmt.Sprintf("accurate-account:%d:%s", accountID, field)))
	return base64.StdEncoding.EncodeToString(out), nil
}
func (b *SecretBox) Open(accountID int64, field, value string) (string, error) {
	if b == nil {
		return "", ErrEncryptionNotConfigured
	}
	buf, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", errors.New("invalid Accurate account ciphertext")
	}
	n := b.aead.NonceSize()
	if len(buf) < n {
		return "", errors.New("invalid Accurate account ciphertext")
	}
	plain, err := b.aead.Open(nil, buf[:n], buf[n:], []byte(fmt.Sprintf("accurate-account:%d:%s", accountID, field)))
	if err != nil {
		return "", errors.New("cannot decrypt Accurate account data; verify ACCURATE_ENCRYPTION_KEY")
	}
	return string(plain), nil
}

// ProviderFactory creates isolated providers for each account.
type ProviderFactory func(provider, email, password, token, signature string) (accurate.Provider, error)
type Entry struct {
	Provider accurate.Provider
	Session  *session.Manager
	Registry *adapter.Registry
}

// Pool coordinates account provider instances without holding its map lock during operations.
type Pool struct {
	mu      sync.Mutex
	entries map[int64]*Entry
	locks   map[int64]*sync.Mutex
	factory ProviderFactory
	store   session.SessionStore
}

func NewPool(st session.SessionStore, factory ProviderFactory) *Pool {
	if factory == nil {
		factory = defaultFactory
	}
	return &Pool{entries: map[int64]*Entry{}, locks: map[int64]*sync.Mutex{}, factory: factory, store: st}
}
func defaultFactory(kind, email, password, token, signature string) (accurate.Provider, error) {
	switch kind {
	case string(accurate.ProviderWeb):
		return web.New(email, password, nil), nil
	case string(accurate.ProviderOfficial):
		opts := []official.Option{}
		if signature != "" {
			opts = append(opts, official.WithSignatureSecret(signature))
		}
		return official.New(token, opts...), nil
	default:
		return nil, errors.New("unsupported Accurate provider")
	}
}
func (p *Pool) Resolve(id int64, kind, email, password, token, signature string) (*Entry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.entries[id]; e != nil {
		return e, nil
	}
	prov, err := p.factory(kind, email, password, token, signature)
	if err != nil {
		return nil, err
	}
	e := &Entry{Provider: prov}
	e.Session = session.NewManager(p.store, id, prov)
	e.Registry = adapter.NewRegistry(prov)
	p.entries[id] = e
	return e, nil
}
func (p *Pool) LockAccount(id int64) func() {
	p.mu.Lock()
	lock := p.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		p.locks[id] = lock
	}
	p.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}
func (p *Pool) Forget(id int64) { p.mu.Lock(); delete(p.entries, id); p.mu.Unlock() }
