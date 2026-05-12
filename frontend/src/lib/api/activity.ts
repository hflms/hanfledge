import { apiFetch, appendPagination, type PaginatedResponse, type PaginationParams, downloadCSV, API_BASE, getToken } from './core';



// ── Student Activity API ────────────────────────────────────

export interface LearningActivity {
  id: number;
  course_id: number;
  teacher_id: number;
  title: string;
  description?: string;
  type: 'autonomous' | 'guided';
  designer_id?: string;
  designer_config?: string;
  steps_config?: string;
  kp_ids: string;
  skill_config: string;
  deadline?: string;
  allow_retry: boolean;
  max_attempts: number;
  status: string;
  created_at: string;
  updated_at?: string;
  published_at?: string;
  has_session?: boolean;
  session_id?: number;
  session_status?: string;
  steps?: ActivityStep[];
  assigned_classes?: ActivityClassAssignment[];
}


export type ContentBlockType = 'markdown' | 'file' | 'video' | 'image';


export type StepType = 'lecture' | 'discussion' | 'quiz' | 'practice' | 'reading' | 'group_work' | 'reflection' | 'ai_tutoring';


export interface ContentBlock {
  type: ContentBlockType;
  content: string;
  file_name?: string;
  file_url?: string;
  mime_type?: string;
}


export interface ActivityStep {
  id: number;
  activity_id: number;
  title: string;
  description: string;
  step_type: StepType;
  sort_order: number;
  content_blocks: string; // JSON string of ContentBlock[]
  duration: number;
  created_at: string;
  updated_at: string;
}


export interface ActivityClassAssignment {
  id: number;
  activity_id: number;
  class_id: number;
}


export interface UploadAssetResponse {
  file_name: string;
  file_url: string;
  file_size: number;
  mime_type: string;
  key: string;
}


export interface StudentSession {
  id: number;
  student_id: number;
  activity_id: number;
  current_kp_id: number;
  active_skill: string;
  scaffold_level: 'high' | 'medium' | 'low';
  status: string;
  started_at: string;
  ended_at?: string;
  is_sandbox?: boolean;
}


export interface Interaction {
  id: number;
  session_id: number;
  role: string;
  content: string;
  skill_id: string;
  tokens_used: number;
  created_at: string;
}


export interface SessionDetail {
  session: StudentSession;
  interactions: Interaction[];
  activity: LearningActivity;
}


export interface StepTransitionResult {
  message: string;
  step_summary: string;
  old_kp_id: number;
  new_kp_id: number;
}


export interface SessionSummary {
  session_id: number;
  student_id: number;
  student_name: string;
  status: string;
  scaffold_level: string;
  started_at: string;
  ended_at?: string;
  duration_min: number;
  mastery_score: number;
}


export interface ActivitySessionStats {
  activity_id: number;
  activity_title: string;
  total_sessions: number;
  active_sessions: number;
  completed_sessions: number;
  completion_rate: number;
  avg_duration_min: number;
  avg_mastery: number;
  sessions: SessionSummary[];
}


// ── Live Monitor API — Real-time Student Monitoring ─────────

export interface LiveActivitySummary {
  activity_id: number;
  activity_title: string;
  activity_status: string;
  total_students: number;
  active_students: number;
  completed_students: number;
  avg_mastery: number;
  avg_duration_min: number;
}


export interface LiveMonitorResponse {
  course_id: number;
  timestamp: string;
  activities: LiveActivitySummary[];
}


export interface LiveStudentInfo {
  student_id: number;
  student_name: string;
  session_id: number;
  status: string;
  duration_min: number;
  mastery_score: number;
  interaction_count: number;
  last_active_at: string;
  scaffold_level: string;
}


export interface LiveStepInfo {
  kp_id: number;
  kp_title: string;
  step_index: number;
  students: LiveStudentInfo[];
}


export interface StudentAlert {
  student_id: number;
  student_name: string;
  session_id: number;
  alert_type: 'idle' | 'stuck' | 'struggling';
  message: string;
}


