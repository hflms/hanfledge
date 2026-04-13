import { apiFetch, downloadCSV } from './core';
import type { KnowledgeMapData } from './course';



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


export async function getSelfMastery(courseId?: number): Promise<StudentMasteryData> {
  const params = new URLSearchParams();
  if (courseId) params.set('course_id', String(courseId));
  const qs = params.toString();
  return apiFetch<StudentMasteryData>(`/student/mastery${qs ? '?' + qs : ''}`);
}


export async function getStudentKnowledgeMap(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/student/knowledge-map?course_id=${courseId}`);
}


export async function getErrorNotebook(opts?: { resolved?: boolean; kpId?: number }): Promise<ErrorNotebookData> {
  const params = new URLSearchParams();
  if (opts?.resolved !== undefined) params.set('resolved', String(opts.resolved));
  if (opts?.kpId !== undefined) params.set('kp_id', String(opts.kpId));
  const qs = params.toString();
  return apiFetch<ErrorNotebookData>(`/student/error-notebook${qs ? '?' + qs : ''}`);
}


export async function getMyAchievements(): Promise<StudentAchievementsData> {
  return apiFetch<StudentAchievementsData>('/student/achievements');
}


export async function getAchievementDefinitions(): Promise<AchievementDefinition[]> {
  return apiFetch<AchievementDefinition[]>('/student/achievements/definitions');
}


export async function exportClassMastery(courseId: number): Promise<void> {
  return downloadCSV(`/export/courses/${courseId}/mastery`, `mastery_${courseId}.csv`);
}


export async function exportErrorNotebookCSV(courseId: number): Promise<void> {
  return downloadCSV(`/export/courses/${courseId}/error-notebook`, `error_notebook_${courseId}.csv`);
}
