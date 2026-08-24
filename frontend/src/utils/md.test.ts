import { describe, it, expect } from 'vitest'
import { md, plainText, isSafeUrl, extractTags } from './md'

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

describe('md 图片渲染（#50：content-img class 挂钩预览）', () => {
  it('外链图渲染带 content-img class，两图各一次', () => {
    const out = md('https://a.com/1.png\n\nhttps://a.com/2.png')
    expect(out).toContain('class="content-img"')
    expect((out.match(/<img/g) ?? []).length).toBe(2)
  })
})

describe('extractTags（#54，与 backend/content/tags_test.go 同组用例钉行为——评审 F1/Q3）', () => {
  it('中文/ASCII 提取 + 归一化小写', () => {
    expect(extractTags('今天心情好 #开心')).toEqual(['开心'])
    expect(extractTags('今天 #AI 和 #RAG')).toEqual(['ai', 'rag'])
  })
  it('大小写归一 + 去重保序', () => {
    expect(extractTags('#AI 然后 #ai')).toEqual(['ai'])
  })
  it('C# 内联不识别（和是字母，非边界）', () => {
    expect(extractTags('C#和#ai')).toEqual([])
  })
  it('引号前是边界（评审 F2 实证）', () => {
    expect(extractTags('他说"#AI"')).toEqual(['ai'])
  })
  it('相邻标签只识别首个（评审 F1 甲案）', () => {
    expect(extractTags('#a#b')).toEqual(['a'])
  })
  it('行内代码内不识别', () => {
    expect(extractTags('`#code`')).toEqual([])
  })
  it('链接/URL 内不识别', () => {
    expect(extractTags('[链接](https://x.com/a#frag)')).toEqual([])
    expect(extractTags('看 https://x.com/a#ai')).toEqual([])
  })
})

describe('md 标签渲染（#54）', () => {
  it('#AI 渲染为可点锚点（href 归一化小写，文本保留原文）', () => {
    expect(md('#AI')).toContain('<a class="tag" href="#/tag/ai">#AI</a>')
  })
  it('中文标签 href URL 编码', () => {
    expect(md('#开心')).toContain('href="#/tag/%E5%BC%80%E5%BF%83"')
  })
  it('C# 不生成标签锚点', () => {
    expect(md('C#')).not.toContain('<a class="tag"')
  })
  it('#a#b 只生成单锚点（评审 F1）', () => {
    const out = md('#a#b')
    expect((out.match(/<a class="tag"/g) ?? []).length).toBe(1)
  })
  it('XSS：#<script> 转义后不生成锚点', () => {
    const out = md('#<script>')
    expect(out).not.toContain('<a class="tag"')
    expect(out).not.toContain('<script>')
  })
  it('引号前 # 是标签（渲染路径，评审 F2）', () => {
    expect(md('他说"#AI"')).toContain('<a class="tag" href="#/tag/ai">#AI</a>')
  })
})
