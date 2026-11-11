import { type Component, createSignal, For, Show } from 'solid-js';
import { useI18n } from '~/i18n';
import {
  authError,
  databases,
  handleLogin,
  handleLogout,
  handleSelectDatabase,
  isAuthLoading,
  isLoginModalOpen,
  pendingChallengeId,
  setIsLoginModalOpen,
  status,
} from '~/stores/accurateAuth';
import { useAuth, PermissionGuard } from '~/auth';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Button } from '~/components/ui/button';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { IconAlertTriangle, IconDatabase, IconKey, IconLoader2, IconLogout } from '@tabler/icons-solidjs';
export const AccurateLoginModal: Component = () => {
  const { t } = useI18n();
  const [otp, setOtp] = createSignal('');
  const auth = useAuth();

  const onRelogin = async (e: Event) => {
    e.preventDefault();
    const success = await handleLogin(otp());
    if (success) {
      setOtp('');
    }
  };

  const handleClose = () => {
    setOtp('');
    setIsLoginModalOpen(false);
  };

  return (
    <Dialog open={isLoginModalOpen()} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2 text-base">
            <Show
              when={databases().length > 0 && !status().connected}
              fallback={
                <Show when={pendingChallengeId()} fallback={<IconKey class="size-5 text-primary" />}>
                  <IconKey class="size-5 text-amber-500" />
                </Show>
              }
            >
              <IconDatabase class="size-5 text-sky-500" />
            </Show>
            {databases().length > 0 && !status().connected
              ? t('barcode.selectDatabase')
              : pendingChallengeId()
              ? t('barcode.twoFactorTitle')
              : t('barcode.accurateConnection')}
          </DialogTitle>
          <DialogDescription>
            {databases().length > 0 && !status().connected
              ? t('barcode.selectDatabaseDesc')
              : pendingChallengeId()
              ? t('barcode.totpDesc')
              : t('barcode.connectAccurateDesc')}
          </DialogDescription>
        </DialogHeader>

        <Show when={authError()}>
          <Alert variant="destructive">
            <IconAlertTriangle class="size-4" />
            <AlertTitle>{t('barcode.authNotice')}</AlertTitle>
            <AlertDescription class="text-xs">{authError()}</AlertDescription>
          </Alert>
        </Show>

        {/* Database Selection */}
        {/* If already connected, show active connection & logout / switch database option */}
        <Show when={status().connected}>
          <div class="space-y-4 py-2">
            <div class="rounded-lg border bg-muted/40 p-3 space-y-1">
              <div class="text-xs font-semibold text-foreground">
                Connected Database
              </div>
              <div class="text-sm font-medium text-emerald-600 dark:text-emerald-400">
                {status().database?.alias || `ID: ${status().database?.id}`}
              </div>
              <div class="text-[11px] text-muted-foreground">
                Host: {status().database?.host || 'Accurate Online'}
              </div>
            </div>

            <Show when={databases().length > 1 && auth.hasPermission('accurate:update')}>
              <div class="space-y-2">
                <label class="text-xs font-semibold text-foreground">Switch Database</label>
                <div class="max-h-48 overflow-y-auto space-y-1.5 pr-1">
                  <For each={databases()}>
                    {(db) => (
                      <button
                        type="button"
                        onClick={() => void handleSelectDatabase(db.id)}
                        disabled={isAuthLoading() || db.id === status().database?.id}
                        class={`w-full text-left p-2.5 rounded-md border text-xs flex items-center justify-between transition-colors ${
                          db.id === status().database?.id
                            ? 'bg-primary/10 border-primary font-medium'
                            : 'bg-card hover:bg-accent'
                        }`}
                      >
                        <span>{db.alias || `Database #${db.id}`}</span>
                        <IconDatabase class="size-3.5 text-muted-foreground" />
                      </button>
                    )}
                  </For>
                </div>
              </div>
            </Show>

            <DialogFooter class="sm:justify-between gap-2 pt-2">
              <Button variant="outline" onClick={handleClose}>
                {t('common.close')}
              </Button>
              <PermissionGuard permission="accurate:delete">
                <Button variant="destructive" onClick={() => void handleLogout()} disabled={isAuthLoading()} class="gap-1.5">
                  <IconLogout class="size-4" />
                  <span>Sign Out</span>
                </Button>
              </PermissionGuard>
            </DialogFooter>
          </div>
        </Show>

        {/* Database Selection */}
        <Show when={!status().connected}>
        <Show
          when={databases().length > 0}
          fallback={
            <Show
              when={pendingChallengeId()}
              fallback={
                <div class="space-y-4 py-2">
                  <div class="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground leading-relaxed">
                    {t('barcode.singleSessionNotice')}
                  </div>

                  <DialogFooter class="sm:justify-end gap-2 pt-2">
                    <Button variant="outline" onClick={handleClose}>
                      {t('common.cancel')}
                    </Button>
                    <PermissionGuard
                      permission="accurate:create"
                      fallback={<Button disabled>Unauthorized</Button>}
                    >
                      <Button onClick={onRelogin} disabled={isAuthLoading()}>
                        <Show when={isAuthLoading()}>
                          <IconLoader2 class="mr-2 size-4 animate-spin" />
                        </Show>
                        {isAuthLoading() ? t('barcode.connecting') : t('barcode.connectToAccurate')}
                      </Button>
                    </PermissionGuard>
                  </DialogFooter>
                </div>
              }
            >
              <form onSubmit={onRelogin} class="space-y-4 py-2">
                <div class="space-y-2">
                  <label class="text-xs font-semibold text-foreground">
                    {t('barcode.totpPrompt')}
                  </label>
                  <input
                    type="text"
                    required
                    maxLength={6}
                    value={otp()}
                    onInput={(e) => setOtp(e.currentTarget.value)}
                    placeholder={t('barcode.totpPlaceholder')}
                    class="w-full text-center font-mono text-lg tracking-widest px-3 py-2 border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                  <p class="text-[11px] text-muted-foreground">
                    {t('barcode.totpHelp')}
                  </p>
                </div>

                <DialogFooter class="sm:justify-end gap-2 pt-2">
                  <Button variant="outline" type="button" onClick={handleClose}>
                    {t('common.cancel')}
                  </Button>
                  <Button type="submit" disabled={isAuthLoading() || otp().trim().length < 6}>
                    <Show when={isAuthLoading()}>
                      <IconLoader2 class="mr-2 size-4 animate-spin" />
                    </Show>
                    {isAuthLoading() ? t('barcode.verifying') : t('barcode.verifyContinue')}
                  </Button>
                </DialogFooter>
              </form>
            </Show>
          }
        >
          <div class="space-y-3 py-2">
            <p class="text-xs text-muted-foreground">
              {t('barcode.selectDatabasePrompt')}
            </p>
            <div class="max-h-60 overflow-y-auto space-y-2 pr-1">
              <For each={databases()}>
                {(db) => (
                  <button
                    type="button"
                    onClick={() => void handleSelectDatabase(db.id)}
                    disabled={isAuthLoading()}
                    class="w-full text-left p-3 rounded-lg border bg-card hover:bg-accent hover:text-accent-foreground transition-colors flex items-center justify-between group"
                  >
                    <div>
                      <div class="font-medium text-xs text-foreground group-hover:text-accent-foreground">
                        {db.alias || t('barcode.databaseFallback', { id: db.id })}
                      </div>
                      <div class="text-[10px] text-muted-foreground">
                        ID: {db.id} • {db.host}
                      </div>
                    </div>
                    <IconDatabase class="size-4 text-muted-foreground group-hover:text-primary transition-colors" />
                  </button>
                )}
              </For>
            </div>
            <DialogFooter class="sm:justify-end pt-2">
              <Button variant="outline" onClick={handleClose}>
                {t('common.cancel')}
              </Button>
            </DialogFooter>
          </div>
        </Show>
        </Show>
      </DialogContent>
    </Dialog>
  );
};
