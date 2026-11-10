import { type Component, createMemo, Show } from 'solid-js';
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
import { getEffectiveConfig, qtyMode } from '~/stores/barcodeConfig';
import { items } from '~/stores/barcodeItems';
import {
  clearStatusMessage,
  retrySavedPresetRefresh,
  statusMessage,
} from '~/stores/presetDraft';
import { IconPrinter } from '@tabler/icons-solidjs';

export const PreviewTab: Component = () => {
  const { t } = useI18n();

  const selectedItems = createMemo(() => items().filter((it) => it.selected));
  const totalLabels = createMemo(() => {
    if (qtyMode() === QTY_MODE.MULTIPLY) {
      return selectedItems().reduce((acc, it) => acc + (it.quantity > 0 ? it.quantity : 1), 0);
    }
    return selectedItems().length;
  });

  return (
    <div class="barcode-print-root space-y-4">
      {/* Live Print Preview Canvas Card */}
      <Card class="overflow-hidden barcode-print-card">
        <CardHeader class="no-print flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 space-y-0 pb-4 border-b">
          <div>
            <CardTitle class="text-base font-semibold flex items-center gap-2">
              <IconPrinter class="size-5 text-primary" />
              {t('barcode.printPreview')}
            </CardTitle>
            <CardDescription>
              {t('barcode.renderingSummary', {
                count: selectedItems().length,
                labels: totalLabels(),
                preset: getEffectiveConfig().name,
              })}
            </CardDescription>
          </div>
        </CardHeader>

        <Show when={statusMessage()}>
          <div
            class={`no-print mx-4 my-3 flex flex-wrap items-center justify-between gap-2 rounded px-3 py-2 text-xs md:mx-6 ${
              statusMessage()!.type === 'error'
                ? 'border border-destructive/20 bg-destructive/10 text-destructive'
                : 'border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
            }`}
            role={statusMessage()!.type === 'error' ? 'alert' : 'status'}
          >
            <span>{statusMessage()!.text}</span>
            <div class="flex items-center gap-2">
              <Show when={statusMessage()!.retry}>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  class="h-7 text-xs"
                  onClick={retrySavedPresetRefresh}
                >
                  {t('common.retry')}
                </Button>
              </Show>
              <button
                type="button"
                aria-label={t('common.close')}
                onClick={clearStatusMessage}
                class="font-bold opacity-70 hover:opacity-100"
              >
                ×
              </button>
            </div>
          </div>
        </Show>

        <CardContent class="p-0 bg-muted/20 min-h-[480px] flex flex-col barcode-print-card">
          <PrintPreview />
        </CardContent>
      </Card>
    </div>
  );
};
