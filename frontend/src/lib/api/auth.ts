/**
 * Authentication API
 */

import { apiFetch } from './core';

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
