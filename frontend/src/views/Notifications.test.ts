// Notifications 通知中心单测（#32）：列表渲染 / 点击已读+跳转 / 空态 / 全部已读。
// mock 模式仿 PostDetail.test.ts：vue-router、../router、vant、stores、api 全 mock。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AppNotification } from '../api/types'
import Empty from '../components/Empty.vue'

const push = vi.fn()
const notifyState = { unread: 1, refreshUnread: vi.fn(), clearUnread: vi.fn(), decrementUnread: vi.fn() }

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/notifications' }),
  useRouter: () => ({ push }),
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => ({ isLoggedIn: true, user: { id: 1 } }) }))
vi.mock('../stores/notify', () => ({ useNotifyStore: () => notifyState }))
vi.mock('../api/notify', () => ({
  listNotifications: vi.fn(),
  markNotificationRead: vi.fn(),
  markAllRead: vi.fn(),
  unreadCount: vi.fn(),
}))

import Notifications from './Notifications.vue'
import * as api from '../api/notify'

function followNotif(over: Partial<AppNotification> = {}): AppNotification {
  return {
    id: 1,
    type: 'follow',
    actor_id: 7,
    actor_name: 'bob',
    target_type: 'user',
    target_id: 7,
    is_read: false,
    created_at: '2026-08-20T10:00:00Z',
    ...over,
  }
}

async function mountWith(items: AppNotification[]) {
  vi.mocked(api.listNotifications).mockResolvedValue({ items, page: 1, page_size: 20 })
  const wrapper = shallowMount(Notifications)
  await flushPromises()
  return wrapper
}

describe('Notifications 通知中心', () => {
  it('渲染 follow 通知列表：文案 + 未读红点', async () => {
    const wrapper = await mountWith([followNotif()])
    expect(wrapper.text()).toContain('bob 关注了你')
    expect(wrapper.findAll('.item')).toHaveLength(1)
    expect(wrapper.find('.dot').exists()).toBe(true)
  })

  it('渲染 report_handled：被举报人视角文案原文透出（#53）', async () => {
    const wrapper = await mountWith([
      followNotif({
        id: 9,
        type: 'report_handled',
        actor_name: undefined,
        target_type: 'post',
        target_id: 9,
        target_title: '你的内容因「垃圾广告」被举报，已删除',
      }),
    ])
    expect(wrapper.text()).toContain('你的内容因「垃圾广告」被举报，已删除')
  })

  it('点击单条：跳转用户主页 + 标记已读 + 红点消失', async () => {
    const wrapper = await mountWith([followNotif()])
    await wrapper.find('.item').trigger('click')
    await flushPromises()
    expect(push).toHaveBeenCalledWith('/user/7')
    expect(api.markNotificationRead).toHaveBeenCalledWith(1)
    expect(wrapper.find('.dot').exists()).toBe(false)
  })

  it('空列表渲染空态', async () => {
    const wrapper = await mountWith([])
    const empty = wrapper.findComponent(Empty)
    expect(empty.exists()).toBe(true)
    expect(empty.props('title')).toBe('暂时没有新通知')
  })

  it('全部已读：调用接口并清除所有红点', async () => {
    const wrapper = await mountWith([followNotif(), followNotif({ id: 2, is_read: true })])
    expect(wrapper.findAll('.dot')).toHaveLength(1)
    await wrapper.find('.all-read').trigger('click')
    await flushPromises()
    expect(api.markAllRead).toHaveBeenCalled()
    expect(wrapper.findAll('.dot')).toHaveLength(0)
  })

  it('全部已读按钮在无未读时禁用', async () => {
    const wrapper = await mountWith([followNotif({ is_read: true })])
    const btn = wrapper.find('.all-read')
    expect(btn.attributes('disabled')).toBeDefined()
  })
})
