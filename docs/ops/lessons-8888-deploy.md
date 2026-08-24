# 8888 部署 · 经验与坑位清单（2026-08-19/20）

> 目的：沉淀本次上线过程中每次卡点的**根因 + 对策 + 可复用验证路径**，供后续部署/运维/CI 修改直接取用。
> 关联：`docs/ops/deployment-v3-8888-container.md`（决策基线）、`docs/ops/acceptance-8888-results.md`（验收）。

## 0. PR 序列一览（本次部署）

| PR | 内容 | 合并时间 | 主要卡点 |
|---|---|---|---|
| #17 | 8888 容器化部署代码落地 | 08-19 02:35 | GitHub API 假 404（MSYS）；合并需授权 |
| #18 | bootstrap pipefail 修复 + lint v7 | 08-19 06:49 | `ssh -T` 退出码怪癖 |
| #19 | 上传 URL 含 :8888 + CI nginx reload | 08-19 08:14 | 验收发现 URL bug；我 scp 污染服务器 repo；评审补 reload 缺口 |
| #20 | golangci-lint 配置迁移 v2 | 08-19 08:56 | 配置 v1/v2 字段混写 |
| #21 | 无评论详情崩溃修复 | 08-19 14:24 | （新会话按 handoff 完成） |
| #22 | 测试数据清理状态更新 | 08-19 15:25 | （新会话） |

## 1. 逐 PR 卡点与根因

### #17 —— 最大的坑：GitHub API 全线 404（不是网络！）

**现象**：`gh api /repos/...`、`gh variable set`、`gh repo view` 等大量 404，一度被误判为「网络波动」。
**根因**：**Git Bash MSYS 路径转换**把 gh 参数里以 `/` 开头的路径改写成了 Windows 路径（`/repos/...` → `C:/Program Files/Git/repos/...`），gh 收到坏路径自然 404。同机 curl 带完整 URL 却能 200 —— 这是区分关键。
**对策**：所有 gh 命令加 `MSYS_NO_PATHCONV=1`；`gh api` 的 POST/PATCH 仍偶发 404 → 直接 `curl` 完整 `https://api.github.com/...` URL 更可靠。
**教训**：**先怀疑本地工具/路径/编码，再归因网络**；用「同 token 不同调用方式对照」定位。
**补充（2026-08-20 修正）**：`gh auth refresh`（用户手动重登）后，`gh issue view/edit` 读路径**恢复正常**——此前残留的「读 404/写 200」实为**鉴权过期叠加**，非纯 API 限制。结论：读到 404 时先查 `gh auth status` + `gh auth refresh`，再走 curl 兜底，勿把「读永远不通」当长期假设。

### #18 —— `ssh -T` 的退出码怪癖

**现象**：bootstrap 首步「GitHub SSH 自检」误报 FATAL。
**根因**：`ssh -T git@github.com` **认证成功也返回 exit 1**（GitHub 不提供 shell，读完问候即断连）；脚本开了 `set -o pipefail`，`ssh | grep` 管道被 ssh 的 exit 1 拖垮。
**对策**：`auth_out=$(ssh -T git@github.com 2>&1 || true)` 先捕获输出再 grep，绕开 pipefail。
**教训**：管道 + `pipefail` 下，命令自身的非零退出会让「语义成功」误判失败。

### #19 —— 上传 URL 缺端口 + 我自己的失误

**卡点 A（验收发现的真 bug）**：nginx `proxy_set_header Host $host` 中 `$host` **剥端口** → api 用 `c.Request.Host` 拼 URL 丢 `:8888` → 图片指向 80 端口 404。**非标准端口部署的典型坑**。对策：`$host` → `$http_host`（保留含端口原始 Host）。

**卡点 B（我的操作失误）**：pre-校验 nginx 配置时**直接把新 web.conf scp 进了服务器 git 仓库** → 产生未提交改动 → 后续 CI 的 `git pull` 被拒，部署假失败。对策：**永远不要用 scp/手工改服务器 repo 内的文件**；验证配置用临时文件或容器只读挂载。
**教训**：服务器上 repo 的改动必须走 git（`git checkout --` 回滚）。

**卡点 C（评审补的执行缺口）**：web.conf 是 bind mount，CI `docker compose up -d` 判定容器 spec 无变化 → 不重建 → **nginx 仍跑旧配置**（「部署成功但修复未生效」假阴性）。对策：部署脚本 `up -d` 后加 `nginx -t && nginx -s reload`（`-t` 先 fail fast）。
**教训**：bind mount 的配置文件变更必须显式 reload；`nginx -T` 输出的是磁盘配置不是运行配置。

### #20 —— golangci-lint v1/v2 配置混写

**现象**：`golangci-lint-action@v7`（golangci-lint 2.12.2）`config verify` 报 `issues.exclude-rules` 非法。
**根因**：`backend/.golangci.yml` 头写 `version: "2"` 却用 v1 字段。v2 迁移：`issues.exclude-rules` → `linters.exclusions.rules`。
**对策**：迁移配置；**本地用与 CI 同版本 Docker 镜像预验证**（`docker run golangci/golangci-lint:v2.12.2 golangci-lint config verify`），避免红 CI 才知道。
**教训**：配置声明版本与实际字段必须一致；升级工具链前先本地验证。

