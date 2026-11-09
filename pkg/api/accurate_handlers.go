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
	"github.com/danielgtaylor/huma/v2"
)

const ErrAccurateSessionInvalidatedDetail = "Your Accurate session was invalidated or expired. Please sign in again."

func (s *Server) checkAccurateEnabled() error {
	if s.accurateProvider == nil || s.accurateSession == nil || s.appConfig == nil || s.appConfig.AccurateProvider == "" {
		return huma.Error401Unauthorized("Accurate integration is not enabled")
	}
	return nil
}

// 1. GET /api/v1/accurate/status
type AccurateStatusOutput struct {
	Body struct {
		Enabled   bool               `json:"enabled"`
		Provider  string             `json:"provider,omitempty"`
		Connected bool               `json:"connected"`
		Database  *accurate.Database `json:"database,omitempty"`
	}
}

func (s *Server) handleAccurateStatus(ctx context.Context, input *struct{}) (*AccurateStatusOutput, error) {
	if s.accurateProvider == nil || s.accurateSession == nil || s.appConfig == nil || s.appConfig.AccurateProvider == "" {
		return &AccurateStatusOutput{
			Body: struct {
				Enabled   bool               `json:"enabled"`
				Provider  string             `json:"provider,omitempty"`
				Connected bool               `json:"connected"`
				Database  *accurate.Database `json:"database,omitempty"`
			}{
				Enabled:   false,
				Connected: false,
			},
		}, nil
	}

	st := s.accurateSession.Status()
	return &AccurateStatusOutput{
		Body: struct {
			Enabled   bool               `json:"enabled"`
			Provider  string             `json:"provider,omitempty"`
			Connected bool               `json:"connected"`
			Database  *accurate.Database `json:"database,omitempty"`
		}{
			Enabled:   true,
			Provider:  s.accurateProvider.ID(),
			Connected: st.Connected,
			Database:  st.Database,
		},
	}, nil
}

// 2. POST /api/v1/accurate/login
type AccurateLoginInput struct {
	Body struct {
		ChallengeID *string `json:"challengeId,omitempty" doc:"Pending MFA challenge ID when completing MFA"`
		OTP         *string `json:"otp,omitempty" doc:"One-time code from authenticator app"`
	}
}

type AccurateLoginOutput struct {
	Body struct {
		Status      string `json:"status" doc:"'success' or 'mfa_required'"`
		ChallengeID string `json:"challengeId,omitempty" doc:"Challenge ID if mfa_required"`
	}
}

func (s *Server) handleAccurateLogin(ctx context.Context, input *AccurateLoginInput) (*AccurateLoginOutput, error) {
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
		return nil, huma.Error401Unauthorized(fmt.Sprintf("Login failed: %v", err))
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

func (s *Server) handleAccurateLogout(ctx context.Context, input *struct{}) (*AccurateLogoutOutput, error) {
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}
	s.accurateSession.Logout()
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

func (s *Server) handleAccurateDatabases(ctx context.Context, input *struct{}) (*AccurateDatabasesOutput, error) {
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}
	dbs, err := s.accurateProvider.ListDatabases(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized(fmt.Sprintf("Failed to list databases: %v", err))
	}
	return &AccurateDatabasesOutput{Body: dbs}, nil
}

// 5. POST /api/v1/accurate/database
type AccurateSelectDatabaseInput struct {
	Body struct {
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
	if err := s.checkAccurateEnabled(); err != nil {
		return nil, err
	}

	if input == nil || (input.Body.ID == nil && (input.Body.UniqueID == nil || *input.Body.UniqueID == "")) {
		return nil, huma.Error400BadRequest("Either database id or uniqueId is required")
	}

	dbs, err := s.accurateProvider.ListDatabases(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized(fmt.Sprintf("Failed to fetch databases: %v", err))
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
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to establish database session: %v", err))
	}

	if target.Alias != "" && openedDB.Alias == "" {
		openedDB.Alias = target.Alias
	}

	if wp, ok := s.accurateProvider.(*web.Provider); ok {
		if ts, ok := wp.TenantSession(); ok {
			s.accurateSession.SetTenantSession(ts)
		}
	} else if op, ok := s.accurateProvider.(*official.Provider); ok {
		host := op.Host()
		if host == "" && openedDB.Host != "" {
			host = openedDB.Host
			op.SetSession(host, op.SessionID(), openedDB)
		}
		s.accurateSession.SetOfficialSession(&accurate.OfficialSessionState{
			DatabaseID: openedDB.ID,
			Host:       host,
			Session:    op.SessionID(),
			Database:   openedDB,
		})
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
	Q        string `query:"q" doc:"Search query keyword"`
	Page     int    `query:"page" default:"1" doc:"1-based page index"`
	PageSize int    `query:"pageSize" default:"50" doc:"Items per page"`
}

type AccurateItemsOutput struct {
	Body accurate.SearchItemsResult
}

func (s *Server) handleAccurateItems(ctx context.Context, input *AccurateItemsInput) (*AccurateItemsOutput, error) {
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
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to search items: %v", err))
	}

	return &AccurateItemsOutput{Body: *res}, nil
}

// 7. GET /api/v1/accurate/sources
type AccurateSourcesOutput struct {
	Body []accurate.SourceInfo
}

func (s *Server) handleAccurateSources(ctx context.Context, input *struct{}) (*AccurateSourcesOutput, error) {
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
	Source   string `path:"source" doc:"Source adapter ID"`
	Q        string `query:"q" doc:"Search keyword"`
	From     string `query:"from" doc:"Filter start date (DD/MM/YYYY)"`
	To       string `query:"to" doc:"Filter end date (DD/MM/YYYY)"`
	Page     int    `query:"page" default:"1"`
	PageSize int    `query:"pageSize" default:"20"`
}

type AccurateSourceDocumentsOutput struct {
	Body struct {
		Documents  []accurate.DocumentHeader `json:"documents"`
		Pagination accurate.Pagination       `json:"pagination"`
	}
}

func (s *Server) handleAccurateSourceDocuments(ctx context.Context, input *AccurateSourceDocumentsInput) (*AccurateSourceDocumentsOutput, error) {
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
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to list documents for source '%s': %v", input.Source, err))
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
	Source string `path:"source" doc:"Source adapter ID"`
	ID     string `path:"id" doc:"Document ID"`
}

type AccurateSourceDocumentItemsOutput struct {
	Body []accurate.NormalizedItem
}

func (s *Server) handleAccurateSourceDocumentItems(ctx context.Context, input *AccurateSourceDocumentItemsInput) (*AccurateSourceDocumentItemsOutput, error) {
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
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to fetch document items: %v", err))
	}

	return &AccurateSourceDocumentItemsOutput{Body: items}, nil
}
