package model

import (
	"time"
)

// User represents an authenticated system user.
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	Role      string    `json:"role"`
	Locale    string    `json:"locale"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LabelLayoutPreset stores a reusable label-element arrangement independently of paper dimensions.
type LabelLayoutPreset struct {
	ID          int64  `json:"id,omitempty"`
	Name        string `json:"name"`
	LabelLayout string `json:"label_layout"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type BarcodePresetSettings struct {
	Symbology        string   `json:"symbology"`
	BarcodeHeight    string   `json:"barcode_height"`
	FontSize         string   `json:"font_size"`
	QtyMode          string   `json:"qty_mode"`
	SkipCount        *int     `json:"skip_count,omitempty"`
	ShowScreenBorder *bool    `json:"show_screen_border,omitempty"`
	Zoom             *float64 `json:"zoom,omitempty"`
}

func DefaultBarcodePresetSettings() BarcodePresetSettings {
	skipCount := 0
	showScreenBorder := true
	zoom := 1.0
	return BarcodePresetSettings{
		Symbology:        string(BarcodeCode128),
		BarcodeHeight:    string(BarcodeHeightMedium),
		FontSize:         string(BarcodeFontMedium),
		QtyMode:          string(BarcodeQuantityMultiply),
		SkipCount:        &skipCount,
		ShowScreenBorder: &showScreenBorder,
		Zoom:             &zoom,
	}
}

// PageDesign represents a customizable barcode printing layout.
type PageDesign struct {
	ID            int64                  `json:"id,omitempty"`
	Name          string                 `json:"name"`
	Paper         string                 `json:"paper,omitempty"`
	PaperWidthMM  float64                `json:"paper_width_mm,omitempty"`
	PaperHeightMM float64                `json:"paper_height_mm,omitempty"`
	LabelWidthMM  float64                `json:"label_width_mm"`
	LabelHeightMM float64                `json:"label_height_mm"`
	Cols          int                    `json:"cols"`
	Rows          int                    `json:"rows"`
	GapXMM        float64                `json:"gap_x_mm,omitempty"`
	GapYMM        float64                `json:"gap_y_mm,omitempty"`
	MarginTopMM   float64                `json:"margin_top_mm,omitempty"`
	MarginLeftMM  float64                `json:"margin_left_mm,omitempty"`
	IsAuditSheet  bool                   `json:"is_audit_sheet,omitempty"`
	LabelLayout   *string                `json:"label_layout,omitempty"`
	Settings      *BarcodePresetSettings `json:"settings,omitempty"`
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
}
