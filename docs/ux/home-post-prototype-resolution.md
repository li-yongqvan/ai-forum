# #6 移动端首页与帖子详情原型 · 决议

> 处理时间：2026-08-18
> 上游输入：`docs/ux/info-architecture.md` v2、`docs/ux/ia-prototype.html`（#9 已裁定采纳）
> 本决议替代 handoff `docs/handoffs/06-home-post-prototype.md`，该 handoff 已处理并删除。

## 决策结论

| 决策 | 选项 | 结论 | 理由 |
|---|---|---|---|
| D0 与 `ia-prototype.html` 的关系 | a) 在其基底上深化两页 / b) 独立两页原型 / c) 直接采纳 ia-prototype | **a + b 的折中**：以 `ia-prototype.html` 的设计 token、组件与数据语境为基底，独立产出只含「首页 + 帖子详情」的聚焦原型 `docs/ux/home-post-prototype.html` | 全站原型已验证导航与全局 chrome，#6 的增量价值是「把 §5.2 信息流/帖子规格逐条落到可点击、可评审、可验证角色权限的状态」；独立文件降低评审负担，同时与已有产物同构 |
| D1 首页信息流覆盖项 | 全部/关注分段、排序、卡片、分页、空态 | **全部纳入** | 与 v2 §5.2/§5.7 对齐；mock 数据扩展至 14 帖以验证分页/加载更多 |
| D2 帖子详情覆盖项 | Markdown 子集、评论树折叠/软删占位、点赞/收藏/分享/举报、管理操作、通知锚点 | **全部纳入** | 含 >3 回复折叠、软删节点 c7、锚点高亮 1.6s、mod/admin 管理操作可见性 |
| D3 技术形态 | 单文件 HTML 零依赖 / Vue3+Vant 组件级 | **单文件 HTML** | prototype 目的是评审而非实现；与 #9 产物同构，浏览器打开即用 |
| D4 交付物与验收 | 原型文件 + 规格对照 + 评审确认 | **交付 `docs/ux/home-post-prototype.html` + 本决议** | 验收：0 控制台错误、无横向滚动、可点目标 ≥44px、角色切换可见性符合 §5.0 矩阵 |

## 交付物

- **可点击原型**：`docs/ux/home-post-prototype.html`
  - 默认以成员（user）身份进入，顶部「演示控制台」可切换游客 / user / moderator / admin 与主题三态。
  - 首页：全部/关注分段、置顶优先+时间倒序、帖子卡片、加载更多/到底啦、空态。
  - 帖子详情：Markdown 子集渲染、评论树（楼层号、>3 条折叠、软删占位）、点赞/收藏/分享/举报、管理操作、底部评论栏。
  - 其他 Tab 为占位，FAB 提示「发帖流程不在本原型范围内」。
- **决议文档**：`docs/ux/home-post-prototype-resolution.md`（本文件）

## v2 §5.2 规格对照清单

| 规格项 | 原型中位置 | 状态 |
|---|---|---|
| 首页「全部 / 关注」分段切换 | 首页顶部 seg | ✅ |
| 排序：置顶优先 + 时间倒序 | `sortedPosts()`，置顶帖在列表顶部 | ✅ |
| 分页：20 条/页 + 加载更多 | 原型用 5 条/页演示加载更多按钮与「到底啦」 | ✅（演示页长） |
| 帖子卡片：头像/名字/时间 + chips + 标题≤2 行 + 摘要 2 行 + 操作行 | `.card` 结构 | ✅ |
| 点赞/收藏即时翻转并计数 | `toggleLike()` / `toggleFav()` | ✅ |
| 正文 Markdown 子集（围栏/行内代码/粗体/链接/外链图） | `md()` | ✅ |
| 评论树：邻接表 + floor、默认全展开、>3 折叠 | `commentHtml()` | ✅ |
| 评论软删占位保留回复链 | `c.deleted` 渲染 | ✅ |
| 通知锚点：折叠祖先展开 + 高亮 1.6s | URL `?c=cmt-id` 触发 `.hl` 动画 | ✅ |
| 管理操作可见性按 §5.0 矩阵 | 帖子/评论 more 菜单按角色渲染 | ✅ |
| 登录墙：操作触发登录提示 | 游客点赞/回复提示登录 | ✅ |
| 双主题 token + safe-area + ≥44px 触控 | CSS token、env(safe-area)、pact 44px | ✅ |

## 验证记录

- 文件可直接用浏览器打开：`docs/ux/home-post-prototype.html`
- 本地静态服务器返回 200，无服务错误
- 预期仍需在浏览器中确认：0 JS 控制台错误、无横向滚动、分段/点赞/收藏/举报/评论/角色切换交互正常

## 遗留问题（已记录为下游依赖）

- **解封审计 gap**：§5.0 权限矩阵含 admin「封禁/解封用户」，但 #5 `moderation_actions` 动作枚举仅 `delete_post/delete_comment/ban_user/warn`，缺少 `unban_user`。解封作为管理操作必须留痕，否则违反「治理操作留痕」原则。本决议已更新 `docs/ux/info-architecture.md` §10，明确要求在 #5 数据模型（或 schema patch）中**必须**加入 `unban_user`；本原型仅在演示控制台展示角色切换，不涉及解封交互。

## 下一步

1. 团队评审 `docs/ux/home-post-prototype.html`，重点确认「单手可用、无歧义」。
2. ~~关闭 GitHub issue #6 并更新 Wayfinder 地图（#1）。~~ ✅ 已通过 `gh api` 同步（见 [#6 resolution comment](https://github.com/li-yongqvan/ai-forum/issues/6#issuecomment-5317256635)）。
3. 后端开工前，在 #5 数据模型层补充 `moderation_actions.action` 枚举值 `unban_user`。
4. 后续实现阶段以 #9 的 `ia-prototype.html` + 本原型共同作为 UI 输入。
