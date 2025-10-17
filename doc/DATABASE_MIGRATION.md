# 数据库迁移说明

## 添加 detection_count 字段

如果你之前运行过该系统，需要手动添加 `detection_count` 字段到 alerts 表。

### SQLite (默认数据库)

```bash
# 连接到数据库
sqlite3 configs/data.db

# 添加字段
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;

# 为已存在的记录更新字段（可选）
UPDATE alerts SET detection_count = 0 WHERE detection_count IS NULL;

# 添加索引以提高查询性能
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);

# 退出
.exit
```

### PostgreSQL

```sql
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
```

### MySQL

```sql
ALTER TABLE alerts ADD COLUMN detection_count INT DEFAULT 0;
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
```

## 自动迁移

如果你删除 `configs/data.db` 文件后重启服务，GORM 会自动创建包含新字段的表结构。

**注意：** 这会丢失所有现有数据！

```bash
# 备份数据库（可选）
cp configs/data.db configs/data.db.backup

# 删除数据库
rm configs/data.db

# 重启服务
./easydarwin
```

## 验证

启动服务后，检查日志确认迁移成功：

```bash
tail -f logs/sugar.log
```

或者通过 API 检查：

```bash
curl http://localhost:5066/api/v1/ai_analysis/alerts?page=1&page_size=10
```

返回的数据应该包含 `detection_count` 字段。

