-- 0011 content.mentions 提及关联表（#72 @用户提及）
-- 多态 (target_type, target_id) 复用 content.likes 先例（0002：target_type CHECK IN ('post','comment')）
-- mentioned_user_id → user.users(id)：跨包逻辑外键，只建索引不建约束（FK 政策 #5 D2）
CREATE TABLE content.mentions (
  id BIGSERIAL PRIMARY KEY,
  target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('post','comment')),
  target_id BIGINT NOT NULL,                 -- 逻辑指向 content.posts/comments（同 schema 内联）
  mentioned_user_id BIGINT NOT NULL,         -- 逻辑FK → user.users(id)
  mentioned_username VARCHAR(64) NOT NULL,   -- 创建时快照（对齐 users.username VARCHAR(64)）
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (target_type, target_id, mentioned_user_id)
);
CREATE INDEX idx_mentions_target ON content.mentions (target_type, target_id);
CREATE INDEX idx_mentions_user ON content.mentions (mentioned_user_id);
