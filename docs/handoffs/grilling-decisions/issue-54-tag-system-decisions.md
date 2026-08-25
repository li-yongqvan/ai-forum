# #54 标签系统（#hashtag）——决策记录

> 本文件 = **#54 grilling 决策记录**（SOP §3.4：决策落档，随 PR 入库）。实现计划见 `docs/plans/54-tag-system.md`；可审计设计文档见 `docs/plans/54-tag-system-设计文档.md`（评审后有条件通过，为实现基线）。
> 溯源约定：**事实**标来源（代码 `file:line` / GitHub issue / 「用户确认(日期)」）；**判断性裁决**标【决策】+理由+备选。
> 数据时点：2026-08-24（决策确认日；服务器 `main`=`0301601`，api/postgres healthy，healthz ok）。

## Grilling 决策（D1–D7，2026-08-24 用户确认）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 落库 vs 纯展示 | **落库**（新建 `content.tags` + `content.post_tags`） | 聚合页需真实数据/计数；用户确认（2026-08-24） |
| D2 | 标签规则 | **微博式**：`#`+连续中文/字母/数字/下划线 ≤30 字；大小写不敏感（归一化小写存储）；`#` 前须为行首/空白/标点才识别 | 用户确认（2026-08-24）。实测边界行为见「规格澄清」 |
| D3 | 解析时机 | **发帖时后端解析落库 + 渲染时 md.ts 高亮**（前后端同一规则） | 落库 D1 的必然；用户确认 |
| D4 | 列表卡片 | **PostCard 显示标签胶囊**（后端 `PostView.tags` 批量带出） | 贴合 ticket 目标 3「详情/列表中可点」；用户确认 |
| D5 | 聚合排序 | **置顶优先 + 发布时间倒序**（与现有列表一致） | 行为可预期，复用 `ListPosts` 排序；用户确认 |
| D6 | 存量回填 | **不回填**（标签只对之后新帖生效） | 帖子不可编辑（`Repo` 无 `UpdatePost`，已核实）；老帖 `#词` 照常渲染可点；用户确认 |
| D7 | 解析范围 | 仅正文 content，不含 title | ticket「发帖正文里 #关键词」；用户确认 |

**备选与为何不选**：
- **D2 宽松**（`#` 后到空白任意字符含标点）：匹配过宽，`#C++`、`#vue-3` 等容易误判，聚合页噪音大。
- **D2 严格**（仅中文/字母/数字，≤20，纯数字不算）：过保守，`#1楼`、数字年份类标签丢失。
- **D2 标签须以字母/下划线开头**（剔除 `#1楼`）：仍挡不住 `color:#ff0000`，且偏离用户已确认的字面规则，评审 Q1 不推荐。
- **D3 仅前端渲染高亮（不落库）**：无真实聚合数据，违背 ticket 目标 2。
- **D6 回填存量**：需一次性数据脚本，帖子不可编辑、数据量小、老帖 `#词` 可点，MVP 不值得；若未来启用编辑需一并设计回填。

**规格澄清（字面规则边界行为，实测确认）**：边界规则**只能**挡 `#` 前是字母的内联误伤（`C#`、`我在#1楼`、`C#和#AI`）；`color:#ff0000`（冒号=边界）、行首 `#1楼`、`他说"#AI很重要"`（引号=边界）**仍会识别**为标签。此行为符合真实 hashtag 习惯（微博同理），用户已确认采用字面规则，以测试钉住。若未来希望「引号内标签不识别」，需把 `"` 排除出边界类（超出字面规则范畴，须回用户确认）。

## 范围（改动文件红线）

- 后端：`backend/migrations/0007_tags_schema.sql`（新增）、`backend/content/tags.go`（新增）、`backend/content/{model,repo,gorm_repo,service}.go`、`backend/internal/httpapi/handler/content.go`（仅 ListPosts 加 `?tag=`）。
- 前端：`frontend/src/utils/md.ts`、`frontend/src/styles/global.css`、`frontend/src/api/{content,types}.ts`、`frontend/src/router/index.ts`、`frontend/src/views/TagDetail.vue`（新增）、`frontend/src/components/PostCard.vue`。
- 测试：后端 `tags_test.go`（新增）+ `service_test.go`/`fake_repo_test.go`/`content_test.go`；前端 `md.test.ts` + `TagDetail.test.ts`（新增，可选 PostCard.test.ts）。
- **不碰**：`notify/moderation`、`.github/workflows/`、`Write.vue`（无 payload 变更）、`CommentTree.vue`（评论标签经 md.ts 免费高亮，但**只亮不聚**——评论不落库，范围登记）。
- **不做**：标签选择器、帖子编辑、标签计数/热词榜、大小写敏感聚合、存量回填、评论标签聚合。

## 评审闭环

- 2026-08-24 独立复核：**有条件通过**，评审意见书 `docs/plans/54-tag-system-设计文档-评审意见书.md`。处置：**F1**（`#a#b` 机制方向修正——Go 改单次 `FindAllStringSubmatch`）、**F2**（两处测试期望改实证值、esc 边界论述改写）、**F3**（`/tag/%20` 空名守卫 `.trim()`）、**F4**（本决策记录随 PR 入库）、**F5**（invariant 13 措辞）、**F6**（评论只亮不聚登记）、**F7**（读侧 best-effort）、**F8**（死锁残余文档化 + 标签名排序缓解）、**F9**（§5.1-4 依据改写）**全采纳**；Q1–Q8 全部认可（Q3 按 F1 甲案改道）。
