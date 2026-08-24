-- 0007 reports 加被举报人 id 快照列（#53 补发被举报人通知：delete 路径幂等不 ResolveTarget，
-- 处理后内容可能已删，须在 CreateReport 存在性校验时快照作者 id）
-- additive 可空列，回滚安全；存量行 NULL → 处理时跳过被举报人通知 + slog.Warn
ALTER TABLE moderation.reports ADD COLUMN target_author_id BIGINT;
