import { USER_ROLE } from '~/domain';

import type { UserRole } from "~/domain";

import {
  createContext,
  useContext,
  createSignal,
  onMount,
  type JSX,
  type ParentComponent,
} from 'solid-js';

export interface AuthUser {
  email: string;
  name: string;
  picture: string;
  role: UserRole;
  locale?: string;
}

export interface AuthConfig {
  auth_enabled: boolean;
  google_client_id?: string;
}

export interface AuthContextValue {
  user: () => AuthUser | null | undefined;
  authConfig: () => AuthConfig | undefined;
  loading: () => boolean;
  login: (credential: string) => Promise<void>;
  logout: () => Promise<void>;
  refetch: () => Promise<void>;
  setUserLocale: (locale: string) => void;
}

const AuthContext = createContext<AuthContextValue>();

export const AuthProvider: ParentComponent = (props) => {
  // undefined = initial loading state, null = unauthenticated, AuthUser = authenticated
  const [user, setUser] = createSignal<AuthUser | null | undefined>(undefined);
  const [authConfig, setAuthConfig] = createSignal<AuthConfig | undefined>(undefined);
  const [loading, setLoading] = createSignal(true);

  const fetchAuth = async () => {
    try {
      setLoading(true);

      // 1. Fetch public auth config
      const cfgRes = await fetch('/auth/config');
      if (!cfgRes.ok) {
        throw new Error('Failed to load auth configuration');
      }
      const cfg: AuthConfig = await cfgRes.json();
      setAuthConfig(cfg);

      // If auth is completely disabled in backend, allow full access
      if (!cfg.auth_enabled) {
        setUser({
          email: '',
          name: 'Developer',
			picture: '',
			role: USER_ROLE.SUPERADMIN,
        });
        setLoading(false);
        return;
      }

      // 2. Fetch current session if auth is enabled
      const meRes = await fetch('/auth/me');
      if (meRes.ok) {
        const data = await meRes.json();
        setUser(data.user);
      } else {
        setUser(null);
      }
    } catch (err) {
      console.error('Auth initialization error:', err);
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void fetchAuth();
  });

  const login = async (credential: string): Promise<void> => {
    setLoading(true);
    try {
      const res = await fetch('/auth/google/callback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ credential }),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || 'Login failed');
      }

      const data = await res.json();
      setUser(data.user);
    } finally {
      setLoading(false);
    }
  };

  const logout = async (): Promise<void> => {
    setLoading(true);
    try {
      await fetch('/auth/logout', { method: 'POST' });
      setUser(null);
    } catch (err) {
      console.error('Logout error:', err);
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  const refetch = async (): Promise<void> => {
    await fetchAuth();
  };

  const setUserLocale = (locale: string) => {
    setUser((prev) => (prev ? { ...prev, locale } : prev));
  };

  const value: AuthContextValue = {
    user,
    authConfig,
    loading,
    login,
    logout,
    refetch,
    setUserLocale,
  };

  return <AuthContext.Provider value={value}>{props.children}</AuthContext.Provider>;
};

export const useAuth = (): AuthContextValue => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
