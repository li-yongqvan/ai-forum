// api/client 会话重置单测（#34 评审 F1/F5/F8）：401 与 403(code=account_banned) 统一触发会话重置钩子，
// 普通 403「权限不足」不清 token（防 mod 误调 admin 接口被登出）。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getToken, request, setAuthFailureHandler, setToken } from './client'

function mockFetchOnce(status: number, body: unknown) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      status,
      ok: status >= 200 && status < 300,
      json: () => Promise.resolve(body),
    }),
  )
}

describe('api/client 会话重置（#34）', () => {
  beforeEach(() => {
    setToken('tk-1')
    setAuthFailureHandler(null)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('403 code=account_banned → 清 token + 触发会话重置钩子', async () => {
    const hook = vi.fn()
    setAuthFailureHandler(hook)
    mockFetchOnce(403, { error: '账号已被封禁', code: 'account_banned' })
    await expect(request('POST', '/posts', { title: 'x' }, { requireAuth: true })).rejects.toThrow('账号已被封禁')
    expect(hook).toHaveBeenCalled()
    expect(getToken()).toBeNull()
  })

  it('403 权限不足（无 code）→ 不清 token、不触发钩子', async () => {
    const hook = vi.fn()
    setAuthFailureHandler(hook)
    mockFetchOnce(403, { error: '权限不足' })
    await expect(request('POST', '/moderation/users/1/ban', { reason: 'x' }, { requireAuth: true })).rejects.toThrow(
      '权限不足',
    )
    expect(hook).not.toHaveBeenCalled()
    expect(getToken()).toBe('tk-1')
  })

  it('401 + requireAuth → 清 token + 触发钩子（统一会话重置）', async () => {
    const hook = vi.fn()
    setAuthFailureHandler(hook)
    mockFetchOnce(401, { error: '登录已过期，请重新登录' })
    await expect(request('POST', '/posts', {}, { requireAuth: true })).rejects.toThrow('登录已过期')
    expect(hook).toHaveBeenCalled()
    expect(getToken()).toBeNull()
  })

  it('401 无 requireAuth（游客容忍读接口）→ 不清 token', async () => {
    mockFetchOnce(401, { error: '未登录' })
    await expect(request('GET', '/posts', undefined, {})).rejects.toThrow('未登录')
    expect(getToken()).toBe('tk-1')
  })
})
