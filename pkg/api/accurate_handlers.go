package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/adapter"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	"github.com/danielgtaylor/huma/v2"
)

const ErrAccurateSessionInvalidatedDetail = "Your Accurate session was invalidated or expired. Please sign in again."

func (s *Server) checkAccurateEnabled() error {
	if s.accurateProvider == nil || s.accurateSession == nil || s.appConfig == nil {
		return huma.Error401Unauthorized("Accurate integration is not enabled")
	}
	return nil
}

func (s *Server) resolveAccurateAccount(ctx context.Context, requested *int64) (*Server, *storage.AccurateAccount, error) {
	if requested != nil && *requested <= 0 {
		return nil, nil, huma.Error400BadRequest("accountId must be positive")
	}
	var active *storage.AccurateAccount
	var err error
	active, err = s.store.GetDefaultAccurateAccount(ctx)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, nil, huma.Error500InternalServerError("Failed to resolve active Accurate account")
	}
	accountID := int64(0)
	if requested != nil {
		accountID = *requested
	} else if active != nil {
		accountID = active.ID
	} else {
		return nil, nil, huma.Error409Conflict("No Accurate account is selected")
	}
	unlock := s.accountPool.LockAccount(accountID)
	lockTransferred := false
	defer func() {
		if !lockTransferred {
			unlock()
		}
	}()
	a, err := s.store.GetAccurateAccount(ctx, accountID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil, huma.Error404NotFound("Accurate account not found")
	}
	if err != nil {
		return nil, nil, huma.Error500InternalServerError("Failed to load Accurate account")
	}
	if requested != nil && (active == nil || active.ID != a.ID) {
		claims := claimsFromContext(ctx)
		allowed := claims == nil && (s.appConfig == nil || s.appConfig.GoogleClientID == "") || claims != nil && claims.Role == "superadmin"
		if claims != nil {
			for _, p := range claims.Permissions {
				if p == string(model.PermAccurateAccountRead) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return nil, nil, huma.Error403Forbidden("Reading another Accurate account requires accurate_account:read")
		}
	}
	if !a.Enabled {
		return nil, nil, huma.Error409Conflict("Accurate account is disabled")
	}
	box, err := s.accountBox()
	if err != nil {
		return nil, nil, huma.Error503ServiceUnavailable("Accurate account encryption is not configured")
	}
	password, err := openAccountSecret(box, a.ID, "password", a.Password)
	if err != nil {
		return nil, nil, huma.Error500InternalServerError("Accurate account credentials cannot be decrypted")
	}
	token, err := openAccountSecret(box, a.ID, "api_token", a.APIToken)
	if err != nil {
		return nil, nil, huma.Error500InternalServerError("Accurate account credentials cannot be decrypted")
	}
	signature, err := openAccountSecret(box, a.ID, "signature_secret", a.SignatureSecret)
	if err != nil {
		return nil, nil, huma.Error500InternalServerError("Accurate account credentials cannot be decrypted")
	}
	entry, err := s.accountPool.Resolve(a.ID, a.Provider, a.Email, password, token, signature)
	if err != nil {
		return nil, nil, huma.Error500InternalServerError("Accurate account provider could not be initialized")
	}
	copy := *s
	copy.accurateProvider = entry.Provider
	copy.accurateSession = entry.Session
	copy.accurateRegistry = entry.Registry
	copy.accountEmail = a.Email
	copy.accountOperationUnlock = unlock
	lockTransferred = true
	return &copy, a, nil
}

// 1. GET /api/v1/accurate/status
type AccurateStatusOutput struct {
	Body struct {
		Enabled     bool               `json:"enabled"`
		Provider    string             `json:"provider,omitempty"`
		Connected   bool               `json:"connected"`
		Database    *accurate.Database `json:"database,omitempty"`
		AccountID   *int64             `json:"accountId,omitempty"`
		AccountName string             `json:"accountName,omitempty"`
	}
}
type accurateStatusInput struct {
	AccountID string `query:"accountId"`
}

func (s *Server) handleAccurateStatus(ctx context.Context, input *accurateStatusInput) (*AccurateStatusOutput, error) {
	var id *int64
	if input != nil {
		id = accountIDParam(input.AccountID)
	}
	if id == nil {
		a, err := s.store.GetDefaultAccurateAccount(ctx)
		if errors.Is(err, storage.ErrNotFound) {
			return &AccurateStatusOutput{}, nil
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to read active Accurate account")
		}
		id = &a.ID
	}
	resolved, a, err := s.resolveAccurateAccount(ctx, id)
	if err != nil {
		if statusErr, ok := err.(interface{ GetStatus() int }); ok && statusErr.GetStatus() == 404 {
			return &AccurateStatusOutput{}, nil
		}
		return nil, err
	}
	defer resolved.releaseAccountOperation()
	st := resolved.accurateSession.Status()
	out := &AccurateStatusOutput{}
	out.Body.Enabled = true
	out.Body.Provider = resolved.accurateProvider.ID()
	out.Body.Connected = st.Connected
	out.Body.Database = st.Database
	out.Body.AccountID = &a.ID
	out.Body.AccountName = a.Name
	return out, nil
}

