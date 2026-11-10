import { createSignal } from 'solid-js';
import { createPageDesign, deletePageDesign, updatePageDesign } from '~/api';
import { useI18n } from '~/i18n';
import {
  applyPageDesign,
  captureBarcodePresetSettings,
  customConfig,
  fromPresetConfig,
  getEffectiveConfig,
  PRESETS,
  presetKey,
  refetchDesigns,
  selectPreset,
  serverDesigns,
  SERVER_PRESET_PREFIX,
  setCustomConfig,
  setPresetDraftActive,
  setPresetKey,
} from '~/stores/barcodeConfig';
import { labelLayout, serializeLayout } from '~/stores/labelLayout';
import type { PageDesign } from '~/types';

export interface StatusMessage {
  type: 'success' | 'error';
  text: string;
  retry?: boolean;
}

export const [isEditing, setIsEditing] = createSignal(false);
export const [isCreating, setIsCreating] = createSignal(false);
export const [isSaving, setIsSaving] = createSignal(false);
export const [statusMessage, setStatusMessage] = createSignal<StatusMessage | null>(null);
export const [designToDelete, setDesignToDelete] = createSignal<PageDesign | null>(null);
export const [isDeletingDesign, setIsDeletingDesign] = createSignal(false);

let previousPresetKey = 'tj-108';

export const clearStatusMessage = () => {
  setStatusMessage(null);
};

export const startCreating = () => {
  previousPresetKey = presetKey();
  setPresetDraftActive(true);
  setIsEditing(false);
  setIsCreating(true);
  setPresetKey('custom');
  setCustomConfig({ ...getEffectiveConfig(), name: '' });
  setStatusMessage(null);
};

export const cancelCreating = () => {
  setIsCreating(false);
  setPresetDraftActive(false);
  const fallback =
    previousPresetKey.startsWith(SERVER_PRESET_PREFIX) ||
    PRESETS[previousPresetKey as keyof typeof PRESETS]
      ? previousPresetKey
      : 'tj-108';
  selectPreset(fallback);
  setStatusMessage(null);
};

export const startEditing = () => {
  setIsCreating(false);
  setIsEditing(true);
  setPresetDraftActive(true);
  const pk = presetKey();
  if (pk.startsWith(SERVER_PRESET_PREFIX)) {
    const id = parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
    const design = (serverDesigns() ?? []).find((d) => d.id === id);
    if (design) applyPageDesign(design);
  }
  setStatusMessage(null);
};

export const cancelEditing = () => {
  setIsEditing(false);
  setPresetDraftActive(false);
  selectPreset(presetKey());
  setStatusMessage(null);
};

export const retrySavedPresetRefresh = async () => {
  const { t } = useI18n();
  const id = Number.parseInt(presetKey().slice(SERVER_PRESET_PREFIX.length), 10);
  try {
    const designs = await refetchDesigns();
    const design = (designs ?? []).find((entry) => entry.id === id);
    if (!design) throw new Error('Saved preset was not returned by the refreshed list');
    applyPageDesign(design);
    setPresetKey('custom');
    setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
    setIsCreating(false);
    setIsEditing(false);
    setPresetDraftActive(false);
    setStatusMessage({ type: 'success', text: t('barcode.changesSaved') });
  } catch {
    setStatusMessage({ type: 'error', text: t('barcode.presetRefreshFailed'), retry: true });
  }
};

const refreshAfterPresetSave = async (id: number, successText: string) => {
  const { t } = useI18n();
  setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
  setPresetDraftActive(true);
  try {
    const designs = await refetchDesigns();
    const design = (designs ?? []).find((entry) => entry.id === id);
    if (!design) throw new Error('Saved preset was not returned by the refreshed list');
    applyPageDesign(design);
    setPresetKey('custom');
    setPresetKey(`${SERVER_PRESET_PREFIX}${id}`);
    setIsCreating(false);
    setIsEditing(false);
    setPresetDraftActive(false);
    setStatusMessage({ type: 'success', text: successText });
  } catch {
    setStatusMessage({ type: 'error', text: t('barcode.presetRefreshFailed'), retry: true });
  }
};

