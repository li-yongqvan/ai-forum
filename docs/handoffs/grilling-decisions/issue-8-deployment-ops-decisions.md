# Grilling #8 部署与运维方案 — 决策记录

- **Ticket**: [#8 部署与运维方案](https://github.com/li-yongqvan/ai-forum/issues/8)
- **类型**: grilling / HITL
- **日期**: 2026-08-18
- **参与**: Claude + @li-yongqvan
- **产出**: `docs/ops/deployment.md`

---

## 目标

确定 AI 智联论坛 MVP 的部署与运维最小可行方案，使技术路线最后一块未决项落地。

---

## 决策清单

### D1 部署形态

- **问题**: 用什么方式跑整个论坛？
- **选项**: Docker Compose all-in-one / 二进制+systemd / Kubernetes / 手动分发
- **决定**: **Docker Compose all-in-one**
- **理由**: 与 #2/#4 一致；运维心智最少；用户一个空服务器即可 `docker compose up`。

### D2 服务器结构

- **问题**: 单台服务器上各组件怎么摆？
- **选项**: 复用现有 nginx / 引入 Caddy 容器 / nginx 反代到 Caddy
- **决定**: **复用服务器现有 nginx**
- **理由**: 目标服务器 `122.51.233.225` 已预装 nginx 并占用 80 端口；避免迁移风险和额外维护面。Go API 跑 `:8080`，PostgreSQL 容器化，静态文件由 nginx 直接托管。
- **与 #2 的关系**: #2 推荐 Caddy 针对 greenfield 服务器；本决策是「已装 nginx」的条件分支，非否定 #2。已回改 `docs/research/go-backend-framework-selection.md` 加条件脚注。

### D3 数据库 / 缓存 / 消息队列

- **问题**: PostgreSQL 怎么跑？要不要 Redis / 消息队列？
- **选项**: PG 容器化+无 Redis / PG 宿主机安装 / 引入 Redis / 引入消息队列
- **决定**: **PostgreSQL 容器化；不引入 Redis / 消息队列**
- **理由**: 与 #4 D4/D5 一致；异步用进程内 goroutine；MVP 不建抽象层。

### D4 反向代理 + HTTPS

- **问题**: 用什么域名和证书方案？
- **选项**: 已有域名 / 买域名 / 无域名+Cloudflare Tunnel / 无域名+自签名 / 无域名+纯 HTTP
- **决定**: **无域名，纯 HTTP on `http://122.51.233.225/`**
- **理由**: 用户明确「现在不用考虑安全问题，整个不是重点」。
- **技术债**: 论坛有登录/发帖，纯 HTTP 明文传输；后续公测或真实用户前必须买域名+HTTPS。

### D5 CI/CD

- **问题**: MVP 代码更新怎么部署？
- **选项**: Actions 构建+watchtower / Actions SSH 直连 / Actions 仅构建+手动部署 / 纯手动
- **决定**: **GitHub Actions SSH 直连服务器执行 `docker compose pull && up -d`**
- **理由**: 推代码即部署，方便；用户接受把 SSH 私钥存 GitHub Secrets 的敏感权衡。

### D6 日志 / 监控 / 备份

- **问题**: 最小可行的可观测与备份方案？
- **决定**:
  - 日志: stdout/stderr + Docker json-file
  - 监控: `/healthz` + 免费外部监控或 cron curl
  - 备份: 宿主机 cron + `pg_dump`，保留 7 天
- **理由**: 不做 Prometheus/Grafana/Loki/Sentry，MVP 阶段过度。

### D7 产出形态

- **问题**: 决议和可执行说明以什么形式交付？
- **选项**: 单份 `docs/ops/deployment.md` / 分散文档 / 直接写代码 / 只写 issue comment
- **决定**: **`docs/ops/deployment.md`**
- **理由**: 集中、易维护、与现有 `docs/research/` 风格一致。

---

## 关键讨论点

### 1. 选择 nginx 而放弃 Caddy 的不一致性如何解决？

- **问题**: #2 推荐 Caddy，选 nginx 会造成研究结论与部署方案不一致。
- **解决方案**:
  1. #8 决议中显式写 override rationale；
  2. 回改 `docs/research/go-backend-framework-selection.md` 加「已装 nginx 的服务器」条件分支；
  3. 部署文档用 nginx 配置而非 Caddyfile。
- **结论**: 不一致被记录为「条件分支」而非简单推翻。

### 2. 无域名纯 HTTP 的安全债

- 用户明确当前不考虑安全，MVP 接受此债。
- 记录为后续迭代首项：购买域名 + HTTPS（nginx+certbot 或迁移 Caddy）。

---

## 后续行动

- [x] 产出 `docs/ops/deployment.md`
- [x] 更新 `docs/research/go-backend-framework-selection.md`
- [x] 关闭 issue #8
- [x] 同步更新 wayfinder 地图 issue #1
- [ ] 仓库初始化代码后，将占位符替换为真实镜像名/路径
- [ ] 服务器上实际执行首次部署
- [ ] 购买域名并启用 HTTPS（后续迭代）
