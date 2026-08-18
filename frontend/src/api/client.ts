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

  if (res.status === 401 && opts.requireAuth) {
    // 登录过期：清 token（D6 401 归一；路由守卫会在下次导航时引导重登）
    setToken(null)
  }

  if (!res.ok) {
    let msg = '请求失败'
    try {
      const data = await res.json()
      if (data && typeof data.error === 'string') msg = data.error
    } catch {
      /* 非 JSON 错误体，用默认文案 */
    }
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}
