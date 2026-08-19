# 8888 部署功能验收结果（2026-08-19）

> 对应方案：`docs/ops/acceptance-8888.md`。环境：`http://122.51.233.225:8888/`（web nginx → api → postgres 全链路）。
> 结论：**21/22 通过，1 个真 bug（UPLOAD-1 上传 URL 缺端口）**，已定位根因，待修复。

## 汇总表

| # | 用例 | 结果 | 说明 |
|---|---|---|---|
| INFRA-1 | `/healthz` | ✅ 200 | `{"status":"ok"}` |
| INFRA-2 | `/` PWA | ✅ 200 | Vue index.html 正常 |
| INFRA-3 | `/api/v1/boards` | ✅ 200 | 6 板块，反代无尾斜杠 |
| INFRA-4 | `docker compose ps` | ✅ 三容器 healthy | |
| AUTH-1 | 注册（新邀请码） | ✅ 201 | 返回 token+user |
| AUTH-2 | 无效邀请码 | ✅ 400 | `邀请码无效或已使用` |
| AUTH-3 | 重复用户名/邮箱 | ✅ 409 | `用户名或邮箱已存在` |
| AUTH-4 | 登录成功 | ✅ 200 | token 正常 |
| AUTH-5 | 登录错密码 | ✅ 401 | `用户名或密码错误` |
| AUTH-6 | `/auth/me` 带 token | ✅ 200 | 返回当前用户 |
| AUTH-7 | `/auth/me` 无 token | ✅ 401 | `未登录` |
| CONTENT-1 | 发帖 | ✅ 201 | post id=1 |
| CONTENT-2 | 帖子详情 | ✅ 200 | 含 `[smoke]` 帖 |
| CONTENT-3 | 帖子列表 | ✅ 200 | count=1 |
| CONTENT-4 | 评论 | ✅ 201 | comment id=1 |
| CONTENT-5 | 评论列表 | ✅ 200 | key 为 `comments`（初判 0 是解析用错 key，非 bug） |
| CONTENT-6 | 点赞 | ✅ 200 | |
| CONTENT-7 | 收藏 | ✅ 200 | |
| CONTENT-8 | 关注 | ✅ 200 | |
| CONTENT-9 | 未授权发帖 | ✅ 401 | |
| UPLOAD-1 | 上传图片 | ❌ **201 但 URL 缺 `:8888`** | 返回 `http://122.51.233.225/uploads/…` → 指向 80 端口，前端按此渲染图片必 404 |
| UPLOAD-2 | 图片直出 | ✅ 200 | 同源 `/uploads/` 由 web nginx alias 直托 |
| UPLOAD-3 | 非图片 | ✅ 400 | `仅支持 jpg/png/gif/webp` |
| UPLOAD-4 | 超限 6MB | ✅ 413 | `图片过大`（5MB 应用上限） |
| UPLOAD-5 | 未授权上传 | ✅ 401 | |
| RL-1 | 登录限流 | ✅ 503 生效并恢复 | burst20 nodelay 后超额 503，5s 后恢复 200 |
| ENC | 中文 UTF-8 往返 | ✅ True/True | 数据正确，终端乱码系 Windows GBK 显示问题 |

## Bug #1：上传 URL 缺端口（影响 发图）

- **现象**：`POST /api/v1/uploads` 返回 `{"url":"http://122.51.233.225/uploads/67fc….png"}`，无 `:8888`。
- **影响**：前端 `ImagePicker.vue` 直接 emit `res.url` → 帖子/评论里的图片链接指向 80 端口（宿主 nginx 默认站）→ **镜像 404**。发图链路核心展示环节不可用。
- **根因**：`handler/upload.go:82` 用 `c.Request.Host` 拼 URL；`deploy/web.conf` 的 `proxy_set_header Host $host;` 中 nginx `$host` **去掉端口** → api 收到 Host=`122.51.233.225`。
- **修复**：web.conf 中所有代理块 `Host $host` → `Host $http_host`（保留原始 Host 含端口）。改后 api 拼出 `http://122.51.233.225:8888/uploads/…`。
- **验收方法**：改后重传一张图，断言返回 URL 含 `:8888` 且 `GET` 200。

## 测试产生的数据（清理清单）

- 用户：`smoketest_user`(id=1)、`smoke_qa_153000`(id=2)
- 已消耗邀请码：`SMOKETEST2026`、`SMOKEQA153000`、`SMOKEDUP<ts>`
- 帖子：id=1 `[smoke] 冒烟测试帖`、id=2 `[smoke] 编码验证-中文标题`
- 评论：id=1（post 1）
- 点赞/收藏/关注：用户 1 对 post1 / user2
- 上传文件：`uploads/67fc02ec6613ecbcd75504aac690b802.png`
- 清理决策：待用户确认（本轮未清理）

## 附注

- 早期通过 Git Bash `curl -d` 发的几条中文内容显示乱码，系**客户端 GBK 编码**所致（非应用 bug）；python UTF-8 显式往返比对 **True/True** 确认应用数据链路正确。
- lint job 的 Node 20 弃用失败已随 PR #18 修复（golangci-lint-action v7）。