## 2. 横切经验（每轮都踩/都要用）

1. **GitHub API 从本机间歇 404**（可能被墙）→ 多路验证：服务器侧真值（git HEAD / DB / 磁盘文件）优先；GHCR 匿名 `ghcr.io/token` 流可作替代验证（查镜像 tag 证明 CI 构建步执行）。
2. **Windows CRLF/LF**：要 scp 到服务器由 bash/nginx 执行的文件必须 LF（`.gitattributes` 已强制 `*.sh`/`web.conf`）。检测用 `tr -cd '\r' | wc -c`（`grep -c $"\r"` 不可靠）。
3. **合并/删生产数据等动作必须用户显式授权**（auto mode 分类器会拦）。
4. **地图 issue #1 按里程碑更新**，别攒到收尾。
5. **CI run 状态优先看服务器落地效果**（git pull 到的 commit、容器重启时间、上传 URL）而不是 GitHub API。
6. 遇到疑似网络问题：**最多重试 4 次、换验证方式、先排除本地因素（MSYS/路径/编码/配置）再归因网络**。
7. **gh `--body-file` 路径坑（#34/#47 两次踩到）**：Git Bash 的 `/tmp/xxx` 路径 gh 不识别（被当字面量 → 文件找不到）；`--body '...'` 内联又会被 bash 解释反引号/`$`/换行。可靠做法：正文先 Write 成文件，再 `--body-file` 用 **仓库内相对路径**（如 `docs/xxx.md`）或 **Windows 绝对路径 + `MSYS_NO_PATHCONV=1`**（如 `C:/Users/.../xxx.md`）。

## 3. 可复用验证命令速查

```bash
# GitHub API（本机）——gh 一律加 MSYS_NO_PATHCONV=1；写操作用 curl 完整 URL
export MSYS_NO_PATHCONV=1
gh api /repos/li-yongqvan/ai-forum/...        # 读
curl -s -H "Authorization: Bearer $TOKEN" https://api.github.com/repos/.../...  # 写/查

# 服务器侧真值
ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git rev-parse --short HEAD'   # CI 部署是否拉到
ssh liyongquan@122.51.233.225 'cd ~/ai-forum && docker compose ps'            # 容器健康
ssh liyongquan@122.51.233.225 'cd ~/ai-forum && docker compose exec -T web nginx -t'  # 配置校验

# GHCR 匿名验证（无需 GitHub API）
TOK=$(curl -s "https://ghcr.io/token?scope=repository:li-yongqvan/ai-forum-api:pull&service=ghcr.io" | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
curl -s -H "Authorization: Bearer $TOK" https://ghcr.io/v2/li-yongqvan/ai-forum-api/tags/list

# 本地验证 golangci 配置（同 CI 版本）
export MSYS_NO_PATHCONV=1
docker run --rm -v "C:/Users/liyongquan/ai-forum/backend:/repo" -w /repo golangci/golangci-lint:v2.12.2 golangci-lint config verify
```

## 4. #43 服务器本地构建（2026-08-24 新增坑位）

1. **proxy.golang.org 服务器不可达（实测超时）** → `go mod download` 必失败。对策：Dockerfile `ARG GOPROXY` + 服务器 build `--build-arg GOPROXY=https://goproxy.cn,direct`（实测 200/0.06s）。CI（runner）可访问默认 proxy.golang.org，默认不覆盖。
2. **服务器无 golang/alpine builder 镜像（Docker Hub 被墙）** → merge 前一次性预灌 `golang:1.26-alpine` + `alpine:3.21` + `docker/dockerfile:1`（BuildKit frontend，Dockerfile:1 `# syntax=` 需要）。本机（中国）可能也拉不到 Docker Hub，走镜像源 `docker pull m.daocloud.io/docker.io/library/golang:1.26-alpine` 再 retag。
3. **前端换装 bind-mount inode 坑** → web 容器挂 `frontend/dist` 的 inode，`rm -rf dist && mv` 换目录后容器仍挂旧 inode、新文件永不生效。对策：`find dist -mindepth 1 -delete`（清内容保目录）+ staging `cp -a` 原地铺新（`deploy/deploy-server.sh`）。
4. **`set -e` 下 `cmd && break` 误杀** → 健康轮询循环里 `[ "$st" = healthy ] && break` 在非 healthy 时返回非零 → set -e 整脚本退出。对策：if/fi + `|| echo none` 兜底。
5. **`docker compose exec` 无 TTY** → 脚本/CI 非交互上下文统一 `-T`（appleboy/ssh-action 分配 PTY 时现版不带 -T 也能跑，但 -T 两边都安全）。
6. **compose 加 `build:` 段后 `pull api` 语义变** → 拉 GHCR 过期 `:latest`；裸 `up -d` 缺镜像时用默认 GOPROXY 构建失败。对策：bootstrap/部署显式 `docker compose build --build-arg ...`。
7. **回退保险 tag 时机** → `docker tag :latest :rollback` 必须在 build **之前**（build 成功覆盖 :latest 后旧镜像变 dangling 被 prune，无回退源）。
