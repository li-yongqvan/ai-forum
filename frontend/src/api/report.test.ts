// api/report 单测（#33）：统一 request 封装（requireAuth + 路径 + query 拼装）。
import { describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({ request: vi.fn() }))

import * as api from './report'
import { request } from './client'

const mocked = vi.mocked(request)

describe('api/report', () => {
  it('createReport 提交举报（POST /reports, requireAuth）', () => {
    api.createReport({ target_type: 'post', target_id: 7, reason: '垃圾广告', note: '备注' })
    expect(mocked).toHaveBeenCalledWith(
      'POST',
      '/reports',
      { target_type: 'post', target_id: 7, reason: '垃圾广告', note: '备注' },
      { requireAuth: true },
    )
  })

  it('listReports 拼 query（status/page/page_size）', () => {
    api.listReports({ status: 'pending', page: 2, pageSize: 50 })
    expect(mocked).toHaveBeenCalledWith(
      'GET',
      '/moderation/reports?status=pending&page=2&page_size=50',
      undefined,
      { requireAuth: true },
    )
  })

  it('countReports 默认 status=pending', () => {
    api.countReports()
    expect(mocked).toHaveBeenCalledWith('GET', '/moderation/reports/count?status=pending', undefined, {
      requireAuth: true,
    })
  })

  it('handleReport 传 action + note', () => {
    api.handleReport(3, 'warn', '备注')
    expect(mocked).toHaveBeenCalledWith(
      'POST',
      '/moderation/reports/3/handle',
      { action: 'warn', note: '备注' },
      { requireAuth: true },
    )
  })

  it('REPORT_REASONS 为 IA §5.6 六枚举', () => {
    expect(api.REPORT_REASONS).toEqual(['垃圾广告', '违法违规', '人身攻击', '抄袭侵权', '引战灌水', '其他'])
  })
})
