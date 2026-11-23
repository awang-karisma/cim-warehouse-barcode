import { type Component, createSignal, For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import {
  IconArrowLeft,
  IconChevronLeft,
  IconChevronRight,
  IconFileInvoice,
  IconLoader2,
  IconPrinter,
} from '@tabler/icons-solidjs';
import { PrintPreview } from '~/components/barcode/PrintPreview';
import { Button } from '~/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '~/components/ui/card';
import { QTY_MODE } from '~/domain';
import { useI18n } from '~/i18n';
import { ROUTES } from '~/routes';
import { getEffectiveConfig, qtyMode } from '~/stores/barcodeConfig';
import { createTransactionPrintStore } from './model';
import {
  DocumentDetailsMeta,
  DocumentFilterBar,
  DocumentIncludeCheckbox,
  DocumentStatusBadge,
  EditableLineTable,
  EmptyFilteredDocuments,
  MobileFloatingDock,
  SourceConnectionNotice,
  TransactionSummaryBar,
} from './TransactionControls';
import type { TransactionSourceConfig } from './types';

export interface TransactionPrintWorkspaceProps {
  config: TransactionSourceConfig;
}

export const TransactionPrintWorkspace: Component<TransactionPrintWorkspaceProps> = (
  props
) => {
  const { t } = useI18n();
  const store = createTransactionPrintStore(props.config);
  const [mobileStep, setMobileStep] = createSignal<'orders' | 'items'>('orders');

  const filtered = () => store.filteredDocuments();
  const focusedDoc = () => store.focusedDocument();

  const focusAndOpenItemsMobile = (docId: number) => {
    store.focusDocument(docId);
    setMobileStep('items');
  };

  const modeLabel = () =>
    qtyMode() === QTY_MODE.MULTIPLY
      ? t('transactionPrint.modeMultiply')
      : t('transactionPrint.modeSingle');

  return (
    <div class="barcode-print-root min-w-0 space-y-3 sm:space-y-4">
      <Show
        when={!store.isPreviewOpen()}
        fallback={
          <div class="barcode-print-root space-y-4">
            <div class="no-print space-y-3">
              <SourceConnectionNotice store={store} compact />
            </div>

            <Card class="overflow-hidden barcode-print-card">
              <CardHeader class="no-print flex flex-col gap-3 border-b pb-4 sm:flex-row sm:items-center sm:justify-between space-y-0">
                <div class="min-w-0 space-y-1">
                  <CardTitle class="flex items-center gap-2 text-base font-semibold">
                    <IconPrinter class="size-5 shrink-0 text-primary" />
                    <span>{t('transactionPrint.previewTitle')}</span>
                  </CardTitle>
                  <CardDescription class="text-xs">
                    {t('transactionPrint.previewMeta', {
                      preset: getEffectiveConfig().name,
                      mode: modeLabel(),
                      items: store.previewItems().length,
                      labels: store.totalLabelsCount(),
                    })}
                  </CardDescription>
                </div>

                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => store.setIsPreviewOpen(false)}
                  class="h-9 shrink-0 gap-1.5 text-xs"
                >
                  <IconArrowLeft class="size-4" />
                  <span>{t('transactionPrint.backToEdit')}</span>
                </Button>
              </CardHeader>

              <CardContent class="p-0 bg-muted/20 min-h-[480px] flex flex-col barcode-print-card">
                <PrintPreview items={store.previewItems()} />
              </CardContent>
            </Card>
          </div>
        }
      >
        <div class="no-print space-y-3">
          <div class="flex flex-wrap items-start justify-between gap-2 sm:gap-3">
            <div class="min-w-0 space-y-1">
              <h1 class="text-lg sm:text-xl font-semibold tracking-tight text-foreground">
                {t(props.config.titleKey)}
              </h1>
              <p class="hidden sm:block text-sm text-muted-foreground max-w-3xl">
                {t(props.config.subtitleKey)}
              </p>
            </div>

            <Button
              as={A}
              href={ROUTES.barcode}
              variant="outline"
              size="sm"
              class="h-9 shrink-0 gap-1.5 text-xs"
            >
              <IconArrowLeft class="size-4" />
              <span class="hidden sm:inline">{t('transactionPrint.backToBarcode')}</span>
              <span class="sm:hidden">{t('common.back')}</span>
            </Button>
          </div>

          <SourceConnectionNotice store={store} />
        </div>

        <div class="no-print min-w-0 space-y-4 pb-36 lg:pb-0">
          <TransactionSummaryBar store={store} class="hidden lg:block" />

          <div class="grid grid-cols-1 gap-4 lg:grid-cols-12 lg:items-start">
            {/* Left column (Desktop) / Step 1 (Mobile): Document List + Search + Status Filter */}
            <Card
              class={`lg:col-span-5 min-w-0 ${
                mobileStep() === 'orders' ? 'block' : 'hidden lg:block'
              }`}
            >
              <CardHeader class="hidden lg:flex p-4 pb-3 space-y-3">
                <DocumentFilterBar
                  store={store}
                  showBulkActions
                  searchInputId="tx-desktop-search"
                  statusSelectId="tx-desktop-status"
                />
              </CardHeader>

              <CardContent class="p-3 sm:p-4 lg:pt-0 space-y-2.5">
                <Show when={store.docsError()}>
                  {(err) => (
                    <div
                      role="alert"
                      class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive"
                    >
                      <span>{err()}</span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        class="h-7 text-xs"
                        onClick={() => void store.refreshDocuments(store.pagination().page)}
                      >
                        {t('common.retry')}
                      </Button>
                    </div>
                  )}
                </Show>

                <Show
                  when={!store.loadingDocs()}
                  fallback={
                    <div class="flex flex-col items-center justify-center gap-2 py-12 text-xs text-muted-foreground">
                      <IconLoader2 class="size-6 animate-spin text-primary" />
                      <span>{t('barcode.loadingAccurateData')}</span>
                    </div>
                  }
                >
                  <Show
                    when={filtered().length > 0}
                    fallback={<EmptyFilteredDocuments store={store} />}
                  >
                    <div class="space-y-2" role="list">
                      <For each={filtered()}>
                        {(doc) => {
                          const isFocused = () =>
                            store.focusedDocId() === doc.header.id;
                          const sel = () => store.getDocSelectionInfo(doc.header.id);

                          return (
                            <div
                              role="listitem"
                              class={`flex items-start gap-3 rounded-lg border p-3 transition-colors ${
                                isFocused()
                                  ? 'border-primary bg-primary/5'
                                  : 'border-border bg-card hover:bg-muted/40'
                              }`}
                            >
                              <div class="pt-0.5 shrink-0">
                                <DocumentIncludeCheckbox
                                  doc={doc}
                                  store={store}
                                  largeTouch
                                />
                              </div>

                              <button
                                type="button"
                                onClick={() => focusAndOpenItemsMobile(doc.header.id)}
                                aria-current={isFocused() ? 'true' : undefined}
                                class="min-w-0 flex-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded"
                              >
                                <div class="flex flex-wrap items-center justify-between gap-1.5">
                                  <span class="font-mono text-sm lg:text-xs font-semibold text-foreground">
                                    {doc.header.number}
                                  </span>
                                  <DocumentStatusBadge statusName={doc.header.statusName} />
                                </div>
                                <Show when={doc.header.description}>
                                  <p class="mt-1 text-xs text-foreground break-words">
                                    {doc.header.description}
                                  </p>
                                </Show>
                                <div class="mt-1.5 flex flex-wrap items-center justify-between gap-2 text-xs lg:text-[11px] text-muted-foreground tabular-nums">
                                  <span class="truncate">
                                    {doc.header.date || '-'}
                                    <Show when={doc.partyName}>
                                      {` • ${doc.partyName}`}
                                    </Show>
                                  </span>
                                  <span class="inline-flex items-center gap-1 font-semibold text-foreground shrink-0">
                                    <Show when={sel().itemsLoaded}>
                                      <span>
                                        {sel().selectedCount}/{sel().totalCount}
                                      </span>
                                    </Show>
                                    <IconChevronRight class="size-3.5 lg:hidden text-muted-foreground" />
                                  </span>
                                </div>
                              </button>
                            </div>
                          );
                        }}
                      </For>
                    </div>
                  </Show>
                </Show>

                {/* Server pagination when connected to live Accurate */}
                <Show
                  when={
                    store.isLiveAccurate() && store.pagination().pageCount > 1
                  }
                >
                  <div class="flex items-center justify-between border-t pt-3 text-xs text-muted-foreground">
                    <span class="tabular-nums">
                      {t('barcode.paginationSummary', {
                        page: store.pagination().page,
                        pages: store.pagination().pageCount,
                        total: store.pagination().rowCount,
                      })}
                    </span>
                    <div class="flex items-center gap-1.5">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        class="h-8 text-xs"
                        disabled={store.pagination().page <= 1 || store.loadingDocs()}
                        onClick={() =>
                          void store.refreshDocuments(store.pagination().page - 1)
                        }
                      >
                        <IconChevronLeft class="size-3.5" />
                        <span class="hidden sm:inline">{t('common.previous')}</span>
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        class="h-8 text-xs"
                        disabled={
                          store.pagination().page >= store.pagination().pageCount ||
                          store.loadingDocs()
                        }
                        onClick={() =>
                          void store.refreshDocuments(store.pagination().page + 1)
                        }
                      >
                        <span class="hidden sm:inline">{t('common.next')}</span>
                        <IconChevronRight class="size-3.5" />
                      </Button>
                    </div>
                  </div>
                </Show>
              </CardContent>
            </Card>

            {/* Right column (Desktop) / Step 2 (Mobile): Focused Document Item Editor */}
            <Card
              class={`lg:col-span-7 min-w-0 ${
                mobileStep() === 'items' ? 'block' : 'hidden lg:block'
              }`}
            >
              <CardContent class="p-3 sm:p-4 space-y-4">
                <div class="flex items-center justify-between gap-2 lg:hidden">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    class="h-8 -ml-2 gap-1 text-xs"
                    onClick={() => setMobileStep('orders')}
                  >
                    <IconChevronLeft class="size-4" />
                    <span>
                      {t('transactionPrint.stepOrders', { count: filtered().length })}
                    </span>
                  </Button>
                </div>

                <Show
                  when={focusedDoc()}
                  fallback={
                    <div class="flex flex-col items-center justify-center gap-2 py-12 text-center text-muted-foreground">
                      <IconFileInvoice class="size-8 opacity-50" />
                      <p class="text-xs">{t('barcode.noDocsFound')}</p>
                    </div>
                  }
                >
                  {(doc) => (
                    <>
                      <DocumentDetailsMeta doc={doc()} store={store} />

                      <Show when={store.focusedDocError()}>
                        {(err) => (
                          <div
                            role="alert"
                            class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive"
                          >
                            <span>{err()}</span>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              class="h-7 text-xs"
                              onClick={() => store.retryFocusedDocItems()}
                            >
                              {t('common.retry')}
                            </Button>
                          </div>
                        )}
                      </Show>

                      <Show
                        when={!store.focusedDocLoading()}
                        fallback={
                          <div class="flex flex-col items-center justify-center gap-2 py-12 text-xs text-muted-foreground">
                            <IconLoader2 class="size-6 animate-spin text-primary" />
                            <span>{t('barcode.loadingAccurateData')}</span>
                          </div>
                        }
                      >
                        <Show
                          when={store.focusedDocLines().length > 0}
                          fallback={
                            <div class="rounded-lg border border-dashed bg-muted/20 p-8 text-center text-xs text-muted-foreground">
                              {t('transactionPrint.emptyDocItems')}
                            </div>
                          }
                        >
                          <EditableLineTable
                            lines={store.focusedDocLines()}
                            store={store}
                          />
                        </Show>
                      </Show>
                    </>
                  )}
                </Show>
              </CardContent>
            </Card>
          </div>

          <MobileFloatingDock
            store={store}
            activeStep={mobileStep}
            onChangeStep={setMobileStep}
          />
        </div>
      </Show>
    </div>
  );
};
