import { createSignal } from 'solid-js';
import type { AccurateNormalizedItem, BarcodeItem } from '~/types';

export const [items, setItems] = createSignal<BarcodeItem[]>([]);

export function addItem(partial?: Partial<BarcodeItem>): void {
  const current = items();
  const nextNum = current.length + 1;
  const newItem: BarcodeItem = {
    id: `item-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
    selected: true,
    itemNo: partial?.itemNo || `BRG-${String(nextNum).padStart(3, '0')}`,
    itemName: partial?.itemName || `New Item ${nextNum}`,
    barcode: partial?.barcode || `899${String(Date.now()).slice(-9)}`,
    quantity: partial?.quantity ?? 1,
    unit: partial?.unit || 'Pcs',
    price: partial?.price ?? 10000,
    documentType: partial?.documentType,
    documentNumber: partial?.documentNumber,
  };
  setItems([...current, newItem]);
}

export function addNormalizedItems(normItems: AccurateNormalizedItem[]): void {
  const current = items();
  const added: BarcodeItem[] = normItems.map((it, idx) => ({
    id: `item-${Date.now()}-${idx}-${Math.random().toString(36).substring(2, 6)}`,
    selected: true,
    itemNo: it.itemNo,
    itemName: it.itemName,
    barcode: it.upcNo || it.itemNo,
    quantity: it.quantity > 0 ? it.quantity : 1,
    unit: it.unitName || 'Pcs',
    price: it.unitPrice ?? 0,
    documentType: it.documentType,
    documentNumber: it.documentNumber,
  }));
  setItems([...current, ...added]);
}

export function updateItemField<K extends keyof BarcodeItem>(id: string, field: K, value: BarcodeItem[K]): void {
  setItems((prev) =>
    prev.map((it) => (it.id === id ? { ...it, [field]: value } : it))
  );
}

export function deleteItem(id: string): void {
  setItems((prev) => prev.filter((it) => it.id !== id));
}

export function clearItems(): void {
  setItems([]);
}

export function toggleSelectAll(checked: boolean): void {
  setItems((prev) => prev.map((it) => ({ ...it, selected: checked })));
}
