import type { BarcodeHeight, BarcodeSymbology, FontSize, QtyMode, UserRole } from "~/domain";
export type { BarcodeHeight, FontSize, QtyMode } from "~/domain";

export interface User {
  id: number;
  email: string;
  name: string;
  picture: string;
  role: UserRole;
  created_at: string;
  updated_at: string;
}

export interface UsersResponse {
  users: User[];
}

export interface Role {
  id: number;
  name: string;
  description: string;
  is_system: boolean;
  permissions: string[];
  created_at: string;
  updated_at: string;
}

export interface RolesResponse {
  roles: Role[];
}

export interface PermissionsResponse {
  permissions: string[];
}

export interface SystemResponse {
  version: string;
  go_version: string;
  uptime_seconds: number;
  port: string;
  db_path: string;
  tables: Record<string, { row_count: number }>;
}

export interface AccurateDatabase {
  id: number;
  alias: string;
  host: string;
  uniqueId?: string;
  isPrivateCloud?: boolean;
}

export interface AccurateStatus {
  enabled: boolean;
  provider?: string;
  connected: boolean;
	accountId?: number;
	accountName?: string;
	 database?: AccurateDatabase;
}
export interface AccurateAccount {
  id: number;
  name: string;
  provider: 'web' | 'official';
  enabled: boolean;
  email: string;
  has_password: boolean;
  has_api_token: boolean;
  has_signature_secret: boolean;
  is_default: boolean;
  created_at: string;
  updated_at: string;
  connected: boolean;
  database?: AccurateDatabase;
}

export interface AccuratePagination {
  page: number;
  pageCount: number;
  pageSize: number;
  rowCount: number;
}

export interface AccurateInventoryItem {
  id: number;
  no: string;
  name: string;
  upcNo: string;
  unit1Name: string;
  unit2Name?: string;
  unitPrice: number;
  allQuantity: number;
  availableToSell: number;
  itemType: string;
  itemTypeName: string;
  canPrintBarcode: boolean;
}

export interface AccurateSearchItemsResult {
  items: AccurateInventoryItem[];
  pagination: AccuratePagination;
}

export interface AccurateSourceInfo {
  id: string;
  name: string;
  dataPlan: string;
  available: boolean;
  reason?: string;
}

export interface AccurateDocumentHeader {
  id: number;
  number: string;
  date: string;
  description: string;
  statusName: string;
}

export interface AccurateDocumentsResult {
  documents: AccurateDocumentHeader[];
  pagination: AccuratePagination;
}

export interface AccurateNormalizedItem {
  itemId: number;
  itemNo: string;
  itemName: string;
  upcNo: string;
  quantity: number;
  unitName: string;
  unitPrice?: number;
  documentType: string;
  documentNumber: string;
}

export interface BarcodeItem {
  id: string;
  selected: boolean;
  itemNo: string;
  itemName: string;
  barcode: string;
  quantity: number;
  unit: string;
  price: number;
  documentType?: string;
  documentNumber?: string;
}

export interface PresetConfig {
  name: string;
  paper: string;
  paperWidthMM: number;
  paperHeightMM: number;
  labelWidthMM: number;
  labelHeightMM: number;
  cols: number;
  rows: number;
  gapXMM: number;
  gapYMM: number;
  marginTopMM: number;
  marginLeftMM: number;
  isAuditSheet: boolean;
}

export type LabelElementType =
  | 'company'
  | 'itemName'
  | 'barcode'
  | 'barcodeText'
  | 'itemCode'
  | 'price'
  | 'unit'
  | 'separator'
  | 'customText';

export interface LabelElement {
  id: string;
  type: LabelElementType;
  x: number;
  y: number;
  w: number;
  h: number;
  align: 'left' | 'center' | 'right';
  fontScale: number;
  visible: boolean;
  customText?: string;
}

export type LabelLayout = LabelElement[];

export interface LabelLayoutPreset {
  id: number;
  name: string;
  label_layout: string;
  created_at?: string;
  updated_at?: string;
}

export interface LabelLayoutPresetsResponse {
  presets: LabelLayoutPreset[];
}

export type Symbology = BarcodeSymbology;

export interface BarcodePresetSettings {
  symbology: Symbology;
  barcode_height: BarcodeHeight;
  font_size: FontSize;
  qty_mode: QtyMode;
  skip_count: number;
  show_screen_border: boolean;
  zoom: number;
}

export interface PageDesign {
  id: number;
  name: string;
  paper: string;
  paper_width_mm: number;
  paper_height_mm: number;
  label_width_mm: number;
  label_height_mm: number;
  cols: number;
  rows: number;
  gap_x_mm: number;
  gap_y_mm: number;
  margin_top_mm: number;
  margin_left_mm: number;
  is_audit_sheet: boolean;
  label_layout?: string;
  settings: BarcodePresetSettings;
  created_at?: string;
  updated_at?: string;
}

export interface PageDesignsResponse {
  designs: PageDesign[];
}
