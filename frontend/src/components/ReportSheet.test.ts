// ReportSheet 举报弹窗单测（#33）：选原因 → 备注（「其他」必填）→ 提交；重复 409 透出服务端文案。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../api/report', () => ({
  createReport: vi.fn(),
  REPORT_REASONS: ['垃圾广告', '违法违规', '人身攻击', '抄袭侵权', '引战灌水', '其他'],
}))

import ReportSheet from './ReportSheet.vue'
import * as api from '../api/report'
import { showToast } from 'vant'

function mountSheet(over: Record<string, unknown> = {}) {
  return shallowMount(ReportSheet, {
    props: { show: true, targetType: 'post', targetId: 7, ...over },
    global: { renderStubDefaultSlot: true }, // 让 van-action-sheet stub 渲染插槽内容
  })
}

describe('ReportSheet 举报弹窗', () => {
  it('渲染六枚举原因（IA §5.6）', () => {
    const wrapper = mountSheet()
    expect(wrapper.findAll('.reason-item')).toHaveLength(6)
    expect(wrapper.text()).toContain('垃圾广告')
    expect(wrapper.text()).toContain('其他')
  })

  it('未选原因：不调接口 + 提示', async () => {
    const wrapper = mountSheet()
    await wrapper.find('.submit').trigger('click')
    expect(api.createReport).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalledWith('请选择举报原因')
  })

  it('选「其他」无备注：拦截', async () => {
    const wrapper = mountSheet()
    const items = wrapper.findAll('.reason-item')
    await items[5].trigger('click') // 「其他」
    await wrapper.find('.submit').trigger('click')
    expect(api.createReport).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalledWith('选「其他」时请填写说明')
  })

  it('选原因 + 备注：提交成功、关弹窗、toast', async () => {
    vi.mocked(api.createReport).mockResolvedValue({ id: 1, message: 'ok' })
    const wrapper = mountSheet()
    await wrapper.findAll('.reason-item')[0].trigger('click') // 垃圾广告
    await wrapper.find('textarea').setValue('广告链接')
    await wrapper.find('.submit').trigger('click')
    await flushPromises()
    expect(api.createReport).toHaveBeenCalledWith({
      target_type: 'post',
      target_id: 7,
      reason: '垃圾广告',
      note: '广告链接',
    })
    expect(wrapper.emitted('update:show')).toBeTruthy()
    expect(wrapper.emitted('submitted')).toBeTruthy()
    expect(showToast).toHaveBeenCalledWith('举报已提交，管理员会尽快处理')
  })

  it('接口失败（如 409 重复举报）：透出服务端文案', async () => {
    vi.mocked(api.createReport).mockRejectedValue(new Error('你已举报过该内容，请等待处理'))
    const wrapper = mountSheet()
    await wrapper.findAll('.reason-item')[0].trigger('click')
    await wrapper.find('.submit').trigger('click')
    await flushPromises()
    expect(showToast).toHaveBeenCalledWith('你已举报过该内容，请等待处理')
  })

  it('频控 429（#53）：透出「举报过于频繁」文案，不关弹窗', async () => {
    vi.mocked(api.createReport).mockRejectedValue(new Error('举报过于频繁，请稍后再试'))
    const wrapper = mountSheet()
    await wrapper.findAll('.reason-item')[0].trigger('click')
    await wrapper.find('.submit').trigger('click')
    await flushPromises()
    expect(showToast).toHaveBeenCalledWith('举报过于频繁，请稍后再试')
    expect(wrapper.emitted('update:show')).toBeFalsy() // 留在弹窗
  })
})
