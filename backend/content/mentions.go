package content

import (
	"regexp"
)

// #72 @用户提及解析：与 frontend/src/utils/md.ts 的 MENTION_RE 保持同步（改一侧须改另一侧，
// 两侧以同组单测钉行为）。
//
// 规则（D5）：`@` + 连续中文/字母/数字/下划线，≤30 码点；`@` 前须为行首/空白/标点/非
// 字母数字下划线@（边界）才识别。保留原文大小写（D2 字面匹配，不归一化小写）。
// 语义：#54 同款（Go FindAllStringSubmatch 与 JS String.replace 全局一致——非重叠匹配、
// 替换文本不参与）。`a@b.com` 的 `@` 前是字母（非边界）不识别；`@@alice` 第二个 `@` 前
// 是 `@`（非边界）不识别。
//
// 注意：剥离逻辑（围栏代码/行内码/图/链接/URL）与 tags.go 共用，见 stripMarkdownForMatching。

var mentionRE = regexp.MustCompile(`(^|[^\p{L}\p{N}_@])@([\p{L}\p{N}_]{1,30})`)

// ParseMentions 从帖子/评论正文提取提及用户名：先剥离 markdown 构造，再非重叠匹配，
// 去重、保首次出现序，保留原文大小写（不归一化）。
func ParseMentions(content string) []string {
	s := stripMarkdownForMatching(content)

	seen := make(map[string]bool)
	mentions := make([]string, 0, 4)
	for _, m := range mentionRE.FindAllStringSubmatch(s, -1) {
		u := m[2]
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		mentions = append(mentions, u)
	}
	return mentions
}
