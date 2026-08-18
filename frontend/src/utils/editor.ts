// 编辑器光标插入工具（#12 D4：上传返回的 URL 插入 markdown 光标处）
export interface InsertResult {
  value: string
  /** 插入后光标应停靠的位置（供调用方 setSelectionRange）。 */
  cursor: number
}

export interface InsertAtCursorOpts {
  /** 图片独占一行：前后各补一个换行（不粘连文字），默认 false。 */
  block?: boolean
}

/**
 * 在文本的 [start, end) 选区处插入 insert，返回新文本与光标位置。
 * 保持行为可测、纯函数：Write（发帖正文，block）与 PostDetail（评论，非 block）共用。
 */
export function insertAtCursor(
  text: string,
  start: number,
  end: number,
  insert: string,
  opts: InsertAtCursorOpts = {},
): InsertResult {
  const before = text.slice(0, start)
  const after = text.slice(end)
  const block = opts.block ?? false
  const sep1 = block && before.length > 0 && !before.endsWith('\n') ? '\n' : ''
  const sep2 = block && after.length > 0 && !after.startsWith('\n') ? '\n' : ''
  return {
    value: before + sep1 + insert + sep2 + after,
    cursor: start + sep1.length + insert.length + sep2.length,
  }
}
