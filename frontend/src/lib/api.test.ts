import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { apiFetch, getToken, setToken, clearToken, getLiveMonitor, getActivityLiveDetail } from './api';

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

// -- Live Monitor API Functions ------------------------------------

describe('getLiveMonitor', () => {
  it('calls the correct endpoint with course_id', async () => {
    setToken('test-token');
    const mockData = {
      course_id: 1,
      timestamp: '2026-03-31T12:00:00Z',
      activities: [
        {
          activity_id: 10,
          activity_title: 'Physics Lab',
          activity_status: 'active',
          total_students: 30,
          active_students: 25,
          completed_students: 5,
          avg_mastery: 0.72,
          avg_duration_min: 15.5,
        },
      ],
    };
    const fetchSpy = mockFetchResponse(200, mockData);
    vi.stubGlobal('fetch', fetchSpy);

    const result = await getLiveMonitor(1);

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/dashboard/live-monitor?course_id=1');
    expect(result.course_id).toBe(1);
    expect(result.activities).toHaveLength(1);
    expect(result.activities[0].activity_title).toBe('Physics Lab');
  });

  it('propagates errors from apiFetch', async () => {
    setToken('test-token');
    const fetchSpy = mockFetchResponse(500, { error: '服务器错误' });
    vi.stubGlobal('fetch', fetchSpy);

    await expect(getLiveMonitor(1)).rejects.toThrow('服务器错误');
  });
});

describe('getActivityLiveDetail', () => {
  it('calls the correct endpoint with activity_id', async () => {
    setToken('test-token');
    const mockData = {
      activity_id: 10,
      title: 'Physics Lab',
      kp_sequence: [{ kp_id: 1, kp_title: 'Newton Laws' }],
      steps: [],
      alerts: [],
      timestamp: '2026-03-31T12:00:00Z',
    };
    const fetchSpy = mockFetchResponse(200, mockData);
    vi.stubGlobal('fetch', fetchSpy);

    const result = await getActivityLiveDetail(10);

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/dashboard/activities/10/live');
    expect(result.activity_id).toBe(10);
    expect(result.title).toBe('Physics Lab');
    expect(result.kp_sequence).toHaveLength(1);
  });

  it('returns alerts when students are struggling', async () => {
    setToken('test-token');
    const mockData = {
      activity_id: 10,
      title: 'Physics Lab',
      kp_sequence: [],
      steps: [],
      alerts: [
        {
          student_id: 5,
          student_name: 'Alice',
          session_id: 100,
          alert_type: 'stuck',
          message: '停滞超过 5 分钟',
        },
      ],
      timestamp: '2026-03-31T12:00:00Z',
    };
    const fetchSpy = mockFetchResponse(200, mockData);
    vi.stubGlobal('fetch', fetchSpy);

    const result = await getActivityLiveDetail(10);

    expect(result.alerts).toHaveLength(1);
    expect(result.alerts[0].alert_type).toBe('stuck');
    expect(result.alerts[0].student_name).toBe('Alice');
  });
});
