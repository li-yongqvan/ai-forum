import { describe, it, expect } from 'vitest'
import { insertAtCursor } from './editor'

describe('insertAtCursor（#12 D4：上传 URL 插入光标处）', () => {
  const url = 'http://localhost:5173/uploads/abc.png'

  it('光标在中间：纯插入，不补换行（默认 block=false）', () => {
    const r = insertAtCursor('abcd', 2, 2, 'X')
    expect(r.value).toBe('abXcd')
    expect(r.cursor).toBe(3)
  })

  it('有选区时替换选区', () => {
    const r = insertAtCursor('hello world', 6, 11, 'X')
    expect(r.value).toBe('hello X')
    expect(r.cursor).toBe(7)
  })

  it('block 模式：图片独占一行（文字前补换行）', () => {
    const r = insertAtCursor('hello', 5, 5, url, { block: true })
    expect(r.value).toBe('hello\n' + url)
    expect(r.cursor).toBe('hello\n'.length + url.length)
  })

  it('block 模式：光标在行首时仅补后换行', () => {
    const r = insertAtCursor('hello', 0, 0, url, { block: true })
    expect(r.value).toBe(url + '\nhello')
    expect(r.cursor).toBe(url.length + 1)
  })

  it('block 模式：空文本不补换行', () => {
    const r = insertAtCursor('', 0, 0, url, { block: true })
    expect(r.value).toBe(url)
    expect(r.cursor).toBe(url.length)
  })

  it('block 模式：前后都已有换行则不再补', () => {
    const r = insertAtCursor('a\n\nb', 2, 2, url, { block: true })
    expect(r.value).toBe('a\n' + url + '\nb')
  })

  it('block 模式：光标位于两行之间', () => {
    const r = insertAtCursor('a\nb', 2, 2, url, { block: true })
    expect(r.value).toBe('a\n' + url + '\nb')
  })
})
