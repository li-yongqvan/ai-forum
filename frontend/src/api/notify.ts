import { request } from './client'
import type { AppNotification } from './types'

// ---- 通知中心（#32：接口需登录，#9 登录墙） ----

export interface ListNotificationsParams {
  unread?: boolean
  page?: number
  pageSize?: number
}

export interface NotificationList {
  items: AppNotification[]
  page: number
  page_size: number
}

export function listNotifications(p: ListNotificationsParams = {}) {
  const qs = new URLSearchParams()
  if (p.unread) qs.set('unread', '1')
  qs.set('page', String(p.page ?? 1))
  qs.set('page_size', String(p.pageSize ?? 20))
  return request<NotificationList>('GET', `/notifications?${qs.toString()}`, undefined, { requireAuth: true })
}

/** 未读通知数（底部 Tab badge）。 */
export function unreadCount() {
  return request<{ count: number }>('GET', '/notifications/unread_count', undefined, { requireAuth: true })
}

export function markNotificationRead(id: number) {
  return request('POST', `/notifications/${id}/read`, undefined, { requireAuth: true })
}

export function markAllRead() {
  return request('POST', '/notifications/read-all', undefined, { requireAuth: true })
}
