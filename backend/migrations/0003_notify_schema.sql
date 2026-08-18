-- 0003 notify schema（#5 D5：notify 零依赖，快照自足行）
CREATE SCHEMA IF NOT EXISTS notify;

CREATE TABLE notify.notifications (
  id BIGSERIAL PRIMARY KEY,
  recipient_id BIGINT NOT NULL,          -- 逻辑FK → user.users
  type VARCHAR(16) NOT NULL CHECK (type IN ('follow','like','comment','reply','report_result')),
  actor_id BIGINT,                       -- 触发人 id（仅客户端跳转，永不 join）
  actor_name VARCHAR(64),                -- 快照
  actor_avatar VARCHAR(255),             -- 快照
  target_type VARCHAR(16),
  target_id BIGINT,
  target_title VARCHAR(255),             -- 跳转锚点 + 标题（快照）
  is_read BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_recipient ON notify.notifications (recipient_id);

CREATE TABLE notify.messages (
  id BIGSERIAL PRIMARY KEY,
  from_user_id BIGINT NOT NULL,          -- 逻辑FK → user.users
  to_user_id BIGINT NOT NULL,            -- 逻辑FK → user.users
  content TEXT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_from ON notify.messages (from_user_id);
CREATE INDEX idx_messages_to ON notify.messages (to_user_id);
