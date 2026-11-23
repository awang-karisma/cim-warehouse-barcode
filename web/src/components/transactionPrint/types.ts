import type { AccurateSource } from '~/domain';
import type { Dictionary } from '~/i18n';
import type {
  AccurateDocumentHeader,
  AccurateNormalizedItem,
  AccuratePagination,
  BarcodeItem,
} from '~/types';

export interface TransactionSampleDocument {
  header: AccurateDocumentHeader;
  supplierOrParty?: string;
  warehouseName?: string;
  items: AccurateNormalizedItem[];
}

export type TranslationStringKey = {
  [K in keyof Dictionary]: Dictionary[K] extends string ? K : never;
}[keyof Dictionary];

export interface TransactionSourceConfig {
  source: AccurateSource;
  titleKey: TranslationStringKey;
  subtitleKey: TranslationStringKey;
  searchPlaceholderKey: TranslationStringKey;
  partyLabelKey?: TranslationStringKey;
  sampleDocuments?: readonly TransactionSampleDocument[];
}

export interface TransactionDocumentRecord {
  header: AccurateDocumentHeader;
  partyName?: string;
  warehouseName?: string;
  isSample: boolean;
}

export interface TransactionLineDraft {
  key: string;
  docId: number;
  linePosition: number;
  original: AccurateNormalizedItem;
  selected: boolean;
  itemName: string;
  quantityText: string;
}

export interface LineValidationError {
  key: string;
  docId: number;
  docNumber: string;
  itemNo: string;
  nameEmpty: boolean;
  quantityInvalid: boolean;
}

export interface DocumentSelectionInfo {
  included: boolean;
  selectedCount: number;
  totalCount: number;
  allSelected: boolean;
  someSelected: boolean;
  loadingItems: boolean;
  itemsLoaded: boolean;
}

export interface LineValidationResult {
  nameEmpty: boolean;
  quantityInvalid: boolean;
  parsedQty: number | null;
  valid: boolean;
}

export interface StatusOption {
  value: string;
  label: string;
  count: number;
}

export interface TransactionPrintStore {
  config: TransactionSourceConfig;
  isLiveAccurate: () => boolean;
  documents: () => readonly TransactionDocumentRecord[];
  filteredDocuments: () => readonly TransactionDocumentRecord[];
  statusOptions: () => StatusOption[];
  statusFilter: () => string;
  setStatusFilter: (status: string) => void;
  searchQuery: () => string;
  setSearchQuery: (q: string) => void;
  resetFilters: () => void;
  hasActiveFilters: () => boolean;
  pagination: () => AccuratePagination;
  loadingDocs: () => boolean;
  docsError: () => string | null;
  refreshDocuments: (page?: number) => Promise<void>;
  focusedDocId: () => number | null;
  focusDocument: (docId: number) => void;
  focusedDocument: () => TransactionDocumentRecord | undefined;
  focusedDocLines: () => TransactionLineDraft[];
  focusedDocLoading: () => boolean;
  focusedDocError: () => string | null;
  retryFocusedDocItems: () => void;
  getLineByKey: (lineKey: string) => TransactionLineDraft | undefined;
  getDocSelectionInfo: (docId: number) => DocumentSelectionInfo;
  includedDocIds: () => Set<number>;
  toggleDocIncluded: (docId: number, include?: boolean) => void;
  toggleFilteredDocsIncluded: (include: boolean) => void;
  clearAllSelection: () => void;
  toggleLineSelected: (lineKey: string, selected?: boolean) => void;
  updateLineName: (lineKey: string, itemName: string) => void;
  updateLineQuantityText: (lineKey: string, quantityText: string) => void;
  stepLineQuantity: (lineKey: string, delta: number) => void;
  resetLineToOriginal: (lineKey: string) => void;
  selectedLines: () => TransactionLineDraft[];
  getLineValidation: (line: TransactionLineDraft) => LineValidationResult;
  validationErrors: () => LineValidationError[];
  totalLabelsCount: () => number;
  canPreview: () => boolean;
  previewItems: () => BarcodeItem[];
  isPreviewOpen: () => boolean;
  setIsPreviewOpen: (open: boolean) => void;
}
