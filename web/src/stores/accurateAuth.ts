import { ACCURATE_LOGIN_STATUS } from '~/domain';

import { createSignal } from 'solid-js';
import * as api from '~/api';
import type { AccurateDatabase, AccurateStatus } from '~/types';

export const [status, setStatus] = createSignal<AccurateStatus>({
  enabled: false,
  connected: false,
});
export const [databases, setDatabases] = createSignal<AccurateDatabase[]>([]);
export const [isAuthLoading, setIsAuthLoading] = createSignal(false);
export const [authError, setAuthError] = createSignal<string | null>(null);
export const [isLoginModalOpen, setIsLoginModalOpen] = createSignal(false);
export const [pendingChallengeId, setPendingChallengeId] = createSignal<string | null>(null);

export async function refreshStatus(): Promise<void> {
  try {
    const s = await api.getAccurateStatus();
    setStatus(s);
  } catch {
    setStatus({ enabled: false, connected: false });
  }
}

export async function handleLogin(otpParam?: string): Promise<boolean> {
  setIsAuthLoading(true);
  setAuthError(null);

  try {
    const res = await api.accurateLogin({
      challengeId: pendingChallengeId() ?? undefined,
      otp: otpParam || undefined,
    });

	if (res.status === ACCURATE_LOGIN_STATUS.MFA_REQUIRED && res.challengeId) {
      setPendingChallengeId(res.challengeId);
      setIsAuthLoading(false);
      return false; // MFA pending
    }

    setPendingChallengeId(null);
    // Fetch accessible databases immediately
    const dbs = await api.getAccurateDatabases();
    setDatabases(dbs);

    // If only one database is available, select it automatically
    if (dbs.length === 1) {
      await handleSelectDatabase(dbs[0].id);
    }

    setIsAuthLoading(false);
    return true; // Authenticated
  } catch (err) {
    setIsAuthLoading(false);
    const msg = err instanceof Error ? err.message : 'Login failed';
    setAuthError(msg);
    return false;
  }
}

export async function handleSelectDatabase(dbId: number): Promise<boolean> {
  setIsAuthLoading(true);
  setAuthError(null);

  try {
    const res = await api.selectAccurateDatabase({ id: dbId });
    if (res.success) {
      await refreshStatus();
      setIsLoginModalOpen(false);
      setIsAuthLoading(false);
      return true;
    }
    setIsAuthLoading(false);
    return false;
  } catch (err) {
    setIsAuthLoading(false);
    const msg = err instanceof Error ? err.message : 'Failed to select database';
    setAuthError(msg);
    return false;
  }
}

export async function handleLogout(): Promise<boolean> {
  setIsAuthLoading(true);
  setAuthError(null);
  try {
    await api.accurateLogout();
    await refreshStatus();
    setIsLoginModalOpen(false);
    setIsAuthLoading(false);
    return true;
  } catch (err) {
    setIsAuthLoading(false);
    const msg = err instanceof Error ? err.message : 'Logout failed';
    setAuthError(msg);
    return false;
  }
}

export function handleSessionInvalidated(message?: string): void {
  setStatus((prev) => ({ ...prev, connected: false }));
  setAuthError(message || 'Your Accurate session was invalidated or expired. Please sign in again.');
  setIsLoginModalOpen(true);
}
