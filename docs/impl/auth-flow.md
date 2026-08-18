# 注册 / 登录 / 鉴权流程约定

> 适用范围：AI 智联论坛 MVP（ai-forum）
> 依据：#5 数据模型、#9 信息架构/登录墙策略、#4 服务拆分（user 包）、#3 前端选型（Vue3 PWA）、#8 部署方案
> 状态：已确认（2026-08-18），生效为鉴权实现约定

---

## 1. 核心机制：JWT（无状态）

- **认证方式**：JWT access token，无 refresh token（MVP 简化）。
- **Token 有效期**：7 天（MVP 阶段长一点，减少重新登录频率）。
- **存储位置**：浏览器 `localStorage`（PWA 常见做法）。
  - *风险*：XSS 可窃取 token；MVP 接受此风险，通过输出时转义 HTML/谨慎处理 innerHTML  mitigation。
  - *替代*：HttpOnly cookie 更安全，但会增加 CSRF 防护复杂度；如社团后续安全要求提高，可迁移。
- **Token payload**：`user_id`、`username`、`role`（guest/user/moderator/admin）、`exp`。
- **Token 传输**：每个需要鉴权的请求在 `Authorization: Bearer <token>` 中携带。

## 2. 登录方式

- 使用 **用户名 + 密码** 登录。
- `users.username` 同时是登录名和展示名（#5 已定，不设 nickname）。
- 登录成功返回：`{ token, user: { id, username, role, avatar_url } }`。
- 登录失败统一返回 401，不区分「用户名不存在」或「密码错误」（防枚举）。

## 3. 注册方式

- 字段：用户名、邮箱、密码、邀请码。
- 邀请码：
  - 由管理员线下生成（#9 定案）。
  - 注册时校验 `invitation_codes.code`，且 `used_by IS NULL`。
  - 注册成功后，把该码标记为 `used_by = 新用户 id`、`used_at = now()`。
- 注册成功后**自动登录**，直接返回 token（减少一步）。
- 用户名、邮箱全局唯一；密码最小 8 位。
- **邮箱验证**：MVP **不做**（已确认），避免引入邮件服务；邮箱作为注册必填字段与全局唯一标识，供后续密码找回/通知预留。如后续需要，再独立设计。

## 4. 登录墙策略（#9 已定，技术侧落地）

| 场景 | 行为 |
|---|---|
| 浏览 feed / 板块 / 帖子详情 | 完全开放，不弹登录 |
| 点赞 / 收藏 / 评论 / 发帖 / 关注 / 私信 | 游客触发 → 跳 `/login`，记录 `returnTo`，登录后返回原页面；取消登录留在原页 |
| 点底部 Tab「通知」或「我的」 | 游客 → **原地展示登录引导页**（不跳转），登录后刷新当前 Tab |
| 访问管理相关页面（/reports 等） | 先鉴权，无权限或游客直接跳登录或 403 |

- `returnTo` 用 query param：`/login?returnTo=/post/42`。
- 登录成功后，前端读取 `returnTo` 并 replace 跳转；若无则去 `/feed`。

## 5. 角色与权限

- 角色枚举（#5 `users.role`）：`guest` / `user` / `moderator` / `admin`。
- JWT payload 带 `role`，前端用此控制菜单/按钮可见性（体验层）。
- **后端每个接口独立鉴权**，不信任前端 role；权限矩阵见 #9 §5.0。
- `guest` 不写入 DB，仅表示未登录状态。

## 6. 密码安全

- 存储：`password_hash` 用 **bcrypt**（Go 标准库 `golang.org/x/crypto/bcrypt`）。
- 传输：HTTPS 未启用（#8 MVP 纯 HTTP on IP），因此密码在公网传输是明文的——这是已知风险，社团内部/校园网使用可接受；若后续上域名 HTTPS，自动消除。
- 密码找回：MVP **不做**（已确认），如需找回联系管理员线下重置。

## 7. 邀请码数据模型补充

#5 数据模型中缺少邀请码表。MVP 新增一张表（放在 `user` schema，已确认）：

```sql
CREATE TABLE user.invitation_codes (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(32) UNIQUE NOT NULL,           -- 线下生成的随机码
  created_by BIGINT NOT NULL,                 -- admin 用户 id，逻辑外键 → users.id
  used_by BIGINT UNIQUE,                      -- 使用该码注册的用户 id，逻辑外键 → users.id
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  -- 一旦 used_by 非空即视为已使用；不删除记录，保证可审计
  CHECK (used_by IS NULL OR used_at IS NOT NULL)
);
```

- 管理员通过直接 INSERT 生成邀请码（DB 直管），MVP 不提供管理 UI。
- 此补充已关联到地图 #1「下游依赖」记录。

## 8. 注销

- 前端清除 `localStorage.token` 并 replace 到 `/feed`。
- 后端提供 `POST /logout` 端点（MVP 可空实现返回 200，因为 JWT 无状态无法真正失效）。
- 若后续需要「强制某用户 token 失效」，需引入 token 黑名单或缩短 token 有效期——MVP 不做。

## 9. 关键 API 端点

| 端点 | 鉴权 | 说明 |
|---|---|---|
| `POST /api/auth/register` | 无 | 注册 |
| `POST /api/auth/login` | 无 | 登录 |
| `POST /api/auth/logout` | 有 | 注销（空实现） |
| `GET /api/auth/me` | 有 | 返回当前登录用户信息 |

## 10. 确认记录（2026-08-18）

| 决策点 | 结论 |
|---|---|
| JWT 有效期 | **7 天，无 refresh token** |
| token 存储 | **localStorage**（接受 XSS 风险，以转义 HTML/慎用 innerHTML 缓解） |
| 邮箱验证 | **不做**（邮箱为注册必填 + 全局唯一标识，供后续预留） |
| 密码找回 | **不做**（线下联系管理员重置） |
| 邀请码表 | **照 §7 DDL 建表**（`user` schema，含审计字段） |

以上决策由用户于 2026-08-18 逐一确认；实现阶段以本文件为准。待确认清单已清空。
