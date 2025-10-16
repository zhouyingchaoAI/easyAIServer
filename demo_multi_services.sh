#!/bin/bash

# yanying 多服务演示脚本
# 演示多个算法服务同时运行

YANYING_HOST="http://localhost:5066"

echo "========================================="
echo "yanying 多算法服务演示"
echo "========================================="
echo ""

# 服务配置
declare -A SERVICES=(
    ["people_counter"]="人数统计服务:人数统计,客流分析:8001"
    ["helmet_detector"]="安全帽检测服务:安全帽检测,施工安全:8002"
    ["fall_detector"]="跌倒检测服务:人员跌倒,老人监护:8003"
    ["smoke_detector"]="吸烟检测服务:吸烟检测,禁烟区监控:8004"
)

# 注册所有服务
echo "📝 正在注册所有服务..."
echo ""

for SERVICE_ID in "${!SERVICES[@]}"; do
    IFS=':' read -ra INFO <<< "${SERVICES[$SERVICE_ID]}"
    NAME="${INFO[0]}"
    TYPES="${INFO[1]}"
    PORT="${INFO[2]}"
    
    # 构建task_types JSON数组
    TASK_TYPES_JSON=$(echo $TYPES | sed 's/,/","/g' | sed 's/^/"/' | sed 's/$/"/')
    
    echo "注册服务: $NAME (ID: $SERVICE_ID)"
    
    RESPONSE=$(curl -s -X POST ${YANYING_HOST}/api/v1/ai_analysis/register \
      -H "Content-Type: application/json" \
      -d '{
        "service_id": "'${SERVICE_ID}'",
        "name": "'${NAME}'",
        "task_types": ['${TASK_TYPES_JSON}'],
        "endpoint": "http://localhost:'${PORT}'/infer",
        "version": "1.0.0"
      }')
    
    if echo "$RESPONSE" | grep -q '"ok":true'; then
        echo "  ✅ 注册成功"
    else
        echo "  ❌ 注册失败: $RESPONSE"
    fi
    echo ""
done

# 查询服务列表
echo "========================================="
echo "📋 已注册的服务列表："
echo "========================================="
curl -s ${YANYING_HOST}/api/v1/ai_analysis/services | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f\"总计: {data['total']} 个服务\n\")
for svc in data.get('services', []):
    print(f\"服务ID: {svc['service_id']}\")
    print(f\"  名称: {svc['name']}\")
    print(f\"  任务类型: {', '.join(svc['task_types'])}\")
    print(f\"  端点: {svc['endpoint']}\")
    print(f\"  版本: {svc['version']}\")
    print()
"

echo ""
echo "========================================="
echo "💓 开始心跳循环（每30秒）..."
echo "按 Ctrl+C 停止"
echo "========================================="
echo ""

COUNTER=1
while true; do
    sleep 30
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 发送第 ${COUNTER} 轮心跳..."
    
    for SERVICE_ID in "${!SERVICES[@]}"; do
        RESPONSE=$(curl -s -X POST ${YANYING_HOST}/api/v1/ai_analysis/heartbeat/${SERVICE_ID})
        
        if echo "$RESPONSE" | grep -q '"ok":true'; then
            echo "  ${SERVICE_ID}: ✅"
        else
            echo "  ${SERVICE_ID}: ❌"
        fi
    done
    
    echo ""
    COUNTER=$((COUNTER + 1))
done

