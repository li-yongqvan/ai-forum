import { ApiError, BASE, getToken, setToken } from './client'

// multipart 图片上传（#12 D4：request() 只处理 JSON，multipart 需独立函数；
// FormData 不设 Content-Type，由浏览器自动带 multipart boundary；Bearer 照常注入）。
export async function uploadImage(file: File): Promise<{ url: string }> {
  const fd = new FormData()
  fd.append('file', file)
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(BASE + '/uploads', { method: 'POST', headers, body: fd })

  if (res.status === 401) {
    // 登录过期：清 token（与 request() 的 requireAuth 行为一致），路由守卫下次导航引导重登
    setToken(null)
  }
  if (!res.ok) {
    let msg = '上传失败'
    try {
      const data = await res.json()
      if (data && typeof data.error === 'string') msg = data.error
    } catch {
      /* 非 JSON 错误体，用默认文案 */
    }
    throw new ApiError(res.status, msg)
  }
  return res.json()
}
