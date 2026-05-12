import { apiFetch } from './core';

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
  mastery_level: number;
  status: 'proficient' | 'developing' | 'struggling';
  last_assessed: string;
}

export interface MasteryHistoryPoint {
  date: string;
  score: number;
}

export interface StudentMasteryData {
  overall_score: number;
  items: StudentMasteryItem[];
  history?: MasteryHistoryPoint[];
}

export interface SessionSummary {
  session_id: number;
  student_name: string;
  duration_minutes: number;
  status: string;
  kps_covered: number;
  engagement_score: number;
}

export interface ActivitySessionStats {
  total_sessions: number;
  active_sessions: number;
  avg_duration: number;
  completion_rate: number;
  recent_sessions: SessionSummary[];
}

export async function getKnowledgeRadar(courseId: number, classId?: number): Promise<KnowledgeRadarData> {
  const qs = classId ? `?class_id=${classId}` : '';
  return apiFetch<KnowledgeRadarData>(`/dashboard/knowledge-radar?course_id=${courseId}${classId ? '&class_id=' + classId : ''}`);
}

export async function getStudentMastery(studentId: number, courseId?: number): Promise<StudentMasteryData> {
  const qs = courseId ? `?course_id=${courseId}` : '';
  return apiFetch<StudentMasteryData>(`/dashboard/students/${studentId}/mastery${qs}`);
}

export async function getActivitySessions(activityId: number): Promise<ActivitySessionStats> {
  return apiFetch<ActivitySessionStats>(`/dashboard/activities/${activityId}/sessions`);
}

export async function getSelfMastery(courseId?: number): Promise<StudentMasteryData> {
  const qs = courseId ? `?course_id=${courseId}` : '';
  return apiFetch<StudentMasteryData>(`/dashboard/me/mastery${qs}`);
}

// ── Live Dashboard ──────────────────────────────────────────

export interface LiveActivitySummary {
  active_students: number;
  avg_engagement: number;
  struggling_students: number;
  completion_rate: number;
}

export interface LiveMonitorResponse {
  course_id: number;
  live_activities: LiveActivitySummary[];
}

export interface LiveStudentInfo {
  session_id: number;
  student_id: number;
  student_name: string;
  status: 'active' | 'idle' | 'struggling' | 'completed';
  current_step: string;
  engagement_score: number;
  last_active: string;
}

export interface LiveStepInfo {
  step_id: number;
  title: string;
  student_count: number;
  avg_time_spent: number;
}

export interface StudentAlert {
  session_id: number;
  student_name: string;
  type: 'stuck' | 'low_engagement' | 'frequent_errors';
  message: string;
  timestamp: string;
}

export interface KPSequenceItem {
  kp_id: number;
  title: string;
  student_count: number;
}

export interface ActivityLiveDetailResponse {
  activity_id: number;
  summary: LiveActivitySummary;
  students: LiveStudentInfo[];
  steps: LiveStepInfo[];
  alerts: StudentAlert[];
  kp_sequence?: KPSequenceItem[];
}

export async function getLiveMonitor(courseId: number): Promise<LiveMonitorResponse> {
  return apiFetch<LiveMonitorResponse>(`/live/courses/${courseId}`);
}

export async function getActivityLiveDetail(activityId: number): Promise<ActivityLiveDetailResponse> {
  return apiFetch<ActivityLiveDetailResponse>(`/live/activities/${activityId}`);
}

export interface SkillEffectivenessItem {
  skill_id: string;
  skill_name: string;
  usage_count: number;
  avg_mastery_gain: number;
  avg_engagement: number;
}

export interface SkillEffectivenessResponse {
  items: SkillEffectivenessItem[];
}

export async function getSkillEffectiveness(courseId: number): Promise<SkillEffectivenessResponse> {
  return apiFetch<SkillEffectivenessResponse>(`/analysis/courses/${courseId}/skills`);
}

export interface KnowledgeMapNode {
  id: string;
  label: string;
  group: 'course' | 'chapter' | 'kp';
  mastery_level?: number;
  status?: 'locked' | 'unlocked' | 'mastered';
}

export interface KnowledgeMapEdge {
  source: string;
  target: string;
  label?: string;
}

export interface KnowledgeMapData {
  nodes: KnowledgeMapNode[];
  edges: KnowledgeMapEdge[];
}

export async function getStudentKnowledgeMap(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/knowledge-map/student?course_id=${courseId}`);
}

export async function getCourseKnowledgeGraph(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/knowledge-map/course/${courseId}`);
}

export interface ErrorNotebookItem {
  id: number;
  kp_id: number;
  kp_title: string;
  error_content: string;
  activity_id: number;
  session_id: number;
  created_at: string;
  resolved: boolean;
  resolution_summary?: string;
  resolution_date?: string;
}

export interface ErrorNotebookData {
  items: ErrorNotebookItem[];
  total: number;
}

export async function getErrorNotebook(opts?: { resolved?: boolean; kpId?: number }): Promise<ErrorNotebookData> {
  const params = new URLSearchParams();
  if (opts?.resolved !== undefined) params.set('resolved', String(opts.resolved));
  if (opts?.kpId !== undefined) params.set('kp_id', String(opts.kpId));
  const qs = params.toString();
  return apiFetch<ErrorNotebookData>(`/student/error-notebook${qs ? '?' + qs : ''}`);
}

export interface AchievementProgress {
  id: number;
  achievement_id: string;
  name: string;
  description: string;
  icon_url: string;
  status: 'locked' | 'in_progress' | 'unlocked';
  progress: number;
  target: number;
  unlocked_at?: string;
}

export interface StudentAchievementsData {
  total_unlocked: number;
  total_available: number;
  achievements: AchievementProgress[];
}

export async function getMyAchievements(): Promise<StudentAchievementsData> {
  return apiFetch<StudentAchievementsData>('/student/achievements');
}

export interface AchievementDefinition {
  id: string;
  name: string;
  description: string;
  icon_url: string;
  target: number;
  event_type: string;
}

export async function getAchievementDefinitions(): Promise<AchievementDefinition[]> {
  return apiFetch<AchievementDefinition[]>('/achievements');
}

export async function getSystemConfig(): Promise<Record<string, string>> {
  return apiFetch<Record<string, string>>('/config');
}

// ── Export Utility ──────────────────────────────────────────

async function downloadCSV(path: string, fallbackFilename: string): Promise<void> {
  const token = localStorage.getItem('hanfledge_token');
  const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

  const res = await fetch(`${API_BASE}${path}`, {
    headers: { Authorization: `Bearer ${token}` }
  });

  if (!res.ok) throw new Error(`Export failed: ${res.status}`);

  const contentDisposition = res.headers.get('Content-Disposition');
  let filename = fallbackFilename;
  if (contentDisposition) {
    const match = contentDisposition.match(/filename="?([^"]+)"?/);
    if (match && match[1]) filename = match[1];
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
