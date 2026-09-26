// #78 搜索词高亮与命中窗口：纯函数，产出分段数组而非 HTML 字符串。
// 禁 v-html（设计文档 §6-5 / X4）：Vue 对文本节点自动转义 ⇒ 零新增 XSS 面。
// 大小写口径与后端 ILIKE 对齐（§7-6：两侧 toLowerCase）；不一致时以召回为准，
// 前端找不到词也只渲染普通段，不丢卡片。

export interface Segment {
  text: string
  hit: boolean
}

/** 摘要窗口的半宽（X6：命中词前后各约 40 字）。 */
export const SNIPPET_RADIUS = 40

/**
 * 从 `q` 得出用于高亮的搜索词（展示用）。
 * 切词口径与后端 `NormalizeQuery` 一致（trim + 按空白切分），但**不做 ILIKE 转义**——
 * 转义只服务于 SQL 语义，服务端仍是唯一权威（§6-1），此处仅是其显示镜像。
 */
export function deriveTerms(q: string): string[] {
  return q.trim().split(/\s+/).filter(Boolean)
}

/** 归一化后的小写词表（丢弃空串，否则会零宽命中而死循环）。 */
function normalized(terms: string[]): string[] {
  return terms.map((t) => t.toLowerCase()).filter((t) => t.length > 0)
}

/**
 * splitByTerms：把 text 按 terms 的每处出现切成有序分段（hit 段前端包 `<mark>`）。
 * 同位置取最长词（"论坛" 与 "论" 同时在表里时不产生单字碎片）；相邻命中段合并为一个 `<mark>`。
 */
export function splitByTerms(text: string, terms: string[]): Segment[] {
  if (!text) return []
  const ws = normalized(terms)
  if (!ws.length) return [{ text, hit: false }]
  const lower = text.toLowerCase()
  const out: Segment[] = []
  let plainStart = -1
  let i = 0
  while (i < text.length) {
    let len = 0
    for (const w of ws) {
      if (w.length > len && lower.startsWith(w, i)) len = w.length
    }
    if (len === 0) {
      if (plainStart < 0) plainStart = i
      i++
      continue
    }
    if (plainStart >= 0) {
      out.push({ text: text.slice(plainStart, i), hit: false })
      plainStart = -1
    }
    const last = out[out.length - 1]
    if (last && last.hit) last.text += text.slice(i, i + len)
    else out.push({ text: text.slice(i, i + len), hit: true })
    i += len
  }
  if (plainStart >= 0) out.push({ text: text.slice(plainStart), hit: false })
  return out
}

export interface Window {
  text: string
  headCut: boolean
  tailCut: boolean
}

/**
 * hitWindow：X6 的命中上下文片段——定位 text 中**最先出现**的命中词，取前后各 radius 字。
 * 无命中（仅标题命中的帖子）⇒ headCut/tailCut 均 false、原样返回，调用方回落到既有两行截断。
 */
export function hitWindow(text: string, terms: string[], radius = SNIPPET_RADIUS): Window {
  const whole: Window = { text, headCut: false, tailCut: false }
  if (!text) return whole
  const lower = text.toLowerCase()
  let at = -1
  let len = 0
  for (const w of normalized(terms)) {
    const k = lower.indexOf(w)
    if (k < 0) continue
    if (k < at || at < 0 || (k === at && w.length > len)) {
      at = k
      len = w.length
    }
  }
  if (at < 0) return whole
  let start = Math.max(0, at - radius)
  let end = Math.min(text.length, at + len + radius)
  // 窗口边界不许落在代理对中间（emoji 被切成半个代理码位会渲染成乱码方块）
  const head = text.charCodeAt(start)
  if (start > 0 && head >= 0xdc00 && head <= 0xdfff) start++
  const tail = text.charCodeAt(end - 1)
  if (end < text.length && tail >= 0xd800 && tail <= 0xdbff) end--
  return { text: text.slice(start, end), headCut: start > 0, tailCut: end < text.length }
}
