// #50 评论图片点击放大：验证 CommentTree.vue 的 `.cbody` @click 注入点（第二处注入点）。
// 用 mount + 真实 md（不 mock ../utils/md，否则不渲染 .content-img）；mock vant/stores/auth/utils/format，stub Avatar。
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CommentTree from './CommentTree.vue'
import type { CommentNode } from '../api/types'

vi.mock('vant', () => ({ showImagePreview: vi.fn(), showToast: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => ({ user: null, isMod: false }) }))
vi.mock('../utils/format', () => ({ formatTime: () => '刚刚' }))

import { showImagePreview } from 'vant'

function makeNode(id: number, over: Partial<CommentNode> = {}): CommentNode {
  return {
    id,
    post_id: 5,
    author_id: 2,
    author_name: `u${id}`,
    parent_id: null,
    floor: null,
    content: '内容',
    deleted: false,
    created_at: '2026-08-19T00:00:00Z',
    replies: [],
    ...over,
  }
}

describe('CommentTree 评论图片点击放大（#50）', () => {
  it('点评论第 2 张图 → showImagePreview 参数正确（startPosition=1、showIndex:true）', async () => {
    const c = makeNode(1, { content: 'https://a.com/1.png\n\nhttps://a.com/2.png' })
    const wrapper = mount(CommentTree, {
      props: { comments: [c], postId: 5 },
      global: { stubs: { Avatar: true } },
    })
    await flushPromises()
    const imgs = wrapper.findAll('.cbody img.content-img')
    expect(imgs.length).toBe(2)
    for (const w of imgs) {
      Object.defineProperty(w.element, 'complete', { configurable: true, value: true })
      Object.defineProperty(w.element, 'naturalWidth', { configurable: true, value: 100 })
    }
    await imgs[1].trigger('click')
    expect(showImagePreview).toHaveBeenCalledWith({
      images: ['https://a.com/1.png', 'https://a.com/2.png'],
      startPosition: 1,
      showIndex: true,
    })
  })

  it('跨评论隔离：评论 A 的图不被评论 B 收集（作用域=单条 .cbody）', async () => {
    const a = makeNode(1, { content: 'https://a.com/1.png' })
    const b = makeNode(2, { content: 'https://b.com/2.png' })
    const wrapper = mount(CommentTree, {
      props: { comments: [a, b], postId: 5 },
      global: { stubs: { Avatar: true } },
    })
    await flushPromises()
    const imgsA = wrapper.findAll('[data-comment-id="1"] .cbody img.content-img')
    const imgsB = wrapper.findAll('[data-comment-id="2"] .cbody img.content-img')
    expect(imgsA.length).toBe(1)
    expect(imgsB.length).toBe(1)
    for (const w of [...imgsA, ...imgsB]) {
      Object.defineProperty(w.element, 'complete', { configurable: true, value: true })
      Object.defineProperty(w.element, 'naturalWidth', { configurable: true, value: 100 })
    }
    // 点评论 A 的图：只收集 A 自己的图（B 的图不混入）
    await imgsA[0].trigger('click')
    expect(showImagePreview).toHaveBeenCalledWith({
      images: ['https://a.com/1.png'],
      startPosition: 0,
      showIndex: false,
    })
  })
})
