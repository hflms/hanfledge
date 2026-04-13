import { apiFetch, type PaginatedResponse } from './core';



export interface MountedSkill {
  id: number;
  kp_id: number;
  skill_id: string;
  scaffold_level: string;
  progressive_rule?: ProgressiveRule | null;
}


export interface ProgressiveRule {
  mastery_threshold?: number;
  degrade_to?: string;
}


// ── Skill API ───────────────────────────────────────────────

export interface SkillToolConfig {
  enabled: boolean;
  description: string;
}


export interface SkillProgressiveTriggers {
  activate_when?: string;
  deactivate_when?: string;
}


export interface SkillMetadata {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  category: string;
  subjects: string[];
  tags: string[];
  scaffolding_levels: string[];
  constraints: Record<string, unknown>;
  tools?: Record<string, SkillToolConfig>;
  progressive_triggers?: SkillProgressiveTriggers;
  evaluation_dimensions?: string[];
}


export interface SkillConstraints {
  skill_id: string;
  raw_markdown: string;
}


export interface SkillDetail {
  metadata: SkillMetadata;
  constraints: SkillConstraints | null;
}


export interface MountSkillResponse {
  message: string;
  count: number;
  mounts: MountedSkill[];
}


// ── AI Auto-Mount API ───────────────────────────────────────

export interface RecommendMount {
  kp_id: number;
  kp_title: string;
  skill_id: string;
  skill_name: string;
  scaffold_level: string;
  reason: string;
}


// ── Custom Skill API (design.md §6.4) ──────────────────────

export type CustomSkillStatus = 'draft' | 'published' | 'shared' | 'archived';

export type CustomSkillVisibility = 'private' | 'school' | 'platform';


export interface CustomSkillTemplate {
  id: string;
  file_name: string;
  content: string;
}


export interface CustomSkill {
  id: number;
  skill_id: string;
  teacher_id: number;
  school_id: number;
  name: string;
  description: string;
  category: string;
  subjects: string;      // JSON array string
  tags: string;           // JSON array string
  skill_md: string;
  tools_config: string;   // JSON object string
  templates: string;      // JSON array string
  status: CustomSkillStatus;
  visibility: CustomSkillVisibility;
  version: number;
  usage_count: number;
  created_at: string;
  updated_at: string;
}


export interface CustomSkillVersion {
  id: number;
  custom_skill_id: number;
  version: number;
  skill_md: string;
  tools_config: string;
  templates: string;
  change_log: string;
  created_at: string;
}


export interface CreateCustomSkillData {
  skill_id: string;
  name: string;
  description?: string;
  category?: string;
  subjects?: string[];
  tags?: string[];
  skill_md: string;
  tools_config?: Record<string, SkillToolConfig>;
  templates?: CustomSkillTemplate[];
}


export interface UpdateCustomSkillData {
  name?: string;
  description?: string;
  category?: string;
  subjects?: string[];
  tags?: string[];
  skill_md?: string;
  tools_config?: Record<string, SkillToolConfig>;
  templates?: CustomSkillTemplate[];
  change_log?: string;
}


export async function listSkills(subject?: string, category?: string): Promise<SkillMetadata[]> {
  const params = new URLSearchParams();
  if (subject) params.set('subject', subject);
  if (category) params.set('category', category);
  const qs = params.toString();
  return apiFetch<SkillMetadata[]>(`/skills${qs ? '?' + qs : ''}`);
}


export async function getSkillDetail(skillId: string): Promise<SkillDetail> {
  return apiFetch<SkillDetail>(`/skills/${skillId}`);
}


