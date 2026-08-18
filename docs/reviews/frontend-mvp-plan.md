# AI 智联论坛 · 前端 MVP 主体实施计划（修订稿 v2）

> **本文档供不了解本项目背景的评审者评估前端实施计划。** 所有必要上下文已内嵌，无需翻仓库或历史文档。
> 状态：**修订稿 v2**（2026-08-18），已并入首轮评审反馈，变更明细见文末「附录 A · 修订记录」。评审反馈建议聚焦第 7 节；欢迎自由补充。
> 关联代码：后端已完成并运行于 `http://localhost:8080`；本文档对应仓库 PR #11 之后的下一步工作。
> 本文件为仓库内同步版，与负责人处的 `AI智联论坛-前端MVP实施计划-v2.md` 保持一致。

---

## 0. 评审者需要回答的问题（可以先读这个）

1. **范围取舍**：前端「主体」先做核心循环（登录取径 / feed / 板块 / 帖子详情 / 发帖 / 用户主页），把通知、私信、举报管理、/me 收藏/关注留到后续（因为它们依赖尚未实现的后端包）——这个切分合理吗？还是应该先把后端补齐、前端一次做完？
2. **设计系统**：采用「Vant 组件库做系统组件 + 自定义做内容组件，用双主题 CSS token 统一」的方案（含 token → Vant 映射表先行，见 §3.2 D3），是否可行/值得？有没有更简的做法？
3. **登录墙实现**：「操作触发跳转登录（带 returnTo，仅站内路径）」+「底 Tab 页（/me、/notifications）游客原地展示登录引导」+「/settings 公开（主题切换游客可用）」——是否符合移动端论坛的常见体验？
4. **后端三处小改**：为支撑「用户主页」「我的帖子」和「赞/藏/关注按钮初始态」，计划给后端加 3 处查询能力（见 §4，含 viewer 上下文字段内嵌方案）。边界是否合适？有没有不该碰后端/应该多碰的地方？
5. **风险与遗漏**：这个范围下，有没有明显缺失的功能、交互、或上线前必须处理的问题？
6. **工作量**：作为第一个前端里程碑，12 个页面 + 双主题 + PWA，估算约 15–25 人日（WBS 见 §6）——是否偏高/偏低？

---

## 1. 项目背景

**产品**：面向学院师生的 AI 主题社区——AI 学习、实践、协作、活动在平台内闭环。典型场景：发帖问技术问题、组队竞赛、发布活动、私信沟通。

**形态与规模**：移动端优先的 PWA（手机浏览器打开即可用，可添加到主屏）。远期目标万级注册，**MVP 收敛为约 50 人常驻的社团内部使用**。

**明确不做**：实时音视频、IM 主场景、在线直播、支付、学校统一登录。

**技术栈（已定案，非本计划评审范围）**：
- 后端：Go + Gin 模块化单体 + PostgreSQL（已完成，含用户/内容两个领域包）
- 前端（本计划）：Vue 3 (TypeScript) + Vite + Vue Router + Pinia + Vant + vite-plugin-pwa
- 通信：RESTful JSON API（前端 fetch 直连；gRPC 仅用于 Go 服务间内部调用）
- 部署：单服务器 Docker Compose + nginx（前端静态产物由 nginx 托管，`/api` 反代到 Go）

**设计输入**（已定稿，本计划遵循）：
- 信息架构 v2：15 个路由页、底部 4 Tab（首页/板块/通知/我的）+ 全站右下角发帖 FAB、两级导航（二级页隐藏 Tab）、登录墙（内容开放浏览、操作触发登录）、权限矩阵（guest/user/moderator/admin）
- 两个已采纳的 HTML 原型，定义了**双主题设计语言**：暖白纸感阅读底 + 墨蓝品牌主色 + 琥珀金动作强调；深色主题全套 token；中文系统字体栈；安全区与 ≥44px 触控目标

**首要约束**：**移动端优先**——手机是第一形态。390px 基准 / ≤430px 全屏、触控目标 ≥44px、safe-area 避让（刘海/Home 条）、单手握持（FAB 右下、Tab 底部）、绝不横向滚动、`prefers-reduced-motion`、弱网 app shell 预缓存。

## 2. 后端现状：前端将消费的 API

> 所有端点实际挂载于 `/api/v1` 前缀下（表中从略），nginx 将 `/api` 反代至 Go。

**已可用**：

