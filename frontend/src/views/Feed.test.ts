// Feed.vue #61 首页热门流：三分段 SegTabs + 热门公开 + 关注游客 LoginGuide。
// mock 范式仿 Me.test.ts / PostDetail.test.ts。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import LoginGuide from '../components/LoginGuide.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/feed' }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: vi.fn() }))
vi.mock('../api/content', () => ({
  listPosts: vi.fn(),
}))

import Feed from './Feed.vue'
import * as api from '../api/content'
import { useAuthStore } from '../stores/auth'

const auth = vi.mocked(useAuthStore)

function mountFeed(isLoggedIn: boolean) {
  auth.mockReturnValue({ isLoggedIn, user: isLoggedIn ? { id: 1 } : null, isMod: false } as never)
  vi.mocked(api.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  return shallowMount(Feed)
}

function labels(wrapper: ReturnType<typeof shallowMount>): string[] {
  return wrapper.findComponent(SegTabs).props('options').map((o: { label: string }) => o.label)
}

describe('Feed.vue #61 首页热门流', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('渲染三分段：全部/热门/关注', async () => {
    const wrapper = mountFeed(true)
    await flushPromises()
    expect(wrapper.findComponent(SegTabs).exists()).toBe(true)
    expect(labels(wrapper)).toEqual(['全部', '热门', '关注'])
  })

  it('热门 tab：fetcher 调 listPosts({tab:"hot"}) + 空态文案', async () => {
    const wrapper = mountFeed(true)
    await flushPromises()
    wrapper.findComponent(SegTabs).vm.$emit('update:modelValue', 'hot')
    await nextTick()
    const pl = wrapper.findComponent(PostList)
    expect(pl.exists()).toBe(true)
    expect(pl.props('emptyTitle')).toBe('暂无热门帖子')
    await pl.props('fetcher')(1, 20)
    expect(api.listPosts).toHaveBeenLastCalledWith({ tab: 'hot', page: 1, pageSize: 20 })
  })

  it('游客切热门：不显示 LoginGuide，仍可请求', async () => {
    const wrapper = mountFeed(false)
    await flushPromises()
    wrapper.findComponent(SegTabs).vm.$emit('update:modelValue', 'hot')
    await nextTick()
    expect(wrapper.findComponent(LoginGuide).exists()).toBe(false)
    expect(wrapper.findComponent(PostList).exists()).toBe(true)
  })

  it('游客切关注：显示 LoginGuide（回归）', async () => {
    const wrapper = mountFeed(false)
    await flushPromises()
    wrapper.findComponent(SegTabs).vm.$emit('update:modelValue', 'follow')
    await nextTick()
    expect(wrapper.findComponent(LoginGuide).exists()).toBe(true)
    expect(wrapper.findComponent(PostList).exists()).toBe(false)
  })

  it('全部 tab 空态文案保持默认', async () => {
    const wrapper = mountFeed(true)
    await flushPromises()
    const pl = wrapper.findComponent(PostList)
    expect(pl.props('emptyTitle')).toBe('还没有帖子')
  })
})
