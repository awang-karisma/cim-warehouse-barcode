import {
  type Component,
  createEffect,
  createMemo,
  createSignal,
  For,
  Index,
  onCleanup,
  onMount,
  Show,
} from 'solid-js';
import { useAuth, PermissionGuard } from '~/auth';
import { QTY_MODE } from '~/domain';
import * as api from '~/api';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '~/components/ui/card';
import { DataTable } from '~/components/DataTable';
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table';
import { useI18n } from '~/i18n';
import {
  refreshStatus,
  setIsLoginModalOpen,
  status,
} from '~/stores/accurateAuth';
import { qtyMode } from '~/stores/barcodeConfig';
import {
  addItem,
  clearItems,
  deleteItem,
  items,
  toggleSelectAll,
  updateItemField,
} from '~/stores/barcodeItems';
import type { AccurateInventoryItem } from '~/types';
import {
  IconFileInvoice,
  IconLoader2,
  IconPackage,
  IconPlugConnected,
  IconPlugConnectedX,
  IconPlus,
  IconScan,
  IconTrash,
} from '@tabler/icons-solidjs';
import { setIsSearchOpen, setIsTransactionOpen } from './drawerState';

export const QueueTab: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();

  let searchInputRef: HTMLInputElement | undefined;
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
    <div class="space-y-6">
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
                <PermissionGuard permission="accurate:read">
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
                </PermissionGuard>
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
                    <PermissionGuard
                      permission="accurate:create"
                      fallback={
                        <span class="inline-flex items-center gap-1.5 px-3 h-9 rounded-md text-xs font-medium bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                          <IconPlugConnectedX class="size-3.5" />
                          <span>{t('barcode.accurateDisconnected')}</span>
                        </span>
                      }
                    >
                      <button
                        type="button"
                        onClick={() => setIsLoginModalOpen(true)}
                        class="inline-flex items-center gap-1.5 px-3 h-9 rounded-md text-xs font-medium bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 hover:bg-amber-500/20 transition-colors"
                      >
                        <IconPlugConnectedX class="size-3.5" />
                        <span>{t('barcode.accurateDisconnected')}</span>
                      </button>
                    </PermissionGuard>
                  }
                >
                  <Show
                    when={auth.hasAnyPermission('accurate:update', 'accurate:delete')}
                    fallback={
                      <div class="inline-flex items-center gap-1.5 px-3 h-9 rounded-md text-xs font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                        <IconPlugConnected class="size-3.5" />
                        <span class="truncate max-w-[140px] font-semibold">
                          {status().database?.alias || t('barcode.connected')}
                        </span>
                      </div>
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
              {t('barcode.queueSummary', {
                total: items().length.toLocaleString(),
                selected: selectedItems().length,
                labels: totalLabels(),
              })}
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
                        >
                        </input>
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
    </div>
  );
};
