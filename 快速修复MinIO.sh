#!/bin/bash
# 快速修复 MinIO 502 问题

echo "🔧 开始修复 MinIO 连接问题..."
echo ""

# 1. 停止所有 easydarwin 进程
echo "1️⃣  停止旧进程..."
pkill -f easydarwin
sleep 2
echo "   ✅ 已停止"

# 2. 检查 MinIO 连接
echo ""
echo "2️⃣  测试 MinIO 连接..."
curl -s http://10.1.6.230:9000/minio/health/live > /dev/null
if [ $? -eq 0 ]; then
    echo "   ✅ MinIO 服务正常"
else
    echo "   ❌ MinIO 服务无法访问"
    echo "   请检查: http://10.1.6.230:9000"
    exit 1
fi

# 3. 确认 bucket 权限
echo ""
echo "3️⃣  检查 bucket 权限..."
PERM=$(/tmp/mc anonymous get test-minio/images 2>&1 | grep -o "public\|private\|download")
if [ "$PERM" = "public" ]; then
    echo "   ✅ Bucket 权限正常 (public)"
else
    echo "   ⚠️  设置 bucket 为 public..."
    /tmp/mc anonymous set public test-minio/images
    echo "   ✅ 权限已设置"
fi

# 4. 清理日志
echo ""
echo "4️⃣  清理旧日志..."
find /code/EasyDarwin/build -name "*.log" -type f -mtime +1 -delete 2>/dev/null
echo "   ✅ 日志已清理"

echo ""
echo "════════════════════════════════════════════"
echo "✅ 修复完成！现在可以启动服务："
echo ""
echo "   ./easydarwin"
echo ""
echo "════════════════════════════════════════════"
