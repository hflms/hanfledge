import { apiFetch, appendPagination, PaginatedResponse, PaginationParams } from './core';
import type { SchoolRole } from './auth';


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


// ── System Config API ───────────────────────────────────────

export async function getSystemConfig(): Promise<Record<string, string>> {
  return apiFetch<Record<string, string>>('/system/config');
}
