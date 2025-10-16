#!/bin/bash

# yanying AI服务演示脚本
# 功能：注册服务并保持心跳

YANYING_HOST="http://localhost:5066"
SERVICE_ID="demo_people_counter"
SERVICE_NAME="演示-人数统计服务"

echo "========================================="
echo "yanying AI服务演示"
echo "========================================="
echo ""

# 1. 注册服务
echo "📝 正在注册服务..."
REGISTER_RESPONSE=$(curl -s -X POST ${YANYING_HOST}/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "'${SERVICE_ID}'",
    "name": "'${SERVICE_NAME}'",
    "task_types": ["人数统计", "客流分析"],
    "endpoint": "http://localhost:8001/infer",
    "version": "1.0.0"
  }')

echo "注册响应: $REGISTER_RESPONSE"

if echo "$REGISTER_RESPONSE" | grep -q '"ok":true'; then
    echo "✅ 服务注册成功！"
    echo ""
    
    # 2. 查询服务列表
    echo "📋 查询服务列表..."
    curl -s ${YANYING_HOST}/api/v1/ai_analysis/services | python3 -m json.tool
    echo ""
    
    # 3. 开始心跳循环
    echo "💓 开始心跳循环（每30秒）..."
    echo "按 Ctrl+C 停止"
    echo ""
    
    COUNTER=1
    while true; do
        sleep 30
        
        HEARTBEAT_RESPONSE=$(curl -s -X POST ${YANYING_HOST}/api/v1/ai_analysis/heartbeat/${SERVICE_ID})
        
        if echo "$HEARTBEAT_RESPONSE" | grep -q '"ok":true'; then
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] 心跳 #${COUNTER} ✅"
        else
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] 心跳 #${COUNTER} ❌ - ${HEARTBEAT_RESPONSE}"
        fi
        
        COUNTER=$((COUNTER + 1))
    done
else
    echo "❌ 服务注册失败: $REGISTER_RESPONSE"
    exit 1
fi

