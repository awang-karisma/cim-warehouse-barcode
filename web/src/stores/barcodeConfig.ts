import { createSignal, createResource, createEffect } from 'solid-js';
import { BARCODE_HEIGHT, BARCODE_SYMBOLOGY, FONT_SIZE, QTY_MODE } from '~/domain';
import { getPageDesigns } from '~/api';
import { loadLayoutFromDesign, resetLayout } from '~/stores/labelLayout';
export { companyName, setCompanyName } from '~/stores/labelLayout';
import type { BarcodeHeight, BarcodePresetSettings, FontSize, PageDesign, PresetConfig, QtyMode, Symbology } from '~/types';
export const SERVER_PRESET_PREFIX = 'server-';
export const PRESETS = {
  'tj-108': {
    name: 'Tom & Jerry No. 108',
    paper: 'A4',
    paperWidthMM: 210,
    paperHeightMM: 297,
    labelWidthMM: 38,
    labelHeightMM: 19,
    cols: 5,
    rows: 14,
    gapXMM: 2.5,
    gapYMM: 1.5,
    marginTopMM: 6,
    marginLeftMM: 5,
    isAuditSheet: false,
  },
  'tj-121': {
    name: 'Tom & Jerry No. 121',
    paper: 'A4',
    paperWidthMM: 210,
    paperHeightMM: 297,
    labelWidthMM: 76,
    labelHeightMM: 38,
    cols: 2,
    rows: 5,
    gapXMM: 8,
    gapYMM: 15,
    marginTopMM: 23,
    marginLeftMM: 25,
    isAuditSheet: false,
  },
  'thermal-50-30': {
    name: 'Continuous Thermal 50x30mm',
    paper: 'Roll',
    paperWidthMM: 50,
    paperHeightMM: 30,
    labelWidthMM: 48,
    labelHeightMM: 28,
    cols: 1,
    rows: 1,
    gapXMM: 0,
    gapYMM: 0,
    marginTopMM: 1,
    marginLeftMM: 1,
    isAuditSheet: false,
  },
  'thermal-40-20': {
    name: 'Continuous Thermal 40x20mm',
    paper: 'Roll',
    paperWidthMM: 40,
    paperHeightMM: 20,
    labelWidthMM: 38,
    labelHeightMM: 18,
    cols: 1,
    rows: 1,
    gapXMM: 0,
    gapYMM: 0,
    marginTopMM: 1,
    marginLeftMM: 1,
    isAuditSheet: false,
  },
  'a4-audit': {
    name: 'Lembar Audit Stok Opname (A4)',
    paper: 'A4',
    paperWidthMM: 210,
    paperHeightMM: 297,
    labelWidthMM: 190,
    labelHeightMM: 15,
    cols: 1,
    rows: 18,
    gapXMM: 0,
    gapYMM: 0,
    marginTopMM: 10,
    marginLeftMM: 10,
    isAuditSheet: true,
  },
  custom: {
    name: 'Kustom Ukuran Sendiri',
    paper: 'A4',
    paperWidthMM: 210,
    paperHeightMM: 297,
    labelWidthMM: 38,
    labelHeightMM: 19,
    cols: 5,
    rows: 14,
    gapXMM: 2.5,
    gapYMM: 1.5,
    marginTopMM: 6,
    marginLeftMM: 5,
    isAuditSheet: false,
  },
} satisfies Record<string, PresetConfig>;

export const [presetKey, setPresetKey] = createSignal<string>('tj-108');
export const [customConfig, setCustomConfig] = createSignal<PresetConfig>({ ...PRESETS.custom });

export const [symbology, setSymbology] = createSignal<Symbology>(BARCODE_SYMBOLOGY.CODE128);
export const [barcodeHeight, setBarcodeHeight] = createSignal<BarcodeHeight>(BARCODE_HEIGHT.MEDIUM);
export const [fontSize, setFontSize] = createSignal<FontSize>(FONT_SIZE.MEDIUM);
export const [qtyMode, setQtyMode] = createSignal<QtyMode>(QTY_MODE.MULTIPLY);
export const [skipCount, setSkipCount] = createSignal<number>(0);

export const [showScreenBorder, setShowScreenBorder] = createSignal<boolean>(true);
export const [zoom, setZoom] = createSignal<number>(1);




export const DEFAULT_BARCODE_PRESET_SETTINGS: BarcodePresetSettings = {
  symbology: BARCODE_SYMBOLOGY.CODE128, barcode_height: BARCODE_HEIGHT.MEDIUM, font_size: FONT_SIZE.MEDIUM, qty_mode: QTY_MODE.MULTIPLY,
  skip_count: 0, show_screen_border: true, zoom: 1,
};

