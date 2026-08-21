# AI 智联论坛 · 移动端信息架构与设计规格 v2（Issue #9 修订版）

> **本文档是「AI 智联论坛」移动端 MVP 的信息架构与界面设计规格 v2，可直接作为 UI 生成输入。**
> v2 基于 2026-08-18 设计评审意见修订：修复内在矛盾 6 处、补齐缺失规格 9 项、澄清表述若干。v1→v2 逐项对照见 §9。
> **v2 定稿裁决（2026-08-18）：** 两处与 v1 grilling 决策冲突的修订，经用户拍板全部采纳——① 取消搜索占位框；② 置顶/精华纳入 MVP 管理操作。
> 用途：① #6「移动端首页与帖子详情原型」的输入；② 供其他模型据此复现设计，对比生成效果。
>
> **配套原型：** `docs/ux/ia-prototype.html`（单文件 HTML，浏览器直接打开）。该原型由 **Kimi Agent 按本规格 v2 生成**（2026-08-18，经浏览器验证：4 Tab 切换/进详情/返回正常、0 JS 错误、无横向溢出），用户裁定采纳为正式交付物；v1 实现已删除（内容已被 v2 规格覆盖）。

---

## 0. 一句话

**AI 智联论坛**：面向学院师生的 AI 主题社区（学习/实践/协作/活动闭环），50 人规模 MVP，移动端优先（PWA），内容治理前置。本规格覆盖 MVP 的全部页面与导航。

## 1. 设计语言

### 1.1 视觉母题：智联 · 数据流
- 亮色为主的**暖白纸感**阅读底 + **墨蓝**品牌主色 + **琥珀金**动作强调（发帖 FAB、选中、待处理徽章）。
- 背景有极淡的连接点阵（径向渐变网格），呼应「AI 智联」的网络母题；品牌 logo 为墨蓝→靛蓝渐变圆角方块内白色写帖图标。
- 克制、精致、内容优先——不抢信息架构评审的注意力，但每个细节有设计意图。

### 1.2 调色板（CSS token，双主题）【v2 修订】
```
亮色：  bg #F6F4EF · surface #FFFFFF · surface-2 #EFECE4 · surface-3 #E7E3D9
        ink #1B2434 · ink-2 #57606E · ink-3 #828C9B · line #E4E0D6
        brand #1F3A5F · brand-2 #2B5FA8 · brand-soft #E4ECF5
        accent #E8A33D · accent-strong #C07A10 · accent-soft #FBF0DC · accent-ink #6B4309
        danger #D64541 · success #2F7D50
深色：  bg #12161D · surface #1A2029 · surface-2 #212936 · surface-3 #293343
        ink #E9ECF2 · ink-2 #A6B0C0 · ink-3 #78839A · line #333E4F
        brand #5B8DCC · brand-2 #7FA8DC · brand-soft #22334A
        accent #E8A33D · accent-strong #E8A33D · accent-soft #3A2C14 · accent-ink #F0C886
        danger #E3655F · success #4CA075
```
规则：`:root` 定完整亮色；`@media (prefers-color-scheme: dark)` 仅在 `:root:not([data-theme="light"])` 下重定义 token；`:root[data-theme="dark"]` 再定义一次（显式主题可覆盖系统）。所有颜色走 token，绝不在 media 块内直接写色值。

**v2 修订点：**
1. 深色主题重定义**全部** token，包括 v1 遗漏的 `brand-soft` / `accent-soft` / `accent-ink`（浅色 tint 原样用在深底上会浮块）；`line` 与 `surface-3` 深色取值错开（v1 同为 `#2A3342` 导致分割线消失）。
2. 对比度达标：`ink-2` 加深至 `#57606E`（亮底 ≥4.5:1，正文/meta 文字可用）；`ink-3` 仅用于 ≥13px 辅助文字或装饰；新增 `accent-strong` 承担白底上的琥珀强调（FAB 底、选中态、徽章底，白图标/白字对比 ≥3:1），`accent #E8A33D` 仅作装饰性填充与渐变。

### 1.3 字体
- 中文系统栈：`"PingFang SC","HarmonyOS Sans SC","Microsoft YaHei","Noto Sans CJK SC",-apple-system,"Segoe UI",sans-serif`（不引 web font，CSP 与加载成本考虑）。
- 数字/计数一律 `font-variant-numeric: tabular-nums`。
- 标题层次靠字重（标题 700/800）与字号（详情 20px / 列表卡 15.5px / 正文 14.5px / meta 11-13px）。

