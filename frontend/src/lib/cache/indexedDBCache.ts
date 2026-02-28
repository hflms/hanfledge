/**
 * L1 Client-Side IndexedDB Cache
 *
 * Per design.md §8.1.3: exact question fingerprint matching in browser
 * IndexedDB for same-student short-term repeated questions.
 * Target latency: < 10ms.
 *
 * Cache key = SHA-256 fingerprint of (sessionId + question text).
 * TTL = 24 hours by default; entries auto-expire on read.
 */

import { openDB, type IDBPDatabase } from 'idb';

// -- Constants ---------------------------------------------------

const DB_NAME = 'hanfledge-cache';
const DB_VERSION = 1;
const STORE_NAME = 'responses';
const DEFAULT_TTL_MS = 24 * 60 * 60 * 1000; // 24 hours

// -- Types -------------------------------------------------------

interface CachedEntry {
    /** SHA-256 hex fingerprint */
    key: string;
    /** Original question text (for debugging) */
    question: string;
    /** Cached coach response */
    response: string;
    /** Session ID this entry belongs to */
    sessionId: number;
    /** Timestamp when this entry was created (ms) */
    createdAt: number;
    /** Timestamp when this entry expires (ms) */
    expiresAt: number;
}

// -- Fingerprint Utility -----------------------------------------

/**
 * Generate a SHA-256 hex fingerprint from sessionId + question text.
 * Falls back to a simple hash if crypto.subtle is unavailable.
 */
async function fingerprint(sessionId: number, question: string): Promise<string> {
    const raw = `${sessionId}:${question.trim().toLowerCase()}`;

    if (typeof crypto !== 'undefined' && crypto.subtle) {
        const encoder = new TextEncoder();
        const data = encoder.encode(raw);
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    }

    // Fallback: simple djb2 hash (non-crypto environments)
    let hash = 5381;
    for (let i = 0; i < raw.length; i++) {
        hash = ((hash << 5) + hash + raw.charCodeAt(i)) & 0xffffffff;
    }
    return `djb2-${(hash >>> 0).toString(16)}`;
}

// -- Database Initialization -------------------------------------

let dbPromise: Promise<IDBPDatabase> | null = null;

function getDB(): Promise<IDBPDatabase> {
    if (!dbPromise) {
        dbPromise = openDB(DB_NAME, DB_VERSION, {
            upgrade(db) {
                if (!db.objectStoreNames.contains(STORE_NAME)) {
                    const store = db.createObjectStore(STORE_NAME, { keyPath: 'key' });
                    store.createIndex('by-session', 'sessionId');
                    store.createIndex('by-expiry', 'expiresAt');
                }
            },
        });
    }
    return dbPromise;
}

// -- Public API --------------------------------------------------

/**
 * Look up a cached response for the given question in the given session.
 * Returns `null` if not found or expired.
 */
export async function getCachedResponse(
    sessionId: number,
    question: string,
): Promise<string | null> {
    try {
        const db = await getDB();
        const key = await fingerprint(sessionId, question);
        const entry: CachedEntry | undefined = await db.get(STORE_NAME, key);

        if (!entry) return null;

        // Check TTL expiration
        if (Date.now() > entry.expiresAt) {
            // Lazy delete expired entry
            await db.delete(STORE_NAME, key);
            return null;
        }

        return entry.response;
    } catch (err) {
        console.warn('[L1 Cache] getCachedResponse error:', err);
        return null;
    }
}

/**
 * Store a question→response pair in the cache.
 */
export async function setCachedResponse(
    sessionId: number,
    question: string,
    response: string,
    ttlMs: number = DEFAULT_TTL_MS,
): Promise<void> {
    try {
        const db = await getDB();
        const key = await fingerprint(sessionId, question);
        const entry: CachedEntry = {
            key,
            question: question.trim(),
            response,
            sessionId,
            createdAt: Date.now(),
            expiresAt: Date.now() + ttlMs,
        };
        await db.put(STORE_NAME, entry);
    } catch (err) {
        console.warn('[L1 Cache] setCachedResponse error:', err);
    }
}

/**
 * Clear all cache entries for a specific session.
 */
export async function clearSessionCache(sessionId: number): Promise<void> {
    try {
        const db = await getDB();
        const tx = db.transaction(STORE_NAME, 'readwrite');
        const index = tx.store.index('by-session');
        let cursor = await index.openCursor(IDBKeyRange.only(sessionId));

        while (cursor) {
            await cursor.delete();
            cursor = await cursor.continue();
        }

        await tx.done;
    } catch (err) {
        console.warn('[L1 Cache] clearSessionCache error:', err);
    }
}

/**
 * Clear ALL cache entries (full invalidation).
 */
export async function clearAllCache(): Promise<void> {
    try {
        const db = await getDB();
        await db.clear(STORE_NAME);
    } catch (err) {
        console.warn('[L1 Cache] clearAllCache error:', err);
    }
}

/**
 * Purge expired entries from the cache.
 * Call periodically (e.g., on app startup) to reclaim storage.
 */
export async function purgeExpiredEntries(): Promise<number> {
    try {
        const db = await getDB();
        const tx = db.transaction(STORE_NAME, 'readwrite');
        const index = tx.store.index('by-expiry');
        const now = Date.now();
        let purged = 0;

        let cursor = await index.openCursor(IDBKeyRange.upperBound(now));
        while (cursor) {
            await cursor.delete();
            purged++;
            cursor = await cursor.continue();
        }

        await tx.done;
        return purged;
    } catch (err) {
        console.warn('[L1 Cache] purgeExpiredEntries error:', err);
        return 0;
    }
}

/**
 * Get cache statistics for debugging / monitoring.
 */
export async function getCacheStats(): Promise<{
    totalEntries: number;
    expiredEntries: number;
}> {
    try {
        const db = await getDB();
        const all = await db.getAll(STORE_NAME);
        const now = Date.now();
        const expired = all.filter((e: CachedEntry) => now > e.expiresAt).length;
        return { totalEntries: all.length, expiredEntries: expired };
    } catch (err) {
        console.warn('[L1 Cache] getCacheStats error:', err);
        return { totalEntries: 0, expiredEntries: 0 };
    }
}
