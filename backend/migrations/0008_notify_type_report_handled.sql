-- 0008 notify.notifications.type 放行 report_handled（#53 被举报人视角的举报处理通知）
-- 约束名 notifications_type_check 已线上实查（0003 内联列级 CHECK，Postgres 确定性命名）
-- 两 ALTER 由迁移事务包裹原子执行；注释禁半角分号（迁移器按分号拆语句）
-- 注意：本文件注释与 CHECK 内不得出现半角分号，否则会被拆断
ALTER TABLE notify.notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notify.notifications ADD CONSTRAINT notifications_type_check CHECK (type IN ('follow','like','comment','reply','report_result','report_handled'));
