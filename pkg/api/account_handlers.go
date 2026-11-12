package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	"github.com/danielgtaylor/huma/v2"
	"net/http"
	"strings"
)

type accurateAccountDTO struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Provider           string `json:"provider"`
	Enabled            bool   `json:"enabled"`
	Email              string `json:"email"`
	HasPassword        bool   `json:"has_password"`
	HasAPIToken        bool   `json:"has_api_token"`
	HasSignatureSecret bool   `json:"has_signature_secret"`
	IsDefault          bool   `json:"is_default"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	Connected          bool   `json:"connected"`
	Database           any    `json:"database,omitempty"`
}

func (s *Server) toAccurateAccountDTO(ctx context.Context, a storage.AccurateAccount) accurateAccountDTO {
	dto := accurateAccountDTO{ID: a.ID, Name: a.Name, Provider: a.Provider, Enabled: a.Enabled, Email: a.Email, HasPassword: a.Password != "", HasAPIToken: a.APIToken != "", HasSignatureSecret: a.SignatureSecret != "", IsDefault: a.IsDefault, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
	provider, _, _, _, tenant, official, err := s.store.LoadAccurateSession(a.ID)
	if err != nil {
		return dto
	}
	if provider == "web" && tenant != nil && tenant.DSI != "" {
		dto.Connected = true
		dto.Database = map[string]any{"id": tenant.Database.ID, "alias": tenant.Database.Alias, "host": tenant.Database.Host, "uniqueId": tenant.Database.UniqueID, "isPrivateCloud": tenant.Database.IsPrivateCloud}
	} else if provider == "official" && official != nil && official.Host != "" && official.Database.ID > 0 {
		dto.Connected = true
		dto.Database = official.Database
	}
	return dto
}
func (s *Server) accountBox() (*accounts.SecretBox, error) {
	if s.appConfig == nil || s.appConfig.AccurateEncryptionKey == "" {
		return nil, accounts.ErrEncryptionNotConfigured
	}
	return accounts.NewSecretBox(s.appConfig.AccurateEncryptionKey)
}

func openAccountSecret(box *accounts.SecretBox, id int64, field, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	return box.Open(id, field, ciphertext)
}
func (s *Server) handleListAccurateAccounts(ctx context.Context, _ *struct{}) (*struct {
	Body []accurateAccountDTO `json:"accounts"`
}, error) {
	as, e := s.store.ListAccurateAccounts(ctx)
	if e != nil {
		return nil, huma.Error500InternalServerError("Failed to load Accurate accounts")
	}
	out := make([]accurateAccountDTO, 0, len(as))
	for _, a := range as {
		out = append(out, s.toAccurateAccountDTO(ctx, a))
	}
	return &struct {
		Body []accurateAccountDTO `json:"accounts"`
	}{Body: out}, nil
}

type createAccurateAccountInput struct {
	Body struct {
		Name            string `json:"name"`
		Provider        string `json:"provider"`
		Enabled         *bool  `json:"enabled,omitempty"`
		Email           string `json:"email"`
		Password        string `json:"password,omitempty"`
		APIToken        string `json:"api_token,omitempty"`
		SignatureSecret string `json:"signature_secret,omitempty"`
	}
}
type createAccurateAccountOutput struct{ Body accurateAccountDTO }

func (s *Server) handleCreateAccurateAccount(ctx context.Context, in *createAccurateAccountInput) (*createAccurateAccountOutput, error) {
	a := storage.AccurateAccount{Name: strings.TrimSpace(in.Body.Name), Provider: in.Body.Provider, Enabled: true, Email: strings.TrimSpace(in.Body.Email), Password: in.Body.Password, APIToken: in.Body.APIToken, SignatureSecret: in.Body.SignatureSecret}
	if in.Body.Enabled != nil {
		a.Enabled = *in.Body.Enabled
	}
	if a.Name == "" {
		return nil, huma.Error400BadRequest("Account name is required")
	}
	switch a.Provider {
	case "web":
		if a.Email == "" || a.Password == "" {
			return nil, huma.Error400BadRequest("Email and password are required for web accounts")
		}
		if a.APIToken != "" {
			return nil, huma.Error400BadRequest("API token is only valid for official accounts")
		}
	case "official":
		if a.APIToken == "" {
			return nil, huma.Error400BadRequest("API token is required for official accounts")
		}
		if a.Email != "" || a.Password != "" {
			return nil, huma.Error400BadRequest("Email and password are only valid for web accounts")
		}
	default:
		return nil, huma.Error400BadRequest("Provider must be web or official")
	}
	box, e := s.accountBox()
	if errors.Is(e, accounts.ErrEncryptionNotConfigured) {
		return nil, huma.Error503ServiceUnavailable("Accurate account encryption is not configured")
	}
	if e != nil {
		return nil, huma.Error500InternalServerError("Account encryption is unavailable")
	}
	id, e := s.store.CreateAccurateAccountSecure(ctx, a, box.Seal)
	if e != nil {
		if strings.Contains(strings.ToLower(e.Error()), "unique") {
			return nil, huma.Error409Conflict("An account with this name already exists")
		}
		return nil, huma.Error500InternalServerError("Failed to create Accurate account")
	}
	saved, e := s.store.GetAccurateAccount(ctx, id)
	if e != nil {
		return nil, huma.Error500InternalServerError("Account was created but could not be read")
	}
	return &createAccurateAccountOutput{Body: s.toAccurateAccountDTO(ctx, *saved)}, nil
}

type accountIDInput struct {
	AccountID int64 `path:"accountId"`
}
type updateAccurateAccountInput struct {
	AccountID int64 `path:"accountId"`
	Body      struct {
		Provider             string `json:"provider,omitempty"`
		Name                 string `json:"name"`
		Enabled              bool   `json:"enabled"`
		Email                string `json:"email"`
		Password             string `json:"password,omitempty"`
		APIToken             string `json:"api_token,omitempty"`
		SignatureSecret      string `json:"signature_secret,omitempty"`
		ClearSignatureSecret bool   `json:"clear_signature_secret,omitempty"`
	}
}
type updateAccurateAccountOutput struct{ Body accurateAccountDTO }

func (s *Server) handleUpdateAccurateAccount(ctx context.Context, in *updateAccurateAccountInput) (*updateAccurateAccountOutput, error) {
	unlock := s.accountPool.LockAccount(in.AccountID)
	defer unlock()

	a, e := s.store.GetAccurateAccount(ctx, in.AccountID)
	if errors.Is(e, storage.ErrNotFound) {
		return nil, huma.Error404NotFound("Accurate account not found")
	}
	if e != nil {
		return nil, huma.Error500InternalServerError("Failed to load Accurate account")
	}
	if in.Body.Provider != "" && in.Body.Provider != a.Provider {
		return nil, huma.Error400BadRequest("Account provider cannot be changed")
	}
	if in.Body.ClearSignatureSecret && in.Body.SignatureSecret != "" {
		return nil, huma.Error400BadRequest("Cannot supply and clear signature secret together")
	}
	credentialChanged := in.Body.Email != a.Email || in.Body.Password != "" || in.Body.APIToken != "" || in.Body.SignatureSecret != "" || in.Body.ClearSignatureSecret
	a.Name = strings.TrimSpace(in.Body.Name)
	a.Enabled = in.Body.Enabled
	a.Email = strings.TrimSpace(in.Body.Email)
	if a.Name == "" {
		return nil, huma.Error400BadRequest("Account name is required")
	}
	if a.Provider == "web" && (a.Email == "" || in.Body.APIToken != "") {
		return nil, huma.Error400BadRequest("Web accounts require an email and cannot use an API token")
	}
	if a.Provider == "official" && (in.Body.Email != "" || in.Body.Password != "") {
		return nil, huma.Error400BadRequest("Official accounts cannot use web credentials")
	}
	box, e := s.accountBox()
	if e != nil {
		return nil, huma.Error503ServiceUnavailable("Accurate account encryption is not configured")
	}
	if in.Body.Password != "" {
		a.Password, e = box.Seal(a.ID, "password", in.Body.Password)
		if e != nil {
			return nil, huma.Error500InternalServerError("Failed to encrypt account credentials")
		}
	}
	if in.Body.APIToken != "" {
		a.APIToken, e = box.Seal(a.ID, "api_token", in.Body.APIToken)
		if e != nil {
			return nil, huma.Error500InternalServerError("Failed to encrypt account credentials")
		}
	}
	if in.Body.SignatureSecret != "" {
		a.SignatureSecret, e = box.Seal(a.ID, "signature_secret", in.Body.SignatureSecret)
		if e != nil {
			return nil, huma.Error500InternalServerError("Failed to encrypt account credentials")
		}
	} else if in.Body.ClearSignatureSecret {
		a.SignatureSecret = ""
	}
	clearSession := credentialChanged || !a.Enabled
	if clearSession {
		e = s.store.UpdateAccurateAccountAndSession(ctx, *a)
	} else {
		e = s.store.UpdateAccurateAccount(ctx, *a)
	}
	if e != nil {
		if errors.Is(e, storage.ErrNotFound) {
			return nil, huma.Error404NotFound("Accurate account not found")
		}
		if strings.Contains(strings.ToLower(e.Error()), "unique") {
			return nil, huma.Error409Conflict("An account with this name already exists")
		}
		return nil, huma.Error500InternalServerError("Failed to update Accurate account")
	}
	if clearSession && s.accountPool != nil {
		s.accountPool.Forget(a.ID)
	}
	saved, e := s.store.GetAccurateAccount(ctx, a.ID)
	if e != nil {
		return nil, huma.Error500InternalServerError("Failed to read Accurate account")
	}
	return &updateAccurateAccountOutput{Body: s.toAccurateAccountDTO(ctx, *saved)}, nil
}

type accountSuccess struct {
	Body struct {
		Success bool `json:"success"`
	}
}

func (s *Server) handleDeleteAccurateAccount(ctx context.Context, in *accountIDInput) (*accountSuccess, error) {
	unlock := s.accountPool.LockAccount(in.AccountID)
	defer unlock()
	if e := s.store.DeleteAccurateAccount(ctx, in.AccountID); e != nil {
		if errors.Is(e, storage.ErrNotFound) {
			return nil, huma.Error404NotFound("Accurate account not found")
		}
		return nil, huma.Error500InternalServerError("Failed to delete Accurate account")
	}
	if s.accountPool != nil {
		s.accountPool.Forget(in.AccountID)
	}
	out := &accountSuccess{}
	out.Body.Success = true
	return out, nil
}
func (s *Server) handleActivateAccurateAccount(ctx context.Context, in *accountIDInput) (*updateAccurateAccountOutput, error) {
	unlock := s.accountPool.LockAccount(in.AccountID)
	defer unlock()
	if e := s.store.SetDefaultAccurateAccount(ctx, in.AccountID); e != nil {
		if errors.Is(e, storage.ErrNotFound) {
			return nil, huma.Error404NotFound("Accurate account not found")
		}
		return nil, huma.Error409Conflict("Connect the account and select a database before using it")
	}
	a, e := s.store.GetAccurateAccount(ctx, in.AccountID)
	if e != nil {
		return nil, huma.Error500InternalServerError("Failed to read Accurate account")
	}
	return &updateAccurateAccountOutput{Body: s.toAccurateAccountDTO(ctx, *a)}, nil
}

var _ = fmt.Sprintf
var _ = http.MethodGet
var _ = model.PermAccurateAccountRead
