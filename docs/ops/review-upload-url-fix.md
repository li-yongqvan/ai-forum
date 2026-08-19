# 评审请求：上传图片 URL 缺端口（8888 部署）修复方案

> 目的：请独立评审本修复方案的**正确性、副作用、与备选方案的取舍**。
> 文档自包含：含部署架构、bug 证据、根因、代码上下文、修复方案、备选方案与待评审问题。

---

## 1. 背景与部署架构

- 产品：ai-forum（Vue3+Vite PWA 前端 / Go+Gin 后端 / PostgreSQL 16），私有仓库，社团内部论坛 MVP。
- 部署：共享阿里云服务器 `122.51.233.225`（无 sudo），Docker Compose all-in-one，**容器 nginx 反代 @8888**（`web` 容器 `nginx:1.27-alpine` 监听 `:8888` → `proxy_pass http://api:8080`）。api 仅回环 `127.0.0.1:8080`。
- 前端 API 基址为相对路径 `/api/v1`，同源反代；静态 `/uploads/` 由 web 容器 nginx `alias` 直托（图片 GET 不经 Go）。
- 当前线上状态：三容器 healthy，`/healthz`、PWA、登录/发帖/评论/点赞/收藏/关注、上传负例、限流均已验收通过（21/22），**唯一未过项即本 bug**。

## 2. Bug 报告

**症状**：`POST /api/v1/uploads`（登录后，multipart 字段 `file`）返回：
```json
{"url":"http://122.51.233.225/uploads/67fc02ec6613ecbcd75504aac690b802.png"}
```
URL **缺少 `:8888` 端口**，指向 80 端口（宿主 nginx 默认站，非本应用）。

**影响**：前端 `ImagePicker.vue` 直接 `emit('uploaded', res.url)`，把该绝对 URL 写入帖子/评论的 markdown 图片语法。浏览器渲染时请求 `http://122.51.233.225/uploads/…`（80 端口）→ 404。**发图链路核心展示环节不可用**。

**复现**：任意登录态用户上传一张合法 png/jpg 即可复现。

## 3. 根因分析（含代码引用）

后端拼 URL 逻辑 `backend/internal/httpapi/handler/upload.go:77-83`：
```go
// publicUploadURL 把相对路径拼成当前请求可达的绝对 URL（#12：md 渲染器只渲染 http(s)
// 绝对地址；开发经 Vite proxy、生产经 nginx 同源托管 /uploads，Host 由反代透传）。
func publicUploadURL(c *gin.Context, rel string) string {
	scheme := "http"
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + rel
}
```
即 URL 的 host 部分完全取自 `c.Request.Host`（api 容器收到的 Host 请求头）。

`deploy/web.conf`（web 容器 nginx，当前线上版本）的 `/api/` 代理块：
```nginx
location /api/ {
    proxy_pass http://api:8080;
    proxy_set_header Host $host;            # ← 疑点
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```
nginx 变量语义：**`$host` = 请求 Host 的 hostname 部分（端口被剥离，且小写）**；`$http_host` = 原始 Host 头原样（含端口）。

**因果链**：客户端请求 `122.51.233.225:8888` → nginx `proxy_set_header Host $host` 把 Host 改写为 `122.51.233.225`（丢端口）→ api 收到 `c.Request.Host = "122.51.233.225"` → `publicUploadURL` 拼出无端口 URL。

注意 `upload.go` 注释明确设计意图是「Host 由反代透传」，即 api 假设收到的是**可被客户端直接访问的原始 Host**。`$host` 在 80/443 标准端口下无感知（URL 可省略默认端口），但在 **8888 非标准端口部署下即暴露**。

## 4. 我的修复方案（主推）

**改动**：`deploy/web.conf` 中所有代理到 api 的块，`proxy_set_header Host $host;` → `proxy_set_header Host $http_host;`（共 3 处：`/api/`、`= /api/v1/auth/login`、`= /api/v1/auth/register`）。

- 效果：api 收到 `Host = 122.51.233.225:8888` → 返回 URL 含端口 → 前端图片可正常加载。
- 改动面：1 个 nginx 配置文件，无后端/前端改动，无新配置键。
- 应用方式：web.conf 是 bind mount（`./deploy/web.conf:/etc/nginx/conf.d/default.conf:ro`），文件更新后需 `nginx -s reload`（或 `docker compose restart web`）使配置生效。

