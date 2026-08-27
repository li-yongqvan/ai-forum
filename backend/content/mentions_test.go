package content

import (
	"reflect"
	"strings"
	"testing"
)

// #72 提及解析单测：正则边界、markdown 剥离、去重。与 frontend/src/utils/md.ts 的 MENTION_RE
// 保持同步（改一侧须改另一侧，两侧以同组单测钉行为，评审 F1/Q3 纪律同 tags）。

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"无提及", "普通文本", []string{}},
		{"单提及", "你好 @alice 欢迎", []string{"alice"}},
		{"行首提及", "@alice 你好", []string{"alice"}},
		{"多个去重保序", "@bob 和 @alice 以及 @bob", []string{"bob", "alice"}},
		{"中文用户名", "@小明 说得对", []string{"小明"}},
		{"下划线用户名", "@alice_wang 你好", []string{"alice_wang"}},
		{"30 字符上限", "@" + strings.Repeat("a", 30) + " 你好", []string{strings.Repeat("a", 30)}},
		{"31 字符截断前缀", "@" + strings.Repeat("a", 31) + " 你好", []string{strings.Repeat("a", 30)}},
		{"邮箱不误识别", "联系 a@b.com 联系", []string{}}, // @ 前是字母 a，非边界
		{"URL 不误识别", "看 http://example.com/x 页", []string{}},
		{"行内代码不识别", "代码 `@alice` 别处", []string{}},
		{"围栏代码不识别", "```\n@alice\n```\n正文", []string{}},
		{"链接文本不识别", "[@alice](http://x.com)", []string{}},
		{"前是字母不识别", "abc@alice 你好", []string{}}, // @ 前是字母 c，非边界
		{"前是 @ 不识别", "@@alice 你好", []string{}},  // 第二个 @ 前是 @，非边界
		{"中文前非边界", "他@alice 你好", []string{}},    // 中文在 \p{L} 排除集内，非边界（与 tags「C#和#AI」同理）
		{"标点前边界", "（@alice）你好", []string{"alice"}},
		{"空 @ 不识别", "说 @ 不识别", []string{}}, // @ 后无字符
		{"数字用户名", "@123 你好", []string{"123"}},
		{"换行后识别", "\n@alice 你好", []string{"alice"}},
		{"行尾 @", "abc @", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentions(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMentions(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
