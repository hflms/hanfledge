import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { clearToken } from './core';

describe('clearToken', () => {
  const originalWindow = global.window;

  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    global.window = originalWindow;
  });

  it('removes the token from localStorage when window is defined', () => {
    // Note: setToken currently does not check for window in core.ts
    // but in test environment window is typically defined by vitest/jsdom.
    // For this test we assume window is defined.
    localStorage.setItem('hanfledge_token', 'my-test-token');
    expect(localStorage.getItem('hanfledge_token')).toBe('my-test-token');

    clearToken();

    expect(localStorage.getItem('hanfledge_token')).toBeNull();
  });

  it('does nothing gracefully when window is undefined', () => {
    // Set a token first
    localStorage.setItem('hanfledge_token', 'my-test-token');

    // Temporarily set window to undefined
    const originalWindow = global.window;
    // @ts-expect-error test purpose
    delete global.window;

    try {
      // This should not throw and should not call localStorage.removeItem
      // because in Node-like environment without window, it might not exist
      // or we just want to ensure our safeguard works.
      const removeItemSpy = vi.spyOn(localStorage, 'removeItem');

      expect(() => clearToken()).not.toThrow();
      expect(removeItemSpy).not.toHaveBeenCalled();

      // Token should still be in localStorage
      expect(localStorage.getItem('hanfledge_token')).toBe('my-test-token');
    } finally {
      // Restore window
      global.window = originalWindow;
    }
  });
});
