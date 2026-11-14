import {
  type Component,
  createMemo,
  createResource,
  createSignal,
  For,
  Show,
} from 'solid-js';
import { useI18n } from '~/i18n';
import { PermissionGuard, useAuth } from '~/auth';
import * as api from '~/api';
	import type { AccurateAccount as AccurateAccountDTO, AccurateDatabase, AccurateStatus } from '~/types';
	import { pendingChallengeFor, refreshStatus, refreshDatabases, statusFor, databasesFor, errorFor, handleLogin, handleSelectDatabase, handleLogout } from '~/stores/accurateAuth';
	import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table';
import { DataTable } from '~/components/DataTable';
import { TableSkeleton } from '~/components/TableSkeleton';
import { Button } from '~/components/ui/button';
import { Badge } from '~/components/ui/badge';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip';
import {
  IconAlertCircle,
  IconCheck,
  IconCloudComputing,
  IconFileText,
  IconInfoCircle,
  IconLoader2,
  IconPencil,
  IconPlugConnected,
  IconPlugConnectedX,
  IconTrash,
} from '@tabler/icons-solidjs';

export const AccurateAccount: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();

  const [actionError, setActionError] = createSignal<string | null>(null);
  const [actionSuccess, setActionSuccess] = createSignal<string | null>(null);
  const [otp, setOtp] = createSignal('');

	const [accountDialogOpen, setAccountDialogOpen] = createSignal(false);
	const [editingAccount, setEditingAccount] = createSignal<AccurateAccountDTO | null>(null);
	const [accountName, setAccountName] = createSignal('');
	const [accountProvider, setAccountProvider] = createSignal<'web' | 'official'>('web');
	const [accountEnabled, setAccountEnabled] = createSignal(true);
	const [accountEmail, setAccountEmail] = createSignal('');
	const [accountPassword, setAccountPassword] = createSignal('');
	const [accountToken, setAccountToken] = createSignal('');
	const [accountSignature, setAccountSignature] = createSignal('');
	const [clearSignature, setClearSignature] = createSignal(false);
	const [savingAccount, setSavingAccount] = createSignal(false);
	const [deletingAccount, setDeletingAccount] = createSignal<AccurateAccountDTO | null>(null);
	const [accountMutationID, setAccountMutationID] = createSignal<number | null>(null);
	const [selectedAccountID, setSelectedAccountID] = createSignal<number | null>(null);
	const [accountsResource, { refetch: refetchAccounts }] = createResource(
		() => auth.hasPermission('accurate_account:read'),
		async (allowed) => allowed ? await api.getAccurateAccounts() : []
	);
	const selectedAccount = createMemo(() => accountsResource()?.find((account) => account.id === selectedAccountID()));
	const [accountDatabases, setAccountDatabases] = createSignal<AccurateDatabase[]>([]);
	const [accountDetailStatus, setAccountDetailStatus] = createSignal<AccurateStatus>({ enabled: false, connected: false });
	const [accountSessionLoading, setAccountSessionLoading] = createSignal(false);
  const clearFeedback = () => {
    setActionError(null);
    setActionSuccess(null);
  };

	const openAccountDialog = (account: AccurateAccountDTO | null = null) => {
		setEditingAccount(account);
		setAccountName(account?.name ?? '');
		setAccountProvider(account?.provider ?? 'web');
		setAccountEnabled(account?.enabled ?? true);
		setAccountEmail(account?.email ?? '');
		setAccountPassword('');
		setAccountToken('');
		setAccountSignature('');
		setClearSignature(false);
		setAccountDialogOpen(true);
	};

	const saveAccount = async (event: SubmitEvent) => {
		event.preventDefault();
		setSavingAccount(true);
		setActionError(null);
		const edited = editingAccount();
		const credentialsChanged = !!edited && (accountEmail() !== edited.email || accountPassword() !== '' || accountToken() !== '' || accountSignature() !== '' || clearSignature());
		try {
			if (editingAccount()) {
				await api.updateAccurateAccount(editingAccount()!.id, {
					name: accountName(), enabled: accountEnabled(), email: accountEmail(),
					password: accountPassword() || undefined, api_token: accountToken() || undefined,
					signature_secret: accountSignature() || undefined, clear_signature_secret: clearSignature(),
				});
			} else {
				await api.createAccurateAccount({
					name: accountName(), provider: accountProvider(), enabled: accountEnabled(), email: accountEmail(),
					password: accountPassword() || undefined, api_token: accountToken() || undefined,
					signature_secret: accountSignature() || undefined,
				});
			}
			if (edited && selectedAccountID() === edited.id && (credentialsChanged || !accountEnabled())) {
				accountViewGeneration++;
				setAccountDetailStatus({enabled:accountEnabled(),connected:false,accountId:edited.id,accountName:accountName()});
				setAccountDatabases([]);
			}
			setAccountDialogOpen(false);
			setActionSuccess(t(editingAccount() ? 'accurate.accountUpdated' : 'accurate.accountCreated'));
			await refetchAccounts();
		} catch (err) {
			setActionError(err instanceof Error ? err.message : t('common.unknownError'));
		} finally { setSavingAccount(false); }
	};

	const activateAccount = async (account: AccurateAccountDTO) => {
		setAccountMutationID(account.id);
		setActionError(null);
		try {
			await api.activateAccurateAccount(account.id);
			await Promise.all([refetchAccounts(), refreshStatus()]);
			setActionSuccess(t('accurate.accountActivated'));
		} catch (err) { setActionError(err instanceof Error ? err.message : t('common.unknownError')); }
		finally { setAccountMutationID(null); }
	};

	const confirmDeleteAccount = async () => {
		const account = deletingAccount();
		if (!account) return;
		setAccountMutationID(account.id);
		try {
			await api.deleteAccurateAccount(account.id);
			setDeletingAccount(null);
			if (selectedAccountID() === account.id) {
				accountViewGeneration++; setSelectedAccountID(null); setAccountDetailStatus({enabled:false,connected:false}); setAccountDatabases([]);
			}
			await Promise.all([refetchAccounts(), refreshStatus()]);
			setActionSuccess(t('accurate.accountDeleted'));
		} catch (err) { setActionError(err instanceof Error ? err.message : t('common.unknownError')); }
		finally { setAccountMutationID(null); }
	};



	let accountViewGeneration = 0;
	const inspectAccount = async (accountID: number) => {
		const generation = ++accountViewGeneration;
		setSelectedAccountID(accountID); setOtp(''); setAccountDatabases([]); setAccountDetailStatus({ enabled: false, connected: false }); setActionError(null);
		setAccountSessionLoading(false);
		try {
			await refreshStatus(accountID);
			if (generation !== accountViewGeneration) return;
			const current = statusFor(accountID);
			setAccountDetailStatus(current);
			const dbs = current.connected ? await refreshDatabases(accountID) : [];
			if (generation === accountViewGeneration) setAccountDatabases(dbs);
		} catch (err) { if (generation === accountViewGeneration) setActionError(err instanceof Error ? err.message : t('common.unknownError')); }
	};
	const connectSelectedAccount = async () => {
		const accountID = selectedAccountID(); if (accountID == null) return;
		const generation = accountViewGeneration; clearFeedback(); setAccountSessionLoading(true);
		try {
			const ok = await handleLogin(otp().trim() || undefined, accountID);
			if (!ok || generation !== accountViewGeneration || selectedAccountID() !== accountID) {
				if (errorFor(accountID)) setActionError(errorFor(accountID));
				return;
			}
			setOtp(''); setAccountDetailStatus(statusFor(accountID)); setAccountDatabases(databasesFor(accountID)); await refetchAccounts();
		} catch (err) { if (generation === accountViewGeneration) setActionError(err instanceof Error ? err.message : t('common.unknownError')); }
		finally { if (generation === accountViewGeneration) setAccountSessionLoading(false); }
	};
	const selectDatabaseForAccount = async (accountID: number, databaseID: number) => {
		const generation = accountViewGeneration; setAccountSessionLoading(true);
		try {
			if (await handleSelectDatabase(databaseID, accountID) && generation === accountViewGeneration && selectedAccountID() === accountID) {
				setAccountDetailStatus(statusFor(accountID)); setAccountDatabases(databasesFor(accountID)); await refetchAccounts();
			} else if (errorFor(accountID)) setActionError(errorFor(accountID));
		} finally { if (generation === accountViewGeneration) setAccountSessionLoading(false); }
	};
	const disconnectSelectedAccount = async () => {
		const accountID = selectedAccountID(); if (accountID == null) return;
		const generation = accountViewGeneration; setAccountSessionLoading(true);
		try {
			if (await handleLogout(accountID)) { setAccountDetailStatus(statusFor(accountID)); setAccountDatabases([]); await refetchAccounts(); }
			else if (errorFor(accountID)) setActionError(errorFor(accountID));
		} finally { if (generation === accountViewGeneration) setAccountSessionLoading(false); }
	};
	const toggleAccountSession = async (account: AccurateAccountDTO) => {
		setSelectedAccountID(account.id);
		setAccountDetailStatus({ enabled: account.enabled, connected: account.connected, accountId: account.id, accountName: account.name, database: account.database });
		if (account.connected) {
			await disconnectSelectedAccount();
			return;
		}
		await connectSelectedAccount();
	};
	const [sources] = createResource(
		() => accountDetailStatus().connected&&auth.hasPermission('accurate:read')?(selectedAccountID()??0):0,
		async(accountID)=>{if(!accountID)return[];try{return await api.getAccurateSources(accountID);}catch(err){if(selectedAccountID()===accountID)setActionError(err instanceof Error?err.message:t('common.unknownError'));return[];}}
	);


	return (
		<Show when={auth.hasPermission('accurate_account:read')} fallback={<Alert variant="destructive"><AlertTitle>{t('common.error')}</AlertTitle><AlertDescription>{t('accurate.accessDenied')}</AlertDescription></Alert>}>
		<div class="space-y-6 w-full min-w-0 max-w-5xl mx-auto pb-12">
      {/* Page Header */}
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border pb-5">
		<div class="flex items-center gap-3 min-w-0">
          <div class="p-2 rounded-lg bg-primary/10 text-primary">
            <IconCloudComputing class="size-6" />
          </div>
		<div class="min-w-0">
			<h1 class="text-2xl font-bold tracking-tight text-foreground">{t('accurate.accountPageTitle')}</h1>
	<p class="text-sm text-muted-foreground mt-0.5 break-words">{t('accurate.accountPageDescription')}</p>
          </div>
        </div>
		<Show when={auth.hasPermission('accurate_account:create')}>
			<Button onClick={() => openAccountDialog()}>{t('accurate.addAccount')}</Button>
		</Show>
      </div>

      {/* Feedback Alerts */}
      <Show when={actionSuccess()}>
        <div class="flex items-center gap-2 p-3 text-sm rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
          <IconCheck class="h-4 w-4 shrink-0" />
          <span>{actionSuccess()}</span>
        </div>
      </Show>

	<Show when={actionError()}>
        {(err) => (
          <Alert variant="destructive">
            <IconAlertCircle class="size-4" />
            <AlertTitle>{t('common.error')}</AlertTitle>
            <AlertDescription class="text-xs">{err()}</AlertDescription>
          </Alert>
        )}
      </Show>

		<Show when={auth.hasPermission('accurate_account:read')}>
			<Card class="border-border shadow-sm">
				<CardHeader>
					<CardTitle>{t('accurate.accountList')}</CardTitle>
					<CardDescription>{t('accurate.activeWorkstationAccount')}: {accountsResource()?.find((account) => account.is_default)?.name || t('accurate.noDatabaseSelected')}</CardDescription>
				</CardHeader>
				<CardContent class="p-0">
					<Show when={!accountsResource.loading} fallback={<div class="p-6"><TableSkeleton columns={5} rows={3} /></div>}>
						<Show when={(accountsResource() || []).length > 0} fallback={<p class="p-6 text-sm text-muted-foreground">{t('accurate.noStoredAccounts')}</p>}>
							<div class="space-y-3 p-3 sm:hidden">
								<For each={accountsResource()}>{(account) => <div class="rounded-md border border-border p-3 space-y-3">
									<div><button type="button" class="font-medium underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => void inspectAccount(account.id)}>{account.name}</button><div class="text-xs text-muted-foreground break-all">{account.email}</div></div>
									<div class="flex flex-wrap items-center gap-2"><Badge variant="outline">{account.provider === 'web' ? t('accurate.providerWeb') : t('accurate.providerOfficial')}</Badge><Badge variant={account.enabled ? 'default' : 'secondary'}>{account.enabled ? t('accurate.enabled') : t('accurate.disabled')}</Badge><Show when={account.is_default}><Badge variant="outline">{t('accurate.activeWorkstationAccount')}</Badge></Show></div>
									<p class="text-sm text-muted-foreground">{account.connected ? account.database?.alias || t('accurate.connected') : t('accurate.disconnected')}</p>
									<div class="flex flex-wrap gap-2"><Show when={auth.hasPermission(account.connected ? 'accurate:delete' : 'accurate:create')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={account.connected ? t('accurate.disconnectBtn') : t('accurate.connectBtn')} disabled={!account.enabled || accountSessionLoading()} onClick={() => void toggleAccountSession(account)}><Show when={account.connected} fallback={<IconPlugConnected class="size-4" />}><IconPlugConnectedX class="size-4" /></Show></TooltipTrigger><TooltipContent>{account.connected ? t('accurate.disconnectBtn') : t('accurate.connectBtn')}</TooltipContent></Tooltip></Show><Show when={auth.hasPermission('accurate_account:update')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={t('common.edit')} onClick={() => openAccountDialog(account)}><IconPencil class="size-4" /></TooltipTrigger><TooltipContent>{t('common.edit')}</TooltipContent></Tooltip><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={t('accurate.useAccount')} disabled={!account.enabled || !account.connected || account.is_default || accountMutationID() === account.id} onClick={() => void activateAccount(account)}><IconCheck class="size-4" /></TooltipTrigger><TooltipContent>{t('accurate.useAccount')}</TooltipContent></Tooltip></Show><Show when={auth.hasPermission('accurate_account:delete')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="destructive" aria-label={t('accurate.deleteAccount')} onClick={() => setDeletingAccount(account)}><IconTrash class="size-4" /></TooltipTrigger><TooltipContent>{t('accurate.deleteAccount')}</TooltipContent></Tooltip></Show></div>
								</div>}</For>
							</div>
							<div class="hidden sm:block">
							<DataTable>
								<TableHeader><TableRow><TableHead>{t('accurate.accountName')}</TableHead><TableHead>{t('accurate.provider')}</TableHead><TableHead>{t('common.status')}</TableHead><TableHead>{t('accurate.activeDatabase')}</TableHead><TableHead class="text-right">{t('common.actions')}</TableHead></TableRow></TableHeader>
								<TableBody>
									<For each={accountsResource()}>{(account) => <TableRow>
						<TableCell><button type="button" class="text-left font-medium underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => void inspectAccount(account.id)}>{account.name}</button><div class="text-xs text-muted-foreground">{account.email}</div></TableCell>
										<TableCell>{account.provider === 'web' ? t('accurate.providerWeb') : t('accurate.providerOfficial')}</TableCell>
										<TableCell><Badge variant={account.enabled ? 'default' : 'secondary'}>{account.enabled ? t('accurate.enabled') : t('accurate.disabled')}</Badge></TableCell>
										<TableCell>{account.connected ? account.database?.alias || t('accurate.connected') : t('accurate.disconnected')}{account.is_default && <Badge class="ml-2" variant="outline">{t('accurate.activeWorkstationAccount')}</Badge>}</TableCell>
										<TableCell class="text-right"><div class="flex justify-end gap-2">
											<Show when={auth.hasPermission(account.connected ? 'accurate:delete' : 'accurate:create')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={account.connected ? t('accurate.disconnectBtn') : t('accurate.connectBtn')} disabled={!account.enabled || accountSessionLoading()} onClick={() => void toggleAccountSession(account)}><Show when={account.connected} fallback={<IconPlugConnected class="size-4" />}><IconPlugConnectedX class="size-4" /></Show></TooltipTrigger><TooltipContent>{account.connected ? t('accurate.disconnectBtn') : t('accurate.connectBtn')}</TooltipContent></Tooltip></Show>
											<Show when={auth.hasPermission('accurate_account:update')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={t('common.edit')} onClick={() => openAccountDialog(account)}><IconPencil class="size-4" /></TooltipTrigger><TooltipContent>{t('common.edit')}</TooltipContent></Tooltip></Show>
											<Show when={auth.hasPermission('accurate_account:update')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="outline" aria-label={t('accurate.useAccount')} disabled={!account.enabled || !account.connected || account.is_default || accountMutationID() === account.id} onClick={() => void activateAccount(account)}><IconCheck class="size-4" /></TooltipTrigger><TooltipContent>{t('accurate.useAccount')}</TooltipContent></Tooltip></Show>
											<Show when={auth.hasPermission('accurate_account:delete')}><Tooltip><TooltipTrigger as={Button} size="icon" variant="destructive" aria-label={t('accurate.deleteAccount')} onClick={() => setDeletingAccount(account)}><IconTrash class="size-4" /></TooltipTrigger><TooltipContent>{t('accurate.deleteAccount')}</TooltipContent></Tooltip></Show>
										</div></TableCell>
									</TableRow>}</For>
								</TableBody>
							</DataTable>
			</div>
						</Show>
					</Show>
				</CardContent>
			</Card>
		</Show>

		<Dialog open={accountDialogOpen()} onOpenChange={setAccountDialogOpen}>
			<DialogContent class="sm:max-w-lg">
				<form onSubmit={saveAccount} class="space-y-4">
					<DialogHeader><DialogTitle>{editingAccount() ? t('common.edit') : t('accurate.addAccount')}</DialogTitle><DialogDescription>{t('accurate.configuredLeaveBlank')}</DialogDescription></DialogHeader>
					<label class="grid gap-1 text-sm">{t('accurate.accountName')}<input required value={accountName()} onInput={(e) => setAccountName(e.currentTarget.value)} class="h-9 rounded-md border border-input bg-background px-3" /></label>
					<Show when={!editingAccount()}><label class="grid gap-1 text-sm">{t('accurate.provider')}<select value={accountProvider()} onChange={(e) => setAccountProvider(e.currentTarget.value as 'web' | 'official')} class="h-9 rounded-md border border-input bg-background px-3"><option value="web">{t('accurate.providerWeb')}</option><option value="official">{t('accurate.providerOfficial')}</option></select></label></Show>
					<label class="flex items-center gap-2 text-sm"><input type="checkbox" checked={accountEnabled()} onChange={(e) => setAccountEnabled(e.currentTarget.checked)} />{t('accurate.enabled')}</label>
					<Show when={accountProvider() === 'web'}><label class="grid gap-1 text-sm">{t('accurate.accountEmail')}<input required type="email" value={accountEmail()} onInput={(e) => setAccountEmail(e.currentTarget.value)} class="h-9 rounded-md border border-input bg-background px-3" /></label><label class="grid gap-1 text-sm">{t('accurate.password')}<input type="password" required={!editingAccount()} value={accountPassword()} onInput={(e) => setAccountPassword(e.currentTarget.value)} class="h-9 rounded-md border border-input bg-background px-3" /></label><Show when={editingAccount()?.has_password}><p class="text-xs text-muted-foreground">{t('accurate.configuredLeaveBlank')}</p></Show></Show>
					<Show when={accountProvider() === 'official'}><label class="grid gap-1 text-sm">API token<input type="password" required={!editingAccount()} value={accountToken()} onInput={(e) => setAccountToken(e.currentTarget.value)} class="h-9 rounded-md border border-input bg-background px-3" /></label><Show when={editingAccount()?.has_api_token}><p class="text-xs text-muted-foreground">{t('accurate.configuredLeaveBlank')}</p></Show></Show>
					<label class="grid gap-1 text-sm">{t('accurate.signatureSecret')}<input type="password" value={accountSignature()} onInput={(e) => setAccountSignature(e.currentTarget.value)} class="h-9 rounded-md border border-input bg-background px-3" /></label>
					<Show when={editingAccount()?.has_signature_secret}><label class="flex items-center gap-2 text-sm"><input type="checkbox" checked={clearSignature()} onChange={(e) => setClearSignature(e.currentTarget.checked)} />{t('accurate.clearSignatureSecret')}</label></Show>
					<DialogFooter><Button type="button" variant="outline" onClick={() => setAccountDialogOpen(false)}>{t('common.cancel')}</Button><Button type="submit" disabled={savingAccount()}>{savingAccount() ? t('common.loading') : t('common.save')}</Button></DialogFooter>
				</form>
			</DialogContent>
		</Dialog>

		<AlertDialog open={!!deletingAccount()} onOpenChange={(open) => !open && setDeletingAccount(null)}>
			<AlertDialogContent><AlertDialogTitle>{t('accurate.deleteAccount')}</AlertDialogTitle><AlertDialogDescription>{t('accurate.deleteAccountWarning', { name: deletingAccount()?.name || '' })}</AlertDialogDescription><AlertDialogFooter><Button variant="outline" onClick={() => setDeletingAccount(null)}>{t('common.cancel')}</Button><Button variant="destructive" disabled={accountMutationID() !== null} onClick={() => void confirmDeleteAccount()}>{t('accurate.deleteAccount')}</Button></AlertDialogFooter></AlertDialogContent>
		</AlertDialog>



		<Show when={accountDetailStatus().connected}>
        {/* Section 4: Document Sources Card (visible when connected + accurate:read) */}
        <PermissionGuard permission="accurate:read">
          <Card class="border-border shadow-sm">
            <CardHeader class="pb-3 border-b border-border/60">
              <CardTitle class="text-base font-semibold flex items-center gap-2">
                <IconFileText class="size-4 text-primary" />
                <span>{t('accurate.documentSources')}</span>
              </CardTitle>
              <CardDescription class="text-xs">{t('accurate.documentSourcesDesc')}</CardDescription>
            </CardHeader>
            <CardContent class="p-0">
              <DataTable>
                <TableHeader>
                  <TableRow class="hover:bg-transparent">
                    <TableHead>{t('accurate.sourceName')}</TableHead>
                    <TableHead class="w-[180px]">{t('accurate.sourceDataPlan')}</TableHead>
                    <TableHead class="w-[160px] text-right">{t('common.status')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <Show
                    when={!sources.loading}
                    fallback={<TableSkeleton columns={3} rows={4} />}
                  >
                    <Show
                      when={(sources() || []).length > 0}
                      fallback={
                        <TableRow>
                          <TableCell
                            colSpan={3}
                            class="h-24 text-center text-sm text-muted-foreground"
                          >
                            {t('accurate.noSources')}
                          </TableCell>
                        </TableRow>
                      }
                    >
                      <For each={sources()}>
                        {(src) => (
                          <TableRow>
                            <TableCell>
                              <div class="font-medium text-sm text-foreground">{src.name}</div>
                              <div class="text-[11px] font-mono text-muted-foreground">{src.id}</div>
                            </TableCell>
                            <TableCell>
                              <Badge variant="outline" class="font-mono text-xs">
                                {src.dataPlan}
                              </Badge>
                            </TableCell>
                            <TableCell class="text-right">
                              <Show
                                when={src.available}
                                fallback={
                                  <Tooltip>
                                    <TooltipTrigger
                                      as="span"
                                      class="inline-flex cursor-help"
                                      aria-label={src.reason || t('accurate.noReasonProvided')}
                                    >
                                      <Badge
                                        variant="outline"
                                        class="gap-1 text-xs bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
                                      >
                                        <span>{t('accurate.sourceUnavailable')}</span>
                                        <IconInfoCircle class="size-3.5" />
                                      </Badge>
                                    </TooltipTrigger>
                                    <TooltipContent class="max-w-xs text-xs">
                                      <div class="font-semibold mb-0.5">
                                        {t('accurate.sourceReason')}
                                      </div>
                                      <div>{src.reason || t('accurate.noReasonProvided')}</div>
                                    </TooltipContent>
                                  </Tooltip>
                                }
                              >
                                <Badge
                                  variant="outline"
                                  class="text-xs bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
                                >
                                  {t('accurate.sourceAvailable')}
                                </Badge>
                              </Show>
                            </TableCell>
                          </TableRow>
                        )}
                      </For>
                    </Show>
                  </Show>
                </TableBody>
              </DataTable>
            </CardContent>
          </Card>
        </PermissionGuard>
      </Show>

		</div>
		</Show>
	);
};

export default AccurateAccount;
