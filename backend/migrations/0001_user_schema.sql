-- 0001 user schema（#5 + 下游依赖：invitation_codes）
-- 包内物理 FK / 跨包逻辑外键：跨 schema 引用只建索引不建约束（#5 D2）
CREATE SCHEMA IF NOT EXISTS "user";

CREATE TABLE "user".users (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(128) UNIQUE NOT NULL,
  username VARCHAR(64) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  avatar_url VARCHAR(255),
  bio TEXT,
  role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('member','moderator','admin')),
  status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','banned')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- 软删（#5 D2）：users 永不硬删，逻辑外键永不失联
CREATE TABLE "user".follows_users (
  id BIGSERIAL PRIMARY KEY,
  follower_id BIGINT NOT NULL,           -- 逻辑FK → users.id（建索引不建约束）
  target_id BIGINT NOT NULL,             -- 逻辑FK → users.id
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (follower_id, target_id),
  CHECK (follower_id <> target_id)
);
CREATE INDEX idx_follows_users_follower ON "user".follows_users (follower_id);
CREATE INDEX idx_follows_users_target ON "user".follows_users (target_id);

-- 邀请码（auth-flow §7，已确认）：线下生成、一次性、审计留痕
CREATE TABLE "user".invitation_codes (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(32) UNIQUE NOT NULL,           -- 线下生成的随机码
  created_by BIGINT NOT NULL,                 -- admin 用户 id，逻辑外键 → users.id
  used_by BIGINT UNIQUE,                      -- 使用该码注册的用户 id，逻辑外键 → users.id
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- 一旦 used_by 非空即视为已使用；不删除记录，保证可审计
  CHECK (used_by IS NULL OR used_at IS NOT NULL)
);
CREATE INDEX idx_invitation_codes_created_by ON "user".invitation_codes (created_by);