func accountIDParam(raw string) *int64 {
	if raw == "" {
		return nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		invalid := int64(-1)
		return &invalid
	}
	return &id
}
func (s *Server) accountServerForRequest(ctx context.Context, id *int64) (*Server, error) {
	resolved, _, err := s.resolveAccurateAccount(ctx, id)
	return resolved, err
}
func (s *Server) releaseAccountOperation() {
	if s.accountOperationUnlock != nil {
		unlock := s.accountOperationUnlock
		s.accountOperationUnlock = nil
		unlock()
	}
}

// 2. POST /api/v1/accurate/login
type AccurateLoginInput struct {
	AccountID string `query:"accountId"`
	Body      struct {
		ChallengeID *string `json:"challengeId,omitempty" doc:"Pending MFA challenge ID when completing MFA"`
		OTP         *string `json:"otp,omitempty" doc:"One-time code from authenticator app"`
	}
}
type accurateAccountQuery struct {
	AccountID string `query:"accountId"`
}
type AccurateLoginOutput struct {
	Body struct {
		Status      string `json:"status" doc:"'success' or 'mfa_required'"`
		ChallengeID string `json:"challengeId,omitempty" doc:"Challenge ID if mfa_required"`
	}
}

func (s *Server) handleAccurateLogin(ctx context.Context, input *AccurateLoginInput) (*AccurateLoginOutput, error) {
	if input != nil {
		resolved, err := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
		if err != nil {
			return nil, err
		}
		s = resolved
		defer s.releaseAccountOperation()
	}
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	challengeID := ""
	if input != nil && input.Body.ChallengeID != nil {
		challengeID = *input.Body.ChallengeID
	}
	otp := ""
	if input != nil && input.Body.OTP != nil {
		otp = *input.Body.OTP
	}

	res, err := s.accurateProvider.Login(ctx, challengeID, otp)
	if err != nil {
		return nil, huma.Error401Unauthorized("Accurate login failed")
	}

	if res.Status == accurate.LoginMFARequired {
		return &AccurateLoginOutput{
			Body: struct {
				Status      string `json:"status" doc:"'success' or 'mfa_required'"`
				ChallengeID string `json:"challengeId,omitempty" doc:"Challenge ID if mfa_required"`
			}{
				Status:      string(accurate.LoginMFARequired),
				ChallengeID: res.ChallengeID,
			},
		}, nil
	}
	if err := s.accurateSession.SetEmail(s.accountEmail); err != nil {
		s.accurateSession.InvalidateSession()
		return nil, huma.Error500InternalServerError("Failed to persist Accurate session")
	}

	return &AccurateLoginOutput{
		Body: struct {
			Status      string `json:"status" doc:"'success' or 'mfa_required'"`
			ChallengeID string `json:"challengeId,omitempty" doc:"Challenge ID if mfa_required"`
		}{
			Status: string(accurate.LoginSuccess),
		},
	}, nil
}

// 3. POST /api/v1/accurate/logout
type AccurateLogoutOutput struct {
	Body struct {
		Success bool `json:"success"`
	}
}

func (s *Server) handleAccurateLogout(ctx context.Context, input *accurateAccountQuery) (*AccurateLogoutOutput, error) {
	if input != nil {
		resolved, err := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
		if err != nil {
			return nil, err
		}
		s = resolved
		defer s.releaseAccountOperation()
	}
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}
	if err := s.accurateSession.Logout(); err != nil {
		return nil, huma.Error500InternalServerError("Failed to clear Accurate session")
	}
	return &AccurateLogoutOutput{
		Body: struct {
			Success bool `json:"success"`
		}{Success: true},
	}, nil
}

// 4. GET /api/v1/accurate/databases
type AccurateDatabasesOutput struct {
	Body []accurate.Database
}

func (s *Server) handleAccurateDatabases(ctx context.Context, input *accurateAccountQuery) (*AccurateDatabasesOutput, error) {
	if input != nil {
		resolved, err := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
		if err != nil {
			return nil, err
		}
		s = resolved
		defer s.releaseAccountOperation()
	}
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}
	dbs, err := s.accurateProvider.ListDatabases(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("Failed to list Accurate databases")
	}
	return &AccurateDatabasesOutput{Body: dbs}, nil
}

