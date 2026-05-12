import { apiFetch, PaginationParams, PaginatedResponse, appendPagination } from './core';
import { MountedSkill } from './course';

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

// ── AI Auto-Mount API ───────────────────────────────────────

export interface RecommendMount {
  kp_id: number;
  kp_title: string;
  skill_id: string;
  skill_name: string;
  scaffold_level: string;
  reason: string;
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

// ── Custom Skill API (design.md §6.4) ──────────────────────

export type CustomSkillStatus = 'draft' | 'published' | 'shared' | 'archived';
export type CustomSkillVisibility = 'private' | 'school' | 'platform';

export interface CustomSkillTemplate {
  name: string;
  description: string;
  example_markdown: string;
}

export interface CustomSkill {
  id: number;
  creator_id: number;
  school_id?: number;
  skill_id: string; // The generated UUID
  name: string;
  description: string;
  category: string;
  subjects: string[];
  scaffolding_levels: string[];
  system_prompt_markdown: string;
  version: string;
  status: CustomSkillStatus;
  visibility: CustomSkillVisibility;
  tags?: string[];
  created_at: string;
  updated_at: string;
}

export interface CustomSkillVersion {
  id: number;
  custom_skill_id: number;
  version: string;
  system_prompt_markdown: string;
  commit_message: string;
  created_at: string;
}

export interface CreateCustomSkillData {
  name: string;
  description: string;
  category: string;
  subjects: string[];
  scaffolding_levels: string[];
  system_prompt_markdown: string;
  tags?: string[];
  visibility?: CustomSkillVisibility;
}

export interface UpdateCustomSkillData {
  name?: string;
  description?: string;
  category?: string;
  subjects?: string[];
  scaffolding_levels?: string[];
  system_prompt_markdown?: string;
  tags?: string[];
  commit_message?: string;
}

export async function createCustomSkill(data: CreateCustomSkillData): Promise<CustomSkill> {
  return apiFetch<CustomSkill>('/custom-skills', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function listCustomSkills(opts?: {
  status?: string;
  visibility?: string;
  category?: string;
  pg?: PaginationParams;
}): Promise<PaginatedResponse<CustomSkill>> {
  const params = new URLSearchParams();
  if (opts?.status) params.set('status', opts.status);
  if (opts?.visibility) params.set('visibility', opts.visibility);
  if (opts?.category) params.set('category', opts.category);
  appendPagination(params, opts?.pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<CustomSkill>>(`/custom-skills${qs ? '?' + qs : ''}`);
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