### 1.4 布局与动效
- **390×844 手机框**：桌面端居中、深色舞台衬托（径向蓝+琥珀微光）、圆角 46px 手机壳；≤430px 全屏无壳。
- 状态栏 44px（9:41 + 信号/wifi/电量 SVG，**原型演示件**；实际 PWA 由系统状态栏 + `env(safe-area-inset-top)` 接管）；导航栏 48px；Tab 58px + `env(safe-area-inset-bottom)`；FAB 54px 圆形，底距 `calc(76px + env(safe-area-inset-bottom))`。
- 触控：所有可点目标 ≥44×44px。
- 动效：页面切换 `pageIn` slide-in 0.24s；底部弹层（sheet）translateY 上滑 0.26s cubic-bezier；toast 底部上浮；点赞/卡片按压有 scale 反馈；尊重 `prefers-reduced-motion`。
- 图标：24×24 线性 stroke 1.8 round 手写 SVG（首页/板块/通知/我的/加号/返回/心/星/分享/更多/旗帜/垃圾桶/盾牌/发送/齿轮/铅笔/聊天气泡/置顶/精华/警告/封禁/关闭等）。

## 2. 信息架构决策（9 项）【v2 修订】

| # | 决策 | 定案 |
|---|---|---|
| D1 | 页面清单 | 15 路由页 + 弹窗层（见 §4）；**无搜索功能，亦不设占位框**（§7）【定稿裁决：取消占位】 |
| D2 | 底部 Tab | **4 tab：首页 / 板块 / 通知 / 我的** + 全站右下角 FAB 发帖 |
| D3 | 层级 | IA 层级两级为主，二级页**隐藏 Tab**、全屏 push 带返回；**导航栈深度不限**（用户主页↔帖子可往复）；**导航栈基于 History API（hash 路由 + popstate）**，系统返回键/浏览器后退行为符合预期 |
| D4 | 治理 | 举报内嵌（帖子/评论/用户页 → 弹窗选原因，**pending 期内防重复**，见 §5.6）；管理操作内嵌（删帖/删评论/置顶/精华/警告/封号，可见性按 §5.0 权限矩阵）【定稿裁决：置顶/精华纳入】；「我的」页管理入口 → 举报处理列表 + 处理弹窗 → `report_result` 通知闭环；申诉留 fog（§7） |
| D5 | 产出 | 可点击单文件 HTML 原型 + 本文档（页面清单/导航图/实体映射/权限矩阵/空态清单） |
| Q1 | 首页信息流 | 首页顶部「**全部 / 关注**」两分段；**排序规则见 §5.2**；空态见 §5.7 |
| Q3 | 板块页 | 顶部「**板块 / 话题**」分段；话题是全站维度（#5 `follows_topics` 语义），从属板块但可独立浏览 |
| Q5 | 通知页 | 顶部「**通知 / 私信**」分段；通知点击跳转锚点（**折叠评论自动展开路径 + 高亮**，见 §5.2）；未读红点 + 全部已读 |
| Q8 | 登录墙 | **内容开放浏览，操作触发登录**（点赞/评论/发帖/关注/私信/通知/我的）；**游客点「通知/我的」Tab 原地展示登录引导页（非跳转）**；登录后返回原页面，取消登录留在原页 |

## 3. 导航结构图（ASCII）

```
L0 认证（全屏，无 Tab / FAB）
  /login 登录 ◄──► /register 注册（邀请码准入，§4 注）──► /feed

L1 一级 Tab（底部常驻 4 + 全站 FAB）
  /feed          信息流（全部/关注 分段）
  /boards        板块/话题 分段
  /notifications 通知/私信 分段（游客=登录引导页）
  /me            我的（个人主页式：资料卡 + 帖子/收藏/关注 分段；游客=登录引导页）
                 └─ 管理入口「举报处理」（moderator/admin 可见，带待处理数徽章）

L2 二级页（push 全屏，隐藏 Tab；FAB 显隐见 §5.1）
  FAB ─► /write 发帖（选板块必填→话题可选，草稿 localStorage 持久化）
  首页流/列表 ─► /post/:id 帖子详情（评论树·点赞·收藏·举报·分享·管理操作）
  /boards ─► /board/:id 板块内帖子（话题 chips 筛选）
  /boards ─► /topic/:id 话题内帖子
  /notifications(私信)/用户主页 ─► /chat/:userId 会话（id=对方用户 id，§5.4）
  /me ─► /profile-edit 资料编辑 · /settings 设置（主题三态+退出登录） · /reports 举报处理列表
  任意作者入口 ─► /user/:id 用户主页（关注/私信；admin 可封禁）

弹窗层（bottom sheet）：举报（选原因）、处理举报（动作+备注）、发帖选板块/话题、帖子更多操作、确认对话框
```

