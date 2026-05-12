import { apiFetch, appendPagination, PaginatedResponse, PaginationParams } from './core';
import type { MountedSkill } from './skill';



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


export async function getCourseKnowledgeGraph(courseId: number): Promise<KnowledgeMapData> {
  return apiFetch<KnowledgeMapData>(`/courses/${courseId}/graph`);
}
