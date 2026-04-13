import { apiFetch, PaginationParams, PaginatedResponse, appendPagination } from './core';
import { Course } from './course';

// ── Student Activity API ────────────────────────────────────

export interface LearningActivity {
  id: number;
  course_id: number;
  title: string;
  type: 'autonomous' | 'guided';
  status: 'draft' | 'published' | 'archived';
  designer_id?: string;
  system_prompt?: string;
  created_at: string;
  course?: Course;
  steps?: ActivityStep[];
  assignments?: ActivityClassAssignment[];
}

export type ContentBlockType = 'markdown' | 'file' | 'video' | 'image';

export type StepType = 'lecture' | 'discussion' | 'quiz' | 'practice' | 'reading' | 'group_work' | 'reflection' | 'ai_tutoring';

export interface ContentBlock {
  type: ContentBlockType;
  content: string;
  meta?: Record<string, unknown>;
}

export interface ActivityStep {
  id: number;
  activity_id: number;
  kp_id: number | null;
  step_type: StepType;
  title: string;
  content_blocks: ContentBlock[];
  sort_order: number;
  active_skill?: string;
}

export interface ActivityClassAssignment {
  id: number;
  activity_id: number;
  class_id: number;
  status: string;
}

export interface UploadAssetResponse {
  url: string;
  type: string;
  size: number;
  name: string;
}

export interface StudentSession {
  id: number;
  student_id: number;
  activity_id: number;
  start_time: string;
  end_time?: string;
  status: string;
  current_step_id?: number;
  current_kp_id?: number;
  is_sandbox?: boolean;
}

export interface Interaction {
  id: number;
  session_id: number;
  sender_type: 'student' | 'agent' | 'system' | 'teacher';
  content: string;
  timestamp: string;
  skill_used?: string;
  metrics?: Record<string, unknown>;
}

export interface SessionDetail {
  session: StudentSession;
  activity: LearningActivity;
  recent_interactions: Interaction[];
}

export async function listStudentActivities(): Promise<LearningActivity[]> {
  return apiFetch<LearningActivity[]>('/student/activities');
}

export async function joinActivity(activityId: number): Promise<{ message: string; session_id: number }> {
  return apiFetch<{ message: string; session_id: number }>(`/student/activities/${activityId}/join`, {
    method: 'POST',
  });
}

export async function previewActivity(activityId: number): Promise<{ message: string; session_id: number; is_sandbox: boolean }> {
  return apiFetch<{ message: string; session_id: number; is_sandbox: boolean }>(`/activities/${activityId}/preview`, {
    method: 'POST',
  });
}

export async function getSession(sessionId: number): Promise<SessionDetail> {
  return apiFetch<SessionDetail>(`/student/sessions/${sessionId}`);
}

export interface StepTransitionResult {
  message: string;
  new_step_id?: number;
  new_kp_id?: number;
}

export async function updateSessionStep(sessionId: number, kpId: number, activeSkill: string): Promise<StepTransitionResult> {
  return apiFetch<StepTransitionResult>(`/student/sessions/${sessionId}/step`, {
    method: 'PUT',
    body: JSON.stringify({ current_kp_id: kpId, active_skill: activeSkill }),
  });
}

// ── Teacher Activity Management API ───────────────────────────

