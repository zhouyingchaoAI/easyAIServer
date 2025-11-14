#!/bin/bash
# 推理慢问题诊断脚本

EASYDARWIN_URL="${1:-http://localhost:5066}"
LOG_FILE="${2:-$(find build -name "*.log" -type f 2>/dev/null | head -1)}"

echo "=== 推理性能问题诊断 ==="
echo "EasyDarwin地址: $EASYDARWIN_URL"
echo "日志文件: $LOG_FILE"
echo ""

echo "【1. 队列状态】"
echo "----------------------------------------"
STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")
echo "$STATS" | python3 -m json.tool
echo ""

QUEUE_SIZE=$(echo "$STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['queue_size'])")
QUEUE_MAX=$(echo "$STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['queue_max_size'])")
UTILIZATION=$(echo "$STATS" | python3 -c "import sys, json; print(json.load(sys.stdin)['queue_utilization'])")

if (( $(echo "$UTILIZATION > 0.9" | bc -l) )); then
    echo "⚠️  队列使用率过高: $(echo "$UTILIZATION * 100" | bc -l | xargs printf "%.2f")%"
    echo "   问题：队列积压严重，图片等待时间过长"
fi
echo ""

echo "【2. 算法服务状态】"
echo "----------------------------------------"
SERVICES=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services")
TOTAL=$(echo "$SERVICES" | python3 -c "import sys, json; print(json.load(sys.stdin)['total'])")
echo "已注册服务数: $TOTAL"

# 计算平均响应时间
AVG_RESPONSE=$(echo "$SERVICES" | python3 -c "
import sys, json
data = json.load(sys.stdin)
services = data.get('services', [])
if services:
    total = sum(s.get('call_count', 0) for s in services)
    if total > 0:
        weighted_avg = sum(s.get('call_count', 0) * 100 for s in services) / total
        print(f'{weighted_avg:.0f}')
    else:
        print('0')
else:
    print('0')
")

echo "平均调用次数: $(echo "$SERVICES" | python3 -c "import sys, json; s=json.load(sys.stdin)['services']; print(sum(x['call_count'] for x in s) // len(s) if s else 0)")"
echo ""

echo "【3. 负载均衡信息】"
echo "----------------------------------------"
LB_INFO=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/load_balance/info")
echo "$LB_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
lb = data.get('load_balance', {})
for task_type, info in lb.items():
    print(f'任务类型: {task_type}')
    print(f'  服务数: {info[\"total_services\"]}')
    services = info.get('services', [])
    if services:
        avg_resp = sum(s.get('avg_response_ms', 0) for s in services) / len(services)
        print(f'  平均响应时间: {avg_resp:.0f}ms')
        max_resp = max(s.get('avg_response_ms', 0) for s in services)
        min_resp = min(s.get('avg_response_ms', 0) for s in services)
        print(f'  最快: {min_resp}ms, 最慢: {max_resp}ms')
"
echo ""

echo "【4. 日志分析（最近1000行）】"
echo "----------------------------------------"
if [ -n "$LOG_FILE" ] && [ -f "$LOG_FILE" ]; then
    echo "4.1 图片丢失统计："
    LOST_COUNT=$(tail -1000 "$LOG_FILE" | grep -c "image not found in MinIO")
    echo "   图片丢失次数: $LOST_COUNT"
    if [ "$LOST_COUNT" -gt 100 ]; then
        echo "   ⚠️  图片丢失过多，说明队列积压导致图片被清理"
    fi
    echo ""
    
    echo "4.2 推理耗时统计（最近成功推理）："
    tail -10000 "$LOG_FILE" | grep "algorithm_call_duration_ms" | grep -v "0ms\|0.00ms" | tail -20 | python3 -c "
import sys
import re
import json

durations = []
for line in sys.stdin:
    # 尝试提取duration值
    match = re.search(r'algorithm_call_duration_ms[":\s]+([0-9.]+)', line)
    if match:
        try:
            val = float(match.group(1))
            if val > 0:
                durations.append(val)
        except:
            pass

if durations:
    print(f'   样本数: {len(durations)}')
    print(f'   平均: {sum(durations)/len(durations):.2f}ms')
    print(f'   最大: {max(durations):.2f}ms')
    print(f'   最小: {min(durations):.2f}ms')
else:
    print('   ⚠️  未找到有效的推理耗时数据')
    print('   可能原因：')
    print('   1. 推理都失败了')
    print('   2. 日志级别不够（需要Debug级别）')
    print('   3. 推理还未完成')
"
    echo ""
    
    echo "4.3 错误统计："
    tail -1000 "$LOG_FILE" | grep -iE "error|failed|timeout" | grep -v "image not found" | tail -10
    echo ""
else
    echo "⚠️  未找到日志文件"
fi

echo "【5. 问题诊断】"
echo "----------------------------------------"
echo "可能原因："
echo ""

# 检查队列积压
if (( $(echo "$UTILIZATION > 0.9" | bc -l) )); then
    echo "1. ❌ 队列积压严重（使用率 > 90%）"
    echo "   解决方案："
    echo "   - 增加 max_concurrent_infer（当前300，可尝试增加到500）"
    echo "   - 增加 max_queue_size（当前50000，可尝试增加到100000）"
    echo "   - 检查算法服务响应时间是否过慢"
    echo ""
fi

# 检查图片丢失
if [ -n "$LOG_FILE" ] && [ -f "$LOG_FILE" ]; then
    LOST_COUNT=$(tail -1000 "$LOG_FILE" | grep -c "image not found in MinIO")
    if [ "$LOST_COUNT" -gt 100 ]; then
        echo "2. ❌ 图片丢失过多（$LOST_COUNT 次）"
        echo "   原因：图片在队列中等待时间过长，被清理机制删除"
        echo "   解决方案："
        echo "   - 提高推理并发数"
        echo "   - 减少抽帧频率"
        echo "   - 增加队列大小"
        echo ""
    fi
fi

# 检查算法服务
if [ "$TOTAL" -lt 5 ]; then
    echo "3. ⚠️  算法服务数量较少（$TOTAL 个）"
    echo "   建议：增加算法服务实例"
    echo ""
fi

echo "【6. 优化建议】"
echo "----------------------------------------"
echo "1. 增加并发推理数："
echo "   编辑 configs/config.toml"
echo "   max_concurrent_infer = 500  # 从300增加到500"
echo ""
echo "2. 增加队列大小："
echo "   max_queue_size = 100000  # 从50000增加到100000"
echo ""
echo "3. 检查算法服务性能："
echo "   - 查看算法服务响应时间"
echo "   - 检查算法服务CPU/GPU使用率"
echo "   - 考虑使用更快的模型或硬件"
echo ""
echo "4. 调整抽帧频率："
echo "   如果推理速度跟不上，可以减少抽帧频率"
echo "   interval_ms = 500  # 从200增加到500"
echo ""

echo "=== 诊断完成 ==="