export const handleSaveNewDesign = async () => {
  const { t } = useI18n();
  const cfg = customConfig();
  const name = cfg.name.trim();
  if (!name || name === PRESETS.custom.name) {
    setStatusMessage({ type: 'error', text: t('barcode.enterPresetName') });
    return;
  }
  setIsSaving(true);
  setStatusMessage(null);
  try {
    const saved = await createPageDesign(
      fromPresetConfig(cfg, serializeLayout(labelLayout()), captureBarcodePresetSettings())
    );
    await refreshAfterPresetSave(saved.id, t('barcode.presetSaved', { name }));
  } catch (err: unknown) {
    setStatusMessage({
      type: 'error',
      text: err instanceof Error ? err.message : t('barcode.savePresetFailed'),
    });
  } finally {
    setIsSaving(false);
  }
};

export const handleUpdateDesign = async () => {
  const { t } = useI18n();
  const pk = presetKey();
  if (!pk.startsWith(SERVER_PRESET_PREFIX)) return;
  const id = Number.parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
  const cfg = customConfig();
  const name = cfg.name.trim();
  if (!name) {
    setStatusMessage({ type: 'error', text: t('barcode.presetNameRequired') });
    return;
  }
  setIsSaving(true);
  setStatusMessage(null);
  try {
    await updatePageDesign(
      id,
      fromPresetConfig(cfg, serializeLayout(labelLayout()), captureBarcodePresetSettings())
    );
    await refreshAfterPresetSave(id, t('barcode.changesSaved'));
  } catch (err: unknown) {
    setStatusMessage({
      type: 'error',
      text: err instanceof Error ? err.message : t('barcode.updatePresetFailed'),
    });
  } finally {
    setIsSaving(false);
  }
};

export const handleSaveAsCopy = async () => {
  const { t } = useI18n();
  const cfg = customConfig();
  const baseName = cfg.name.trim() || t('barcode.customLabel');
  const name =
    baseName.includes('Copy') || baseName.includes('Salinan')
      ? `${baseName} 2`
      : `${baseName}${t('barcode.copySuffix')}`;
  setIsSaving(true);
  setStatusMessage(null);
  try {
    const copyCfg = { ...cfg, name };
    const saved = await createPageDesign(
      fromPresetConfig(copyCfg, serializeLayout(labelLayout()), captureBarcodePresetSettings())
    );
    await refreshAfterPresetSave(saved.id, t('barcode.copyCreated', { name }));
  } catch (err: unknown) {
    setStatusMessage({
      type: 'error',
      text: err instanceof Error ? err.message : t('barcode.createCopyFailed'),
    });
  } finally {
    setIsSaving(false);
  }
};

export const handleDeleteCurrentDesign = () => {
  const pk = presetKey();
  if (!pk.startsWith(SERVER_PRESET_PREFIX)) return;
  const id = parseInt(pk.slice(SERVER_PRESET_PREFIX.length), 10);
  const design = (serverDesigns() ?? []).find((d) => d.id === id);
  if (design) {
    setDesignToDelete(design);
  }
};

export const handleConfirmDelete = async () => {
  const target = designToDelete();
  if (!target) return;
  setIsDeletingDesign(true);
  try {
    await deletePageDesign(target.id);
    await refetchDesigns();
    setIsEditing(false);
    setIsCreating(false);
    if (presetKey() === `${SERVER_PRESET_PREFIX}${target.id}`) {
      selectPreset('tj-108');
    }
    setDesignToDelete(null);
    setStatusMessage({ type: 'success', text: `Deleted "${target.name}"` });
  } catch (err: unknown) {
    setStatusMessage({
      type: 'error',
      text: err instanceof Error ? err.message : 'Failed to delete',
    });
  } finally {
    setIsDeletingDesign(false);
  }
};
