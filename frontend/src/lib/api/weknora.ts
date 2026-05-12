import { apiFetch, PaginationParams, PaginatedResponse, appendPagination } from './core';

// ── Market / Plugin API ───────────────────────────────────────

export interface MarketplacePlugin {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  icon_url: string;
  manifest_url: string;
  tags: string[];
  downloads: number;
  rating: number;
  created_at: string;
}

export interface InstalledPlugin {
  id: number;
  school_id: number;
  plugin_id: string;
  version: string;
  status: 'active' | 'inactive' | 'error';
  config: Record<string, unknown>;
  installed_at: string;
}

export async function listMarketplacePlugins(params?: {
  query?: string;
  category?: string;
  pg?: PaginationParams;
}): Promise<PaginatedResponse<MarketplacePlugin>> {
  const qs = new URLSearchParams();
  if (params?.query) qs.set('q', params.query);
  if (params?.category) qs.set('category', params.category);
  appendPagination(qs, params?.pg);

  const qsStr = qs.toString();
  return apiFetch<PaginatedResponse<MarketplacePlugin>>(`/marketplace/plugins${qsStr ? '?' + qsStr : ''}`);
}

export async function getMarketplacePlugin(pluginId: string): Promise<{ plugin: MarketplacePlugin }> {
  return apiFetch<{ plugin: MarketplacePlugin }>(`/marketplace/plugins/${pluginId}`);
}

export async function submitMarketplacePlugin(plugin: Partial<MarketplacePlugin>): Promise<{ plugin: MarketplacePlugin }> {
  return apiFetch<{ plugin: MarketplacePlugin }>('/marketplace/plugins', {
    method: 'POST',
    body: JSON.stringify(plugin),
  });
}

export async function installPlugin(schoolId: number, pluginId: string): Promise<{ installed: InstalledPlugin }> {
  return apiFetch<{ installed: InstalledPlugin }>(`/schools/${schoolId}/plugins`, {
    method: 'POST',
    body: JSON.stringify({ plugin_id: pluginId }),
  });
}

export async function uninstallPlugin(id: number): Promise<void> {
  return apiFetch<void>(`/schools/plugins/${id}`, {
    method: 'DELETE',
  });
}

export async function listInstalledPlugins(schoolId: number): Promise<InstalledPlugin[]> {
  return apiFetch<InstalledPlugin[]>(`/schools/${schoolId}/plugins`);
}

// ── WeKnora Knowledge Base API ──────────────────────────────

export interface WeKnoraKB {
  id: string;
  name: string;
  description: string;
  status: string;
  document_count: number;
  created_at: string;
}

export interface WeKnoraKBRef {
  id: number;
  course_id: number;
  weknora_kb_id: string;
  name: string;
  created_at: string;
}

export async function listWeKnoraKnowledgeBases(): Promise<WeKnoraKB[]> {
  return apiFetch<WeKnoraKB[]>('/weknora/kbs');
}

export async function getCourseWeKnoraRefs(courseId: number): Promise<WeKnoraKBRef[]> {
  return apiFetch<WeKnoraKBRef[]>(`/courses/${courseId}/weknora-refs`);
}

export async function bindWeKnoraKnowledgeBase(
  courseId: number,
  weknoraKbId: string,
  name: string
): Promise<WeKnoraKBRef> {
  return apiFetch<WeKnoraKBRef>(`/courses/${courseId}/weknora-refs`, {
    method: 'POST',
    body: JSON.stringify({ weknora_kb_id: weknoraKbId, name }),
  });
}

export async function unbindWeKnoraKnowledgeBase(courseId: number, refId: number): Promise<void> {
  return apiFetch<void>(`/courses/${courseId}/weknora-refs/${refId}`, {
    method: 'DELETE',
  });
}

export async function getWeKnoraLoginToken(): Promise<{ token: string; weknora_url: string }> {
  return apiFetch<{ token: string; weknora_url: string }>('/weknora/login-token', {
    method: 'POST',
  });
}
