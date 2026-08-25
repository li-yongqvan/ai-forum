// Chat 私信聊天页单测（#59）：消息渲染、发送回显、进入即标记已读。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppMessage } from '../api/types'
import Empty from '../components/Empty.vue'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { peerId: '2' } }),
  useRouter: () => ({ push }),
}))
vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => ({ isLoggedIn: true, user: { id: 1 } }) }))
vi.mock('../stores/notify', () => ({ useNotifyStore: () => ({ refreshUnread: vi.fn() }) }))
vi.mock('../api/notify', () => ({
  listConversationMessages: vi.fn(),
  markConversationRead: vi.fn(),
  sendMessage: vi.fn(),
}))

import Chat from './Chat.vue'
import * as api from '../api/notify'

function msg(over: Partial<AppMessage> = {}): AppMessage {
  return {
    id: 1,
    from_user_id: 2,
    to_user_id: 1,
    content: 'hello',
    is_read: true,
    created_at: '2026-08-20T10:00:00Z',
    ...over,
  }
}

async function mountChat(items: AppMessage[] = [msg()]) {
  vi.mocked(api.listConversationMessages).mockResolvedValue({ items, page: 1, page_size: 20 })
  const wrapper = shallowMount(Chat)
  await flushPromises()
  return wrapper
}

describe('Chat 私信页', () => {
  beforeEach(() => vi.clearAllMocks())

  it('挂载后加载消息并标记已读', async () => {
    await mountChat([msg()])
    expect(api.listConversationMessages).toHaveBeenCalledWith(2, { page: 1, pageSize: 20 })
    expect(api.markConversationRead).toHaveBeenCalledWith(2)
  })

  it('渲染对方与己方气泡', async () => {
    const wrapper = await mountChat([
      msg({ id: 1, from_user_id: 2, content: 'hi' }),
      msg({ id: 2, from_user_id: 1, content: 'hey' }),
    ])
    expect(wrapper.text()).toContain('hi')
    expect(wrapper.text()).toContain('hey')
    const rows = wrapper.findAll('.bubble-row')
    expect(rows).toHaveLength(2)
  })

  it('发送消息：清空输入并回显', async () => {
    vi.mocked(api.sendMessage).mockResolvedValue(msg({ id: 3, from_user_id: 1, content: 'new' }))
    const wrapper = await mountChat([])
    const input = wrapper.find('.input')
    await input.setValue('new')
    await wrapper.find('.send').trigger('click')
    await flushPromises()
    expect(api.sendMessage).toHaveBeenCalledWith(2, 'new')
    expect((input.element as HTMLInputElement).value).toBe('')
    expect(wrapper.text()).toContain('new')
  })

  it('发送空消息不调用 API', async () => {
    const wrapper = await mountChat([])
    await wrapper.find('.input').setValue('   ')
    await wrapper.find('.send').trigger('click')
    expect(api.sendMessage).not.toHaveBeenCalled()
  })

  it('空消息列表渲染空态', async () => {
    const wrapper = await mountChat([])
    const empty = wrapper.findComponent(Empty)
    expect(empty.exists()).toBe(true)
    expect(empty.props('title')).toBe('还没有消息')
  })
})
