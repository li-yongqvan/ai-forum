// Me.vue #23 收藏/关注列表：三段 SegTabs + 关注内嵌子分段 + 列表装配。
// mock 范式仿 PostDetail.test.ts；store 以 callable 形式 mock 以便按测试覆盖游客/登录。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import FollowList from '../components/FollowList.vue'
import LoginGuide from '../components/LoginGuide.vue'
import type { UserProfile } from '../api/types'

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/me' }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
vi.mock('vant', () => ({ showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: vi.fn() }))
vi.mock('../api/content', () => ({
  getUserProfile: vi.fn(),
  listPosts: vi.fn(),
  listFavorites: vi.fn(),
  listFollows: vi.fn(),
  unfollow: vi.fn(),
  favorite: vi.fn(),
  unfavorite: vi.fn(),
}))

import Me from './Me.vue'
import * as api from '../api/content'
import { useAuthStore } from '../stores/auth'

const auth = vi.mocked(useAuthStore)

function mockProfile(): UserProfile {
  return {
    id: 1,
    username: 'alice',
    avatar_url: null,
    bio: null,
    joined_at: '2026-08-01T00:00:00Z',
    post_count: 0,
    follower_count: 0,
    following_count: 0,
  }
}

function mountMe(isLoggedIn: boolean) {
  auth.mockReturnValue({ isLoggedIn, user: isLoggedIn ? { id: 1 } : null, isMod: false } as never)
  vi.mocked(api.getUserProfile).mockResolvedValue(mockProfile())
  vi.mocked(api.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  vi.mocked(api.listFavorites).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  vi.mocked(api.listFollows).mockResolvedValue({ items: [], page: 1, page_size: 20 })
  return shallowMount(Me)
}

// FollowList 是泛型 SFC，其类型不满足 FindComponentSelector，findComponent 需 `as any`（vue-tsc 泛型组件限制）。
const FL = FollowList as any

function labels(wrapper: ReturnType<typeof shallowMount>, idx: number): string[] {
  return wrapper
    .findAllComponents(SegTabs)[idx]
    .props('options')
    .map((o: { label: string }) => o.label)
}

describe('Me.vue #23 收藏/关注列表', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('游客：渲染 LoginGuide，无 SegTabs/PostList', async () => {
    const wrapper = mountMe(false)
    await flushPromises()
    expect(wrapper.findComponent(LoginGuide).exists()).toBe(true)
    expect(wrapper.findComponent(SegTabs).exists()).toBe(false)
    expect(wrapper.findComponent(PostList).exists()).toBe(false)
  })

  it('登录默认帖子 tab：资料卡 + 我的帖子 fetcher', async () => {
    const wrapper = mountMe(true)
    await flushPromises()
    expect(api.getUserProfile).toHaveBeenCalledWith(1)
    expect(wrapper.findAllComponents(SegTabs)).toHaveLength(1)
    expect(labels(wrapper, 0)).toEqual(['帖子', '收藏', '关注'])
    const pl = wrapper.findComponent(PostList)
    expect(pl.exists()).toBe(true)
    expect(pl.props('emptyTitle')).toBe('还没有发布过帖子')
    await pl.props('fetcher')(1, 20)
    expect(api.listPosts).toHaveBeenCalledWith({ authorId: 1, page: 1, pageSize: 20 })
  })

  it('收藏 tab：fetcher 调 listFavorites + 空态文案', async () => {
    const wrapper = mountMe(true)
    await flushPromises()
    wrapper.findAllComponents(SegTabs)[0].vm.$emit('update:modelValue', 'favs')
    await nextTick()
    const pl = wrapper.findComponent(PostList)
    expect(pl.props('emptyTitle')).toBe('还没有收藏帖子')
    await pl.props('fetcher')(1, 20)
    expect(api.listFavorites).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
  })

  it('关注 tab：嵌套 SegTabs（用户/板块/话题）+ 用户列表装配', async () => {
    const wrapper = mountMe(true)
    await flushPromises()
    wrapper.findAllComponents(SegTabs)[0].vm.$emit('update:modelValue', 'follows')
    await nextTick()
    expect(wrapper.findAllComponents(SegTabs)).toHaveLength(2)
    expect(labels(wrapper, 1)).toEqual(['用户', '板块', '话题'])
    const fl = wrapper.findComponent(FL)
    expect(fl.exists()).toBe(true)
    expect(fl.props('emptyTitle')).toBe('还没有关注用户')
    await fl.props('fetcher')(1, 20)
    expect(api.listFollows).toHaveBeenCalledWith('user', { page: 1, pageSize: 20 })
  })

  it('关注子分段切换：板块/话题 fetcher', async () => {
    const wrapper = mountMe(true)
    await flushPromises()
    wrapper.findAllComponents(SegTabs)[0].vm.$emit('update:modelValue', 'follows')
    await nextTick()
    const inner = wrapper.findAllComponents(SegTabs)[1]

    inner.vm.$emit('update:modelValue', 'boards')
    await nextTick()
    let fl = wrapper.findComponent(FL)
    await fl.props('fetcher')(1, 20)
    expect(api.listFollows).toHaveBeenCalledWith('board', { page: 1, pageSize: 20 })

    inner.vm.$emit('update:modelValue', 'topics')
    await nextTick()
    fl = wrapper.findComponent(FL)
    await fl.props('fetcher')(1, 20)
    expect(api.listFollows).toHaveBeenCalledWith('topic', { page: 1, pageSize: 20 })
  })

  it('关注行取消关注：unfollow prop 构造正确 target_type/target_id', async () => {
    const wrapper = mountMe(true)
    await flushPromises()
    wrapper.findAllComponents(SegTabs)[0].vm.$emit('update:modelValue', 'follows')
    await nextTick()
    const fl = wrapper.findComponent(FL)
    await fl.props('unfollow')({ id: 2, username: 'bob' })
    expect(api.unfollow).toHaveBeenCalledWith({ target_type: 'user', target_id: 2 })
  })
})
