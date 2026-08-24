-- 0007 标签 schema（#54）
-- 归一化小写 name 唯一（大小写不敏感）；post_tags 一对多，两表物理 FK
CREATE TABLE content.tags (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(60) NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE content.post_tags (
  id BIGSERIAL PRIMARY KEY,
  tag_id BIGINT NOT NULL REFERENCES content.tags (id) ON DELETE CASCADE,
  post_id BIGINT NOT NULL REFERENCES content.posts (id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tag_id, post_id)
);
CREATE INDEX idx_post_tags_post ON content.post_tags (post_id);
CREATE INDEX idx_post_tags_tag ON content.post_tags (tag_id);
