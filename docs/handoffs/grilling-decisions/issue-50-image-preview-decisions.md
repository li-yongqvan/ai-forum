# #50 图片点击放大查看（手机）——决策记录

> 本文件 = **#50 grilling 决策记录**（SOP §3.4：决策落档，随 PR 入库）。实现计划见 `docs/plans/50-image-preview.md`。
> 溯源约定：**事实**标来源（代码 `file:line` / GitHub issue / 「用户确认(日期)」）；**判断性裁决**标【决策】+理由+备选。
> 数据时点：2026-08-23（决策确认日；服务器 `main`=`0c10359`，api/postgres healthy，healthz ok）。

## Grilling 决策（D1–D4，2026-08-23 用户确认）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 多图滑动 | **同一段内容（帖子正文 / 单条评论）所有图进同一个预览，左右滑动切换 + 页码** | #50 ticket「目标」段（多图滑动切换是明确目标）+ 用户确认（2026-08-23） |
| D2 | 预览内能力 | **不内置**「查看原图 / 保存图片」；想保存 → 关掉预览，**长按正文图 → 手机系统菜单** | 用户确认（2026-08-23）。「查看原图」无意义依据：#12 已定案**不做缩略图**（地图 #1 决策 9 / D5），预览显示的就是原图（同一 URL） |
| D3 | 交互冲突 | **只响应图片本身**：事件委托 `closest('.content-img')` 过滤，点文字/链接照常；命中时 `preventDefault()`（**防御性保留**）；**不拦截**长按/右键/文字选择 | 用户确认（2026-08-23）。技术事实（评审 F2 实证）：`md.ts:39` 图片规则先于 `:43` 链接规则执行，`<img>` **从不落入 `<a>` 内**（4 种写法实跑均不产生 `<a><img></a>`）——`preventDefault` 无既有行为可改，为防御性保留（面向未来 md.ts 支持标准图片链接语法）；长按触发 `contextmenu` 而非 `click`，不冲突 |
| D4 | 加载失败 | 点击**已确认失败**的图（`img.complete && img.naturalWidth === 0`）→ **不打开预览，toast「图片加载失败」**；其余照常 | 用户确认（2026-08-23）。选 toast 提示 → 需同时修 toast 样式（见下「范围」） |

**备选与为何不选**：
- **D2 预览内加「保存」按钮**：移动端（尤其 iOS + 非 HTTPS）下载限制多、实现复杂、成功率低，MVP 不值得。
- **D4 照常打开（失败图显示破图占位）**：对"源图已删除"场景提示不友好，选显式提示。
- **D1 只放大点击那张**：多图帖子/评论体验差，不符 ticket 目标。

## 范围（改动文件红线）

- 纯前端：`frontend/src/utils/md.ts`（图片渲染加 class）、`frontend/src/styles/global.css`（`.content-img` cursor，F6 采纳）、新建 `frontend/src/utils/imagePreview.ts`（事件委托处理器）、`frontend/src/views/PostDetail.vue`（正文注入点）、`frontend/src/components/CommentTree.vue`（评论注入点，只加 `.cbody` 事件绑定）+ 对应测试 + 本决策记录。
- **后端零改动**（handoff 红线；SOP「真值优先」）。
- **#47 评论树逻辑不动**：`CommentTree.vue` 只给 `.cbody` 加一行 `@click`，深度截断/展开/回复按钮逻辑全部不碰（handoff 红线）。
- **不做**：预览内保存按钮、缩略图/查看原图、右键菜单拦截、全站 toast 样式统一入口改造。
- **顺带修复（用户选 D4 时认可）**：函数式组件样式不随按需引入加载（已从构建产物核实：线上所有 CSS chunk 均无 `.van-toast` 规则，**线上 toast 至今无样式**）→ 在 `utils/imagePreview.ts` 里 `import 'vant/es/toast/style'` + `'vant/es/image-preview/style'`，一次性修好 toast + 保证预览渲染正常（CSS 全局生效，无需动 main.ts）。

## 评审闭环

- 2026-08-23 独立复核：**有条件通过（0 阻塞）**，评审意见书 `docs/plans/50-image-preview-设计文档-评审意见书.md`。处置：**F1**（§5.2 精确代码回填）、**F2**（`preventDefault` 改述为防御性保留，无行为变更）、**F3**（地图 #50 已登记更正）、**F4**（`CommentTree.test.ts` 补 `vi.mock('../utils/imagePreview')`）全采纳；**F5**（0×0 合法图误判标注）、**F6**（`.content-img { cursor: zoom-in }`）、**F7**（`startPosition<0` 防御随 F1 回填）建议全采纳。
