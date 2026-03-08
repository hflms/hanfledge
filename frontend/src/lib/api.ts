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

// -- Pagination Types -----------------------------------------

/** Standard paginated response from the backend. */
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

/** Common pagination query parameters. */
export interface PaginationParams {
  page?: number;
  limit?: number;
}

/** Appends pagination params to a URLSearchParams object. */
function appendPagination(params: URLSearchParams, pg?: PaginationParams): void {
  if (pg?.page) params.set('page', String(pg.page));
  if (pg?.limit) params.set('limit', String(pg.limit));
}

// -- Auth Types -----------------------------------------------

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

// ── Admin API ───────────────────────────────────────────────

export interface School {
  id: number;
  name: string;
  code: string;
  city: string;
  status: string;
  created_at: string;
}

export interface ClassItem {
  id: number;
  name: string;
  grade_level: number;
  school_id: number;
  created_at: string;
}

export interface AdminUser {
  id: number;
  phone: string;
  display_name: string;
  status: string;
  created_at: string;
  school_roles?: SchoolRole[];
}

export async function listSchools(pg?: PaginationParams): Promise<PaginatedResponse<School>> {
  const params = new URLSearchParams();
  appendPagination(params, pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<School>>(`/schools${qs ? '?' + qs : ''}`);
}

export async function createSchool(data: {
  name: string;
  code: string;
  city: string;
}): Promise<School> {
  return apiFetch<School>('/schools', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function listClasses(pg?: PaginationParams): Promise<PaginatedResponse<ClassItem>> {
  const params = new URLSearchParams();
  appendPagination(params, pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<ClassItem>>(`/classes${qs ? '?' + qs : ''}`);
}

export async function createClass(data: {
  school_id: number;
  name: string;
  grade_level: number;
}): Promise<ClassItem> {
  return apiFetch<ClassItem>('/classes', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function listAdminUsers(pg?: PaginationParams): Promise<PaginatedResponse<AdminUser>> {
  const params = new URLSearchParams();
  appendPagination(params, pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<AdminUser>>(`/users${qs ? '?' + qs : ''}`);
}

export async function createAdminUser(data: {
  phone: string;
  password: string;
  display_name: string;
  role_name: string;
  school_id?: number;
  class_id?: number;
}): Promise<AdminUser> {
  return apiFetch<AdminUser>('/users', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function batchCreateUsers(data: {
  school_id: number;
  class_id?: number;
  role_name: string;
  users: Array<{ name: string; phone: string }>;
}): Promise<{ message: string; count: number }> {
  return apiFetch<{ message: string; count: number }>('/users/batch', {
    method: 'POST',
    body: JSON.stringify(data),
  });
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
  error_message?: string;
  created_at: string;
}

export async function listCourses(pg?: PaginationParams): Promise<PaginatedResponse<Course>> {
  const params = new URLSearchParams();
  appendPagination(params, pg);
  const qs = params.toString();
  return apiFetch<PaginatedResponse<Course>>(`/courses${qs ? '?' + qs : ''}`);
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
  error_message?: string;
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
  error_message?: string;
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

export async function updateSessionStep(sessionId: number, kpId: number, activeSkill: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/sessions/${sessionId}/step`, {
    method: 'PUT',
    body: JSON.stringify({ kp_id: kpId, active_skill: activeSkill }),
  });
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

export async function publishActivity(activityId: number): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/activities/${activityId}/publish`, {
    method: 'POST',
  });
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
  description?: string;
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

export async function getCourseKnowledgeGraph(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/courses/${courseId}/graph`);
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

// ── Achievement API (design.md §5.2 Step 4) ──────────────────

export interface AchievementProgress {
  id: number;
  type: string;           // streak_breaker | deep_inquiry | fallacy_hunter
  tier: string;           // bronze | silver | gold | diamond
  name: string;
  description: string;
  icon: string;
  threshold: number;
  progress: number;
  unlocked: boolean;
  unlocked_at?: string;
}

export interface StudentAchievementsData {
  total_unlocked: number;
  total_count: number;
  achievements: AchievementProgress[];
}

export async function getMyAchievements(): Promise<StudentAchievementsData> {
  return apiFetch<StudentAchievementsData>('/student/achievements');
}

export interface AchievementDefinition {
  id: number;
  type: string;
  tier: string;
  name: string;
  description: string;
  icon: string;
  threshold: number;
  sort_order: number;
}

export async function getAchievementDefinitions(): Promise<AchievementDefinition[]> {
  return apiFetch<AchievementDefinition[]>('/student/achievements/definitions');
}

// ── System Config API ───────────────────────────────────────

export async function getSystemConfig(): Promise<Record<string, string>> {
  return apiFetch<Record<string, string>>('/system/config');
}

// ── WebSocket Helpers ───────────────────────────────────────

export interface WSEvent {
  event: string;
  payload: unknown;
  timestamp: number;
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

// ── Teacher Intervention API — Phase 6 ───────────────────────

export async function sendIntervention(sessionId: number, type: 'takeover' | 'whisper', content: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>(`/sessions/${sessionId}/intervention`, {
    method: 'POST',
    body: JSON.stringify({ type, content }),
  });
}

// ── WeKnora API ──────────────────────────────────────────────

export interface WeKnoraKB {
  id: string;
  name: string;
  description: string;
  file_count: number;
  token_count: number;
  chunk_count: number;
  created_at: string;
  updated_at: string;
}

export interface WeKnoraKBRef {
  id: number;
  course_id: number;
  kb_id: string;
  kb_name: string;
  added_by_id: number;
  created_at: string;
}

export async function listWeKnoraKnowledgeBases(): Promise<WeKnoraKB[]> {
  const resp = await apiFetch<{ data: WeKnoraKB[] }>('/weknora/knowledge-bases');
  return resp.data || [];
}

export async function getCourseWeKnoraRefs(courseId: number): Promise<WeKnoraKBRef[]> {
  const resp = await apiFetch<{ data: WeKnoraKBRef[] }>(`/courses/${courseId}/weknora-refs`);
  return resp.data || [];
}

export async function bindWeKnoraKnowledgeBase(
  courseId: number,
  kbId: string,
  kbName: string
): Promise<WeKnoraKBRef> {
  return apiFetch<WeKnoraKBRef>(`/courses/${courseId}/weknora-refs`, {
    method: 'POST',
    body: JSON.stringify({ kb_id: kbId, kb_name: kbName }),
  });
}

export async function unbindWeKnoraKnowledgeBase(courseId: number, refId: number): Promise<void> {
  return apiFetch<void>(`/courses/${courseId}/weknora-refs/${refId}`, {
    method: 'DELETE',
  });
}

export async function getWeKnoraLoginToken(): Promise<{ token: string; weknora_url: string }> {
  return apiFetch<{ token: string; weknora_url: string }>('/weknora/login-token');
}
