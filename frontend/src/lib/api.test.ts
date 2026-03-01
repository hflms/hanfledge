import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { apiFetch, getToken, setToken, clearToken } from './api';

// -- Helpers -------------------------------------------------------

function mockFetchResponse(status: number, body: unknown, headers?: Record<string, string>) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 401 ? 'Unauthorized' : 'OK',
    json: () => Promise.resolve(body),
    headers: new Headers(headers),
  } as Response);
}

// -- Setup ---------------------------------------------------------

beforeEach(() => {
  localStorage.clear();
  // Prevent actual navigation in 401 tests
  Object.defineProperty(window, 'location', {
    writable: true,
    value: { href: '' },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

// -- Token Management ----------------------------------------------

describe('Token management', () => {
  it('getToken returns null when no token is stored', () => {
    expect(getToken()).toBeNull();
  });

  it('setToken stores and getToken retrieves the token', () => {
    setToken('my-jwt');
    expect(getToken()).toBe('my-jwt');
  });

  it('clearToken removes the token', () => {
    setToken('my-jwt');
    clearToken();
    expect(getToken()).toBeNull();
  });
});

// -- apiFetch -------------------------------------------------------

describe('apiFetch', () => {
  it('attaches Authorization header when token exists', async () => {
    setToken('test-token');
    const fetchSpy = mockFetchResponse(200, { data: 'ok' });
    vi.stubGlobal('fetch', fetchSpy);

    await apiFetch('/test');

    const [, init] = fetchSpy.mock.calls[0];
    expect(init.headers['Authorization']).toBe('Bearer test-token');
  });

  it('does not attach Authorization header when no token', async () => {
    const fetchSpy = mockFetchResponse(200, { data: 'ok' });
    vi.stubGlobal('fetch', fetchSpy);

    await apiFetch('/test');

    const [, init] = fetchSpy.mock.calls[0];
    expect(init.headers['Authorization']).toBeUndefined();
  });

  it('sets Content-Type to application/json for non-FormData', async () => {
    const fetchSpy = mockFetchResponse(200, { ok: true });
    vi.stubGlobal('fetch', fetchSpy);

    await apiFetch('/test', { method: 'POST', body: JSON.stringify({ a: 1 }) });

    const [, init] = fetchSpy.mock.calls[0];
    expect(init.headers['Content-Type']).toBe('application/json');
  });

  it('does not set Content-Type for FormData', async () => {
    const fetchSpy = mockFetchResponse(200, { ok: true });
    vi.stubGlobal('fetch', fetchSpy);

    const form = new FormData();
    form.append('file', 'test');
    await apiFetch('/upload', { method: 'POST', body: form });

    const [, init] = fetchSpy.mock.calls[0];
    expect(init.headers['Content-Type']).toBeUndefined();
  });

  it('returns parsed JSON on success', async () => {
    const fetchSpy = mockFetchResponse(200, { items: [1, 2, 3] });
    vi.stubGlobal('fetch', fetchSpy);

    const result = await apiFetch<{ items: number[] }>('/data');
    expect(result).toEqual({ items: [1, 2, 3] });
  });

  it('clears token and redirects to /login on 401', async () => {
    setToken('expired-token');
    const fetchSpy = mockFetchResponse(401, { error: 'unauthorized' });
    vi.stubGlobal('fetch', fetchSpy);

    await expect(apiFetch('/protected')).rejects.toThrow('认证已过期');
    expect(getToken()).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('throws error with server message on non-ok response', async () => {
    const fetchSpy = mockFetchResponse(400, { error: '参数错误' });
    vi.stubGlobal('fetch', fetchSpy);

    await expect(apiFetch('/bad')).rejects.toThrow('参数错误');
  });

  it('throws fallback error when server returns non-JSON error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('not json')),
    } as unknown as Response));

    // The code does: err.error || `请求失败 (${res.status})`
    // When json fails, catch returns { error: res.statusText } = { error: 'Internal Server Error' }
    // So err.error = 'Internal Server Error'
    await expect(apiFetch('/error')).rejects.toThrow('Internal Server Error');
  });
});
