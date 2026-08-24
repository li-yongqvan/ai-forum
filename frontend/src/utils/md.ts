// Markdown 子集渲染器（IA §5.2：围栏代码/行内代码/粗体/链接/外链图）
// 安全（D4/v2）：先 HTML 转义防 XSS；链接协议白名单 http/https/mailto（伪协议被拦）；外链 rel="noopener noreferrer"
// 实现：先抽离代码/图片/链接为占位符，最后还原，避免生成物被后续规则二次处理（防 <a href="<a href=…>）。

const esc = (s: string): string =>
  String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')

/** 链接协议白名单（v2 §3.2 D4）。 */
export function isSafeUrl(url: string): boolean {
  return /^(https?:|mailto:)/i.test(url)
}

/** 生成安全外链；不满足白名单返回 null（保持为转义后的纯文本）。 */
function safeLink(url: string, text?: string): string | null {
  if (!isSafeUrl(url)) return null
  const t = text ?? url
  return `<a href="${url}" target="_blank" rel="noopener noreferrer">${t}</a>`
}

// #54 标签规则（与 backend/content/tags.go 的 tagRE 保持同步，改一侧须改另一侧——评审 F1/Q3）。
// 字面规则（D2）：`#` + 连续中文/字母/数字/下划线 ≤30；`#` 前须为行首/空白/标点才识别。
// JS `String.replace` 与 Go `FindAllStringSubmatch` 语义一致（原始串非重叠匹配、替换文本不参与）
// → `#a#b` 只识别第一个；`"`/`;`/`:` 都是边界。
const TAG_RE = /(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})/gu

/** 标签名归一化（与后端 NormalizeTag 一致）：小写。 */
function normalizeTag(s: string): string {
  return s.toLowerCase()
}

export function md(src: string): string {
  let s = esc(src)
  const blocks: string[] = []
  const pushBlock = (html: string) => {
    blocks.push(html)
    return `\x00${blocks.length - 1}\x00`
  }

  // 围栏代码块：整体抽离
  s = s.replace(/```(\w*)\n([\s\S]*?)```/g, (_m, _lang: string, code: string) =>
    pushBlock(`<pre><code>${code.replace(/\n$/, '')}</code></pre>`),
  )
  // 行内代码：抽离（避免其内容被后续链接/粗体规则误伤）
  s = s.replace(/`([^`\n]+)`/g, (_m, code: string) => pushBlock(`<code>${code}</code>`))
  // 外链图片（先于裸链接，仅 http(s)）
  s = s.replace(/(https?:\/\/[^\s]+\.(?:png|jpe?g|gif|webp))/gi, (m: string) =>
    pushBlock(`<img src="${m}" alt="图片" loading="lazy" class="content-img">`),
  )
  // [text](url)
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g, (_m, text: string, url: string) =>
    pushBlock(safeLink(url, text) ?? `[${text}](${url})`),
  )
  // 剩余裸链接（白名单兜底；排除引号/括号防误匹配已生成 HTML）
  s = s.replace(/(https?:\/\/[^\s<")]+)/gi, (m: string) => pushBlock(safeLink(m) ?? m))
  // 标签（#54）：行首/空白/标点后 # + 连续中文/字母/数字/下划线 ≤30，可点进聚合页
  // （先于粗体：`**#ai**` → 占位符机制天然落到 `<strong><a>#ai</a></strong>`）
  s = s.replace(TAG_RE, (_m: string, lead: string, tag: string) =>
    lead + pushBlock(`<a class="tag" href="#/tag/${encodeURIComponent(normalizeTag(tag))}">#${tag}</a>`),
  )
  // 粗体
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  // 段落
  s = s
    .split(/\n{2,}/)
    .map((p) => (p.trim() ? `<p>${p.replace(/\n/g, '<br>')}</p>` : ''))
    .join('')
  // 还原占位
  s = s.replace(/\x00(\d+)\x00/g, (_m, i: string) => blocks[+i])
  return `<div class="md">${s}</div>`
}

/** #54 提取正文标签（归一化小写、去重、保首次出现序）；与后端 ParseTags 同规则、同组用例钉行为。 */
export function extractTags(src: string): string[] {
  // 与 md() 相同的剥离顺序（围栏→行内码→图→链接→裸URL），块替换为空格（空格与占位符 \x00N\x00 边界等价）
  let s = esc(src)
  s = s.replace(/```(\w*)\n([\s\S]*?)```/g, ' ')
  s = s.replace(/`([^`\n]+)`/g, ' ')
  s = s.replace(/(https?:\/\/[^\s]+\.(?:png|jpe?g|gif|webp))/gi, ' ')
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g, ' ')
  s = s.replace(/(https?:\/\/[^\s<")]+)/gi, ' ')
  const seen = new Set<string>()
  const tags: string[] = []
  s.replace(TAG_RE, (_m: string, _lead: string, tag: string) => {
    const t = normalizeTag(tag)
    if (!seen.has(t)) {
      seen.add(t)
      tags.push(t)
    }
    return ' '
  })
  return tags
}

/** 纯文本摘要（帖子卡/预览用）：去掉 markdown 语法。 */
export function plainText(src: string): string {
  return String(src)
    .replace(/```[\s\S]*?```/g, '[代码]')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/https?:\/\/\S+/g, '')
    .replace(/\n+/g, ' ')
    .trim()
}
