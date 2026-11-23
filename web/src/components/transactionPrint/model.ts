import { createEffect, createMemo, createSignal } from 'solid-js';
import * as api from '~/api';
import { QTY_MODE } from '~/domain';
import {
  activeStatus,
  handleAccurateContextChanged,
  handleSessionInvalidated,
} from '~/stores/accurateAuth';
import { qtyMode } from '~/stores/barcodeConfig';
import type {
  AccurateNormalizedItem,
  AccuratePagination,
  BarcodeItem,
} from '~/types';
import type {
  DocumentSelectionInfo,
  LineValidationError,
  LineValidationResult,
  StatusOption,
  TransactionDocumentRecord,
  TransactionLineDraft,
  TransactionPrintStore,
  TransactionSourceConfig,
} from './types';

export function parsePositiveIntStrict(raw: string): number | null {
  const trimmed = raw.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isSafeInteger(parsed) || parsed < 1) return null;
  return parsed;
}

function normalizeStatusValue(raw: string | undefined): string {
  return (raw ?? '').trim();
}

export function createTransactionPrintStore(
  config: TransactionSourceConfig
): TransactionPrintStore {
  const [searchQuery, setSearchQuery] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal('all');
  const [documents, setDocuments] = createSignal<TransactionDocumentRecord[]>([]);
  const [pagination, setPagination] = createSignal<AccuratePagination>({
    page: 1,
    pageCount: 1,
    pageSize: 20,
    rowCount: 0,
  });
  const [loadingDocs, setLoadingDocs] = createSignal(false);
  const [docsError, setDocsError] = createSignal<string | null>(null);

  const [focusedDocId, setFocusedDocId] = createSignal<number | null>(null);
  const [includedDocIds, setIncludedDocIds] = createSignal<Set<number>>(new Set());
  const [pendingIncludeDocIds, setPendingIncludeDocIds] = createSignal<Set<number>>(
    new Set()
  );

  const [docLineKeysByDocId, setDocLineKeysByDocId] = createSignal<
    Record<number, string[]>
  >({});
  const [linesByKey, setLinesByKey] = createSignal<
    Record<string, TransactionLineDraft>
  >({});
  const [loadingItemsDocIds, setLoadingItemsDocIds] = createSignal<Set<number>>(
    new Set()
  );
  const [docItemsError, setDocItemsError] = createSignal<Record<number, string>>(
    {}
  );
  const [isPreviewOpen, setIsPreviewOpen] = createSignal(false);

  const isLiveAccurate = createMemo(() => {
    const s = activeStatus();
    return Boolean(s.connected && s.accountId && s.database?.id);
  });

  let listToken = 0;
  const itemTokens = new Map<number, number>();
  let observedContext = '';

  const seedSampleDocuments = () => {
    const samples = config.sampleDocuments ?? [];
    const records: TransactionDocumentRecord[] = samples.map((s) => ({
      header: s.header,
      partyName: s.supplierOrParty,
      warehouseName: s.warehouseName,
      isSample: true,
    }));

    const keyMap: Record<number, string[]> = {};
    const draftMap: Record<string, TransactionLineDraft> = {};
    for (const s of samples) {
      const keys: string[] = [];
      s.items.forEach((item, idx) => {
        const linePosition = idx + 1;
        const key = `${s.header.id}:${linePosition}`;
        keys.push(key);
        draftMap[key] = {
          key,
          docId: s.header.id,
          linePosition,
          original: item,
          selected: false,
          itemName: item.itemName,
          quantityText: String(item.quantity > 0 ? item.quantity : 1),
        };
      });
      keyMap[s.header.id] = keys;
    }

    setDocuments(records);
    setPagination({
      page: 1,
      pageCount: 1,
      pageSize: Math.max(records.length, 20),
      rowCount: records.length,
    });
    setDocLineKeysByDocId(keyMap);
    setLinesByKey(draftMap);
    setIncludedDocIds(new Set<number>());
    setPendingIncludeDocIds(new Set<number>());
    setLoadingItemsDocIds(new Set<number>());
    setDocItemsError({});
    setDocsError(null);
    setLoadingDocs(false);
    setFocusedDocId(records[0]?.header.id ?? null);
  };

  const loadDocItems = async (docId: number, autoSelectWhenReady = false) => {
    if (!isLiveAccurate()) return;
    if (docLineKeysByDocId()[docId] !== undefined) return;

    const accountID = activeStatus().accountId;
    const databaseID = activeStatus().database?.id;
    if (!accountID || !databaseID) return;

    if (loadingItemsDocIds().has(docId)) {
      if (autoSelectWhenReady) {
        setPendingIncludeDocIds((prev) => new Set(prev).add(docId));
      }
      return;
    }

    const token = (itemTokens.get(docId) ?? 0) + 1;
    itemTokens.set(docId, token);

    setLoadingItemsDocIds((prev) => new Set(prev).add(docId));
    setDocItemsError((prev) => {
      const next = { ...prev };
      delete next[docId];
      return next;
    });

    try {
      const items: AccurateNormalizedItem[] =
        await api.getAccurateSourceDocumentItems(config.source, docId, accountID);

      if (
        itemTokens.get(docId) !== token ||
        activeStatus().accountId !== accountID ||
        activeStatus().database?.id !== databaseID
      ) {
        return;
      }

      const shouldSelect =
        autoSelectWhenReady ||
        pendingIncludeDocIds().has(docId) ||
        includedDocIds().has(docId);

      const keys: string[] = [];
      const newDrafts: Record<string, TransactionLineDraft> = {};
      items.forEach((item, idx) => {
        const linePosition = idx + 1;
        const key = `${docId}:${linePosition}`;
        keys.push(key);
        newDrafts[key] = {
          key,
          docId,
          linePosition,
          original: item,
          selected: shouldSelect,
          itemName: item.itemName,
          quantityText: String(item.quantity > 0 ? Math.floor(item.quantity) : 1),
        };
      });

      setLinesByKey((prev) => ({ ...prev, ...newDrafts }));
      setDocLineKeysByDocId((prev) => ({ ...prev, [docId]: keys }));
      setPendingIncludeDocIds((prev) => {
        const next = new Set(prev);
        next.delete(docId);
        return next;
      });

      if (shouldSelect && keys.length > 0) {
        setIncludedDocIds((prev) => new Set(prev).add(docId));
      } else if (keys.length === 0) {
        setIncludedDocIds((prev) => {
          const next = new Set(prev);
          next.delete(docId);
          return next;
        });
      }
    } catch (err) {
      if (
        itemTokens.get(docId) !== token ||
        activeStatus().accountId !== accountID ||
        activeStatus().database?.id !== databaseID
      ) {
        return;
      }
      if (err instanceof api.ApiError && err.status === 401) {
        handleSessionInvalidated(err.message);
      } else if (err instanceof api.ApiError && (err.status === 409 || err.status === 403)) {
        await handleAccurateContextChanged(accountID);
      }
      setDocItemsError((prev) => ({
        ...prev,
        [docId]: err instanceof Error ? err.message : 'Failed to load items',
      }));
      setPendingIncludeDocIds((prev) => {
        const next = new Set(prev);
        next.delete(docId);
        return next;
      });
    } finally {
      if (itemTokens.get(docId) === token) {
        setLoadingItemsDocIds((prev) => {
          const next = new Set(prev);
          next.delete(docId);
          return next;
        });
      }
    }
  };

  const refreshDocuments = async (page = 1) => {
    if (!isLiveAccurate()) {
      seedSampleDocuments();
      return;
    }

    const accountID = activeStatus().accountId;
    const databaseID = activeStatus().database?.id;
    if (!accountID || !databaseID) return;

    const token = ++listToken;
    setLoadingDocs(true);
    setDocsError(null);

    try {
      const res = await api.getAccurateSourceDocuments(config.source, {
        q: searchQuery().trim() || undefined,
        page,
        pageSize: 20,
        accountId: accountID,
      });

      if (
        token !== listToken ||
        activeStatus().accountId !== accountID ||
        activeStatus().database?.id !== databaseID
      ) {
        return;
      }

      const records: TransactionDocumentRecord[] = res.documents.map((header) => ({
        header,
        isSample: false,
      }));
      setDocuments(records);
      setPagination(res.pagination);

      const currentFocus = focusedDocId();
      const stillVisible =
        currentFocus !== null && records.some((d) => d.header.id === currentFocus);
      const nextFocus = stillVisible
        ? currentFocus
        : (records[0]?.header.id ?? null);
      setFocusedDocId(nextFocus);
      if (nextFocus !== null) {
        void loadDocItems(nextFocus);
      }
    } catch (err) {
      if (
        token !== listToken ||
        activeStatus().accountId !== accountID ||
        activeStatus().database?.id !== databaseID
      ) {
        return;
      }
      if (err instanceof api.ApiError && err.status === 401) {
        handleSessionInvalidated(err.message);
      } else if (err instanceof api.ApiError && (err.status === 409 || err.status === 403)) {
        await handleAccurateContextChanged(accountID);
      }
      setDocsError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      if (token === listToken) {
        setLoadingDocs(false);
      }
    }
  };

  createEffect(() => {
    const s = activeStatus();
    const ctx = s.connected
      ? `live:${s.accountId ?? ''}:${s.database?.id ?? ''}`
      : 'offline';
    if (ctx === observedContext) return;
    observedContext = ctx;

    listToken++;
    itemTokens.clear();
    setStatusFilter('all');
    setSearchQuery('');
    setIsPreviewOpen(false);

    if (s.connected && s.accountId && s.database?.id) {
      setDocuments([]);
      setDocLineKeysByDocId({});
      setLinesByKey({});
      setIncludedDocIds(new Set<number>());
      setPendingIncludeDocIds(new Set<number>());
      setLoadingItemsDocIds(new Set<number>());
      setDocItemsError({});
      setFocusedDocId(null);
      void refreshDocuments(1);
    } else {
      seedSampleDocuments();
    }
  });

  const statusOptions = createMemo<StatusOption[]>(() => {
    const counts = new Map<string, number>();
    for (const doc of documents()) {
      const status = normalizeStatusValue(doc.header.statusName);
      if (!status) continue;
      counts.set(status, (counts.get(status) ?? 0) + 1);
    }
    return Array.from(counts.entries()).map(([value, count]) => ({
      value,
      label: value,
      count,
    }));
  });

  const filteredDocuments = createMemo(() => {
    const q = searchQuery().trim().toLowerCase();
    const st = statusFilter();
    return documents().filter((doc) => {
      if (st !== 'all' && normalizeStatusValue(doc.header.statusName) !== st) {
        return false;
      }
      if (!q) return true;
      return (
        doc.header.number.toLowerCase().includes(q) ||
        doc.header.description.toLowerCase().includes(q) ||
        (doc.partyName?.toLowerCase().includes(q) ?? false)
      );
    });
  });

  const hasActiveFilters = createMemo(
    () => searchQuery().trim().length > 0 || statusFilter() !== 'all'
  );

  const resetFilters = () => {
    const hadQuery = searchQuery().trim().length > 0;
    setSearchQuery('');
    setStatusFilter('all');
    if (hadQuery && isLiveAccurate()) {
      void refreshDocuments(1);
    }
  };

  const focusedDocument = createMemo(() => {
    const id = focusedDocId();
    const all = documents();
    if (id !== null) {
      const exact = all.find((d) => d.header.id === id);
      if (exact) return exact;
    }
    return all[0];
  });

  const focusDocument = (docId: number) => {
    setFocusedDocId(docId);
    if (isLiveAccurate()) {
      void loadDocItems(docId);
    }
  };

  const getLineByKey = (lineKey: string): TransactionLineDraft | undefined =>
    linesByKey()[lineKey];

  const getDocLines = (docId: number): TransactionLineDraft[] => {
    const keys = docLineKeysByDocId()[docId] ?? [];
    const map = linesByKey();
    const result: TransactionLineDraft[] = [];
    for (const k of keys) {
      const line = map[k];
      if (line) result.push(line);
    }
    return result;
  };

  const focusedDocLines = createMemo(() => {
    const doc = focusedDocument();
    return doc ? getDocLines(doc.header.id) : [];
  });

  const focusedDocLoading = createMemo(() => {
    const doc = focusedDocument();
    return doc ? loadingItemsDocIds().has(doc.header.id) : false;
  });

  const focusedDocError = createMemo(() => {
    const doc = focusedDocument();
    return doc ? (docItemsError()[doc.header.id] ?? null) : null;
  });

  const retryFocusedDocItems = () => {
    const doc = focusedDocument();
    if (doc) void loadDocItems(doc.header.id);
  };

  const getDocSelectionInfo = (docId: number): DocumentSelectionInfo => {
    const keys = docLineKeysByDocId()[docId];
    const itemsLoaded = keys !== undefined;
    const loadingItems = loadingItemsDocIds().has(docId);
    const lines = getDocLines(docId);
    const selectedCount = lines.filter((l) => l.selected).length;
    const totalCount = lines.length;
    return {
      included: includedDocIds().has(docId),
      selectedCount,
      totalCount,
      allSelected: itemsLoaded && totalCount > 0 && selectedCount === totalCount,
      someSelected: selectedCount > 0 && selectedCount < totalCount,
      loadingItems,
      itemsLoaded,
    };
  };

  const toggleDocIncluded = (docId: number, include?: boolean) => {
    const currentlyIncluded =
      includedDocIds().has(docId) || pendingIncludeDocIds().has(docId);
    const nextInclude = include ?? !currentlyIncluded;
    const keys = docLineKeysByDocId()[docId];

    if (nextInclude && keys === undefined && isLiveAccurate()) {
      setPendingIncludeDocIds((prev) => new Set(prev).add(docId));
      setIncludedDocIds((prev) => new Set(prev).add(docId));
      void loadDocItems(docId, true);
      return;
    }

    if (!nextInclude) {
      setPendingIncludeDocIds((prev) => {
        const next = new Set(prev);
        next.delete(docId);
        return next;
      });
    }

    setIncludedDocIds((prev) => {
      const next = new Set(prev);
      if (nextInclude) next.add(docId);
      else next.delete(docId);
      return next;
    });

    setLinesByKey((prev) => {
      const next = { ...prev };
      for (const line of Object.values(prev)) {
        if (line.docId === docId) {
          next[line.key] = { ...line, selected: nextInclude };
        }
      }
      return next;
    });
  };

  const toggleFilteredDocsIncluded = (include: boolean) => {
    for (const doc of filteredDocuments()) {
      toggleDocIncluded(doc.header.id, include);
    }
  };

  const clearAllSelection = () => {
    setIncludedDocIds(new Set<number>());
    setPendingIncludeDocIds(new Set<number>());
    setLinesByKey((prev) => {
      const next: Record<string, TransactionLineDraft> = {};
      for (const [k, line] of Object.entries(prev)) {
        next[k] = { ...line, selected: false };
      }
      return next;
    });
  };

  const toggleLineSelected = (lineKey: string, selected?: boolean) => {
    const map = linesByKey();
    const target = map[lineKey];
    if (!target) return;
    const nextSelected = selected ?? !target.selected;

    const updatedMap = {
      ...map,
      [lineKey]: { ...target, selected: nextSelected },
    };
    setLinesByKey(updatedMap);

    const anySelectedInDoc = Object.values(updatedMap).some(
      (l) => l.docId === target.docId && l.selected
    );
    setIncludedDocIds((prev) => {
      const next = new Set(prev);
      if (anySelectedInDoc) next.add(target.docId);
      else next.delete(target.docId);
      return next;
    });
  };

  const updateLineName = (lineKey: string, itemName: string) => {
    setLinesByKey((prev) => {
      const existing = prev[lineKey];
      if (!existing) return prev;
      return { ...prev, [lineKey]: { ...existing, itemName } };
    });
  };

  const updateLineQuantityText = (lineKey: string, quantityText: string) => {
    setLinesByKey((prev) => {
      const existing = prev[lineKey];
      if (!existing) return prev;
      return { ...prev, [lineKey]: { ...existing, quantityText } };
    });
  };

  const stepLineQuantity = (lineKey: string, delta: number) => {
    const existing = linesByKey()[lineKey];
    if (!existing) return;
    const parsed = parsePositiveIntStrict(existing.quantityText);
    const nextQty = parsed === null ? 1 : Math.max(1, parsed + delta);
    updateLineQuantityText(lineKey, String(nextQty));
  };

  const resetLineToOriginal = (lineKey: string) => {
    setLinesByKey((prev) => {
      const existing = prev[lineKey];
      if (!existing) return prev;
      const origQty =
        existing.original.quantity > 0 ? Math.floor(existing.original.quantity) : 1;
      return {
        ...prev,
        [lineKey]: {
          ...existing,
          itemName: existing.original.itemName,
          quantityText: String(origQty),
        },
      };
    });
  };

  const selectedLines = createMemo(() => {
    const map = linesByKey();
    const keyMap = docLineKeysByDocId();
    const list: TransactionLineDraft[] = [];
    const seenDocs = new Set<number>();

    for (const doc of documents()) {
      seenDocs.add(doc.header.id);
      const keys = keyMap[doc.header.id] ?? [];
      for (const k of keys) {
        const line = map[k];
        if (line?.selected) list.push(line);
      }
    }
    for (const [docIdStr, keys] of Object.entries(keyMap)) {
      const docId = Number(docIdStr);
      if (seenDocs.has(docId)) continue;
      for (const k of keys) {
        const line = map[k];
        if (line?.selected) list.push(line);
      }
    }
    return list;
  });

  const getLineValidation = (line: TransactionLineDraft): LineValidationResult => {
    const nameEmpty = line.itemName.trim().length === 0;
    const parsedQty = parsePositiveIntStrict(line.quantityText);
    const quantityInvalid = parsedQty === null;
    return {
      nameEmpty,
      quantityInvalid,
      parsedQty,
      valid: !nameEmpty && !quantityInvalid,
    };
  };

  const validationErrors = createMemo<LineValidationError[]>(() => {
    const errs: LineValidationError[] = [];
    for (const line of selectedLines()) {
      const v = getLineValidation(line);
      if (!v.valid) {
        errs.push({
          key: line.key,
          docId: line.docId,
          docNumber: line.original.documentNumber,
          itemNo: line.original.itemNo,
          nameEmpty: v.nameEmpty,
          quantityInvalid: v.quantityInvalid,
        });
      }
    }
    return errs;
  });

  const totalLabelsCount = createMemo(() => {
    const selected = selectedLines();
    if (qtyMode() === QTY_MODE.SINGLE) {
      return selected.length;
    }
    let total = 0;
    for (const line of selected) {
      const qty = parsePositiveIntStrict(line.quantityText);
      if (qty !== null) total += qty;
    }
    return total;
  });

  const canPreview = createMemo(
    () => selectedLines().length > 0 && validationErrors().length === 0
  );

  const previewItems = createMemo<BarcodeItem[]>(() => {
    if (!canPreview()) return [];
    return selectedLines().map((line) => {
      const qty = parsePositiveIntStrict(line.quantityText) ?? 1;
      return {
        id: `tx-print-${config.source}-${line.key}`,
        selected: true,
        itemNo: line.original.itemNo,
        itemName: line.itemName.trim(),
        barcode: line.original.upcNo || line.original.itemNo,
        quantity: qty,
        unit: line.original.unitName || 'Pcs',
        price: line.original.unitPrice ?? 0,
        documentType: line.original.documentType,
        documentNumber: line.original.documentNumber,
      };
    });
  });

  return {
    config,
    isLiveAccurate,
    documents,
    filteredDocuments,
    statusOptions,
    statusFilter,
    setStatusFilter,
    searchQuery,
    setSearchQuery,
    resetFilters,
    hasActiveFilters,
    pagination,
    loadingDocs,
    docsError,
    refreshDocuments,
    focusedDocId,
    focusDocument,
    focusedDocument,
    focusedDocLines,
    focusedDocLoading,
    focusedDocError,
    retryFocusedDocItems,
    getLineByKey,
    getDocSelectionInfo,
    includedDocIds,
    toggleDocIncluded,
    toggleFilteredDocsIncluded,
    clearAllSelection,
    toggleLineSelected,
    updateLineName,
    updateLineQuantityText,
    stepLineQuantity,
    resetLineToOriginal,
    selectedLines,
    getLineValidation,
    validationErrors,
    totalLabelsCount,
    canPreview,
    previewItems,
    isPreviewOpen,
    setIsPreviewOpen,
  };
}
