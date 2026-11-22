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
const [workstationStatusState, setWorkstationStatusState] = createSignal<'loading' | 'ready' | 'failed' | 'denied'>('loading');
const activeState = createMemo(() => {
  const id = activeAccountID();
  return id == null ? undefined : accountStates()[id];
});
export const activeStatus = createMemo(() => activeState()?.status ?? disconnected);
export const activeDatabases = createMemo(() => activeState()?.databases ?? []);
export const activeAuthLoading = createMemo(() => activeState()?.loading ?? false);
export const activeAuthError = createMemo(() => activeState()?.error ?? null);
export const activePendingChallengeId = createMemo(() => activeState()?.pendingChallengeId ?? null);
export { activeAccountID, isLoginModalOpen, setIsLoginModalOpen, workstationStatusState };
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
	const token = statusRequestTokens.get(id);
	setLoading(id,true);setError(id,null);
	try {
		const result = await api.getAccurateDatabases(id);
		if (statusRequestTokens.get(id) !== token) return [];
		setDatabases(id,result);
		return result;
	} catch(err) {
		if (statusRequestTokens.get(id) === token) setError(id,err instanceof Error?err.message:'Failed to list databases');
		return [];
	} finally { setLoading(id,false); }
}
function setLoading(id: number, loading: boolean) { patchAccount(id, { loading }); }
function setError(id: number, error: string | null) { patchAccount(id, { error }); }
function setDatabases(id: number, databases: AccurateDatabase[]) { patchAccount(id, { databases }); }
// Metadata reads are cheap and must neither cancel an explicit validation nor
// erase its outcome for the same established database session.
let statusSequence = 0;
const statusRequestTokens = new Map<number, number>();
const metadataRequestTokens = new Map<number, number>();
const validationResults = new Map<number, { status: AccurateStatus; databaseID?: number }>();
let activeStatusRequestToken = 0;
function invalidateStatusRequest(accountId: number): number {
	const token = ++statusSequence;
	statusRequestTokens.set(accountId, token);
	validationResults.delete(accountId);
	return token;
}
function applyStatus(accountId: number, status: AccurateStatus, explicitValidation = false) {
	const cleared = status.sessionState === 'expired' || status.sessionState === 'disconnected'
		|| explicitValidation && status.sessionState === 'unverified';
	patchAccount(accountId, { status, ...(cleared ? { databases: [], pendingChallengeId: null } : {}) });
}
function applyMetadata(accountId: number, metadata: AccurateStatus) {
	const checked = validationResults.get(accountId);
	if (metadata.provider === 'web' && checked && (
		metadata.connected && metadata.database?.id === checked.databaseID ||
		!metadata.connected && checked.status.sessionState === 'expired'
	)) {
		applyStatus(accountId, { ...metadata, connected: checked.status.connected, database: checked.status.database,
			sessionState: checked.status.sessionState, verificationError: checked.status.verificationError });
	} else {
		validationResults.delete(accountId);
		applyStatus(accountId, metadata);
	}
}
function markUnverified(accountId: number) {
	const previous = statusFor(accountId);
	patchAccount(accountId, { status: { ...previous, connected: false, database: undefined,
		sessionState: previous.provider === 'official' ? 'not_applicable' : 'unverified',
		verificationError: 'Unable to verify the Accurate session. Please try again.' }, databases: [], pendingChallengeId: null });
}
export async function verifyStatus(accountId: number): Promise<AccurateStatus> {
	const previous = statusFor(accountId);
	const databaseID = previous.database?.id ?? validationResults.get(accountId)?.databaseID;
	const token = invalidateStatusRequest(accountId);
	// Checking is tracked by the account page, not by clearing the shared session.
	try {
		const result = await api.getAccurateStatus(accountId, true);
		if (statusRequestTokens.get(accountId) !== token) return statusFor(accountId);
		validationResults.set(accountId, { status: result, databaseID: result.database?.id ?? databaseID });
		applyStatus(accountId, result, true);
		return result;
	} catch (error) {
		if (statusRequestTokens.get(accountId) !== token) return statusFor(accountId);
		markUnverified(accountId);
		validationResults.set(accountId, { status: statusFor(accountId), databaseID });
		throw error;
	}
}
export async function refreshStatus(accountId?: number): Promise<AccurateStatus | undefined> {
	const token = ++statusSequence;
	if (accountId == null) {
		activeStatusRequestToken = token;
		if (workstationStatusState() !== 'ready') setWorkstationStatusState('loading');
	} else metadataRequestTokens.set(accountId, token);
	const startedAccountID = accountId ?? activeAccountID();
	try {
		const result = await api.getAccurateStatus(accountId);
		// Per-account reads cannot settle or hide the workstation indicator.
		if (accountId != null) {
			if (metadataRequestTokens.get(accountId) === token && (statusRequestTokens.get(accountId) ?? 0) <= token) applyMetadata(accountId, result);
			return result;
		}
		if (activeStatusRequestToken !== token) return;
		setWorkstationStatusState('ready');
		const selectedID = result.accountId ?? null;
		const previousID = activeAccountID();
		if (previousID !== selectedID) {
			if (previousID != null) setPendingChallengeFor(previousID, null);
			setIsLoginModalOpen(false);
		}
		setActiveAccountID(selectedID);
		if (selectedID != null && (statusRequestTokens.get(selectedID) ?? 0) <= token && (metadataRequestTokens.get(selectedID) ?? 0) <= token) {
			metadataRequestTokens.set(selectedID, token);
			applyMetadata(selectedID, result);
		}
		return result;
	} catch (err) {
		const currentRequest = accountId == null ? activeStatusRequestToken === token : metadataRequestTokens.get(accountId) === token;
		if (!currentRequest) return;
		if (accountId == null) setWorkstationStatusState(err instanceof api.ApiError && err.status === 403 ? 'denied' : 'failed');
		if (startedAccountID != null && (statusRequestTokens.get(startedAccountID) ?? 0) <= token
			&& (metadataRequestTokens.get(startedAccountID) ?? 0) <= token
			&& !validationResults.has(startedAccountID)) markUnverified(startedAccountID);
	}
}

