-- 0010 补齐 user schema 包内物理外键（#69）
-- 策略：包内物理 FK，跨包逻辑外键（#5 D2）。
-- users 表软删永不硬删，ON DELETE 默认 NO ACTION 即可。

ALTER TABLE "user".follows_users
  ADD CONSTRAINT fk_follows_users_follower
  FOREIGN KEY (follower_id) REFERENCES "user".users(id);

ALTER TABLE "user".follows_users
  ADD CONSTRAINT fk_follows_users_target
  FOREIGN KEY (target_id) REFERENCES "user".users(id);

ALTER TABLE "user".invitation_codes
  ADD CONSTRAINT fk_invitation_codes_created_by
  FOREIGN KEY (created_by) REFERENCES "user".users(id);

ALTER TABLE "user".invitation_codes
  ADD CONSTRAINT fk_invitation_codes_used_by
  FOREIGN KEY (used_by) REFERENCES "user".users(id);
