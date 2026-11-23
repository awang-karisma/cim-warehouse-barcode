import { type Component, createMemo, createSignal, For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import {
  IconAlertTriangle,
  IconBarcode,
  IconCheck,
  IconFilter,
  IconInfoCircle,
  IconLoader2,
  IconMinus,
  IconPlugConnected,
  IconPlus,
  IconPrinter,
  IconRefresh,
  IconRotate,
  IconSearch,
  IconX,
} from '@tabler/icons-solidjs';
import { useAuth } from '~/auth';
import { DataTable } from '~/components/DataTable';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox, CheckboxControl, CheckboxLabel } from '~/components/ui/checkbox';
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table';
import { QTY_MODE } from '~/domain';
import { useI18n } from '~/i18n';
import { ROUTES } from '~/routes';
import { activeStatus } from '~/stores/accurateAuth';
import { qtyMode } from '~/stores/barcodeConfig';
import type {
  TransactionDocumentRecord,
  TransactionLineDraft,
  TransactionPrintStore,
} from './types';

export const SourceConnectionNotice: Component<{
  store: TransactionPrintStore;
  compact?: boolean;
}> = (props) => {
  const { t } = useI18n();
  const auth = useAuth();

  return (
    <Show
      when={props.store.isLiveAccurate()}
      fallback={
        <div
          role="note"
          class="no-print flex flex-wrap items-center justify-between gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-900 dark:text-amber-200"
        >
          <div class="flex items-start gap-2 min-w-0 flex-1">
            <IconInfoCircle class="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <div class="min-w-0 flex-1 flex flex-wrap items-center gap-x-2 gap-y-0.5">
              <Badge
                variant="outline"
                class="border-amber-500/40 bg-amber-500/15 text-[11px] font-semibold text-amber-800 dark:text-amber-200"
              >
                {t('transactionPrint.sampleDataBadge')}
              </Badge>
              <span class="leading-snug">
                {props.compact
                  ? t('transactionPrint.previewSampleNotice')
                  : t('transactionPrint.sampleDataNotice')}
              </span>
            </div>
          </div>
          <Show when={!props.compact && auth.hasPermission('accurate_account:read')}>
            <Button
              as={A}
              href={ROUTES.accurate}
              variant="outline"
              size="sm"
              class="h-7 shrink-0 text-[11px] border-amber-500/40 bg-background/80"
            >
              {t('transactionPrint.connectAccurate')}
            </Button>
          </Show>
        </div>
      }
    >
      <Show when={!props.compact}>
        <div class="no-print flex flex-wrap items-center justify-between gap-2 rounded-lg border border-emerald-500/25 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-900 dark:text-emerald-200">
          <div class="flex items-center gap-2 min-w-0">
            <IconPlugConnected class="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
            <span class="truncate font-medium">
              {t('transactionPrint.liveAccurateConnected', {
                account:
                  activeStatus().accountName ||
                  activeStatus().database?.alias ||
                  'Accurate Online',
              })}
            </span>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={props.store.loadingDocs()}
            onClick={() => void props.store.refreshDocuments(props.store.pagination().page)}
            class="h-7 gap-1 text-[11px]"
          >
            <IconRefresh
              class={`size-3.5 ${props.store.loadingDocs() ? 'animate-spin' : ''}`}
            />
            <span>{t('common.refresh')}</span>
          </Button>
        </div>
      </Show>
    </Show>
  );
};

export const DocumentStatusBadge: Component<{ statusName: string }> = (props) => {
  const { t } = useI18n();
  const display = () => props.statusName || t('barcode.statusActive');
  const normalized = () => display().toLowerCase();
  const variant = () => {
    const s = normalized();
    if (s.includes('disetujui') || s.includes('selesai') || s.includes('approved') || s.includes('closed')) {
      return 'success' as const;
    }
    if (
      s.includes('sebagian') ||
      s.includes('menunggu') ||
      s.includes('proses') ||
      s.includes('partial') ||
      s.includes('waiting') ||
      s.includes('queue')
    ) {
      return 'warning' as const;
    }
    return 'secondary' as const;
  };

  return (
    <Badge variant={variant()} class="whitespace-nowrap font-medium">
      {display()}
    </Badge>
  );
};

