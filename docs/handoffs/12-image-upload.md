# Handoff · #12 图片上传（MVP 本地上传）

> 供新会话接手实现。决策只依赖本文件 + issue #12 + grilling-decisions 归档 + 地图 #1，不依赖会话记忆。
> 产出日期 2026-08-18。

## 1. Goal of next session

**产出目标**：按方案 A 实现 #12 —— 后端本地图片上传接口 + 前端图片选择器，发帖/评论可插入图片；测试跑通并同步 IA §7。

**Prompts to answer**（实现前需与用户确认或取得倾向）：
1. 大小上限：默认 5MB？是否进 config 可配？
2. 类型校验：扩展名白名单 vs MIME sniff（建议双校验防伪造扩展名）。
3. 存储目录：生产 `/opt/ai-forum/uploads`（#8 约定）；开发/测试路径怎么定？
4. 是否限制单用户上传频率/配额（MVP 建议不做）。

**Definition of Done**：
- 后端 `POST /api/v1/uploads`（需登录）：multipart 校验 → 磁盘存储 → 返回可访问 URL。
- 前端发帖/评论图片选择器：上传成功后把 URL 插入 markdown 光标处；上传中/失败态。
- `go test ./...` + Vitest + Playwright 冒烟通过。
- 更新 `docs/ux/info-architecture.md` §7（仅外链 → 本地上传 + 外链）。
- 更新地图 #1 frontier 状态 → 关闭 issue #12；本 handoff 文档删除、INDEX 移到已解决。

## 2. State of play

**Done（已交付，新会话无需重做）**：
- Ticket 落地：issue #12、地图 #1 frontier 条目、决策归档 `docs/handoffs/grilling-decisions/issue-12-image-upload-decisions.md`（D1 范围 A / D2 独立 issue / D3 编辑 body）。
- 后端基线（`feat/backend-scaffold`，PR #11 打开）：auth + content 完整，测试通过、live curl 验证。**上传接口挂 `router.go` 的 `authed` 组**（`middleware.Auth` 已就绪，加一行 `authed.POST("/uploads", …)` 即可）。
- 前端基线：12 页 + 双主题 + PWA + 登录墙（Vant 4.10，`van-uploader` 可用）。md 渲染器 `frontend/src/utils/md.ts` 已支持外链图 + 协议白名单——上传返回的 URL 渲染**无需改动**。

**In progress**：无。

**Blocking**：无硬阻塞。依赖 #8 部署约定（nginx 托管静态 + `/api` 反代）与 PR #11 基线。

**环境**：本机后端 `docker compose up` 跑 8080；前端 `npm run dev` 跑 5173（Vite proxy `/api`→8080）。新会话需自行启动后台任务。

## 3. Open decisions

### D1 存储路径与目录
- 生产 `/opt/ai-forum/uploads`（#8 部署约定），nginx 托管 `location /uploads/`（图片 GET 不经 Go）。
- 开发/测试：本地临时目录（测试用 `t.TempDir()`），路径进 `internal/config`。
- **倾向**: config 提供 `UploadsDir`，默认 `/opt/ai-forum/uploads`；测试注入临时目录。

### D2 校验（大小 / 类型）
- 大小上限：建议 5MB，config 可配。
- 类型：扩展名白名单（jpg/png/gif/webp）+ MIME sniff（`http.DetectContentType`）**双校验**，防伪造扩展名。
- **倾向**: 双校验 + 5MB。

### D3 文件名与防穿越
- 服务端生成 `UUID + 白名单扩展名`，不信任用户文件名；拒绝路径穿越字符。
- **倾向**: UUID 文件名。

### D4 前端交互
- Vant `van-uploader` 或自定义：选择 → 上传 → 返回 URL → 插入 markdown 光标处；上传中/失败 toast。
- **注意**：`frontend/src/api/client.ts` 的 `request()` 只处理 JSON（`JSON.stringify` body + JSON `Content-Type`），**multipart 需新增独立函数**（`FormData` + Bearer，不设 `Content-Type` 由浏览器带 boundary）。
- 草稿（localStorage）与图片 URL 兼容：URL 是绝对路径，存 markdown 文本即可。

### D5 范围边界
- 不做：缩略图、图片删除/管理页、配额限制、额外鉴权。
- 与 #5 数据模型无关：图片是磁盘文件，post/comment 的 markdown 直接引用 URL，**无需新表/迁移**。

## 4. Skills to use (next session)

- `codebase-design` —— 上传落点定位（#7 深模块：独立 upload 模块 vs content 包内命令，按包边界决定，倾向独立小模块或 content 命令）。
- `tdd` —— 后端接口测试（沿用 `handler/*_test.go` 模式）+ 前端上传组件测试。
- `webapp-testing` / Playwright —— 上传 → 发帖 → 详情渲染图片冒烟。
- `grilling` —— D1/D2/D4 若需用户拍板（决策先行，一次一个问题）。
- GitHub 操作统一用 `gh` CLI；**MCP github 对本仓库不可依赖**（此前返回 Not Found）。

## 5. Artifacts (reference only)

- Issue #12：[图片上传（MVP 本地上传）](https://github.com/li-yongqvan/ai-forum/issues/12)
- 决策归档：`docs/handoffs/grilling-decisions/issue-12-image-upload-decisions.md`
- 地图 #1：[Wayfinder 地图](https://github.com/li-yongqvan/ai-forum/issues/1)（frontier 条目）
- 后端路由/鉴权：`backend/internal/httpapi/router.go`、`backend/internal/httpapi/middleware/auth.go`
- 前端 API 客户端（multipart 需新增函数）：`frontend/src/api/client.ts`
- 发帖编辑器（图片选择器落点）：`frontend/src/views/Write.vue`
- 评论输入框（底部评论栏）：`frontend/src/views/PostDetail.vue`
- md 渲染器（外链图已支持）：`frontend/src/utils/md.ts`
- 部署方案（nginx 托管静态 + `/api` 反代）：`docs/ops/deployment.md`
- IA v2（§7 待升级）：`docs/ux/info-architecture.md`
- 基线 PR：#11（`feat/backend-scaffold`）
- Handoff 索引：`docs/handoffs/INDEX.md`