**备选方案（供评审权衡）**：

| 方案 | 做法 | 优点 | 缺点 |
|---|---|---|---|
| **A. nginx `$http_host`（主推）** | web.conf 改 Host 头 | 一行配置；符合「Host 由反代透传」设计意图；无代码改动 | 信任客户端 Host（见 §5 安全讨论） |
| **B. 应用层 PUBLIC_BASE_URL** | 新增 `PUBLIC_BASE_URL=http://122.51.233.225:8888` 配置键，`publicUploadURL` 优先用它 | 显式、不依赖 Host 头；防 Host 注入；多环境清晰 | 新增配置键需进 `.env`/`.env.example`；与「Host 透传」设计分叉 |
| **C. X-Forwarded-Host/Port** | web.conf 加 `proxy_set_header X-Forwarded-Host $http_host;`（+Port），后端读取 | 符合代理转发惯例，可顺带处理 scheme | 需同步改 Go 代码；当前后端只认 `X-Forwarded-Proto` |
| **D. 前端改相对路径** | 上传返回相对路径，前端拼绝对 URL | 无后端/代理改动 | 改渲染层（md 渲染器注释要求绝对 http(s) URL）；面更大 |

我倾向 A：最小改动、贴合现有设计意图。但若评审认为 Host 头信任是安全红线，则 B 更稳（代价是加配置键）。

## 5. 需要评审重点评估的问题

1. **正确性**：`$http_host` 是否确实能修复（Host 含端口透传）？有无其它代理头（如 `X-Forwarded-Host`/`X-Forwarded-Proto`）遗漏导致 URL 仍不正确？
2. **副作用**：`Host` 头从 `$host`（剥端口）改为 `$http_host`（含端口），对后端**其它依赖 Host 的逻辑**有无影响？（api 是 Gin，路由不依赖 Host；可评审有无中间件按 Host 分流/鉴权/防 Host 注入。）
3. **安全**：api 用 `c.Request.Host` 拼对外 URL，本质是信任客户端可控的 Host 头。方案 A 是否引入新的 Host 头注入/钓鱼面？是否应改为服务端白名单/显式配置（方案 B）？在「共享服务器 + 公网可达 8888 + 开放注册」场景下的风险评级？
4. **一致性**：`/healthz` 块当前**没有**任何 `proxy_set_header`（nginx 默认行为），是否应顺带统一？是否影响什么？
5. **运维**：bind mount 的 web.conf 更新后，`nginx -s reload` vs `docker compose restart web` 的取舍；CI 部署脚本（`docker compose up -d`）不会自动 reload nginx，是否需要把 reload 加进部署脚本？
6. **回归**：修复后 80/443 标准端口部署是否不受影响（`$http_host` 在标准端口同样正确）？开发环境（Vite proxy）路径是否有影响？

## 6. 验证计划（修复后）

1. 改 web.conf → reload nginx → 登录态重传一张图片，断言返回 URL 为 `http://122.51.233.225:8888/uploads/<file>`；
2. `curl -I <返回URL>` → 200 且 content-type 为图片；
3. 回归：`/healthz` 200、登录/发帖正常；
4. 浏览器人工确认帖子内图片渲染。

## 7. 相关代码路径（评审用）

- `backend/internal/httpapi/handler/upload.go`（`Upload` / `publicUploadURL`，L20-90）
- `backend/internal/config/config.go`（当前无 public base URL 键，L35-46）
- `deploy/web.conf`（nginx 反代配置，全量见仓库）
- `frontend/src/api/upload.ts`（`fetch(BASE + '/uploads', …)`，返回 `res.url`）
- `frontend/src/components/ImagePicker.vue:35`（`emit('uploaded', res.url)`）
- `compose.yml`（web 容器 bind mount web.conf、api `env_file .env`）

## 8. 当前线上/仓库状态（评审需知的基线）

- PR #18 已合 main（bootstrap pipefail 修复 + golangci-lint-action v7）；`DEPLOY_ENABLED=true`；CI 全链路部署已验证跑通。
- 本 bug 的修复尚未提交/合入；`deploy/web.conf` 当前仍是 `$host` 版本。
- 验收留痕：`docs/ops/acceptance-8888-results.md`。
