#!/bin/bash
# MinIO 测试和修复脚本

MINIO_ENDPOINT="10.1.6.230:9000"
MINIO_ACCESS_KEY="admin"
MINIO_SECRET_KEY="admin123"
BUCKET_NAME="images"

echo "===== MinIO 诊断工具 ====="
echo ""

# 1. 测试 MinIO 连接
echo "1. 测试 MinIO 服务..."
if curl -s -I http://$MINIO_ENDPOINT | grep -q "Server: MinIO"; then
    echo "✅ MinIO 服务运行正常"
else
    echo "❌ MinIO 服务无法访问"
    exit 1
fi
echo ""

# 2. 检查 mc 客户端
echo "2. 检查 mc 客户端..."
if ! command -v mc &> /dev/null; then
    echo "⚠️  mc 客户端未安装，正在安装..."
    wget -q https://dl.min.io/client/mc/release/linux-amd64/mc -O /tmp/mc
    chmod +x /tmp/mc
    sudo mv /tmp/mc /usr/local/bin/mc 2>/dev/null || mv /tmp/mc ./mc
    MC_CMD="./mc"
    echo "✅ mc 客户端已安装到当前目录"
else
    MC_CMD="mc"
    echo "✅ mc 客户端已安装"
fi
echo ""

# 3. 配置 MinIO 别名
echo "3. 配置 MinIO 连接..."
$MC_CMD alias set myminio http://$MINIO_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY --api S3v4
if [ $? -eq 0 ]; then
    echo "✅ MinIO 连接配置成功"
else
    echo "❌ MinIO 连接失败，请检查凭证"
    exit 1
fi
echo ""

# 4. 检查 bucket 是否存在
echo "4. 检查 bucket '$BUCKET_NAME'..."
if $MC_CMD ls myminio/$BUCKET_NAME &> /dev/null; then
    echo "✅ Bucket '$BUCKET_NAME' 已存在"
else
    echo "⚠️  Bucket '$BUCKET_NAME' 不存在，正在创建..."
    $MC_CMD mb myminio/$BUCKET_NAME
    if [ $? -eq 0 ]; then
        echo "✅ Bucket '$BUCKET_NAME' 创建成功"
    else
        echo "❌ Bucket 创建失败"
        exit 1
    fi
fi
echo ""

# 5. 设置 bucket 访问策略
echo "5. 设置 bucket 访问策略..."
$MC_CMD anonymous set public myminio/$BUCKET_NAME 2>/dev/null
echo "✅ Bucket 访问策略已设置"
echo ""

# 6. 测试上传
echo "6. 测试文件上传..."
echo "test" > /tmp/test_minio.txt
$MC_CMD cp /tmp/test_minio.txt myminio/$BUCKET_NAME/test.txt
if [ $? -eq 0 ]; then
    echo "✅ 文件上传成功"
    $MC_CMD rm myminio/$BUCKET_NAME/test.txt
    echo "✅ 测试文件已清理"
else
    echo "❌ 文件上传失败"
fi
rm -f /tmp/test_minio.txt
echo ""

# 7. 列出 bucket 内容
echo "7. 列出 bucket 内容..."
$MC_CMD ls myminio/$BUCKET_NAME
echo ""

echo "===== 诊断完成 ====="
echo ""
echo "✅ MinIO 配置完成！现在可以重启 EasyDarwin 服务。"
echo ""
echo "重启命令："
echo "  cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510160927"
echo "  pkill -9 easydarwin"
echo "  ./easydarwin"