export const DocumentFilterBar: Component<{
  store: TransactionPrintStore;
  showBulkActions?: boolean;
  searchInputId?: string;
  statusSelectId?: string;
}> = (props) => {
  const { t } = useI18n();
  const searchId = () => props.searchInputId ?? 'tx-doc-search-input';
  const statusId = () => props.statusSelectId ?? 'tx-doc-status-select';
  const filtered = () => props.store.filteredDocuments();
  const allFilteredIncluded = () =>
    filtered().length > 0 &&
    filtered().every((d) => {
      const sel = props.store.getDocSelectionInfo(d.header.id);
      return sel.itemsLoaded ? sel.allSelected : sel.included;
    });

  return (
    <div class="space-y-2.5">
      <div class="grid grid-cols-1 gap-2 sm:grid-cols-12">
        <div class="sm:col-span-7 space-y-1">
          <label for={searchId()} class="block text-xs font-medium text-muted-foreground">
            {t('transactionPrint.searchLabel')}
          </label>
          <div class="relative flex items-center">
            <IconSearch class="pointer-events-none absolute left-2.5 size-4 text-muted-foreground" />
            <input
              id={searchId()}
              type="search"
              value={props.store.searchQuery()}
              onInput={(e) => props.store.setSearchQuery(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && props.store.isLiveAccurate()) {
                  void props.store.refreshDocuments(1);
                }
              }}
              placeholder={t(props.store.config.searchPlaceholderKey)}
              class="h-10 sm:h-9 w-full rounded-md border border-input bg-background pl-8 pr-8 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
            <Show when={props.store.searchQuery().length > 0}>
              <button
                type="button"
                aria-label={t('transactionPrint.clearSearch')}
                onClick={() => props.store.setSearchQuery('')}
                class="absolute right-2 rounded p-1 text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <IconX class="size-4" />
              </button>
            </Show>
          </div>
        </div>

        <div class="sm:col-span-5 space-y-1">
          <label for={statusId()} class="block text-xs font-medium text-muted-foreground">
            {t('transactionPrint.statusFilterLabel')}
          </label>
          <select
            id={statusId()}
            value={props.store.statusFilter()}
            onChange={(e) => props.store.setStatusFilter(e.currentTarget.value)}
            class="h-10 sm:h-9 w-full rounded-md border border-input bg-background px-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <option value="all">
              {t('transactionPrint.statusAll', { count: props.store.documents().length })}
            </option>
            <For each={props.store.statusOptions()}>
              {(opt) => (
                <option value={opt.value}>
                  {`${opt.label} (${opt.count})`}
                </option>
              )}
            </For>
          </select>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="text-xs text-muted-foreground tabular-nums">
          {t('transactionPrint.showingDocs', {
            shown: filtered().length,
            total: props.store.documents().length,
          })}
        </span>

        <div class="flex flex-wrap items-center gap-1.5">
          <Show when={props.store.hasActiveFilters()}>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="h-8 text-xs text-muted-foreground hover:text-foreground"
              onClick={() => props.store.resetFilters()}
            >
              {t('transactionPrint.resetFilters')}
            </Button>
          </Show>
          <Show when={props.showBulkActions && filtered().length > 0}>
            <Button
              type="button"
              variant="outline"
              size="sm"
              class="h-8 text-xs"
              onClick={() =>
                props.store.toggleFilteredDocsIncluded(!allFilteredIncluded())
              }
            >
              {allFilteredIncluded()
                ? t('transactionPrint.excludeFiltered')
                : t('transactionPrint.includeFiltered')}
            </Button>
          </Show>
        </div>
      </div>
    </div>
  );
};

