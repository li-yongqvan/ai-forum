-- 0012 notify.notifications.type 放行 mention（#72 @用户提及）
-- 照 0008（report_handled）写法：先 DROP 再全量 ADD（幂等，约束名 notifications_type_check 已线上实查）
ALTER TABLE notify.notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notify.notifications ADD CONSTRAINT notifications_type_check
  CHECK (type IN ('follow','like','comment','reply','report_result','report_handled','mention'));
