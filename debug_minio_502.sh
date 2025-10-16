#!/bin/bash

# MinIO 502错误深度调试脚本

echo "========================================="
echo "MinIO 502错误深度调试"
echo "========================================="
echo ""

MINIO_ENDPOINT="10.1.6.230:9000"
BUCKET="images"

# 1. 测试MinIO健康检查
echo "1. 测试MinIO健康检查..."
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://$MINIO_ENDPOINT/minio/health/live")
echo "   状态码: $HEALTH"
if [ "$HEALTH" == "200" ]; then
    echo "   ✅ 健康检查正常"
else
    echo "   ❌ 健康检查失败"
fi
echo ""

# 2. 测试ListObjects API (S3 v2)
echo "2. 测试ListObjects API (S3 v2)..."
RESPONSE=$(curl -s -w "\n%{http_code}" "http://$MINIO_ENDPOINT/$BUCKET?max-keys=1")
HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "   状态码: $HTTP_CODE"
if [ "$HTTP_CODE" == "200" ]; then
    echo "   ✅ ListObjects API正常"
    echo "   响应: $(echo $BODY | head -c 200)..."
elif [ "$HTTP_CODE" == "502" ]; then
    echo "   ❌ 返回502 Bad Gateway"
    echo "   这就是问题所在！"
else
    echo "   状态码: $HTTP_CODE"
    echo "   响应: $BODY"
fi
echo ""

# 3. 测试不同的API endpoint
echo "3. 测试bucket是否可访问..."
for api_path in "/$BUCKET" "/$BUCKET/" "/$BUCKET?list-type=2" ; do
    echo "   测试: http://$MINIO_ENDPOINT$api_path"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://$MINIO_ENDPOINT$api_path")
    echo "   状态码: $STATUS"
done
echo ""

# 4. 检查是否有反向代理
echo "4. 检查网络路径..."
echo "   追踪路由:"
traceroute -m 5 -w 1 10.1.6.230 2>&1 | head -5
echo ""

# 5. 检查MinIO版本
echo "5. 获取MinIO服务器信息..."
SERVER_HEADER=$(curl -s -I "http://$MINIO_ENDPOINT/minio/health/live" | grep -i "^Server:")
echo "   $SERVER_HEADER"
echo ""

# 6. 测试使用AWS签名的请求
echo "6. 测试认证请求..."
# 这需要AWS签名，暂时跳过
echo "   (需要AWS签名工具)"
echo ""

# 7. 可能的解决方案
echo "========================================="
echo "可能的问题和解决方案:"
echo "========================================="
echo ""

if [ "$HTTP_CODE" == "502" ]; then
    echo "❌ MinIO返回502，可能原因:"
    echo ""
    echo "1. MinIO后面有反向代理(Nginx/HAProxy)配置问题"
    echo "   解决: 检查代理配置或直连MinIO"
    echo ""
    echo "2. MinIO版本太老，不支持某些API"
    echo "   解决: 升级MinIO到最新版本"
    echo ""
    echo "3. MinIO配置了错误的域名/网关"
    echo "   解决: 检查MinIO启动参数"
    echo ""
    echo "4. 网络设备（负载均衡器）干扰"
    echo "   解决: 使用内网直连地址"
    echo ""
    echo "临时解决方案:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "方案A: 在同一台机器上部署MinIO"
    echo "  endpoint = 'localhost:9000'"
    echo ""
    echo "方案B: 使用MinIO的域名而不是IP"
    echo "  endpoint = 'minio.yourdomain.com:9000'"
    echo ""
    echo "方案C: 暂时禁用AI分析，使用本地存储"
    echo "  [frame_extractor]"
    echo "  store = 'local'"
    echo "  [ai_analysis]"
    echo "  enable = false"
    echo ""
fi

echo "========================================="
echo "调试完成"
echo "========================================="