export async function listTeacherActivities(courseId?: number, pg?: PaginationParams): Promise<PaginatedResponse<LearningActivity>> {
  const params = new URLSearchParams();
  if (courseId) params.set('course_id', String(courseId));
  appendPagination(params, pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<LearningActivity>>(`/activities${qs ? '?' + qs : ''}`);
}

export async function createActivity(data: {
  course_id: number;
  title: string;
  type: 'autonomous' | 'guided';
  designer_id?: string;
  system_prompt?: string;
}): Promise<LearningActivity> {
  return apiFetch<LearningActivity>('/activities', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export interface InstructionalDesigner {
  id: string;
  name: string;
  description: string;
  version: string;
}

export async function listDesigners(): Promise<InstructionalDesigner[]> {
  return apiFetch<InstructionalDesigner[]>('/designers');
}

export async function publishActivity(activityId: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/activities/${activityId}/publish`, {
    method: 'POST',
  });
}

export async function getActivity(activityId: number): Promise<LearningActivity> {
  return apiFetch<LearningActivity>(`/activities/${activityId}`);
}

export async function updateActivity(
  activityId: number,
  data: Partial<{
    title: string;
    type: 'autonomous' | 'guided';
    designer_id: string;
    system_prompt: string;
  }>
): Promise<LearningActivity> {
  return apiFetch<LearningActivity>(`/activities/${activityId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export interface SaveStepData {
  id?: number;
  kp_id: number | null;
  step_type: StepType;
  title: string;
  content_blocks: ContentBlock[];
  sort_order: number;
  active_skill?: string;
}

export async function saveActivitySteps(
  activityId: number,
  steps: SaveStepData[]
): Promise<{ message: string; steps: ActivityStep[] }> {
  return apiFetch<{ message: string; steps: ActivityStep[] }>(`/activities/${activityId}/steps`, {
    method: 'PUT',
    body: JSON.stringify({ steps }),
  });
}

export interface SuggestStepRequest {
  activity_id: number;
  kp_id: number;
  context?: string;
}

export interface SuggestStepResponseBlock {
  type: string;
  content: string;
}

export interface SuggestStepResult {
  title: string;
  step_type: StepType;
  content_blocks: SuggestStepResponseBlock[];
  recommended_skill: string;
}

export async function suggestStepContent(
  request: SuggestStepRequest
): Promise<SuggestStepResult> {
  return apiFetch<SuggestStepResult>('/activities/suggest-step', {
    method: 'POST',
    body: JSON.stringify(request),
  });
}

export async function uploadActivityAsset(
  activityId: number,
  file: File
): Promise<UploadAssetResponse> {
  const formData = new FormData();
  formData.append('file', file);
  return apiFetch<UploadAssetResponse>(`/activities/${activityId}/assets`, {
    method: 'POST',
    body: formData,
  });
}

// ── Teacher Activity Analysis API ───────────────────────────

export interface InquiryTreeNode {
  id: string;
  label: string;
  type: 'concept' | 'question' | 'misconception' | 'resolution';
  children?: InquiryTreeNode[];
}

export interface InquiryTreeResponse {
  session_id: number;
  tree: InquiryTreeNode;
}

export interface InteractionLogEntry {
  id: number;
  timestamp: string;
  sender: string;
  content: string;
  intent?: string;
  skill_used?: string;
  metrics?: {
    coherence?: number;
    engagement?: number;
  };
}

export interface InteractionLogResponse {
  session_id: number;
  student_name: string;
  logs: InteractionLogEntry[];
}

export async function getInquiryTree(sessionId: number): Promise<InquiryTreeResponse> {
  return apiFetch<InquiryTreeResponse>(`/analysis/sessions/${sessionId}/inquiry-tree`);
}

export async function getInteractionLog(sessionId: number): Promise<InteractionLogResponse> {
  return apiFetch<InteractionLogResponse>(`/analysis/sessions/${sessionId}/interaction-log`);
}

// ── WebSocket & Intervention ─────────────────────────────────

export interface WSEvent {
  event: string;
  payload: unknown;
}

export function createWSUrl(sessionId: number): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';
  const hostUrl = new URL(API_BASE);
  const host = hostUrl.host;

  let token = '';
  if (typeof window !== 'undefined') {
    token = localStorage.getItem('hanfledge_token') || '';
  }

  return `${protocol}//${host}/api/v1/ws/session/${sessionId}?token=${token}`;
}

export async function sendIntervention(sessionId: number, type: 'takeover' | 'whisper', content: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/live/sessions/${sessionId}/intervention`, {
    method: 'POST',
    body: JSON.stringify({ type, content }),
  });
}
