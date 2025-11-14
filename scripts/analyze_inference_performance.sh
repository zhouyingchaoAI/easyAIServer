#!/bin/bash
# 推理性能分析脚本（简化版，不依赖bc）

EASYDARWIN_URL="${1:-http://localhost:5066}"
LOG_FILE="${2:-$(find build -name "*.log" -type f 2>/dev/null | head -1)}"

echo "=== 推理性能分析 ==="
echo ""

# 1. 队列状态
echo "【队列状态】"
STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")
echo "$STATS" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'队列大小: {d[\"queue_size\"]}/{d[\"queue_max_size\"]}')
util = d['queue_utilization'] * 100
print(f'使用率: {util:.2f}%')
if util > 90:
    print('⚠️  队列积压严重！')
print(f'已处理: {d[\"processed_total\"]}')
print(f'平均推理时间: {d[\"avg_inference_ms\"]}ms')
print(f'总推理次数: {d[\"total_inferences\"]}')
"
echo ""

# 2. 算法服务
echo "【算法服务】"
SERVICES=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/services")
echo "$SERVICES" | python3 -c "
import sys, json
d = json.load(sys.stdin)
services = d.get('services', [])
print(f'已注册服务数: {len(services)}')
if services:
    calls = [s['call_count'] for s in services]
    print(f'调用次数范围: {min(calls)} - {max(calls)}')
    print(f'平均调用次数: {sum(calls) // len(calls)}')
"
echo ""

# 3. 负载均衡
echo "【负载均衡】"
LB=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/load_balance/info")
echo "$LB" | python3 -c "
import sys, json
d = json.load(sys.stdin)
lb = d.get('load_balance', {})
for task_type, info in lb.items():
    print(f'任务类型: {task_type}')
    services = info.get('services', [])
    if services:
        resp_times = [s.get('avg_response_ms', 0) for s in services]
        print(f'  服务数: {len(services)}')
        print(f'  平均响应时间: {sum(resp_times)/len(resp_times):.0f}ms')
        print(f'  最快: {min(resp_times)}ms, 最慢: {max(resp_times)}ms')
"
echo ""

# 4. 日志分析
if [ -n "$LOG_FILE" ] && [ -f "$LOG_FILE" ]; then
    echo "【日志分析（最近5000行）】"
    
    # 图片丢失
    LOST=$(tail -5000 "$LOG_FILE" | grep -c "image not found in MinIO")
    echo "图片丢失次数: $LOST"
    
    # 推理耗时（简单统计）
    echo ""
    echo "推理耗时统计："
    tail -10000 "$LOG_FILE" | grep "algorithm_call_duration_ms" | python3 << 'PYEOF'
import sys
import re

durations = []
for line in sys.stdin:
    # 尝试多种格式提取
    patterns = [
        r'algorithm_call_duration_ms["\s:]+([0-9.]+)',
        r'"algorithm_call_duration_ms":\s*([0-9.]+)',
        r'algorithm_call_duration_ms.*?([0-9.]+)ms'
    ]
    for pattern in patterns:
        match = re.search(pattern, line)
        if match:
            try:
                val = float(match.group(1))
                if val > 0:
                    durations.append(val)
                    break
            except:
                pass

if durations:
    print(f'  样本数: {len(durations)}')
    print(f'  平均: {sum(durations)/len(durations):.2f}ms')
    print(f'  最大: {max(durations):.2f}ms')
    print(f'  最小: {min(durations):.2f}ms')
else:
    print('  ⚠️  未找到有效的推理耗时数据')
PYEOF
fi

echo ""
echo "=== 分析完成 ==="

