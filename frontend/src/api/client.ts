// fetch 包装（D6）：统一 /api/v1 前缀、Bearer 注入、401 归一、统一错误体
const TOKEN_KEY = 'af_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(token: string | null): void {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export const BASE = '/api/v1'

// #34（评审 F1）：会话失效（401）或账号被封禁（403 code=account_banned）时统一调用，
// 由 auth store 注册清 Pinia state 的回调——仅清 localStorage 不清 store 会让「封禁即登出」失效。
let authFailureHandler: (() => void) | null = null
export function setAuthFailureHandler(fn: (() => void) | null): void {
  authFailureHandler = fn
}

interface RequestOpts {
  /** 该接口是否要求登录：401 时清除过期 token 并抛错（否则游客容忍，如公开读接口）。 */
  requireAuth?: boolean
}

export async function request<T>(method: string, path: string, body?: unknown, opts: RequestOpts = {}): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return undefined as T

  let data: { error?: unknown; code?: unknown } | null = null
  try {
    data = await res.json()
  } catch {
    /* 非 JSON 错误体 */
  }
  const msg = data && typeof data.error === 'string' ? data.error : '请求失败'

  // 会话重置：401（需登录接口）或 403 code=account_banned（被封，F5 机器可读信号）。
  // 普通 403「权限不足」不清 token（防止 mod 误调 admin 接口被登出）。
  const sessionLost =
    (res.status === 401 && opts.requireAuth) || (res.status === 403 && data?.code === 'account_banned')
  if (sessionLost) {
    setToken(null)
    authFailureHandler?.()
  }

  if (!res.ok) {
    throw new ApiError(res.status, msg)
  }
  return data as T
}
