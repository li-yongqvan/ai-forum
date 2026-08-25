# Package user — 身份域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 数据模型、#7 深模块规范、auth-flow 实现约定。本文件描述「调用者必须知道的事实」；签名与逐方法语义见 Godoc，两者不重复。

## 范围

对外提供 `Service` 接口（粗粒度命令 + 读模型查询）。`Repo` 是实现内部 seam，调用方不应使用；测试跨 `Service` 验证（#7 接口即测试面）。

## 不变量（Invariants）

- **用户名/邮箱全局唯一**，注册即校验；重复返回 `ErrUsernameTaken` / `ErrEmailTaken`。
- **邀请码一次性**：注册成功后立即标记已用（`used_by`/`used_at`），记录不删除（审计留痕）；已用或不存在一律返回 `ErrInvalidInvite`。
- **密码存储**：仅存 bcrypt hash，永不存明文、永不返回 hash。
- **角色映射（一致性修正，计划已记录）**：DB 存 `member/moderator/admin`；`Service` 返回与 JWT 载荷均为对外取值 `user/moderator/admin`（`member→user` 由本包在边界映射）。`guest` 仅前端状态，永不入库、不入 token。
- **软删**：users 永不硬删（`deleted_at`）；逻辑外键永不因删除失联。
- **封禁/解封（#34）**：状态存 `status`（active|banned，零迁移）；`Ban` 条件更新 `active→banned`、`Unban` 条件更新 `banned→active`（RowsAffected=0 → `ErrAlreadyBanned`/`ErrNotBanned`，**幂等防重复审计**）；**不能封自己、不能封 admin**；审计由 handler 层调用 `moderation.RecordAction` 追加（本包不写审计表）。
- **写拦截（#34）**：`IsActive(userID)` 返回用户是否可执行写操作（存在且非 banned；软删/不存在 → false，fail-closed），供写拦截中间件 `RequireActive` 查 DB 即时生效（JWT claims 最长 7 天，封禁须即时阻断）。
- **`PublicProfileView.Banned`（#34）**：服务层恒算（`status=="banned"`）；序列化门控在 handler（仅 admin/self 可见，治理信息不外泄）。
- **我的关注用户列表（#23）**：`ListFollowedUsers` 按关注时间倒序 + 分页，需登录（游客 `ErrAuthRequired`）；行模型 `UserFollowView` 仅暴露最小画像字段（username/avatar_url/bio + `viewer.following`），不泄 Email/Role、不带多余计数。

## 调用顺序约束（Ordering）

- 无强制调用顺序：命令彼此独立，可任意顺序调用。
- 注册即自动登录（返回 token），无需先注册再单独登录。
- `Follow` 前置条件：目标用户必须存在、非本人、未重复关注——服务内部校验，调用方无需预查。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 调用方建议映射 |
|---|---|---|
| `ErrUsernameTaken` / `ErrEmailTaken` | 注册唯一性冲突 | 409 |
| `ErrInvalidInvite` | 邀请码不存在或已用 | 400 |
| `ErrWeakPassword` | 密码 < 8 位 | 400 |
| `ErrBadCredential` | 登录失败统一语义（用户不存在/密码错**同错**，防枚举） | 401 |
| `ErrBanned` | 账号被封禁 | 401（保持统一，不泄露账号状态） |
| `ErrNotFound` | 目标用户不存在 | 404 |
| `ErrSelfFollow` / `ErrAlreadyFollow` | 关注非法 | 400 / 409 |
| `ErrAuthRequired` | 我的关注用户列表游客（#23） | 401 |
| `ErrSelfBan` / `ErrCannotBanAdmin` | 封禁守卫（不能封自己/封管理员，#34） | 400 |
| `ErrAlreadyBanned` | 重复封禁（#34 幂等） | 409 |
| `ErrNotBanned` | 解封未封用户（#34 幂等） | 409 |

除上述哨兵外，其余 error 均为基础设施故障（DB 等），调用方应记录日志并按 500 处理。

## 必需配置（Required config）

- `TokenIssuer` 依赖：本包不自行创建，由调用方注入（#7 接受依赖而非创建）。生产实现见 `internal/auth.Manager`（HS256，7 天有效期）。
- `Repo` 依赖：真实实现 `NewGormRepo(db)` 需一个已迁移的 PostgreSQL 连接（schema `user` 就绪）。

## 性能特征（Performance）

- 注册/登录各 1–2 次索引查询 + 1 次写入；bcrypt DefaultCost（~10）单次 ~50–100ms，为注册/登录路径的固有成本，无需调用方缓存。
- 关注路径 ≤3 次索引查询 + 1 次写入；50 人规模（#5）下无放大风险，不做冗余计数列。
