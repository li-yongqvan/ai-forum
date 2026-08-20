import { request } from './client'
import type { Page, Report, ReportStatus, ReportTargetType } from './types'

// ---- 举报/治理（#33：举报创建需登录；队列/处理需 mod 组，接口按 403 归一） ----

/** 举报原因枚举（IA v2 §5.6：D4；「其他」时备注必填）。 */
export const REPORT_REASONS = ['垃圾广告', '违法违规', '人身攻击', '抄袭侵权', '引战灌水', '其他'] as const

export interface CreateReportPayload {
  target_type: ReportTargetType
  target_id: number
  reason: string
  note?: string
}

export function createReport(payload: CreateReportPayload) {
  return request<{ id: number; message: string }>('POST', '/reports', payload, { requireAuth: true })
}

export interface ListReportsParams {
  status?: ReportStatus
  page?: number
  pageSize?: number
}

export function listReports(p: ListReportsParams = {}) {
  const qs = new URLSearchParams()
  qs.set('status', p.status ?? 'pending')
  qs.set('page', String(p.page ?? 1))
  qs.set('page_size', String(p.pageSize ?? 20))
  return request<Page<Report>>('GET', `/moderation/reports?${qs.toString()}`, undefined, { requireAuth: true })
}

/** 待处理数（Me 页徽章）。 */
export function countReports(status: ReportStatus = 'pending') {
  return request<{ count: number }>('GET', `/moderation/reports/count?status=${status}`, undefined, { requireAuth: true })
}

export function handleReport(id: number, action: string, note?: string) {
  return request('POST', `/moderation/reports/${id}/handle`, { action, note }, { requireAuth: true })
}