export const EmptyFilteredDocuments: Component<{
  store: TransactionPrintStore;
}> = (props) => {
  const { t } = useI18n();
  return (
    <div class="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed bg-muted/20 px-4 py-8 text-center">
      <IconSearch class="size-8 text-muted-foreground/60" />
      <div class="space-y-1">
        <p class="text-sm font-semibold text-foreground">
          {t('transactionPrint.emptySearchTitle')}
        </p>
        <p class="text-xs text-muted-foreground max-w-md">
          {t('transactionPrint.emptySearchDesc')}
        </p>
      </div>
      <Show when={props.store.hasActiveFilters()}>
        <Button
          type="button"
          variant="outline"
          size="sm"
          class="mt-1 h-9 text-xs"
          onClick={() => props.store.resetFilters()}
        >
          {t('transactionPrint.resetFilters')}
        </Button>
      </Show>
    </div>
  );
};

export const TransactionSummaryBar: Component<{
  store: TransactionPrintStore;
  class?: string;
}> = (props) => {
  const { t } = useI18n();
  const includedCount = () => props.store.includedDocIds().size;
  const selectedCount = () => props.store.selectedLines().length;
  const labelsCount = () => props.store.totalLabelsCount();
  const errCount = () => props.store.validationErrors().length;

  return (
    <div
      class={`no-print space-y-2 rounded-lg border bg-card p-3 sm:p-4 shadow-sm ${props.class ?? ''}`}
    >
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span class="inline-flex items-center rounded-md border bg-muted/50 px-2.5 py-1 font-medium text-foreground tabular-nums">
            {t('transactionPrint.includedDocsCount', { count: includedCount() })}
          </span>
          <span class="inline-flex items-center rounded-md border bg-muted/50 px-2.5 py-1 font-medium text-foreground tabular-nums">
            {t('transactionPrint.selectedLinesCount', { count: selectedCount() })}
          </span>
          <span class="inline-flex items-center gap-1.5 rounded-md border border-primary/25 bg-primary/10 px-2.5 py-1 font-semibold text-primary tabular-nums">
            <span>{t('transactionPrint.totalLabelsCount', { count: labelsCount() })}</span>
            <span class="text-[11px] font-normal opacity-80">
              •{' '}
              {qtyMode() === QTY_MODE.MULTIPLY
                ? t('transactionPrint.modeMultiply')
                : t('transactionPrint.modeSingle')}
            </span>
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <Show when={selectedCount() > 0 || includedCount() > 0}>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="h-9 text-xs text-muted-foreground hover:text-destructive"
              onClick={() => props.store.clearAllSelection()}
            >
              {t('transactionPrint.clearSelection')}
            </Button>
          </Show>
          <Button
            type="button"
            size="sm"
            disabled={!props.store.canPreview()}
            onClick={() => props.store.setIsPreviewOpen(true)}
            class="h-9 gap-1.5 text-xs"
          >
            <IconPrinter class="size-4" />
            <span>{t('transactionPrint.previewLabels', { count: labelsCount() })}</span>
          </Button>
        </div>
      </div>

      <Show when={selectedCount() === 0}>
        <p class="text-xs text-muted-foreground">{t('transactionPrint.emptySelectionHint')}</p>
      </Show>

      <Show when={errCount() > 0}>
        <div
          role="alert"
          class="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive"
        >
          <IconAlertTriangle class="mt-0.5 size-4 shrink-0" />
          <span>{t('transactionPrint.validationBlocked', { count: errCount() })}</span>
        </div>
      </Show>
    </div>
  );
};

