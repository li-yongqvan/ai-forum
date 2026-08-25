# #50 图片点击放大查看（手机）——补图片预览（Vant ImagePreview）

> 计划文档 · 2026-08-23 · 实现会话产出（决策已与用户逐项确认）。handoff：`docs/handoffs/50-image-preview.md`；决策记录：`docs/handoffs/grilling-decisions/issue-50-image-preview-decisions.md`。
> 状态：待用户审阅后进入实现。

## Context（为什么做）

帖子 / 评论里的图片（markdown 渲染）在手机上**点击无法放大**——`md.ts` 渲染纯 `<img>`，前端从未集成图片预览（Vant `showImagePreview` 零引用）。电脑可右键新开，手机点击无反应。属**功能缺失补齐**（非回归 bug）。

目标：点击帖子正文 / 评论里的图片 → 弹出**全屏放大预览**（Vant `showImagePreview`），**同一段内容**（帖子正文 / 单条评论）多张图可**左右滑动切换**。

**纯前端，后端零改动。** 真值核对（2026-08-23）：服务器 `main`=`0c10359`（与本地一致）、api/postgres healthy、healthz ok；open PR：#49（lessons docs，统筹方安排与本 PR 一并合）、#29（workflow 验收线，未安排不动）。地图 #1 已读（**#50 已登记**：决策区 + Roadmap 均含「#50 图片放大预览（进行中）」，统筹方 2026-08-23 10:02 更新，F3 更正；本会话不 edit #1）。

## 已与用户确认的决策（2026-08-23 grilling，D1–D4）

1. **多图滑动**：同一段内容所有图进同一个预览，左右滑动切换、显示页码。
2. **预览内能力**：不内置「查看原图 / 保存」——#12 已定案无缩略图，预览即原图；保存靠关掉预览后**长按正文图 → 系统菜单**。
3. **交互冲突**：只响应图片本身。事件委托 `closest('.content-img')` 过滤，点文字/链接照常；命中时 `preventDefault()`（**防御性保留**——评审 F2 实证当前 `md()` 不产出 `<a><img></a>`、无既有行为可改，面向未来兜底）；**不拦截**长按/右键/文字选择。
4. **加载失败**：点**已确认失败**的图（`img.complete && img.naturalWidth === 0`）→ 不打开预览，toast「图片加载失败」；其余照常。

## 改动文件（范围红线：纯前端，后端零改动）

- `frontend/src/utils/md.ts` —— 图片渲染加 `class="content-img"`（改造点①，唯一一行）。
- `frontend/src/styles/global.css` —— 加 `.content-img { cursor: zoom-in }`（F6 采纳：桌面端可点视觉暗示，一行）。
- `frontend/src/utils/imagePreview.ts` —— **新建**，事件委托处理器 `handleContentImageClick`（改造点②，两处注入点共用）。
- `frontend/src/views/PostDetail.vue` —— 正文 `v-html` div 加 `@click`（注入点①）。
- `frontend/src/components/CommentTree.vue` —— `.cbody` 加 `@click`（注入点②），**只加这一行，#47 评论树逻辑不碰**。
- 测试：`md.test.ts` 扩展 + 新建 `imagePreview.test.ts` + `PostDetail.test.ts` 扩展 + 新建 `CommentTree.image-preview.test.ts`。
- 文档：`docs/handoffs/grilling-decisions/issue-50-image-preview-decisions.md`（已生成）。

不碰 `backend/`、`api/content.ts`、`api/types.ts`、`vite.config.ts`、其他 views。

## 关键机制（已核实，实现依据）

**`showImagePreview` API（Vant ^4.10，已从 `node_modules/vant/es/image-preview/` 核实）**：
- `showImagePreview(options)` 支持 `{ images, startPosition, showIndex, ... }`；`closeOnClickImage` **默认 true**（点图关闭）、`loop`、`teleport:'body'` 均默认即合。
- **`showIndex` 默认 true 且单图也渲染 "1 / 1"**（`ImagePreview.mjs:76`）→ 显式传 `showIndex: images.length > 1`（单图不显示页码，多图显示"1/2"）。

**函数式组件样式必须显式引入（关键，已从构建产物核实）**：`showToast`/`showImagePreview` 是函数式调用，样式**不随 unplugin-vue-components 按需引入加载**——线上所有 CSS chunk 均无 `.van-toast` 规则（**线上 toast 至今无样式**）。故 `imagePreview.ts` 显式 `import 'vant/es/image-preview/style'` + `import 'vant/es/toast/style'`（CSS 全局生效，一次性修好 toast + 保证预览 overlay/swipe/页码正常）。

**递归 CommentTree 无双重触发（已按模板 DOM 结构追踪确认）**：`.cbody`（单条评论内容）**互不嵌套**——嵌套 CommentTree 渲染在 `.replies`（`.cbody` 的旁支），点击冒泡不经过其他 `.cbody`。故把 `@click` 绑在**每条评论的 `.cbody`**（而非 `.comments` 根）：
- 点击只命中最近一条评论的 handler，`e.currentTarget` = 该 `.cbody` → 收集**该评论自己**的图、startPosition 不错位；
- **绝无双触发**；且「同一内容多图」天然按「帖子正文 / 单条评论」切分，符合 D1。

**重复 URL / lazy 加载**：`images.indexOf(clicked)` 按**元素引用**定位（同 URL 重复图也准）；`loading="lazy"` 的图未加载时 `img.complete` 为 false，不误判失败，打开预览由 Vant 直接加载 URL。