export interface KPSequenceItem {
  kp_id: number;
  kp_title: string;
}


export interface ActivityLiveDetailResponse {
  activity_id: number;
  title: string;
  kp_sequence: KPSequenceItem[];
  steps: LiveStepInfo[];
  alerts: StudentAlert[];
  timestamp: string;
}


export interface InstructionalDesigner {
  id: string;
  name: string;
  description: string;
  intervention_style: string;
  is_built_in: boolean;
  created_at: string;
  updated_at: string;
}


export interface SaveStepData {
  id?: number;
  title: string;
  description?: string;
  step_type?: StepType;
  sort_order: number;
  content_blocks?: string;
  duration?: number;
}


// ── AI Step Suggestion ──────────────────────────────────────

export interface SuggestStepRequest {
  step_type: StepType;
  step_title?: string;
  step_description?: string;
  activity_title?: string;
  knowledge_points?: string[];
}


export interface SuggestStepResponseBlock {
  type: string;
  content: string;
}


export interface SuggestStepResult {
  title: string;
  description: string;
  content_blocks: SuggestStepResponseBlock[];
  duration: number;
}


// ── Analytics V2 API — Phase G ──────────────────────────────

export interface InquiryTreeNode {
  id: number;
  role: string;
  content: string;
  skill_id?: string;
  depth: number;
  turn_type: string;
  time: string;
  children?: InquiryTreeNode[];
}


export interface InquiryTreeResponse {
  session_id: number;
  student_name: string;
  total_turns: number;
  max_depth: number;
  skill_used: string;
  roots: InquiryTreeNode[];
}


export interface InteractionLogEntry {
  id: number;
  role: string;
  content: string;
  skill_id?: string;
  tokens_used: number;
  created_at: string;
  faithfulness_score?: number;
  actionability_score?: number;
  answer_restraint_score?: number;
  eval_status: string;
}


export interface InteractionLogResponse {
  session_id: number;
  student_name: string;
  active_skill: string;
  scaffold_level: string;
  status: string;
  started_at: string;
  ended_at?: string;
  interactions: InteractionLogEntry[];
}


export interface SkillEffectivenessItem {
  skill_id: string;
  session_count: number;
  interaction_count: number;
  evaluated_count: number;
  avg_faithfulness: number;
  avg_actionability: number;
  avg_answer_restraint: number;
  avg_context_precision: number;
  avg_context_recall: number;
  avg_mastery_delta: number;
}


export interface SkillEffectivenessResponse {
  course_id: number;
  course_title: string;
  items: SkillEffectivenessItem[];
}


// ── WebSocket Helpers ───────────────────────────────────────

export interface WSEvent {
  event: string;
  payload: unknown;
  timestamp: number;
}


export async function listStudentActivities(): Promise<LearningActivity[]> {
  return apiFetch<LearningActivity[]>('/student/activities');
}


export async function joinActivity(activityId: number): Promise<{ message: string; session_id: number }> {
  return apiFetch<{ message: string; session_id: number }>(`/activities/${activityId}/join`, {
    method: 'POST',
  });
}


export async function previewActivity(activityId: number): Promise<{ message: string; session_id: number; is_sandbox: boolean }> {
  return apiFetch<{ message: string; session_id: number; is_sandbox: boolean }>(`/activities/${activityId}/preview`, {
    method: 'POST',
  });
}


export async function getSession(sessionId: number): Promise<SessionDetail> {
  return apiFetch<SessionDetail>(`/sessions/${sessionId}`);
}


export async function updateSessionStep(sessionId: number, kpId: number, activeSkill: string): Promise<StepTransitionResult> {
  return apiFetch<StepTransitionResult>(`/sessions/${sessionId}/step`, {
    method: 'PUT',
    body: JSON.stringify({ kp_id: kpId, active_skill: activeSkill }),
  });
}


export async function getActivitySessions(activityId: number): Promise<ActivitySessionStats> {
  return apiFetch<ActivitySessionStats>(`/activities/${activityId}/sessions`);
}