## 4. 页面清单（15 路由页 + 弹窗）

| 路由 | 页面 | 层级 | 主要入口 | 对应实体(#5) | 关键操作 |
|---|---|---|---|---|---|
| /login | 登录 | L0 | 登出/操作触发 | users | 登录（原型：选演示身份） |
| /register | 注册 | L0 | 登录页切换 | users | 邀请码 + 注册（**邀请码由管理员线下生成/分发，一次性有效，DB 直管，管理 UI 留 fog**） |
| /feed | 首页信息流 | L1 | Tab | posts/boards/topics/follows_* | 全部/关注切换、点赞/收藏/举报 |
| /boards | 板块 | L1 | Tab | boards/topics/follows_* | 板块宫格↔话题列表分段 |
| /notifications | 通知 | L1 | Tab | notifications/messages | 通知/私信分段、未读、全部已读 |
| /me | 我的 | L1 | Tab | users/favorites/follows_* | 帖子/收藏/关注分段；管理入口(mod) |
| /post/:id | 帖子详情 | L2 | 列表/通知/关注流 | posts/comments/likes/favorites | 评论树、点赞收藏分享、举报、管理(mod) |
| /board/:id | 板块内帖子 | L2 | 板块卡 | posts/boards/topics | 话题 chips 筛选 |
| /topic/:id | 话题内帖子 | L2 | 话题行 | posts/topics | — |
| /write | 发帖 | L2 | FAB | posts/boards/topics | 选板块(必填)、话题(可选)、草稿持久化、发布 |
| /user/:id | 用户主页 | L2 | 作者/头像 | users/posts/follows_users | 关注/私信；封禁(admin) |
| /chat/:userId | 私信会话 | L2 | 私信列表/用户主页 | messages | 发送（**id=对方用户 id；首次发起自动建会话**） |
| /profile-edit | 资料编辑 | L2 | 我的资料卡 | users | 保存 |
| /settings | 设置 | L2 | 我的 | users | **主题切换（跟随系统/浅色/深色）**、演示角色切换（原型）、退出登录 |
| /reports | 举报处理 | L2 | 我的·管理入口 | reports/moderation_actions | 处理弹窗→append 审计→结果通知 |
| — | 弹窗层 | overlay | 各页 | reports/follows | 举报原因/处理动作/选择器/确认框 |

**页面→实体映射原则**（#5）：内容页对 content schema（boards/topics/posts/comments/likes/favorites/follows_boards/follows_topics）；消息页对 notify schema（notifications/messages）；治理页对 moderation schema（reports/moderation_actions）；认证/我的对 user schema（users/follows_users）。通知按 `target_type` 跳转锚点（follow→用户主页，like/comment/reply→帖子详情评论锚点，report_result→结果提示）。

## 5. 交互规格

### 5.0 角色权限矩阵【v2 新增】

| 动作 | guest | user | moderator | admin |
|---|---|---|---|---|
| 浏览 feed/板块/帖子详情 | ✓ | ✓ | ✓ | ✓ |
| 点赞/收藏/评论/发帖/关注 | 登录墙 | ✓ | ✓ | ✓ |
| 私信/通知/我的 | 登录墙（原地引导页） | ✓ | ✓ | ✓ |
| 举报（帖子/评论/用户） | 登录墙 | ✓ | ✓ | ✓ |
| 删除自己的帖子/评论 | — | ✓ | ✓ | ✓ |
| 删除他人帖子/评论 | — | — | ✓ | ✓ |
| 置顶/取消置顶 · 精华/取消精华 | — | — | ✓ | ✓ |
| 警告用户 | — | — | ✓ | ✓ |
| 封禁/解封用户 | — | — | — | ✓（解封须记 moderation_actions，见 §10） |
| 处理举报（/reports） | — | — | ✓（动作集不含封禁） | ✓（全动作集） |

实现约束：所有管理操作**前后端双重鉴权**；前端按角色渲染可见性只是体验层，接口必须独立校验。

### 5.1 全局
- **Tab 显隐**：L1 显示 4 Tab；L2 隐藏 Tab；auth 页无 Tab。**FAB 显隐**：L1 显示；L2 中 `/write`、`/post/:id`、`/chat/:userId` 隐藏（避免遮挡底部输入栏），其余 L2 保留；auth 页隐藏。
- **返回**：导航栈基于 History API（hash 路由 pushState + popstate），页面顶部返回键、系统返回键、浏览器后退三者行为一致。
- **登录墙**：游客可看 feed/板块/帖子详情；点赞/评论/发帖/关注/私信 → 跳登录页并记录 returnTo，登录后回原页面，取消登录留在原页；「通知/我的」Tab → 原地登录引导页。
- **接口**：内容接口公开、操作接口鉴权；私信/通知/管理接口必须鉴权。

### 5.2 信息流 / 帖子【v2 修订】
- 分段控件：胶囊式，选中墨蓝底白字，切换即重渲染（state 驱动）。
- **排序规则**：「全部」= 置顶帖优先（按操作时间倒序）+ 其余按发布时间倒序；「关注」= 关注来源（用户/板块/话题）帖子按发布时间倒序；板块/话题内列表同「全部」规则。
- **分页**：每页 20 条，滚动到底加载更多；无更多时显示「到底啦」。
- 帖子卡片：头像/名字/时间 + chips（置顶·精华·板块）→ 标题（≤2 行）→ 摘要（2 行 clamp）→ 操作行（赞数/评论数/收藏/···）。
- 点赞/收藏/关注状态即时翻转并计数（关注流、收藏列表随之变化）。
- **帖子正文**：Markdown 子集——围栏代码块、行内代码、**粗体**、链接；图片渲染**外链 URL + 本地上传 URL**（#12 升级：原「仅外链渲染、本地上传留 fog」）。渲染前先转义 HTML。
- **评论树**：邻接表 + floor 楼层号（仅顶层计数）；回复缩进+左线；**默认全部展开**，单条评论的回复数 >3 时超出部分折叠为「展开 N 条回复」；软删占位显示「评论已删除」（回复链保留）；评论内容走 md 渲染（与正文一致，#12 起图片 URL 可渲染）。
- **通知锚点**：跳转到折叠内评论时，自动展开其祖先链上所有折叠、滚动定位并高亮 1.6s（`accent-soft` 底色渐隐）。

### 5.3 发帖【v2 修订】
- FAB → 全屏写帖页；选板块（必填，bottom sheet）→ 话题（从属板块，自动过滤，可选）；右上角发布。
- **图片上传**（#12）：正文工具栏「图片」按钮 → 选图（相册/拍照）→ 上传成功自动把 URL 插入 markdown 光标处（独占一行），失败 toast；登录态经 `POST /api/v1/uploads`，类型 jpg/png/gif/webp、默认 ≤5MB。
- **草稿持久化**：标题/正文/板块/话题进 localStorage（key 含用户 id，防抖 300ms 写入）；离开页面、PWA 被杀进程后重进 /write 自动恢复并 toast 提示「已恢复草稿」；发布成功或手动清空后清除；7 天过期。图片 URL 为绝对地址，直接存 markdown 文本即可。

### 5.4 通知 / 私信
- 未读左侧红点；「全部已读」清空；点击单条跳锚点（§5.2）。
- **`/chat/:userId` 的 id = 对方用户 id**；从用户主页「私信」进入时若无会话则自动创建空会话。私信轮询即可（§7 无完整 IM）；会话页发送后清空输入、列表滚动到底。

### 5.5 我的 / 用户主页（复用同一 Profile 组件）
- 自己=「我的」：资料卡 + 帖子/收藏/关注 分段 + 编辑资料/设置；他人=「用户主页」：资料卡 + 关注/私信按钮（admin 另有封禁/解封）。
- 治理入口只在 **moderator/admin** 角色可见（「我的」页显示「举报处理」，带待处理数徽章）；管理操作内嵌在对应页面，可见性与动作集严格按 §5.0 矩阵。

### 5.6 治理闭环【v2 修订】
- **举报原因枚举**：垃圾广告 / 违法违规 / 人身攻击 / 抄袭侵权 / 引战灌水 / 其他（选「其他」时备注必填）。
- **防重复语义**：同一举报者对同一目标存在 **status=pending** 的举报时禁止重复提交（DB 部分唯一索引 `UNIQUE(reporter,target,type) WHERE status='pending'`）；已结案（resolved/dismissed）后可再次举报。
- 流程：举报（帖子/评论/用户「···」→ 弹窗选原因）→ moderator/admin 在 /reports 处理 → 处理弹窗动作集（按角色）：**忽略**(dismissed) / **删除内容**(resolved+删除) / **警告**(resolved+警告) / **封禁用户**(resolved+封禁，仅 admin)；所有动作可附备注 → append `moderation_actions` 审计 → 系统发 `report_result` 通知给举报人（含处理结论）。
  > **#34 偏离批注（2026-08-21，用户确认）**：举报处理弹窗**不含「封禁用户」动作**（范围收敛）。封禁/解封能力经 admin 组端点 `POST /moderation/users/:id/ban|unban`（写 `ban_user`/`unban_user` 审计）+ 用户主页封禁/解封入口提供；决策记录 `docs/handoffs/grilling-decisions/issue-34-governance-permissions-decisions.md`。

### 5.7 空状态清单【v2 新增】

| 场景 | 文案 | 引导动作 |
|---|---|---|
| 关注流为空 | 「还没有关注动态」+ 引导语 | 按钮「去看看全部」→ 切分段 |
| 板块内无帖子 | 「这个板块还很安静」 | 按钮「发第一帖」→ /write |
| 话题内无帖子 | 「这个话题还没有讨论」 | 按钮「发起讨论」→ /write |
| 无通知 | 「暂时没有新通知」 | — |
| 无私信 | 「还没有私信会话」 | — |
| 我的帖子为空 | 「还没有发布过帖子」 | 按钮「去发帖」 |
| 我的收藏为空 | 「还没有收藏内容」 | — |
| 我的关注为空 | 「还没有关注任何人/板块/话题」 | — |
| 待处理举报为空 | 「没有待处理的举报」 | — |
| 用户主页无帖 | 「TA 还没有发布过帖子」 | — |

### 5.8 主题切换【v2 新增】
- /settings 提供三态：**跟随系统（默认）/ 浅色 / 深色**；选择写入 localStorage；「跟随系统」时删除 `data-theme`，由 `prefers-color-scheme` 接管（§1.2 三态规则）。

## 6. 技术约束（生成时务必遵守）

1. **单文件、零依赖**：HTML + 内联 CSS/JS，无构建、无 CDN、无外链（CSP 兼容）；图标为内联 SVG。
2. **移动端优先**：390px 基准，≤430px 全屏、桌面套手机框；页面本身**绝不横向滚动**（代码块/宽内容局部 `overflow-x:auto`）。
3. **双主题 token 化**：§1.2 的三态规则（`:root` / `@media` / `[data-theme=dark]`），body 显式背景；**所有 token 含 soft 色变体在深色下全部重定义**。
4. **可交互、状态驱动**：所有分段/点赞/收藏/举报/发帖/已读/管理操作都是真实状态变化，非静态截图；角色可切换以观察可见性（按 §5.0 矩阵）。
5. **安全区与触控**：Tab/FAB/sheet 底部加 `env(safe-area-inset-bottom)`；可点目标 ≥44×44px；文字对比度正文 ≥4.5:1、大字号/图标 ≥3:1。
6. **导航**：hash 路由 + History API，禁止自造与浏览器历史脱节的栈。
7. mock 数据用 AI 社团真实语境（板块 6 个、话题 10 个、帖子 8 篇、评论树含软删、通知 5 条、会话 3 组、举报 3 条（pending 2 + resolved 1）），内容围绕 LLM/RAG/Agent/PyTorch/数模竞赛/工作坊。

## 7. 决策边界（明确不做，防过度设计）【v2 修订】
- 无搜索功能（**连同占位框一起不做**）【定稿裁决：采纳 v2】；无完整 IM（私信轮询即可）；无图片管理（上传后不可删除/管理页，缩略图/配额/频控留后续，见 #12 D5）；无申诉流程（触发条件：出现争议处理或封号投诉时再立项）；无邀请码管理 UI（管理员线下分发，DB 直管）；无重型管理后台（举报处理是最小闭环件）；无 PC 端。

## 8. 参考
- 原型：`docs/ux/ia-prototype.html`（v1 实现，差异见文首注）
- MVP 功能集：`docs/forum-wayfinder-input.md`；数据模型：#5 resolution（14 表/4 schema）
- 前端选型：#3 `docs/research/mobile-frontend-selection.md`（Vue3+Vant PWA）；设计术语：#7 `docs/design-principles.md`

## 9. v1→v2 修订记录

| 类别 | 条目 | 修订 |
|---|---|---|
| 缺失 | 图片上传（#12） | §5.2/§5.3/§7：落地本地图片上传（发帖+评论选择器，URL 插入 markdown；后端 `POST /api/v1/uploads` 双校验+UUID 文件名），升级「仅外链渲染」为「本地上传+外链」 |
| 硬伤 | 深色 token 遗漏 | §1.2：深色重定义全部 token；`line`/`surface-3` 深色取值错开 |
| 硬伤 | 置顶/精华 chips 无来源 | D4+§5.0：补最小管理操作，moderator/admin 在帖子「···」内切换【定稿裁决：采纳】 |
| 硬伤 | 评论折叠 vs 通知锚点冲突 | §5.2：默认全展开、>3 回复折叠；锚点自动展开祖先链+高亮 |
| 硬伤 | 角色权限不清 | §5.0 新增权限矩阵；前后端双重鉴权写入约束 |
| 硬伤 | 导航栈脱离浏览器历史 | D3+§5.1+§6.6：改为 History API（hash+popstate） |
| 硬伤 | FAB 遮挡输入栏 | §5.1：/write、/post/:id、/chat 隐藏 FAB |
| 缺失 | 排序规则 | §5.2：置顶优先+时间倒序 |
| 缺失 | 空状态 | §5.7 空态清单表 |
| 缺失 | 草稿范围 | §5.3：localStorage 持久化、恢复提示、7 天过期 |
| 缺失 | 正文格式 | §5.2：Markdown 子集+外链图片，上传留 fog |
| 缺失 | 主题切换入口 | §5.8：/settings 三态 |
| 缺失 | 邀请码链路 | §4：注明管理员线下分发、一次性、管理 UI 留 fog |
| 缺失 | /chat id 语义 | §5.4：id=对方用户 id，自动建会话 |
| 缺失 | 分页 | §5.2：20 条/页加载更多 |
| 缺失 | 举报原因枚举/防重复语义 | §5.6：六类原因；pending 期部分唯一索引 |
| 小问题 | 「唯一三级」表述 | D3：改为「IA 两级，栈深不限」 |
| 小问题 | 对比度 | §1.2：ink-2 加深、新增 accent-strong、ink-3 限用途 |
| 小问题 | 搜索占位框 | D1+§7：占位框取消【定稿裁决：采纳】 |
| 小问题 | safe-area/触控 | §1.4+§6.5：env() + 44px 写入约束；状态栏注明原型演示件 |

## 10. 遗留标注（评审补充，非 v2 原文）

- **解封的审计动作**【#6 已升级为下游依赖，必须修复】：§5.0 权限矩阵含「封禁/解封用户」，但 #5 `moderation_actions` 动作枚举仅 `delete_post/delete_comment/ban_user/warn`，无 `unban_user`。解封属于管理操作，**必须写入审计**，否则直接违反「治理操作留痕」原则。因此后端落地前，必须在 #5 数据模型（或一份明确的 schema patch）中**把 `unban_user` 加入 `moderation_actions.action` 枚举**，并同步接口层校验；不得以「仅改 `users.status`」的方式绕过审计。详见 `docs/ux/home-post-prototype-resolution.md` 与 #5 后续评论。
- **分页**：50 人规模数据量小，§5.2「每页 20 条加载更多」可简化为前端无限滚动 + 服务端简单分页参数（`limit/offset`），非阻塞项。
- **原型对齐**：✅ 已完成（2026-08-18）——#9 全站原型 `docs/ux/ia-prototype.html` 已裁定采纳；#6 聚焦原型 `docs/ux/home-post-prototype.html` 已产出，见 [#6 resolution comment](https://github.com/li-yongqvan/ai-forum/issues/6#issuecomment-5317256635)。
