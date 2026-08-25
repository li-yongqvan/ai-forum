// Notifications 通知中心单测（#32）：列表渲染 / 点击已读+跳转 / 空态 / 全部已读。
// mock 模式仿 PostDetail.test.ts：vue-router、../router、vant、stores、api 全 mock。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AppNotification, Conversation } from '../api/types'
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
  listConversations: vi.fn(),
  markConversationRead: vi.fn(),
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

function conv(over: Partial<Conversation> = {}): Conversation {
  return {
    peer_id: 2,
    peer_name: 'alice',
    peer_avatar: undefined,
    last_message: {
      id: 1,
      from_user_id: 2,
      to_user_id: 1,
      content: 'hi',
      is_read: false,
      created_at: '2026-08-20T10:00:00Z',
    },
    unread_count: 1,
    ...over,
  }
}

async function mountWith(items: AppNotification[]) {
  vi.mocked(api.listNotifications).mockResolvedValue({ items, page: 1, page_size: 20 })
  vi.mocked(api.listConversations).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  const wrapper = shallowMount(Notifications, { global: { stubs: { SegTabs: false } } })
  await flushPromises()
  return wrapper
}

async function mountWithConversations(conversations: Conversation[]) {
  vi.mocked(api.listNotifications).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  vi.mocked(api.listConversations).mockResolvedValue({ items: conversations, page: 1, page_size: 20 })
  const wrapper = shallowMount(Notifications, { global: { stubs: { SegTabs: false } } })
  await flushPromises()
  return wrapper
}

async function switchToMsg(wrapper: ReturnType<typeof shallowMount>) {
  const tabs = wrapper.findAll('.seg-item')
  const msgTab = tabs.find((w) => w.text() === '私信')
  await msgTab!.trigger('click')
  await flushPromises()
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

  it('全部已读：调用接口并刷新 badge（合并语义，评审 F4）', async () => {
    const wrapper = await mountWith([followNotif(), followNotif({ id: 2, is_read: true })])
    expect(wrapper.findAll('.dot')).toHaveLength(1)
    await wrapper.find('.all-read').trigger('click')
    await flushPromises()
    expect(api.markAllRead).toHaveBeenCalled()
    expect(notifyState.refreshUnread).toHaveBeenCalled()
    expect(wrapper.findAll('.dot')).toHaveLength(0)
  })

  it('全部已读按钮在无未读时禁用', async () => {
    const wrapper = await mountWith([followNotif({ is_read: true })])
    const btn = wrapper.find('.all-read')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('私信 tab：渲染会话列表 + 未读徽标', async () => {
    const wrapper = await mountWithConversations([conv()])
    await switchToMsg(wrapper)
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('hi')
    expect(wrapper.find('.dot').exists()).toBe(true)
  })

  it('私信 tab：点击会话跳转聊天页', async () => {
    const wrapper = await mountWithConversations([conv()])
    await switchToMsg(wrapper)
    await wrapper.find('.item').trigger('click')
    expect(push).toHaveBeenCalledWith('/messages/2')
  })

  it('私信 tab：本人发的消息预览带「我:」前缀', async () => {
    const wrapper = await mountWithConversations([
      conv({ last_message: { ...conv().last_message, from_user_id: 1, content: 'ok' } }),
    ])
    await switchToMsg(wrapper)
    expect(wrapper.text()).toContain('我: ok')
  })

  it('私信 tab：空会话渲染空态', async () => {
    const wrapper = await mountWithConversations([])
    await switchToMsg(wrapper)
    const empty = wrapper.findComponent(Empty)
    expect(empty.exists()).toBe(true)
    expect(empty.props('title')).toBe('还没有私信')
  })
})
