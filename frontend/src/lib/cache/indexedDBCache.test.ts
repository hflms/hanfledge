import { describe, it, expect, beforeEach } from 'vitest';
import {
  getCachedResponse,
  setCachedResponse,
  clearSessionCache,
  clearAllCache,
  purgeExpiredEntries,
  getCacheStats,
} from './indexedDBCache';

// -- Setup ---------------------------------------------------------
// fake-indexeddb is auto-registered in test/setup.ts

beforeEach(async () => {
  await clearAllCache();
});

// -- Tests ---------------------------------------------------------

describe('indexedDBCache', () => {
  it('returns null for cache miss', async () => {
    const result = await getCachedResponse(1, 'unknown question');
    expect(result).toBeNull();
  });

  it('stores and retrieves a cached response', async () => {
    await setCachedResponse(1, 'What is 2+2?', 'The answer is 4.');
    const result = await getCachedResponse(1, 'What is 2+2?');
    expect(result).toBe('The answer is 4.');
  });

  it('normalizes question text (trim + lowercase)', async () => {
    await setCachedResponse(1, '  Hello World  ', 'response');
    const result = await getCachedResponse(1, 'hello world');
    expect(result).toBe('response');
  });

  it('scopes cache by session ID', async () => {
    await setCachedResponse(1, 'question', 'answer-1');
    await setCachedResponse(2, 'question', 'answer-2');

    expect(await getCachedResponse(1, 'question')).toBe('answer-1');
    expect(await getCachedResponse(2, 'question')).toBe('answer-2');
  });

  it('returns null for expired entry', async () => {
    // Set with 1ms TTL — will expire immediately
    await setCachedResponse(1, 'expiring', 'old-answer', 1);
    // Wait a tiny bit for expiry
    await new Promise((r) => setTimeout(r, 10));

    const result = await getCachedResponse(1, 'expiring');
    expect(result).toBeNull();
  });

  it('clearSessionCache removes entries for specific session', async () => {
    await setCachedResponse(1, 'q1', 'a1');
    await setCachedResponse(1, 'q2', 'a2');
    await setCachedResponse(2, 'q1', 'a3');

    await clearSessionCache(1);

    expect(await getCachedResponse(1, 'q1')).toBeNull();
    expect(await getCachedResponse(1, 'q2')).toBeNull();
    expect(await getCachedResponse(2, 'q1')).toBe('a3');
  });

  it('clearAllCache removes everything', async () => {
    await setCachedResponse(1, 'q1', 'a1');
    await setCachedResponse(2, 'q2', 'a2');

    await clearAllCache();

    expect(await getCachedResponse(1, 'q1')).toBeNull();
    expect(await getCachedResponse(2, 'q2')).toBeNull();
  });

  it('purgeExpiredEntries removes only expired entries', async () => {
    await setCachedResponse(1, 'valid', 'still-good', 60_000);
    await setCachedResponse(1, 'expired', 'gone', 1);
    await new Promise((r) => setTimeout(r, 10));

    const purged = await purgeExpiredEntries();
    expect(purged).toBe(1);

    expect(await getCachedResponse(1, 'valid')).toBe('still-good');
    expect(await getCachedResponse(1, 'expired')).toBeNull();
  });

  it('getCacheStats returns correct counts', async () => {
    await setCachedResponse(1, 'q1', 'a1', 60_000);
    await setCachedResponse(1, 'q2', 'a2', 1);
    await new Promise((r) => setTimeout(r, 10));

    const stats = await getCacheStats();
    expect(stats.totalEntries).toBe(2);
    expect(stats.expiredEntries).toBe(1);
  });
});
