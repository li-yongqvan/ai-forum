import { describe, it, expect } from 'vitest'
import { md, plainText, isSafeUrl } from './md'

describe('md 渲染（IA §5.2 子集）', () => {
  it('围栏代码块', () => {
    const out = md('```python\nprint(1)\n```')
    expect(out).toContain('<pre><code>')
    expect(out).toContain('print(1)')
  })
  it('行内代码', () => {
    expect(md('用 `npm i` 安装')).toContain('<code>npm i</code>')
  })
  it('粗体', () => {
    expect(md('**重要**')).toContain('<strong>重要</strong>')
  })
  it('段落与换行', () => {
    const out = md('第一段\n第二行\n\n第二段')
    expect(out).toContain('<p>第一段<br>第二行</p>')
    expect(out).toContain('<p>第二段</p>')
  })
})

describe('md 安全（D4/v2）', () => {
  it('先转义 HTML（XSS）', () => {
    const out = md('<script>alert(1)</script>')
    expect(out).not.toContain('<script>')
    expect(out).toContain('&lt;script&gt;')
  })
  it('行内代码内也转义', () => {
    expect(md('`<b>x</b>`')).not.toContain('<b>x</b>')
  })
  it('javascript: 伪协议链接不生成 <a>', () => {
    const out = md('[点我](javascript:alert(1))')
    expect(out).not.toContain('<a')
  })
  it('合法链接带 rel=noopener noreferrer', () => {
    const out = md('[官网](https://example.com)')
    expect(out).toContain(
      '<a href="https://example.com" target="_blank" rel="noopener noreferrer">官网</a>',
    )
  })
})

describe('isSafeUrl 白名单', () => {
  it('http/https/mailto 允许', () => {
    expect(isSafeUrl('https://x.com')).toBe(true)
    expect(isSafeUrl('http://x.com')).toBe(true)
    expect(isSafeUrl('mailto:a@b.c')).toBe(true)
  })
  it('伪协议拒绝', () => {
    expect(isSafeUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeUrl('data:text/html,hi')).toBe(false)
    expect(isSafeUrl('vbscript:x')).toBe(false)
  })
})

describe('plainText 摘要', () => {
  it('去掉 markdown 语法', () => {
    const out = plainText('**粗体**、`code`、[链接](https://x)、多行\n文字')
    expect(out).not.toContain('**')
    expect(out).not.toContain('`')
    expect(out).toContain('粗体')
    expect(out).toContain('文字')
  })
  it('代码块折叠', () => {
    expect(plainText('前\n```\ncode\n```\n后')).toContain('[代码]')
  })
})
