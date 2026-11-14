package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/version"
	"github.com/danielgtaylor/huma/v2"
)

type ResultStatus string

const ResultOK ResultStatus = "ok"

// HealthResponse defines the health check response.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type HealthOutput struct {
	Body HealthResponse
}

func (s *Server) handleHealth(ctx context.Context, input *struct{}) (*HealthOutput, error) {
	return &HealthOutput{
		Body: HealthResponse{
			Status: string(ResultOK),
		},
	}, nil
}

// SystemResponse defines system diagnostics information.
type SystemResponse struct {
	Version       string                       `json:"version"`
	GoVersion     string                       `json:"go_version"`
	UptimeSeconds int64                        `json:"uptime_seconds"`
	Port          string                       `json:"port"`
	DBPath        string                       `json:"db_path"`
	Tables        map[string]storage.TableStat `json:"tables"`
}

type SystemOutput struct {
	Body SystemResponse
}

func (s *Server) handleSystem(ctx context.Context, input *struct{}) (*SystemOutput, error) {
	cfg := s.appConfig
	port := "8080"
	dbPath := "data/barcode.db"
	if cfg != nil {
		port = cfg.Port
		dbPath = cfg.DBPath
	}
	if s.store != nil && s.store.IsPostgres() {
		dbPath = "postgresql"
	}

	var tables map[string]storage.TableStat
	if s.store != nil {
		tStats, err := s.store.GetTableStats(ctx)
		if err == nil {
			tables = tStats
		}
	}
	if tables == nil {
		tables = make(map[string]storage.TableStat)
	}

	uptime := int64(0)
	if !s.startTime.IsZero() {
		uptime = int64(time.Since(s.startTime).Seconds())
	}

	resp := SystemResponse{
		Version:       version.Version,
		GoVersion:     runtime.Version(),
		UptimeSeconds: uptime,
		Port:          port,
		DBPath:        dbPath,
		Tables:        tables,
	}
	return &SystemOutput{Body: resp}, nil
}

type StatusOKResponse struct {
	Status string `json:"status" example:"ok"`
}

type StatusOKOutput struct {
	Body StatusOKResponse
}

type SetLocaleInput struct {
	Body struct {
		Locale string `json:"locale" enum:"en,id" doc:"Preferred language locale ('en' or 'id')"`
	}
}

type SetLocaleOutput struct {
	Body struct {
		Locale string `json:"locale"`
	}
}

func (s *Server) handleSetUserLocale(ctx context.Context, input *SetLocaleInput) (*SetLocaleOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	locale := strings.ToLower(strings.TrimSpace(input.Body.Locale))
	if model.Locale(locale) != model.LocaleEnglish && model.Locale(locale) != model.LocaleIndonesian {
		return nil, huma.Error400BadRequest("invalid locale: must be 'en' or 'id'")
	}
	claims := claimsFromContext(ctx)
	var email string
	if claims != nil {
		email = claims.Email
	}
	if email == "" {
		return &SetLocaleOutput{Body: struct {
			Locale string `json:"locale"`
		}{Locale: locale}}, nil
	}
	if err := s.store.UpdateUserLocale(ctx, email, locale); err != nil {
		return nil, huma.Error500InternalServerError("failed to update locale", err)
	}
	return &SetLocaleOutput{Body: struct {
		Locale string `json:"locale"`
	}{Locale: locale}}, nil
}

type PageDesignsOutput struct {
	Body struct {
		Designs []model.PageDesign `json:"designs"`
	}
}

type CreatePageDesignOutput struct {
	Body struct {
		Status string `json:"status"`
		ID     int64  `json:"id"`
	}
}

type CreatePageDesignInput struct {
	Body model.PageDesign
}

type UpdatePageDesignInput struct {
	ID   int64 `path:"id" doc:"Page design ID"`
	Body model.PageDesign
}

type DeletePageDesignInput struct {
	ID int64 `path:"id" doc:"Page design ID"`
}