export async function getLiveMonitor(courseId: number): Promise<LiveMonitorResponse> {
  return apiFetch<LiveMonitorResponse>(`/dashboard/live-monitor?course_id=${courseId}`);
}


export async function getActivityLiveDetail(activityId: number): Promise<ActivityLiveDetailResponse> {
  return apiFetch<ActivityLiveDetailResponse>(`/dashboard/activities/${activityId}/live`);
}


// ── Teacher Activity API ────────────────────────────────────

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
  type?: 'autonomous' | 'guided';
  designer_id?: string;
  designer_config?: Record<string, unknown>;
  steps_config?: unknown[];
  kp_ids: number[];
  class_ids?: number[];
  deadline?: string;
  allow_retry?: boolean;
  max_attempts?: number;
  skill_config?: Record<string, unknown>;
}): Promise<LearningActivity> {
  return apiFetch<LearningActivity>('/activities', {
    method: 'POST',
    body: JSON.stringify(data),
  });
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
  data: {
    title?: string;
    description?: string;
    type?: 'autonomous' | 'guided';
    designer_id?: string;
    designer_config?: Record<string, unknown>;
    kp_ids?: number[];
    skill_config?: Record<string, unknown>;
    deadline?: string;
    allow_retry?: boolean;
    max_attempts?: number;
    class_ids?: number[];
  },
): Promise<LearningActivity> {
  return apiFetch<LearningActivity>(`/activities/${activityId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}


export async function saveActivitySteps(
  activityId: number,
  steps: SaveStepData[],
): Promise<ActivityStep[]> {
  return apiFetch<ActivityStep[]>(`/activities/${activityId}/steps`, {
    method: 'PUT',
    body: JSON.stringify({ steps }),
  });
}


export async function suggestStepContent(
  activityId: number,
  params: SuggestStepRequest,
): Promise<{ suggestion: SuggestStepResult }> {
  return apiFetch<{ suggestion: SuggestStepResult }>(`/activities/${activityId}/steps/suggest`, {
    method: 'POST',
    body: JSON.stringify(params),
  });
}


export async function uploadActivityAsset(
  activityId: number,
  file: File,
): Promise<UploadAssetResponse> {
  const formData = new FormData();
  formData.append('file', file);
  return apiFetch<UploadAssetResponse>(`/activities/${activityId}/upload`, {
    method: 'POST',
    body: formData,
  });
}


export async function getInquiryTree(sessionId: number): Promise<InquiryTreeResponse> {
  return apiFetch<InquiryTreeResponse>(`/sessions/${sessionId}/inquiry-tree`);
}


export async function getInteractionLog(sessionId: number): Promise<InteractionLogResponse> {
  return apiFetch<InteractionLogResponse>(`/sessions/${sessionId}/interactions`);
}


export async function getSkillEffectiveness(courseId: number): Promise<SkillEffectivenessResponse> {
  return apiFetch<SkillEffectivenessResponse>(`/dashboard/skill-effectiveness?course_id=${courseId}`);
}


export function createWSUrl(sessionId: number): string {
  let base = API_BASE;
  if (base.startsWith('/')) {
    if (typeof window !== 'undefined') {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      base = `${protocol}//${window.location.host}${base}`;
    }
  } else {
    base = base.replace(/^http/, 'ws');
  }
  const token = getToken();
  return `${base}/sessions/${sessionId}/stream${token ? '?token=' + token : ''}`;
}


export async function exportActivitySessions(activityId: number): Promise<void> {
  return downloadCSV(`/export/activities/${activityId}/sessions`, `sessions_${activityId}.csv`);
}


export async function exportInteractionLog(sessionId: number): Promise<void> {
  return downloadCSV(`/export/sessions/${sessionId}/interactions`, `interactions_${sessionId}.csv`);
}


// ── Teacher Intervention API — Phase 6 ───────────────────────

export async function sendIntervention(sessionId: number, type: 'takeover' | 'whisper', content: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/sessions/${sessionId}/intervention`, {
    method: 'POST',
    body: JSON.stringify({ type, content }),
  });
}
