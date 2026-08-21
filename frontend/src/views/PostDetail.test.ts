// PostDetail 无评论场景回归测试：防 "Cannot read properties of null (reading 'reduce')" 回归。
// mock 刻意返回后端旧契约 {"comments":null}（空评论树被序列化为 null），
// 验证前端 `?? []` 防御 + 评论数 0 + 空状态渲染。
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import CommentTree from '../components/CommentTree.vue'
import Empty from '../components/Empty.vue'
import type { CommentNode as CommentNodeType, CommentTree as CommentTreeType, Post } from '../api/types'

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
// 可变登录态：回复流用例需切 isLoggedIn=true（现有用例默认 false，向后兼容）
const authState = vi.hoisted(() => ({ isLoggedIn: false, user: null, isMod: false }))
vi.mock('../stores/auth', () => ({
  useAuthStore: () => authState,
}))
vi.mock('../api/content', () => ({
  getPost: vi.fn(),
  getComments: vi.fn(),
  createComment: vi.fn(),
}))

import PostDetail from './PostDetail.vue'
import * as api from '../api/content'
import { showToast } from 'vant'

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

function makeCommentNode(id: number, replies: CommentNodeType[] = [], parentId: number | null = null): CommentNodeType {
  return {
    id,
    post_id: 5,
    author_id: 1,
    author_name: `u${id}`,
    parent_id: parentId,
    floor: null,
    content: `内容${id}`,
    deleted: false,
    created_at: '2026-08-19T00:00:00Z',
    replies,
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

describe('PostDetail 分享（Web Share API + 复制兜底）', () => {
  const postUrl = `${location.origin}/#/post/5`

  // jsdom 无原生 share/clipboard：以 configurable 属性注入，逐用例覆盖成功/取消/报错/不支持
  function setShare(impl: undefined | ((data: ShareData) => Promise<void>)) {
    Object.defineProperty(navigator, 'share', { configurable: true, value: impl })
  }
  function setClipboard(writeText: ((url: string) => void) | undefined) {
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  }

  afterEach(() => {
    // 还原为未定义，避免污染同文件其他用例
    setShare(undefined)
    setClipboard(undefined)
    vi.clearAllMocks()
  })

  async function mountAndClickShare() {
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    vi.mocked(api.getComments).mockResolvedValue({ post_id: 5, comments: [] })
    const wrapper = shallowMount(PostDetail)
    await flushPromises()
    const btn = wrapper.findAll('button.dact').find((b) => b.text().includes('分享'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await flushPromises()
    return wrapper
  }

  it('支持原生分享：调用 navigator.share(title,url)，不再复制链接/弹提示', async () => {
    const shareSpy = vi.fn().mockResolvedValue(undefined)
    const clipboardSpy = vi.fn()
    setShare(shareSpy)
    setClipboard(clipboardSpy)

    await mountAndClickShare()

    expect(shareSpy).toHaveBeenCalledWith({ title: '人工测试', url: postUrl })
    expect(clipboardSpy).not.toHaveBeenCalled()
    expect(showToast).not.toHaveBeenCalled()
  })

  it('用户取消原生分享（AbortError）：静默结束，不复制、不提示', async () => {
    const shareSpy = vi.fn().mockRejectedValue(Object.assign(new Error('canceled'), { name: 'AbortError' }))
    const clipboardSpy = vi.fn()
    setShare(shareSpy)
    setClipboard(clipboardSpy)

    await mountAndClickShare()

    expect(shareSpy).toHaveBeenCalled()
    expect(clipboardSpy).not.toHaveBeenCalled()
    expect(showToast).not.toHaveBeenCalled()
  })

  it('原生分享抛出其他错误：回退复制链接并提示', async () => {
    const shareSpy = vi.fn().mockRejectedValue(new Error('NotAllowedError'))
    const clipboardSpy = vi.fn()
    setShare(shareSpy)
    setClipboard(clipboardSpy)

    await mountAndClickShare()

    expect(clipboardSpy).toHaveBeenCalledWith(postUrl)
    expect(showToast).toHaveBeenCalledWith('链接已复制')
  })

  it('不支持原生分享：回退复制链接并提示', async () => {
    const clipboardSpy = vi.fn()
    setShare(undefined)
    setClipboard(clipboardSpy)

    await mountAndClickShare()

    expect(clipboardSpy).toHaveBeenCalledWith(postUrl)
    expect(showToast).toHaveBeenCalledWith('链接已复制')
  })

  it('复制链接也失败：toast 直接展示链接兜底', async () => {
    setShare(undefined)
    setClipboard(() => {
      throw new Error('denied')
    })

    await mountAndClickShare()

    expect(showToast).toHaveBeenCalledWith(`链接：${postUrl}`)
  })
})

describe('PostDetail 回复评论：自动展开分支 + 滚动定位新评论（D4）', () => {
  afterEach(() => {
    vi.restoreAllMocks() // 还原 document.querySelector spy
    vi.clearAllMocks() // 清所有 vi.fn()（createComment/getComments/getPost）的 call 历史
    authState.isLoggedIn = false
  })

  it('回复二级评论：展开其一级分支、滚动定位新评论并高亮', async () => {
    authState.isLoggedIn = true
    const treeBefore: CommentTreeType = { post_id: 5, comments: [makeCommentNode(1, [makeCommentNode(2)])] }
    const treeAfter: CommentTreeType = {
      post_id: 5,
      comments: [makeCommentNode(1, [makeCommentNode(2, [makeCommentNode(999, [], 2)])])],
    }
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    vi.mocked(api.getComments).mockResolvedValueOnce(treeBefore).mockResolvedValueOnce(treeAfter)
    vi.mocked(api.createComment).mockResolvedValue({ id: 999 })

    const fakeEl = { scrollIntoView: vi.fn(), classList: { add: vi.fn(), remove: vi.fn() } }
    const origQS = document.querySelector.bind(document)
    vi.spyOn(document, 'querySelector').mockImplementation((sel) =>
      sel === '[data-comment-id="999"]' ? (fakeEl as unknown as Element) : origQS(sel as string),
    )

    const wrapper = shallowMount(PostDetail)
    await flushPromises()

    // 触发回复二级评论（id=2）
    wrapper.findComponent(CommentTree).vm.$emit('reply', 2, 'alice')
    await wrapper.find('.cinput').setValue('新回复')
    await wrapper.find('.sbtn').trigger('click')
    await flushPromises()
    await nextTick()

    expect(api.createComment).toHaveBeenCalledWith({ post_id: 5, parent_id: 2, content: '新回复' })
    // shallowMount 下 CommentTree 是 stub：不实现 watcher，expandRootId 保持 1 可断言
    expect(wrapper.findComponent(CommentTree).props('expandRootId')).toBe(1)
    expect(fakeEl.scrollIntoView).toHaveBeenCalledWith({ block: 'center' })
    expect(fakeEl.classList.add).toHaveBeenCalledWith('comment-flash')
  })

  it('同分支二次回复：expandRootId 再次 = 根 id（闭环可重复触发，F2）', async () => {
    authState.isLoggedIn = true
    const treeBefore: CommentTreeType = { post_id: 5, comments: [makeCommentNode(1, [makeCommentNode(2)])] }
    const treeAfter1: CommentTreeType = {
      post_id: 5,
      comments: [makeCommentNode(1, [makeCommentNode(2, [makeCommentNode(999, [], 2)])])],
    }
    const treeAfter2: CommentTreeType = {
      post_id: 5,
      comments: [
        makeCommentNode(1, [makeCommentNode(2, [makeCommentNode(999, [], 2), makeCommentNode(1000, [], 2)])]),
      ],
    }
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    vi.mocked(api.getComments)
      .mockResolvedValueOnce(treeBefore)
      .mockResolvedValueOnce(treeAfter1)
      .mockResolvedValueOnce(treeAfter2)
    vi.mocked(api.createComment).mockResolvedValueOnce({ id: 999 }).mockResolvedValueOnce({ id: 1000 })
    // shallowMount 下无真实评论 DOM：revealComment 的 querySelector 会走 console.warn 分支，静默掉
    vi.spyOn(console, 'warn').mockImplementation(() => {})

    const wrapper = shallowMount(PostDetail)
    await flushPromises()

    // 第一次回复
    wrapper.findComponent(CommentTree).vm.$emit('reply', 2, 'alice')
    await wrapper.find('.cinput').setValue('第一条')
    await wrapper.find('.sbtn').trigger('click')
    await flushPromises()
    await nextTick()
    expect(wrapper.findComponent(CommentTree).props('expandRootId')).toBe(1)

    // 第二次回复（同分支）
    wrapper.findComponent(CommentTree).vm.$emit('reply', 2, 'alice')
    await wrapper.find('.cinput').setValue('第二条')
    await wrapper.find('.sbtn').trigger('click')
    await flushPromises()
    await nextTick()

    expect(api.createComment).toHaveBeenCalledTimes(2)
    expect(wrapper.findComponent(CommentTree).props('expandRootId')).toBe(1)
  })

  it('兜底-回复：createComment 无 id 时取父节点末子并滚动（F1/Q2）', async () => {
    authState.isLoggedIn = true
    const treeBefore: CommentTreeType = { post_id: 5, comments: [makeCommentNode(1, [makeCommentNode(2)])] }
    const treeAfter: CommentTreeType = {
      post_id: 5,
      comments: [makeCommentNode(1, [makeCommentNode(2, [makeCommentNode(999, [], 2)])])],
    }
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    vi.mocked(api.getComments).mockResolvedValueOnce(treeBefore).mockResolvedValueOnce(treeAfter)
    vi.mocked(api.createComment).mockResolvedValue({} as unknown as { id: number }) // 无 id

    const fakeEl = { scrollIntoView: vi.fn(), classList: { add: vi.fn(), remove: vi.fn() } }
    const origQS = document.querySelector.bind(document)
    vi.spyOn(document, 'querySelector').mockImplementation((sel) =>
      sel === '[data-comment-id="999"]' ? (fakeEl as unknown as Element) : origQS(sel as string),
    )

    const wrapper = shallowMount(PostDetail)
    await flushPromises()
    wrapper.findComponent(CommentTree).vm.$emit('reply', 2, 'alice')
    await wrapper.find('.cinput').setValue('新回复')
    await wrapper.find('.sbtn').trigger('click')
    await flushPromises()
    await nextTick()

    // 兜底命中 999（父 2 的末子）→ 滚动到它
    expect(document.querySelector).toHaveBeenCalledWith('[data-comment-id="999"]')
    expect(fakeEl.scrollIntoView).toHaveBeenCalledWith({ block: 'center' })
  })

  it('兜底-顶层：直接回复帖子无 id 时取 tree 末元素（F1/Q2）', async () => {
    authState.isLoggedIn = true
    const treeBefore: CommentTreeType = { post_id: 5, comments: [makeCommentNode(1)] }
    const treeAfter: CommentTreeType = {
      post_id: 5,
      comments: [makeCommentNode(1), makeCommentNode(999, [], null)],
    }
    vi.mocked(api.getPost).mockResolvedValue(mockPost())
    vi.mocked(api.getComments).mockResolvedValueOnce(treeBefore).mockResolvedValueOnce(treeAfter)
    vi.mocked(api.createComment).mockResolvedValue({} as unknown as { id: number }) // 无 id

    const fakeEl = { scrollIntoView: vi.fn(), classList: { add: vi.fn(), remove: vi.fn() } }
    const origQS = document.querySelector.bind(document)
    vi.spyOn(document, 'querySelector').mockImplementation((sel) =>
      sel === '[data-comment-id="999"]' ? (fakeEl as unknown as Element) : origQS(sel as string),
    )

    const wrapper = shallowMount(PostDetail)
    await flushPromises()
    // 直接回复帖子（不设 replyTo）→ parentId null → 产生新一级评论
    await wrapper.find('.cinput').setValue('新顶层')
    await wrapper.find('.sbtn').trigger('click')
    await flushPromises()
    await nextTick()

    expect(api.createComment).toHaveBeenCalledWith({ post_id: 5, parent_id: null, content: '新顶层' })
    // 顶层回复（parentId null）不展开分支：expandRootId 保持 null
    expect(wrapper.findComponent(CommentTree).props('expandRootId')).toBe(null)
    // 兜底命中 999（tree 末元素）→ 滚动到它
    expect(document.querySelector).toHaveBeenCalledWith('[data-comment-id="999"]')
    expect(fakeEl.scrollIntoView).toHaveBeenCalledWith({ block: 'center' })
  })
})
