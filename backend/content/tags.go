package content

import (
	"regexp"
	"strings"
)

// #54 标签解析：与 frontend/src/utils/md.ts 的 TAG_RE 保持同步（改一侧须改另一侧，
// 两侧以同组单测钉行为，评审 F1/Q3）。
//
// 规则（D2 微博式）：`#` + 连续中文/字母/数字/下划线，≤30 码点；`#` 前须为行首/空白/
// 标点（边界）才识别；归一化小写存储，大小写不敏感。
// 语义（评审 F1 实证）：Go `FindAllStringSubmatch` 与 JS `String.replace(全局正则)`
// 一致——都在原始串上做非重叠匹配，替换文本不参与匹配。故 `#a#b` 只识别第一个
// （第二个 `#` 前是字母 a，非边界）；`他说"#AI"` 识别 `ai`（`"`/esc 后 `;` 都是边界）。

var tagRE = regexp.MustCompile(`(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})`)

// NormalizeTag 归一化标签名：去首尾空白 + 转小写（大小写不敏感，D2）。
func NormalizeTag(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// 剥离正则镜像 md.ts 顺序（围栏代码 → 行内代码 → 外链图 → [text](url) → 裸 URL），
// 每处替换为单个空格：空格是边界，与前端占位符 \x00N\x00 边界等价（评审 F1③）。
var (
	fencedRE = regexp.MustCompile("```\\w*\\n[\\s\\S]*?```")
	codeRE   = regexp.MustCompile("`[^`\\n]+`")
	imgRE    = regexp.MustCompile(`(?i)https?:\/\/[^\s]+\.(?:png|jpe?g|gif|webp)`)
	linkRE   = regexp.MustCompile(`\[[^\]]+\]\(https?:\/\/[^)]+\)`)
	urlRE    = regexp.MustCompile(`(?i)https?:\/\/[^\s<")]+`)
)

// stripMarkdownForMatching 剥离 markdown 构造（围栏代码/行内码/图/链接/裸 URL），
// 每处替换为单个空格。标签与提及解析共用（#72）：在剥离后文本上做非重叠匹配。
func stripMarkdownForMatching(s string) string {
	s = fencedRE.ReplaceAllString(s, " ")
	s = codeRE.ReplaceAllString(s, " ")
	s = imgRE.ReplaceAllString(s, " ")
	s = linkRE.ReplaceAllString(s, " ")
	s = urlRE.ReplaceAllString(s, " ")
	return s
}

// ParseTags 从帖子正文提取标签：先剥离 markdown 构造（代码块/链接/URL 内的 `#` 不识别），
// 再在剩余文本上做非重叠匹配，归一化小写、去重、保首次出现序。
func ParseTags(content string) []string {
	s := stripMarkdownForMatching(content)

	seen := make(map[string]bool)
	tags := make([]string, 0, 4)
	for _, m := range tagRE.FindAllStringSubmatch(s, -1) {
		t := NormalizeTag(m[2])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		tags = append(tags, t)
	}
	return tags
}
