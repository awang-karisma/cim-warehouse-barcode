
import { createEffect, createResource, For, Show, createSignal } from 'solid-js';
import { createLabelLayoutPreset, deleteLabelLayoutPreset, getLabelLayoutPresets, updateLabelLayoutPreset } from '~/api';
import { useAuth } from '~/auth';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '~/components/ui/dialog';
import { useI18n } from '~/i18n';
import { getEffectiveConfig } from '~/stores/barcodeConfig';
import { addElement, labelLayout, removeElement, resetLayout, setLabelLayout, updateElement, serializeLayout, deserializeLayout } from '~/stores/labelLayout';
import type { LabelElement, LabelElementType, LabelLayoutPreset } from '~/types';


export function LabelLayoutDesigner() {
  const { t } = useI18n();
  const [selectedId, setSelectedId] = createSignal('itemName-1');
  const [drag, setDrag] = createSignal<{ id: string; mode: 'move' | 'resize'; x: number; y: number; el: LabelElement }>();
  const cfg = () => getEffectiveConfig();
  const pxPerMm = () => Math.min(280 / Math.max(cfg().labelWidthMM, 1), 200 / Math.max(cfg().labelHeightMM, 1));
  const selected = () => labelLayout().find((el) => el.id === selectedId());
  const auth = useAuth();
  const canCreatePreset = () => auth.hasPermission('preset:create');
  const canUpdatePreset = () => auth.hasPermission('preset:update');
  const canDeletePreset = () => auth.hasPermission('preset:delete');
  const [layoutPresets, { refetch: refetchLayoutPresets }] = createResource(() => getLabelLayoutPresets().then((result) => result.presets).catch(() => [] as LabelLayoutPreset[]));
  const [selectedPresetId, setSelectedPresetId] = createSignal('');
  const [presetName, setPresetName] = createSignal('');
  const [presetError, setPresetError] = createSignal('');
  const [isSavingPreset, setIsSavingPreset] = createSignal(false);
  let layoutPresetSelect: HTMLSelectElement | undefined;
  createEffect(() => {
    const id = selectedPresetId();
    const presets = layoutPresets();
    if (layoutPresetSelect && presets) {
      layoutPresetSelect.value = presets.some((preset) => String(preset.id) === id) ? id : '';
    }
  });
  const [confirmingDelete, setConfirmingDelete] = createSignal(false);
  const deleteLayoutPreset = async () => {
    if (!selectedPresetId()) return;
    setIsSavingPreset(true);
    setPresetError('');
    try {
      await deleteLabelLayoutPreset(Number(selectedPresetId()));
      setSelectedPresetId('');
      setPresetName('');
      setConfirmingDelete(false);
      await refetchLayoutPresets();
    } catch (error) {
      setPresetError(error instanceof Error ? error.message : t('barcode.deletePresetFailed'));
    } finally {
      setIsSavingPreset(false);
    }
  };
  let canvasRef: HTMLDivElement | undefined;
  const name = (type: LabelElementType) => {
    switch (type) {
      case 'company': return t('barcode.elemCompanyHeader');
      case 'itemName': return t('barcode.elemItemName');
      case 'barcode': return t('barcode.elemBarcodeGraphic');
      case 'barcodeText': return t('barcode.elemBarcodeText');
      case 'itemCode': return t('barcode.elemItemCode');
      case 'price': return t('barcode.elemPrice');
      case 'unit': return t('barcode.elemUnit');
      case 'separator': return t('barcode.elemSeparator');
      case 'customText': return t('barcode.elemCustomText');
    }
  };

  const beginDrag = (event: PointerEvent, el: LabelElement, mode: 'move' | 'resize') => {
    event.preventDefault();
    event.stopPropagation();
    setSelectedId(el.id);
    setDrag({ id: el.id, mode, x: event.clientX, y: event.clientY, el });
    canvasRef?.setPointerCapture(event.pointerId);
  };
  const moveDrag = (event: PointerEvent) => {
    const active = drag();
    if (!active) return;
    const scale = pxPerMm();
    const dx = (event.clientX - active.x) / scale;
    const dy = (event.clientY - active.y) / scale;
    const w = cfg().labelWidthMM;
    const h = cfg().labelHeightMM;
    if (active.mode === 'move') {
      updateElement(active.id, {
        x: Math.max(0, Math.min(w - active.el.w, active.el.x + dx)),
        y: Math.max(0, Math.min(h - active.el.h, active.el.y + dy)),
      });
    } else {
      updateElement(active.id, {
        w: Math.max(2, Math.min(w - active.el.x, active.el.w + dx)),
        h: Math.max(1, Math.min(h - active.el.y, active.el.h + dy)),
      });
    }
  };

  const numeric = (element: () => LabelElement | undefined, key: 'x' | 'y' | 'w' | 'h' | 'fontScale', min: number) => (
    <label class="space-y-0.5 text-[10px] text-muted-foreground">
      <span>{key === 'x' ? t('barcode.posX') : key === 'y' ? t('barcode.posY') : key === 'w' ? t('barcode.sizeW') : key === 'h' ? t('barcode.sizeH') : t('barcode.fontScale')}</span>
      <input type="number" min={min} step={key === 'fontScale' ? 0.05 : 0.5} value={element()?.[key] ?? 0}
        onInput={(event) => {
          const current = element();
          if (current) updateElement(current.id, { [key]: Number(event.currentTarget.value) });
        }}
        class="w-full h-7 px-1.5 border rounded bg-background text-xs text-foreground" />
    </label>
  );
  const selectLayoutPreset = (id: string) => {
    setSelectedPresetId(id);
    setPresetError('');
    const preset = (layoutPresets() ?? []).find((item) => String(item.id) === id);
    if (!preset) {
      setPresetName('');
      return;
    }
    setPresetName(preset.name);
    const nextLayout = deserializeLayout(preset.label_layout, cfg().labelWidthMM, cfg().labelHeightMM);
    setLabelLayout(nextLayout);
    if (!nextLayout.some((element) => element.id === selectedId())) setSelectedId(nextLayout[0]?.id ?? '');
  };

  const saveLayoutPreset = async (update: boolean) => {
    const name = presetName().trim();
    if (!name) {
      setPresetError(t('barcode.presetNameRequired'));
      return;
    }
    setIsSavingPreset(true);
    setPresetError('');
    try {
      const payload = { name, label_layout: serializeLayout(labelLayout()) };
      if (update && selectedPresetId()) {
        await updateLabelLayoutPreset(Number(selectedPresetId()), payload);
      } else {
        const result = await createLabelLayoutPreset(payload);
        setSelectedPresetId(String(result.id));
      }
      await refetchLayoutPresets();
    } catch (error) {
      setPresetError(error instanceof Error ? error.message : t('barcode.savePresetFailed'));
    } finally {
      setIsSavingPreset(false);
    }
  };

  return (
    <Dialog>
      <DialogTrigger class="h-8 px-3 rounded-md border bg-background hover:bg-muted text-xs font-medium">{t('barcode.layoutDesigner')}</DialogTrigger>
      <DialogContent class="w-[calc(100vw-2rem)] max-w-4xl max-h-[calc(100dvh-2rem)] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('barcode.labelLayout')}</DialogTitle>
          <DialogDescription>{t('barcode.selectElement')}</DialogDescription>
        </DialogHeader>
        <section class="space-y-2 min-w-0" aria-label={t('barcode.layoutDesigner')}>
        <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_auto] gap-2 items-end">
          <div class="space-y-1">
            <label class="block text-[10px] text-muted-foreground">{t('barcode.labelLayoutPresets')}</label>
            <select ref={layoutPresetSelect} aria-label={t('barcode.labelLayoutPresets')} value={selectedPresetId()} onChange={(event) => selectLayoutPreset(event.currentTarget.value)} class="w-full h-8 px-2 border rounded bg-background text-xs">
              <option value="">{t('barcode.chooseLayoutPreset')}</option>
              <For each={layoutPresets() ?? []}>{(preset) => <option value={String(preset.id)}>{preset.name}</option>}</For>
            </select>
          </div>
          <div class="flex flex-wrap justify-end gap-1.5">
            <Show when={auth.hasAnyPermission('preset:create', 'preset:update', 'preset:delete')}>
              <Show when={auth.hasAnyPermission('preset:create', 'preset:update')}>
                <input value={presetName()} onInput={(event) => setPresetName(event.currentTarget.value)} placeholder={t('barcode.layoutPresetName')} class="h-8 w-36 px-2 border rounded bg-background text-xs" />
              </Show>
              <Show when={canCreatePreset()}>
                <button type="button" disabled={isSavingPreset()} class="h-8 px-2 border rounded bg-background hover:bg-muted text-xs disabled:opacity-50" onClick={() => void saveLayoutPreset(false)}>{t('barcode.saveLayoutPreset')}</button>
              </Show>
              <Show when={canUpdatePreset()}>
                <button type="button" disabled={!selectedPresetId() || isSavingPreset()} class="h-8 px-2 border rounded bg-background hover:bg-muted text-xs disabled:opacity-50" onClick={() => void saveLayoutPreset(true)}>{t('barcode.updateLayoutPreset')}</button>
              </Show>
              <Show when={canDeletePreset()}>
                <button type="button" disabled={!selectedPresetId() || isSavingPreset()} class="h-8 px-2 border rounded text-destructive hover:bg-destructive/10 text-xs disabled:opacity-50" onClick={() => setConfirmingDelete(true)}>{t('common.delete')}</button>
              </Show>
            </Show>
            <select aria-label={t('barcode.addElement')} class="h-8 px-2 border rounded bg-background text-xs"
              value="" onChange={(event) => {
                const type = event.currentTarget.value;
                if (type === 'separator' || type === 'customText') {
                  addElement(type);
                  const last = labelLayout()[labelLayout().length - 1];
                  if (last) setSelectedId(last.id);
                }
                event.currentTarget.value = '';
              }}>
              <option value="" disabled>{t('barcode.addElement')}</option>
              <option value="separator">{t('barcode.addSeparator')}</option>
              <option value="customText">{t('barcode.addCustomText')}</option>
            </select>
            <button type="button" class="h-8 px-2 border rounded bg-background hover:bg-muted text-xs" onClick={() => resetLayout(cfg().labelWidthMM, cfg().labelHeightMM)}>
              {t('barcode.layoutReset')}
            </button>
          </div>
        </div>
        <Show when={confirmingDelete()}><div role="alert" class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/5 px-2 py-1.5 text-xs">
          <span>{t('barcode.deleteLayoutPresetConfirm', { name: presetName() })}</span>
          <div class="flex gap-1.5">
            <button type="button" disabled={isSavingPreset()} class="h-7 px-2 rounded bg-destructive text-destructive-foreground" onClick={() => void deleteLayoutPreset()}>{t('common.delete')}</button>
            <button type="button" class="h-7 px-2 rounded border bg-background" onClick={() => setConfirmingDelete(false)}>{t('common.cancel')}</button>
          </div>
        </div></Show>
        <Show when={presetError()}><p role="alert" class="text-xs text-destructive">{presetError()}</p></Show>
      <div class="flex justify-center overflow-auto rounded-md border bg-muted/20 p-3">
        <div ref={canvasRef} class="relative shrink-0 border border-border bg-white shadow-sm touch-none" style={{ width: `${cfg().labelWidthMM * pxPerMm()}px`, height: `${cfg().labelHeightMM * pxPerMm()}px` }}
          onPointerMove={moveDrag} onPointerUp={() => setDrag(undefined)} onPointerCancel={() => setDrag(undefined)}>
          <For each={labelLayout()}>{(el) => {
            const active = () => selectedId() === el.id;
            return <div role="button" tabindex="0" aria-label={name(el.type)} aria-pressed={active()} onClick={() => setSelectedId(el.id)}
              onPointerDown={(event) => beginDrag(event, el, 'move')}
              class={`absolute overflow-hidden border text-center text-[9px] leading-tight cursor-move select-none ${active() ? 'border-primary bg-primary/10 text-foreground z-10' : 'border-border bg-muted/70 text-muted-foreground'}`}
              style={{ left: `${el.x * pxPerMm()}px`, top: `${el.y * pxPerMm()}px`, width: `${el.w * pxPerMm()}px`, height: `${el.h * pxPerMm()}px`, 'text-align': el.align, opacity: el.visible ? 1 : 0.35 }}>
              <span class="block truncate px-0.5">{name(el.type)}</span>
              <Show when={active()}><button type="button" aria-label={t('barcode.resizeElement')} onPointerDown={(event) => beginDrag(event, el, 'resize')}
                class="absolute bottom-0 right-0 size-2.5 bg-primary border border-white cursor-nwse-resize" /></Show>
            </div>;
          }}</For>
        </div>
      </div>
      <p class="text-[10px] text-muted-foreground">{t('barcode.selectElement')}</p>
      <Show when={selectedId()}><div class="grid grid-cols-2 sm:grid-cols-4 gap-1.5 rounded-md border p-2">
        {numeric(selected, 'x', 0)}{numeric(selected, 'y', 0)}{numeric(selected, 'w', 1)}{numeric(selected, 'h', 1)}
        <Show when={selected() && selected()!.type !== 'barcode' && selected()!.type !== 'separator'}>{numeric(selected, 'fontScale', 0.5)}</Show>
        <label class="flex items-center gap-1.5 text-xs"><input type="checkbox" checked={selected()?.visible ?? false} onChange={(event) => { const el = selected(); if (el) updateElement(el.id, { visible: event.currentTarget.checked }); }} />{t('barcode.visible')}</label>
        <div class="col-span-2 flex gap-1">
          <For each={['left', 'center', 'right'] as const}>{(align) => <button type="button" onClick={() => { const el = selected(); if (el) updateElement(el.id, { align }); }} aria-pressed={selected()?.align === align}
            class={`h-7 flex-1 rounded border text-xs ${selected()?.align === align ? 'bg-primary text-primary-foreground' : 'bg-background'}`}>{align === 'left' ? t('barcode.alignLeft') : align === 'center' ? t('barcode.alignCenter') : t('barcode.alignRight')}</button>}</For>
        </div>
        <Show when={selected()?.type === 'customText' || selected()?.type === 'company'}><label class="col-span-2 space-y-0.5 text-[10px] text-muted-foreground"><span>{t('barcode.customText')}</span><input class="w-full h-7 px-1.5 border rounded bg-background text-xs text-foreground" value={selected()?.customText ?? ''} onInput={(event) => { const el = selected(); if (el) updateElement(el.id, { customText: event.currentTarget.value }); }} /></label></Show>
      </div></Show>
      <div class="max-h-28 overflow-y-auto divide-y rounded-md border">
        <For each={labelLayout()}>{(el) => <div class={`flex items-center gap-2 px-2 py-1 text-xs ${selectedId() === el.id ? 'bg-muted' : ''}`}>
          <button type="button" class="flex-1 min-w-0 truncate text-left" onClick={() => setSelectedId(el.id)}>{name(el.type)}</button>
          <label class="flex items-center" title={t('barcode.visible')}><input type="checkbox" checked={el.visible} onChange={(event) => updateElement(el.id, { visible: event.currentTarget.checked })} /></label>
          <Show when={el.type === 'separator' || el.type === 'customText'}><button type="button" class="text-destructive" aria-label={t('barcode.removeElement')} onClick={() => { removeElement(el.id); if (selectedId() === el.id) setSelectedId('itemName-1'); }}>×</button></Show>
        </div>}</For>
      </div>
    </section>
      </DialogContent>
    </Dialog>
  );
}
