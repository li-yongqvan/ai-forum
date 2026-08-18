import { request } from './client'
import type { AuthResult, AuthUser } from './types'

export function login(username: string, password: string) {
  return request<AuthResult>('POST', '/auth/login', { username, password })
}

export function register(payload: {
  username: string
  email: string
  password: string
  invite_code: string
}) {
  return request<AuthResult>('POST', '/auth/register', payload)
}

export function me() {
  return request<AuthUser>('GET', '/auth/me', undefined, { requireAuth: true })
}

export function logout() {
  return request<{ message: string }>('POST', '/auth/logout', undefined, { requireAuth: true })
}
