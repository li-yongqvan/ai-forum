# #50 图片点击放大查看（手机）——设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / 实查命令输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-23（真值核对执行日；服务器实查 2026-08-23 09:xx，代码/Vant/样式实查同日）。
> 评审状态：**有条件通过**（2026-08-23 独立复核；评审意见书 `docs/plans/50-image-preview-设计文档-评审意见书.md`；F1–F7 处置见 §10）。
> 源文档：`docs/plans/50-image-preview.md`（本设计文档的前身，供对照）；决策记录：`docs/handoffs/grilling-decisions/issue-50-image-preview-decisions.md`。

## §0 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP，已上线 `http://122.51.233.225:8888/`（PWA，手机优先）。本票是纯前端功能补齐。

**技术栈与目录**（可自行查阅）：
- 前端 `frontend/`：Vue 3 + TypeScript + Vite + Vant（^4.10，`frontend/package.json:19`）+ Pinia + PWA。
- 后端 `backend/`：Go（Gin + GORM + PostgreSQL）。**本票零后端改动**。
- 关键前端文件：
  - `frontend/src/utils/md.ts` —— Markdown 子集渲染器（IA §5.2：围栏代码/行内代码/粗体/链接/**外链图**），输出 `<div class="md">…</div>`，先 HTML 转义防 XSS。
  - `frontend/src/views/PostDetail.vue` —— 帖子详情页（正文 + 评论树 + 赞藏/分享/举报 + 底部评论栏）。
  - `frontend/src/components/CommentTree.vue` —— **自递归**评论树组件（#47 深度截断：默认只显一级、最多 3 层）。
  - `frontend/src/api/*` —— API 层；`frontend/src/utils/*` —— 纯函数工具（`editor.ts`/`format.ts`/`md.ts`）。

**本票必须理解的前端机制**：
1. **md 渲染注入点（两处）**：帖子正文 `PostDetail.vue:354`、评论正文 `CommentTree.vue:80`，都是 `v-html="md(…)"`——markdown 渲染出的 HTML 直接插入 DOM。`v-html` 内容里的元素无法在模板里直接绑 `@click`，需**事件委托**（在容器上监听，冒泡里 `closest` 定位）。
2. **Vant 函数式组件**：`showImagePreview`/`showToast` 是函数式调用（`import { showImagePreview } from 'vant'`），非模板组件。**其样式不随 unplugin-vue-components 按需引入自动加载**（§2.5 实查）——这是本票一个关键技术事实。
3. **#47 评论树**：`CommentTree.vue` 递归渲染 `.comment`→`.cbody`（评论内容）→`.replies`（子树）。本票只给 `.cbody` 加事件绑定，**不得改动** #47 的深度截断/展开逻辑。

**角色与权限**：本票与权限无关（图片预览对游客/登录用户一致开放）。

**与本票相关的前序工作**：
- **#12 图片上传**（已上线）：后端 `POST /api/v1/uploads`，前端发帖/评论工具栏「图片」按钮上传后**把裸 URL 插入 markdown 光标处**（`Write.vue:96-100` + `frontend/src/utils/editor.ts`）。**#12 已定案不做缩略图**（地图 issue #1 决策 9：「不做缩略图/删除/配额（#12 D5）」）→ 预览显示的就是原图。
- **#47 评论深度截断**（已上线，PR #48 合并）：评论树深度截断逻辑，本票注入点所在组件。

**术语速查**：
- **注入点**：`v-html="md(…)"` 的容器元素（正文 div / 评论 `.cbody`）。
- **`.content-img`**：本票给渲染出的 `<img>` 加的 class，是事件委托的选择器。
- **事件委托**：在容器上绑一个 `click` 监听，通过 `e.target.closest('.content-img')` 判断是否命中图片。
- **`showImagePreview`**：Vant 4 全屏图片预览函数（多图可滑动、点图关闭、双指缩放）。

## §1 背景与目标（为什么做）

- **需求来源**：GitHub **issue #50**「图片点击无法放大查看（手机）——补图片预览（Vant ImagePreview）」（`gh issue view 50`，state=OPEN，2026-08-23 实查）。ticket 正文标注「地图 #1（child）：线上缺陷」。
- **交接依据**：`docs/handoffs/50-image-preview.md`（自包含交接，§3 列 4 项 open decisions，本会话已逐一 grilling 确认，见 §3）。
- **问题**：帖子/评论里的图片（md 渲染）在手机上点击无法放大——电脑可右键新开，手机点击无反应。属**功能缺失补齐**（非回归 bug）。
- **目标**：点击帖子正文 / 评论里的图片 → 弹出**全屏放大预览**（Vant `showImagePreview`），**同一段内容**（帖子正文 / 单条评论）多张图可**左右滑动切换**。
- **范围一句话**：纯前端；后端零改动；#47 评论树逻辑不碰。

## §2 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-23 实查）

命令：`ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps --format "{{.Name}} {{.Status}}" && curl -s http://localhost:8888/healthz'`

结果摘录：
```
0c10359 Merge pull request #48 from li-yongqvan/feat/comment-depth-fold
ai-forum-api Up 36 hours (healthy)
ai-forum-postgres Up 4 days (healthy)
ai-forum-web Up 2 days
{"status":"ok"}
```
结论：✅ 线上 HEAD = `0c10359`（= 本地 `main` 当前，`git status` 显示 up to date）、api/postgres healthy、healthz 200。

### 2.2 GitHub 状态（2026-08-23 实查）

- `gh pr list --state open` → 仅 2 个 open PR：**#49** `docs(ops): lessons 补 gh --body-file 路径坑`（head `docs/lessons-gh-bodyfile`）、**#29** `docs(workflow): #26 验收线…`（head `docs/workflow-acceptance-fix`）。
  - 结论：✅ PR #49 待合（统筹方安排与本 PR 一并合）；PR #29 未安排。
- `gh issue view 50 --json state,title` → `state=OPEN title=图片点击无法放大查看（手机）——补图片预览（Vant ImagePreview）`。
  - 结论：✅ 本票 OPEN，无评论。
- 地图 **issue #1**（`gh issue view 1`）：决策 9 含「不做缩略图/删除/配额（#12 D5）」；**#50 已登记**——决策区「**#50 图片放大预览（进行中，2026-08-22 建票）**」与 Roadmap 清单「3. **#50 图片放大预览**」均含（**F3 更正**：地图 updatedAt `2026-08-23T02:02:50Z`=10:02 本地，晚于本文档初稿对 issue #1 的读取、早于成稿——属统筹方同步更新，初稿「尚未登记」系核验时点差异）。本会话仍不 edit #1（SOP §10 由统筹方更新）。

### 2.3 代码真值（本机仓库，2026-08-23 实查）

| 声明 | 实查命令/行 | 结果 |
|---|---|---|
| `md.ts` 图片渲染纯 `<img>`、无 class、无点击处理 | `grep -n "img src" frontend/src/utils/md.ts` | `md.ts:40` `pushBlock(\`<img src="${m}" alt="图片" loading="lazy">\`)`；图片规则正则 `md.ts:39` `/(https?:\/\/[^\s]+\.(?:png|jpe?g|gif|webp))/gi` ✅ |
| 前端零引用 `showImagePreview`/`ImagePreview` | `grep -rn "showImagePreview\|ImagePreview" frontend/src/ frontend/e2e/` | 零命中 ✅（印证「从未集成图片预览」） |
| 帖子正文注入点 | `grep -n 'v-html="md(post.content)"' frontend/src/views/PostDetail.vue` | `PostDetail.vue:354` `<div v-html="md(post.content)"></div>` ✅ |
| 评论正文注入点 | `grep -n 'v-html="md(c.content)"' frontend/src/components/CommentTree.vue` | `CommentTree.vue:80` `<div class="cbody" v-html="md(c.content)"></div>` ✅ |
| 发帖/评论插入的是**裸 URL** | `grep -n "insertAtCursor\|onImage" frontend/src/views/Write.vue` | `Write.vue:100` `insertAtCursor(content.value, start, end, url, { block: true })`；`editor.ts:17-33` 纯插入函数 ✅（裸 URL，md 图片正则直接匹配） |
| 内联图片已有样式约束 | `grep -n "\.md img" frontend/src/styles/global.css` | `global.css:109-113` `.md img { max-width:100%; border-radius:12px; margin:6px 0 }` ✅（无需新增 CSS） |
| `CommentTree.vue` 递归结构（`.cbody` 互不嵌套） | 读 `frontend/src/components/CommentTree.vue:63-121` | `.cbody` 在 `.cmain`（:72）内；递归实例渲染在 `.replies`（:93-101），是 `.prow2` 的**旁支**——`.cbody` 之间互不嵌套 ✅（支撑 §6.2 无双重触发论证） |
| 测试 mock 模式 | 读 `frontend/src/views/PostDetail.test.ts:18-21`（mock `vant` 为 `{ showToast, showConfirmDialog }`）、`frontend/src/components/CommentTree.test.ts:8-11`（mock `../utils/md`，但**不 mock vant**） | ✅ 现状如此；本票①给 PostDetail vant mock 补 `showImagePreview` ②给现有 `CommentTree.test.ts` 补 `vi.mock('../utils/imagePreview')`（F4：组件新增 util 导入后避免传递加载真实 vant） |

### 2.4 Vant `showImagePreview` API 真值（node_modules 实查，2026-08-23）

命令：`sed -n '1,60p' frontend/node_modules/vant/es/image-preview/function-call.mjs` + `grep closeOnClickImage frontend/node_modules/vant/es/image-preview/ImagePreview.d.ts`

结果摘录：
```
// function-call.mjs defaultConfig:
  showIndex: true,        // 默认 true
  closeable: false,       // 默认无关闭按钮
  closeOnClickOverlay: true,
  loop: true,
  teleport: "body",
// ImagePreview.d.ts: closeOnClickImage: { type: BooleanConstructor; default: true }
```
结论：✅ `showImagePreview` 支持 `{ images, startPosition, showIndex, … }`；`closeOnClickImage` 默认 true（点图关闭）；**`showIndex` 默认 true 且单图也渲染页码**——`ImagePreview.mjs:76` `renderIndex` 在 `props.showIndex` 时无条件输出 `${active+1} / ${images.length}`。→ 需显式传 `showIndex: images.length > 1`（§6.4）。

### 2.5 前端样式加载真值（关键，实查 2026-08-23）

**问题**：`showImagePreview`/`showToast` 是函数式调用，其样式是否自动加载？

实查命令与结果：
- 本地构建产物：`grep -o "\.van-toast\b" frontend/dist/assets/*.css` → **零命中**；`\.van-image-preview\b` → 零命中。
- 线上构建产物（SSH `cd ~/ai-forum/frontend/dist/assets && grep -l "\.van-toast\b" *.css`）：`van-toast -> NONE`；`van-image-preview -> NONE`。唯一命中 `van-toast` 的是 CSS 变量（`--van-toast-background`，来自 `frontend/src/styles/vant-theme.css`），**非 `.van-toast` 类规则**。
- `frontend/node_modules/vant/es/toast/style/index.mjs`：内容为 `import "../../style/base.css"; … import "../index.css"`（含自身 toast css，路径为 `../index.css`）；但**该 style 入口无人引用**（`grep -rn "vant/es" frontend/src/` 零命中）→ toast 样式从未被引入。
- `frontend/node_modules/vant/es/image-preview/style/index.mjs`：含 `import "../index.css"`（image-preview 自身 css）+ swipe/swipe-item/image/popup/overlay/loading/badge/icon/base 依赖。

结论：✅ **函数式组件样式不随按需引入自动加载**——线上 `.van-toast` 结构样式缺失（toast 至今无 Vant 结构样式，属**既有缺陷**，非本票引入）；预览若要渲染正常（overlay/swipe/页码），**必须显式引入样式** `import 'vant/es/image-preview/style'`（§6.1）。

## §3 Grilling 决策记录（D1–D4，2026-08-23 用户确认）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 多图滑动 | **同一段内容（帖子正文 / 单条评论）所有图进同一个预览，左右滑动切换 + 页码** | #50 ticket「目标」段 + 用户确认（2026-08-23） |
| D2 | 预览内能力 | **不内置**「查看原图 / 保存图片」；想保存 → 关掉预览，**长按正文图 → 手机系统菜单** | 用户确认（2026-08-23）；「查看原图无意义」依据 = §2.2 #1 决策 9（#12 无缩略图，预览即原图） |
| D3 | 交互冲突 | **只响应图片本身**：事件委托 `closest('.content-img')` 过滤，点文字/链接照常；命中时 `preventDefault()`；**不拦截**长按/右键/文字选择 | 用户确认（2026-08-23）；技术依据 §2.3（md 图片为裸 `<img>`；长按触发 `contextmenu` 非 `click`） |
| D4 | 加载失败 | 点击**已确认失败**的图（`img.complete && img.naturalWidth === 0`）→ **不打开预览，toast「图片加载失败」**；其余照常 | 用户确认（2026-08-23）；选 toast → 需 §6.1 样式引入修复 |

**备选与为何不选**：
- D2 预览内加「保存」按钮：移动端（尤其 iOS + 非 HTTPS）下载限制多、成功率低，MVP 不值得。
- D4 照常打开（失败图显示破图占位）：对「源图已删除」场景提示不友好。

## §4 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 改动文件 | 纯前端：`md.ts`（加 class）+ `global.css`（`.content-img` cursor，F6 采纳）+ 新建 `utils/imagePreview.ts` + `PostDetail.vue`（+1 行 @click）+ `CommentTree.vue`（+1 行 @click）+ 对应测试 + 决策/计划文档 | handoff §4「只动 … 图片相关 + 测试」（global.css 属图片相关）；§5 |
| **不做** | 后端任何改动 | handoff §4 红线；本票无后端需求 |
| **不做** | 改 `CommentTree.vue` 的 #47 逻辑（深度截断/展开/回复按钮/删除占位/expand-root-id） | handoff §4 红线；§5.4 只加 `.cbody` 一行 |
| **不做** | 预览内保存按钮 / 缩略图 / 查看原图 | D2 |
| **不做** | 拦截长按/右键/文字选择 | D3 |
| **不做** | 全站 toast 样式统一入口改造（main.ts 全局引入） | 本票只需 toast 在提示场景可用；CSS 全局生效（§6.1）即已修复全站 toast，无需另改 main.ts |

**顺带修复（用户选 D4 时认可，非独立范围）**：`utils/imagePreview.ts` 显式 `import 'vant/es/toast/style'`，因 CSS 全局生效，**一次性修复线上 toast 无样式**（§2.5）。判定为 1 行、低风险、与 D4 提示直接相关，纳入本 PR。

## §5 实现方案（每项给「依据」回指 §2/§3）

### 5.1 `frontend/src/utils/md.ts`——图片渲染加 class（改造点①）

- `md.ts:39-41` 图片规则输出加 `class="content-img"`：`<img src="${m}" alt="图片" loading="lazy" class="content-img">`。
- 依据：§2.3（现状无 class，是事件委托选择器的唯一挂钩点）；D3（只响应图片）。
- 改动后内联图行为不变（`alt`/`loading` 不变，`global.css:109` 样式适用），仅多一个 class。

### 5.2 新建 `frontend/src/utils/imagePreview.ts`——事件委托处理器（改造点②）

- 导出 `handleContentImageClick(e: MouseEvent): void`，**按此精确实现**（F1/F7 评审更正：`querySelectorAll` 返回 `NodeList` 无 `.indexOf`、Vant `images` 需 `string[]` URL、需 `startPosition < 0` 防御）：

```ts
import { showImagePreview, showToast } from 'vant'
// 函数式组件样式不随按需引入加载（§2.5）：显式引入保证预览/提示渲染正常
import 'vant/es/image-preview/style'
import 'vant/es/toast/style'

/** #50 图片点击放大：md 渲染容器的事件委托处理器（@click）。 */
export function handleContentImageClick(e: MouseEvent): void {
  const container = e.currentTarget as HTMLElement | null
  const target = e.target as Element | null
  if (!container || !target) return
  const img = target.closest('.content-img') as HTMLImageElement | null
  if (!img) return
  e.preventDefault() // 命中即拦默认行为：防御性保留（评审 F2 实证当前 md() 不产出 <a><img></a>，无行为变更），面向未来
  const images = Array.from(container.querySelectorAll<HTMLImageElement>('img.content-img'))
  const startPosition = images.indexOf(img) // 按元素引用定位，同 URL 重复图也准
  if (startPosition < 0) return
  if (img.complete && img.naturalWidth === 0) {
    showToast('图片加载失败')
    return
  }
  showImagePreview({ images: images.map((i) => i.src), startPosition, showIndex: images.length > 1 })
}
```

- 逻辑要点：`closest('.content-img')` 未命中直接 return（只响应图片，D3）；`preventDefault` 为**防御性保留**（F2：当前 `md()` 图片规则先于链接规则，`<img>` 从不落入 `<a>` 内，无既有跳转行为可改）；`startPosition < 0` 防御（F7）；失败检测（D4）；`showIndex: images.length > 1`（D1 + §6.4）。
- 依据：§2.4（API 默认值）、§2.5（样式必须显式引入）、D1/D3/D4、F1/F2/F7。
- 放在 `utils/`：纯函数签名、可无组件环境单测（§8），与 `editor.ts`/`format.ts` 同目录惯例一致。

### 5.3 `frontend/src/views/PostDetail.vue`——正文注入点（改造点①）

- import `handleContentImageClick`（`../utils/imagePreview`）。
- `:354` 正文 div 改为 `<div v-html="md(post.content)" @click="handleContentImageClick"></div>`。
- 依据：§2.3（注入点位置）；事件委托对 `v-html` 内容天然有效（事件冒泡到容器）。
- **范围红线**：不改该文件任何其他逻辑（赞藏/分享/评论树/举报等）。

### 5.4 `frontend/src/components/CommentTree.vue`——评论注入点（改造点①）

- import `handleContentImageClick`。
- `:80` 评论正文改为 `<div class="cbody" v-html="md(c.content)" @click="handleContentImageClick"></div>`。
- **绑定在 `.cbody`（单条评论）而非 `.comments` 根**：递归树中 `.cbody` 互不嵌套（§2.3 结构实查）→ 点击只命中最近一条评论的 handler、收集**该评论自己**的图、startPosition 不错位、绝无双触发（§6.2）。
- **范围红线**：#47 深度截断/展开/回复按钮/删除占位/`expand-root-id` watch **一行不动**（§4）。

### 5.5 文档

- `docs/handoffs/grilling-decisions/issue-50-image-preview-decisions.md`（已生成，随 PR 入库，SOP §3.4）。
- `docs/plans/50-image-preview.md` + 本设计文档（供评审）。

### 5.6 `frontend/src/styles/global.css`——图片可点视觉暗示（F6 采纳）

- 加 `.content-img { cursor: zoom-in }`（一行，桌面端 hover 出放大图标）。
- 依据：F6（评审建议，非必需）；`.md img` 规则本就在 `global.css:109`，属图片相关。移动端无碍（点击是直觉动作），纯桌面增强。

## §6 关键设计裁决（【决策】，含理由与备选）

### 6.1 函数式组件样式显式引入（toast 既有缺陷的处置）
- **问题**：`showImagePreview`/`showToast` 样式不随按需引入加载（§2.5），预览不引入样式会渲染异常；toast 已是线上既有无样式缺陷。
- **定案【决策】**：`utils/imagePreview.ts` 显式 `import 'vant/es/image-preview/style'` + `'vant/es/toast/style'`；CSS 全局生效 → 预览渲染正常 + **一次性修复全站 toast**。
- **理由**：1 行、零副作用（CSS 体积增量小，多数依赖 base/icon/overlay 已被 `<van-action-sheet>` 引入）；不引入 toast 则 D4 的「提示失败」表现差。
- **备选（不选）**：在 `main.ts` 全局引入 vant 样式（越范围、非最小改动）；仅预览不修 toast（D4 提示无样式，半吊子）。

### 6.2 事件委托绑定位置：`.cbody`（每条评论）而非 `.comments` 根
- **问题**：`CommentTree` 自递归，若在 `.comments` 根绑定，点击会同时触发嵌套实例的 handler → 多开预览 + startPosition 错位。
- **定案【决策】**：把 `@click` 绑在**每条评论的 `.cbody`**（天然是「单条评论 = 同一内容」的作用域）。
- **理由**：`.cbody` 互不嵌套（§2.3 结构实查）→ 点击只命中最近一个 handler；`e.currentTarget` = 该 `.cbody` → 收集该评论自己的图、startPosition 正确；**无双触发**；符合 D1「同一内容」切分。
- **备选（不选）**：`.comments` 根 + `depth===1` 门控（多一层条件、且根容器收集全树图片、跨评论混滑，不符 D1 语义）；stopPropagation 拦截（容易误伤其他冒泡需求）。

### 6.3 加载失败检测口径
- **问题**：怎么判断「图已加载失败」从而走 D4 提示，而不误伤正常/未加载图。
- **定案【决策】**：`img.complete && img.naturalWidth === 0` 判定失败；否则（含仍在加载 `complete=false`）打开预览（Vant 自带加载态）。
- **理由**：`complete` 为加载终态标记、`naturalWidth===0` 表示加载失败；`loading="lazy"` 未加载的图 `complete=false` → 不误判，预览直接按 URL 加载。
- **备选（不选）**：`onerror` 逐图标记（需在渲染层注入，改 md 输出、侵入大）；打开前过滤失败图（打乱滑动顺序、startPosition 错位）。
- **已知限制（F5）**：`naturalWidth===0` 会把「0×0 合法图」误判为失败（`complete=true` 且 `naturalWidth=0` 的极小合法图会走 toast）。极罕见，接受，不改实现。

### 6.4 `showIndex` 显式传 `images.length > 1`
- **问题**：Vant `showIndex` 默认 true，单图也显示「1 / 1」（§2.4）。
- **定案【决策】**：`showImagePreview({ … , showIndex: images.length > 1 })`——多图显示页码，单图不显示。
- **理由**：单图显示「1/1」是无意义噪音；多图页码是 D1 滑动体验的一部分。
- **备选（不选）**：接受默认（单图也显示「1/1」）；完全不显示页码（多图体验弱）。

## §7 边界与不变量清单

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 只响应图片，不碰文字/链接 | `closest('.content-img')` 未命中即 return | D3/§5.2 |
| 2 | 命中图片即拦截默认点击行为（**防御性保留**：F2 实证当前 `md()` 不产出 `<a><img></a>`、无行为变更；面向未来） | `e.preventDefault()` | D3/§5.2/F2 |
| 3 | 长按/右键/文字选择不受影响 | 不监听/不拦截 `contextmenu`、`mousedown` | D3 |
| 4 | 评论树内点击每张图只开一次预览（无双重触发） | `@click` 绑在互不嵌套的 `.cbody` | §6.2/§2.3 |
| 5 | startPosition 指向被点击的图（含同 URL 重复图） | `images.indexOf(clicked)` 按元素引用 | §5.2 |
| 6 | 预览只含「同一段内容」的图（正文/单条评论不混） | 作用域 = 绑定事件的容器 | D1/§6.2 |
| 7 | 已失败图点击不打开空预览、有提示 | `complete && naturalWidth===0` → toast | D4/§6.3（已知限制：0×0 合法图误判，F5，接受） |
| 8 | 预览渲染正常（overlay/swipe/页码） | 显式 `import 'vant/es/image-preview/style'` | §6.1/§2.5 |
| 9 | toast 提示有样式 | `import 'vant/es/toast/style'`（CSS 全局生效） | §6.1/§2.5 |
| 10 | #47 评论树逻辑不变 | 只加 `.cbody` 一行 @click，其余不动 | §4 |
| 11 | 后端零改动 | 改动清单不含任何 `backend/` 文件 | §4 |
| 12 | 空正文/空评论/软删占位不崩 | 无 `.content-img` → handler 直接 return | §5.2 |
| 13 | 单图不显示「1/1」页码 | `showIndex: images.length > 1` | §6.4 |
| 14 | 图片可点有视觉暗示（桌面） | `.content-img { cursor: zoom-in }`（global.css） | F6（采纳）/§5.6 |

## §8 测试与验证计划

**单测（vitest，`frontend/src/`，与组件同目录 `<Name>.test.ts`，mock 模式参照现有测试）**：
- `frontend/src/utils/md.test.ts` 扩展：图片 URL 渲染含 `class="content-img"`；两图各渲染一次（`<img` 计数 2）。
- **新建 `frontend/src/utils/imagePreview.test.ts`**（核心）：mock `vant` 的 `showImagePreview`/`showToast`；容器 `addEventListener('click', handleContentImageClick)` + 真实 DOM 派发 `new MouseEvent('click', { bubbles: true })`：
  1. 点第 2 张 → `showImagePreview({ images:[s1,s2], startPosition:1, showIndex:true })`；
  2. 单图 → `showIndex:false`；
  3. 点非图片元素 → 不调用；
  4. 坏图（`Object.defineProperty` 设 `naturalWidth:0, complete:true`）→ `showToast('图片加载失败')`、不打开预览；
  5. 正常图（`naturalWidth:100, complete:true`）→ 打开预览（显式设属性，排除 jsdom 默认 `naturalWidth=0` 干扰）。
- `frontend/src/views/PostDetail.test.ts` 扩展：vant mock 补 `showImagePreview: vi.fn()`；新增 describe——正文含两图，点第 2 张 → 断言参数。
- **新建 `frontend/src/components/CommentTree.image-preview.test.ts`**：**不 mock `../utils/md`**（真实 md 才渲染 `.content-img`），mock `vant`/`stores/auth`/`utils/format`，stub `Avatar`；评论内容含两图，点第 2 张 → 断言参数（验证评论注入点）。
- **现有 `frontend/src/components/CommentTree.test.ts` 补 mock**（F4）：`vi.mock('../utils/imagePreview', () => ({ handleContentImageClick: vi.fn() }))`——该测试不测图片预览，mock 最干净；避免组件新增 util 导入后传递加载真实 vant（本仓「凡碰 vant 必 mock」惯例，`PostDetail.test.ts`/`ReportSheet.test.ts` 均 mock）。

**本地全量（push 前）**：
```
cd "C:\Users\liyongquan\ai-forum\frontend"
npm run test:unit
npm run build            # vue-tsc -b && vite build（类型检查 + 构建）
cd "C:\Users\liyongquan\ai-forum"
git checkout -- frontend/components.d.ts   # build 会改写，提交前还原（SOP workflow.md §4.5）
git status --porcelain                     # 确认仅本次文件；仓库有大量其他会话遗留未跟踪文件，git add 只加自己的
```

**设备模拟冒烟**（`webapp-testing` / 临时 Playwright spec，viewport 390×844，本地 `npm run dev` + 后端 8080 + 造数；不提交 CI，参照 `frontend/e2e/comment-depth-smoke.spec.ts` 模式）：点正文图 → 全屏预览；多图滑动 + 页码；点图关闭；评论图同样可预览；坏图点击 → toast；截图存 `.smoketest/50/`。

**后端零改动确认**：`git status` 无 `backend/` 文件。

## §9 待评审焦点（Q1–Q7，给评审 agent 定向）

- **Q1** 样式引入方案：`import 'vant/es/image-preview/style'` + `'vant/es/toast/style'` 是否最小正确方案？有无 CSS 体积/重复引入副作用？把「修复 toast 既有缺陷」纳入本 PR（§4 顺带修复）是否越范围？值得盯：涉及既有缺陷处置 + 1 行改动的边界判断。
- **Q2** 事件委托绑在 `.cbody`（§6.2）：基于「`.cbody` 互不嵌套」的**无双重触发**论证是否成立？有无遗漏的冒泡路径（如 `.cbody` 内嵌套其他可点击元素）？值得盯：递归组件的双重触发是最易翻车的点。
- **Q3** 加载失败检测 `img.complete && img.naturalWidth === 0`（§6.3）：在真实浏览器（含 `loading="lazy"`、慢网、缓存）下的可靠性；jsdom 测试靠 `Object.defineProperty` 定属性是否掩盖了真实行为？值得盯：这是 D4 的判定核心。
- **Q4** `showIndex: images.length > 1`（§6.4）：单图不显示页码是否符合产品预期？与 Vant 默认语义的偏离是否值得？值得盯：小决策但直接影响观感。
- **Q5** 范围红线（§4）：改动清单是否严格遵守「后端零改动 + #47 逻辑不碰 + 纯前端」？测试文件是否齐全（4 个测试文件 + 1 个扩展）？值得盯：handoff 红线是硬约束。
- **Q6** `preventDefault()` 行为（§5.2/§7 #2）：原问「图片嵌链接既有跳转是否被改」——**评审 F2 实证否定前提**：`md()` 逐规则跑实，`[![a](img)](url)` / `[https://x.png](url)` / `[text](img)` / `![](img)` 四种写法**均不产出 `<a><img></a>`**（`[https://x.png](url)` 图被丢弃只剩占位文本），**无既有跳转行为**。`preventDefault` 定案为**防御性保留、无行为变更**（面向未来 md.ts 支持标准图片链接语法）。已按 F2 更正 §5.2/§7 #2/决策记录 D3。
- **Q7** 未复核项：**评审指正「无未复核项」表述不成立**——§2.2「#50 尚未登记」曾为假（F3，地图实查已登记）；服务器实查为作者自报（评审方无通道）；Vant 运行时表现（多图滑动/toast/`naturalWidth`）依赖 §8 冒烟实测，**不可省**。现状：**代码/GitHub 侧已实核；服务器侧为自报；运行时表现待冒烟**。

## §10 评审意见采纳记录（2026-08-23 独立复核：有条件通过）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1** §5.2 与源计划不一致（`NodeList.indexOf` / `images` 传元素） | 重要，属实 | §5.2 回填与源计划一致的精确代码（`Array.from` + `.map(i=>i.src)` + `startPosition<0`） |
| **F2** 「`<a><img></a>` 跳转」前提不实 | 重要，属实（`md()` 实证 4 种写法均不产出） | `preventDefault` 改述为「防御性保留、无行为变更」；更正 §5.2/§7 #2/决策记录 D3 依据/源计划注释 |
| **F3** 「#50 尚未登记 Roadmap」不实 | 重要，属实（地图已登记，updatedAt 10:02 晚于初读、早于成稿） | §2.2 更正为「已登记（进行中）」+ 时点标注；源计划 Context 同步 |
| **F4** 现有 `CommentTree.test.ts` 未 mock 传递导入 | 重要，属实 | 补 `vi.mock('../utils/imagePreview')`；§8/源计划 §改动5 测试清单加该项 |
| **F5** 0×0 合法图误判 | 建议，采纳（标注） | §6.3/§7 #7 标注已知限制，不改实现 |
| **F6** cursor 视觉暗示 | 建议，采纳 | 加 `.content-img { cursor: zoom-in }`（§5.6/§7 #14） |
| **F7** §5.2 未提 `startPosition<0` 防御 | 建议，采纳 | 随 F1 一并回填 |
| Q1–Q5、Q6（改述后）、Q7（改述后） | 认可 | 按计划执行；冒烟「坏图 toast / 多图滑动+页码 / 点图关闭」**不可省** |

**推翻项**：无。全部评审发现经独立复核属实；评审方未复核的 SSH/服务器项已如实标注（作者自报）；运行时表现待冒烟实测。
