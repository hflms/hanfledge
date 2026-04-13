import { apiFetch } from './core';



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
