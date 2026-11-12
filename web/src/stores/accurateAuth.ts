import { createMemo, createSignal } from 'solid-js';
import { ACCURATE_LOGIN_STATUS } from '~/domain';
import * as api from '~/api';
import type { AccurateDatabase, AccurateStatus } from '~/types';

type AccountAuthState = {
  status: AccurateStatus;
  databases: AccurateDatabase[];
  pendingChallengeId: string | null;
  loading: boolean;
  error: string | null;
};
const disconnected: AccurateStatus = { enabled: false, connected: false };
const [activeAccountID, setActiveAccountID] = createSignal<number | null>(null);
const [accountStates, setAccountStates] = createSignal<Record<number, AccountAuthState>>({});
const [isLoginModalOpen, setIsLoginModalOpen] = createSignal(false);
const activeState = createMemo(() => {
  const id = activeAccountID();
  return id == null ? undefined : accountStates()[id];
});
export const activeStatus = createMemo(() => activeState()?.status ?? disconnected);
export const activeDatabases = createMemo(() => activeState()?.databases ?? []);
export const activeAuthLoading = createMemo(() => activeState()?.loading ?? false);
export const activeAuthError = createMemo(() => activeState()?.error ?? null);
export const activePendingChallengeId = createMemo(() => activeState()?.pendingChallengeId ?? null);
export { activeAccountID, isLoginModalOpen, setIsLoginModalOpen };

function patchAccount(id: number, patch: Partial<AccountAuthState>) {
  setAccountStates((all) => ({
    ...all,
    [id]: { status: all[id]?.status ?? { ...disconnected, accountId: id }, databases: all[id]?.databases ?? [], pendingChallengeId: all[id]?.pendingChallengeId ?? null, loading: all[id]?.loading ?? false, error: all[id]?.error ?? null, ...patch },
  }));
}
export const pendingChallengeFor = (id: number) => accountStates()[id]?.pendingChallengeId ?? null;
export function setPendingChallengeFor(id: number, challengeId: string | null) { patchAccount(id, { pendingChallengeId: challengeId }); }

export const statusFor = (id:number):AccurateStatus => accountStates()[id]?.status ?? {...disconnected,accountId:id};
export const databasesFor = (id:number):AccurateDatabase[] => accountStates()[id]?.databases ?? [];
export const errorFor = (id:number):string|null => accountStates()[id]?.error ?? null;

export async function refreshDatabases(id:number):Promise<AccurateDatabase[]> {
	setLoading(id,true);setError(id,null);
	try { const result=await api.getAccurateDatabases(id);setDatabases(id,result);return result; }
	catch(err) { setError(id,err instanceof Error?err.message:'Failed to list databases');return []; }
	finally { setLoading(id,false); }
}
function setLoading(id: number, loading: boolean) { patchAccount(id, { loading }); }
function setError(id: number, error: string | null) { patchAccount(id, { error }); }
function setDatabases(id: number, databases: AccurateDatabase[]) { patchAccount(id, { databases }); }

export async function refreshStatus(accountId?: number): Promise<void> {
  try {
    const result = await api.getAccurateStatus(accountId);
    if (accountId != null) {
      patchAccount(accountId, { status: result });
      return;
    }
	const selectedID = result.accountId ?? null;
	const previousID = activeAccountID();
	if (previousID !== selectedID) {
		if (previousID != null) setPendingChallengeFor(previousID,null);
		setIsLoginModalOpen(false);
	}
    setActiveAccountID(selectedID);
    if (selectedID != null) patchAccount(selectedID, { status: result });
  } catch {
    if (accountId != null) patchAccount(accountId, { status: { ...disconnected, accountId } });
	else { const previousID=activeAccountID(); setActiveAccountID(null); if(previousID!=null)setPendingChallengeFor(previousID,null); setIsLoginModalOpen(false); }
  }
}

export async function handleLogin(otpParam?: string, requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) { return false; }
  setLoading(accountID, true);
  setError(accountID, null);
  try {
    const res = await api.accurateLogin({
      challengeId: pendingChallengeFor(accountID) ?? undefined,
      otp: otpParam || undefined,
    }, accountID);
    if (res.status === ACCURATE_LOGIN_STATUS.MFA_REQUIRED && res.challengeId) {
      setPendingChallengeFor(accountID, res.challengeId);
      return false;
    }
    setPendingChallengeFor(accountID, null);
    const databases = await api.getAccurateDatabases(accountID);
    setDatabases(accountID, databases);
    if (databases.length === 1) await handleSelectDatabase(databases[0].id, accountID);
    else await refreshStatus(accountID);
    return true;
  } catch (err) {
    setError(accountID, err instanceof Error ? err.message : 'Login failed');
    return false;
  } finally { setLoading(accountID, false); }
}

export async function handleSelectDatabase(databaseID: number, requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) return false;
  setLoading(accountID, true);
  setError(accountID, null);
  try {
    const result = await api.selectAccurateDatabase({ id: databaseID }, accountID);
    if (!result.success) return false;
    await refreshStatus(accountID);
    if (accountID === activeAccountID()) setIsLoginModalOpen(false);
    return true;
  } catch (err) {
    setError(accountID, err instanceof Error ? err.message : 'Failed to select database');
    return false;
  } finally { setLoading(accountID, false); }
}

export async function handleLogout(requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) return false;
  setLoading(accountID, true);
  setError(accountID, null);
  try {
    await api.accurateLogout(accountID);
    setDatabases(accountID, []);
    setPendingChallengeFor(accountID, null);
    await refreshStatus(accountID);
    if (accountID === activeAccountID()) setIsLoginModalOpen(false);
    return true;
  } catch (err) {
    setError(accountID, err instanceof Error ? err.message : 'Logout failed');
    return false;
  } finally { setLoading(accountID, false); }
}

export function handleSessionInvalidated(message?: string, accountID = activeAccountID()): void {
  if (accountID == null) return;
  const previous = accountStates()[accountID]?.status ?? disconnected;
  patchAccount(accountID, { status: { ...previous, connected: false, database: undefined }, databases: [], pendingChallengeId: null, error: message || 'Your Accurate session was invalidated or expired. Please sign in again.' });
  if (accountID === activeAccountID()) setIsLoginModalOpen(true);
}
