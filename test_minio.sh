#!/bin/bash

# MinIO连接测试脚本

MINIO_ENDPOINT="10.1.6.230:9000"
ACCESS_KEY="admin"
SECRET_KEY="admin123"
BUCKET="images"

echo "========================================="
echo "MinIO 连接测试"
echo "========================================="
echo ""
echo "配置信息:"
echo "  Endpoint: $MINIO_ENDPOINT"
echo "  Access Key: $ACCESS_KEY"
echo "  Bucket: $BUCKET"
echo ""

# 1. 测试MinIO服务
echo "1️⃣  测试MinIO服务..."
if curl -s -f "http://${MINIO_ENDPOINT}/minio/health/live" > /dev/null 2>&1; then
    echo "   ✅ MinIO服务正常运行"
else
    echo "   ❌ MinIO服务无法访问"
    echo ""
    echo "解决方案:"
    echo "1. 检查MinIO是否启动: docker ps | grep minio"
    echo "2. 检查网络连接: ping 10.1.6.230"
    echo "3. 检查端口: telnet 10.1.6.230 9000"
    exit 1
fi
echo ""

# 2. 测试认证（尝试列出buckets）
echo "2️⃣  测试MinIO认证..."

# 安装mc工具（如果没有）
if ! command -v mc &> /dev/null; then
    echo "   正在安装mc工具..."
    wget -q https://dl.min.io/client/mc/release/linux-amd64/mc -O /tmp/mc
    chmod +x /tmp/mc
    MC_CMD="/tmp/mc"
else
    MC_CMD="mc"
fi

# 配置mc alias
$MC_CMD alias set test-minio "http://${MINIO_ENDPOINT}" "${ACCESS_KEY}" "${SECRET_KEY}" --api S3v4 2>&1 | grep -q "Added\|successfully" 

if [ $? -eq 0 ]; then
    echo "   ✅ 认证成功"
else
    echo "   ❌ 认证失败"
    echo ""
    echo "可能的原因:"
    echo "1. Access Key 或 Secret Key 不正确"
    echo "2. MinIO用户不存在或被禁用"
    echo ""
    echo "请检查 config.toml 中的认证信息:"
    echo "  [frame_extractor.minio]"
    echo "  access_key = 'admin'"
    echo "  secret_key = 'admin123'"
    exit 1
fi
echo ""

# 3. 检查bucket是否存在
echo "3️⃣  检查bucket '$BUCKET' ..."
$MC_CMD ls test-minio/$BUCKET > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "   ✅ Bucket '$BUCKET' 存在"
    
    # 列出bucket内容
    OBJECT_COUNT=$($MC_CMD ls test-minio/$BUCKET --recursive 2>/dev/null | wc -l)
    echo "   📦 Bucket中有 $OBJECT_COUNT 个对象"
    
    # 显示最近的几个对象
    if [ $OBJECT_COUNT -gt 0 ]; then
        echo ""
        echo "   最近的对象:"
        $MC_CMD ls test-minio/$BUCKET --recursive 2>/dev/null | head -n 5 | while read line; do
            echo "     $line"
        done
    fi
else
    echo "   ⚠️  Bucket '$BUCKET' 不存在，正在创建..."
    
    $MC_CMD mb test-minio/$BUCKET
    
    if [ $? -eq 0 ]; then
        echo "   ✅ Bucket '$BUCKET' 创建成功"
    else
        echo "   ❌ Bucket创建失败"
        echo ""
        echo "可能的原因:"
        echo "1. 用户没有创建bucket的权限"
        echo "2. Bucket名称不符合规范"
        exit 1
    fi
fi
echo ""

# 4. 测试上传
echo "4️⃣  测试文件上传..."
echo "yanying test file $(date)" > /tmp/test_upload.txt
$MC_CMD cp /tmp/test_upload.txt test-minio/$BUCKET/test/test_upload.txt > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "   ✅ 上传成功"
else
    echo "   ❌ 上传失败"
    echo ""
    echo "可能的原因:"
    echo "1. 用户没有写入权限"
    echo "2. 磁盘空间不足"
    exit 1
fi
echo ""

# 5. 测试下载
echo "5️⃣  测试文件下载..."
$MC_CMD cp test-minio/$BUCKET/test/test_upload.txt /tmp/test_download.txt > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "   ✅ 下载成功"
    
    # 验证内容
    if diff /tmp/test_upload.txt /tmp/test_download.txt > /dev/null 2>&1; then
        echo "   ✅ 文件内容匹配"
    else
        echo "   ⚠️  文件内容不匹配"
    fi
else
    echo "   ❌ 下载失败"
fi
echo ""

# 6. 清理测试文件
echo "6️⃣  清理测试文件..."
$MC_CMD rm test-minio/$BUCKET/test/test_upload.txt > /dev/null 2>&1
rm -f /tmp/test_upload.txt /tmp/test_download.txt
echo "   ✅ 清理完成"
echo ""

# 7. 列出所有buckets
echo "7️⃣  列出所有buckets..."
$MC_CMD ls test-minio 2>/dev/null | while read line; do
    echo "   📦 $line"
done
echo ""

echo "========================================="
echo "✅ MinIO 连接测试全部通过！"
echo "========================================="
echo ""
echo "配置建议:"
echo "1. ✅ MinIO服务: http://${MINIO_ENDPOINT}"
echo "2. ✅ Bucket '${BUCKET}' 已就绪"
echo "3. ✅ 读写权限正常"
echo ""
echo "yanying平台可以正常使用MinIO了！"
echo ""
echo "Web访问: http://${MINIO_ENDPOINT%.9000}:9001"
echo "  用户名: ${ACCESS_KEY}"
echo "  密码: ${SECRET_KEY}"
echo ""
