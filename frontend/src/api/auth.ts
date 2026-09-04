import { request } from './client'
import type { AuthResult, AuthUser, RegisterPayload } from './types'

export function login(username: string, password: string) {
  return request<AuthResult>('POST', '/auth/login', { username, password })
}

// #76 open 版：免码注册（email/invite_code 由后端忽略，载荷收窄）
export function register(payload: RegisterPayload) {
  return request<AuthResult>('POST', '/auth/register', payload)
}

export function me() {
  return request<AuthUser>('GET', '/auth/me', undefined, { requireAuth: true })
}

export function logout() {
  return request<{ message: string }>('POST', '/auth/logout', undefined, { requireAuth: true })
}
