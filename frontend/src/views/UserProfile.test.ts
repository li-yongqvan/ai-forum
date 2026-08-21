// UserProfile #34 单测：封禁/解封入口（仅 admin）、封禁徽标、用户举报入口（ReportSheet）、
// 提交封禁/解封调 banUser/unbanUser（评审 F8：auth store mock 含 isAdmin）。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UserProfile as Profile } from '../api/types'

const authState: { isLoggedIn: boolean; isAdmin: boolean; user: { id: number } | null } = {
  isLoggedIn: true,
  isAdmin: true,
  user: { id: 1 },
}

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '9' }, fullPath: '/user/9' }),
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
// 受控 van-dialog stub：渲染 default slot 并暴露 props，测试直接驱动 beforeClose。
vi.mock('vant', async (importOriginal) => {
  const { defineComponent, h } = await import('vue')
  const VanDialogStub = defineComponent({
    name: 'VanDialog',
    props: ['show', 'title', 'beforeClose'],
    setup(_props, { slots }) {
      return () => h('div', { class: 'dialog-stub' }, [slots.default?.()])
    },
  })
  return {
    ...(await importOriginal<typeof import('vant')>()),
    showToast: vi.fn(),
    Dialog: VanDialogStub,
  }
})
vi.mock('../stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('../api/content', () => ({
  getUserProfile: vi.fn(),
  listPosts: vi.fn(),
  follow: vi.fn(),
  unfollow: vi.fn(),
}))
vi.mock('../api/report', () => ({ banUser: vi.fn(), unbanUser: vi.fn() }))

import UserProfile from './UserProfile.vue'
import * as contentApi from '../api/content'
import * as reportApi from '../api/report'
import { goLoginWithReturn } from '../router'
import { showToast } from 'vant'

function profile(over: Partial<Profile> = {}): Profile {
  return {
    id: 9,
    username: 'bob',
    avatar_url: null,
    bio: null,
    joined_at: '2026-08-01T00:00:00Z',
    post_count: 0,
    follower_count: 0,
    following_count: 0,
    ...over,
  }
}

async function mountWith(p: Profile) {
  vi.mocked(contentApi.getUserProfile).mockResolvedValue(p)
  vi.mocked(contentApi.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  const wrapper = shallowMount(UserProfile, { global: { renderStubDefaultSlot: true } })
  await flushPromises()
  return wrapper
}

function banButton(wrapper: ReturnType<typeof shallowMount>) {
  return wrapper.findAll('.btn').find((w) => w.text() === '封禁' || w.text() === '解封')
}
function reportButton(wrapper: ReturnType<typeof shallowMount>) {
  return wrapper.findAll('.btn').find((w) => w.text() === '举报')
}

beforeEach(() => vi.clearAllMocks())

describe('UserProfile #34 封禁/解封', () => {
  it('非 admin：不渲染封禁/解封按钮', async () => {
    authState.isAdmin = false
    const wrapper = await mountWith(profile())
    expect(banButton(wrapper)).toBeUndefined()
    authState.isAdmin = true
  })

  it('admin + 未封禁：按钮文案「封禁」、无徽标', async () => {
    authState.isAdmin = true
    const wrapper = await mountWith(profile())
    expect(banButton(wrapper)?.text()).toBe('封禁')
    expect(wrapper.text()).not.toContain('该用户已封禁')
  })

  it('admin + 已封禁：按钮文案「解封」+ 封禁徽标', async () => {
    authState.isAdmin = true
    const wrapper = await mountWith(profile({ banned: true }))
    expect(banButton(wrapper)?.text()).toBe('解封')
    expect(wrapper.text()).toContain('该用户已封禁')
  })

  it('点封禁 → 弹窗开；空原因不调 API，填原因调 banUser', async () => {
    authState.isAdmin = true
    const wrapper = await mountWith(profile())
    await banButton(wrapper)!.trigger('click')
    const dialog = wrapper.findComponent({ name: 'VanDialog' })
    expect(dialog.exists()).toBe(true)
    const beforeClose = dialog.props('beforeClose') as (action: string) => unknown

    beforeClose('confirm') // 空原因
    expect(reportApi.banUser).not.toHaveBeenCalled()

    await wrapper.find('.ban-reason').setValue('人身攻击')
    const res = beforeClose('confirm')
    if (res instanceof Promise) await res
    expect(reportApi.banUser).toHaveBeenCalledWith(9, '人身攻击')
    expect(showToast).toHaveBeenCalledWith('已封禁')
  })

  it('点解封 → 填原因调 unbanUser', async () => {
    authState.isAdmin = true
    const wrapper = await mountWith(profile({ banned: true }))
    await banButton(wrapper)!.trigger('click')
    const dialog = wrapper.findComponent({ name: 'VanDialog' })
    const beforeClose = dialog.props('beforeClose') as (action: string) => unknown
    await wrapper.find('.ban-reason').setValue('误封纠正')
    const res = beforeClose('confirm')
    if (res instanceof Promise) await res
    expect(reportApi.unbanUser).toHaveBeenCalledWith(9, '误封纠正')
    expect(showToast).toHaveBeenCalledWith('已解封')
  })

  it('未登录点举报 → 引导登录', async () => {
    authState.isLoggedIn = false
    const wrapper = await mountWith(profile())
    await reportButton(wrapper)!.trigger('click')
    expect(goLoginWithReturn).toHaveBeenCalled()
    authState.isLoggedIn = true
  })

  it('已登录点举报 → 打开 ReportSheet（target-type=user）', async () => {
    authState.isLoggedIn = true
    const wrapper = await mountWith(profile())
    await reportButton(wrapper)!.trigger('click')
    const sheet = wrapper.findComponent({ name: 'ReportSheet' })
    expect(sheet.exists()).toBe(true)
    expect(sheet.props('targetType')).toBe('user')
    expect(sheet.props('targetId')).toBe(9)
  })
})
