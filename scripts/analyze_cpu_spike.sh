#!/bin/bash
# CPU使用率暴涨分析脚本 - 分析日志文件

LOG_FILE="${1:-./cpu_spike_monitor.log}"

if [ ! -f "$LOG_FILE" ]; then
    echo "❌ 日志文件不存在: $LOG_FILE"
    echo ""
    echo "用法: $0 [日志文件路径]"
    echo "示例: $0 ./cpu_spike_monitor.log"
    exit 1
fi

echo "=== CPU使用率暴涨分析 ==="
echo "日志文件: $LOG_FILE"
echo ""

# 统计CPU暴涨次数
spike_count=$(grep -c "CPU使用率突然暴涨" "$LOG_FILE" 2>/dev/null || echo "0")
high_cpu_count=$(grep -c "CPU使用率过高" "$LOG_FILE" 2>/dev/null || echo "0")

echo "📊 统计信息："
echo "   CPU突然暴涨次数: $spike_count"
echo "   CPU使用率过高次数: $high_cpu_count"
echo ""

# 显示所有CPU暴涨事件
if [ $spike_count -gt 0 ]; then
    echo "🚨 CPU突然暴涨事件："
    echo "----------------------------------------"
    grep "CPU使用率突然暴涨" "$LOG_FILE" | head -20
    echo ""
fi

# 分析最常出现的进程
echo "📋 最常出现的CPU占用进程："
echo "----------------------------------------"
grep -A 10 "占用CPU最高的进程" "$LOG_FILE" | grep -E "^[0-9]" | \
    awk '{print $1, $2, $NF}' | \
    sort | uniq -c | sort -rn | head -10 | \
    awk '{printf "出现次数: %-5s | PID: %-8s | CPU: %-6s | 命令: %s\n", $1, $2, $3, $4}'
echo ""

# 显示最近的CPU暴涨事件详情
if [ $spike_count -gt 0 ]; then
    echo "📝 最近的CPU暴涨事件详情："
    echo "----------------------------------------"
    # 获取最后一次暴涨事件的完整信息
    last_spike_line=$(grep -n "CPU使用率突然暴涨" "$LOG_FILE" | tail -1 | cut -d: -f1)
    if [ -n "$last_spike_line" ]; then
        sed -n "${last_spike_line},$((last_spike_line + 15))p" "$LOG_FILE"
    fi
fi

echo ""
echo "分析完成时间: $(date)"

