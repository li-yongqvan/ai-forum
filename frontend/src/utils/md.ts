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
    pushBlock(`<img src="${m}" alt="图片" loading="lazy">`),
  )
  // [text](url)
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g, (_m, text: string, url: string) =>
    pushBlock(safeLink(url, text) ?? `[${text}](${url})`),
  )
  // 剩余裸链接（白名单兜底；排除引号/括号防误匹配已生成 HTML）
  s = s.replace(/(https?:\/\/[^\s<")]+)/gi, (m: string) => pushBlock(safeLink(m) ?? m))
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
