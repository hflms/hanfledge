/**
 * Generates a secure random ID using the Web Crypto API.
 * @param prefix An optional prefix for the ID.
 * @returns A string representing a secure random ID.
 */
export function generateId(prefix?: string): string {
    const uuid = crypto.randomUUID();
    return prefix ? `${prefix}-${uuid}` : uuid;
}
