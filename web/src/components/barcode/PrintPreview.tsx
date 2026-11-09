import { BARCODE_HEIGHT, BARCODE_SYMBOLOGY, FONT_SIZE, QTY_MODE } from '~/domain';

import { type Component, createEffect, For, onCleanup, Show } from 'solid-js';
import { useI18n } from '~/i18n';
import JsBarcode from 'jsbarcode';
import qrcode from 'qrcode-generator';
import { DATAMatrix } from 'datamatrix-svg-ts';
import { items } from '~/stores/barcodeItems';
import {
  barcodeHeight,
  fontSize,
  getEffectiveConfig,
  qtyMode,
  showScreenBorder,
  skipCount,
  symbology,
  zoom,
} from '~/stores/barcodeConfig';
import { companyName, labelLayout } from '~/stores/labelLayout';
import type { BarcodeItem, LabelElement, PresetConfig } from '~/types';
import { Button } from '~/components/ui/button';
import { IconPrinter } from '@tabler/icons-solidjs';

const LabelContent: Component<{
  element: LabelElement;
  item: BarcodeItem;
  barcodeVal: string;
  baseFontSize: string;
  barcodeHeight: number;
  barcodeWidth: number;
  locale: string;
}> = (props) => {
  const el = () => props.element;
  const style = () => ({
    position: 'absolute' as const,
    left: `${el().x}mm`,
    top: `${el().y}mm`,
    width: `${el().w}mm`,
    height: `${el().h}mm`,
    'text-align': el().align,
    'font-size': `calc(${props.baseFontSize} * ${el().fontScale})`,
    overflow: 'hidden',
    'box-sizing': 'border-box' as const,
  });
  return <div style={style()}>
    <Show when={el().type === 'company'}><div class="lbl-header text-gray-700">{el().customText || companyName()}</div></Show>
    <Show when={el().type === 'itemName'}><div class="lbl-name text-black">{props.item.itemName}</div></Show>
    <Show when={el().type === 'barcode'}><div class="lbl-barcode" style={{ 'justify-content': el().align === 'left' ? 'flex-start' : el().align === 'right' ? 'flex-end' : 'center', height: '100%' }}>
      <BarcodeGraphic barcode={props.barcodeVal} symbology={symbology()} height={props.barcodeHeight} width={props.barcodeWidth} />
    </div></Show>
    <Show when={el().type === 'barcodeText'}><div class="lbl-code text-black">{props.barcodeVal}</div></Show>
    <Show when={el().type === 'itemCode'}><span class="font-mono font-semibold text-gray-700">{props.item.itemNo}</span></Show>
    <Show when={el().type === 'price'}><span class="lbl-price text-black">{formatRupiah(props.item.price, props.locale)}</span></Show>
    <Show when={el().type === 'unit'}><span class="text-gray-700">/ {props.item.unit || 'Pcs'}</span></Show>
    <Show when={el().type === 'separator'}><hr style={{ margin: 0, 'border-top': '0.5px solid #cbd5e1' }} /></Show>
    <Show when={el().type === 'customText'}><span>{el().customText}</span></Show>
  </div>;
};
function formatRupiah(num: number, loc = 'id-ID'): string {
  return 'Rp ' + Number(num || 0).toLocaleString(loc);
}


