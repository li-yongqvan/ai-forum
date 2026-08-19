// PostDetail 无评论场景回归测试：防 "Cannot read properties of null (reading 'reduce')" 回归。
// mock 刻意返回后端旧契约 {"comments":null}（空评论树被序列化为 null），
// 验证前端 `?? []` 防御 + 评论数 0 + 空状态渲染。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CommentTree from '../components/CommentTree.vue'
import Empty from '../components/Empty.vue'
import type { CommentTree as CommentTreeType, Post } from '../api/types'

// 注：PostDetail 依赖的 ../router 在模块加载时就有副作用（createRouter），
// 且 vue-router 需被 mock（测试环境没有 <Router> 上下文）→ 两个都 mock。
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '5' }, fullPath: '/post/5' }),
  useRouter: () => ({ replace: vi.fn() }),
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
vi.mock('vant', () => ({
  showToast: vi.fn(),
  showConfirmDialog: vi.fn(),
}))
vi.mock('../stores/auth', () => ({
  useAuthStore: () => ({ isLoggedIn: false, user: null, isMod: false }),
}))
vi.mock('../api/content', () => ({
  getPost: vi.fn(),
  getComments: vi.fn(),
}))

import PostDetail from './PostDetail.vue'
import * as api from '../api/content'

function mockPost(): Post {
  return {
    id: 5,
    board_id: 1,
    board_name: '学习讨论',
    topic_id: null,
    topic_name: null,
    author_id: 1,
    author_name: 'smoketest_user',
    author_avatar: null,
    title: '人工测试',
    content: '',
    is_pinned: false,
    is_featured: false,
    view_count: 0,
    like_count: 0,
    comment_count: 0,
    favorite_count: 0,
    created_at: '2026-08-19T00:00:00Z',
  }
}

describe('PostDetail 无评论场景', () => {
  it('后端返回 comments:null 时不崩溃：无渲染错误、评论数 0、渲染空状态', async () => {
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    // 刻意模拟后端旧契约（空评论树序列化为 null），tsconfig 覆盖 *.test.ts，需强转
    vi.mocked(api.getComments).mockResolvedValue({ post_id: 5, comments: null } as unknown as CommentTreeType)

    // 渲染错误（如 tree 被置 null 后 v-if="tree.length" 抛错）会进 errorHandler，
    // 断言无错 = 防回归的关键；否则该崩溃只表现为 unhandled rejection，测试会假阳性。
    const renderErrors: unknown[] = []
    const wrapper = shallowMount(PostDetail, {
      global: { config: { errorHandler: (err: unknown) => { renderErrors.push(err) } } },
    })
    await flushPromises()

    // 关键断言：无渲染错误（修复缺失时这里会捕到 "Cannot read properties of null (reading 'length')"）
    expect(renderErrors).toHaveLength(0)
    // 未崩溃：评论数渲染为 0
    expect(wrapper.find('.chead').text()).toContain('评论 0')
    // 空树 → 评论树组件不渲染，渲染空状态
    expect(wrapper.findComponent(CommentTree).exists()).toBe(false)
    const empty = wrapper.findComponent(Empty)
    expect(empty.exists()).toBe(true)
    expect(empty.props('title')).toBe('还没有评论')
  })
})
