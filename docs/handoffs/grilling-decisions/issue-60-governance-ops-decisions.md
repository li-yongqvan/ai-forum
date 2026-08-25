# issue #60 治理/运维收尾决策记录

> 日期：2026-08-25
> 背景：issue #60「治理/运维收尾：只读审计端点 + 举报弹窗封禁动作 + 异地备份」

## 已确认决策

### D1：审计端点权限与过滤
- **决策**：`GET /api/v1/moderation/actions` 对 **moderator+** 开放（与举报队列同权限）。
- **理由**：审计日志是治理透明性的基础读能力，不需要 admin-only；mod 日常处理也需要核对历史动作。
- **过滤能力**：`target_type`+`target_id`、`action`、`moderator_id`，分页 `page`/`page_size`（默认 20、最大 100），按 `created_at DESC, id DESC` 排序。

### D2：弹窗封禁动作走 `handleReport(action=ban_user)`
- **决策**：前端举报处理弹窗增加「封禁用户」按钮；点击后一步完成：封禁用户 + 追加审计 + 举报结案 + 双通知。
- **理由**：
  - 与 IA §5.6「处理动作」字面一致；
  - 比「先记报告、再跳用户页 ban」减少操作步骤和状态不一致窗口；
  - 复用现有 `HandleReport` 事务边界与通知链路。
- **限制**：仅 admin 可见/可用（#34 已限定 ban/unban 仅 admin）。

### D3：ban 原因必填
- **决策**：弹窗内选择「封禁用户」后，备注框变为「封禁原因（必填）」，提交前校验非空；后端同步校验空/纯空白 → 400。
- **理由**：封禁是强治理动作， moderation_actions.reason 必须留痕；空 reason 违反审计意图。
- **长度**：≤500 runes（与 moderation_actions.reason VARCHAR(500) 同源）。

### D4：二次确认
- **决策**：点击「确认封禁」先弹 `showConfirmDialog`（Vant），文案明确告知「解封需管理员操作」。
- **理由**：防止误触；封禁后果严重（无法登录、写拦截）。

### D5：ban 目标解析
- **决策**：举报目标为 post/comment 时，优先使用 `reports.target_author_id` 快照（#53 已引入）定位被封用户；快照缺失且内容已删时回退 `content.ResolveTarget`，仍失败则保持 pending。
- **理由**：内容被删除后仍可追责到作者；与 warn 走 ResolveTarget 刻意区分（warn 不封人，目标必须存在才能归责）。
- **user 目标**：直接封 `target_id`。

### D6：通知
- **决策**：ban 成功后同时给举报人发 `report_result`（结论句「已封禁用户」）、给被封用户发 `report_handled`（「你因「原因」被举报，已被封禁」）。
- **理由**：与 delete/warn 对齐，保障举报人与被处理人双端触达（#53 D1/D2）。

### D7：错误透传
- **决策**：user 域封禁错误（自封/封 admin/已封/不存在）经 `UserGateway.BanUser` adapter 翻译为 moderation 哨兵错误，再经 `respondModerationError` 映射 HTTP：自封/封 admin → 400，已封 → 409，不存在 → 404。
- **理由**：moderation 包不 import user 包（S1 seam 原则），错误语义在 adapter 收敛翻译。

### D8：异地备份方案
- **决策**：新增 `scripts/backup-offsite.sh` + `scripts/backup.conf.example`；目标可配置、可插拔（`local://`/`scp://`/`oss://`）。
- **短期接法**：`local://$HOME/backups-offsite`（同机目录），先通电验证；`scp://` 与 `oss://` 占位。
- **dry-run**：`--dry-run` 或 `BACKUP_DRY_RUN=1` 只打印不拷贝/不删除。
- **配置位置**：实际配置放在服务器 `$HOME/backup.conf`（仓库外），仓库只提交 `.example`。

## 与本票相关的协调

- **与 #59 合并顺序**：#59 先合；#60 只新增 `router.go` 一行（`GET /moderation/actions`），与 #59 改动范围低冲突，rebase 即可。
- **与 #61 合并顺序**：#61 尚未详细设计，#60 的 `moderation_actions` 索引与审计读模型不依赖 #61。
- **地图 #1**：由统筹方在 PR 合并后更新；本实现不直接改地图文件。

## 范围红线

- **不动**：notify/content 包实现、`.github/workflows/`、数据库表结构（只加索引迁移）。
- **只读审计**：`moderation_actions` 保持 append-only，不暴露 update/delete。
- **不真上 OSS/SCP**：本期仅 placeholder，长期目标未定（用户明示「都可能换」）。

## 验证要点

- 后端：`go test ./...`（含 testcontainers PG 集成测试）。
- 前端：`npm run test:unit && npm run build`；提交前 `git checkout -- frontend/components.d.ts`。
- 备份脚本：`bash -n scripts/backup-offsite.sh` + 本地临时目录 dry-run + real-run 验证 retention。
- 服务器：部署后 curl `/healthz`、审计端点过滤、bakcup-offsite `--dry-run`。
