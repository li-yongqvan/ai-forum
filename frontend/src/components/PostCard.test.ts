// #78 PostCard 的 terms 分支（设计文档 §5.2-7：补两条 prop 分支用例）。
// 重点钉 §7-5 的可证伪形式：未传 terms 时 .ptitle/.pabstract 内**只有一个文本节点**
// （零 <mark>、零注释占位），即渲染结果与引入高亮之前一致。
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { Post } from '../api/types'
import PostCard from './PostCard.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

function post(over: Partial<Post> = {}): Post {
  return {
    id: 1,
    board_id: 1,
    board_name: '综合讨论',
    topic_id: null,
    topic_name: null,
    author_id: 7,
    author_name: 'alice',
    author_avatar: null,
    title: '论坛 第 1 期公告',
    content: '正文开头。这里提到论坛一次。',
    is_pinned: false,
    is_featured: false,
    view_count: 0,
    like_count: 0,
    comment_count: 0,
    favorite_count: 0,
    created_at: '2026-09-26T00:00:00Z',
    ...over,
  }
}

const mountCard = (props: { post: Post; terms?: string[] }) =>
  shallowMount(PostCard, { props })

describe('PostCard #78 terms 分支', () => {
  it('未传 terms ⇒ 标题/摘要的 innerHTML 与单个文本节点逐字节一致（§7-5）', () => {
    const w = mountCard({ post: post() })
    const t = w.find('.ptitle')
    const a = w.find('.pabstract')
    expect(t.findAll('mark')).toHaveLength(0)
    expect(a.findAll('mark')).toHaveLength(0)
    // innerHTML 逐字节：无元素、无注释占位、无多余空格
    expect(t.element.innerHTML).toBe('论坛 第 1 期公告')
    expect(a.element.innerHTML).toBe('正文开头。这里提到论坛一次。')
    expect([...t.element.childNodes].every((n) => n.nodeType === Node.TEXT_NODE)).toBe(true)
  })

  it('terms 为空数组 ⇒ 同上（结果页无词时不改变渲染）', () => {
    const w = mountCard({ post: post(), terms: [] })
    expect(w.find('.ptitle').element.innerHTML).toBe('论坛 第 1 期公告')
    expect(w.find('.pabstract').element.innerHTML).toBe('正文开头。这里提到论坛一次。')
    expect(w.findAll('mark')).toHaveLength(0)
  })

  it('传 terms ⇒ 标题命中段包 <mark>，其余保持纯文本', () => {
    const w = mountCard({ post: post(), terms: ['论坛'] })
    const marks = w.find('.ptitle').findAll('mark')
    expect(marks).toHaveLength(1)
    expect(marks[0].text()).toBe('论坛')
    expect(w.find('.ptitle').text()).toBe('论坛 第 1 期公告')
  })

  it('多词各自高亮，大小写与后端 ILIKE 同口径（§7-6）', () => {
    const w = mountCard({
      post: post({ title: 'AI 论坛与 ai 实践', content: '无目标词' }),
      terms: ['ai', '论坛'],
    })
    expect(w.find('.ptitle').findAll('mark').map((m) => m.text())).toEqual(['AI', '论坛', 'ai'])
    // 正文不含词 ⇒ 摘要零高亮，但卡片绝不因"前端找不到词"而被丢弃（§7-6）
    expect(w.find('.pabstract').findAll('mark')).toHaveLength(0)
    expect(w.find('.pabstract').text()).toBe('无目标词')
  })

  it('正文命中在后段 ⇒ 摘要切命中窗口并在两端加省略号（X6）', () => {
    const long = 'x'.repeat(200) + '论坛' + 'y'.repeat(200)
    const w = mountCard({ post: post({ content: long }), terms: ['论坛'] })
    const a = w.find('.pabstract')
    expect(a.text().startsWith('…')).toBe(true)
    expect(a.text().endsWith('…')).toBe(true)
    expect(a.findAll('mark')[0].text()).toBe('论坛')
    expect(a.text()).toContain('论坛')
    expect(a.text().length).toBeLessThan(long.length)
  })

  it('仅标题命中（正文无词）⇒ 摘要回落到既有整段文本，不加省略号（X6）', () => {
    const w = mountCard({ post: post({ content: '正文里没有目标词' }), terms: ['论坛'] })
    expect(w.find('.pabstract').text()).toBe('正文里没有目标词')
    expect(w.find('.pabstract').text()).not.toContain('…')
  })

  it('markdown 语法先剥离再高亮（摘要走 plainText，命中词在加粗里也能亮）', () => {
    const w = mountCard({ post: post({ content: '**论坛**公告' }), terms: ['论坛'] })
    expect(w.find('.pabstract').text()).toBe('论坛公告')
    expect(w.find('.pabstract').findAll('mark')[0].text()).toBe('论坛')
  })

  it('HTML 敏感字符走文本节点，不产生元素（禁 v-html 的可证伪面，X4）', () => {
    const w = mountCard({
      post: post({ title: '<img src=x onerror=alert(1)>论坛' }),
      terms: ['论坛'],
    })
    expect(w.find('.ptitle').findAll('img')).toHaveLength(0)
    expect(w.find('.ptitle').text()).toBe('<img src=x onerror=alert(1)>论坛')
    expect(w.find('.ptitle').findAll('mark').map((m) => m.text())).toEqual(['论坛'])
  })
})
