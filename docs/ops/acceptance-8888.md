# 8888 部署功能验收测试方案

> 目标：对 `http://122.51.233.225:8888/`（web nginx → api → postgres 全链路）做功能层验收。
> 环境：生产服务器（共享，无 sudo）。测试会**写入生产库**（测试账号/帖子/上传），已获用户授权；数据用独立前缀便于识别，清理另议。
> 执行：curl 脚本化，逐步断言 HTTP 码 + 响应关键字段，输出 PASS/FAIL 表。

## 0. 测试数据策略

| 项 | 值 |
|---|---|
| 已存在测试用户 | `smoketest_user` / `Smoketest123!`（上轮注册，id=1，密码已知） |
| 新增测试用户 | `smoke_qa_<ts>`（每个新用户消耗一个新建邀请码） |
| 邀请码来源 | 服务器 DB 直插 `"user".invitation_codes`（一次性） |
| 上传/帖子/评论 | 全部挂测试用户下，标题/内容带 `[smoke]` 前缀，便于识别清理 |

## 1. 基础设施（基线，已通过，复核一次）

| 用例 | 步骤 | 期望 |
|---|---|---|
| INFRA-1 | `GET /healthz` | 200 `{"status":"ok"}` |
| INFRA-2 | `GET /` | 200，返回 PWA index.html |
| INFRA-3 | `GET /api/v1/boards` | 200，6 个板块（反代无尾斜杠） |
| INFRA-4 | 服务器 `docker compose ps` | 三容器 healthy |

## 2. 认证（TC-AUTH）

| 用例 | 步骤 | 期望 |
|---|---|---|
| AUTH-1 注册成功 | 新邀请码 + `{username,email,password,invite_code}` | 201 + `token` + `user` |
| AUTH-2 无效邀请码 | 用 AUTH-1 后的已用码或乱码 | 4xx（`邀请码无效或已使用`） |
| AUTH-3 重复用户名/邮箱 | 复用 `smoketest_user` 的 username | 4xx 冲突 |
| AUTH-4 登录成功 | `{username,password}` 正确 | 200 + `token` |
| AUTH-5 登录失败 | 错误密码 | 401 |
| AUTH-6 me | 带 token `GET /auth/me` | 200 返回当前用户 |
| AUTH-7 me 未授权 | 无/坏 token `GET /auth/me` | 401 |

## 3. 内容（TC-CONTENT）

| 用例 | 步骤 | 期望 |
|---|---|---|
| CONTENT-1 发帖 | 带 token `POST /posts` `{board_id:1, title:"[smoke]…", content:"…"}` | 201 |
| CONTENT-2 帖子详情 | `GET /posts/:id` | 200，含刚建的帖 |
| CONTENT-3 帖子列表 | `GET /posts` | 200，含 `[smoke]` 帖 |
| CONTENT-4 评论 | 带 token `POST /comments` `{post_id, content:"[smoke]…"}` | 201 |
| CONTENT-5 评论列表 | `GET /posts/:id/comments` | 200 含该评论 |
| CONTENT-6 点赞 | 带 token `POST /likes`（body 含 target_type/target_id） | 200 |
| CONTENT-7 收藏 | 带 token `POST /favorites` | 200 |
| CONTENT-8 关注 | 带 token `POST /follows` | 200 |
| CONTENT-9 未授权写 | 无 token `POST /posts` | 401 |

## 4. 上传（TC-UPLOAD）

| 用例 | 步骤 | 期望 |
|---|---|---|
| UPLOAD-1 上传图片 | 带 token `POST /uploads` multipart `file`=png/jpg | 201 `{url}` |
| UPLOAD-2 图片直出 | `GET <url>`（web alias `/uploads/`） | 200，content-type 图片，不经 Go |
| UPLOAD-3 非图片 | 上传 txt | 400 `仅支持 jpg/png/gif/webp` |
| UPLOAD-4 超限 | 上传 >5MB（应用校验）/ >20m（nginx） | 413 |
| UPLOAD-5 未授权 | 无 token 上传 | 401 |

## 5. 限流（可选，TC-RL）

| 用例 | 步骤 | 期望 |
|---|---|---|
| RL-1 登录限流 | 快速连续 >20 次 login | 触发 `limit_req`（429/503）后恢复 |

> 可选：因会临时影响测试 IP 的登录，跑之前会说明。测完自然恢复（`limit_req_zone 10r/m`）。

## 6. 执行与结果记录

1. 按表逐条执行，每条打印：用例号 → 实际 HTTP 码 → 关键字段 → PASS/FAIL。
2. 全部完成后汇总 PASS/FAIL 表 + 失败项分析（含网络间歇性 404 的区分）。
3. 记录测试产生的数据清单（账号/帖子/评论/上传），供后续清理决策。

## 7. 已知注意

- 本机到 GitHub API 的网络间歇性 404 不影响本测试（目标服务器为直连 8888）。
- 发帖/发图等核心链路最终仍建议你在浏览器里人工过一遍（你更需要的真实体验）。
