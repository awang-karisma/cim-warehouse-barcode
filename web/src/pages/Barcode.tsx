import { type Component } from 'solid-js';
import { useSearchParams } from '@solidjs/router';
import { AccurateLoginModal } from '~/components/barcode/AccurateLoginModal';
import { ItemSearchDrawer } from '~/components/barcode/ItemSearchDrawer';
import { TransactionDrawer } from '~/components/barcode/TransactionDrawer';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Button } from '~/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import { useI18n } from '~/i18n';
import {
  designToDelete,
  handleConfirmDelete,
  isDeletingDesign,
  setDesignToDelete,
} from '~/stores/presetDraft';
import {
  IconLayout2,
  IconPrinter,
  IconScan,
  IconTemplate,
} from '@tabler/icons-solidjs';
import { DesignerTab } from './barcode/DesignerTab';
import {
  isSearchOpen,
  isTransactionOpen,
  setIsSearchOpen,
  setIsTransactionOpen,
} from './barcode/drawerState';
import { PresetsTab } from './barcode/PresetsTab';
import { PreviewTab } from './barcode/PreviewTab';
import { QueueTab } from './barcode/QueueTab';

const VALID_TABS = ['queue', 'designer', 'presets', 'preview'] as const;
type TabKey = (typeof VALID_TABS)[number];

export const Barcode: Component = () => {
  const { t } = useI18n();
  const [search, setSearch] = useSearchParams<{ tab?: string }>();

  const activeTab = (): TabKey => {
    const tab = search.tab;
    if (tab && (VALID_TABS as readonly string[]).includes(tab)) {
      return tab as TabKey;
    }
    return 'queue';
  };

  const handleTabChange = (value: string) => {
    const nextTab = (VALID_TABS as readonly string[]).includes(value) ? value : 'queue';
    setSearch({ tab: nextTab });
  };

  return (
    <div class="barcode-print-root space-y-6">
      <Tabs value={activeTab()} onChange={handleTabChange} class="space-y-6">
        <div class="no-print">
          <TabsList class="no-print w-full grid grid-cols-2 sm:w-auto sm:inline-flex h-auto sm:h-9 p-1 gap-1 sm:gap-0">
            <TabsTrigger value="queue" class="gap-2 text-xs sm:text-sm h-8 sm:h-7 px-3">
              <IconScan class="size-4 shrink-0" />
              <span>{t('barcode.queueTab')}</span>
            </TabsTrigger>
            <TabsTrigger value="designer" class="gap-2 text-xs sm:text-sm h-8 sm:h-7 px-3">
              <IconLayout2 class="size-4 shrink-0" />
              <span>{t('barcode.designerTab')}</span>
            </TabsTrigger>
            <TabsTrigger value="presets" class="gap-2 text-xs sm:text-sm h-8 sm:h-7 px-3">
              <IconTemplate class="size-4 shrink-0" />
              <span>{t('barcode.presetsTab')}</span>
            </TabsTrigger>
            <TabsTrigger value="preview" class="gap-2 text-xs sm:text-sm h-8 sm:h-7 px-3">
              <IconPrinter class="size-4 shrink-0" />
              <span>{t('barcode.previewTab')}</span>
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="queue">
          <QueueTab />
        </TabsContent>

        <TabsContent value="designer">
          <DesignerTab onGoToPresets={() => handleTabChange('presets')} />
        </TabsContent>

        <TabsContent value="presets">
          <PresetsTab />
        </TabsContent>

        <TabsContent value="preview">
          <PreviewTab />
        </TabsContent>
      </Tabs>

      <AccurateLoginModal />
      <ItemSearchDrawer isOpen={isSearchOpen()} onClose={() => setIsSearchOpen(false)} />
      <TransactionDrawer isOpen={isTransactionOpen()} onClose={() => setIsTransactionOpen(false)} />

      {/* Delete Confirmation Alert Dialog */}
      <AlertDialog
        open={!!designToDelete()}
        onOpenChange={(open) => {
          if (!open) setDesignToDelete(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogTitle>{t('barcode.deleteDialogTitle')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('barcode.deleteDialogDesc', { name: designToDelete()?.name ?? '' })}
          </AlertDialogDescription>
          <div class="flex flex-col sm:flex-row sm:justify-end gap-2 mt-4">
            <Button
              variant="outline"
              size="sm"
              disabled={isDeletingDesign()}
              onClick={() => setDesignToDelete(null)}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              size="sm"
              disabled={isDeletingDesign()}
              onClick={handleConfirmDelete}
            >
              {isDeletingDesign() ? t('common.deleting') : t('common.delete')}
            </Button>
          </div>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
};
