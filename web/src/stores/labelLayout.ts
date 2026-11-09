import { createSignal } from 'solid-js';
import type { LabelElement, LabelElementType, LabelLayout, PageDesign } from '~/types';

export function createDefaultLayout(lblW: number, lblH: number): LabelLayout {
  const pad = 1.5;
  const aw = Math.max(1, lblW - pad * 2);
  const x = Math.min(pad, Math.max(0, lblW / 2));
  const footerY = Math.max(0, lblH - pad - 2.5);
  return [
    { id: 'company-1', type: 'company', x, y: pad, w: aw, h: 2.5, align: 'center', fontScale: 0.9, visible: false },
    { id: 'itemName-1', type: 'itemName', x, y: pad, w: aw, h: 3.5, align: 'center', fontScale: 1, visible: true },
    { id: 'barcode-1', type: 'barcode', x, y: 5, w: aw, h: 7, align: 'center', fontScale: 1, visible: true },
    { id: 'barcodeText-1', type: 'barcodeText', x, y: 12, w: aw, h: 2.5, align: 'center', fontScale: 0.95, visible: true },
    { id: 'sep-1', type: 'separator', x, y: Math.max(0, footerY - 0.5), w: aw, h: 0.3, align: 'center', fontScale: 1, visible: true },
    { id: 'itemCode-1', type: 'itemCode', x, y: footerY, w: aw * 0.4, h: 2.5, align: 'left', fontScale: 0.85, visible: true },
    { id: 'price-1', type: 'price', x: x + aw * 0.4, y: footerY, w: aw * 0.4, h: 2.5, align: 'right', fontScale: 0.85, visible: true },
    { id: 'unit-1', type: 'unit', x: x + aw * 0.8, y: footerY, w: aw * 0.2, h: 2.5, align: 'right', fontScale: 0.85, visible: true },
  ];
}

export const [labelLayout, setLabelLayout] = createSignal<LabelLayout>(createDefaultLayout(38, 19));
export const [companyName, setCompanyName] = createSignal('WAREHOUSE');

export function updateElement(id: string, patch: Partial<LabelElement>): void {
  setLabelLayout((layout) => layout.map((element) => element.id === id ? { ...element, ...patch } : element));
}

export function addElement(type: 'separator' | 'customText'): void {
  const layout = labelLayout();
  const id = `${type}-${Date.now()}`;
  setLabelLayout([...layout, {
    id,
    type,
    x: 2,
    y: 2,
    w: 20,
    h: type === 'separator' ? 0.5 : 3,
    align: 'center',
    fontScale: 1,
    visible: true,
    ...(type === 'customText' ? { customText: 'Text' } : {}),
  }]);
}

export function removeElement(id: string): void {
  setLabelLayout((layout) => layout.filter((element) =>
    element.id !== id || (element.type !== 'separator' && element.type !== 'customText')
  ));
}

export function resetLayout(lblW: number, lblH: number): void {
  setLabelLayout(createDefaultLayout(lblW, lblH));
}

export function serializeLayout(layout: LabelLayout): string {
  return JSON.stringify(layout);
}

export function deserializeLayout(json: string | null | undefined, lblW: number, lblH: number): LabelLayout {
  if (json) {
    try {
      const parsed: unknown = JSON.parse(json);
      if (Array.isArray(parsed) && parsed.length > 0 && parsed.every(isLabelElement)) {
        return parsed;
      }
    } catch {
      // Fall through to the dimension-aware default layout for malformed stored data.
    }
  }
  return createDefaultLayout(lblW, lblH);
}

export function loadLayoutFromDesign(design: PageDesign, lblW: number, lblH: number): void {
  setLabelLayout(deserializeLayout(design.label_layout, lblW, lblH));
}

export function isElementVisible(type: LabelElementType): boolean {
  return labelLayout().some((element) => element.type === type && element.visible);
}

function isLabelElement(value: unknown): value is LabelElement {
  if (!value || typeof value !== 'object') return false;
  const element = value as Partial<LabelElement>;
  return typeof element.id === 'string'
    && typeof element.type === 'string'
    && ['company', 'itemName', 'barcode', 'barcodeText', 'itemCode', 'price', 'unit', 'separator', 'customText'].includes(element.type)
    && [element.x, element.y, element.w, element.h, element.fontScale].every((n) => typeof n === 'number' && Number.isFinite(n))
    && (element.align === 'left' || element.align === 'center' || element.align === 'right')
    && typeof element.visible === 'boolean';
}
