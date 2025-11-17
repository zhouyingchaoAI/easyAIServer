#!/bin/bash
# 诊断算法服务注册问题

echo "=== 算法服务注册诊断 ==="
echo ""

# 1. 检查配置
echo "【1. 检查配置】"
if [ -f "configs/config.toml" ]; then
    echo "✓ 配置文件存在"
    echo "AI分析插件配置："
    grep -A 10 "\[ai_analysis\]" configs/config.toml | head -10
else
    echo "✗ 配置文件不存在"
fi
echo ""

# 2. 检查服务是否运行
echo "【2. 检查服务状态】"
if pgrep -f "EasyDarwin" > /dev/null; then
    echo "✓ EasyDarwin进程正在运行"
    PID=$(pgrep -f "EasyDarwin" | head -1)
    echo "  进程ID: $PID"
else
    echo "✗ EasyDarwin进程未运行"
fi
echo ""

# 3. 测试注册API
echo "【3. 测试注册API】"
EASYDARWIN_URL="http://localhost:5066"
REGISTER_RESPONSE=$(curl -s -X POST "$EASYDARWIN_URL/api/v1/ai_analysis/register" \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "test_service",
    "name": "测试服务",
    "task_types": ["人数统计"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
  }')

echo "注册响应: $REGISTER_RESPONSE"
echo ""

# 4. 检查服务列表API
echo "【4. 检查服务列表API】"
SERVICES_RESPONSE=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services")
echo "服务列表响应: $SERVICES_RESPONSE"
echo ""

# 5. 检查日志
echo "【5. 检查最近日志】"
if [ -f "logs/sugar.log" ]; then
    echo "AI分析相关日志（最近20行）："
    tail -100 logs/sugar.log | grep -i "ai analysis\|registry\|register" | tail -20
else
    echo "✗ 日志文件不存在"
fi
echo ""

# 6. 检查AI分析服务是否初始化
echo "【6. 检查AI分析服务初始化】"
if [ -f "logs/sugar.log" ]; then
    if grep -q "AI analysis plugin started successfully" logs/sugar.log; then
        echo "✓ AI分析插件已成功启动"
    else
        echo "✗ AI分析插件可能未成功启动"
        echo "查找启动相关日志："
        grep -i "ai analysis.*start\|ai analysis.*failed\|ai analysis.*error" logs/sugar.log | tail -10
    fi
else
    echo "✗ 无法检查（日志文件不存在）"
fi
echo ""

# 7. 检查Frame Extractor配置
echo "【7. 检查Frame Extractor配置】"
if [ -f "configs/config.toml" ]; then
    STORE=$(grep -A 5 "\[frame_extractor\]" configs/config.toml | grep "store" | head -1 | awk -F'=' '{print $2}' | tr -d ' "')
    if [ "$STORE" = "minio" ]; then
        echo "✓ Frame Extractor使用MinIO存储（AI分析要求）"
    else
        echo "✗ Frame Extractor未使用MinIO存储（当前: $STORE）"
        echo "  AI分析插件需要 frame_extractor.store = 'minio'"
    fi
else
    echo "✗ 无法检查（配置文件不存在）"
fi
echo ""

echo "=== 诊断完成 ==="
echo ""
echo "【可能的问题和解决方案】"
echo "1. 如果返回 'registry not ready'："
echo "   - 检查AI分析插件是否已启动"
echo "   - 检查配置文件中 [ai_analysis] enable = true"
echo "   - 检查Frame Extractor是否使用MinIO存储"
echo ""
echo "2. 如果返回 'AI analysis service not ready'："
echo "   - 检查服务是否正在运行"
echo "   - 检查日志中的错误信息"
echo ""
echo "3. 如果服务未启动："
echo "   - 检查配置文件中的 enable = true"
echo "   - 检查MinIO连接是否正常"
echo "   - 检查消息队列连接是否正常（如果配置了）"