export const DocumentDetailsMeta: Component<{
  doc: TransactionDocumentRecord;
  store: TransactionPrintStore;
}> = (props) => {
  const { t } = useI18n();
  const sel = () => props.store.getDocSelectionInfo(props.doc.header.id);

  return (
    <div class="flex flex-col gap-3 rounded-lg border bg-muted/30 p-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0 space-y-1">
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-mono text-sm font-semibold text-foreground">
            {props.doc.header.number}
          </span>
          <DocumentStatusBadge statusName={props.doc.header.statusName} />
          <Show when={props.doc.header.date}>
            <span class="text-xs text-muted-foreground tabular-nums">
              {props.doc.header.date}
            </span>
          </Show>
        </div>
        <Show when={props.doc.header.description}>
          <p class="text-sm text-foreground break-words">{props.doc.header.description}</p>
        </Show>
        <Show when={props.doc.partyName || props.doc.warehouseName}>
          <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <Show when={props.doc.partyName}>
              <span>
                {t(props.store.config.partyLabelKey ?? 'transactionPrint.partyLabel')}:{' '}
                <strong class="font-medium text-foreground">{props.doc.partyName}</strong>
              </span>
            </Show>
            <Show when={props.doc.warehouseName}>
              <span>
                {t('transactionPrint.warehouseLabel')}:{' '}
                <strong class="font-medium text-foreground">{props.doc.warehouseName}</strong>
              </span>
            </Show>
          </div>
        </Show>
      </div>

      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <Button
          type="button"
          variant={sel().allSelected ? 'secondary' : 'outline'}
          size="sm"
          disabled={sel().loadingItems}
          class="h-9 sm:h-8 w-full sm:w-auto text-xs"
          onClick={() =>
            props.store.toggleDocIncluded(props.doc.header.id, !sel().allSelected)
          }
        >
          {sel().allSelected
            ? t('transactionPrint.excludeAllDocLines')
            : t('transactionPrint.includeAllDocLines')}
        </Button>
      </div>
    </div>
  );
};

