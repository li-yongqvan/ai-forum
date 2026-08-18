-- 0004 moderation schema（#5 D6 + 下游依赖：action 补 unban_user）
-- reports 与 moderation_actions 永不删除（审计）
CREATE SCHEMA IF NOT EXISTS moderation;

CREATE TABLE moderation.reports (
  id BIGSERIAL PRIMARY KEY,
  reporter_id BIGINT NOT NULL,           -- 逻辑FK → user.users
  target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('user','post','comment')),
  target_id BIGINT NOT NULL,             -- 跨 schema 多态、无 FK
  reason TEXT NOT NULL,
  -- 一致性修正（计划已记录）：#5 原文 'open'，IA v2 §5.6 与原型用 'pending'，以较新的 v2 为准
  status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','resolved','dismissed')),
  handler_id BIGINT,                     -- 逻辑FK → user.users
  handled_at TIMESTAMPTZ,
  handling_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reports_reporter ON moderation.reports (reporter_id);
CREATE INDEX idx_reports_status ON moderation.reports (status);

-- 防重复举报：同一举报者对同一目标（同类型）在 pending 期内只允许一条（IA v2 §5.6）
CREATE UNIQUE INDEX uq_reports_pending ON moderation.reports (reporter_id, target_type, target_id) WHERE status = 'pending';

-- append-only 审计日志：仅 created_at，写死不可改（#5 D6）
CREATE TABLE moderation.moderation_actions (
  id BIGSERIAL PRIMARY KEY,
  moderator_id BIGINT NOT NULL,          -- 逻辑FK → user.users
  -- 下游依赖#1（#6/#9 遗留标注）：解封必须留痕，补 unban_user 进枚举
  action VARCHAR(32) NOT NULL CHECK (action IN ('delete_post','delete_comment','ban_user','warn','unban_user')),
  target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('user','post','comment')),
  target_id BIGINT NOT NULL,
  reason VARCHAR(500) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_moderation_actions_moderator ON moderation.moderation_actions (moderator_id);
