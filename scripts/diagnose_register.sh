#!/bin/bash
# 诊断算法服务注册问题

EASYDARWIN_URL="${1:-http://localhost:5066}"

echo "=== 算法服务注册诊断 ==="
echo "EasyDarwin地址: $EASYDARWIN_URL"
echo ""

echo "【1. 检查服务状态】"
echo "----------------------------------------"
STATUS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats" 2>/dev/null)
if [ $? -eq 0 ]; then
    echo "✅ AI分析服务已启动"
    echo "$STATUS" | python3 -m json.tool 2>/dev/null || echo "$STATUS"
else
    echo "❌ AI分析服务未启动或无法访问"
    echo "   错误: 无法连接到 $EASYDARWIN_URL"
fi
echo ""

echo "【2. 检查注册中心状态】"
echo "----------------------------------------"
SERVICES=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services" 2>/dev/null)
if [ $? -eq 0 ]; then
    echo "✅ 注册中心可访问"
    echo "$SERVICES" | python3 -m json.tool 2>/dev/null || echo "$SERVICES"
else
    echo "❌ 注册中心不可访问"
    echo "   可能原因："
    echo "   - AI分析服务未启动"
    echo "   - Registry未初始化"
fi
echo ""

echo "【3. 尝试注册测试服务】"
echo "----------------------------------------"
TEST_DATA='{
  "service_id": "test_service_'$(date +%s)'",
  "name": "测试服务",
  "task_types": ["人数统计"],
  "endpoint": "http://127.0.0.1:8000/infer",
  "version": "1.0.0"
}'

REGISTER_RESPONSE=$(curl -s -X POST "$EASYDARWIN_URL/api/v1/ai_analysis/register" \
  -H "Content-Type: application/json" \
  -d "$TEST_DATA" 2>/dev/null)

if [ $? -eq 0 ]; then
    echo "注册响应:"
    echo "$REGISTER_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$REGISTER_RESPONSE"
    
    if echo "$REGISTER_RESPONSE" | grep -q '"ok".*true'; then
        echo "✅ 注册成功"
    elif echo "$REGISTER_RESPONSE" | grep -q "registry not ready"; then
        echo "❌ 注册失败: registry not ready"
        echo ""
        echo "可能原因："
        echo "1. AI分析插件启动失败"
        echo "2. MinIO连接失败"
        echo "3. 配置文件错误"
        echo ""
        echo "检查方法："
        echo "1. 查看日志: tail -100 logs/*.log | grep -iE 'AI analysis|registry|minio'"
        echo "2. 检查配置: cat configs/config.toml | grep -A 10 '\[ai_analysis\]'"
        echo "3. 检查MinIO: curl -s http://172.16.5.207:9000/minio/health/live"
    elif echo "$REGISTER_RESPONSE" | grep -q "AI analysis service not ready"; then
        echo "❌ 注册失败: AI analysis service not ready"
        echo ""
        echo "可能原因："
        echo "1. AI分析插件未启动"
        echo "2. 插件启动失败"
        echo ""
        echo "检查方法："
        echo "1. 查看日志: tail -100 logs/*.log | grep -iE 'AI analysis|start failed'"
        echo "2. 检查配置: cat configs/config.toml | grep -A 5 '\[ai_analysis\]'"
    else
        echo "❌ 注册失败"
        echo "响应: $REGISTER_RESPONSE"
    fi
else
    echo "❌ 无法连接到EasyDarwin服务"
    echo "   请检查："
    echo "   1. 服务是否运行: ps aux | grep easydarwin"
    echo "   2. 端口是否正确: netstat -tlnp | grep 5066"
    echo "   3. 防火墙设置"
fi
echo ""

echo "【4. 检查配置】"
echo "----------------------------------------"
if [ -f "configs/config.toml" ]; then
    echo "AI分析配置:"
    grep -A 10 '\[ai_analysis\]' configs/config.toml 2>/dev/null | head -15
    echo ""
    echo "Frame Extractor配置:"
    grep -A 5 '\[frame_extractor\]' configs/config.toml 2>/dev/null | head -10
else
    echo "⚠️  配置文件不存在: configs/config.toml"
fi
echo ""

echo "【5. 检查日志（最近相关错误）】"
echo "----------------------------------------"
if [ -d "logs" ]; then
    echo "最近的AI分析相关日志:"
    tail -50 logs/*.log 2>/dev/null | grep -iE "AI analysis|registry|register|minio" | tail -10
else
    echo "⚠️  日志目录不存在: logs/"
fi
echo ""

echo "=== 诊断完成 ==="
echo ""
echo "💡 常见问题解决方案："
echo "1. 如果显示 'registry not ready':"
echo "   - 检查AI分析插件是否启动成功"
echo "   - 检查MinIO连接是否正常"
echo "   - 查看启动日志中的错误信息"
echo ""
echo "2. 如果显示 'AI analysis service not ready':"
echo "   - 确认配置文件中 [ai_analysis] enable = true"
echo "   - 检查插件启动日志"
echo ""
echo "3. 如果无法连接服务:"
echo "   - 确认服务正在运行"
echo "   - 检查端口是否正确（默认5066）"
echo "   - 检查防火墙设置"