func validatePageDesign(d *model.PageDesign) error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return huma.Error400BadRequest("name cannot be empty")
	}
	if d.LabelWidthMM <= 0 {
		return huma.Error400BadRequest("label_width_mm must be greater than 0")
	}
	if d.LabelHeightMM <= 0 {
		return huma.Error400BadRequest("label_height_mm must be greater than 0")
	}
	if d.Cols < 1 {
		return huma.Error400BadRequest("cols must be at least 1")
	}
	if d.Rows < 1 {
		return huma.Error400BadRequest("rows must be at least 1")
	}
	if d.PaperWidthMM < 0 {
		return huma.Error400BadRequest("paper_width_mm cannot be negative")
	}
	if d.PaperHeightMM < 0 {
		return huma.Error400BadRequest("paper_height_mm cannot be negative")
	}
	if d.PaperWidthMM == 0 {
		d.PaperWidthMM = 210
	}
	if d.PaperHeightMM == 0 {
		d.PaperHeightMM = 297
	}
	d.Paper = strings.TrimSpace(d.Paper)
	if d.Paper == "" {
		d.Paper = "A4"
	}
	if d.Settings == nil {
		defaults := model.DefaultBarcodePresetSettings()
		d.Settings = &defaults
	} else {
		settings := d.Settings
		switch model.BarcodeSymbology(settings.Symbology) {
		case model.BarcodeCode128, model.BarcodeCode39, model.BarcodeEAN13, model.BarcodeQRCode, model.BarcodeDataMatrix:
		default:
			return huma.Error400BadRequest("settings.symbology is invalid")
		}
		switch model.BarcodeHeight(settings.BarcodeHeight) {
		case model.BarcodeHeightCompact, model.BarcodeHeightMedium, model.BarcodeHeightLarge:
		default:
			return huma.Error400BadRequest("settings.barcode_height is invalid")
		}
		switch model.BarcodeFontSize(settings.FontSize) {
		case model.BarcodeFontSmall, model.BarcodeFontMedium, model.BarcodeFontLarge:
		default:
			return huma.Error400BadRequest("settings.font_size is invalid")
		}
		switch model.BarcodeQuantityMode(settings.QtyMode) {
		case model.BarcodeQuantitySingle, model.BarcodeQuantityMultiply:
		default:
			return huma.Error400BadRequest("settings.qty_mode is invalid")
		}
		if settings.SkipCount == nil || settings.ShowScreenBorder == nil || settings.Zoom == nil {
			return huma.Error400BadRequest("all settings fields are required")
		}
		if *settings.SkipCount < 0 || *settings.SkipCount > 50 {
			return huma.Error400BadRequest("settings.skip_count must be between 0 and 50")
		}
		if math.IsNaN(*settings.Zoom) || math.IsInf(*settings.Zoom, 0) || *settings.Zoom < 0.5 || *settings.Zoom > 1.5 {
			return huma.Error400BadRequest("settings.zoom must be between 0.5 and 1.5")
		}
	}
	return nil
}

func (s *Server) handleGetPageDesigns(ctx context.Context, input *struct{}) (*PageDesignsOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	designs, err := s.store.GetPageDesigns(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get page designs", err)
	}
	out := &PageDesignsOutput{}
	out.Body.Designs = designs
	return out, nil
}

func (s *Server) handleCreatePageDesign(ctx context.Context, input *CreatePageDesignInput) (*CreatePageDesignOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	design := input.Body
	if err := validatePageDesign(&design); err != nil {
		return nil, err
	}
	id, err := s.store.CreatePageDesign(ctx, design)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create page design", err)
	}
	out := &CreatePageDesignOutput{}
	out.Body.Status = "ok"
	out.Body.ID = id
	return out, nil
}

func (s *Server) handleUpdatePageDesign(ctx context.Context, input *UpdatePageDesignInput) (*StatusOKOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	if input.ID <= 0 {
		return nil, huma.Error400BadRequest("invalid page design id")
	}
	design := input.Body
	if err := validatePageDesign(&design); err != nil {
		return nil, err
	}
	if err := s.store.UpdatePageDesign(ctx, input.ID, design); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("page design %d not found", input.ID))
		}
		return nil, huma.Error500InternalServerError("failed to update page design", err)
	}
	return &StatusOKOutput{
		Body: StatusOKResponse{Status: string(ResultOK)},
	}, nil
}

