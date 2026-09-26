// #78 搜索词高亮纯函数：设计文档 §5.2-7 要求的"全分支"覆盖。
// 这里只测纯函数（SOP §7：测试跨 seam 行为），组件侧的 mark 渲染在 PostCard.test.ts 钉。
import { describe, expect, it } from 'vitest'
import { deriveTerms, hitWindow, splitByTerms } from './highlight'

/** 不变量：分段拼回去必须等于原文（任何分支都不许吞字或加字）。 */
const join = (text: string, terms: string[]) =>
  splitByTerms(text, terms)
    .map((s) => s.text)
    .join('')

describe('deriveTerms（搜索词表，展示用）', () => {
  it('空串与纯空白 ⇒ 空表', () => {
    expect(deriveTerms('')).toEqual([])
    expect(deriveTerms('   \t \n')).toEqual([])
  })

  it('按空白切词并去掉首尾空白', () => {
    expect(deriveTerms('  论坛  搜索 ')).toEqual(['论坛', '搜索'])
    expect(deriveTerms('a\tb\nc')).toEqual(['a', 'b', 'c'])
  })

  it('ILIKE 元字符保持字面（高亮面向原文，不带后端转义）', () => {
    expect(deriveTerms('100% a_b c\\d')).toEqual(['100%', 'a_b', 'c\\d'])
  })
})

describe('splitByTerms（分段，不产出 HTML）', () => {
  it('空文本 ⇒ 零段', () => {
    expect(splitByTerms('', ['论'])).toEqual([])
    expect(splitByTerms('', [])).toEqual([])
  })

  it('无搜索词 ⇒ 单一非命中段，等价于原文（§7-5 非搜索态渲染基线）', () => {
    expect(splitByTerms('论坛公告', [])).toEqual([{ text: '论坛公告', hit: false }])
  })

  it('搜索词表全为空串 ⇒ 同上（防零宽命中死循环）', () => {
    expect(splitByTerms('论坛公告', ['', ''])).toEqual([{ text: '论坛公告', hit: false }])
  })

  it('无命中 ⇒ 单一非命中段', () => {
    expect(splitByTerms('完全无关的标题', ['论坛'])).toEqual([
      { text: '完全无关的标题', hit: false },
    ])
  })

  it('词在开头 / 中间 / 结尾', () => {
    expect(splitByTerms('论坛周报', ['论坛'])).toEqual([
      { text: '论坛', hit: true },
      { text: '周报', hit: false },
    ])
    expect(splitByTerms('本期论坛报', ['论坛'])).toEqual([
      { text: '本期', hit: false },
      { text: '论坛', hit: true },
      { text: '报', hit: false },
    ])
    expect(splitByTerms('本周论坛', ['论坛'])).toEqual([
      { text: '本周', hit: false },
      { text: '论坛', hit: true },
    ])
  })

  it('同词多次出现 ⇒ 每处都切段', () => {
    expect(splitByTerms('论坛与论坛', ['论坛'])).toEqual([
      { text: '论坛', hit: true },
      { text: '与', hit: false },
      { text: '论坛', hit: true },
    ])
  })

  it('多词各自命中（AND 是召回语义，高亮每词独立）', () => {
    expect(splitByTerms('论坛公告：搜索上线', ['论坛', '搜索'])).toEqual([
      { text: '论坛', hit: true },
      { text: '公告：', hit: false },
      { text: '搜索', hit: true },
      { text: '上线', hit: false },
    ])
  })

  it('ASCII 大小写不敏感，命中段保留原文大小写（与后端 ILIKE 同口径，X2）', () => {
    expect(splitByTerms('AI Agent 实践', ['ai'])).toEqual([
      { text: 'AI', hit: true },
      { text: ' Agent 实践', hit: false },
    ])
    expect(splitByTerms('ai agent', ['AI'])).toEqual([
      { text: 'ai', hit: true },
      { text: ' agent', hit: false },
    ])
  })

  it('同位置多词 ⇒ 取最长词，不产生单字碎片', () => {
    expect(splitByTerms('论坛专题', ['论', '论坛'])).toEqual([
      { text: '论坛', hit: true },
      { text: '专题', hit: false },
    ])
  })

  it('相邻命中合并为一个 <mark> 段', () => {
    expect(splitByTerms('ab', ['a', 'b'])).toEqual([{ text: 'ab', hit: true }])
  })

  it('元字符按字面匹配（非正则：. * ? ( ) [ ] 不当量词）', () => {
    expect(splitByTerms('a.c', ['a.c'])).toEqual([{ text: 'a.c', hit: true }])
    expect(splitByTerms('a.c', ['a'])).toEqual([
      { text: 'a', hit: true },
      { text: '.c', hit: false },
    ])
    expect(splitByTerms('100%', ['100%'])).toEqual([{ text: '100%', hit: true }])
    expect(splitByTerms('.*(x)[+]', ['.*'])).toEqual([
      { text: '.*', hit: true },
      { text: '(x)[+]', hit: false },
    ])
  })

  it('全拼回不变（穷举分支的公共不变量）', () => {
    const samples = ['', '论坛', 'AI 智联论坛 全文搜索', '100% a_b \\ %', 'abababa', '中文']
    for (const s of samples) {
      for (const t of [[], ['论'], ['a', 'b'], ['100%'], ['中文', 'AI']]) {
        expect(join(s, t)).toBe(s)
      }
    }
  })
})

