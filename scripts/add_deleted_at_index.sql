-- 为 alerts 表的 deleted_at 字段添加索引（如果不存在）
-- SQLite
CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON alerts(deleted_at);

-- PostgreSQL
-- CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON alerts(deleted_at);

-- MySQL
-- CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON alerts(deleted_at);