// Query consumers report a changed context; only the store owns its recovery read.
export async function handleAccurateContextChanged(sourceAccountID: number): Promise<void> {
	if (sourceAccountID !== activeAccountID()) return;
	await refreshStatus();
}

export async function handleLogin(otpParam?: string, requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) return false;
  invalidateStatusRequest(accountID);
  setLoading(accountID, true);
  setError(accountID, null);
  const submittedChallengeID = otpParam ? pendingChallengeFor(accountID) : null;
  if (!otpParam) {
    setPendingChallengeFor(accountID, null);
		setDatabases(accountID, []);
		const previous = statusFor(accountID);
		patchAccount(accountID, { status: { ...previous, connected: false, database: undefined,
			sessionState: previous.provider === 'official' ? 'not_applicable' : 'unverified', verificationError: undefined } });
	}
	try {
    const res = await api.accurateLogin({ challengeId: submittedChallengeID ?? undefined, otp: otpParam || undefined }, accountID);
		invalidateStatusRequest(accountID);
    if (res.status === ACCURATE_LOGIN_STATUS.MFA_REQUIRED && res.challengeId) { setPendingChallengeFor(accountID, res.challengeId); return false; }
    setPendingChallengeFor(accountID, null);
    const databases = await api.getAccurateDatabases(accountID);
    setDatabases(accountID, databases);
    if (databases.length === 1) await handleSelectDatabase(databases[0].id, accountID);
    else await refreshStatus(accountID);
    return true;
  } catch (err) {
	if (submittedChallengeID != null && pendingChallengeFor(accountID) === submittedChallengeID) {
		setPendingChallengeFor(accountID, null);
		setDatabases(accountID, []);
		const previous = statusFor(accountID);
		patchAccount(accountID, { status: { ...previous, connected: false, database: undefined,
			sessionState: previous.provider === 'official' ? 'not_applicable' : 'disconnected', verificationError: undefined } });
	}
    setError(accountID, err instanceof Error ? err.message : 'Login failed');
    return false;
  }
  finally { setLoading(accountID, false); }
}

export async function handleSelectDatabase(databaseID: number, requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) return false;
  invalidateStatusRequest(accountID);
  setLoading(accountID, true);
  setError(accountID, null);
  try {
    const result = await api.selectAccurateDatabase({ id: databaseID }, accountID);
		invalidateStatusRequest(accountID);
    if (!result.success) return false;
    await refreshStatus(accountID);
    if (accountID === activeAccountID()) setIsLoginModalOpen(false);
    return true;
  } catch (err) { setError(accountID, err instanceof Error ? err.message : 'Failed to select database'); return false; }
  finally { setLoading(accountID, false); }
}

export async function handleLogout(requestedAccountID?: number): Promise<boolean> {
  const accountID = requestedAccountID ?? activeAccountID();
  if (accountID == null) return false;
  invalidateStatusRequest(accountID);
  setLoading(accountID, true);
  setError(accountID, null);
  try {
    await api.accurateLogout(accountID);
		invalidateStatusRequest(accountID);
    setDatabases(accountID, []);
    setPendingChallengeFor(accountID, null);
    await refreshStatus(accountID);
    if (accountID === activeAccountID()) setIsLoginModalOpen(false);
    return true;
  } catch (err) { setError(accountID, err instanceof Error ? err.message : 'Logout failed'); return false; }
  finally { setLoading(accountID, false); }
}

export function handleSessionInvalidated(message?: string, accountID = activeAccountID()): void {
  if (accountID == null) return;
	invalidateStatusRequest(accountID);
  const previous = accountStates()[accountID]?.status ?? disconnected;
	const detail = message || 'Your Accurate session was invalidated or expired. Please sign in again.';
	patchAccount(accountID, { status: { ...previous, connected: false, database: undefined, sessionState: 'expired', verificationError: undefined }, databases: [], pendingChallengeId: null, error: detail });
	validationResults.set(accountID, { status: statusFor(accountID), databaseID: previous.database?.id });
  if (accountID === activeAccountID()) setIsLoginModalOpen(true);
}
