/**
 * Modular API client - re-exports for backward compatibility.
 * 
 * Migration strategy:
 * 1. Keep original api.ts as-is
 * 2. Gradually extract modules (auth, admin, course, etc.)
 * 3. Update imports in components one by one
 * 4. Remove api.ts when all imports migrated
 */

export * from './core';
export * from './auth';