describe('hitWindow（X6 命中上下文片段）', () => {
  it('空文本 / 无搜索词 ⇒ 原样、两端未裁', () => {
    expect(hitWindow('', ['论'])).toEqual({ text: '', headCut: false, tailCut: false })
    expect(hitWindow('正文', []).text).toBe('正文')
  })

  it('正文无命中（仅标题命中）⇒ 整段回落，交调用方走既有两行截断', () => {
    expect(hitWindow('正文里没东西', ['论坛'])).toEqual({
      text: '正文里没东西',
      headCut: false,
      tailCut: false,
    })
  })

  it('命中在开头 ⇒ 不裁头，只裁尾', () => {
    const w = hitWindow('论坛' + 'x'.repeat(100), ['论坛'])
    expect(w.headCut).toBe(false)
    expect(w.tailCut).toBe(true)
    expect(w.text.startsWith('论坛')).toBe(true)
    expect(w.text.length).toBe(2 + 40)
  })

  it('命中在中间 ⇒ 前后各 40 字，两端加裁切标记', () => {
    const text = 'a'.repeat(100) + '论坛' + 'b'.repeat(100)
    const w = hitWindow(text, ['论坛'])
    expect(w).toEqual({ text: 'a'.repeat(40) + '论坛' + 'b'.repeat(40), headCut: true, tailCut: true })
  })

  it('命中在结尾 ⇒ 裁头不裁尾', () => {
    const w = hitWindow('y'.repeat(100) + '论坛', ['论坛'])
    expect(w.headCut).toBe(true)
    expect(w.tailCut).toBe(false)
    expect(w.text.endsWith('论坛')).toBe(true)
  })

  it('整段都在窗口内 ⇒ 不裁', () => {
    expect(hitWindow('短正文 论坛', ['论坛'])).toEqual({
      text: '短正文 论坛',
      headCut: false,
      tailCut: false,
    })
  })

  it('取最先出现的命中词，而非最长的词', () => {
    const text = '前段 搜索 出现 两次 论坛'
    expect(hitWindow(text, ['论坛', '搜索'], 2).text).toBe('段 搜索 出')
  })

  it('大小写不敏感定位，窗口保留原文大小写', () => {
    const w = hitWindow('xxxxAIxxxx', ['ai'], 2)
    expect(w.text).toBe('xxAIxx')
  })

  it('窗口边界不切代理对（emoji 不落半个码位，radius=1 会正好压在 😃 中间）', () => {
    const text = '😀🙁😃论坛' + 'z'.repeat(80)
    const w = hitWindow(text, ['论坛'], 1)
    expect(w.text).toBe('论坛z')
    expect(w.headCut).toBe(true)
    expect(w.tailCut).toBe(true)
  })
})
