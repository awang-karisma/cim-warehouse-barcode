import { BARCODE_HEIGHT, BARCODE_SYMBOLOGY, FONT_SIZE, QTY_MODE, USER_ROLE } from '~/domain';

import { useI18n } from '~/i18n';
import {
  type Component,
  createEffect,
  createMemo,
  createSignal,
  Index,
  For,
  onCleanup,
  onMount,
  Show,
} from 'solid-js';
import * as api from '~/api';
import { createPageDesign, deletePageDesign, updatePageDesign } from '~/api';
import { useAuth } from '~/auth';
import { AccurateLoginModal } from '~/components/barcode/AccurateLoginModal';
import { ItemSearchDrawer } from '~/components/barcode/ItemSearchDrawer';
import { LabelLayoutDesigner } from '~/components/barcode/LabelLayoutDesigner';
import { PrintPreview } from '~/components/barcode/PrintPreview';
import { TransactionDrawer } from '~/components/barcode/TransactionDrawer';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import {
  refreshStatus,
  setIsLoginModalOpen,
  status
} from '~/stores/accurateAuth';
import {
  barcodeHeight,
  customConfig,
  applyPageDesign,
  captureBarcodePresetSettings,
  fontSize,
  fromPresetConfig,
  getEffectiveConfig,
  presetKey,
	SERVER_PRESET_PREFIX,
  PRESETS,
  qtyMode,
  refetchDesigns,
  selectPreset,
  serverDesigns,
  setBarcodeHeight,
  setCustomConfig,
  setPresetDraftActive,
  setFontSize,
  setPresetKey,
  setQtyMode,
  setShowScreenBorder,
  setSkipCount,
  setSymbology,
  setZoom,
  showScreenBorder,
  skipCount,
  symbology,
  toPresetConfig,
  zoom,
} from '~/stores/barcodeConfig';
import { labelLayout, serializeLayout } from '~/stores/labelLayout';
import {
  addItem,
  clearItems,
  deleteItem,
  items,
  toggleSelectAll,
  updateItemField,
} from '~/stores/barcodeItems';
import type { AccurateInventoryItem, BarcodeHeight, FontSize, PageDesign, Symbology } from '~/types';

import {
  IconCopy,
  IconDeviceFloppy,
  IconEdit,
  IconEye,
  IconEyeOff,
  IconFileInvoice,
  IconLoader2,
  IconPackage,
  IconPlugConnected,
  IconPlugConnectedX,
  IconPlus,
  IconPrinter,
  IconScan,
  IconTrash,
  IconX,
} from '@tabler/icons-solidjs';
import { DataTable } from '~/components/DataTable';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table';

