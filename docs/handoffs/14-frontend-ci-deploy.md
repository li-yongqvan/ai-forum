# Handoff · 前端产物自动部署进 CI（阻断项 2）

> ticket：[#14](https://github.com/li-yongqvan/ai-forum/issues/14)（2026-08-18 建票）
> 状态：已解决（2026-08-18 实现完成并合入：deploy.yml 前端构建 + scp 上传 + deployment.md 同步 + actionlint/前端 build 校验通过；PR [#15](https://github.com/li-yongqvan/ai-forum/pull/15) 已合入 main，CI 全绿；配 SSH secrets + 服务器 `chown /var/www/ai-forum` 后真实生效）

---

## 1. Goal of next session

在 `.github/workflows/deploy.yml` 的 **deploy job** 补上「前端构建 + scp 上传」，让 CI 之后前端静态产物自动更新到服务器 `/var/www/ai-forum/`（现状：只部署 API 镜像，前端停在旧版）；同步把 `docs/ops/deployment.md` 的手动前端部署章节改写为「CI 自动化 + 服务器初始化前置」。

**Prompts to answer**（会话开头先回答再动手）：
- 前端构建始终在 CI 运行（作构建验证），还是仅在配置了 SSH secrets 后构建？
- `/var/www/ai-forum` 的写权限怎么给到部署用户（scp 无 sudo）？

**验收标准**：
- 合并到 main + 配好 SSH secrets 后：服务器 `ls /var/www/ai-forum/` 直接见 `index.html` + `assets/`，`curl http://122.51.233.225/` 返回 PWA。
- 未配 SSH secrets 时：前端照常构建（验证），上传步跳过，run 不红。
- 改后的 deploy.yml 通过 actionlint（`bash scripts/check-workflows.sh`，且 workflow-lint.yml 全分支兜底）。

## 2. State of play

**Done（本会话已完成探查与设计，未写任何代码）**：
- 现状确认：`deploy.yml` 为 3-job（test/lint/deploy）。deploy job 内 SSH secrets 已 hoist 到 job 级 `env`（`SSH_HOST/SSH_USER/SSH_PRIVATE_KEY`），Deploy to server 步有 `if: env.SSH_HOST != ''` 门控；**没有任何前端步骤**，只 `git pull && docker compose pull/up api`。
- 前端构建无需环境变量：`frontend/src/api/client.ts` 的 `BASE = '/api/v1'` 是相对路径（nginx 反代）；`vite.config.ts` 默认 `base=/`、outDir=dist；uploads 走 nginx `/uploads/` 托管（独立于静态根）。生产包与 dev 无差异。
- `frontend/package-lock.json` 存在 → CI 用 `npm ci`。Node 要求：Vite 8.2.1 engines = `^20.19.0 || >=22.12.0` → 用 **Node 22**。
- scp-action 最新版：`appleboy/scp-action@v1.0.0`（repo 已用同作者 `appleboy/ssh-action@v1.0.3`，风格一致）。
- **服务器前置缺口**：`docs/ops/deployment.md` §4 step 6 手动部署用 `sudo cp`，`/var/www/ai-forum` **从未 chown 给部署用户**——scp-action 无 sudo，必须先补 chown。
- 已上线 `workflow-lint.yml`（全分支 push 跑 actionlint，**阻塞**）→ 改 deploy.yml 必须过 actionlint。

**Blocking**：无，任务可独立实现。

## 3. Open decisions

| # | 决策 | 选项 | lean | 依赖 |
|---|---|---|---|---|
| 1 | 前端构建时机 | A. 始终构建（仅上传步门控 SSH_HOST）／ B. 仅配了服务器才构建+上传 | **A**（与 API 镜像「always build+push, gated deploy」一致，免费获得 main 上的前端构建验证） | — |
| 2 | 上传方式 | A. scp-action 直传 `/var/www/ai-forum`（服务器初始化补 chown）／ B. scp 到中转目录 + ssh 内 `sudo mv` | **A**（与 §4 step 1 对 `/opt/ai-forum` 的 chown 模式一致，简单） | 服务器初始化加 `sudo chown $USER:$USER /var/www/ai-forum` |
| 3 | 旧文件清理 | A. 暂不做（scp 只增改不删，Vite 哈希资源留存，仅占磁盘）／ B. `find /var/www/ai-forum/assets -mtime +30 -delete` | **A**（MVP 可接受） | — |

## 4. Skills to use (next session)

- `codebase-design`（改 deploy.yml 结构时保持局部性；不要引入超出任务的抽象）
- `git-guardrails-claude-code`（commit 规范、分支操作）
- 本地校验直接跑 `bash scripts/check-workflows.sh`（不需额外 skill）；验证时 `cd frontend && npm run build`

## 5. Artifacts (reference only)

- `.github/workflows/deploy.yml` — 要改的文件（deploy job 结构、env 三段式、门控写法）
- `docs/ops/deployment.md` — §3.2（nginx root + `location /uploads/`）、§4 step 6（首部署清单，需补 chown）、§5.1（CI 示例，需补前端两步、删「可选：重新部署前端静态文件」注释）
- `docs/impl/testing.md` §4.1 — actionlint 规则 + `if` 上下文白名单（改 deploy.yml 的红线：secrets 永不进 if）
- `scripts/check-workflows.sh` — 本地 actionlint 校验脚本
- `docs/reviews/ci-secrets-if-debate.md` — secrets-in-if 事件复盘（理解 CI 0 秒失败背景，避免重蹈）
