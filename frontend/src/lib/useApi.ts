import useSWR, { type SWRConfiguration } from 'swr';
import { apiFetch } from './api';

/**
 * Global fetcher for SWR, hooked up to apiFetch.
 */
export const swrFetcher = async <Data>(url: string): Promise<Data> => {
    return apiFetch<Data>(url);
};

/**
 * Hook for fetching data via SWR.
 * @param url The API endpoint (e.g. '/courses') or null if not ready to fetch.
 * @param options SWR configuration options.
 */
export function useApi<Data = unknown, Error = unknown>(
    url: string | null,
    options?: SWRConfiguration<Data, Error>
) {
    return useSWR<Data, Error>(url, swrFetcher<Data>, options);
}
