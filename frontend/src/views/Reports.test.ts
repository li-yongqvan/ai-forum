// Reports 举报处理队列单测（#33）：列表渲染 / 动作集按目标类型 / 处理提交 / 非 mod 守卫。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Report } from '../api/types'
import Empty from '../components/Empty.vue'

const authState = { isMod: true }
vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('../api/report', () => ({
  listReports: vi.fn(),
  handleReport: vi.fn(),
  countReports: vi.fn(),
}))

import Reports from './Reports.vue'
import * as api from '../api/report'
import { showToast } from 'vant'

beforeEach(() => {
  vi.clearAllMocks() // 清调用历史（保留 mockResolvedValue 实现）
})

function report(over: Partial<Report> = {}): Report {
  return {
    id: 1,
    reporter_id: 2,
    reporter_username: 'bob',
    target_type: 'post',
    target_id: 9,
    target_title: '被举报的帖子',
    reason: '垃圾广告',
    status: 'pending',
    created_at: '2026-08-20T10:00:00Z',
    ...over,
  }
}

async function mountWith(items: Report[]) {
  vi.mocked(api.listReports).mockResolvedValue({ items, page: 1, page_size: 100 })
  const wrapper = shallowMount(Reports, { global: { renderStubDefaultSlot: true } }) // 让 van-popup stub 渲染弹窗内容
  await flushPromises()
  return wrapper
}

describe('Reports 举报处理队列', () => {
  it('渲染待处理举报：原因 + 举报人 + 目标标题', async () => {
    const wrapper = await mountWith([report()])
    expect(wrapper.text()).toContain('垃圾广告')
    expect(wrapper.text()).toContain('举报人：bob')
    expect(wrapper.text()).toContain('被举报的帖子')
    expect(wrapper.findAll('.row')).toHaveLength(1)
  })

  it('空队列渲染空态', async () => {
    const wrapper = await mountWith([])
    expect(wrapper.findComponent(Empty).exists()).toBe(true)
    expect(wrapper.findComponent(Empty).props('title')).toBe('没有待处理的举报')
  })

  it('动作集按目标类型：帖子 → 忽略/删除帖子/警告（O4 明示仅留痕）', async () => {
    const wrapper = await mountWith([report()])
    await wrapper.find('.hbtn').trigger('click')
    const labels = wrapper.findAll('.act').map((w) => w.text())
    expect(labels).toEqual(['忽略', '删除帖子', '警告（会通知被举报人）'])
  })

  it('评论目标 → 删除评论动作', async () => {
    const wrapper = await mountWith([report({ id: 2, target_type: 'comment', target_id: 5, target_title: '评论内容' })])
    await wrapper.find('.hbtn').trigger('click')
    const labels = wrapper.findAll('.act').map((w) => w.text())
    expect(labels).toEqual(['忽略', '删除评论', '警告（会通知被举报人）'])
  })

  it('选择动作 + 备注 → 调 handleReport 并刷新', async () => {
    const wrapper = await mountWith([report()])
    await wrapper.find('.hbtn').trigger('click')
    await wrapper.findAll('.act')[1].trigger('click') // 删除帖子
    await wrapper.find('.note').setValue('违规内容')
    await wrapper.find('.submit').trigger('click')
    await flushPromises()
    expect(api.handleReport).toHaveBeenCalledWith(1, 'delete_post', '违规内容')
    expect(showToast).toHaveBeenCalledWith('已处理')
    expect(api.listReports).toHaveBeenCalledTimes(2) // 提交后刷新
  })

  it('非 mod：无权限空态', async () => {
    authState.isMod = false
    const wrapper = await mountWith([])
    expect(wrapper.findComponent(Empty).exists()).toBe(true)
    expect(wrapper.findComponent(Empty).props('title')).toBe('无权限访问')
    authState.isMod = true
  })
})
