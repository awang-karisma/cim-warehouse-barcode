import { type Component, createEffect, createSignal, For, Show } from 'solid-js';
import { useI18n } from '~/i18n';
import * as api from '~/api';
import { handleSessionInvalidated } from '~/stores/accurateAuth';
import { addNormalizedItems } from '~/stores/barcodeItems';
import type { AccurateDocumentHeader, AccurateNormalizedItem, AccuratePagination, AccurateSourceInfo } from '~/types';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '~/components/ui/sheet';
import { Button } from '~/components/ui/button';
import { Badge } from '~/components/ui/badge';
import {
  IconSearch,
  IconLoader2,
  IconFileInvoice,
  IconPlus,
  IconChevronLeft,
  IconChevronRight,
  IconArrowLeft,
} from '@tabler/icons-solidjs';

interface TransactionDrawerProps {
  isOpen: boolean;
  onClose: () => void;
}

export const TransactionDrawer: Component<TransactionDrawerProps> = (props) => {
  const { t } = useI18n();
  const [sources, setSources] = createSignal<AccurateSourceInfo[]>([]);
  const [selectedSource, setSelectedSource] = createSignal<string>('');
  const [documents, setDocuments] = createSignal<AccurateDocumentHeader[]>([]);
  const [pagination, setPagination] = createSignal<AccuratePagination>({ page: 1, pageCount: 1, pageSize: 20, rowCount: 0 });
  const [selectedDoc, setSelectedDoc] = createSignal<AccurateDocumentHeader | null>(null);
  const [docItems, setDocItems] = createSignal<AccurateNormalizedItem[]>([]);
  const [selectedItemIndices, setSelectedItemIndices] = createSignal<Set<number>>(new Set());

  const [searchQuery, setSearchQuery] = createSignal('');
  const [fromDate, setFromDate] = createSignal('');
  const [toDate, setToDate] = createSignal('');

  const [isLoading, setIsLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  let listRequestToken = 0;
  let detailRequestToken = 0;

  createEffect(() => {
    if (props.isOpen && sources().length === 0) {
      void (async () => {
        try {
          const list = await api.getAccurateSources();
          setSources(list);
        } catch (err) {
          setError(err instanceof Error ? err.message : t('barcode.fetchSourcesFailed'));
        }
      })();
    }
  });

  const formatToAccurateDate = (d: string) => {
    if (!d) return '';
    const parts = d.split('-');
    if (parts.length === 3 && parts[0].length === 4) {
      return `${parts[2]}/${parts[1]}/${parts[0]}`;
    }
    return d;
  };

  const fetchDocuments = async (sourceId: string, page = 1) => {
    if (!sourceId) return;

    const token = ++listRequestToken;
    setIsLoading(true);
    setError(null);
    setSelectedDoc(null);
    setDocItems([]);
    setSelectedItemIndices(new Set<number>());

    try {
      const res = await api.getAccurateSourceDocuments(sourceId, {
        q: searchQuery(),
        from: formatToAccurateDate(fromDate()),
        to: formatToAccurateDate(toDate()),
        page,
        pageSize: 20,
      });
      if (token !== listRequestToken) return;
      setDocuments(res.documents);
      setPagination(res.pagination);
    } catch (err) {
      if (token !== listRequestToken) return;
      if (err instanceof api.ApiError && err.status === 401) {
        handleSessionInvalidated(err.message);
      }
      setError(err instanceof Error ? err.message : t('barcode.loadDocumentsFailed'));
    } finally {
      if (token === listRequestToken) {
        setIsLoading(false);
      }
    }
  };

  const handleSelectDoc = async (doc: AccurateDocumentHeader) => {
    const currentSource = selectedSource();
    setSelectedDoc(doc);
    setIsLoading(true);
    setError(null);
    setDocItems([]);
    setSelectedItemIndices(new Set<number>());

    const token = ++detailRequestToken;
    try {
      const items = await api.getAccurateSourceDocumentItems(currentSource, doc.id);
      if (token !== detailRequestToken || selectedSource() !== currentSource) return;
      setDocItems(items);
      setSelectedItemIndices(new Set(items.map((_, i) => i)));
    } catch (err) {
      if (token !== detailRequestToken || selectedSource() !== currentSource) return;
      if (err instanceof api.ApiError && err.status === 401) {
        handleSessionInvalidated(err.message);
      }
      setError(err instanceof Error ? err.message : t('barcode.loadDocItemsFailed'));
    } finally {
      if (token === detailRequestToken) {
        setIsLoading(false);
      }
    }
  };

  const handleAddItems = () => {
    const selected = docItems().filter((_, i) => selectedItemIndices().has(i));
    addNormalizedItems(selected);
    setSelectedDoc(null);
    setDocItems([]);
    setSelectedItemIndices(new Set<number>());
    props.onClose();
  };

  return (
    <Sheet open={props.isOpen} onOpenChange={(open) => !open && props.onClose()}>
      <SheetContent position="right" class="w-full sm:max-w-2xl flex flex-col p-0">
        <div class="px-6 py-4 border-b bg-card">
          <SheetHeader>
            <SheetTitle class="text-base flex items-center gap-2">
              <IconFileInvoice class="size-5 text-primary" />
              {t('barcode.loadTransactionsTitle')}
            </SheetTitle>
            <SheetDescription class="text-xs">
              {t('barcode.loadTransactionsDesc')}
            </SheetDescription>
          </SheetHeader>

          {/* Source Tabs */}
          <div class="mt-4 flex flex-wrap gap-1.5">
            <For each={sources()}>
              {(src) => (
                <Button
                  variant={selectedSource() === src.id ? 'default' : 'outline'}
                  size="sm"
                  disabled={!src.available}
                  onClick={() => {
                    if (selectedSource() === src.id && !selectedDoc()) return;
                    setSelectedSource(src.id);
                    setSelectedDoc(null);
                    setDocItems([]);
                    setSelectedItemIndices(new Set<number>());
                    void fetchDocuments(src.id, 1);
                  }}
                  class="text-xs h-8"
                >
                  {src.name.split(' (')[0]}
                </Button>
              )}
            </For>
          </div>

          {/* Filter Bar (only when source is selected and document is not selected) */}
          <Show when={Boolean(selectedSource() && !selectedDoc())}>
            <div class="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-2">
              <div class="relative sm:col-span-1">
                <input
                  type="text"
                  value={searchQuery()}
                  onInput={(e) => setSearchQuery(e.currentTarget.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void fetchDocuments(selectedSource(), 1);
                  }}
                  placeholder={t('barcode.docSearchPlaceholder')}
                  class="w-full h-8 px-2.5 text-xs border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>
              <input
                type="date"
                value={fromDate()}
                onInput={(e) => setFromDate(e.currentTarget.value)}
                class="h-8 px-2.5 text-xs border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
              />
              <div class="flex items-center gap-1.5">
                <input
                  type="date"
                  value={toDate()}
                  onInput={(e) => setToDate(e.currentTarget.value)}
                  class="flex-1 h-8 px-2.5 text-xs border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                />
                <Button size="sm" onClick={() => void fetchDocuments(selectedSource(), 1)} disabled={isLoading()} class="h-8">
                  <IconSearch class="size-3.5" />
                </Button>
              </div>
            </div>
          </Show>
        </div>

        {/* Content Pane */}
        <div class="flex-1 overflow-y-auto p-4 space-y-3">
          <Show when={error()}>
            <div class="p-3 bg-destructive/10 text-destructive border border-destructive/20 rounded-md text-xs">
              {error()}
            </div>
          </Show>

          <Show
            when={!isLoading()}
            fallback={
              <div class="py-16 flex flex-col items-center justify-center gap-2 text-muted-foreground text-xs">
                <IconLoader2 class="size-6 animate-spin text-primary" />
                <span>{t('barcode.loadingAccurateData')}</span>
              </div>
            }
          >
            {/* View 1: Document Details */}
            <Show
              when={selectedDoc()}
              fallback={
                <Show
                  when={selectedSource()}
                  fallback={
                    /* View 0: No Source Selected */
                    <div class="py-16 flex flex-col items-center justify-center text-center p-6 text-muted-foreground">
                      <div class="size-12 rounded-full bg-muted/60 flex items-center justify-center mb-3">
                        <IconFileInvoice class="size-6 text-muted-foreground/60" />
                      </div>
                      <p class="text-sm font-semibold text-foreground">{t('barcode.noDocTypeSelected')}</p>
                      <p class="text-xs text-muted-foreground mt-1 max-w-sm">
                        {t('barcode.noDocTypeSelectedDesc')}
                      </p>
                    </div>
                  }
                >
                  {/* View 2: Documents List */}
                  <Show
                    when={documents().length > 0}
                    fallback={
                      <div class="py-16 text-center text-muted-foreground text-xs">
                        {t('barcode.noDocsFound')}
                      </div>
                    }
                  >
                  <div class="border rounded-md overflow-hidden bg-card">
                    <table class="w-full text-xs text-left">
                      <thead class="bg-muted/50 border-b text-muted-foreground font-medium">
                        <tr>
                          <th class="p-2.5">{t('barcode.colDocNumber')}</th>
                          <th class="p-2.5">{t('common.date')}</th>
                          <th class="p-2.5">{t('barcode.colDescription')}</th>
                          <th class="p-2.5 text-right">{t('common.status')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-border">
                        <For each={documents()}>
                          {(doc) => (
                            <tr
                              class="hover:bg-muted/40 cursor-pointer transition-colors"
                              onClick={() => void handleSelectDoc(doc)}
                            >
                              <td class="p-2.5 font-mono text-[11px] font-semibold text-primary">
                                {doc.number}
                              </td>
                              <td class="p-2.5 text-muted-foreground">{doc.date || '-'}</td>
                              <td class="p-2.5 text-foreground max-w-xs truncate">
                                {doc.description || '-'}
                              </td>
                              <td class="p-2.5 text-right">
                                <Badge variant="secondary" class="text-[10px]">
                                  {doc.statusName || t('barcode.statusActive')}
                                </Badge>
                              </td>
                            </tr>
                          )}
                        </For>
                      </tbody>
                    </table>
                  </div>

                  {/* Pagination */}
                  <div class="flex items-center justify-between text-xs text-muted-foreground pt-2">
                    <span>
                      {t('barcode.paginationSummary', { page: pagination().page, pages: pagination().pageCount || 1, total: pagination().rowCount })}
                    </span>
                    <div class="flex items-center gap-1">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pagination().page <= 1 || isLoading()}
                        onClick={() => void fetchDocuments(selectedSource(), pagination().page - 1)}
                      >
                        <IconChevronLeft class="size-3.5 mr-1" /> {t('common.previous')}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pagination().page >= pagination().pageCount || isLoading()}
                        onClick={() => void fetchDocuments(selectedSource(), pagination().page + 1)}
                      >
                        {t('common.next')} <IconChevronRight class="size-3.5 ml-1" />
                      </Button>
                    </div>
                  </div>
                </Show>
                </Show>
              }
            >
              {(doc) => (
                <div class="space-y-3">
                  <div class="flex items-center justify-between bg-muted/40 p-3 rounded-lg border">
                    <div>
                      <div class="font-bold text-sm text-foreground">{doc().number}</div>
                      <div class="text-[11px] text-muted-foreground">
                        {t('barcode.dateLabel')} {doc().date || '-'} • {t('barcode.statusLabel')} {doc().statusName || '-'}
                      </div>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        setSelectedDoc(null);
                        setDocItems([]);
                        setSelectedItemIndices(new Set<number>());
                      }}
                    >
                      <IconArrowLeft class="size-3.5 mr-1" /> {t('barcode.backToDocuments')}
                    </Button>
                  </div>

                  {/* Items in document */}
                  <div class="border rounded-md overflow-hidden bg-card">
                    <table class="w-full text-xs text-left">
                      <thead class="bg-muted/50 border-b text-muted-foreground font-medium">
                        <tr>
                          <th class="p-2.5 w-8 text-center">
                            <input
                              type="checkbox"
                              checked={docItems().length > 0 && selectedItemIndices().size === docItems().length}
                              onChange={(e) => {
                                if (e.currentTarget.checked) {
                                  setSelectedItemIndices(new Set(docItems().map((_, i) => i)));
                                } else {
                                  setSelectedItemIndices(new Set<number>());
                                }
                              }}
                              class="rounded border-border"
                            />
                          </th>
                          <th class="p-2.5">{t('barcode.colItemCode')}</th>
                          <th class="p-2.5">{t('barcode.colItemName')}</th>
                          <th class="p-2.5">{t('barcode.colBarcode')}</th>
                          <th class="p-2.5 text-right">{t('barcode.colQuantity')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-border">
                        <For each={docItems()}>
                          {(item, idx) => {
                            const isChecked = selectedItemIndices().has(idx());
                            return (
                              <tr
                                class={`hover:bg-muted/40 cursor-pointer transition-colors ${
                                  isChecked ? 'bg-primary/5' : ''
                                }`}
                                onClick={() => {
                                  const next = new Set(selectedItemIndices());
                                  if (next.has(idx())) next.delete(idx());
                                  else next.add(idx());
                                  setSelectedItemIndices(next);
                                }}
                              >
                                <td class="p-2.5 text-center" onClick={(e) => e.stopPropagation()}>
                                  <input
                                    type="checkbox"
                                    checked={isChecked}
                                    onChange={() => {
                                      const next = new Set(selectedItemIndices());
                                      if (next.has(idx())) next.delete(idx());
                                      else next.add(idx());
                                      setSelectedItemIndices(next);
                                    }}
                                    class="rounded border-border"
                                  />
                                </td>
                                <td class="p-2.5 font-mono text-[11px] font-medium">{item.itemNo}</td>
                                <td class="p-2.5 font-medium text-foreground">{item.itemName}</td>
                                <td class="p-2.5 font-mono text-[11px] text-muted-foreground">
                                  {item.upcNo || item.itemNo}
                                </td>
                                <td class="p-2.5 text-right font-medium">
                                  {item.quantity} {item.unitName}
                                </td>
                              </tr>
                            );
                          }}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </Show>
          </Show>
        </div>

        {/* Footer */}
        <div class="p-4 border-t bg-card flex items-center justify-between">
          <Show
            when={selectedDoc()}
            fallback={
              <Button variant="outline" size="sm" onClick={props.onClose} class="ml-auto">
                {t('common.close')}
              </Button>
            }
          >
            <span class="text-xs text-muted-foreground">
              {t('barcode.itemsSelectedOfTotal', { selected: selectedItemIndices().size, total: docItems().length })}
            </span>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setSelectedDoc(null);
                  setDocItems([]);
                  setSelectedItemIndices(new Set<number>());
                }}
              >
                {t('common.back')}
              </Button>
              <Button
                size="sm"
                disabled={selectedItemIndices().size === 0}
                onClick={handleAddItems}
              >
                <IconPlus class="size-4 mr-1.5" />
                {t('barcode.addSelectedToQueue')}
              </Button>
            </div>
          </Show>
        </div>
      </SheetContent>
    </Sheet>
  );
};