export function captureBarcodePresetSettings(): BarcodePresetSettings {
  return {
    symbology: symbology(), barcode_height: barcodeHeight(), font_size: fontSize(), qty_mode: qtyMode(),
    skip_count: skipCount(), show_screen_border: showScreenBorder(), zoom: zoom(),
  };
}

export function applyBarcodePresetSettings(settings: BarcodePresetSettings): void {
  setSymbology(settings.symbology);
  setBarcodeHeight(settings.barcode_height);
  setFontSize(settings.font_size);
  setQtyMode(settings.qty_mode);
  setSkipCount(settings.skip_count);
  setShowScreenBorder(settings.show_screen_border);
  setZoom(settings.zoom);
}

let presetDraftActive = false;
let lastAppliedDesignSignature = '';
export function setPresetDraftActive(active: boolean): void { presetDraftActive = active; }


export const [serverDesigns, { refetch: refetchDesigns }] = createResource(
  () => getPageDesigns().then((r) => r.designs)
);

export function toPresetConfig(d: PageDesign): PresetConfig {
  return {
    name: d.name,
    paper: d.paper,
    paperWidthMM: d.paper_width_mm,
    paperHeightMM: d.paper_height_mm,
    labelWidthMM: d.label_width_mm,
    labelHeightMM: d.label_height_mm,
    cols: d.cols,
    rows: d.rows,
    gapXMM: d.gap_x_mm,
    gapYMM: d.gap_y_mm,
    marginTopMM: d.margin_top_mm,
    marginLeftMM: d.margin_left_mm,
    isAuditSheet: d.is_audit_sheet,
  };
}

export function fromPresetConfig(
  cfg: PresetConfig,
  labelLayout: string | undefined,
  settings: BarcodePresetSettings
): Omit<PageDesign, 'id' | 'created_at' | 'updated_at'> {
  return {
    name: cfg.name,
    paper: cfg.paper,
    paper_width_mm: cfg.paperWidthMM,
    paper_height_mm: cfg.paperHeightMM,
    label_width_mm: cfg.labelWidthMM,
    label_height_mm: cfg.labelHeightMM,
    cols: cfg.cols,
    rows: cfg.rows,
    gap_x_mm: cfg.gapXMM,
    gap_y_mm: cfg.gapYMM,
    margin_top_mm: cfg.marginTopMM,
    margin_left_mm: cfg.marginLeftMM,
    is_audit_sheet: cfg.isAuditSheet,
    label_layout: labelLayout,
    settings,
  };
}

export function applyPageDesign(design: PageDesign): void {
  const cfg = toPresetConfig(design);
  setCustomConfig(cfg);
  loadLayoutFromDesign(design, cfg.labelWidthMM, cfg.labelHeightMM);
  applyBarcodePresetSettings(design.settings);
  lastAppliedDesignSignature = JSON.stringify(design);
}

const applyBuiltinPreset = (key: keyof typeof PRESETS): void => {
	const cfg = PRESETS[key] || PRESETS['tj-108'];
  setCustomConfig({ ...cfg });
  applyBarcodePresetSettings(DEFAULT_BARCODE_PRESET_SETTINGS);
  resetLayout(cfg.labelWidthMM, cfg.labelHeightMM);
};

createEffect(() => {
  const key = presetKey();
  const designs = serverDesigns();
	if (!key.startsWith(SERVER_PRESET_PREFIX) || !designs) return;
	const id = Number.parseInt(key.slice(SERVER_PRESET_PREFIX.length), 10);
  const match = designs.find((design) => design.id === id);
  if (match) {
    const signature = JSON.stringify(match);
    if (!presetDraftActive && signature !== lastAppliedDesignSignature) applyPageDesign(match);
    return;
  }
  if (!presetDraftActive) {
    setPresetKey('tj-108');
    applyBuiltinPreset('tj-108');
  }
});

export function selectPreset(key: string): void {
  lastAppliedDesignSignature = '';
  setPresetKey(key);
	if (key.startsWith(SERVER_PRESET_PREFIX)) {
		const id = Number.parseInt(key.slice(SERVER_PRESET_PREFIX.length), 10);
    const match = (serverDesigns() ?? []).find((design) => design.id === id);
    if (match) applyPageDesign(match);
    return;
  }
	applyBuiltinPreset(key as keyof typeof PRESETS);
}

export function getEffectiveConfig(): PresetConfig {
  const pk = presetKey();
  if (pk === 'custom') {
    return customConfig();
  }
	if (pk.startsWith(SERVER_PRESET_PREFIX)) {
		const id = parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
    const designs = serverDesigns() ?? [];
    const match = designs.find((d) => d.id === id);
    if (match) return customConfig();
    return PRESETS['tj-108'];
  }
	return PRESETS[pk as keyof typeof PRESETS] || PRESETS['tj-108'];
}

