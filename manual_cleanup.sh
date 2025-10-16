#!/bin/bash

# 手动清理MinIO旧图片脚本

BUCKET="test-minio/images"
DAYS=${1:-7}  # 默认清理7天前的图片

echo "========================================="
echo "MinIO图片手动清理工具"
echo "========================================="
echo ""
echo "Bucket: $BUCKET"
echo "保留天数: $DAYS 天"
echo ""

# 查看当前存储
echo "当前存储使用:"
/tmp/mc du $BUCKET
echo ""

# 查询要删除的文件
echo "查找 ${DAYS} 天前的图片..."
OLD_FILES=$(/tmp/mc find $BUCKET --older-than ${DAYS}d)
COUNT=$(echo "$OLD_FILES" | grep -v "^$" | wc -l)

if [ "$COUNT" -eq 0 ]; then
    echo "✅ 没有需要清理的文件"
    exit 0
fi

echo "找到 $COUNT 个旧文件"
echo ""

# 确认删除
read -p "确认删除这些文件？(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "取消删除"
    exit 0
fi

# 执行删除
echo ""
echo "正在删除..."
/tmp/mc find $BUCKET --older-than ${DAYS}d --exec "mc rm {}"

echo ""
echo "✅ 清理完成"
echo ""

# 查看清理后的存储
echo "清理后存储使用:"
/tmp/mc du $BUCKET
echo ""

echo "========================================="
echo "建议设置自动清理："
echo "/tmp/mc ilm add $BUCKET --expiry-days $DAYS"
echo "========================================="

