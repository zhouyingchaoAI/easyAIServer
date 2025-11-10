#!/bin/bash
# 彻底调试算法服务注册问题
# 在宿主机上运行

echo "🔍 开始彻底调试算法服务注册问题"
echo "========================================"
echo ""

EASYDARWIN_URL="http://172.16.5.207:5066"

# 1. 检查EasyDarwin进程
echo "步骤 1/8: 检查 EasyDarwin 进程"
echo "---"
ps aux | grep easydarwin | grep -v grep
EASYDARWIN_PID=$(ps aux | grep easydarwin | grep -v grep | awk '{print $2}' | head -1)
if [ ! -z "$EASYDARWIN_PID" ]; then
    echo "PID: $EASYDARWIN_PID"
    echo "启动时间: $(ps -p $EASYDARWIN_PID -o lstart= 2>/dev/null || echo '无法获取')"
else
    echo "❌ EasyDarwin 未运行"
fi
echo ""

# 2. 检查心跳相关进程
echo "步骤 2/8: 检查心跳维持脚本"
echo "---"
HEARTBEAT_PROCS=$(ps aux | grep -E "heartbeat|maintain" | grep -v grep)
if [ -z "$HEARTBEAT_PROCS" ]; then
    echo "✅ 无心跳脚本运行"
else
    echo "⚠️  发现心跳进程:"
    echo "$HEARTBEAT_PROCS"
fi
echo ""

# 3. 检查实际运行的算法服务
echo "步骤 3/8: 检查实际运行的算法服务 (7901-7914)"
echo "---"
RUNNING_SERVICES=0
for port in {7901..7914}; do
    response=$(curl -s -m 1 http://172.16.5.207:$port/health 2>&1)
    if echo "$response" | grep -q "service_id"; then
        service_id=$(echo "$response" | grep -o '"service_id":"[^"]*"' | cut -d'"' -f4)
        echo "✅ 端口 $port: $service_id"
        ((RUNNING_SERVICES++))
    fi
done
echo "运行中的服务: $RUNNING_SERVICES 个"
echo ""

# 4. 检查平台注册的服务
echo "步骤 4/8: 检查平台注册的服务"
echo "---"
SERVICES_DATA=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services" 2>/dev/null)
REGISTERED_COUNT=$(echo "$SERVICES_DATA" | grep -o '"total":[0-9]*' | grep -o '[0-9]*')
echo "平台注册的服务: $REGISTERED_COUNT 个"
echo ""
if [ "$REGISTERED_COUNT" -gt 0 ]; then
    echo "详细列表:"
    echo "$SERVICES_DATA" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for s in data.get('services', []):
        port = s.get('endpoint', '').split(':')[-1].split('/')[0]
        print(f\"  - {s.get('service_id')}: 端口{port}, 调用{s.get('call_count', 0)}次\")
except: pass
" 2>/dev/null || echo "$SERVICES_DATA"
fi
echo ""

# 5. 检查数据库中是否有算法服务记录
echo "步骤 5/8: 检查数据库"
echo "---"
if [ -f /code/EasyDarwin/configs/data.db ]; then
    TABLES=$(sqlite3 /code/EasyDarwin/configs/data.db ".tables" 2>/dev/null | grep -i "algorithm\|service")
    if [ -z "$TABLES" ]; then
        echo "✅ 数据库中无算法服务相关表"
    else
        echo "⚠️  发现表: $TABLES"
        sqlite3 /code/EasyDarwin/configs/data.db "SELECT * FROM $TABLES LIMIT 5;" 2>/dev/null || echo "无法读取"
    fi
else
    echo "⚠️  数据库文件不存在"
fi
echo ""

# 6. 检查配置文件中是否有算法服务
echo "步骤 6/8: 检查配置文件"
echo "---"
ALGO_IN_CONFIG=$(grep -i "algorithm.*service\|head_detector" /code/EasyDarwin/configs/config.toml 2>/dev/null)
if [ -z "$ALGO_IN_CONFIG" ]; then
    echo "✅ 配置文件中无算法服务配置"
else
    echo "⚠️  配置文件中发现:"
    echo "$ALGO_IN_CONFIG"
fi
echo ""

# 7. 检查日志中的注册记录
echo "步骤 7/8: 检查最近的注册日志"
echo "---"
LOG_FILE=$(find /code/EasyDarwin/build -name "*.log" -type f 2>/dev/null | grep -E "sugar|202511" | head -1)
if [ ! -z "$LOG_FILE" ] && [ -f "$LOG_FILE" ]; then
    echo "日志文件: $LOG_FILE"
    RECENT_REGISTER=$(tail -200 "$LOG_FILE" | grep "registered successfully" | tail -5)
    if [ -z "$RECENT_REGISTER" ]; then
        echo "近期无注册记录"
    else
        echo "最近5次注册:"
        echo "$RECENT_REGISTER" | while read line; do
            echo "  $line"
        done
    fi
else
    echo "未找到日志文件"
fi
echo ""

# 8. 分析结论
echo "步骤 8/8: 分析结论"
echo "======================================"
echo ""

if [ "$REGISTERED_COUNT" -gt 0 ] && [ "$RUNNING_SERVICES" -eq 0 ]; then
    echo "🔴 问题：平台有 $REGISTERED_COUNT 个注册，但没有实际服务运行"
    echo ""
    echo "可能原因："
    echo "  1. 有心跳脚本在维持虚假注册"
    echo "  2. 平台没有真正重启"
    echo "  3. 有其他进程在持续调用注册API"
    echo ""
    echo "解决方案："
    echo "  方法1: 强制重启平台"
    echo "    pkill -9 easydarwin"
    echo "    sleep 3"
    echo "    ./easydarwin.com"
    echo ""
    echo "  方法2: 使用 clear_all API（需要新版本）"
    echo "    curl -X POST http://localhost:5066/api/v1/ai_analysis/clear_all"
    echo ""
elif [ "$REGISTERED_COUNT" -eq "$RUNNING_SERVICES" ] && [ "$RUNNING_SERVICES" -gt 0 ]; then
    echo "✅ 正常：注册数量与实际服务数量一致"
    echo "   注册: $REGISTERED_COUNT 个"
    echo "   运行: $RUNNING_SERVICES 个"
else
    echo "⚠️  不匹配："
    echo "   平台注册: $REGISTERED_COUNT 个"
    echo "   实际运行: $RUNNING_SERVICES 个"
    echo ""
    echo "建议：让实际服务重新注册"
fi

echo ""
echo "======================================"
echo "调试完成"
echo "======================================"