| 领域 | 端点 | 说明 |
|---|---|---|
| 鉴权 | `POST /auth/register` | 邀请码准入（管理员线下发码），注册即自动登录 |
| | `POST /auth/login` · `GET /auth/me` · `POST /auth/logout` | JWT 7 天，存 localStorage |
| 内容 | `GET /boards` · `GET /topics?board_id=` | 板块/话题（已内置 6 板块/10 话题种子数据） |
| | `GET /posts?tab=all\|follow&board_id=&topic_id=&page=&page_size=` | feed 流（全部=置顶优先+时间倒序；关注=聚合用户/板块/话题三来源） |
| | `GET /posts/:id` · `GET /posts/:id/comments` | 详情（浏览量自增）+ 评论树（邻接表，软删占位） |
| | `POST /posts` · `DELETE /posts/:id` | 发帖（板块必选/话题可选）；删帖（作者或 moderator+） |
| | `POST /posts/:id/pin` · `/feature` | 置顶/精华（moderator+） |
| | `POST /comments` · `DELETE /comments/:id` | 评论（顶层自动楼层号）、删评论 |
| | `POST /likes` · `DELETE /likes` · `POST /favorites` · `DELETE /favorites` | 点赞/收藏（post/comment；重复 409、取消幂等） |
| | `POST /follows` · `DELETE /follows` | 关注/取关（target_type: user\|board\|topic） |

**尚未实现**（影响前端取舍）：
- 通知中心、私信（notify 领域包——骨架已建，未实现）
- 举报提交/处理（moderation 领域包——骨架已建，未实现）
- 收藏列表、关注列表查询、编辑帖子/评论（无更新端点）
- 按作者查帖子、用户公开资料、viewer 状态字段——**本计划将补，见 §4**

## 3. 前端主体实施计划

### 3.1 目标与范围

**做**：让论坛核心循环真正可用——登录取径 → 逛 feed/板块 → 看帖子详情与评论 → 发帖 → 点赞/收藏/关注 → 用户主页。含双主题设计系统、登录墙、发帖草稿、无限滚动分页、PWA 安装，以及**轻量治理能力**（删帖/删评论/置顶/精华，后端已就绪，前端仅按角色显隐入口）。

**明确不做（留后续里程碑，与后端 notify/moderation 一起）**：
- /notifications 页真实数据（前端先放「即将上线」引导页）
- /me 的收藏/关注 tab（前端先空态）
- 私信会话（用户主页「私信」按钮先为 stub，点击提示「即将上线」）
- 举报的提交/处理闭环（入口先置「即将上线」）
- **编辑帖子/编辑评论**（后端无更新端点；用户侧以删除重发代替，需在产品文案中管理预期）

### 3.2 关键决策

| # | 决策 | 内容 |
|---|---|---|
| D1 | 脚手架 | `frontend/` 独立包；Vite + Vue3(TS) + Vue Router(hash) + Pinia + Vant + vite-plugin-pwa + Vitest；npm |
| D2 | 页面 | 12 页（清单见 §3.3）；hash 路由；登录墙规则显式化——**底 Tab 页（/me、/notifications）游客原地展示登录引导（不跳转）；二级页（/write）跳转登录带 `returnTo`**；`returnTo` 仅接受 `/` 开头的站内路径（防开放重定向）；**/settings 公开**（主题切换游客可用，退出登录区仅登录态显示） |
| D3 | 设计系统 | 双主题 CSS token 直接移植原型（亮/深三态规则）；**Vant 做系统组件**（输入框/按钮/弹层/Toast/Tabbar）并主题化到品牌色；**内容组件自定义**（帖子卡/评论树/分段/头像）保持原型视觉；**先行产出「自有 token → Vant `--van-*` CSS 变量映射表」（亮/暗两套），作为设计系统阶段的第一件验收物**；Vant 按需引入控制包体 |
| D4 | Markdown | 移植原型子集渲染器（围栏代码/行内代码/粗体/链接/外链图），**先 HTML 转义**（防 XSS）；**链接协议白名单（仅 http/https/mailto，拦截 `javascript:` 等伪协议）；外链统一 `target="_blank" rel="noopener"`** |
| D5 | 状态 | Pinia：auth（token/用户，localStorage）+ theme（三态：跟随系统/浅色/深色） |
| D6 | API | fetch 包装：统一 `/api/v1` 前缀（dev 环境经 Vite proxy 转发）、Bearer 注入、401 归一（登录过期清 token）、统一错误体 |
| D7 | 交互 | 无限滚动分页（**page/page_size，与 §2 后端实装一致**）；帖子卡赞藏**初始态取自响应内嵌 viewer 字段（见 §4）**，乐观翻转、失败回滚；评论树默认全展开、>3 回复折叠、软删占位；发帖草稿 localStorage（防抖 300ms、7 天过期、恢复提示） |
| D8 | PWA | vite-plugin-pwa：Manifest + Workbox 预缓存 app shell（轻量离线，不做完整离线读写）；图标由 SVG 生成 192/512/maskable，**另出 apple-touch-icon（180×180，iOS 不读 manifest 图标）**；**Service Worker 采用 autoUpdate 更新策略**（避免发版后旧页面引用已失效 chunk）；验收标准中写明 **iOS 主屏 PWA 与 Safari 的 localStorage 相互独立**（登录态不互通，属平台限制） |
| D9 | 测试 | Vitest：md 渲染器（含 XSS 转义与协议白名单）、时间格式化、auth/theme store；**E2E 不入 CI**（本地 Playwright 冒烟） |