export const EditableLineTable: Component<{
  lines: TransactionLineDraft[];
  store: TransactionPrintStore;
}> = (props) => {
  const { t, intlLocale } = useI18n();
  const lineKeys = createMemo(
    () => props.lines.map((l) => l.key),
    undefined,
    {
      equals: (a, b) =>
        a.length === b.length && a.every((key, index) => key === b[index]),
    }
  );

  return (
    <>
      {/* Mobile stacked line cards (< md) — zero horizontal scrolling */}
      <div class="md:hidden space-y-2.5">
        <For each={lineKeys()}>
          {(lineKey) => {
            const line = () => props.store.getLineByKey(lineKey);
            const validation = () => {
              const current = line();
              return current
                ? props.store.getLineValidation(current)
                : { nameEmpty: false, quantityInvalid: false, parsedQty: 1, valid: true };
            };
            const isModified = () => {
              const current = line();
              if (!current) return false;
              const origQty =
                current.original.quantity > 0 ? Math.floor(current.original.quantity) : 1;
              return (
                current.itemName !== current.original.itemName ||
                current.quantityText !== String(origQty)
              );
            };
            const hasUpc = () => Boolean(line()?.original.upcNo);
            const parsedQty = () => validation().parsedQty;

            return (
              <Show when={line()}>
                {(currentLine) => (
                  <div
                    class={`rounded-lg border p-3 space-y-2.5 transition-colors ${
                      currentLine().selected
                        ? 'border-primary/40 bg-card'
                        : 'border-border bg-muted/20 opacity-80'
                    }`}
                  >
                    <div class="flex items-start justify-between gap-2">
                      <div class="flex items-start gap-2.5 min-w-0">
                        <div class="pt-0.5 shrink-0">
                          <Checkbox
                            checked={currentLine().selected}
                            onChange={(checked) =>
                              props.store.toggleLineSelected(currentLine().key, checked)
                            }
                            class="items-center"
                          >
                            <CheckboxControl class="size-5" />
                            <CheckboxLabel class="sr-only">
                              {t('transactionPrint.selectLineAria', {
                                name: currentLine().original.itemName,
                                doc: currentLine().original.documentNumber,
                              })}
                            </CheckboxLabel>
                          </Checkbox>
                        </div>
                        <div class="min-w-0 space-y-0.5">
                          <div class="font-mono text-xs font-semibold text-foreground break-all">
                            {currentLine().original.itemNo}
                          </div>
                          <div class="flex items-center gap-1 font-mono text-[11px] text-muted-foreground break-all">
                            <IconBarcode class="size-3.5 shrink-0" />
                            <span>
                              {hasUpc()
                                ? currentLine().original.upcNo
                                : t('transactionPrint.upcFallback', {
                                    code: currentLine().original.itemNo,
                                  })}
                            </span>
                          </div>
                        </div>
                      </div>

                      <Show when={isModified()}>
                        <button
                          type="button"
                          title={t('transactionPrint.resetLine')}
                          aria-label={t('transactionPrint.resetLine')}
                          onClick={() => props.store.resetLineToOriginal(currentLine().key)}
                          class="flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-background text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          <IconRotate class="size-3.5" />
                        </button>
                      </Show>
                    </div>

                    <div class="space-y-1">
                      <label class="block text-[11px] font-medium text-muted-foreground">
                        {t('transactionPrint.colItemName')}
                      </label>
                      <input
                        type="text"
                        value={currentLine().itemName}
                        aria-label={t('transactionPrint.itemNameAria', {
                          code: currentLine().original.itemNo,
                        })}
                        aria-invalid={validation().nameEmpty}
                        onInput={(e) =>
                          props.store.updateLineName(
                            currentLine().key,
                            e.currentTarget.value
                          )
                        }
                        class={`h-10 w-full rounded-md border px-3 text-sm bg-background focus-visible:outline-none focus-visible:ring-2 ${
                          validation().nameEmpty
                            ? 'border-destructive focus-visible:ring-destructive'
                            : 'border-input focus-visible:ring-ring'
                        }`}
                      />
                      <Show when={validation().nameEmpty}>
                        <p class="text-xs font-medium text-destructive" role="alert">
                          {t('transactionPrint.nameRequiredError')}
                        </p>
                      </Show>
                    </div>

                    <div class="flex flex-wrap items-center justify-between gap-2 pt-0.5">
                      <div class="space-y-1">
                        <div
                          class={`inline-flex items-center rounded-md border bg-background ${
                            validation().quantityInvalid ? 'border-destructive' : 'border-input'
                          }`}
                        >
                          <button
                            type="button"
                            aria-label={t('transactionPrint.decreaseQty')}
                            disabled={parsedQty() === null || parsedQty()! <= 1}
                            onClick={() => props.store.stepLineQuantity(currentLine().key, -1)}
                            class="flex h-10 w-10 items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-l-md"
                          >
                            <IconMinus class="size-4" />
                          </button>
                          <input
                            type="text"
                            inputMode="numeric"
                            value={currentLine().quantityText}
                            aria-label={t('transactionPrint.quantityAria', {
                              code: currentLine().original.itemNo,
                            })}
                            aria-invalid={validation().quantityInvalid}
                            onInput={(e) =>
                              props.store.updateLineQuantityText(
                                currentLine().key,
                                e.currentTarget.value
                              )
                            }
                            class="h-10 w-14 bg-transparent text-center text-sm font-semibold tabular-nums focus:outline-none"
                          />
                          <button
                            type="button"
                            aria-label={t('transactionPrint.increaseQty')}
                            onClick={() => props.store.stepLineQuantity(currentLine().key, 1)}
                            class="flex h-10 w-10 items-center justify-center text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-r-md"
                          >
                            <IconPlus class="size-4" />
                          </button>
                        </div>
                        <Show when={validation().quantityInvalid}>
                          <p class="text-xs font-medium text-destructive" role="alert">
                            {t('transactionPrint.qtyIntegerError')}
                          </p>
                        </Show>
                      </div>

                      <div class="text-right tabular-nums">
                        <div class="text-xs font-semibold text-foreground">
                          {currentLine().original.unitName || 'Pcs'}
                        </div>
                        <div class="text-xs text-muted-foreground">
                          {`Rp ${Number(currentLine().original.unitPrice ?? 0).toLocaleString(intlLocale())}`}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </Show>
            );
          }}
        </For>
      </div>

      {/* Desktop DataTable (md+) */}
      <div class="hidden md:block">
        <DataTable>
          <TableHeader>
            <TableRow>
              <TableHead class="w-10 text-center">{t('transactionPrint.colSelect')}</TableHead>
              <TableHead class="w-40">{t('transactionPrint.colBarcode')}</TableHead>
              <TableHead class="min-w-[200px]">{t('transactionPrint.colItemName')}</TableHead>
              <TableHead class="w-36 text-center">{t('transactionPrint.colPrintQty')}</TableHead>
              <TableHead class="w-32 text-right">{t('transactionPrint.colUnitPrice')}</TableHead>
              <TableHead class="w-10 text-center"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <For each={lineKeys()}>
              {(lineKey) => {
                const line = () => props.store.getLineByKey(lineKey);
                const validation = () => {
                  const current = line();
                  return current
                    ? props.store.getLineValidation(current)
                    : { nameEmpty: false, quantityInvalid: false, parsedQty: 1, valid: true };
                };
                const isModified = () => {
                  const current = line();
                  if (!current) return false;
                  const origQty =
                    current.original.quantity > 0 ? Math.floor(current.original.quantity) : 1;
                  return (
                    current.itemName !== current.original.itemName ||
                    current.quantityText !== String(origQty)
                  );
                };
                const hasUpc = () => Boolean(line()?.original.upcNo);
                const parsedQty = () => validation().parsedQty;

                return (
                  <Show when={line()}>
                    {(currentLine) => (
                      <TableRow class={currentLine().selected ? '' : 'opacity-65'}>
                        <TableCell class="text-center align-top pt-3.5">
                          <Checkbox
                            checked={currentLine().selected}
                            onChange={(checked) =>
                              props.store.toggleLineSelected(currentLine().key, checked)
                            }
                            class="justify-center"
                          >
                            <CheckboxControl />
                            <CheckboxLabel class="sr-only">
                              {t('transactionPrint.selectLineAria', {
                                name: currentLine().original.itemName,
                                doc: currentLine().original.documentNumber,
                              })}
                            </CheckboxLabel>
                          </Checkbox>
                        </TableCell>

                        <TableCell class="align-top pt-3">
                          <div class="space-y-0.5 min-w-0">
                            <div class="font-mono text-xs font-semibold text-foreground break-all">
                              {currentLine().original.itemNo}
                            </div>
                            <div class="flex items-center gap-1 font-mono text-[11px] text-muted-foreground break-all">
                              <IconBarcode class="size-3 shrink-0" />
                              <span>
                                {hasUpc()
                                  ? currentLine().original.upcNo
                                  : t('transactionPrint.upcFallback', {
                                      code: currentLine().original.itemNo,
                                    })}
                              </span>
                            </div>
                          </div>
                        </TableCell>

                        <TableCell class="align-top p-2">
                          <div class="space-y-1">
                            <input
                              type="text"
                              value={currentLine().itemName}
                              aria-label={t('transactionPrint.itemNameAria', {
                                code: currentLine().original.itemNo,
                              })}
                              aria-invalid={validation().nameEmpty}
                              onInput={(e) =>
                                props.store.updateLineName(
                                  currentLine().key,
                                  e.currentTarget.value
                                )
                              }
                              class={`h-8 w-full min-w-[180px] rounded-md border px-2.5 text-xs bg-background focus-visible:outline-none focus-visible:ring-2 ${
                                validation().nameEmpty
                                  ? 'border-destructive focus-visible:ring-destructive'
                                  : 'border-input focus-visible:ring-ring'
                              }`}
                            />
                            <Show when={validation().nameEmpty}>
                              <p class="text-[11px] font-medium text-destructive" role="alert">
                                {t('transactionPrint.nameRequiredError')}
                              </p>
                            </Show>
                          </div>
                        </TableCell>

                        <TableCell class="align-top p-2 text-center">
                          <div class="inline-flex flex-col items-center gap-1">
                            <div
                              class={`inline-flex items-center rounded-md border bg-background ${
                                validation().quantityInvalid ? 'border-destructive' : 'border-input'
                              }`}
                            >
                              <button
                                type="button"
                                aria-label={t('transactionPrint.decreaseQty')}
                                disabled={parsedQty() === null || parsedQty()! <= 1}
                                onClick={() => props.store.stepLineQuantity(currentLine().key, -1)}
                                class="flex h-8 w-7 items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-l-md"
                              >
                                <IconMinus class="size-3.5" />
                              </button>
                              <input
                                type="text"
                                inputMode="numeric"
                                value={currentLine().quantityText}
                                aria-label={t('transactionPrint.quantityAria', {
                                  code: currentLine().original.itemNo,
                                })}
                                aria-invalid={validation().quantityInvalid}
                                onInput={(e) =>
                                  props.store.updateLineQuantityText(
                                    currentLine().key,
                                    e.currentTarget.value
                                  )
                                }
                                class="h-8 w-12 bg-transparent text-center text-xs font-semibold tabular-nums focus:outline-none"
                              />
                              <button
                                type="button"
                                aria-label={t('transactionPrint.increaseQty')}
                                onClick={() => props.store.stepLineQuantity(currentLine().key, 1)}
                                class="flex h-8 w-7 items-center justify-center text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-r-md"
                              >
                                <IconPlus class="size-3.5" />
                              </button>
                            </div>
                            <Show when={validation().quantityInvalid}>
                              <p
                                class="text-[11px] font-medium text-destructive max-w-[140px]"
                                role="alert"
                              >
                                {t('transactionPrint.qtyIntegerError')}
                              </p>
                            </Show>
                          </div>
                        </TableCell>

                        <TableCell class="align-top pt-3 text-right whitespace-nowrap">
                          <div class="text-xs font-medium text-foreground tabular-nums">
                            {currentLine().original.unitName || 'Pcs'}
                          </div>
                          <div class="text-[11px] text-muted-foreground tabular-nums">
                            {`Rp ${Number(currentLine().original.unitPrice ?? 0).toLocaleString(intlLocale())}`}
                          </div>
                        </TableCell>

                        <TableCell class="align-top pt-2.5 text-center">
                          <Show when={isModified()}>
                            <button
                              type="button"
                              title={t('transactionPrint.resetLine')}
                              aria-label={t('transactionPrint.resetLine')}
                              onClick={() => props.store.resetLineToOriginal(currentLine().key)}
                              class="rounded p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                            >
                              <IconRotate class="size-3.5" />
                            </button>
                          </Show>
                        </TableCell>
                      </TableRow>
                    )}
                  </Show>
                );
              }}
            </For>
          </TableBody>
        </DataTable>
      </div>
    </>
  );
};

export const DocumentIncludeCheckbox: Component<{
  doc: TransactionDocumentRecord;
  store: TransactionPrintStore;
  largeTouch?: boolean;
}> = (props) => {
  const { t } = useI18n();
  const sel = () => props.store.getDocSelectionInfo(props.doc.header.id);

  return (
    <Show
      when={!sel().loadingItems}
      fallback={
        <div class="flex size-5 items-center justify-center">
          <IconLoader2 class="size-4 animate-spin text-primary" />
        </div>
      }
    >
      <Checkbox
        checked={sel().itemsLoaded ? sel().allSelected : sel().included}
        indeterminate={sel().someSelected}
        onChange={(checked) => props.store.toggleDocIncluded(props.doc.header.id, checked)}
        class="justify-center"
      >
        <CheckboxControl
          class={`data-[indeterminate]:bg-primary data-[indeterminate]:text-primary-foreground ${
            props.largeTouch ? 'size-5' : ''
          }`}
        >
          {sel().someSelected ? <IconMinus class="size-3.5" /> : <IconCheck class="size-3.5" />}
        </CheckboxControl>
        <CheckboxLabel class="sr-only">
          {sel().allSelected
            ? t('transactionPrint.excludeDoc', { number: props.doc.header.number })
            : t('transactionPrint.includeDoc', { number: props.doc.header.number })}
        </CheckboxLabel>
      </Checkbox>
    </Show>
  );
};

export const MobileFloatingDock: Component<{
  store: TransactionPrintStore;
  activeStep: () => 'orders' | 'items';
  onChangeStep: (step: 'orders' | 'items') => void;
}> = (props) => {
  const { t } = useI18n();
  const [filterOpen, setFilterOpen] = createSignal(false);

  const hasFilter = () => props.store.hasActiveFilters();
  const errCount = () => props.store.validationErrors().length;
  const labelsCount = () => props.store.totalLabelsCount();
  const selectedCount = () => props.store.selectedLines().length;
  const includedCount = () => props.store.includedDocIds().size;
  const focusedNumber = () => props.store.focusedDocument()?.header.number ?? '—';

  return (
    <div class="no-print lg:hidden">
      <Show when={filterOpen()}>
        <div
          class="df-dock-backdrop"
          onClick={() => setFilterOpen(false)}
          aria-hidden="true"
        />
      </Show>

      <div class="fixed inset-x-3 bottom-3 z-40 mx-auto max-w-lg pb-[env(safe-area-inset-bottom)]">
        <Show when={filterOpen()}>
          <div class="mb-2 rounded-xl border bg-card p-3.5 shadow-xl space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-foreground">
                {t('transactionPrint.filterButton')}
              </span>
              <button
                type="button"
                aria-label={t('transactionPrint.closeFilter')}
                onClick={() => setFilterOpen(false)}
                class="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <IconX class="size-4" />
              </button>
            </div>
            <DocumentFilterBar
              store={props.store}
              showBulkActions
              searchInputId="tx-mobile-dock-search"
              statusSelectId="tx-mobile-dock-status"
            />
            <Show when={selectedCount() > 0 || includedCount() > 0}>
              <div class="flex justify-end border-t pt-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  class="h-8 text-xs text-destructive hover:bg-destructive/10"
                  onClick={() => props.store.clearAllSelection()}
                >
                  {t('transactionPrint.clearSelection')}
                </Button>
              </div>
            </Show>
          </div>
        </Show>

        <div class="rounded-xl border bg-card/95 backdrop-blur-md p-2 shadow-lg space-y-2">
          <Show when={errCount() > 0}>
            <div
              role="alert"
              class="flex items-center gap-1.5 rounded-md border border-destructive/30 bg-destructive/10 px-2.5 py-1.5 text-[11px] font-medium text-destructive"
            >
              <IconAlertTriangle class="size-3.5 shrink-0" />
              <span class="truncate">
                {t('transactionPrint.validationBlocked', { count: errCount() })}
              </span>
            </div>
          </Show>

          <div class="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
            <button
              type="button"
              onClick={() => props.onChangeStep('orders')}
              class={`h-9 rounded-md px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                props.activeStep() === 'orders'
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {t('transactionPrint.stepOrders', {
                count: props.store.filteredDocuments().length,
              })}
            </button>
            <button
              type="button"
              onClick={() => props.onChangeStep('items')}
              class={`h-9 rounded-md px-2 text-xs font-medium truncate transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                props.activeStep() === 'items'
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {t('transactionPrint.stepItems')} · {focusedNumber()}
            </button>
          </div>

          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <Button
                type="button"
                variant={filterOpen() || hasFilter() ? 'secondary' : 'outline'}
                size="sm"
                aria-expanded={filterOpen()}
                onClick={() => setFilterOpen((v) => !v)}
                class="h-10 shrink-0 gap-1.5 px-3 text-xs relative"
              >
                <IconFilter class="size-4" />
                <span>{t('transactionPrint.filterButton')}</span>
                <Show when={hasFilter()}>
                  <span class="size-2 rounded-full bg-primary" />
                </Show>
              </Button>

              <div class="min-w-0 text-[11px] leading-tight text-muted-foreground tabular-nums truncate">
                {t('transactionPrint.compactCountSummary', {
                  docs: includedCount(),
                  lines: selectedCount(),
                  labels: labelsCount(),
                })}
              </div>
            </div>

            <Button
              type="button"
              size="sm"
              disabled={!props.store.canPreview()}
              onClick={() => props.store.setIsPreviewOpen(true)}
              class="h-10 shrink-0 gap-1.5 px-3.5 text-xs"
            >
              <IconPrinter class="size-4" />
              <span>{t('transactionPrint.previewShort', { count: labelsCount() })}</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};