function resolveSheetDimensions(cfg: PresetConfig) {
  const isAudit = cfg.isAuditSheet;
  const lblW = isAudit ? 190 : cfg.labelWidthMM;
  const lblH = isAudit ? 15 : cfg.labelHeightMM;
  const mTop = isAudit ? 10 : cfg.marginTopMM;
  const mLeft = isAudit ? 10 : cfg.marginLeftMM;

  let sheetW = cfg.paperWidthMM || 210;
  let sheetH = cfg.paperHeightMM || 297;
  if (cfg.paper === 'Roll') {
    sheetW = cfg.paperWidthMM > 0 ? cfg.paperWidthMM : lblW + mLeft * 2;
    sheetH = cfg.paperHeightMM > 0 ? cfg.paperHeightMM : lblH + mTop * 2;
  }

  return { sheetW, sheetH, lblW, lblH, mTop, mLeft, isAudit };
}
export const PrintPreview: Component = () => {
  const { t, intlLocale } = useI18n();
  const selectedItems = () => items().filter((it) => it.selected);

  // Expand list based on qtyMode (single vs multiply)
  const expandedItems = () => {
    const list: BarcodeItem[] = [];
	const isMultiply = qtyMode() === QTY_MODE.MULTIPLY;
    for (const it of selectedItems()) {
      const count = isMultiply ? Math.floor(it.quantity > 0 ? it.quantity : 1) : 1;
      for (let i = 0; i < count; i++) {
        list.push(it);
      }
    }
    return list;
  };

  const config = () => getEffectiveConfig();

  // Dynamic Print CSS update
  createEffect(() => {
    const cfg = config();
    const { sheetW, sheetH } = resolveSheetDimensions(cfg);

    let styleEl = document.getElementById('dynamic-print-css') as HTMLStyleElement | null;
    if (!styleEl) {
      styleEl = document.createElement('style');
      styleEl.id = 'dynamic-print-css';
      document.head.appendChild(styleEl);
    }

    styleEl.innerHTML = `
      @page {
        size: ${sheetW}mm ${sheetH}mm;
        margin: 0;
      }
    `;
  });

  onCleanup(() => {
    const styleEl = document.getElementById('dynamic-print-css');
    if (styleEl) styleEl.innerHTML = '';
  });

  // Calculate layout pagination
  const layout = () => {
    const cfg = config();
    const { sheetW, sheetH, lblW, lblH, mTop, mLeft, isAudit } = resolveSheetDimensions(cfg);
    const paper = cfg.paper;
    const gapX = isAudit ? 0 : cfg.gapXMM;
    const gapY = isAudit ? 0 : cfg.gapYMM;

    let rowsPerSheet = 1;
    if (paper === 'Roll') {
      rowsPerSheet = 1;
    } else if (isAudit) {
      rowsPerSheet = 18;
    } else {
      rowsPerSheet = Math.floor((sheetH - mTop * 2 + gapY) / (lblH + gapY)) || 1;
    }

    const cols = isAudit ? 1 : cfg.cols;
    const labelsPerSheet = cols * rowsPerSheet;

    const allData = expandedItems();
    const skip = isAudit ? 0 : skipCount();
    const totalSlots = skip + allData.length;
    const totalSheets = Math.ceil(totalSlots / labelsPerSheet) || 1;

    // Group slots into sheets
    const sheets: Array<Array<{ isSkip: boolean; item?: BarcodeItem }>> = [];
    let currentSlot = 0;
    let dataIdx = 0;

    for (let s = 0; s < totalSheets; s++) {
      const sheetSlots: Array<{ isSkip: boolean; item?: BarcodeItem }> = [];
      for (let i = 0; i < labelsPerSheet; i++) {
        if (currentSlot >= totalSlots) break;
        if (currentSlot < skip) {
          sheetSlots.push({ isSkip: true });
        } else {
          sheetSlots.push({ isSkip: false, item: allData[dataIdx] });
          dataIdx++;
        }
        currentSlot++;
      }
      sheets.push(sheetSlots);
    }

    return {
      sheetW,
      sheetH,
      lblW,
      lblH,
      cols,
      rowsPerSheet,
      gapX,
      gapY,
      mTop,
      mLeft,
      sheets,
      totalSheets,
      isAudit,
    };
  };

  const fontSizeCss = () => {
    const fs = fontSize();
	return fs === FONT_SIZE.SMALL ? '0.55rem' : fs === FONT_SIZE.LARGE ? '0.75rem' : '0.65rem';
  };

  const barcodePxHeight = () => {
    const bh = barcodeHeight();
	return bh === BARCODE_HEIGHT.COMPACT ? 18 : bh === BARCODE_HEIGHT.LARGE ? 32 : 24;
  };

  return (
    <div class="flex-1 overflow-auto p-4 flex flex-col items-center preview-pane bg-muted/20">
      {/* Top Preview Info Bar */}
      <div class="no-print w-full bg-card border rounded-lg px-4 py-2.5 mb-4 flex items-center justify-between text-xs text-muted-foreground shadow-sm flex-shrink-0">
        <div class="flex items-center gap-2">
          <span class="font-bold text-foreground">{config().name}</span>
          <span>•</span>
          <span>
            {t('barcode.perSheet', { cols: layout().cols, rows: layout().rowsPerSheet })}
          </span>
          <span>•</span>
          <span class="font-semibold text-primary">{t('barcode.totalLabels', { count: expandedItems().length })}</span>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-xs text-muted-foreground">
            {layout().totalSheets} {t('barcode.pages')}
          </span>
          <Button
            size="sm"
            onClick={() => window.print()}
            disabled={expandedItems().length === 0}
            class="h-8 shadow-sm"
          >
            <IconPrinter class="size-4 mr-1.5" />
            {t('barcode.printLabels', { count: expandedItems().length })}
          </Button>
        </div>
      </div>

      <Show
        when={expandedItems().length > 0}
        fallback={
          <div class="my-auto text-center p-12 bg-card rounded-xl border shadow-sm max-w-md">
            <IconPrinter class="w-12 h-12 text-muted-foreground/40 mx-auto mb-3" />
            <p class="text-sm font-semibold text-foreground">{t('barcode.noItemsSelected')}</p>
            <p class="text-xs text-muted-foreground mt-1">
              {t('barcode.selectAtLeastOne')}
            </p>
          </div>
        }
      >
        {/* Printable Sheets Wrapper */}
        <div
          class={`sheet-wrapper ${showScreenBorder() ? 'screen-border' : ''}`}
          style={{ transform: `scale(${zoom()})` }}
        >
          <For each={layout().sheets}>
            {(sheet) => {
              const info = layout();

              // 1. Audit Sheet Template
              if (info.isAudit) {
                return (
                  <div
                    class="print-sheet audit-sheet bg-white text-black"
                    style={{
                      width: `${info.sheetW}mm`,
                      height: `${info.sheetH}mm`,
                    }}
                  >
                    <div class="mb-4 flex items-center justify-between border-b-2 border-black pb-2">
                      <div>
                        <h2 class="text-base font-bold text-black uppercase tracking-wide">
                          {t('barcode.auditSheetTitle')}
                        </h2>
                        <p class="text-[10px] text-gray-600 font-medium">
                          {t('barcode.auditSheetSubtitle')}
                        </p>
                      </div>
                      <div class="text-right text-[10px] text-gray-800">
                        <div>{t('barcode.auditDate')}</div>
                        <div>{t('barcode.auditAuditor')}</div>
                      </div>
                    </div>

                    <table class="audit-table text-black">
                      <thead>
                        <tr>
                          <th style={{ width: '5%' }}>#</th>
                          <th style={{ width: '15%' }}>{t('barcode.colCode')}</th>
                          <th style={{ width: '35%' }}>{t('barcode.colItemName')}</th>
                          <th style={{ width: '25%' }}>{t('barcode.colBarcode')}</th>
                          <th style={{ width: '10%' }}>{t('barcode.colSystem')}</th>
                          <th style={{ width: '10%' }}>{t('barcode.colActual')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        <For each={sheet}>
                          {(slot, idx) => {
                            if (slot.isSkip || !slot.item) return null;
                            const it = slot.item;
                            return (
                              <tr>
                                <td class="center font-mono">{idx() + 1}</td>
                                <td class="font-mono font-semibold">{it.itemNo}</td>
                                <td>{it.itemName}</td>
                                <td class="center">
                                  <BarcodeGraphic
                                    barcode={it.barcode || it.itemNo}
                                    symbology={symbology()}
                                    height={16}
                                    width={1.0}
                                  />
                                  <span class="block text-[8px] font-mono mt-0.5">
                                    {it.barcode || it.itemNo}
                                  </span>
                                </td>
                                <td class="right font-semibold">
                                  {it.quantity} {it.unit}
                                </td>
                                <td class="center">_______</td>
                              </tr>
                            );
                          }}
                        </For>
                      </tbody>
                    </table>
                  </div>
                );
              }

              // 2. Standard Label Grid Template
              return (
                <div
                  class="print-sheet bg-white text-black"
                  style={{
                    width: `${info.sheetW}mm`,
                    height: `${info.sheetH}mm`,
                    'padding-top': `${info.mTop}mm`,
                    'padding-left': `${info.mLeft}mm`,
                  }}
                >
                  <div
                    style={{
                      display: 'grid',
                      'grid-template-columns': `repeat(${info.cols}, ${info.lblW}mm)`,
                      'grid-auto-rows': `${info.lblH}mm`,
                      'column-gap': `${info.gapX}mm`,
                      'row-gap': `${info.gapY}mm`,
                      width: 'fit-content',
                    }}
                  >
                    <For each={sheet}>
                      {(slot) => {
                        if (slot.isSkip || !slot.item) {
                          return (
                            <div
                              class="barcode-label skip-placeholder"
                              style={{
                                width: `${info.lblW}mm`,
                                height: `${info.lblH}mm`,
                              }}
                            />
                          );
                        }

                        const it = slot.item;
                        const barcodeVal = it.barcode || it.itemNo || '12345678';

                        return (
                          <div
                            class="barcode-label bg-white text-black"
                            style={{
                              width: `${info.lblW}mm`,
                              height: `${info.lblH}mm`,
                              position: 'relative',
                              'font-size': fontSizeCss(),
                            }}
                          >
                            <For each={labelLayout().filter((el) => el.visible)}>
                              {(el) => <LabelContent
                                element={el}
                                item={it}
                                barcodeVal={barcodeVal}
                                baseFontSize={fontSizeCss()}
                                barcodeHeight={barcodePxHeight()}
                                barcodeWidth={info.lblW < 45 ? 1.0 : 1.3}
                                locale={intlLocale()}
                              />}
                            </For>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                </div>
              );
            }}
          </For>
        </div>
      </Show>
    </div>
  );
};

interface BarcodeGraphicProps {
  barcode: string;
  symbology: string;
  height: number;
  width: number;
}

const BarcodeGraphic: Component<BarcodeGraphicProps> = (props) => {
  let containerRef: HTMLDivElement | undefined;

  createEffect(() => {
    if (!containerRef) return;
    containerRef.innerHTML = '';

    const val = props.barcode || '12345678';
    const sym = props.symbology;

	if (sym === BARCODE_SYMBOLOGY.QRCODE) {
		try {
			const qr = qrcode(0, 'M');
			qr.addData(val);
			qr.make();
			containerRef.innerHTML = qr.createSvgTag({ scalable: true, margin: 0 });
			const svg = containerRef.querySelector('svg');
			if (svg) {
				svg.style.width = `${props.height * 1.2}px`;
				svg.style.height = `${props.height * 1.2}px`;
			}
		} catch {
			containerRef.innerHTML = `<span class="text-[9px] text-red-500">QR Error</span>`;
		}
	} else if (sym === BARCODE_SYMBOLOGY.DATAMATRIX) {
      try {
        const svg = DATAMatrix({ message: val, padding: 0, palette: { foreground: 'currentColor' } });
        svg.style.width = `${props.height * 1.2}px`;
        svg.style.height = `${props.height * 1.2}px`;
        containerRef.appendChild(svg);
      } catch {
        containerRef.innerHTML = `<span class="text-[9px] text-red-500">Data Matrix Error</span>`;
      }
    } else {
      const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      try {
        JsBarcode(svg, val, {
          format: sym,
          width: props.width,
          height: props.height,
          displayValue: false,
          margin: 0,
        });
        containerRef.appendChild(svg);
      } catch {
        // Fallback to CODE128
        try {
          JsBarcode(svg, val, {
					format: BARCODE_SYMBOLOGY.CODE128,
            width: 1.0,
            height: props.height,
            displayValue: false,
            margin: 0,
          });
          containerRef.appendChild(svg);
        } catch {
          containerRef.innerHTML = `<span class="text-[9px] text-red-500">Barcode Error</span>`;
        }
      }
    }
  });

  return <div ref={containerRef} class="flex items-center justify-center" />;
};