### 3.3 页面清单（12 页）

| 路由 | 页面 | 登录墙 | 数据依赖 |
|---|---|---|---|
| /login | 登录 | — | auth.login |
| /register | 注册（邀请码） | — | auth.register |
| /feed | 首页信息流（全部/关注） | 公开（**游客点「关注」tab 原地展示登录引导**） | GET /posts |
| /boards | 板块宫格 / 话题列表 | 公开 | GET /boards, /topics |
| /board/:id | 板块内帖子 | 公开 | GET /posts?board_id= |
| /topic/:id | 话题内帖子 | 公开 | GET /posts?topic_id= |
| /post/:id | 帖子详情 + 评论树 | 公开 | GET /posts/:id, /comments |
| /write | 发帖（选板块/话题 + 草稿） | 需登录→跳转 | POST /posts |
| /user/:id | 用户主页（资料卡 + 帖子流 + 关注/私信按钮） | 公开 | **GET /users/:id**、GET /posts?author_id= |
| /me | 我的（资料卡 + 帖子 tab；收藏/关注空态） | 原地引导 | GET /auth/me |
| /notifications | 通知/私信（引导 stub） | 原地引导 | — |
| /settings | 设置（主题三态 + 退出登录） | **公开**（退出登录区仅登录态显示） | 本地 |

弹窗层（bottom sheet）：发帖选板块/话题、帖子/评论「···」更多操作（**含删除——作者或 moderator+；置顶/精华——moderator+，按角色显隐**）、举报入口（stub 提示「治理即将上线」）、确认对话框。

> 与信息架构 v2 对账：IA v2 共 15 个路由页，本里程碑实现其中 12 页；其余 3 页（私信会话、通知详情、搜索——以 IA v2 清单为准）均依赖未就绪的后端能力，随后续里程碑交付。

### 3.4 目录结构

```
frontend/
├── index.html  package.json  vite.config.ts  tsconfig.json
├── public/icons/（PWA 图标，含 apple-touch-icon）
└── src/
    ├── main.ts  App.vue（框架：navbar/tabbar/fab 按页面显隐）
    ├── styles/tokens.css  global.css  vant-theme.css（双主题 + token→Vant 映射）
    ├── router/index.ts          # hash 路由 + 登录墙守卫 + returnTo 校验
    ├── stores/auth.ts  theme.ts
    ├── api/client.ts  auth.ts  content.ts  types.ts
    ├── utils/md.ts  format.ts
    ├── components/PostCard  CommentTree  SegTabs  Avatar  Empty  LoginGuide
    └── views/Login  Register  Feed  Boards  BoardDetail  TopicDetail
            PostDetail  Write  UserProfile  Me  Notifications  Settings
```

## 4. 后端三处最小补充（随前端里程碑一并交付）

1. `GET /api/v1/posts?author_id=` —— 让「用户主页」和「我的帖子」可用（现有列表查询加一个 WHERE 条件）。
2. **viewer 上下文字段内嵌** —— `GET /posts`（列表）与 `GET /posts/:id`（详情）在请求携带有效凭证时，为每个帖子附带 `viewer: { liked, favorited, following_author }`，供赞/藏/关注按钮渲染初始态，避免「已操作却显示未操作」、也避免列表页为每个卡片补发状态请求。**v1 评审稿中的独立 `follows/state` 端点取消，并入本项**（用户主页的关注初始态由下方 `GET /users/:id` 的 `viewer.following` 提供；板块/话题详情页的关注按钮初始态同理，由对应响应内嵌 `viewer.following`）。
3. `GET /api/v1/users/:id` —— 用户公开资料（昵称/头像/简介/加入时间 + 帖子数/粉丝数/关注数 + `viewer.following`），供用户主页头部渲染；**零帖用户的主页因此也能正常展示**（否则从帖子列表反查 author 会在零帖时连昵称都渲染不出）。

其余一律不动后端。收藏列表、关注列表、编辑帖子/评论等留后续。

## 5. 验证方案

