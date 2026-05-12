/**
 * Core API utilities: token management, authenticated fetch, and shared helpers.
 */

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';



/**
 * Gets the stored JWT token from localStorage.
 */
export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('hanfledge_token');
}


/**
 * Stores the JWT token to localStorage.
 */
export function setToken(token: string): void {
  localStorage.setItem('hanfledge_token', token);
}


/**
 * Removes the JWT token from localStorage.
 */
export function clearToken(): void {
  localStorage.removeItem('hanfledge_token');
}


/**
 * Makes an authenticated API request.
 * Automatically attaches JWT token and handles common errors.
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  // Don't set Content-Type for FormData (browser sets boundary automatically)
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    clearToken();
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new Error('认证已过期，请重新登录');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `请求失败 (${res.status})`);
  }

  return res.json();
}


// ── Auth API ────────────────────────────────────────────────

// -- Pagination Types -----------------------------------------

/** Standard paginated response from the backend. */
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}


/** Common pagination query parameters. */
export interface PaginationParams {
  page?: number;
  limit?: number;
}


/** Appends pagination params to a URLSearchParams object. */
export function appendPagination(params: URLSearchParams, pg?: PaginationParams): void {
  if (pg?.page) params.set('page', String(pg.page));
  if (pg?.limit) params.set('limit', String(pg.limit));
}


// ── Data Export API ─────────────────────────────────────────

/**
 * Triggers a CSV file download by fetching an export endpoint.
 * The response is a CSV blob; this export function creates a temporary
 * download link and clicks it programmatically.
 */
export async function downloadCSV(path: string, fallbackFilename: string): Promise<void> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, { headers });

  if (res.status === 401) {
    clearToken();
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new Error('认证已过期，请重新登录');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `导出失败 (${res.status})`);
  }

  // Extract filename from Content-Disposition if available
  const disposition = res.headers.get('Content-Disposition');
  let filename = fallbackFilename;
  if (disposition) {
    const match = disposition.match(/filename=(.+)/);
    if (match) filename = match[1];
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
