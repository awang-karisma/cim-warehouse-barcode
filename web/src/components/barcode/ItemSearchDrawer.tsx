import { type Component, createSignal, For, Show } from 'solid-js';
import { useI18n } from '~/i18n';
import * as api from '~/api';
import { handleSessionInvalidated } from '~/stores/accurateAuth';
import { addNormalizedItems } from '~/stores/barcodeItems';
import type { AccurateInventoryItem, AccurateNormalizedItem, AccuratePagination } from '~/types';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '~/components/ui/sheet';
import { Button } from '~/components/ui/button';
import {
  IconSearch,
  IconLoader2,
  IconPackage,
  IconPlus,
  IconChevronLeft,
  IconChevronRight,
} from '@tabler/icons-solidjs';

interface ItemSearchDrawerProps {
  isOpen: boolean;
  onClose: () => void;
}

export const ItemSearchDrawer: Component<ItemSearchDrawerProps> = (props) => {
  const { t } = useI18n();
  const [query, setQuery] = createSignal('');
  const [items, setItems] = createSignal<AccurateInventoryItem[]>([]);
  const [pagination, setPagination] = createSignal<AccuratePagination>({ page: 1, pageCount: 1, pageSize: 50, rowCount: 0 });
  const [selectedIds, setSelectedIds] = createSignal<Set<number>>(new Set());
  const [isLoading, setIsLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  const fetchItems = async (page = 1) => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await api.searchAccurateItems({ q: query(), page, pageSize: 50 });
      setItems(res.items);
      setPagination(res.pagination);
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        handleSessionInvalidated(err.message);
      }
      setError(err instanceof Error ? err.message : t('barcode.searchItemsFailed'));
    } finally {
      setIsLoading(false);
    }
  };

  const toggleSelect = (id: number) => {
    const next = new Set(selectedIds());
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    setSelectedIds(next);
  };

  const toggleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(items().filter((it) => it.canPrintBarcode).map((it) => it.id)));
    } else {
      setSelectedIds(new Set<number>());
    }
  };

  const handleAddSelected = () => {
    const selected = items().filter((it) => selectedIds().has(it.id));
    const normalized: AccurateNormalizedItem[] = selected.map((it) => ({
      itemId: it.id,
      itemNo: it.no,
      itemName: it.name,
      upcNo: it.upcNo || it.no,
      quantity: 1,
      unitName: it.unit1Name || 'Pcs',
      unitPrice: it.unitPrice,
      documentType: 'Master Item',
      documentNumber: it.no,
    }));

    addNormalizedItems(normalized);
    setSelectedIds(new Set<number>());
    props.onClose();
  };

  const isAllSelected = () => {
    const printable = items().filter((it) => it.canPrintBarcode);
    return printable.length > 0 && selectedIds().size === printable.length;
  };

  return (
    <Sheet open={props.isOpen} onOpenChange={(open) => !open && props.onClose()}>
      <SheetContent position="right" class="w-full sm:max-w-2xl flex flex-col p-0">
        <div class="px-6 py-4 border-b bg-card">
          <SheetHeader>
            <SheetTitle class="text-base flex items-center gap-2">
              <IconPackage class="size-5 text-primary" />
              {t('barcode.searchInventory')}
            </SheetTitle>
            <SheetDescription class="text-xs">
              {t('barcode.searchInventoryDesc')}
            </SheetDescription>
          </SheetHeader>

          {/* Search Controls */}
          <div class="mt-4 flex items-center gap-2">
            <div class="relative flex-1">
              <IconSearch class="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <input
                type="text"
                value={query()}
                onInput={(e) => setQuery(e.currentTarget.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void fetchItems(1);
                }}
                placeholder={t('barcode.searchItemPlaceholder')}
                class="w-full pl-9 pr-3 py-2 text-xs border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <Button onClick={() => void fetchItems(1)} disabled={isLoading()} size="sm">
              <Show when={isLoading()} fallback={<IconSearch class="size-4 mr-1.5" />}>
                <IconLoader2 class="size-4 mr-1.5 animate-spin" />
              </Show>
              {t('common.search')}
            </Button>
          </div>
        </div>

        {/* Content Table */}
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
                <span>{t('barcode.searchingItems')}</span>
              </div>
            }
          >
            <Show
              when={items().length > 0}
              fallback={
                <div class="py-16 text-center text-muted-foreground text-xs">
                  {query() ? t('barcode.noItemsMatch') : t('barcode.enterQueryPrompt')}
                </div>
              }
            >
              <div class="border rounded-md overflow-hidden bg-card">
                <table class="w-full text-xs text-left">
                  <thead class="bg-muted/50 border-b text-muted-foreground font-medium">
                    <tr>
                      <th class="p-2.5 w-8 text-center">
                        <input
                          type="checkbox"
                          checked={isAllSelected()}
                          onChange={(e) => toggleSelectAll(e.currentTarget.checked)}
                          class="rounded border-border"
                        />
                      </th>
                      <th class="p-2.5">{t('barcode.colItemCode')}</th>
                      <th class="p-2.5">{t('barcode.colItemName')}</th>
                      <th class="p-2.5">{t('barcode.colBarcode')}</th>
                      <th class="p-2.5 text-right">{t('barcode.available')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-border">
                    <For each={items()}>
                      {(it) => {
                        const isSelected = () => selectedIds().has(it.id);
                        return (
                          <tr
                            class={`hover:bg-muted/40 transition-colors ${
                              !it.canPrintBarcode ? 'opacity-50' : 'cursor-pointer'
                            } ${isSelected() ? 'bg-primary/5' : ''}`}
                            onClick={() => it.canPrintBarcode && toggleSelect(it.id)}
                          >
                            <td class="p-2.5 text-center" onClick={(e) => e.stopPropagation()}>
                              <input
                                type="checkbox"
                                checked={isSelected()}
                                disabled={!it.canPrintBarcode}
                                onChange={() => toggleSelect(it.id)}
                                class="rounded border-border"
                              />
                            </td>
                            <td class="p-2.5 font-mono text-[11px] font-medium">{it.no}</td>
                            <td class="p-2.5 font-medium text-foreground">{it.name}</td>
                            <td class="p-2.5 font-mono text-[11px] text-muted-foreground">
                              {it.upcNo || it.no}
                            </td>
                            <td class="p-2.5 text-right font-medium">
                              {it.availableToSell} {it.unit1Name}
                            </td>
                          </tr>
                        );
                      }}
                    </For>
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div class="flex items-center justify-between text-xs text-muted-foreground pt-2">
                <span>
                  {t('barcode.showingItemsOf', { count: items().length, total: pagination().rowCount })}
                </span>
                <div class="flex items-center gap-1">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pagination().page <= 1 || isLoading()}
                    onClick={() => void fetchItems(pagination().page - 1)}
                  >
                    <IconChevronLeft class="size-3.5 mr-1" /> {t('common.previous')}
                  </Button>
                  <span class="px-2 text-xs font-medium">
                    {pagination().page} / {pagination().pageCount || 1}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pagination().page >= pagination().pageCount || isLoading()}
                    onClick={() => void fetchItems(pagination().page + 1)}
                  >
                    {t('common.next')} <IconChevronRight class="size-3.5 ml-1" />
                  </Button>
                </div>
              </div>
            </Show>
          </Show>
        </div>

        {/* Footer Actions */}
        <div class="p-4 border-t bg-card flex items-center justify-between">
          <span class="text-xs text-muted-foreground">
            {t('barcode.itemsSelectedCount', { count: selectedIds().size })}
          </span>
          <div class="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={props.onClose}>
              {t('common.cancel')}
            </Button>
            <Button
              size="sm"
              disabled={selectedIds().size === 0}
              onClick={handleAddSelected}
            >
              <IconPlus class="size-4 mr-1.5" />
              {t('barcode.addToPrintQueue')}
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
};