## 改动 1：`md.ts`

`md.ts:39-41` 图片规则加 class（唯一改动）：
```ts
s = s.replace(/(https?:\/\/[^\s]+\.(?:png|jpe?g|gif|webp))/gi, (m: string) =>
  pushBlock(`<img src="${m}" alt="图片" loading="lazy" class="content-img">`),
)
```
发帖/评论插入的是**裸 URL**（`Write.vue:100` + `editor.ts`，`block` 换行），该规则已匹配，行为不变，仅加 class。

## 改动 2：新建 `utils/imagePreview.ts`

```ts
import { showImagePreview, showToast } from 'vant'
// 函数式组件样式不随按需引入加载（toast 此前一直无样式）；显式引入保证预览/提示渲染正常
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
  const startPosition = images.indexOf(img)
  if (startPosition < 0) return
  if (img.complete && img.naturalWidth === 0) {
    showToast('图片加载失败')
    return
  }
  showImagePreview({ images: images.map((i) => i.src), startPosition, showIndex: images.length > 1 })
}
```
纯函数签名（无 Vue 依赖），可无组件环境单测；`e.currentTarget` = 绑定事件的 md 容器（正文 div / 评论 `.cbody`）。

## 改动 3：`PostDetail.vue`

- import `handleContentImageClick`（`../utils/imagePreview`）。
- `:354` 正文 div：`<div v-html="md(post.content)" @click="handleContentImageClick"></div>`。
- 仅此两行，不碰赞藏/分享/评论树/举报等任何现有逻辑。

## 改动 4：`CommentTree.vue`

- import `handleContentImageClick`（`../utils/imagePreview`）。
- `:80` 评论正文：`<div class="cbody" v-html="md(c.content)" @click="handleContentImageClick"></div>`。
- **#47 逻辑（深度截断 / 展开 / 回复按钮 / 删除占位 / expand-root-id watch）一行不动**。

## 改动 5：测试

- **`md.test.ts`** 扩展：图片 URL 渲染含 `class="content-img"`；两图各渲染一次（`<img` 计数 2）。
- **新建 `imagePreview.test.ts`**（核心，mock `vant` 的 `showImagePreview`/`showToast`，容器 `addEventListener` + 真实 DOM 派发 `new MouseEvent('click', { bubbles: true })`）：
  1. 点第 2 张 → `showImagePreview({ images:[s1,s2], startPosition:1, showIndex:true })`；
  2. 单图 → `showIndex:false`；
  3. 点非图片元素 → 不调用；
  4. 坏图（`defineProperty` `naturalWidth:0, complete:true`）→ `showToast('图片加载失败')`、不打开预览；
  5. 正常图（`naturalWidth:100, complete:true`）→ 打开预览（显式设属性，排除 jsdom 默认 `naturalWidth=0` 干扰）。
- **`PostDetail.test.ts`** 扩展：vant mock 补 `showImagePreview: vi.fn()`（util 经 'vant' 取用，缺则 undefined）；新增 describe——正文含两图，点第 2 张 → 断言参数（`images`/`startPosition:1`/`showIndex:true`）。
- **新建 `CommentTree.image-preview.test.ts`**：**不 mock md**（真实 md 才渲染 `.content-img`），mock `vant`/`stores/auth`/`utils/format`，stub `Avatar`；评论内容含两图，点第 2 张 → 断言参数（验证评论注入点）。
- **现有 `CommentTree.test.ts` 补 mock**（F4）：`vi.mock('../utils/imagePreview', () => ({ handleContentImageClick: vi.fn() }))`——该测试不测图片预览，mock 最干净；避免组件新增 util 导入后传递加载真实 vant（本仓「凡碰 vant 必 mock」惯例，`PostDetail.test.ts:18`/`ReportSheet.test.ts:5`）。

## 边界情况（实现时对照）

帖子正文与评论各是独立容器（正文图不会混入评论图）；多图同 URL（元素引用定位）；lazy 未加载图（不误判失败）；**图片嵌链接不产生 `<a><img></a>`**（F2 实证：`md()` 图片规则先于链接规则，`<img>` 从不落入 `<a>` 内；`preventDefault` 仅防御性保留，无行为变更）；软删评论占位（无 `.cbody` 无 img，天然跳过）；空正文/空评论（无 `.content-img`，handler 直接 return）。

## 验证（SOP §7/§9）

```
cd "C:\Users\liyongquan\ai-forum\frontend"
npm run test:unit
npm run build            # vue-tsc -b && vite build（类型检查 + 构建）
cd "C:\Users\liyongquan\ai-forum"
git checkout -- frontend/components.d.ts   # build 会改写，提交前还原
git status --porcelain                     # 确认仅本次文件；仓库有大量其他会话遗留未跟踪文件，git add 只加自己的
```

设备模拟冒烟（`webapp-testing` / 临时 Playwright spec，390×844，不提交；本地 `npm run dev` + 后端 8080 + 造数）：点正文图 → 全屏预览；多图滑动 + "2/3" 页码；点图关闭；评论图同样可预览；坏图点击 → toast「图片加载失败」；截图存 `.smoketest/50/`。

分支：`git checkout -b feat/image-preview`；动手前 `git status --porcelain` 查脏、`git fetch origin && git pull origin main`。PR 走 SOP §8（`MSYS_NO_PATHCONV=1 gh`，合并前 `/code-review`，**用户授权后** merge，**一并合 PR #49**）；地图 #1 更新交给统筹方，本会话不自行 edit。
