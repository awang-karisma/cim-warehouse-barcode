import type {
  SystemResponse,
  UsersResponse,
  AccurateStatus,
  AccurateDatabase,
  AccurateSearchItemsResult,
  AccurateSourceInfo,
  AccurateDocumentsResult,
  AccurateNormalizedItem,
  PageDesign,
  PageDesignsResponse,
  LabelLayoutPreset,
  LabelLayoutPresetsResponse,
} from './types';

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function qs(params?: Record<string, string | number | boolean | undefined>): string {
  if (!params) return '';
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      searchParams.set(key, String(value));
    }
  }
  return searchParams.toString();
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, options);
  if (!res.ok) {
    const errorText = await res.text();
    let message = errorText;
    try {
      const parsed = JSON.parse(errorText);
      if (parsed?.detail) message = parsed.detail;
      else if (parsed?.error) message = parsed.error;
      else if (parsed?.title) message = parsed.title;
    } catch {}
    throw new ApiError(message || `HTTP ${res.status}: ${res.statusText}`, res.status);
  }
  return res.json() as Promise<T>;
}

// System info
export const getSystemInfo = () => apiFetch<SystemResponse>('/api/v1/system');

// Auth endpoints
export const getAuthConfig = () =>
  apiFetch<{ auth_enabled: boolean; google_client_id?: string }>('/auth/config');

export const login = (credential: string) =>
  apiFetch<{ user: { email: string; name: string; picture: string; role: string; locale?: string } }>(
    '/auth/google/callback',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ credential }),
    }
  );

export const getMe = () =>
  apiFetch<{ user: { email: string; name: string; picture: string; role: string; locale?: string } }>('/auth/me');

export const logout = () =>
  apiFetch<{ status: string }>('/auth/logout', { method: 'POST' });

// User locale
export const setUserLocale = (locale: string) =>
  apiFetch<{ locale: string }>('/api/v1/user/locale', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ locale }),
  });

// Admin user endpoints
export const getUsers = () => apiFetch<UsersResponse>('/api/v1/admin/users');

export const setUserRole = (email: string, role: string) =>
  apiFetch<{ status: string }>(`/api/v1/admin/users/${encodeURIComponent(email)}/role`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  });

export const deleteUser = (email: string) =>
  apiFetch<{ status: string }>(`/api/v1/admin/users/${encodeURIComponent(email)}`, {
    method: 'DELETE',
  });

// Accurate Online Integration Endpoints
export const getAccurateStatus = () =>
  apiFetch<AccurateStatus>('/api/v1/accurate/status');

export const accurateLogin = (payload: { challengeId?: string; otp?: string } = {}) =>
  apiFetch<{ status: string; challengeId?: string }>('/api/v1/accurate/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

export const accurateLogout = () =>
  apiFetch<{ success: boolean }>('/api/v1/accurate/logout', {
    method: 'POST',
  });

export const getAccurateDatabases = () =>
  apiFetch<AccurateDatabase[]>('/api/v1/accurate/databases');

export const selectAccurateDatabase = (payload: { id?: number; uniqueId?: string }) =>
  apiFetch<{ success: boolean; database: AccurateDatabase }>('/api/v1/accurate/database', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

export const searchAccurateItems = (params?: { q?: string; page?: number; pageSize?: number }) => {
  const query = qs(params);
  return apiFetch<AccurateSearchItemsResult>(`/api/v1/accurate/items${query ? '?' + query : ''}`);
};

export const getAccurateSources = () =>
  apiFetch<AccurateSourceInfo[]>('/api/v1/accurate/sources');

export const getAccurateSourceDocuments = (
  source: string,
  params?: { q?: string; from?: string; to?: string; page?: number; pageSize?: number }
) => {
  const query = qs(params);
  return apiFetch<AccurateDocumentsResult>(
    `/api/v1/accurate/sources/${encodeURIComponent(source)}/documents${query ? '?' + query : ''}`
  );
};

export const getAccurateSourceDocumentItems = (source: string, id: number | string) =>
  apiFetch<AccurateNormalizedItem[]>(
    `/api/v1/accurate/sources/${encodeURIComponent(source)}/documents/${encodeURIComponent(id)}/items`
  );

// Page Designs CRUD
export const getPageDesigns = () =>
  apiFetch<PageDesignsResponse>('/api/v1/page-designs');

export const createPageDesign = (design: Omit<PageDesign, 'id' | 'created_at' | 'updated_at'>) =>
  apiFetch<{ status: string; id: number }>('/api/v1/page-designs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(design),
  });

export const updatePageDesign = (id: number, design: Omit<PageDesign, 'id' | 'created_at' | 'updated_at'>) =>
  apiFetch<{ status: string }>(`/api/v1/page-designs/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(design),
  });

export const deletePageDesign = (id: number) =>
  apiFetch<{ status: string }>(`/api/v1/page-designs/${id}`, {
    method: 'DELETE',
  });

// Label Layout Presets CRUD
export const getLabelLayoutPresets = () =>
  apiFetch<LabelLayoutPresetsResponse>('/api/v1/label-layout-presets');

export const createLabelLayoutPreset = (preset: Pick<LabelLayoutPreset, 'name' | 'label_layout'>) =>
  apiFetch<{ status: string; id: number }>('/api/v1/label-layout-presets', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(preset),
  });

export const updateLabelLayoutPreset = (id: number, preset: Pick<LabelLayoutPreset, 'name' | 'label_layout'>) =>
  apiFetch<{ status: string }>(`/api/v1/label-layout-presets/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(preset),
  });

export const deleteLabelLayoutPreset = (id: number) =>
  apiFetch<{ status: string }>(`/api/v1/label-layout-presets/${id}`, { method: 'DELETE' });
