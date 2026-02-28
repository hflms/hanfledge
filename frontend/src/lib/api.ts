/**
 * API Client for Hanfledge Backend
 * Wraps fetch with JWT auth and error handling.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

/**
 * Gets the stored JWT token from localStorage.
 */
export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('hanfledge_token');
}

/**
 * Stores the JWT token to localStorage.
 */
export function setToken(token: string): void {
  localStorage.setItem('hanfledge_token', token);
}

/**
 * Removes the JWT token from localStorage.
 */
export function clearToken(): void {
  localStorage.removeItem('hanfledge_token');
}

/**
 * Makes an authenticated API request.
 * Automatically attaches JWT token and handles common errors.
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  // Don't set Content-Type for FormData (browser sets boundary automatically)
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    clearToken();
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new Error('认证已过期，请重新登录');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `请求失败 (${res.status})`);
  }

  return res.json();
}

// ── Auth API ────────────────────────────────────────────────

export interface LoginResponse {
  token: string;
  user: User;
}

export interface User {
  id: number;
  phone: string;
  display_name: string;
  status: string;
  school_roles?: SchoolRole[];
}

export interface SchoolRole {
  id: number;
  user_id: number;
  school_id: number | null;
  role_id: number;
  role?: { id: number; name: string };
  school?: { id: number; name: string };
}

export async function login(phone: string, password: string): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ phone, password }),
  });
}

export async function getMe(): Promise<User> {
  return apiFetch<User>('/auth/me');
}

// ── Course API ──────────────────────────────────────────────

export interface Course {
  id: number;
  school_id: number;
  teacher_id: number;
  title: string;
  subject: string;
  grade_level: number;
  description?: string;
  status: string;
  created_at: string;
  chapters?: Chapter[];
}

export interface Chapter {
  id: number;
  course_id: number;
  title: string;
  sort_order: number;
  knowledge_points?: KnowledgePoint[];
}

export interface KnowledgePoint {
  id: number;
  chapter_id: number;
  title: string;
  difficulty: number;
  is_key_point: boolean;
  neo4j_node_id: string;
  mounted_skills?: MountedSkill[];
}

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

export interface Document {
  id: number;
  course_id: number;
  file_name: string;
  status: string;
  page_count: number;
  created_at: string;
}

export async function listCourses(): Promise<Course[]> {
  return apiFetch<Course[]>('/courses');
}

export async function createCourse(data: {
  school_id: number;
  title: string;
  subject: string;
  grade_level: number;
  description?: string;
}): Promise<Course> {
  return apiFetch<Course>('/courses', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getCourseOutline(courseId: number): Promise<{
  course: Course;
  documents: Document[];
}> {
  return apiFetch(`/courses/${courseId}/outline`);
}

export async function uploadMaterial(courseId: number, file: File): Promise<{
  message: string;
  document: Document;
  page_count: number;
}> {
  const formData = new FormData();
  formData.append('file', file);
  return apiFetch(`/courses/${courseId}/materials`, {
    method: 'POST',
    body: formData,
  });
}

export async function getDocuments(courseId: number): Promise<Document[]> {
  return apiFetch<Document[]>(`/courses/${courseId}/documents`);
}

export async function deleteDocument(courseId: number, docId: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/courses/${courseId}/documents/${docId}`, {
    method: 'DELETE',
  });
}

export async function retryDocument(courseId: number, docId: number): Promise<{
  message: string;
  document: Document;
  page_count: number;
}> {
  return apiFetch(`/courses/${courseId}/documents/${docId}/retry`, {
    method: 'POST',
  });
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

// ── Student Activity API ────────────────────────────────────

export interface LearningActivity {
  id: number;
  course_id: number;
  teacher_id: number;
  title: string;
  kp_ids: string;
  skill_config: string;
  deadline?: string;
  allow_retry: boolean;
  max_attempts: number;
  status: string;
  created_at: string;
  published_at?: string;
  has_session?: boolean;
  session_id?: number;
  session_status?: string;
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
}

export async function listStudentActivities(): Promise<LearningActivity[]> {
  return apiFetch<LearningActivity[]>('/student/activities');
}

export async function joinActivity(activityId: number): Promise<{ message: string; session_id: number }> {
  return apiFetch<{ message: string; session_id: number }>(`/activities/${activityId}/join`, {
    method: 'POST',
  });
}

export async function getSession(sessionId: number): Promise<SessionDetail> {
  return apiFetch<SessionDetail>(`/sessions/${sessionId}`);
}

// ── Dashboard Analytics API — Phase 5 ───────────────────────

export interface KnowledgeRadarData {
  course_id: number;
  course_title: string;
  labels: string[];
  values: number[];
  student_count: number;
}

export interface StudentMasteryItem {
  kp_id: number;
  kp_title: string;
  chapter_title: string;
  mastery_score: number;
  attempt_count: number;
  correct_count: number;
  last_attempt_at?: string;
  updated_at: string;
}

export interface MasteryHistoryPoint {
  date: string;
  avg_mastery: number;
  attempt_count: number;
}

export interface StudentMasteryData {
  student_id: number;
  student_name: string;
  items: StudentMasteryItem[];
  history: MasteryHistoryPoint[];
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

export async function getKnowledgeRadar(courseId: number, classId?: number): Promise<KnowledgeRadarData> {
  const params = new URLSearchParams({ course_id: String(courseId) });
  if (classId) params.set('class_id', String(classId));
  return apiFetch<KnowledgeRadarData>(`/dashboard/knowledge-radar?${params}`);
}

export async function getStudentMastery(studentId: number, courseId?: number): Promise<StudentMasteryData> {
  const params = new URLSearchParams();
  if (courseId) params.set('course_id', String(courseId));
  const qs = params.toString();
  return apiFetch<StudentMasteryData>(`/students/${studentId}/mastery${qs ? '?' + qs : ''}`);
}

export async function getActivitySessions(activityId: number): Promise<ActivitySessionStats> {
  return apiFetch<ActivitySessionStats>(`/activities/${activityId}/sessions`);
}

export async function getSelfMastery(courseId?: number): Promise<StudentMasteryData> {
  const params = new URLSearchParams();
  if (courseId) params.set('course_id', String(courseId));
  const qs = params.toString();
  return apiFetch<StudentMasteryData>(`/student/mastery${qs ? '?' + qs : ''}`);
}

// ── Teacher Activity API ────────────────────────────────────

export async function listTeacherActivities(courseId?: number): Promise<LearningActivity[]> {
  const params = new URLSearchParams();
  if (courseId) params.set('course_id', String(courseId));
  const qs = params.toString();
  return apiFetch<LearningActivity[]>(`/activities${qs ? '?' + qs : ''}`);
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

export async function getInquiryTree(sessionId: number): Promise<InquiryTreeResponse> {
  return apiFetch<InquiryTreeResponse>(`/sessions/${sessionId}/inquiry-tree`);
}

export async function getInteractionLog(sessionId: number): Promise<InteractionLogResponse> {
  return apiFetch<InteractionLogResponse>(`/sessions/${sessionId}/interactions`);
}

export async function getSkillEffectiveness(courseId: number): Promise<SkillEffectivenessResponse> {
  return apiFetch<SkillEffectivenessResponse>(`/dashboard/skill-effectiveness?course_id=${courseId}`);
}

// ── Student Knowledge Map API ───────────────────────────────

export interface KnowledgeMapNode {
  id: number;
  neo4j_id: string;
  title: string;
  chapter_id: number;
  chapter_title: string;
  difficulty: number;
  is_key_point: boolean;
  mastery: number;        // 0.0~1.0, -1 = no data
  attempt_count: number;
}

export interface KnowledgeMapEdge {
  source: string;  // neo4j id
  target: string;  // neo4j id
  type: string;    // "REQUIRES" | "RELATES_TO"
}

export interface KnowledgeMapData {
  course_id: number;
  course_title: string;
  nodes: KnowledgeMapNode[];
  edges: KnowledgeMapEdge[];
  avg_mastery: number;
  mastered_count: number;
  weak_count: number;
}

export async function getStudentKnowledgeMap(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/student/knowledge-map?course_id=${courseId}`);
}

// ── Error Notebook API ─────────────────────────────────────

export interface ErrorNotebookItem {
  id: number;
  kp_id: number;
  kp_title: string;
  chapter_title: string;
  session_id: number;
  student_input: string;
  coach_guidance: string;
  error_type: string;
  mastery_at_error: number;
  resolved: boolean;
  resolved_at?: string;
  archived_at: string;
}

export interface ErrorNotebookData {
  items: ErrorNotebookItem[];
  total_count: number;
  unresolved_count: number;
  resolved_count: number;
}

export async function getErrorNotebook(opts?: { resolved?: boolean; kpId?: number }): Promise<ErrorNotebookData> {
  const params = new URLSearchParams();
  if (opts?.resolved !== undefined) params.set('resolved', String(opts.resolved));
  if (opts?.kpId !== undefined) params.set('kp_id', String(opts.kpId));
  const qs = params.toString();
  return apiFetch<ErrorNotebookData>(`/student/error-notebook${qs ? '?' + qs : ''}`);
}

// ── WebSocket Helpers ───────────────────────────────────────

export interface WSEvent {
  event: string;
  payload: unknown;
  timestamp: number;
}

export function createWSUrl(sessionId: number): string {
  const base = API_BASE.replace(/^http/, 'ws');
  const token = getToken();
  return `${base}/sessions/${sessionId}/stream${token ? '?token=' + token : ''}`;
}

// ── Data Export API ─────────────────────────────────────────

/**
 * Triggers a CSV file download by fetching an export endpoint.
 * The response is a CSV blob; this function creates a temporary
 * download link and clicks it programmatically.
 */
async function downloadCSV(path: string, fallbackFilename: string): Promise<void> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, { headers });

  if (res.status === 401) {
    clearToken();
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new Error('认证已过期，请重新登录');
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `导出失败 (${res.status})`);
  }

  // Extract filename from Content-Disposition if available
  const disposition = res.headers.get('Content-Disposition');
  let filename = fallbackFilename;
  if (disposition) {
    const match = disposition.match(/filename=(.+)/);
    if (match) filename = match[1];
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export async function exportActivitySessions(activityId: number): Promise<void> {
  return downloadCSV(`/export/activities/${activityId}/sessions`, `sessions_${activityId}.csv`);
}

export async function exportClassMastery(courseId: number): Promise<void> {
  return downloadCSV(`/export/courses/${courseId}/mastery`, `mastery_${courseId}.csv`);
}

export async function exportErrorNotebookCSV(courseId: number): Promise<void> {
  return downloadCSV(`/export/courses/${courseId}/error-notebook`, `error_notebook_${courseId}.csv`);
}

export async function exportInteractionLog(sessionId: number): Promise<void> {
  return downloadCSV(`/export/sessions/${sessionId}/interactions`, `interactions_${sessionId}.csv`);
}
