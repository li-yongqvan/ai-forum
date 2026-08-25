-- Migration: moderation_actions 查询索引（#60 审计端点按 target 过滤）
CREATE INDEX IF NOT EXISTS idx_moderation_actions_target
    ON moderation.moderation_actions (target_type, target_id);

CREATE INDEX IF NOT EXISTS idx_moderation_actions_moderator
    ON moderation.moderation_actions (moderator_id);

CREATE INDEX IF NOT EXISTS idx_moderation_actions_action
    ON moderation.moderation_actions (action);
