import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from './auth'
import * as authApi from '../api/auth'
import type { AuthUser } from '../api/types'

vi.mock('../api/auth', () => ({
  login: vi.fn(),
  register: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
}))

const USER: AuthUser = { id: 1, username: 'alice', email: 'a@x.edu', role: 'user', avatar_url: null }

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('登录：持久化 token 与用户', async () => {
    const store = useAuthStore()
    vi.mocked(authApi.login).mockResolvedValue({ token: 'tk-1', user: USER })
    await store.login('alice', 'pw')
    expect(store.token).toBe('tk-1')
    expect(store.isLoggedIn).toBe(true)
    expect(store.user?.username).toBe('alice')
    expect(localStorage.getItem('af_token')).toBe('tk-1')
  })

  it('注册：两字段载荷透传 + 会话持久化（#76 open 版）', async () => {
    const store = useAuthStore()
    vi.mocked(authApi.register).mockResolvedValue({ token: 'tk-2', user: USER })
    await store.register({ username: 'alice', password: 'secret123' })
    expect(authApi.register).toHaveBeenCalledWith({ username: 'alice', password: 'secret123' })
    expect(store.token).toBe('tk-2')
    expect(store.isLoggedIn).toBe(true)
    expect(store.user?.username).toBe('alice')
    expect(localStorage.getItem('af_token')).toBe('tk-2')
  })

  it('fetchMe：token 有效则载入用户', async () => {
    localStorage.setItem('af_token', 'tk-1')
    const store = useAuthStore()
    vi.mocked(authApi.me).mockResolvedValue(USER)
    const u = await store.fetchMe()
    expect(u?.username).toBe('alice')
    expect(store.isLoggedIn).toBe(true)
  })

  it('fetchMe：token 失效则清除（401 归一）', async () => {
    localStorage.setItem('af_token', 'bad')
    const store = useAuthStore()
    vi.mocked(authApi.me).mockRejectedValue(new Error('未登录'))
    await store.fetchMe()
    expect(store.isLoggedIn).toBe(false)
    expect(localStorage.getItem('af_token')).toBeNull()
  })

  it('登出：清空 token 与用户', async () => {
    localStorage.setItem('af_token', 'tk-1')
    const store = useAuthStore()
    store.user = USER
    vi.mocked(authApi.logout).mockResolvedValue({ message: 'ok' })
    await store.logout()
    expect(store.isLoggedIn).toBe(false)
    expect(store.user).toBeNull()
    expect(localStorage.getItem('af_token')).toBeNull()
  })

  it('角色 getter', () => {
    const store = useAuthStore()
    store.user = { ...USER, role: 'moderator' }
    expect(store.isMod).toBe(true)
    expect(store.isAdmin).toBe(false)
  })
})
