# Handoff · ai-forum 两个线上 bug 修复

> 目的：新会话接手修两个已定位的 bug。**根因已查清、方案已定稿**，新会话按 §2/§3 直接实现，无需重新探查。
> 生成：2026-08-19。使用 matt 技能库 `handoff` 技能格式。

## 1. 新会话要做什么

修复两个线上问题并走完整流程（PR → 用户授权合并 → CI 部署 → 验证 → 更新地图 #1）：

1. **前端崩溃 bug**（真 bug，影响所有无评论帖子查看详情）：`Cannot read properties of null (reading 'reduce')`
2. **测试数据乱码清理**（我的历史测试数据污染，非应用 bug）：清理 `[smoke]` 测试帖子/评论 + 测试账号

## 2. Bug A：无评论帖子查看详情崩溃

**现象**：任何**没有评论**的帖子打开详情页，前端报 `Cannot read properties of null (reading 'reduce')`。用户发图后查看新帖即触发。

**根因（已实锤）**：
- 后端 `GET /api/v1/posts/:id/comments` 空评论树返回 `{"post_id":X,"comments":null}`（非空时是数组）→ 契约不一致。
- 前端 `frontend/src/views/PostDetail.vue:40`：`commentCount.value = countNodes(t.comments)`，`countNodes`（L32-34）对入参直接 `.reduce(...)` → `null.reduce()` 崩溃。

**修复方案（两处都做，契约 + 防御）**：
1. **后端**：`GetCommentTree`（handler 在 `backend/internal/httpapi/handler/content.go:237` 调用 `h.svc.GetCommentTree`）空树返回 `[]` 而非 `nil`，保证 `comments` 恒为数组。具体实现点在 content 服务层，需定位后改。
2. **前端**：`PostDetail.vue:40` 改 `countNodes(t.comments ?? [])`（防御一行）。同时检查 `CommentTree.vue` 组件对 `tree.value`（可能被设为 null）的渲染是否也需要 `?? []` 兜底。

**验证**：curl `GET /api/v1/posts/5/comments`（无评论帖）应返回 `"comments":[]`；浏览器打开无评论帖子详情不再报错。建议补一个前端单测（PostDetail 无评论场景）防回归（`tdd` 技能）。

## 3. Bug B：清理乱码测试数据

**现象**：测试账号发的中文帖子在论坛显示乱码（`�`）。

**根因**：验收阶段我用 Git Bash `curl -d '...中文...'` 发帖，Windows 按 **GBK 编码**发送 → 字节进库不可逆损坏（U+FFFD）。DB hex 实锤：**帖子 id 1、3、4**（`[smoke]` 前缀）+ 帖子 1 上的**评论 id 1** 已损坏、无法恢复。**非应用 bug**（python 发的 id 2 和用户自己发的 id 5 `人工测试` 都是干净 UTF-8）。

**方案**：删生产库中这些测试数据（需用户确认才执行）：
- 帖子：`"content".posts` 中 id 1、3、4（`[smoke]` 开头）及关联 likes/favorites/follows
- 评论：id 1（及关联）
- 测试用户：`smoketest_user`(id=1)、`smoke_qa_153000`(id=2)（及其发的内容/关注关系）
- 已消耗邀请码：`SMOKETEST2026`、`SMOKEQA153000` 等（可留作审计，不必删）
- 上传文件：服务器 `~/ai-forum/uploads/` 下的测试图（`.smoketest` 相关上传，可留可清）

> 注意：删生产数据前**先跟用户确认删除清单**。数据库操作用服务器 `docker compose exec -T postgres psql -U forum forum`。schema 是 `"content"`、`"user"` 等。

## 4. 当前部署与协作约定（新会话必读）

- **线上**：`http://122.51.233.225:8888/`（容器 nginx @8888 → api → postgres，共享服务器无 sudo）。`DEPLOY_ENABLED=true`，CI push main 自动 构建→scp→`up -d`→nginx reload。服务器 repo `~/ai-forum`（read-only deploy key 拉取）。
- **git 流程**：分支 → PR → **用户明确授权后合并**（`gh pr merge N --merge`）→ CI 部署 → 服务器侧验证。参考 PR #17-20 流程。
- **地图 #1 必须按里程碑更新**（GitHub issue `li-yongqvan/ai-forum#1`，`gh issue edit 1`），修完每个 bug 同步记一笔。
- **本机 GitHub API 间歇 404**：gh 命令一律 `MSYS_NO_PATHCONV=1`；疑似网络问题**最多重试 4 次、换验证方式、先排除本地因素**再归因网络；服务器侧真值（git HEAD / DB / 磁盘）优先。

## 5. Suggested skills

- `diagnosing-bugs` — 若修复后仍有异常，用它走诊断循环
- `tdd` — 给 reduce 修复补回归测试（前端无评论场景）
- `code-review` — 合并前对 PR 做一轮自审
- `grilling` — 如需用户对「删生产测试数据清单」做最终确认决策

## 6. Artifacts（引用，不重复内容）

- 验收结果（含测试数据清单、smoke 账号）：`docs/ops/acceptance-8888-results.md`
- 上传 URL 修复评审：`docs/ops/review-upload-url-fix.md`
- 部署决策基线 v3.1：`docs/ops/deployment-v3-8888-container.md`
- 线上状态地图：GitHub issue #1
- 上一轮 handoff（执行记录）：`docs/handoffs/deploy-8888-container-exec.md`
- 相关代码：`frontend/src/views/PostDetail.vue`、`backend/internal/httpapi/handler/content.go`（GetComments/GetCommentTree）

## 7. 安全注意（脱敏说明）

- **本会话曾把 gh token 打印到过输出**（`gho_…`，用户机器上，可能已残留终端历史）——建议用户后续轮换该 token（`gh auth refresh`）。本文档不含任何 token/密码。
- 测试账号密码（如 `Smoketest123!`）不在本文档列出；新会话如需要向用户索要。
- 服务器 `.env`（POSTGRES_PASSWORD/JWT_SECRET）只存在于服务器，勿外泄、勿进 git。
