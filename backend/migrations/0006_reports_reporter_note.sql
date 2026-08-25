-- 0006 reports 加举报人备注列（#33 O2 裁决：reason 只存六枚举之一，备注独立存储）
-- additive 可空列，回滚安全；应用启动 embed 自动跑（migrate.go 词典序）
ALTER TABLE moderation.reports ADD COLUMN reporter_note TEXT;
