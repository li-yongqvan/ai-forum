# 测试策略约定

> 适用范围：AI 智联论坛 MVP（ai-forum）
> 依据：#7 深模块设计规范、#4 服务拆分、#8 CI/CD 方案
> 状态：已确认（2026-08-18），生效为测试实现约定

---

## 1. 总体目标

MVP 测试以「保护核心行为不被破坏」为核心，不求 100% 覆盖率，但求：

1. 每个 service/包的深接口有测试穿过 seam（#7 原则）。
2. 关键 REST 端点（登录、发帖、评论、点赞、关注、通知）有集成测试。
3. 前端至少有一条主流程 E2E 冒烟测试。
4. CI 部署前必须测试通过。

## 2. Go 后端测试

### 2.1 单元测试：service 层（重点）

- 覆盖范围：每个包的 `service` 层命令与查询，尤其是：
  - `user`: Register / Login / Follow / Ban
  - `content`: CreatePost / CreateComment / Like / Favorite / GetPost / ListFeed
  - `notify`: CreateNotification / MarkRead / SendMessage
  - `moderation`: CreateReport / HandleReport
- 测试内容：成功路径 + 主要失败路径（参数非法、权限不足、重复操作、软删对象）。
- 依赖隔离：
  - repo 层通过 interface 注入，测试使用 **in-memory fake** 或 **测试数据库**。
  - 测试数据库：**testcontainers-go 起真实 PostgreSQL**（已确认）；CI 在 ubuntu-latest 上运行（自带 Docker），本机测试需 Docker。
- 工具：`testify/assert`、`testify/require`、`stretchr/testify/mock`（若用 fake 则不需要 mock 库）。

### 2.2 集成测试：HTTP handler 层

- 覆盖关键端点：
  - `POST /api/auth/register` / `POST /api/auth/login`
  - `POST /api/posts` / `GET /api/posts/:id`
  - `POST /api/comments`
  - `POST /api/likes` / `POST /api/favorites`
  - `POST /api/follows`
- 每个测试：启动完整 Gin 应用（测试配置），连接测试数据库，每个测试独立事务并回滚。
- 工具：`net/http/httptest` + GORM + 测试数据库。

### 2.3 覆盖率目标

- service 层核心模块 ≥ 70%（已确认）。
- handler 层关键端点覆盖即可，不强制百分比。
- repo 层不测（通过集成测试间接覆盖），除非遇到复杂 SQL。

### 2.4 与深模块设计的关系（#7）

- 测试只跨 seam，不测实现内部。
- 每个 Module 的 Interface 必须有测试验证其深度。
- 写测试时执行「删除测试」：如果某模块删掉后测试复杂度消失，则该模块是浅的。
- 单 Adapter 警示：如果测试中大量 mock 私有实现，说明 seam 位置不对。

## 3. 前端测试

### 3.1 单元测试

- 工具：Vitest（与 Vite 同构）。
- 范围：工具函数、composables、store actions；不强制覆盖 Vue 组件。
- 理由：UI 交互已在原型阶段通过 `docs/ux/home-post-prototype.html` 验证，组件级单元测试 ROI 低。

### 3.2 E2E 冒烟测试

- 工具：**Playwright**（已确认）。
- 至少 1 条主流程：
  - 游客浏览 feed → 点击帖子进入详情 → 触发登录 → 登录成功返回原帖 → 发表评论。
- 环境：E2E 需要真实后端 + 测试数据库，在 CI 中单独起一个服务矩阵。
- **E2E 不加入 MVP CI**（已确认）：搭建成本高，保留 `e2e/` 目录供本地跑，项目稳定前补齐。

## 4. CI/CD 集成（#8）

- GitHub Actions 工作流：
  1. `go test ./...` 必须通过。
  2. `go build ./...` 必须成功。
  3. `npm ci && npm run test:unit`（Vitest）建议通过。
  4. 部署 gate：以上通过后，才 SSH 到服务器执行 `docker compose pull && up -d`。
- **golangci-lint 作为非阻塞检查加入 CI**（已确认）：与测试并行跑，失败不阻断部署，仅提示。
- E2E 不加入 MVP 阶段的 CI（避免拖慢），但保留 `e2e/` 目录供本地跑。

## 5. 测试数据

- Go 测试：使用 fixture factory（如每个测试生成随机用户名/邮箱），避免依赖固定 mock 数据文件。
- 前端单元测试：简单 mock 数据即可。
- E2E：使用预置测试账号 + 初始化 SQL fixture。

## 6. 不写测试的模块

- 纯 DTO/VO 转换函数（如 `PostView` 拼装）：除非含复杂逻辑。
- 第三方 adapter（如 GORM repo 的 CRUD 透传）：通过集成测试覆盖。
- 配置读取、main 函数：MVP 阶段不测。

## 7. 确认记录（2026-08-18）

| 决策点 | 结论 |
|---|---|
| service 层覆盖率目标 | **≥ 70%**（核心模块） |
| 测试数据库 | **testcontainers-go 真实 PostgreSQL** |
| 前端 E2E 工具 | **Playwright** |
| E2E 加入 MVP CI | **否**（保留 `e2e/` 目录本地跑） |
| golangci-lint | **加入 CI，非阻塞**（并行跑，失败不阻断部署） |

以上决策由用户于 2026-08-18 确认/采纳；实现阶段以本文件为准。待确认清单已清空。