export async function mountSkill(chapterId: number, data: {
  skill_id: string;
  scaffold_level?: string;
  constraints_json?: Record<string, unknown>;
  priority?: number;
}): Promise<MountSkillResponse> {
  return apiFetch<MountSkillResponse>(`/chapters/${chapterId}/skills`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}


export async function unmountSkill(chapterId: number, mountId: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/chapters/${chapterId}/skills/${mountId}`, {
    method: 'DELETE',
  });
}


export async function updateSkillConfig(chapterId: number, mountId: number, data: {
  scaffold_level?: string;
  progressive_rule?: Record<string, unknown>;
}): Promise<{ message: string; mount: MountedSkill }> {
  return apiFetch<{ message: string; mount: MountedSkill }>(`/chapters/${chapterId}/skills/${mountId}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}


export async function mountSkillToKP(kpId: number, data: {
  skill_id: string;
  scaffold_level?: string;
  constraints_json?: Record<string, unknown>;
  priority?: number;
}): Promise<MountSkillResponse> {
  return apiFetch<MountSkillResponse>(`/knowledge-points/${kpId}/skills`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}


export async function unmountSkillFromKP(kpId: number, mountId: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/knowledge-points/${kpId}/skills/${mountId}`, {
    method: 'DELETE',
  });
}


export async function updateKPSkillConfig(kpId: number, mountId: number, data: {
  scaffold_level?: string;
  progressive_rule?: Record<string, unknown>;
}): Promise<{ message: string; mount: MountedSkill }> {
  return apiFetch<{ message: string; mount: MountedSkill }>(`/knowledge-points/${kpId}/skills/${mountId}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}


export async function recommendSkills(courseId: number): Promise<{ recommendations: RecommendMount[] }> {
  return apiFetch<{ recommendations: RecommendMount[] }>(`/courses/${courseId}/skills/recommend`, {
    method: 'POST',
  });
}


export async function batchMountSkills(courseId: number, mounts: { kp_id: number; skill_id: string; scaffold_level: string }[]): Promise<{ message: string; count: number }> {
  return apiFetch<{ message: string; count: number }>(`/courses/${courseId}/skills/batch-mount`, {
    method: 'POST',
    body: JSON.stringify({ mounts }),
  });
}


export async function createCustomSkill(data: CreateCustomSkillData): Promise<CustomSkill> {
  return apiFetch<CustomSkill>('/custom-skills', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}


export async function listCustomSkills(opts?: {
  status?: CustomSkillStatus;
  visibility?: CustomSkillVisibility;
}): Promise<CustomSkill[]> {
  const params = new URLSearchParams();
  if (opts?.status) params.set('status', opts.status);
  if (opts?.visibility) params.set('visibility', opts.visibility);
  const qs = params.toString();
  return apiFetch<CustomSkill[]>(`/custom-skills${qs ? '?' + qs : ''}`);
}


export async function getCustomSkill(id: number): Promise<CustomSkill> {
  return apiFetch<CustomSkill>(`/custom-skills/${id}`);
}


export async function updateCustomSkill(id: number, data: UpdateCustomSkillData): Promise<CustomSkill> {
  return apiFetch<CustomSkill>(`/custom-skills/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}


export async function deleteCustomSkill(id: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/custom-skills/${id}`, {
    method: 'DELETE',
  });
}


export async function publishCustomSkill(id: number): Promise<{ message: string; skill: CustomSkill }> {
  return apiFetch<{ message: string; skill: CustomSkill }>(`/custom-skills/${id}/publish`, {
    method: 'POST',
  });
}


export async function shareCustomSkill(id: number, visibility: 'school' | 'platform'): Promise<{ message: string; skill: CustomSkill }> {
  return apiFetch<{ message: string; skill: CustomSkill }>(`/custom-skills/${id}/share`, {
    method: 'POST',
    body: JSON.stringify({ visibility }),
  });
}


export async function archiveCustomSkill(id: number): Promise<{ message: string; skill: CustomSkill }> {
  return apiFetch<{ message: string; skill: CustomSkill }>(`/custom-skills/${id}/archive`, {
    method: 'POST',
  });
}


export async function listCustomSkillVersions(id: number): Promise<CustomSkillVersion[]> {
  return apiFetch<CustomSkillVersion[]>(`/custom-skills/${id}/versions`);
}


// -- Marketplace API -----------------------------------------------

export interface MarketplacePlugin {
  id: number;
  plugin_id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  type: string;
  trust_level: string;
  category: string;
  tags: string;
  icon_url?: string;
  downloads: number;
  rating: number;
  rating_count: number;
  status: string;
}


export interface InstalledPlugin {
  id: number;
  school_id: number;
  plugin_id: string;
  version: string;
  enabled: boolean;
}


export async function listMarketplacePlugins(params?: {
  type?: string;
  category?: string;
  q?: string;
  page?: number;
  limit?: number;
}): Promise<PaginatedResponse<MarketplacePlugin>> {
  const searchParams = new URLSearchParams();
  if (params?.type) searchParams.set('type', params.type);
  if (params?.category) searchParams.set('category', params.category);
  if (params?.q) searchParams.set('q', params.q);
  if (params?.page) searchParams.set('page', String(params.page));
  if (params?.limit) searchParams.set('limit', String(params.limit));
  return apiFetch(`/marketplace/plugins?${searchParams.toString()}`);
}


export async function getMarketplacePlugin(pluginId: string): Promise<{ plugin: MarketplacePlugin }> {
  return apiFetch(`/marketplace/plugins/${pluginId}`);
}


export async function submitMarketplacePlugin(plugin: Partial<MarketplacePlugin>): Promise<{ plugin: MarketplacePlugin }> {
  return apiFetch('/marketplace/plugins', {
    method: 'POST',
    body: JSON.stringify(plugin),
  });
}


export async function installPlugin(schoolId: number, pluginId: string): Promise<{ installed: InstalledPlugin }> {
  return apiFetch('/marketplace/install', {
    method: 'POST',
    body: JSON.stringify({ school_id: schoolId, plugin_id: pluginId }),
  });
}


export async function uninstallPlugin(id: number): Promise<void> {
  return apiFetch(`/marketplace/installed/${id}`, { method: 'DELETE' });
}


export async function listInstalledPlugins(schoolId: number): Promise<InstalledPlugin[]> {
  return apiFetch(`/marketplace/installed?school_id=${schoolId}`);
}
