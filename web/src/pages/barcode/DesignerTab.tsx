import type { Component } from 'solid-js';
import { LabelLayoutDesigner } from '~/components/barcode/LabelLayoutDesigner';
import { Button } from '~/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '~/components/ui/card';
import {
  BARCODE_HEIGHT,
  BARCODE_SYMBOLOGY,
  FONT_SIZE,
  QTY_MODE,
} from '~/domain';
import { useI18n } from '~/i18n';
import {
  barcodeHeight,
  fontSize,
  getEffectiveConfig,
  qtyMode,
  setBarcodeHeight,
  setFontSize,
  setQtyMode,
  setShowScreenBorder,
  setSkipCount,
  setSymbology,
  setZoom,
  showScreenBorder,
  skipCount,
  symbology,
  zoom,
} from '~/stores/barcodeConfig';
import type { BarcodeHeight, FontSize, Symbology } from '~/types';
import { IconArrowRight, IconTemplate } from '@tabler/icons-solidjs';

export interface DesignerTabProps {
  onGoToPresets: () => void;
}

export const DesignerTab: Component<DesignerTabProps> = (props) => {
  const { t } = useI18n();

  return (
    <div class="space-y-6 no-print">
      {/* Active Preset Summary Strip */}
      <Card>
        <CardHeader class="p-4 sm:p-5">
          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <div class="flex items-center gap-3">
              <div class="p-2 rounded-lg bg-primary/10 text-primary">
                <IconTemplate class="size-5" />
              </div>
              <div>
                <CardTitle class="text-sm font-semibold">
                  {t('barcode.stickerPaperPreset')}: {getEffectiveConfig().name}
                </CardTitle>
                <CardDescription class="text-xs">
                  {getEffectiveConfig().cols} × {getEffectiveConfig().rows} •{' '}
                  {getEffectiveConfig().labelWidthMM} × {getEffectiveConfig().labelHeightMM} mm •{' '}
                  {getEffectiveConfig().paper}
                </CardDescription>
              </div>
            </div>

            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={props.onGoToPresets}
              class="h-8 gap-1.5 text-xs self-start sm:self-auto"
            >
              <span>{t('barcode.customizeInPresets')}</span>
              <IconArrowRight class="size-3.5" />
            </Button>
          </div>
        </CardHeader>
      </Card>

      {/* 4 Settings Sections in Grid */}
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4 text-xs">
        {/* Section 1: Symbology & Dimensions */}
        <div class="rounded-lg border bg-card p-4 space-y-3">
          <div class="font-semibold text-foreground text-xs uppercase tracking-wider">
            {t('barcode.symbologySizing')}
          </div>

          <div class="space-y-2">
            <div>
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
            </div>

            <div class="grid grid-cols-2 gap-2 pt-1">
              <div>
                <label class="block text-[11px] text-muted-foreground mb-1">{t('barcode.height')}</label>
                <select
                  value={barcodeHeight()}
                  onChange={(e) => setBarcodeHeight(e.currentTarget.value as BarcodeHeight)}
                  class="w-full h-8 text-xs px-2 border rounded-md bg-background"
                >
                  <option value={BARCODE_HEIGHT.COMPACT}>{t('barcode.heightCompact')}</option>
                  <option value={BARCODE_HEIGHT.MEDIUM}>{t('barcode.heightMedium')}</option>
                  <option value={BARCODE_HEIGHT.LARGE}>{t('barcode.heightLarge')}</option>
                </select>
              </div>
              <div>
                <label class="block text-[11px] text-muted-foreground mb-1">{t('barcode.font')}</label>
                <select
                  value={fontSize()}
                  onChange={(e) => setFontSize(e.currentTarget.value as FontSize)}
                  class="w-full h-8 text-xs px-2 border rounded-md bg-background"
                >
                  <option value={FONT_SIZE.SMALL}>{t('barcode.fontSmall')}</option>
                  <option value={FONT_SIZE.MEDIUM}>{t('barcode.fontMedium')}</option>
                  <option value={FONT_SIZE.LARGE}>{t('barcode.fontLarge')}</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        {/* Section 2: Quantity Mode & Offset */}
        <div class="rounded-lg border bg-card p-4 space-y-3">
          <div class="font-semibold text-foreground text-xs uppercase tracking-wider">
            {t('barcode.qtyModeOffset')}
          </div>

          <div class="space-y-3">
            <div>
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setQtyMode(QTY_MODE.MULTIPLY)}
                  class={`h-8 rounded-md border text-xs font-medium transition-colors flex items-center justify-center text-center px-1 truncate ${
                    qtyMode() === QTY_MODE.MULTIPLY
                      ? 'bg-primary text-primary-foreground border-primary shadow-sm'
                      : 'bg-background hover:bg-muted text-foreground'
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
                      ? 'bg-primary text-primary-foreground border-primary shadow-sm'
                      : 'bg-background hover:bg-muted text-foreground'
                  }`}
                  title={t('barcode.singleTooltip')}
                >
                  {t('barcode.singleBtn')}
                </button>
              </div>
            </div>

            <div>
              <label class="block text-[11px] text-muted-foreground mb-1">
                {t('barcode.skipUsedSlots')}
              </label>
              <input
                type="number"
                min={0}
                max={50}
                value={skipCount()}
                onInput={(e) => setSkipCount(parseInt(e.currentTarget.value, 10) || 0)}
                class="w-full h-8 px-2.5 text-xs border rounded-md bg-background font-mono"
              />
            </div>
          </div>
        </div>

        {/* Section 3: Visual Layout Designer */}
        <div class="rounded-lg border bg-card p-4 space-y-3 flex flex-col justify-between">
          <div class="font-semibold text-foreground text-xs uppercase tracking-wider">
            {t('barcode.layoutDesigner')}
          </div>

          <div class="pt-2">
            <LabelLayoutDesigner />
          </div>
        </div>

        {/* Section 4: Canvas Settings */}
        <div class="rounded-lg border bg-card p-4 space-y-3">
          <div class="font-semibold text-foreground text-xs uppercase tracking-wider">
            {t('barcode.canvasSettings')}
          </div>

          <div class="space-y-3">
            <label class="flex items-center gap-2 cursor-pointer text-xs">
              <input
                type="checkbox"
                checked={showScreenBorder()}
                onChange={(e) => setShowScreenBorder(e.currentTarget.checked)}
                class="rounded border-border text-primary"
              />
              <span>{t('barcode.screenDashedGrid')}</span>
            </label>

            <div class="space-y-1.5 pt-1">
              <div class="flex items-center justify-between text-[11px] text-muted-foreground">
                <span>{t('barcode.zoomScale')}</span>
                <span class="font-mono font-medium">{Math.round(zoom() * 100)}%</span>
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
    </div>
  );
};
