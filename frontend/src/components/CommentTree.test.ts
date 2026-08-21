// CommentTree #47 深度截断测试：默认只显一级、展开/收起、N 统计、深度 3 封顶压平、回复按钮深度限制、删除占位。
// 用 mount（非 shallow）：组件自递归，shallow 会 stub 掉子实例、测不到深度行为。
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CommentTree from './CommentTree.vue'
import type { CommentNode } from '../api/types'

vi.mock('../stores/auth', () => ({ useAuthStore: () => ({ user: null, isMod: false }) }))
// md 走 v-html：mock 成简单 HTML 片段，避免真实 md 依赖；formatTime 静态化
vi.mock('../utils/md', () => ({ md: (s: string) => `<p>${s}</p>` }))
vi.mock('../utils/format', () => ({ formatTime: () => '刚刚' }))

function makeNode(id: number, over: Partial<CommentNode> = {}): CommentNode {
  return {
    id,
    post_id: 5,
    author_id: 2,
    author_name: `u${id}`,
    parent_id: null,
    floor: null,
    content: `内容${id}`,
    deleted: false,
    created_at: '2026-08-19T00:00:00Z',
    replies: [],
    ...over,
  }
}
function mountTree(comments: CommentNode[], extra: Record<string, unknown> = {}) {
  return mount(CommentTree, {
    props: { comments, postId: 5, ...extra },
    global: { stubs: { Avatar: true } },
  })
}

describe('CommentTree 深度截断（#47）', () => {
  it('默认只显示一级评论：后代不渲染、有展开按钮', () => {
    const l1 = makeNode(1, { replies: [makeNode(2), makeNode(3)] })
    const wrapper = mountTree([l1])
    expect(wrapper.text()).toContain('内容1')
    expect(wrapper.text()).not.toContain('内容2')
    expect(wrapper.text()).not.toContain('内容3')
    expect(wrapper.findAll('.expand').length).toBe(1)
  })

  it('按钮文案 N = 全部后代总数（多级、含所有深度）', () => {
    // L1 → (R1 → (R1a → R1a1), R4)：后代 = R1 + R1a + R1a1 + R4 = 4
    const l1 = makeNode(1, {
      replies: [makeNode(2, { replies: [makeNode(21, { replies: [makeNode(211)] })] }), makeNode(3)],
    })
    const wrapper = mountTree([l1])
    expect(wrapper.find('.expand').text()).toBe('展开 4 条回复')
  })

  it('N=1（单个回复）', () => {
    const l1 = makeNode(1, { replies: [makeNode(2)] })
    const wrapper = mountTree([l1])
    expect(wrapper.find('.expand').text()).toBe('展开 1 条回复')
  })

  it('无回复的一级评论不显示展开按钮', () => {
    const l1 = makeNode(1)
    const wrapper = mountTree([l1])
    expect(wrapper.find('.expand').exists()).toBe(false)
    expect(wrapper.text()).toContain('内容1')
  })

  it('点击展开显示完整子树，「收起」出现', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2, { replies: [makeNode(3)] })] })
    const wrapper = mountTree([l1])
    await wrapper.find('.expand').trigger('click')
    expect(wrapper.text()).toContain('内容2')
    expect(wrapper.text()).toContain('内容3')
    expect(wrapper.text()).toContain('收起')
  })

  it('收起折叠回「展开 N 条回复」', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2)] })
    const wrapper = mountTree([l1])
    await wrapper.find('.expand').trigger('click') // 展开
    expect(wrapper.text()).toContain('内容2')
    await wrapper.find('.expand').trigger('click') // 收起（按钮文案已是「收起」）
    expect(wrapper.text()).not.toContain('内容2')
    expect(wrapper.find('.expand').text()).toBe('展开 1 条回复')
  })

  it('深度 ≥4 压平到第 3 层：L4/L5 可见、存在 .replies.flat、所有实例 depth ≤ 3', async () => {
    const l1 = makeNode(1, {
      replies: [makeNode(2, { replies: [makeNode(3, { replies: [makeNode(4, { replies: [makeNode(5)] })] })] })],
    })
    const wrapper = mountTree([l1])
    await wrapper.find('.expand').trigger('click')
    expect(wrapper.text()).toContain('内容4')
    expect(wrapper.text()).toContain('内容5')
    expect(wrapper.findAll('.replies.flat').length).toBeGreaterThan(0)
    for (const w of wrapper.findAllComponents(CommentTree)) {
      expect(w.props('depth')).toBeLessThanOrEqual(3)
    }
  })

  it('回复按钮：一级/二级有、三级无（举报仍在）', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2, { replies: [makeNode(3)] })] })
    const wrapper = mountTree([l1])
    await wrapper.find('.expand').trigger('click')
    const l1Actions = wrapper.find('[data-comment-id="1"] .cactions')
    const l2Actions = wrapper.find('[data-comment-id="2"] .cactions')
    const l3Actions = wrapper.find('[data-comment-id="3"] .cactions')
    expect(l1Actions.text()).toContain('回复')
    expect(l2Actions.text()).toContain('回复')
    expect(l3Actions.text()).not.toContain('回复')
    expect(l3Actions.text()).toContain('举报')
  })

  it('删除占位仍渲染子评论', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2, { deleted: true, content: '', replies: [makeNode(3)] })] })
    const wrapper = mountTree([l1])
    await wrapper.find('.expand').trigger('click')
    expect(wrapper.text()).toContain('评论已删除')
    expect(wrapper.text()).toContain('内容3')
  })

  it('expandRootId 从 null→rootId 触发展开并 emit expand-root-handled', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2)] })
    const wrapper = mountTree([l1]) // expandRootId 默认 null
    expect(wrapper.text()).not.toContain('内容2')
    await wrapper.setProps({ expandRootId: 1 })
    expect(wrapper.text()).toContain('内容2')
    expect(wrapper.emitted('expand-root-handled')).toBeTruthy()
  })

  it('expand-root-handled 复位后二次 setProps 仍触发（invariant #7 闭环）', async () => {
    const l1 = makeNode(1, { replies: [makeNode(2)] })
    const wrapper = mountTree([l1])
    await wrapper.setProps({ expandRootId: 1 })
    expect(wrapper.emitted('expand-root-handled')).toHaveLength(1)
    // 父组件收到 handled 后复位 prop 为 null
    await wrapper.setProps({ expandRootId: null })
    // 二次触发：null→rootId 仍展开
    await wrapper.setProps({ expandRootId: 1 })
    expect(wrapper.emitted('expand-root-handled')).toHaveLength(2)
    expect(wrapper.text()).toContain('内容2')
    await flushPromises()
  })

  it('各一级分支独立展开互不影响', async () => {
    const l1 = makeNode(1, { replies: [makeNode(11)] })
    const l2 = makeNode(2, { replies: [makeNode(22)] })
    const wrapper = mountTree([l1, l2])
    // 展开 L1 分支
    await wrapper.findAll('.expand')[0].trigger('click')
    expect(wrapper.text()).toContain('内容11')
    expect(wrapper.text()).not.toContain('内容22')
    // 再展开 L2 分支：两者都在（互不收起）
    await wrapper.findAll('.expand')[1].trigger('click')
    expect(wrapper.text()).toContain('内容11')
    expect(wrapper.text()).toContain('内容22')
    expect(wrapper.findAll('.expand').length).toBe(2)
  })
})
