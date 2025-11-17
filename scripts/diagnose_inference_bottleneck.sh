#!/bin/bash
# 诊断推理瓶颈问题

echo "=== 推理瓶颈诊断 ==="
echo ""

# 1. 检查配置
echo "【1. 检查配置】"
if [ -f "configs/config.toml" ]; then
    MAX_CONCURRENT=$(grep -A 10 "\[ai_analysis\]" configs/config.toml | grep "max_concurrent_infer" | awk -F'=' '{print $2}' | tr -d ' ')
    MAX_QUEUE=$(grep -A 10 "\[ai_analysis\]" configs/config.toml | grep "max_queue_size" | awk -F'=' '{print $2}' | tr -d ' ')
    echo "  最大并发推理数: $MAX_CONCURRENT"
    echo "  最大队列大小: $MAX_QUEUE"
else
    echo "  ✗ 配置文件不存在"
fi
echo ""

# 2. 检查队列状态
echo "【2. 检查队列状态】"
EASYDARWIN_URL="http://localhost:5066"
STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")
if [ $? -eq 0 ]; then
    QUEUE_SIZE=$(echo "$STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('queue_size', 0))" 2>/dev/null)
    QUEUE_MAX=$(echo "$STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('queue_max_size', 0))" 2>/dev/null)
    UTILIZATION=$(echo "$STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('queue_utilization', 0))" 2>/dev/null)
    SUCCESS_RATE=$(echo "$STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('success_rate_per_sec', 0))" 2>/dev/null)
    AVG_TIME=$(echo "$STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('avg_inference_ms', 0))" 2>/dev/null)
    
    echo "  当前队列大小: $QUEUE_SIZE"
    echo "  最大队列大小: $QUEUE_MAX"
    echo "  队列利用率: $(echo "$UTILIZATION * 100" | bc -l | xargs printf "%.2f")%"
    echo "  每秒成功推理数: $SUCCESS_RATE"
    echo "  平均推理时间: ${AVG_TIME}ms"
    
    # 计算理论吞吐量
    if [ ! -z "$MAX_CONCURRENT" ] && [ ! -z "$AVG_TIME" ] && [ "$AVG_TIME" != "0" ]; then
        THEORETICAL_RATE=$(echo "scale=2; $MAX_CONCURRENT * 1000 / $AVG_TIME" | bc -l)
        echo "  理论最大吞吐量: $THEORETICAL_RATE 张/秒 (基于并发数=$MAX_CONCURRENT, 平均时间=${AVG_TIME}ms)"
        ACTUAL_RATE=$(echo "$SUCCESS_RATE" | awk '{print int($1)}')
        THEORETICAL_RATE_INT=$(echo "$THEORETICAL_RATE" | awk '{print int($1)}')
        if [ "$ACTUAL_RATE" -lt "$THEORETICAL_RATE_INT" ]; then
            EFFICIENCY=$(echo "scale=2; $ACTUAL_RATE * 100 / $THEORETICAL_RATE_INT" | bc -l)
            echo "  ⚠️  实际吞吐量低于理论值，效率: ${EFFICIENCY}%"
        fi
    fi
else
    echo "  ✗ 无法获取统计信息"
fi
echo ""

# 3. 检查算法服务
echo "【3. 检查算法服务】"
SERVICES=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services")
if [ $? -eq 0 ]; then
    SERVICE_COUNT=$(echo "$SERVICES" | python3 -c "import sys, json; d=json.load(sys.stdin); print(len(d.get('services', [])))" 2>/dev/null)
    echo "  注册的算法服务数: $SERVICE_COUNT"
    
    if [ "$SERVICE_COUNT" -gt 0 ]; then
        echo "  服务详情:"
        echo "$SERVICES" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for svc in d.get('services', []):
    print(f\"    - {svc.get('service_id', 'N/A')}: {svc.get('call_count', 0)} 次调用\")
" 2>/dev/null
    else
        echo "  ⚠️  没有注册的算法服务！"
    fi
else
    echo "  ✗ 无法获取服务列表"
fi
echo ""

# 4. 检查日志中的瓶颈
echo "【4. 检查日志中的瓶颈】"
if [ -f "logs/sugar.log" ]; then
    echo "  最近10条推理相关日志:"
    tail -500 logs/sugar.log | grep -i "inference\|scheduling\|semaphore\|worker" | tail -10
    echo ""
    
    echo "  检查是否有semaphore等待:"
    SEMAPHORE_WAIT=$(tail -1000 logs/sugar.log | grep -i "semaphore_wait" | wc -l)
    if [ "$SEMAPHORE_WAIT" -gt 0 ]; then
        echo "    ⚠️  发现 $SEMAPHORE_WAIT 条semaphore等待日志，可能存在并发瓶颈"
        tail -1000 logs/sugar.log | grep -i "semaphore_wait" | tail -5
    else
        echo "    ✓ 未发现semaphore等待"
    fi
    echo ""
    
    echo "  检查是否有算法服务调用失败:"
    FAILED=$(tail -1000 logs/sugar.log | grep -i "inference.*fail\|call.*fail\|error" | wc -l)
    if [ "$FAILED" -gt 0 ]; then
        echo "    ⚠️  发现 $FAILED 条失败日志"
        tail -1000 logs/sugar.log | grep -i "inference.*fail\|call.*fail" | tail -5
    else
        echo "    ✓ 未发现调用失败"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

# 5. 分析问题
echo "【5. 问题分析】"
if [ ! -z "$QUEUE_SIZE" ] && [ ! -z "$SUCCESS_RATE" ]; then
    if [ "$QUEUE_SIZE" -gt 1000 ]; then
        echo "  ⚠️  队列积压严重 (${QUEUE_SIZE} 张图片等待处理)"
        
        if [ ! -z "$SUCCESS_RATE" ] && [ "$SUCCESS_RATE" != "0" ]; then
            ESTIMATED_TIME=$(echo "scale=0; $QUEUE_SIZE / $SUCCESS_RATE" | bc -l)
            echo "    预计需要 ${ESTIMATED_TIME} 秒才能处理完当前队列"
        fi
        
        echo ""
        echo "  可能的原因:"
        echo "    1. 并发数配置过低 (当前: $MAX_CONCURRENT)"
        echo "    2. 算法服务响应慢 (平均: ${AVG_TIME}ms)"
        echo "    3. 算法服务数量不足 (当前: $SERVICE_COUNT)"
        echo "    4. 网络延迟或连接问题"
        echo ""
        echo "  建议:"
        if [ ! -z "$MAX_CONCURRENT" ] && [ "$MAX_CONCURRENT" -lt 100 ]; then
            echo "    - 增加 max_concurrent_infer (当前: $MAX_CONCURRENT，建议: 200-500)"
        fi
        if [ ! -z "$AVG_TIME" ] && [ "$AVG_TIME" -gt 200 ]; then
            echo "    - 优化算法服务性能 (当前平均响应时间: ${AVG_TIME}ms，建议: <100ms)"
        fi
        if [ ! -z "$SERVICE_COUNT" ] && [ "$SERVICE_COUNT" -lt 10 ]; then
            echo "    - 增加算法服务实例 (当前: $SERVICE_COUNT，建议: 10-20)"
        fi
    else
        echo "  ✓ 队列状态正常"
    fi
fi
echo ""

echo "=== 诊断完成 ==="