// 5. POST /api/v1/accurate/database
type AccurateSelectDatabaseInput struct {
	AccountID string `query:"accountId"`
	Body      struct {
		ID       *int64  `json:"id,omitempty"`
		UniqueID *string `json:"uniqueId,omitempty"`
	}
}
type AccurateSelectDatabaseOutput struct {
	Body struct {
		Success  bool              `json:"success"`
		Database accurate.Database `json:"database"`
	}
}

func (s *Server) handleAccurateSelectDatabase(ctx context.Context, input *AccurateSelectDatabaseInput) (*AccurateSelectDatabaseOutput, error) {
	if input != nil {
		resolved, err := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
		if err != nil {
			return nil, err
		}
		s = resolved
		defer s.releaseAccountOperation()
	}
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	if input == nil || (input.Body.ID == nil && (input.Body.UniqueID == nil || *input.Body.UniqueID == "")) {
		return nil, huma.Error400BadRequest("Either database id or uniqueId is required")
	}

	dbs, err := s.accurateProvider.ListDatabases(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("Failed to fetch Accurate databases")
	}

	var target *accurate.Database
	for _, db := range dbs {
		if input.Body.ID != nil && db.ID == *input.Body.ID {
			target = &db
			break
		}
		if input.Body.UniqueID != nil && strings.EqualFold(db.UniqueID, *input.Body.UniqueID) {
			target = &db
			break
		}
	}

	if target == nil {
		return nil, huma.Error404NotFound("Selected database not found in accessible databases")
	}

	openedDB, err := s.accurateProvider.OpenDatabase(ctx, target.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to establish Accurate database session")
	}

	if target.Alias != "" && openedDB.Alias == "" {
		openedDB.Alias = target.Alias
	}

	if wp, ok := s.accurateProvider.(*web.Provider); ok {
		ts, ok := wp.TenantSession()
		if !ok {
			return nil, huma.Error500InternalServerError("Accurate provider did not establish a tenant session")
		}
		if err := s.accurateSession.SetTenantSession(ts); err != nil {
			return nil, huma.Error500InternalServerError("Failed to persist Accurate database session")
		}
	} else if op, ok := s.accurateProvider.(*official.Provider); ok {
		host := op.Host()
		if host == "" && openedDB.Host != "" {
			host = openedDB.Host
			op.SetSession(host, op.SessionID(), openedDB)
		}
		if err := s.accurateSession.SetOfficialSession(&accurate.OfficialSessionState{DatabaseID: openedDB.ID, Host: host, Session: op.SessionID(), Database: openedDB}); err != nil {
			return nil, huma.Error500InternalServerError("Failed to persist Accurate database session")
		}
	} else if s.accurateProvider.ID() == string(accurate.ProviderOfficial) {
		host := openedDB.Host
		if err := s.accurateSession.SetOfficialSession(&accurate.OfficialSessionState{DatabaseID: openedDB.ID, Host: host, Database: openedDB}); err != nil {
			return nil, huma.Error500InternalServerError("Failed to persist Accurate database session")
		}
	}

	return &AccurateSelectDatabaseOutput{
		Body: struct {
			Success  bool              `json:"success"`
			Database accurate.Database `json:"database"`
		}{
			Success:  true,
			Database: openedDB,
		},
	}, nil
}

// 6. GET /api/v1/accurate/items
type AccurateItemsInput struct {
	AccountID string `query:"accountId"`
	Q         string `query:"q" doc:"Search query keyword"`
	Page      int    `query:"page" default:"1" doc:"1-based page index"`
	PageSize  int    `query:"pageSize" default:"50" doc:"Items per page"`
}

type AccurateItemsOutput struct {
	Body accurate.SearchItemsResult
}

func (s *Server) handleAccurateItems(ctx context.Context, input *AccurateItemsInput) (*AccurateItemsOutput, error) {
	resolved, resolveErr := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
	if resolveErr != nil {
		return nil, resolveErr
	}
	s = resolved
	defer s.releaseAccountOperation()
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	if !s.accurateSession.Status().Connected {
		return nil, huma.Error401Unauthorized("No active Accurate database session. Please connect and select a database first.")
	}

	page := 1
	pageSize := 50
	query := ""
	if input != nil {
		query = input.Q
		if input.Page >= 1 {
			page = input.Page
		}
		if input.PageSize >= 1 {
			pageSize = input.PageSize
			if pageSize > 200 {
				pageSize = 200
			}
		}
	}

	res, err := adapter.SearchItems(ctx, s.accurateProvider, query, page, pageSize)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			s.accurateSession.InvalidateSession()
			return nil, huma.Error401Unauthorized(ErrAccurateSessionInvalidatedDetail)
		}
		return nil, huma.Error500InternalServerError("Failed to search Accurate items")
	}

	return &AccurateItemsOutput{Body: *res}, nil
}

// 7. GET /api/v1/accurate/sources
type AccurateSourcesOutput struct {
	Body []accurate.SourceInfo
}

