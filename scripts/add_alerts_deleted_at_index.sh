#!/bin/bash

# 为 alerts 表的 deleted_at 字段添加索引
# 解决 COUNT 查询全表扫描的性能问题

set -e

DB_PATH="${1:-configs/data.db}"

if [ ! -f "$DB_PATH" ]; then
    echo "错误: 数据库文件不存在: $DB_PATH"
    echo "用法: $0 [数据库路径]"
    exit 1
fi

echo "=========================================="
echo "为 alerts 表的 deleted_at 字段添加索引"
echo "=========================================="
echo ""
echo "数据库: $DB_PATH"
echo ""

# 检查索引是否已存在
INDEX_EXISTS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_alerts_deleted_at';" 2>/dev/null || echo "0")

if [ "$INDEX_EXISTS" = "1" ]; then
    echo "✓ 索引 idx_alerts_deleted_at 已存在"
    echo ""
    echo "验证索引信息:"
    sqlite3 "$DB_PATH" ".schema idx_alerts_deleted_at" || echo "  (无法显示索引详情)"
else
    echo "创建索引..."
    sqlite3 "$DB_PATH" << EOF
-- 创建 deleted_at 字段索引
CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON alerts(deleted_at);

-- 验证索引创建
SELECT 
    name, 
    sql 
FROM sqlite_master 
WHERE type='index' AND name='idx_alerts_deleted_at';
EOF

    if [ $? -eq 0 ]; then
        echo ""
        echo "✓ 索引创建成功!"
    else
        echo ""
        echo "✗ 索引创建失败"
        exit 1
    fi
fi

echo ""
echo "=========================================="
echo "索引统计信息"
echo "=========================================="

# 获取表信息
sqlite3 "$DB_PATH" << EOF
-- 显示索引列表
SELECT 
    name as index_name,
    tbl_name as table_name
FROM sqlite_master 
WHERE type='index' AND tbl_name='alerts'
ORDER BY name;
EOF

echo ""
echo "=========================================="
echo "性能验证"
echo "=========================================="

# 检查表数据量
TOTAL_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM alerts;" 2>/dev/null || echo "0")
DELETED_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM alerts WHERE deleted_at IS NOT NULL;" 2>/dev/null || echo "0")
ACTIVE_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM alerts WHERE deleted_at IS NULL;" 2>/dev/null || echo "0")

echo "总记录数: $TOTAL_COUNT"
echo "已删除记录数: $DELETED_COUNT"
echo "活跃记录数: $ACTIVE_COUNT"
echo ""

if [ "$TOTAL_COUNT" -gt 1000 ]; then
    echo "⚠ 数据量较大 ($TOTAL_COUNT 条记录)，建议验证索引是否生效"
    echo ""
    echo "验证方法:"
    echo "  sqlite3 $DB_PATH \"EXPLAIN QUERY PLAN SELECT COUNT(*) FROM alerts WHERE deleted_at IS NULL;\""
    echo ""
    echo "如果看到 'USING INDEX idx_alerts_deleted_at'，说明索引已生效"
fi

echo ""
echo "=========================================="
echo "✓ 完成"
echo "=========================================="
