import { describe, it, expect, vi } from 'vitest';
import { swrFetcher } from './useApi';

// Mock the apiFetch function
vi.mock('./api', () => ({
  apiFetch: vi.fn(),
}));

import { apiFetch } from './api';

describe('swrFetcher', () => {
  it('calls apiFetch with the provided URL', async () => {
    const mockData = { items: [1, 2, 3] };
    vi.mocked(apiFetch).mockResolvedValue(mockData);

    const result = await swrFetcher<typeof mockData>('/courses');

    expect(apiFetch).toHaveBeenCalledWith('/courses');
    expect(result).toEqual(mockData);
  });

  it('propagates errors from apiFetch', async () => {
    vi.mocked(apiFetch).mockRejectedValue(new Error('认证已过期'));

    await expect(swrFetcher('/protected')).rejects.toThrow('认证已过期');
  });

  it('passes through the generic type correctly', async () => {
    const mockData = { course_id: 1, title: 'Physics' };
    vi.mocked(apiFetch).mockResolvedValue(mockData);

    const result = await swrFetcher<{ course_id: number; title: string }>('/courses/1');

    expect(result.course_id).toBe(1);
    expect(result.title).toBe('Physics');
  });
});