func (s *Server) handleAccurateSources(ctx context.Context, input *accurateAccountQuery) (*AccurateSourcesOutput, error) {
	if input != nil {
		resolved, err := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
		if err != nil {
			return nil, err
		}
		s = resolved
		defer s.releaseAccountOperation()
	}
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}
	if s.accurateRegistry == nil {
		return &AccurateSourcesOutput{Body: []accurate.SourceInfo{}}, nil
	}
	return &AccurateSourcesOutput{Body: s.accurateRegistry.List()}, nil
}

// 8. GET /api/v1/accurate/sources/{source}/documents
type AccurateSourceDocumentsInput struct {
	AccountID string `query:"accountId"`
	Source    string `path:"source" doc:"Source adapter ID"`
	Q         string `query:"q" doc:"Search keyword"`
	From      string `query:"from" doc:"Filter start date (DD/MM/YYYY)"`
	To        string `query:"to" doc:"Filter end date (DD/MM/YYYY)"`
	Page      int    `query:"page" default:"1"`
	PageSize  int    `query:"pageSize" default:"20"`
}

type AccurateSourceDocumentsOutput struct {
	Body struct {
		Documents  []accurate.DocumentHeader `json:"documents"`
		Pagination accurate.Pagination       `json:"pagination"`
	}
}

func (s *Server) handleAccurateSourceDocuments(ctx context.Context, input *AccurateSourceDocumentsInput) (*AccurateSourceDocumentsOutput, error) {
	resolved, resolveErr := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
	if resolveErr != nil {
		return nil, resolveErr
	}
	s = resolved
	defer s.releaseAccountOperation()
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	if s.accurateRegistry == nil {
		return nil, huma.Error404NotFound("Source adapters not configured")
	}

	src, ok := s.accurateRegistry.Get(input.Source)
	if !ok {
		return nil, huma.Error404NotFound(fmt.Sprintf("Source adapter '%s' not found", input.Source))
	}

	if !src.IsAvailable() {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Source '%s' is unavailable: %s", input.Source, src.UnavailableReason()))
	}

	if !s.accurateSession.Status().Connected {
		return nil, huma.Error401Unauthorized("No active Accurate database session. Please connect first.")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	docs, p, err := src.ListDocuments(ctx, s.accurateProvider, input.Q, input.From, input.To, page, pageSize)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			s.accurateSession.InvalidateSession()
			return nil, huma.Error401Unauthorized(ErrAccurateSessionInvalidatedDetail)
		}
		return nil, huma.Error500InternalServerError("Failed to list Accurate source documents")
	}

	return &AccurateSourceDocumentsOutput{
		Body: struct {
			Documents  []accurate.DocumentHeader `json:"documents"`
			Pagination accurate.Pagination       `json:"pagination"`
		}{
			Documents:  docs,
			Pagination: p,
		},
	}, nil
}

// 9. GET /api/v1/accurate/sources/{source}/documents/{id}/items
type AccurateSourceDocumentItemsInput struct {
	AccountID string `query:"accountId"`
	Source    string `path:"source" doc:"Source adapter ID"`
	ID        string `path:"id" doc:"Document ID"`
}

type AccurateSourceDocumentItemsOutput struct {
	Body []accurate.NormalizedItem
}

func (s *Server) handleAccurateSourceDocumentItems(ctx context.Context, input *AccurateSourceDocumentItemsInput) (*AccurateSourceDocumentItemsOutput, error) {
	resolved, resolveErr := s.accountServerForRequest(ctx, accountIDParam(input.AccountID))
	if resolveErr != nil {
		return nil, resolveErr
	}
	s = resolved
	defer s.releaseAccountOperation()
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	if s.accurateRegistry == nil {
		return nil, huma.Error404NotFound("Source adapters not configured")
	}

	src, ok := s.accurateRegistry.Get(input.Source)
	if !ok {
		return nil, huma.Error404NotFound(fmt.Sprintf("Source adapter '%s' not found", input.Source))
	}

	if !src.IsAvailable() {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Source '%s' is unavailable: %s", input.Source, src.UnavailableReason()))
	}

	docID, err := strconv.ParseInt(input.ID, 10, 64)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid document ID")
	}

	if !s.accurateSession.Status().Connected {
		return nil, huma.Error401Unauthorized("No active Accurate database session. Please connect first.")
	}

	items, err := src.GetDocumentItems(ctx, s.accurateProvider, docID)
	if err != nil {
		if errors.Is(err, accurate.ErrSessionExpired) {
			s.accurateSession.InvalidateSession()
			return nil, huma.Error401Unauthorized(ErrAccurateSessionInvalidatedDetail)
		}
		return nil, huma.Error500InternalServerError("Failed to fetch Accurate document items")
	}

	return &AccurateSourceDocumentItemsOutput{Body: items}, nil
}
