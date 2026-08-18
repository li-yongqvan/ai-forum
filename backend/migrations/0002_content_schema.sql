-- 0002 content schema（#5）
-- 物理 FK 仅 content 包内；author_id 等逻辑 FK 建索引不建约束
CREATE SCHEMA IF NOT EXISTS content;

CREATE TABLE content.boards (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  description TEXT,
  parent_id BIGINT REFERENCES content.boards (id),   -- 自引用物理FK，一行支持多级板块
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE content.topics (
  id BIGSERIAL PRIMARY KEY,
  board_id BIGINT NOT NULL REFERENCES content.boards (id),
  name VARCHAR(64) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_topics_board ON content.topics (board_id);

CREATE TABLE content.posts (
  id BIGSERIAL PRIMARY KEY,
  board_id BIGINT NOT NULL REFERENCES content.boards (id),
  topic_id BIGINT REFERENCES content.topics (id),
  author_id BIGINT NOT NULL,             -- 逻辑FK → user.users
  title VARCHAR(255) NOT NULL,
  content TEXT NOT NULL,
  is_pinned BOOLEAN NOT NULL DEFAULT false,
  is_featured BOOLEAN NOT NULL DEFAULT false,
  view_count INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
-- 故意不加 like_count/comment_count 冗余列（#5：50 人规模读侧 COUNT 零压力）
CREATE INDEX idx_posts_author ON content.posts (author_id);
CREATE INDEX idx_posts_board ON content.posts (board_id);
CREATE INDEX idx_posts_topic ON content.posts (topic_id);

CREATE TABLE content.comments (
  id BIGSERIAL PRIMARY KEY,
  post_id BIGINT NOT NULL REFERENCES content.posts (id),
  author_id BIGINT NOT NULL,             -- 逻辑FK → user.users
  parent_id BIGINT REFERENCES content.comments (id),  -- 邻接表（#5 D3）
  floor INT,                             -- 仅顶层评论 1..N，回复不占楼层
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_comments_post ON content.comments (post_id);
CREATE INDEX idx_comments_author ON content.comments (author_id);

CREATE TABLE content.likes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,               -- 逻辑FK → user.users
  target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('post','comment')),
  target_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, target_type, target_id)
);
CREATE INDEX idx_likes_target ON content.likes (target_type, target_id);

CREATE TABLE content.favorites (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,               -- 逻辑FK → user.users
  post_id BIGINT NOT NULL REFERENCES content.posts (id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, post_id)              -- 只收藏帖子（#5 砍掉多态）
);
CREATE INDEX idx_favorites_post ON content.favorites (post_id);

CREATE TABLE content.follows_boards (
  id BIGSERIAL PRIMARY KEY,
  follower_id BIGINT NOT NULL,           -- 逻辑FK → user.users
  board_id BIGINT NOT NULL REFERENCES content.boards (id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (follower_id, board_id)
);
CREATE INDEX idx_follows_boards_board ON content.follows_boards (board_id);

CREATE TABLE content.follows_topics (
  id BIGSERIAL PRIMARY KEY,
  follower_id BIGINT NOT NULL,           -- 逻辑FK → user.users
  topic_id BIGINT NOT NULL REFERENCES content.topics (id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (follower_id, topic_id)
);
CREATE INDEX idx_follows_topics_topic ON content.follows_topics (topic_id);
