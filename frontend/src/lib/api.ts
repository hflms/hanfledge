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

// ── Skill API ───────────────────────────────────────────────

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
