import { type Component, For, Show } from 'solid-js';
import { useAuth } from '~/auth';
import { Button } from '~/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '~/components/ui/card';
import { useI18n } from '~/i18n';
import {
  customConfig,
  getEffectiveConfig,
  PRESETS,
  presetKey,
  refetchDesigns,
  selectPreset,
  serverDesigns,
  SERVER_PRESET_PREFIX,
  setCustomConfig,
  setPresetDraftActive,
} from '~/stores/barcodeConfig';
import {
  cancelCreating,
  cancelEditing,
  handleDeleteCurrentDesign,
  handleSaveAsCopy,
  handleSaveNewDesign,
  handleUpdateDesign,
  isCreating,
  isEditing,
  isSaving,
  setStatusMessage,
  startCreating,
  startEditing,
} from '~/stores/presetDraft';
import {
  IconCopy,
  IconDeviceFloppy,
  IconEdit,
  IconPlus,
  IconTrash,
  IconX,
} from '@tabler/icons-solidjs';

export const PresetsTab: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();

  const canCreateDesign = () => auth.hasPermission('design:create');
  const canUpdateDesign = () => auth.hasPermission('design:update');
  const canDeleteDesign = () => auth.hasPermission('design:delete');

  const selectedPaper = () => {
    const p = customConfig().paper;
    if (p === 'A4' || p === 'A5' || p === 'Letter' || p === 'Roll') {
      return p;
    }
    return 'Custom';
  };

  const isCustomPaper = () => selectedPaper() === 'Custom' || selectedPaper() === 'Roll';

  const handlePaperChange = (newPaper: string) => {
    const current = customConfig();
    if (newPaper === 'A4') {
      setCustomConfig({
        ...current,
        paper: 'A4',
        paperWidthMM: 210,
        paperHeightMM: 297,
      });
    } else if (newPaper === 'A5') {
      setCustomConfig({
        ...current,
        paper: 'A5',
        paperWidthMM: 148,
        paperHeightMM: 210,
      });
    } else if (newPaper === 'Letter') {
      setCustomConfig({
        ...current,
        paper: 'Letter',
        paperWidthMM: 216,
        paperHeightMM: 279,
      });
    } else if (newPaper === 'Roll') {
      setCustomConfig({
        ...current,
        paper: 'Roll',
        paperWidthMM: current.labelWidthMM + current.marginLeftMM * 2,
        paperHeightMM: current.labelHeightMM + current.marginTopMM * 2,
        cols: 1,
        rows: 1,
      });
    } else {
      setCustomConfig({
        ...current,
        paper: 'Custom',
      });
    }
  };

  return (
    <Card class="no-print">
      <CardHeader class="border-b pb-4">
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
          <div>
            <CardTitle class="text-base font-semibold">{t('barcode.presetsTabTitle')}</CardTitle>
            <CardDescription>{t('barcode.presetsTabDesc')}</CardDescription>
          </div>
          <div class="text-xs text-muted-foreground font-medium">
            {t('barcode.stickerPaperPreset')}: <span class="text-foreground font-semibold">{getEffectiveConfig().name}</span>
          </div>
        </div>
      </CardHeader>

      <CardContent class="p-4 md:p-6 space-y-6 max-w-xl">
        {/* Section 1: Paper & Preset Selection */}
        <div class="space-y-3">
          <label class="block font-semibold text-muted-foreground text-xs uppercase tracking-wider">
            {t('barcode.stickerPaperPreset')}
          </label>
          <p class="text-xs leading-snug text-muted-foreground">{t('barcode.presetContentsHelp')}</p>
          <select
            value={presetKey()}
            onChange={(e) => {
              const nextVal = e.currentTarget.value;
              if (!nextVal || nextVal === 'custom') return;
              cancelEditing();
              cancelCreating();
              setPresetDraftActive(false);
              setStatusMessage(null);
              selectPreset(nextVal);
            }}
            class="w-full h-9 text-xs px-2.5 border rounded-md bg-background font-medium"
          >
            <option value="custom" disabled class="hidden">
              {t('barcode.newPresetUnsaved')}
            </option>
            <optgroup label={t('barcode.builtinPresets')}>
              <For each={Object.entries(PRESETS).filter(([k]) => k !== 'custom')}>
                {([k, p]) => <option value={k}>{p.name}</option>}
              </For>
            </optgroup>
            <Show when={(serverDesigns() ?? []).length > 0}>
              <optgroup label={t('barcode.customPresets')}>
                <For each={serverDesigns() ?? []}>
                  {(d) => <option value={`${SERVER_PRESET_PREFIX}${d.id}`}>{d.name}</option>}
                </For>
              </optgroup>
            </Show>
          </select>

          <Show when={serverDesigns.error}>
            <div class="flex items-center justify-between gap-2 rounded border border-destructive/30 bg-destructive/10 px-3 py-1.5 text-xs text-destructive">
              <span>{t('barcode.presetRefreshFailed')}</span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-6 px-2 text-xs"
                onClick={() => refetchDesigns()}
              >
                {t('common.retry')}
              </Button>
            </div>
          </Show>

          {/* Action buttons under dropdown */}
          <Show when={auth.hasAnyPermission('design:create', 'design:update', 'design:delete')}>
            <div class="pt-1">
              <div class="grid grid-cols-3 gap-2">
                <Show
                  when={canUpdateDesign()}
                  fallback={<div />}
                >
                  <Button
                    type="button"
                    variant={isEditing() ? 'secondary' : 'outline'}
                    size="sm"
                    class="w-full h-8 text-xs gap-1 px-1"
                    disabled={!presetKey().startsWith(SERVER_PRESET_PREFIX) || isCreating()}
                    onClick={() => {
                      if (isEditing()) cancelEditing();
                      else startEditing();
                    }}
                    title={
                      !presetKey().startsWith(SERVER_PRESET_PREFIX)
                        ? t('barcode.builtinCannotEdit')
                        : isEditing()
                        ? t('barcode.closePresetEditor')
                        : t('barcode.editCustomPreset')
                    }
                  >
                    <Show when={isEditing()} fallback={<IconEdit class="size-3.5" />}>
                      <IconX class="size-3.5" />
                    </Show>
                    <span>{isEditing() ? t('common.cancel') : t('barcode.editPreset')}</span>
                  </Button>
                </Show>

                <Show
                  when={canDeleteDesign()}
                  fallback={<div />}
                >
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    class="w-full h-8 text-xs text-destructive hover:text-destructive hover:bg-destructive/10 disabled:text-muted-foreground disabled:hover:bg-transparent disabled:opacity-40 gap-1 px-1"
                    disabled={!presetKey().startsWith(SERVER_PRESET_PREFIX) || isCreating()}
                    onClick={handleDeleteCurrentDesign}
                    title={
                      !presetKey().startsWith(SERVER_PRESET_PREFIX)
                        ? t('barcode.builtinCannotDelete')
                        : t('barcode.deleteCustomPreset')
                    }
                  >
                    <IconTrash class="size-3.5" />
                    <span>{t('common.delete')}</span>
                  </Button>
                </Show>

                <Show
                  when={canCreateDesign()}
                  fallback={<div />}
                >
                  <Button
                    type="button"
                    variant={isCreating() ? 'secondary' : 'outline'}
                    size="sm"
                    class="w-full h-8 text-xs gap-1 px-1"
                    onClick={() => {
                      if (isCreating()) cancelCreating();
                      else startCreating();
                    }}
                    title={isCreating() ? t('barcode.cancelNewPreset') : t('barcode.createNewPreset')}
                  >
                    <Show when={isCreating()} fallback={<IconPlus class="size-3.5" />}>
                      <IconX class="size-3.5" />
                    </Show>
                    <span>{isCreating() ? t('common.cancel') : t('barcode.newPreset')}</span>
                  </Button>
                </Show>
              </div>
            </div>
          </Show>
        </div>

        {/* Create / Edit Form */}
        <Show when={isCreating() || isEditing()}>
          <div class="space-y-4 pt-4 border-t border-border">
            {/* Header with Title */}
            <div class="flex items-center justify-between gap-1">
              <span class="font-semibold text-muted-foreground uppercase tracking-wider text-xs">
                {t('barcode.customDimensions')}
              </span>
            </div>

            {/* Design Name */}
            <Show when={auth.hasAnyPermission('design:create', 'design:update') || presetKey() === 'custom'}>
              <div>
                <label class="block text-muted-foreground text-xs mb-1">{t('barcode.designName')}</label>
                <input
                  type="text"
                  value={customConfig().name}
                  onInput={(e) => setCustomConfig({ ...customConfig(), name: e.currentTarget.value })}
                  placeholder={t('barcode.designNamePlaceholder')}
                  class="w-full px-3 py-1.5 border rounded-md bg-background text-xs"
                />
              </div>
            </Show>

            {/* Label Dimensions */}
            <div>
              <div class="text-muted-foreground text-xs font-medium mb-1.5">{t('barcode.labelSize')}</div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.width')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="1"
                    value={customConfig().labelWidthMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        labelWidthMM: parseFloat(e.currentTarget.value) || 1,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.height')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="1"
                    value={customConfig().labelHeightMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        labelHeightMM: parseFloat(e.currentTarget.value) || 1,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
              </div>
            </div>

            {/* Grid: Columns & Rows */}
            <div>
              <div class="text-muted-foreground text-xs font-medium mb-1.5">{t('barcode.gridLayout')}</div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.columns')}</label>
                  <input
                    type="number"
                    min="1"
                    max="20"
                    value={customConfig().cols}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        cols: parseInt(e.currentTarget.value, 10) || 1,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.rows')}</label>
                  <input
                    type="number"
                    min="1"
                    max="50"
                    value={customConfig().rows}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        rows: parseInt(e.currentTarget.value, 10) || 1,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
              </div>
            </div>

            {/* Gaps: Horizontal & Vertical */}
            <div>
              <div class="text-muted-foreground text-xs font-medium mb-1.5">{t('barcode.gaps')}</div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.gapX')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="0"
                    value={customConfig().gapXMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        gapXMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.gapY')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="0"
                    value={customConfig().gapYMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        gapYMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
              </div>
            </div>

            {/* Margins: Top & Left */}
            <div>
              <div class="text-muted-foreground text-xs font-medium mb-1.5">{t('barcode.margins')}</div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.marginTop')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="0"
                    value={customConfig().marginTopMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        marginTopMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.marginLeft')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="0"
                    value={customConfig().marginLeftMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        marginLeftMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2.5 py-1.5 border rounded-md bg-background text-xs"
                  />
                </div>
              </div>
            </div>

            {/* Paper Sheet */}
            <div>
              <div class="text-muted-foreground text-xs font-medium mb-1.5">{t('barcode.paperSheet')}</div>
              <div class="grid grid-cols-3 gap-2">
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.paper')}</label>
                  <select
                    value={selectedPaper()}
                    onChange={(e) => handlePaperChange(e.currentTarget.value)}
                    class="w-full px-2 py-1.5 border rounded-md bg-background text-xs"
                  >
                    <option value="A4">A4</option>
                    <option value="A5">A5</option>
                    <option value="Letter">Letter</option>
                    <option value="Custom">Custom</option>
                    <option value="Roll">{t('barcode.paperRoll')}</option>
                  </select>
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.width')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="1"
                    disabled={!isCustomPaper()}
                    value={customConfig().paperWidthMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        paperWidthMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2 py-1.5 border rounded-md bg-background text-xs disabled:opacity-50 disabled:bg-muted/50 disabled:cursor-not-allowed"
                  />
                </div>
                <div>
                  <label class="block text-muted-foreground text-[10px] mb-0.5">{t('barcode.height')}</label>
                  <input
                    type="number"
                    step="0.5"
                    min="1"
                    disabled={!isCustomPaper()}
                    value={customConfig().paperHeightMM}
                    onInput={(e) =>
                      setCustomConfig({
                        ...customConfig(),
                        paperHeightMM: parseFloat(e.currentTarget.value) || 0,
                      })
                    }
                    class="w-full px-2 py-1.5 border rounded-md bg-background text-xs disabled:opacity-50 disabled:bg-muted/50 disabled:cursor-not-allowed"
                  />
                </div>
              </div>
            </div>

            {/* Audit Sheet Toggle */}
            <label class="flex items-center gap-2 cursor-pointer text-xs text-muted-foreground pt-1">
              <input
                type="checkbox"
                checked={customConfig().isAuditSheet}
                onChange={(e) =>
                  setCustomConfig({
                    ...customConfig(),
                    isAuditSheet: e.currentTarget.checked,
                  })
                }
                class="rounded border-border text-primary"
              />
              <span>{t('barcode.auditSheetMode')}</span>
            </label>

            {/* Bottom Action Buttons */}
            <Show when={auth.hasAnyPermission('design:create', 'design:update')}>
              <div class="pt-3 border-t border-border flex items-center justify-end gap-2">
                <Show
                  when={isCreating()}
                  fallback={
                    <>
                      <Show when={canCreateDesign()}>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          class="h-8 px-2.5 text-xs gap-1"
                          disabled={isSaving()}
                          onClick={handleSaveAsCopy}
                          title={t('barcode.saveAsCopyTitle')}
                        >
                          <IconCopy class="size-3.5" />
                          <span>{t('barcode.createCopy')}</span>
                        </Button>
                      </Show>
                      <Show when={canUpdateDesign()}>
                        <Button
                          type="button"
                          size="sm"
                          class="h-8 px-3 text-xs gap-1"
                          disabled={isSaving()}
                          onClick={handleUpdateDesign}
                          title={t('barcode.saveChangesTitle')}
                        >
                          <IconDeviceFloppy class="size-3.5" />
                          <span>{isSaving() ? t('common.saving') : t('common.save')}</span>
                        </Button>
                      </Show>
                    </>
                  }
                >
                  <Show when={canCreateDesign()}>
                    <Button
                      type="button"
                      size="sm"
                      class="h-8 px-3 text-xs gap-1"
                      disabled={isSaving()}
                      onClick={handleSaveNewDesign}
                      title={t('barcode.saveReusablePresetTitle')}
                    >
                      <IconDeviceFloppy class="size-3.5" />
                      <span>{isSaving() ? t('common.saving') : t('barcode.savePreset')}</span>
                    </Button>
                  </Show>
                </Show>
              </div>
            </Show>
          </div>
        </Show>
      </CardContent>
    </Card>
  );
};