func (s *Server) handleDeletePageDesign(ctx context.Context, input *DeletePageDesignInput) (*StatusOKOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	if input.ID <= 0 {
		return nil, huma.Error400BadRequest("invalid page design id")
	}
	if err := s.store.DeletePageDesign(ctx, input.ID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("page design %d not found", input.ID))
		}
		return nil, huma.Error500InternalServerError("failed to delete page design", err)
	}
	return &StatusOKOutput{
		Body: StatusOKResponse{Status: string(ResultOK)},
	}, nil
}

type LabelLayoutPresetsOutput struct {
	Body struct {
		Presets []model.LabelLayoutPreset `json:"presets"`
	}
}

type CreateLabelLayoutPresetOutput struct {
	Body struct {
		Status string `json:"status"`
		ID     int64  `json:"id"`
	}
}

type CreateLabelLayoutPresetInput struct {
	Body model.LabelLayoutPreset
}

type UpdateLabelLayoutPresetInput struct {
	ID   int64 `path:"id" doc:"Label layout preset ID"`
	Body model.LabelLayoutPreset
}

type LabelLayoutPresetIDInput struct {
	ID int64 `path:"id" doc:"Label layout preset ID"`
}

func validateLabelLayoutPreset(preset *model.LabelLayoutPreset) error {
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		return huma.Error400BadRequest("name cannot be empty")
	}
	if !json.Valid([]byte(preset.LabelLayout)) {
		return huma.Error400BadRequest("label_layout must be valid JSON")
	}
	return nil
}

func (s *Server) handleGetLabelLayoutPresets(ctx context.Context, input *struct{}) (*LabelLayoutPresetsOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	presets, err := s.store.GetLabelLayoutPresets(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get label layout presets", err)
	}
	out := &LabelLayoutPresetsOutput{}
	out.Body.Presets = presets
	return out, nil
}

func (s *Server) handleCreateLabelLayoutPreset(ctx context.Context, input *CreateLabelLayoutPresetInput) (*CreateLabelLayoutPresetOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	preset := input.Body
	if err := validateLabelLayoutPreset(&preset); err != nil {
		return nil, err
	}
	id, err := s.store.CreateLabelLayoutPreset(ctx, preset)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create label layout preset", err)
	}
	out := &CreateLabelLayoutPresetOutput{}
	out.Body.Status = "ok"
	out.Body.ID = id
	return out, nil
}

func (s *Server) handleUpdateLabelLayoutPreset(ctx context.Context, input *UpdateLabelLayoutPresetInput) (*StatusOKOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	if input.ID <= 0 {
		return nil, huma.Error400BadRequest("invalid label layout preset id")
	}
	preset := input.Body
	if err := validateLabelLayoutPreset(&preset); err != nil {
		return nil, err
	}
	if err := s.store.UpdateLabelLayoutPreset(ctx, input.ID, preset); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("label layout preset %d not found", input.ID))
		}
		return nil, huma.Error500InternalServerError("failed to update label layout preset", err)
	}
	return &StatusOKOutput{Body: StatusOKResponse{Status: string(ResultOK)}}, nil
}

func (s *Server) handleDeleteLabelLayoutPreset(ctx context.Context, input *LabelLayoutPresetIDInput) (*StatusOKOutput, error) {
	if s.store == nil {
		return nil, huma.Error503ServiceUnavailable("storage service not available")
	}
	if input.ID <= 0 {
		return nil, huma.Error400BadRequest("invalid label layout preset id")
	}
	if err := s.store.DeleteLabelLayoutPreset(ctx, input.ID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("label layout preset %d not found", input.ID))
		}
		return nil, huma.Error500InternalServerError("failed to delete label layout preset", err)
	}
	return &StatusOKOutput{Body: StatusOKResponse{Status: string(ResultOK)}}, nil
}
