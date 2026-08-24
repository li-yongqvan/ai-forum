package content

import (
	"reflect"
	"strings"
	"testing"
)

// 与 frontend/src/utils/md.test.ts 的 extractTags 用例保持一致（评审 F1/Q3：两侧同组用例钉行为）。

func TestParseTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"中文标签", "今天心情好 #开心", []string{"开心"}},
		{"ASCII 多个", "#AI #RAG #Agent", []string{"ai", "rag", "agent"}},
		{"大小写归一+去重保序", "#AI 然后 #ai", []string{"ai"}},
		{"下划线数字", "#deep_learning #RAG2", []string{"deep_learning", "rag2"}},
		{"C# 内联不识别", "C#和#AI", []string{}},            // 和是字母，非边界
		{"行首数字标签", "#1楼", []string{"1楼"}},             // 字面规则：行首 # 是边界
		{"hex 颜色", "color:#ff0000", []string{"ff0000"}}, // 冒号是边界
		{"引号前是边界", `他说"#AI很重要"`, []string{"ai很重要"}}, // 评审 F2 实证
		{"双井号不识别", "##ai", []string{}},
		{"相邻标签只识别首个", "#a#b", []string{"a"}}, // 评审 F1 甲案：第二个 # 前是字母
		{"行内代码内不识别", "`#code`", []string{}},
		{"围栏代码内不识别", "```go\n#foo\n```", []string{}},
		{"链接URL内不识别", "[链接](https://x.com/a#frag)", []string{}},
		{"裸URL内不识别", "看 https://x.com/a#ai", []string{}},
		{"超长截断30", "#" + strings.Repeat("a", 35), []string{strings.Repeat("a", 30)}},
		{"井号后空格不识别", "# tag", []string{}},
		{"emoji 不识别", "#😀", []string{}},
		{"换行后识别", "\n#AI", []string{"ai"}},
		{"行尾井号", "abc #", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTags(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTags(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{" AI ", "ai"},
		{"", ""},
		{"AI", "ai"},
		{"#AI", "#ai"},
	}
	for _, tt := range tests {
		if got := NormalizeTag(tt.in); got != tt.want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