1. `npm install && npm run build` —— 产物可构建；**核对包体预算：首屏 JS gzip ≤ 300 KB（建议值）**
2. `npm run test:unit`（Vitest）—— md 渲染器（含 XSS 转义、协议白名单）、格式化、auth/theme store
3. **Playwright 本地冒烟**（连运行中的后端）：注册 → feed 可见种子帖 → 发帖 → 帖子详情 → 评论 → 点赞 → 关注 → 关注流出现；**点赞/收藏/关注后刷新页面，状态保持（验证 viewer 字段）**；**moderator 账号置顶/删帖各一次**
4. 手动：双主题切换三态（**含游客态主题切换**）、登录墙（游客点发帖 → /login?returnTo= 回跳；**returnTo 外部地址被拒**）、发帖草稿恢复、PWA 可安装
5. **真机走查（Android 优先；环境无 iOS，2026-08-18 调整）**：
   - **Android（OnePlus Ace 5 / Chrome）**：安全区与 ≥44px 触控目标、双主题三态（含游客）、登录墙（returnTo 回跳/站外被拒）、草稿恢复、断网 app shell 可打开、核心循环手测
   - **PWA 安装项延后**：Chrome 仅对 HTTPS/localhost 提供安装，LAN HTTP 与线上 `http://122.51.233.225/` 均为纯 HTTP（#8 已知债）——安装验证待上域名+HTTPS
   - **iOS 专属项待有设备后补**：apple-touch-icon 生效、主屏 PWA 与 Safari localStorage 独立、iOS safe-area；代码已按规范实现（`apple-touch-icon-180x180.png`、`env(safe-area-inset)`）
6. 后端小改回归：`go test ./...` + live curl

## 6. 工作量估算（WBS）

| 阶段 | 内容 | 估时（人日） |
|---|---|---|
| 1. 脚手架 + 设计系统 | Vite/路由/Pinia 骨架；双主题 token 移植；token → Vant 映射表打通（§3.2 D3 验收物） | 2–3 |
| 2. 基础设施 | api client（/api/v1、401 归一）、auth/theme store、登录墙守卫 + returnTo 校验 | 1–2 |
| 3. 组件 + 12 页面 | 帖子详情（评论树）与发帖页最重，各 1.5–2 天；其余每页 0.5–1 天；含 PostCard/CommentTree/md 渲染器 | 8–12 |
| 4. 后端三处小改 + 联调 | author_id 过滤、viewer 字段、users/:id | 1–2 |
| 5. 测试 + 冒烟 | Vitest 单测 + Playwright 本地冒烟 + 修复 | 2–3 |
| 6. PWA + 真机走查 | manifest/图标/SW 策略；iOS Safari 与主屏验证 | 1–2 |

**合计约 15–25 人日（单人 3–5 周）**。前提：原型与双主题 token 已定稿、AI 辅助编码。低估风险集中在：双主题细节走查、评论树交互、Vant 主题化映射。

## 7. 评审焦点问题

见第 0 节。特别希望评审者从「第一次用这个产品的成员」视角判断：**这 12 页是否足够让人用起来？哪些缺失会让产品显得半成品？**

## 8. 后续里程碑（本计划之外，供参考）

- 后端 notify + moderation 包实现 → 前端接通：通知中心、私信、举报提交/处理闭环、/me 收藏/关注 tab
- 编辑帖子/评论（需后端补更新端点）
- OpenAPI 3.1 描述后端 → 前端类型自动生成
- Web Push 可选增强（iOS 16.4+ 需添加到主屏，MVP 不做）

---

## 附录 A · 修订记录（v1 评审稿 → v2）

1. **后端补充由 2 处改为 3 处**：新增 `GET /users/:id` 公开资料端点（修复零帖用户主页无数据源）；赞/藏/关注初始态统一改为响应内嵌 `viewer` 字段，原独立 `follows/state` 端点取消并入（§0、§4、§3.2 D7）。
2. **分页参数统一**为 `page/page_size`，与后端实装一致（§3.2 D7）。
3. **/settings 由「需登录」改为「公开」**：主题切换游客可用，退出登录区仅登录态显示（§3.2 D2、§3.3）。
4. **登录墙规则显式化**：「底 Tab 原地引导 / 二级页跳转登录」；returnTo 增加站内路径校验；游客点 feed「关注」tab 原地引导（§3.2 D2、§3.3）。
5. **md 渲染器安全补充**：链接协议白名单（http/https/mailto）+ 外链 `rel="noopener"`（§3.2 D4，测试同步覆盖）。
6. **轻量治理能力纳入本期**：删帖/删评论/置顶/精华经「···」菜单按角色显隐（后端已就绪，§3.1、§3.3、§5）。
7. **「编辑帖子/评论」明确列入不做清单**（§3.1），并写入后续里程碑（§8）。
8. **PWA 补充**：apple-touch-icon、SW autoUpdate 策略、iOS 主屏独立存储写入验收（§3.2 D8、§5）。
9. **Vant 主题化前置验收物**：token → `--van-*` 映射表（亮/暗两套）+ 按需引入（§3.2 D3、§3.4）。
10. **新增 §6 工作量估算（WBS）**，回答 §0 第 6 问；§5 增加包体预算与真机走查项。
11. 文档一致性：统一 `/api/v1` 前缀表述（§2）；12 页与 IA v2 15 页对账说明（§3.3）。
