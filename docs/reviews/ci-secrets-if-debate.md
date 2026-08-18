# 辩论裁决书：deploy.yml「CI 0 秒」——secrets 能否用于 step 级 `if`

> 争议双方：
> - **Agent A（Claude，本文作者）**：`secrets` 上下文在 `if` 条件中 **job 级、step 级均不可用**；修复 = secrets 落到 job 级 `env`，step `if` 用 `env` 上下文。
> - **Agent B（另一位 agent）**：根因是 job 级 `if` 用了 secrets（**正确**）；主张「secrets 只能在 env、`steps[*].if`、`steps[*].env`、`steps[*].with` 中使用」，修复 = 把条件移到 **step 级 `if`**（**错误**）。
>
> 结论先行：**B 对根因的定位正确，但提出的修复方案基于一个错误前提，执行后 CI 依然会 0 秒失败。A 的修复已通过三重验证。** 本文给出可复现证据，供你拿去与 B 对质。

---

## 1. 分歧点（精确到一句）

B 的原话：**「（secrets）只能在 env、`steps[*].if`、`steps[*].env`、`steps[*].with`」「把条件从 job 级移到 deploy 步的 step 级 if（该位置明确支持 secrets）」**。

A 的反驳：**`secrets` 上下文不允许出现在任何 `if` 条件中——无论 job 级还是 step 级。** 这是 GitHub Actions 的硬性规定，与层级无关。

---

## 2. 证据一：GitHub 官方文档（权威来源）

出处：<https://docs.github.com/en/actions/learn-github-actions/contexts> → 「Context availability」表。

| 位置 | `if` 中允许的上下文（原文列表） | `secrets` 是否可用 |
|---|---|---|
| `jobs.<job_id>.if` | `github, needs, vars, inputs` | ❌ 不在列表 |
| `jobs.<job_id>.steps[*].if` | `github, needs, strategy, matrix, job, runner, env, vars, steps, inputs` | ❌ **不在列表** |

`secrets` 在文档表中**可用的**位置是：`env`、`jobs.<job_id>.env`、`jobs.<job_id>.outputs.<id>`、`jobs.<job_id>.steps.env`、`jobs.<job_id>.steps.name`、`jobs.<job_id>.steps.run`、`jobs.<job_id>.steps.timeout-minutes`、`jobs.<job_id>.steps.with`、`jobs.<job_id>.steps.working-directory`、`jobs.<job_id>.steps.continue-on-error` 等——**唯独没有任何 `if`**。

> B 列出的「env、steps[*].if、steps[*].env、steps[*].with」中，`steps[*].if` 是唯一一项错误，其余三项恰好都在允许列表里——很像把「availability 表里与 if 相邻的几行」误读成了「if 里可用」。

## 3. 证据二：actionlint 实测（可复现，30 秒）

actionlint 是 GitHub Actions 的语义级 linter，内置上下文可用性检查。对 **step 级** 写法做最小复现：

```bash
cat <<'EOF' | docker run --rm -i rhysd/actionlint:latest -oneline -
name: repro
on: push
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
        if: ${{ secrets.X != '' }}
EOF
```

输出（exit 1）：

```
<stdin>:8:17: context "secrets" is not allowed here. available contexts are "env", "github", "inputs", "job", "matrix", "needs", "runner", "steps", "strategy", "vars". see https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability for more details [expression]
```

**注意这句话**：`available contexts are "env", ..., "vars"`——与官方文档的 step-`if` 允许列表**逐字一致**，`secrets` 不在其中。这正是 B 方案（step 级 `if` 用 secrets）执行后的报错。

## 4. 证据三：仓库内实证（为什么 4 次 run 全 0 秒）

`git log --follow` 显示 deploy.yml 只有两个提交：

- `36fdce7`（后端脚手架）**引入时就是 job 级**：`deploy:` 下第 42 行 `if: ${{ secrets.SSH_HOST != '' }}`。
- `00b89ef`（A 的修复）改为 env 方案。

4 次 0 秒失败的 run（02:10 / 06:58 / 07:03 / 07:54）全部发生在 `36fdce7` 之后、`00b89ef` 之前的提交上——**全部是 job 级版本**。B 的「job→step」版本从未被提交过（只在工作区短暂存在，已被 A 的修复覆盖），所以 step 级版本的行为没有 run 实测——但按证据一、二，它必然同样报错。

> 推论：B 与 A 对根因的表述一致（job 级 secrets-if → 解析失败 → run 名退化为文件路径 → `branches: [main]` 失效 → feat 分支误触发）。分歧只在**修法**，而修法上 B 的牌打错了位置。

## 5. 我的修复为何正确（且是唯一正确路径）

`if` 条件里要判断「是否配置了 SSH secrets」，唯一的合法通道是 **`env` 上下文**（它在 step-`if` 允许列表内，见证据一）。

```yaml
deploy:
  needs: test
  runs-on: ubuntu-latest
  permissions: { contents: read, packages: write }
  env:                                   # secrets → job 级 env（secrets 允许用于 env）
    SSH_HOST: ${{ secrets.SSH_HOST }}
    SSH_USER: ${{ secrets.SSH_USER }}
    SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
  steps:
    # ...checkout / buildx / login / build-push 照旧（镜像仍推 GHCR）...
    - name: Deploy to server
      if: env.SSH_HOST != ''             # step if 用 env 上下文（允许列表内）
      uses: appleboy/ssh-action@v1.0.3
      with:
        host: ${{ env.SSH_HOST }}        # 消费 job env，而非直接 secrets
        username: ${{ env.SSH_USER }}
        key: ${{ env.SSH_PRIVATE_KEY }}
```

验证：`docker run --rm -v "C:/Users/liyongquan/ai-forum:/repo" -w /repo rhysd/actionlint:latest -oneline .github/workflows/deploy.yml` → **exit 0，零告警**。

行为等价于 B 的意图：未配 SSH secrets 时部署步被跳过，镜像照常推 GHCR；配了则正常 SSH 部署。

## 6. 顺带澄清一个 B 的表述

B 说「修复现在在 feat/image-upload 分支上（未提交）」。事实上该修复已由 A **提交（`00b89ef`）并推送**到 `origin/feat/image-upload`，且 actionlint 通过。B 留下的那个「step 级」改动是**未提交的工作区状态**，存在即会再次让 CI 0 秒失败——已按 A 方案覆盖。

## 7. 给 B 的复核路径（30 秒内可自证）

1. 打开官方文档链接（证据一），看 step-`if` 允许列表里有没有 `secrets`。
2. 跑证据二的 actionlint 最小复现，看它对 step 级 `if: ${{ secrets... }}` 的报错。
3. 用 A 的 env 方案跑同一命令，exit 0。

## 8. 后续规则（防止复发）

- **规则**：`secrets` 永不进 `if` 条件（job/step 皆禁）；要按 secrets 分支，一律 `secrets → job 级 env → step if 用 env`。
- 建议将这条规则与 `if` 上下文白名单写入 `docs/impl/testing.md` 或 CI 约定，并在 repo 里把 actionlint 加为本地校验（`golangci-lint` 管 Go，workflow 用 actionlint）。
- 落地顺序（用户已确认）：修 `feat/backend-scaffold`（cherry-pick `00b89ef`）→ 合并 PR #11、PR #13 到 main → 首次真实跑 `go test` CI（仓库未配 SSH secrets，部署步必跳过，不会上云）。
