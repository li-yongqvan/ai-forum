import { request } from './client'
import type { AppNotification, AppMessage, Conversation } from './types'

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

/** 未读数（底部 Tab badge）。自 #59 起 = 通知未读 + 私信未读合并值。 */
export function unreadCount() {
  return request<{ count: number }>('GET', '/notifications/unread_count', undefined, { requireAuth: true })
}

export function markNotificationRead(id: number) {
  return request('POST', `/notifications/${id}/read`, undefined, { requireAuth: true })
}

export function markAllRead() {
  return request('POST', '/notifications/read-all', undefined, { requireAuth: true })
}

// ---- 私信（#59） ----

export interface ListConversationsParams {
  page?: number
  pageSize?: number
}

export interface ConversationList {
  items: Conversation[]
  page: number
  page_size: number
}

export interface MessageList {
  items: AppMessage[]
  page: number
  page_size: number
}

export function listConversations(p: ListConversationsParams = {}) {
  const qs = new URLSearchParams()
  qs.set('page', String(p.page ?? 1))
  qs.set('page_size', String(p.pageSize ?? 20))
  return request<ConversationList>('GET', `/conversations?${qs.toString()}`, undefined, { requireAuth: true })
}

export function listConversationMessages(peerId: number, p: ListConversationsParams = {}) {
  const qs = new URLSearchParams()
  qs.set('page', String(p.page ?? 1))
  qs.set('page_size', String(p.pageSize ?? 20))
  return request<MessageList>('GET', `/conversations/${peerId}/messages?${qs.toString()}`, undefined, { requireAuth: true })
}

export function sendMessage(toUserId: number, content: string) {
  return request<AppMessage>('POST', '/messages', { to_user_id: toUserId, content }, { requireAuth: true })
}

export function markConversationRead(peerId: number) {
  return request('POST', `/conversations/${peerId}/read`, undefined, { requireAuth: true })
}
