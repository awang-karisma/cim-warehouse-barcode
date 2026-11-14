package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"strings"
)

type AccurateAccount struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	Email           string `json:"email"`
	Password        string `json:"-"`
	APIToken        string `json:"-"`
	SignatureSecret string `json:"-"`
	DeviceID        string `json:"-"`
	Cookies         string `json:"-"`
	Tenant          string `json:"-"`
	OfficialSession string `json:"-"`
	IsDefault       bool   `json:"is_default"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

const accountColumns = `id,name,provider,enabled,email,password_encrypted,api_token_encrypted,signature_secret_encrypted,device_id,cookies,tenant,official_session,is_default,created_at,updated_at`

func (s *Storage) CreateAccurateAccount(ctx context.Context, a AccurateAccount) (int64, error) {
	if a.Password != "" || a.APIToken != "" || a.SignatureSecret != "" {
		return 0, errors.New("plaintext Accurate credentials must not be stored")
	}
	enabled := 0
	if a.Enabled {
		enabled = 1
	}
	var id int64
	e := s.db.QueryRowContext(ctx, s.rebind(`INSERT INTO accurate_accounts(name,provider,enabled,email,password_encrypted,api_token_encrypted,signature_secret_encrypted) VALUES(?,?,?,?,?,?,?) RETURNING id`), strings.TrimSpace(a.Name), a.Provider, enabled, a.Email, a.Password, a.APIToken, a.SignatureSecret).Scan(&id)
	if e != nil {
		return 0, e
	}
	return id, nil
}
func (s *Storage) CreateAccurateAccountSecure(ctx context.Context, a AccurateAccount, seal func(int64, string, string) (string, error)) (int64, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	enabled := 0
	if a.Enabled {
		enabled = 1
	}
	var id int64
	e = tx.QueryRowContext(ctx, s.rebind(`INSERT INTO accurate_accounts(name,provider,enabled,email) VALUES(?,?,?,?) RETURNING id`), strings.TrimSpace(a.Name), a.Provider, enabled, a.Email).Scan(&id)
	if e != nil {
		return 0, e
	}
	p, t, ss := "", "", ""
	if a.Password != "" {
		p, e = seal(id, "password", a.Password)
		if e != nil {
			return 0, e
		}
	}
	if a.APIToken != "" {
		t, e = seal(id, "api_token", a.APIToken)
		if e != nil {
			return 0, e
		}
	}
	if a.SignatureSecret != "" {
		ss, e = seal(id, "signature_secret", a.SignatureSecret)
		if e != nil {
			return 0, e
		}
	}
	if _, e = tx.ExecContext(ctx, s.rebind(`UPDATE accurate_accounts SET password_encrypted=?,api_token_encrypted=?,signature_secret_encrypted=? WHERE id=?`), p, t, ss, id); e != nil {
		return 0, e
	}
	return id, tx.Commit()
}
func scanAccurateAccount(r interface{ Scan(...any) error }) (*AccurateAccount, error) {
	a := new(AccurateAccount)
	var en, active int
	var createdAt, updatedAt timestampString
	e := r.Scan(&a.ID, &a.Name, &a.Provider, &en, &a.Email, &a.Password, &a.APIToken, &a.SignatureSecret, &a.DeviceID, &a.Cookies, &a.Tenant, &a.OfficialSession, &active, &createdAt, &updatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	a.Enabled = en != 0
	a.IsDefault = active != 0
	a.CreatedAt = createdAt.value
	a.UpdatedAt = updatedAt.value
	return a, nil
}
func (s *Storage) GetAccurateAccount(ctx context.Context, id int64) (*AccurateAccount, error) {
	return scanAccurateAccount(s.db.QueryRowContext(ctx, s.rebind(`SELECT `+accountColumns+` FROM accurate_accounts WHERE id=?`), id))
}
func (s *Storage) ListAccurateAccounts(ctx context.Context) ([]AccurateAccount, error) {
	rs, e := s.db.QueryContext(ctx, `SELECT `+accountColumns+` FROM accurate_accounts ORDER BY lower(name)`)
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	out := []AccurateAccount{}
	for rs.Next() {
		a, e := scanAccurateAccount(rs)
		if e != nil {
			return nil, e
		}
		out = append(out, *a)
	}
	return out, rs.Err()
}
func (s *Storage) UpdateAccurateAccount(ctx context.Context, a AccurateAccount) error {
	return s.updateAccurateAccount(ctx, a, false)
}
func (s *Storage) UpdateAccurateAccountAndSession(ctx context.Context, a AccurateAccount) error {
	return s.updateAccurateAccount(ctx, a, true)
}
func (s *Storage) updateAccurateAccount(ctx context.Context, a AccurateAccount, clearSession bool) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	clearDefault := !a.Enabled
	res, e := tx.ExecContext(ctx, s.rebind(`UPDATE accurate_accounts SET name=?,enabled=?,email=?,password_encrypted=?,api_token_encrypted=?,signature_secret_encrypted=?,is_default=CASE WHEN ? THEN 0 ELSE is_default END,device_id=CASE WHEN ? THEN '' ELSE device_id END,cookies=CASE WHEN ? THEN '[]' ELSE cookies END,tenant=CASE WHEN ? THEN '' ELSE tenant END,official_session=CASE WHEN ? THEN '' ELSE official_session END,updated_at=CURRENT_TIMESTAMP WHERE id=?`), strings.TrimSpace(a.Name), a.Enabled, a.Email, a.Password, a.APIToken, a.SignatureSecret, clearDefault, clearSession, clearSession, clearSession, clearSession, a.ID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
func (s *Storage) DeleteAccurateAccount(ctx context.Context, id int64) error {
	r, e := s.db.ExecContext(ctx, s.rebind(`DELETE FROM accurate_accounts WHERE id=?`), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Storage) SetDefaultAccurateAccount(ctx context.Context, id int64) error {
	provider, _, _, _, tenant, official, err := s.LoadAccurateSession(id)
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	connected := provider == string(accurate.ProviderWeb) && tenant != nil && tenant.DSI != "" || provider == string(accurate.ProviderOfficial) && official != nil && official.Host != "" && official.Database.ID > 0
	if !connected {
		return errors.New("account not connected")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var enabled int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT enabled FROM accurate_accounts WHERE id=?`), id).Scan(&enabled); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if enabled == 0 {
		return errors.New("account disabled")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE accurate_accounts SET is_default=0 WHERE is_default=1`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE accurate_accounts SET is_default=1,updated_at=CURRENT_TIMESTAMP WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Storage) GetDefaultAccurateAccount(ctx context.Context) (*AccurateAccount, error) {
	return scanAccurateAccount(s.db.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accurate_accounts WHERE is_default=1`))
}
func (s *Storage) SaveAccurateSession(id int64, provider, email, device string, jar *accurate.MemoryJar, tenant *accurate.TenantSession, official *accurate.OfficialSessionState) error {
	if s.secretBox == nil {
		return accounts.ErrEncryptionNotConfigured
	}
	cookies := "[]"
	var e error
	if jar != nil {
		b, err := json.Marshal(jar.AllCookies())
		if err != nil {
			return err
		}
		cookies = string(b)
	}
	tj := ""
	if tenant != nil {
		b, err := json.Marshal(tenant)
		if err != nil {
			return err
		}
		tj = string(b)
	}
	oj := ""
	if official != nil {
		b, err := json.Marshal(official)
		if err != nil {
			return err
		}
		oj = string(b)
	}
	if cookies != "[]" {
		cookies, e = s.secretBox.Seal(id, "cookies", cookies)
		if e != nil {
			return e
		}
	}
	if tj != "" {
		tj, e = s.secretBox.Seal(id, "tenant", tj)
		if e != nil {
			return e
		}
	}
	if oj != "" {
		oj, e = s.secretBox.Seal(id, "official_session", oj)
		if e != nil {
			return e
		}
	}
	r, e := s.db.Exec(s.rebind(`UPDATE accurate_accounts SET provider=?,email=?,device_id=?,cookies=?,tenant=?,official_session=? WHERE id=?`), provider, email, device, cookies, tj, oj, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Storage) LoadAccurateSession(id int64) (provider, email, device string, cookies []*accurate.Cookie, tenant *accurate.TenantSession, official *accurate.OfficialSessionState, err error) {
	var cj, tj, oj string
	err = s.db.QueryRow(s.rebind(`SELECT provider,email,device_id,cookies,tenant,official_session FROM accurate_accounts WHERE id=?`), id).Scan(&provider, &email, &device, &cj, &tj, &oj)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		return
	}
	if err != nil {
		return
	}
	if cj != "" && cj != "[]" {
		if s.secretBox == nil {
			err = accounts.ErrEncryptionNotConfigured
			return
		}
		cj, err = s.secretBox.Open(id, "cookies", cj)
		if err != nil {
			return
		}
		err = json.Unmarshal([]byte(cj), &cookies)
		if err != nil {
			return
		}
	}
	if tj != "" {
		if s.secretBox == nil {
			err = accounts.ErrEncryptionNotConfigured
			return
		}
		tj, err = s.secretBox.Open(id, "tenant", tj)
		if err != nil {
			return
		}
		tenant = new(accurate.TenantSession)
		err = json.Unmarshal([]byte(tj), tenant)
		if err != nil {
			return
		}
	}
	if oj != "" {
		if s.secretBox == nil {
			err = accounts.ErrEncryptionNotConfigured
			return
		}
		oj, err = s.secretBox.Open(id, "official_session", oj)
		if err != nil {
			return
		}
		official = new(accurate.OfficialSessionState)
		err = json.Unmarshal([]byte(oj), official)
	}
	return
}
func (s *Storage) ClearAccurateSession(id int64) error {
	_, e := s.db.Exec(s.rebind(`UPDATE accurate_accounts SET device_id='',cookies='[]',tenant='',official_session='' WHERE id=?`), id)
	return e
}
func (s *Storage) UpdateAccurateSecrets(ctx context.Context, id int64, password, token, signature string) error {
	_, e := s.db.ExecContext(ctx, s.rebind(`UPDATE accurate_accounts SET password_encrypted=?,api_token_encrypted=?,signature_secret_encrypted=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`), password, token, signature, id)
	return e
}
func (s *Storage) verifyAccurateEncryption() error {
	rows, e := s.db.Query(`SELECT id,password_encrypted,api_token_encrypted,signature_secret_encrypted,cookies,tenant,official_session FROM accurate_accounts`)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var fields [6]string
		if e = rows.Scan(&id, &fields[0], &fields[1], &fields[2], &fields[3], &fields[4], &fields[5]); e != nil {
			return e
		}
		for i, v := range fields {
			if v == "" || v == "[]" {
				continue
			}
			if s.secretBox == nil {
				return fmt.Errorf("Accurate account data is encrypted; configure ACCURATE_ENCRYPTION_KEY")
			}
			field := []string{"password", "api_token", "signature_secret", "cookies", "tenant", "official_session"}[i]
			if _, e = s.secretBox.Open(id, field, v); e != nil {
				return e
			}
		}
	}
	return rows.Err()
}
