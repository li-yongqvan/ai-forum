// TagDetail（#54 标签聚合页）：仿 PostDetail.test.ts 的 mock 模式（vue-router / api / vant）。
import { shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import PostList from '../components/PostList.vue'
import TagDetail from './TagDetail.vue'

// 可变路由参数：各用例挂载前改写（vue-router 会解码 %20 → 空格，故空名场景直接给空格）
const routeState = vi.hoisted(() => ({ name: 'AI' }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { name: routeState.name } }),
  useRouter: () => ({ push: vi.fn() }),
}))
// PostList 内部 import '../router' 的 goLoginWithReturn 与 auth store，均需 mock（仿 PostDetail.test.ts）
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
const authState = vi.hoisted(() => ({ isLoggedIn: false, user: null, isMod: false }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('../api/content', () => ({ listPosts: vi.fn() }))
vi.mock('vant', () => ({ showToast: vi.fn(), showConfirmDialog: vi.fn() }))

import * as api from '../api/content'

afterEach(() => {
  vi.clearAllMocks()
})

type Fetcher = (page: number, pageSize: number) => Promise<{ items: unknown[] }>

function mountTag() {
  return shallowMount(TagDetail, { global: { stubs: { PostList: true } } })
}

describe('TagDetail（#54 标签聚合页）', () => {
  it('头部显示归一化小写标签名', () => {
    routeState.name = 'AI'
    const wrapper = mountTag()
    expect(wrapper.find('.tname').text()).toContain('# ai')
  })

  it('fetcher 按标签请求 listPosts（归一化小写 + 分页透传）', async () => {
    routeState.name = 'AI'
    const wrapper = mountTag()
    const fetcher = wrapper.findComponent(PostList).props('fetcher') as Fetcher
    await fetcher(1, 20)
    expect(api.listPosts).toHaveBeenCalledWith({ tag: 'ai', page: 1, pageSize: 20 })
  })

  it('空名守卫（/tag/%20 解码为空格 → trim 后空）不发请求返回空（评审 F3）', async () => {
    routeState.name = ' '
    const wrapper = mountTag()
    const fetcher = wrapper.findComponent(PostList).props('fetcher') as Fetcher
    const res = await fetcher(1, 20)
    expect(res).toEqual({ items: [] })
    expect(api.listPosts).not.toHaveBeenCalled()
  })

  it('空态标题透传 PostList', () => {
    routeState.name = 'ai'
    const wrapper = mountTag()
    expect(wrapper.findComponent(PostList).props('emptyTitle')).toBe('还没有这个标签的帖子')
  })
})