export const Barcode: Component = () => {
  const { t } = useI18n();
  const [isInspectorOpen, setIsInspectorOpen] = createSignal(true);
  const [isSearchOpen, setIsSearchOpen] = createSignal(false);
  const [isTransactionOpen, setIsTransactionOpen] = createSignal(false);

  let searchInputRef: HTMLInputElement | undefined;
  const auth = useAuth();
  const isAdmin = () => {
    const role = auth.user()?.role;
	return role === USER_ROLE.ADMIN || role === USER_ROLE.SUPERADMIN;
  };

  const [isEditing, setIsEditing] = createSignal(false);
  const [isCreating, setIsCreating] = createSignal(false);
  let previousPresetKey = 'tj-108';

  const startCreating = () => {
    previousPresetKey = presetKey();
    setPresetDraftActive(true);
    setIsEditing(false);
    setIsCreating(true);
    setPresetKey('custom');
    setCustomConfig({ ...getEffectiveConfig(), name: '' });
    setStatusMessage(null);
  };

  const cancelCreating = () => {
    setIsCreating(false);
    setPresetDraftActive(false);
		const fallback = previousPresetKey.startsWith(SERVER_PRESET_PREFIX) || PRESETS[previousPresetKey as keyof typeof PRESETS] ? previousPresetKey : 'tj-108';
    selectPreset(fallback);
    setStatusMessage(null);
  };

  const startEditing = () => {
    setIsCreating(false);
    setIsEditing(true);
    setPresetDraftActive(true);
    const pk = presetKey();
	if (pk.startsWith(SERVER_PRESET_PREFIX)) {
		const id = parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
      const design = (serverDesigns() ?? []).find((d) => d.id === id);
      if (design) applyPageDesign(design);
    }
    setStatusMessage(null);
  };

  const cancelEditing = () => {
    setIsEditing(false);
    setPresetDraftActive(false);
    selectPreset(presetKey());
    setStatusMessage(null);
  };

  const [isSaving, setIsSaving] = createSignal(false);
  const [statusMessage, setStatusMessage] = createSignal<{ type: 'success' | 'error'; text: string; retry?: boolean } | null>(null);
  const [designToDelete, setDesignToDelete] = createSignal<PageDesign | null>(null);
  const [isDeletingDesign, setIsDeletingDesign] = createSignal(false);

  const retrySavedPresetRefresh = async () => {
	const id = Number.parseInt(presetKey().slice(SERVER_PRESET_PREFIX.length), 10);
    try {
      const designs = await refetchDesigns();
      const design = (designs ?? []).find((entry) => entry.id === id);
      if (!design) throw new Error('Saved preset was not returned by the refreshed list');
      applyPageDesign(design);
      setPresetKey('custom');
		setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
      setIsCreating(false);
      setIsEditing(false);
      setPresetDraftActive(false);
      setStatusMessage({ type: 'success', text: t('barcode.changesSaved') });
    } catch {
      setStatusMessage({ type: 'error', text: t('barcode.presetRefreshFailed'), retry: true });
    }
  };

  const refreshAfterPresetSave = async (id: number, successText: string) => {
	setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
    setPresetDraftActive(true);
    try {
      const designs = await refetchDesigns();
      const design = (designs ?? []).find((entry) => entry.id === id);
      if (!design) throw new Error('Saved preset was not returned by the refreshed list');
      applyPageDesign(design);
      setPresetKey('custom');
		setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
      setIsCreating(false);
      setIsEditing(false);
      setPresetDraftActive(false);
      setStatusMessage({ type: 'success', text: successText });
    } catch {
      setStatusMessage({ type: 'error', text: t('barcode.presetRefreshFailed'), retry: true });
    }
  };

  const handleSaveNewDesign = async () => {
    const cfg = customConfig();
    const name = cfg.name.trim();
    if (!name || name === PRESETS.custom.name) {
      setStatusMessage({ type: 'error', text: t('barcode.enterPresetName') });
      return;
    }
    setIsSaving(true);
    setStatusMessage(null);
    try {
      const saved = await createPageDesign(fromPresetConfig(cfg, serializeLayout(labelLayout()), captureBarcodePresetSettings()));
      await refreshAfterPresetSave(saved.id, t('barcode.presetSaved', { name }));
    } catch (err: unknown) {
      setStatusMessage({ type: 'error', text: err instanceof Error ? err.message : t('barcode.savePresetFailed') });
    } finally {
      setIsSaving(false);
    }
  };

  const handleUpdateDesign = async () => {
    const pk = presetKey();
	if (!pk.startsWith(SERVER_PRESET_PREFIX)) return;
	const id = Number.parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
    const cfg = customConfig();
    const name = cfg.name.trim();
    if (!name) {
      setStatusMessage({ type: 'error', text: t('barcode.presetNameRequired') });
      return;
    }
    setIsSaving(true);
    setStatusMessage(null);
    try {
      await updatePageDesign(id, fromPresetConfig(cfg, serializeLayout(labelLayout()), captureBarcodePresetSettings()));
      await refreshAfterPresetSave(id, t('barcode.changesSaved'));
    } catch (err: unknown) {
      setStatusMessage({ type: 'error', text: err instanceof Error ? err.message : t('barcode.updatePresetFailed') });
    } finally {
      setIsSaving(false);
    }
  };

  const handleSaveAsCopy = async () => {
    const cfg = customConfig();
    const baseName = cfg.name.trim() || t('barcode.customLabel');
    const name = baseName.includes('Copy') || baseName.includes('Salinan') ? `${baseName} 2` : `${baseName}${t('barcode.copySuffix')}`;
    setIsSaving(true);
    setStatusMessage(null);
    try {
      const copyCfg = { ...cfg, name };
      const saved = await createPageDesign(fromPresetConfig(copyCfg, serializeLayout(labelLayout()), captureBarcodePresetSettings()));
      await refreshAfterPresetSave(saved.id, t('barcode.copyCreated', { name }));
    } catch (err: unknown) {
      setStatusMessage({ type: 'error', text: err instanceof Error ? err.message : t('barcode.createCopyFailed') });
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteCurrentDesign = () => {
    const pk = presetKey();
	if (!pk.startsWith(SERVER_PRESET_PREFIX)) return;
	const id = parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
    const design = (serverDesigns() ?? []).find((d) => d.id === id);
    if (design) {
      setDesignToDelete(design);
    }
  };

  const handleConfirmDelete = async () => {
    const target = designToDelete();
    if (!target) return;
    setIsDeletingDesign(true);
    try {
      await deletePageDesign(target.id);
      await refetchDesigns();
      setIsEditing(false);
      setIsCreating(false);
		if (presetKey() === `${SERVER_PRESET_PREFIX}${target.id}`) {
        selectPreset('tj-108');
      }
      setDesignToDelete(null);
      setStatusMessage({ type: 'success', text: `Deleted "${target.name}"` });
    } catch (err: unknown) {
      setStatusMessage({ type: 'error', text: err instanceof Error ? err.message : 'Failed to delete' });
    } finally {
      setIsDeletingDesign(false);
    }
  };
  const selectedPaper = () => {
    const p = customConfig().paper;
    if (p === 'A4' || p === 'A5' || p === 'Letter' || p === 'Roll') {
      return p;
    }
    return 'Custom';
  };

  const isCustomPaper = () => selectedPaper() === 'Custom' || selectedPaper() === 'Roll';

  const handlePaperChange = (newPaper: string) => {
    const current = customConfig();
    if (newPaper === 'A4') {
      setCustomConfig({
        ...current,
        paper: 'A4',
        paperWidthMM: 210,
        paperHeightMM: 297,
      });
    } else if (newPaper === 'A5') {
      setCustomConfig({
        ...current,
        paper: 'A5',
        paperWidthMM: 148,
        paperHeightMM: 210,
      });
    } else if (newPaper === 'Letter') {
      setCustomConfig({
        ...current,
        paper: 'Letter',
        paperWidthMM: 216,
        paperHeightMM: 279,
      });
    } else if (newPaper === 'Roll') {
      setCustomConfig({
        ...current,
        paper: 'Roll',
        paperWidthMM: current.labelWidthMM + current.marginLeftMM * 2,
        paperHeightMM: current.labelHeightMM + current.marginTopMM * 2,
        cols: 1,
        rows: 1,
      });
    } else {
      setCustomConfig({
        ...current,
        paper: 'Custom',
      });
    }
  };

  let searchContainerRef: HTMLDivElement | undefined;
  let debounceTimer: number | undefined;
  let searchRequestId = 0;

  const [searchQuery, setSearchQuery] = createSignal('');
  const [searchResults, setSearchResults] = createSignal<AccurateInventoryItem[]>([]);
  const [searchMessage, setSearchMessage] = createSignal<string | null>(null);
  const [isSearching, setIsSearching] = createSignal(false);
  const [isPopupOpen, setIsPopupOpen] = createSignal(false);
  const [highlightedIndex, setHighlightedIndex] = createSignal(-1);

  onMount(() => {
    void refreshStatus();
  });

  onCleanup(() => {
    clearTimeout(debounceTimer);
  });

  createEffect(() => {
    if (!isPopupOpen()) return;

    const handleOutsideClick = (e: MouseEvent) => {
      if (searchContainerRef && !searchContainerRef.contains(e.target as Node)) {
        setIsPopupOpen(false);
        setHighlightedIndex(-1);
      }
    };

    document.addEventListener('mousedown', handleOutsideClick);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleOutsideClick);
    });
  });

  const selectedItems = createMemo(() => items().filter((it) => it.selected));
  const totalLabels = createMemo(() => {
	if (qtyMode() === QTY_MODE.MULTIPLY) {
      return selectedItems().reduce((acc, it) => acc + (it.quantity > 0 ? it.quantity : 1), 0);
    }
    return selectedItems().length;
  });

  const isAllSelected = createMemo(
    () => items().length > 0 && selectedItems().length === items().length
  );

  const addInventoryItemToList = (it: AccurateInventoryItem) => {
    addItem({
      itemNo: it.no,
      itemName: it.name,
      barcode: it.upcNo || it.no,
      quantity: 1,
      unit: it.unit1Name || 'Pcs',
      price: it.unitPrice ?? 0,
      documentType: 'Master Item',
      documentNumber: it.no,
    });
    setSearchQuery('');
    setSearchResults([]);
    setIsPopupOpen(false);
    setSearchMessage(null);
    setHighlightedIndex(-1);
    searchInputRef?.focus();
  };

  const executeSearch = async (q: string, fromEnter: boolean) => {
    const reqId = ++searchRequestId;
    setIsSearching(true);
    setSearchMessage(null);

    try {
      const res = await api.searchAccurateItems({ q, page: 1, pageSize: 50 });
      if (reqId !== searchRequestId) return;

      const all = res.items || [];
      const printable = all.filter((it) => it.canPrintBarcode);

      if (all.length === 0) {
        setSearchResults([]);
        setIsPopupOpen(false);
        setHighlightedIndex(-1);
        if (fromEnter) {
          setSearchMessage('Item not found in Accurate.');
        }
      } else if (printable.length === 0) {
        setSearchResults([]);
        setIsPopupOpen(false);
        setHighlightedIndex(-1);
        if (fromEnter) {
          setSearchMessage('No barcode-printable inventory items found.');
        }
      } else if (printable.length === 1 && fromEnter) {
        addInventoryItemToList(printable[0]);
      } else {
        setSearchResults(printable);
        setIsPopupOpen(true);
        setHighlightedIndex(0);
      }
    } catch {
      if (reqId !== searchRequestId) return;
      setSearchResults([]);
      setIsPopupOpen(false);
      setHighlightedIndex(-1);
      if (fromEnter) {
        setSearchMessage('Error searching Accurate items.');
      }
    } finally {
      if (reqId === searchRequestId) {
        setIsSearching(false);
      }
    }
  };

  const handleSearchInput = (e: InputEvent) => {
    const val = (e.currentTarget as HTMLInputElement).value;
    setSearchQuery(val);
    clearTimeout(debounceTimer);

    if (val.trim().length >= 2 && status().connected) {
      debounceTimer = window.setTimeout(() => {
        void executeSearch(val.trim(), false);
      }, 250);
    } else {
      setSearchResults([]);
      setIsPopupOpen(false);
      setSearchMessage(null);
      setHighlightedIndex(-1);
    }
  };

  const handleSearchKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      if (!isPopupOpen() || searchResults().length === 0) return;
      e.preventDefault();
      setHighlightedIndex((prev) => (prev + 1) % searchResults().length);
      return;
    }

    if (e.key === 'ArrowUp') {
      if (!isPopupOpen() || searchResults().length === 0) return;
      e.preventDefault();
      setHighlightedIndex((prev) => (prev - 1 + searchResults().length) % searchResults().length);
      return;
    }

    if (e.key === 'Escape') {
      if (isPopupOpen()) {
        e.preventDefault();
        setIsPopupOpen(false);
        setHighlightedIndex(-1);
      }
      return;
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      const val = searchQuery().trim();
      if (!val) return;

      const idx = highlightedIndex();
      if (isPopupOpen() && idx >= 0 && idx < searchResults().length) {
        addInventoryItemToList(searchResults()[idx]);
      } else if (status().connected) {
        void executeSearch(val, true);
      } else {
        addItem({
          itemNo: val,
          itemName: `Item ${val}`,
          barcode: val,
          quantity: 1,
        });
        setSearchQuery('');
        searchInputRef?.focus();
      }
    }
  };

  return (
    <div class="space-y-6 barcode-print-root">
      {/* Card 1: Quick Action Toolbar & Search */}
      <Card class="no-print">
        <CardContent class="p-4 md:p-6">
          <div class="flex flex-col sm:flex-row sm:items-end justify-between gap-4">
            {/* Barcode Quick Scan / Live Lookup */}
            <div ref={searchContainerRef} class="relative w-full sm:flex-1 sm:max-w-md">
              <div class="text-xs font-semibold text-muted-foreground flex items-center gap-1 mb-1.5">
                <IconScan class="size-3 text-primary" />
                <span>{t('barcode.quickScan')}</span>
              </div>
              <div class="relative">
                <input
                  ref={searchInputRef}
                  type="text"
                  value={searchQuery()}
                  onInput={handleSearchInput}
                  onKeyDown={handleSearchKeyDown}
                  placeholder={
                    status().connected
                      ? t('barcode.scanPlaceholder')
                      : t('barcode.scanPlaceholderOffline')
                  }
                  class="h-9 w-full px-3 pr-8 font-mono text-xs border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                />
                <Show when={isSearching()}>
                  <IconLoader2 class="absolute right-2.5 top-2.5 size-4 animate-spin text-muted-foreground" />
                </Show>
              </div>

              <Show when={searchMessage()}>
                <div class="absolute left-0 right-0 top-full mt-1 p-2 bg-amber-50 dark:bg-amber-950/50 text-amber-800 dark:text-amber-200 text-[11px] rounded border border-amber-200 dark:border-amber-800 z-30 shadow-md">
                  {searchMessage()}
                </div>
              </Show>

              {/* Autocomplete Popup */}
              <Show when={isPopupOpen() && searchResults().length > 0}>
                <div class="absolute left-0 right-0 top-full mt-1 max-h-60 overflow-y-auto bg-popover border border-border rounded-lg shadow-xl z-50 divide-y divide-border text-xs">
                  <For each={searchResults()}>
                    {(it, idx) => (
                      <button
                        type="button"
                        onClick={() => addInventoryItemToList(it)}
                        class={`w-full text-left p-2.5 hover:bg-accent hover:text-accent-foreground flex items-center justify-between transition-colors ${
                          highlightedIndex() === idx() ? 'bg-accent text-accent-foreground' : ''
                        }`}
                      >
                        <div>
                          <div class="font-medium text-foreground">{it.name}</div>
                          <div class="text-[10px] text-muted-foreground font-mono">
                            {it.no} • {it.upcNo || t('barcode.noBarcode')}
                          </div>
                        </div>
                        <div class="text-right text-[11px] font-semibold text-primary">
                          {it.availableToSell} {it.unit1Name}
                        </div>
                      </button>
                    )}
                  </For>
                </div>
              </Show>
            </div>

            {/* Import Buttons Group + Accurate status badge */}
            <div class="flex items-center gap-2 flex-wrap">
              <Show when={status().connected}>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setIsSearchOpen(true)}
                  class="h-9 gap-1 text-xs"
                >
                  <IconPackage class="size-3.5" />
                  <span>{t('barcode.accurateItems')}</span>
                </Button>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setIsTransactionOpen(true)}
                  class="h-9 gap-1 text-xs"
                >
                  <IconFileInvoice class="size-3.5" />
                  <span>{t('barcode.loadPoDo')}</span>
                </Button>
              </Show>

              {/* Accurate Connection Status */}
              <Show
                when={status().enabled}
                fallback={
                  <Badge variant="outline" class="text-xs text-muted-foreground border-dashed h-9 px-3">
                    {t('barcode.offlineMode')}
                  </Badge>
                }
              >
                <Show
                  when={status().connected}
                  fallback={
                    <button
                      type="button"
                      onClick={() => setIsLoginModalOpen(true)}
                      class="inline-flex items-center gap-1.5 px-3 h-9 rounded-md text-xs font-medium bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 hover:bg-amber-500/20 transition-colors"
                    >
                      <IconPlugConnectedX class="size-3.5" />
                      <span>{t('barcode.accurateDisconnected')}</span>
                    </button>
                  }
                >
                  <button
                    type="button"
                    onClick={() => setIsLoginModalOpen(true)}
                    class="inline-flex items-center gap-1.5 px-3 h-9 rounded-md text-xs font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/20 transition-colors"
                  >
                    <IconPlugConnected class="size-3.5" />
                    <span class="truncate max-w-[140px] font-semibold">
                      {status().database?.alias || t('barcode.connected')}
                    </span>
                  </button>
                </Show>
              </Show>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Card 2: Queued Items Table */}
      <Card class="no-print">
        <CardHeader class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 space-y-0 pb-4">
          <div>
            <CardTitle class="text-base font-semibold">{t('barcode.printQueue')}</CardTitle>
            <CardDescription>
              {t('barcode.queueSummary', { total: items().length.toLocaleString(), selected: selectedItems().length, labels: totalLabels() })}
            </CardDescription>
          </div>

          <div class="flex items-center gap-2">
            <Button
              size="sm"
              onClick={() => addItem()}
              class="text-xs h-8 gap-1"
            >
              <IconPlus class="size-3.5" />
              <span>{t('barcode.addRow')}</span>
            </Button>

            <Button
              variant="outline"
              size="sm"
              onClick={() => toggleSelectAll(!isAllSelected())}
              class="text-xs h-8"
            >
              {isAllSelected() ? t('barcode.deselectAll') : t('barcode.selectAll')}
            </Button>

            <Show when={items().length > 0}>
              <Button
                variant="ghost"
                size="sm"
                onClick={clearItems}
                class="text-xs h-8 text-destructive hover:bg-destructive/10 gap-1"
              >
                <IconTrash class="size-3.5" />
                <span>{t('barcode.clearAll')}</span>
              </Button>
            </Show>
          </div>
        </CardHeader>

        <CardContent class="p-4 md:p-6 pt-0 space-y-4">
          <DataTable>
            <TableHeader>
              <TableRow>
                <TableHead class="w-10 text-center">
                  <input
                    type="checkbox"
                    checked={isAllSelected()}
                    onChange={(e) => toggleSelectAll(e.currentTarget.checked)}
                    class="rounded border-border text-primary"
                  />
                </TableHead>
                <TableHead class="w-32">{t('barcode.colItemCode')}</TableHead>
                <TableHead class="min-w-[180px]">{t('barcode.colItemName')}</TableHead>
                <TableHead class="w-36">{t('barcode.colBarcode')}</TableHead>
                <TableHead class="w-28 text-center">{t('barcode.colQuantity')}</TableHead>
                <TableHead class="w-20 text-center">{t('barcode.colUnit')}</TableHead>
                <TableHead class="w-28 text-right">{t('barcode.colPrice')}</TableHead>
                <TableHead class="w-12 text-center"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <Index each={items()}>
                {(item) => (
                  <TableRow class={item().selected ? '' : 'opacity-40'}>
                    <TableCell class="text-center">
                      <input
                        type="checkbox"
                        checked={item().selected}
                        onChange={(e) =>
                          updateItemField(item().id, 'selected', e.currentTarget.checked)
                        }
                        class="rounded border-border text-primary cursor-pointer"
                      />
                    </TableCell>
                    <TableCell class="p-2">
                      <input
                        type="text"
                        value={item().itemNo}
                        onInput={(e) =>
                          updateItemField(item().id, 'itemNo', e.currentTarget.value)
                        }
                        class="w-full px-2 py-1 text-xs font-mono font-medium border rounded bg-background"
                      />
                    </TableCell>
                    <TableCell class="p-2">
                      <input
                        type="text"
                        value={item().itemName}
                        onInput={(e) =>
                          updateItemField(item().id, 'itemName', e.currentTarget.value)
                        }
                        class="w-full px-2 py-1 text-xs border rounded bg-background"
                      />
                    </TableCell>
                    <TableCell class="p-2">
                      <input
                        type="text"
                        value={item().barcode}
                        onInput={(e) =>
                          updateItemField(item().id, 'barcode', e.currentTarget.value)
                        }
                        class="w-full px-2 py-1 text-xs font-mono border rounded bg-background"
                      />
                    </TableCell>
                    <TableCell class="p-2 text-center">
                      <div class="inline-flex items-center border rounded bg-background">
                        <button
                          type="button"
                          onClick={() =>
                            updateItemField(item().id, 'quantity', Math.max(1, item().quantity - 1))
                          }
                          class="px-2 py-1 text-muted-foreground hover:text-foreground text-xs font-bold"
                        >
                          -
                        </button>
                        <input
                          type="number"
                          min={1}
                          value={item().quantity}
                          onInput={(e) =>
                            updateItemField(
                              item().id,
                              'quantity',
                              parseInt(e.currentTarget.value, 10) || 1
                            )
                          }
                          class="w-12 text-center font-semibold text-xs bg-transparent focus:outline-none"
                        />
                        <button
                          type="button"
                          onClick={() =>
                            updateItemField(item().id, 'quantity', item().quantity + 1)
                          }
                          class="px-2 py-1 text-muted-foreground hover:text-foreground text-xs font-bold"
                        >
                          +
                        </button>
                      </div>
                    </TableCell>
                    <TableCell class="p-2 text-center">
                      <input
                        type="text"
                        value={item().unit}
                        onInput={(e) =>
                          updateItemField(item().id, 'unit', e.currentTarget.value)
                        }
                        class="w-16 px-1.5 py-1 text-xs text-center border rounded bg-background"
                      />
                    </TableCell>
                    <TableCell class="p-2 text-right">
                      <input
                        type="number"
                        step={500}
                        value={item().price}
                        onInput={(e) =>
                          updateItemField(item().id, 'price', parseFloat(e.currentTarget.value) || 0)
                        }
                        class="w-24 px-1.5 py-1 text-xs text-right border rounded bg-background"
                      />
                    </TableCell>
                    <TableCell class="text-center p-2">
                      <button
                        type="button"
                        onClick={() => deleteItem(item().id)}
                        class="p-1 text-muted-foreground hover:text-destructive rounded transition-colors"
                        title={t('barcode.deleteRow')}
                      >
                        <IconTrash class="size-3.5" />
                      </button>
                    </TableCell>
                  </TableRow>
                )}
              </Index>

              <Show when={items().length === 0}>
                <TableRow>
                  <TableCell colspan={8} class="h-32 text-center text-muted-foreground text-xs">
                    {t('barcode.emptyQueue')}
                  </TableCell>
                </TableRow>
              </Show>
            </TableBody>
          </DataTable>
        </CardContent>
      </Card>

      {/* Card 3: Live Print Preview Canvas */}
      <Card class="overflow-hidden barcode-print-card">
        <CardHeader class="no-print flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 space-y-0 pb-4 border-b">
          <div>
            <CardTitle class="text-base font-semibold flex items-center gap-2">
              <IconPrinter class="size-5 text-primary" />
              {t('barcode.printPreview')}
            </CardTitle>
            <CardDescription>
              {t('barcode.renderingSummary', { count: selectedItems().length, labels: totalLabels(), preset: getEffectiveConfig().name })}
            </CardDescription>
          </div>

          <div class="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsInspectorOpen(!isInspectorOpen())}
              class="h-8 text-xs gap-1.5"
            >
              <Show when={isInspectorOpen()} fallback={<IconEye class="size-4" />}>
                <IconEyeOff class="size-4" />
              </Show>
              <span>{isInspectorOpen() ? t('barcode.hideSettings') : t('barcode.showSettings')}</span>
            </Button>
          </div>
        </CardHeader>

        {/* Collapsible Inspector Panel (integrated into Card 3) */}
        <Show when={isInspectorOpen()}>
          <div class="no-print border-b bg-card px-4 py-4 md:px-6">
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5 gap-4 text-xs items-start">
              {/* Section 1: Paper & Preset */}
              <div class="space-y-1.5">
                <label class="block font-semibold text-muted-foreground text-[11px] uppercase">
                  {t('barcode.stickerPaperPreset')}
                </label>
                <p class="text-[10px] leading-snug text-muted-foreground">{t('barcode.presetContentsHelp')}</p>
                <select
                  value={presetKey()}
                  onChange={(e) => {
                    const nextVal = e.currentTarget.value;
                    if (!nextVal || nextVal === 'custom') return;
                    setIsEditing(false);
                    setIsCreating(false);
                    setPresetDraftActive(false);
                    setStatusMessage(null);
                    selectPreset(nextVal);
                  }}
                  class="w-full h-8 text-xs px-2.5 border rounded-md bg-background font-medium"
                >
                  <option value="custom" disabled class="hidden">
                    {t('barcode.newPresetUnsaved')}
                  </option>
                  <optgroup label={t('barcode.builtinPresets')}>
                    <For each={Object.entries(PRESETS).filter(([k]) => k !== 'custom')}>
                      {([k, p]) => <option value={k}>{p.name}</option>}
                    </For>
                  </optgroup>
                  <Show when={(serverDesigns() ?? []).length > 0}>
                    <optgroup label={t('barcode.customPresets')}>
                      <For each={serverDesigns() ?? []}>
						{(d) => <option value={`${SERVER_PRESET_PREFIX}${d.id}`}>{d.name}</option>}
                      </For>
                    </optgroup>
                  </Show>
                </select>
                <Show when={serverDesigns.error}>
                  <div class="flex items-center justify-between gap-2 rounded border border-destructive/30 bg-destructive/10 px-2 py-1 text-[10px] text-destructive">
                    <span>{t('barcode.presetRefreshFailed')}</span>
                    <Button type="button" variant="outline" size="sm" class="h-6 px-2 text-[10px]" onClick={() => refetchDesigns()}>{t('common.retry')}</Button>
                  </div>
                </Show>

                {/* Action buttons under dropdown */}
                <Show when={isAdmin()}>
                  <div class="pt-0.5">
                    <label class="block text-[10px] text-muted-foreground select-none" aria-hidden="true">
                      &nbsp;
                    </label>
                    <div class="grid grid-cols-3 gap-1.5">
                      <Button
                        type="button"
                        variant={isEditing() ? 'secondary' : 'outline'}
                        size="sm"
                        class="w-full h-7 text-xs gap-1 px-1"
						disabled={!presetKey().startsWith(SERVER_PRESET_PREFIX) || isCreating()}
                        onClick={() => {
                          if (isEditing()) cancelEditing();
                          else startEditing();
                        }}
                        title={
						!presetKey().startsWith(SERVER_PRESET_PREFIX)
                            ? t('barcode.builtinCannotEdit')
                            : isEditing()
                            ? t('barcode.closePresetEditor')
                            : t('barcode.editCustomPreset')
                        }
                      >
                        <Show when={isEditing()} fallback={<IconEdit class="size-3.5" />}>
                          <IconX class="size-3.5" />
                        </Show>
                        <span>{isEditing() ? t('common.cancel') : t('barcode.editPreset')}</span>
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        class="w-full h-7 text-xs text-destructive hover:text-destructive hover:bg-destructive/10 disabled:text-muted-foreground disabled:hover:bg-transparent disabled:opacity-40 gap-1 px-1"
						disabled={!presetKey().startsWith(SERVER_PRESET_PREFIX) || isCreating()}
                        onClick={handleDeleteCurrentDesign}
                        title={
						!presetKey().startsWith(SERVER_PRESET_PREFIX)
                            ? t('barcode.builtinCannotDelete')
                            : t('barcode.deleteCustomPreset')
                        }
                      >
                        <IconTrash class="size-3.5" />
                        <span>{t('common.delete')}</span>
                      </Button>
                      <Button
                        type="button"
                        variant={isCreating() ? 'secondary' : 'outline'}
                        size="sm"
                        class="w-full h-7 text-xs gap-1 px-1"
                        onClick={() => {
                          if (isCreating()) cancelCreating();
                          else startCreating();
                        }}
                        title={isCreating() ? t('barcode.cancelNewPreset') : t('barcode.createNewPreset')}
                      >
                        <Show when={isCreating()} fallback={<IconPlus class="size-3.5" />}>
                          <IconX class="size-3.5" />
                        </Show>
                        <span>{isCreating() ? t('common.cancel') : t('barcode.newPreset')}</span>
                      </Button>
                    </div>
                  </div>
                </Show>
                <Show when={isCreating() || isEditing()}>
                  <div class="space-y-2 pt-2 border-t border-border/50">
                    {/* Header with Title */}
                    <div class="flex items-center justify-between gap-1">
                      <span class="font-semibold text-muted-foreground uppercase tracking-wider text-[10px]">
                        {t('barcode.customDimensions')}
                      </span>
                    </div>


                    {/* Design Name */}
                    <Show when={isAdmin() || presetKey() === 'custom'}>
                      <div>
                        <label class="block text-muted-foreground text-[10px]">{t('barcode.designName')}</label>
                        <input
                          type="text"
                          value={customConfig().name}
                          onInput={(e) => setCustomConfig({ ...customConfig(), name: e.currentTarget.value })}
                          placeholder={t('barcode.designNamePlaceholder')}
                          class="w-full px-2 py-1 border rounded bg-background text-xs"
                        />
                      </div>
                    </Show>

                    {/* Label Dimensions */}
                    <div>
                      <div class="text-muted-foreground text-[10px] font-medium mb-1">{t('barcode.labelSize')}</div>
                      <div class="grid grid-cols-2 gap-1.5">
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.width')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="1"
                            value={customConfig().labelWidthMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                labelWidthMM: parseFloat(e.currentTarget.value) || 1,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.height')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="1"
                            value={customConfig().labelHeightMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                labelHeightMM: parseFloat(e.currentTarget.value) || 1,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Grid: Columns & Rows */}
                    <div>
                      <div class="text-muted-foreground text-[10px] font-medium mb-1">{t('barcode.gridLayout')}</div>
                      <div class="grid grid-cols-2 gap-1.5">
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.columns')}</label>
                          <input
                            type="number"
                            min="1"
                            max="20"
                            value={customConfig().cols}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                cols: parseInt(e.currentTarget.value, 10) || 1,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.rows')}</label>
                          <input
                            type="number"
                            min="1"
                            max="50"
                            value={customConfig().rows}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                rows: parseInt(e.currentTarget.value, 10) || 1,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Gaps: Horizontal & Vertical */}
                    <div>
                      <div class="text-muted-foreground text-[10px] font-medium mb-1">{t('barcode.gaps')}</div>
                      <div class="grid grid-cols-2 gap-1.5">
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.gapX')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="0"
                            value={customConfig().gapXMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                gapXMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.gapY')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="0"
                            value={customConfig().gapYMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                gapYMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Margins: Top & Left */}
                    <div>
                      <div class="text-muted-foreground text-[10px] font-medium mb-1">{t('barcode.margins')}</div>
                      <div class="grid grid-cols-2 gap-1.5">
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.marginTop')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="0"
                            value={customConfig().marginTopMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                marginTopMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.marginLeft')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="0"
                            value={customConfig().marginLeftMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                marginLeftMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-2 py-1 border rounded bg-background text-xs"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Paper Sheet */}
                    <div>
                      <div class="text-muted-foreground text-[10px] font-medium mb-1">{t('barcode.paperSheet')}</div>
                      <div class="grid grid-cols-3 gap-1">
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.paper')}</label>
                          <select
                            value={selectedPaper()}
                            onChange={(e) => handlePaperChange(e.currentTarget.value)}
                            class="w-full px-1.5 py-1 border rounded bg-background text-xs"
                          >
                            <option value="A4">A4</option>
                            <option value="A5">A5</option>
                            <option value="Letter">Letter</option>
                            <option value="Custom">Custom</option>
                            <option value="Roll">{t('barcode.paperRoll')}</option>
                          </select>
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.width')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="1"
                            disabled={!isCustomPaper()}
                            value={customConfig().paperWidthMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                paperWidthMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-1.5 py-1 border rounded bg-background text-xs disabled:opacity-50 disabled:bg-muted/50 disabled:cursor-not-allowed"
                          />
                        </div>
                        <div>
                          <label class="block text-muted-foreground text-[9px]">{t('barcode.height')}</label>
                          <input
                            type="number"
                            step="0.5"
                            min="1"
                            disabled={!isCustomPaper()}
                            value={customConfig().paperHeightMM}
                            onInput={(e) =>
                              setCustomConfig({
                                ...customConfig(),
                                paperHeightMM: parseFloat(e.currentTarget.value) || 0,
                              })
                            }
                            class="w-full px-1.5 py-1 border rounded bg-background text-xs disabled:opacity-50 disabled:bg-muted/50 disabled:cursor-not-allowed"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Audit Sheet Toggle */}
                    <label class="flex items-center gap-1.5 cursor-pointer text-[10px] text-muted-foreground pt-0.5">
                      <input
                        type="checkbox"
                        checked={customConfig().isAuditSheet}
                        onChange={(e) =>
                          setCustomConfig({
                            ...customConfig(),
                            isAuditSheet: e.currentTarget.checked,
                          })
                        }
                        class="rounded border-border text-primary"
                      />
                      <span>{t('barcode.auditSheetMode')}</span>
                    </label>

                    {/* Bottom Action Buttons */}
                    <Show when={isAdmin()}>
                      <div class="pt-2 border-t border-border/50 flex items-center justify-end gap-1.5">
                        <Show
                          when={isCreating()}
                          fallback={
                            <>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                class="h-7 px-2 text-xs gap-1"
                                disabled={isSaving()}
                                onClick={handleSaveAsCopy}
                                title={t('barcode.saveAsCopyTitle')}
                              >
                                <IconCopy class="size-3.5" />
                                <span>{t('barcode.createCopy')}</span>
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                class="h-7 px-2.5 text-xs gap-1"
                                disabled={isSaving()}
                                onClick={handleUpdateDesign}
                                title={t('barcode.saveChangesTitle')}
                              >
                                <IconDeviceFloppy class="size-3.5" />
                                <span>{isSaving() ? t('common.saving') : t('common.save')}</span>
                              </Button>
                            </>
                          }
                        >
                          <Button
                            type="button"
                            size="sm"
                            class="h-7 px-2.5 text-xs gap-1"
                            disabled={isSaving()}
                            onClick={handleSaveNewDesign}
                            title={t('barcode.saveReusablePresetTitle')}
                          >
                            <IconDeviceFloppy class="size-3.5" />
                            <span>{isSaving() ? t('common.saving') : t('barcode.savePreset')}</span>
                          </Button>
                        </Show>
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>

              {/* Section 2: Symbology & Dimensions */}
              <div class="space-y-1.5">
                <label class="block font-semibold text-muted-foreground text-[11px] uppercase">
                  {t('barcode.symbologySizing')}
                </label>
                <select
                  value={symbology()}
                  onChange={(e) => setSymbology(e.currentTarget.value as Symbology)}
                  class="w-full h-8 text-xs px-2.5 border rounded-md bg-background"
                >
					<option value={BARCODE_SYMBOLOGY.CODE128}>{t('barcode.code128')}</option>
					<option value={BARCODE_SYMBOLOGY.CODE39}>{t('barcode.code39')}</option>
					<option value={BARCODE_SYMBOLOGY.EAN13}>{t('barcode.ean13')}</option>
					<option value={BARCODE_SYMBOLOGY.QRCODE}>{t('barcode.qrCode')}</option>
					<option value={BARCODE_SYMBOLOGY.DATAMATRIX}>{t('barcode.dataMatrix')}</option>
                </select>

                <div class="grid grid-cols-2 gap-1.5 pt-0.5">
                  <div>
                    <label class="block text-[10px] text-muted-foreground">{t('barcode.height')}</label>
                    <select
                      value={barcodeHeight()}
                      onChange={(e) => setBarcodeHeight(e.currentTarget.value as BarcodeHeight)}
                      class="w-full h-7 text-xs px-2 border rounded bg-background"
                    >
						<option value={BARCODE_HEIGHT.COMPACT}>{t('barcode.heightCompact')}</option>
						<option value={BARCODE_HEIGHT.MEDIUM}>{t('barcode.heightMedium')}</option>
						<option value={BARCODE_HEIGHT.LARGE}>{t('barcode.heightLarge')}</option>
                    </select>
                  </div>
                  <div>
                    <label class="block text-[10px] text-muted-foreground">{t('barcode.font')}</label>
                    <select
                      value={fontSize()}
                      onChange={(e) => setFontSize(e.currentTarget.value as FontSize)}
                      class="w-full h-7 text-xs px-2 border rounded bg-background"
                    >
						<option value={FONT_SIZE.SMALL}>{t('barcode.fontSmall')}</option>
						<option value={FONT_SIZE.MEDIUM}>{t('barcode.fontMedium')}</option>
						<option value={FONT_SIZE.LARGE}>{t('barcode.fontLarge')}</option>
                    </select>
                  </div>
                </div>
              </div>

              {/* Section 3: Quantity Mode & Offset */}
              <div class="space-y-1.5">
                <label class="block font-semibold text-muted-foreground text-[11px] uppercase">
                  {t('barcode.qtyModeOffset')}
                </label>
                <div class="grid grid-cols-2 gap-1.5">
                  <button
                    type="button"
					onClick={() => setQtyMode(QTY_MODE.MULTIPLY)}
                    class={`h-8 rounded-md border text-xs font-medium transition-colors flex items-center justify-center text-center px-1 truncate ${
						qtyMode() === QTY_MODE.MULTIPLY
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'bg-background hover:bg-muted'
                    }`}
                    title={t('barcode.byQtyTooltip')}
                  >
                    {t('barcode.byQtyBtn')}
                  </button>
                  <button
                    type="button"
					onClick={() => setQtyMode(QTY_MODE.SINGLE)}
                    class={`h-8 rounded-md border text-xs font-medium transition-colors flex items-center justify-center text-center px-1 truncate ${
						qtyMode() === QTY_MODE.SINGLE
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'bg-background hover:bg-muted'
                    }`}
                    title={t('barcode.singleTooltip')}
                  >
                    {t('barcode.singleBtn')}
                  </button>
                </div>

                <div class="pt-0.5">
                  <label class="block text-[10px] text-muted-foreground">
                    {t('barcode.skipUsedSlots')}
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={50}
                    value={skipCount()}
                    onInput={(e) => setSkipCount(parseInt(e.currentTarget.value, 10) || 0)}
                    class="w-full h-7 px-2 text-xs border rounded bg-background font-mono"
                  />
                </div>
              </div>

              <LabelLayoutDesigner />

              {/* Section 5: Canvas Settings */}
              <div class="space-y-1.5 sm:col-span-2 lg:col-span-4 xl:col-span-1">
                <label class="block font-semibold text-muted-foreground text-[11px] uppercase">
                  {t('barcode.canvasSettings')}
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-xs">
                  <input
                    type="checkbox"
                    checked={showScreenBorder()}
                    onChange={(e) => setShowScreenBorder(e.currentTarget.checked)}
                    class="rounded border-border text-primary"
                  />
                  <span>{t('barcode.screenDashedGrid')}</span>
                </label>

                <div class="space-y-1 pt-1">
                  <div class="flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>{t('barcode.zoomScale')}</span>
                    <span>{Math.round(zoom() * 100)}%</span>
                  </div>
                  <input
                    type="range"
                    min={0.5}
                    max={1.5}
                    step={0.05}
                    value={zoom()}
                    onInput={(e) => setZoom(parseFloat(e.currentTarget.value))}
                    class="w-full h-1.5 bg-muted rounded-lg appearance-none cursor-pointer"
                  />
                </div>
              </div>
            </div>
          </div>
        </Show>
        <Show when={statusMessage()}>
          <div class={`no-print mx-4 my-3 flex flex-wrap items-center justify-between gap-2 rounded px-3 py-2 text-xs md:mx-6 ${statusMessage()!.type === 'error' ? 'border border-destructive/20 bg-destructive/10 text-destructive' : 'border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'}`} role={statusMessage()!.type === 'error' ? 'alert' : 'status'}>
            <span>{statusMessage()!.text}</span>
            <div class="flex items-center gap-2">
              <Show when={statusMessage()!.retry}>
                <Button type="button" variant="outline" size="sm" class="h-7 text-xs" onClick={retrySavedPresetRefresh}>{t('common.retry')}</Button>
              </Show>
              <button type="button" aria-label={t('common.close')} onClick={() => setStatusMessage(null)} class="font-bold opacity-70 hover:opacity-100">×</button>
            </div>
          </div>
        </Show>

        <CardContent class="p-0 bg-muted/20 min-h-[480px] flex flex-col barcode-print-card">
          <PrintPreview />
        </CardContent>
      </Card>

      <AccurateLoginModal />
      <ItemSearchDrawer isOpen={isSearchOpen()} onClose={() => setIsSearchOpen(false)} />
      <TransactionDrawer isOpen={isTransactionOpen()} onClose={() => setIsTransactionOpen(false)} />

      {/* Delete Confirmation Alert Dialog */}
      <AlertDialog
        open={!!designToDelete()}
        onOpenChange={(open) => {
          if (!open) setDesignToDelete(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogTitle>{t('barcode.deleteDialogTitle')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('barcode.deleteDialogDesc', { name: designToDelete()?.name ?? '' })}
          </AlertDialogDescription>
          <div class="flex flex-col sm:flex-row sm:justify-end gap-2 mt-4">
            <Button
              variant="outline"
              size="sm"
              disabled={isDeletingDesign()}
              onClick={() => setDesignToDelete(null)}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              size="sm"
              disabled={isDeletingDesign()}
              onClick={handleConfirmDelete}
            >
              {isDeletingDesign() ? t('common.deleting') : t('common.delete')}
            </Button>
          </div>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
};
