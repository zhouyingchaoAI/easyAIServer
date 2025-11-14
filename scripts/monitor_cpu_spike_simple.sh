#!/bin/bash
# 简化版CPU监控脚本 - 实时显示CPU使用率最高的进程

# 配置参数
INTERVAL=2              # 刷新间隔（秒）
TOP_N=15                # 显示前N个进程
CPU_THRESHOLD=30        # 只显示CPU使用率超过此值的进程

echo "=== CPU使用率实时监控 ==="
echo "刷新间隔: ${INTERVAL}秒"
echo "显示前${TOP_N}个CPU占用最高的进程"
echo "按 Ctrl+C 退出"
echo ""

while true; do
    clear
    echo "=== $(date '+%Y-%m-%d %H:%M:%S') ==="
    echo ""
    
    # 显示系统整体CPU使用率
    echo "【系统CPU使用率】"
    top -bn1 | grep "Cpu(s)" | sed 's/.*, *\([0-9.]*\)%* id.*/\1/' | awk '{print "CPU空闲率: " $1 "% | CPU使用率: " (100-$1) "%"}'
    echo ""
    
    # 显示占用CPU最高的进程
    echo "【占用CPU最高的进程 (PID | CPU% | MEM% | 命令)】"
    echo "----------------------------------------"
    ps aux --sort=-%cpu | head -n $((TOP_N + 1)) | tail -n +2 | \
        awk -v threshold=$CPU_THRESHOLD '$3 > threshold {printf "%-8s %-6s %-6s %s\n", $2, $3, $4, $11" "$12" "$13" "$14" "$15" "$16" "$17" "$18" "$19" "$20" "$21}' | \
        while read pid cpu mem cmd; do
            if [ -n "$pid" ]; then
                printf "%-8s %-6s %-6s %s\n" "$pid" "$cpu%" "$mem%" "$cmd"
            fi
        done
    
    echo ""
    echo "按 Ctrl+C 退出，${INTERVAL}秒后刷新..."
    
    sleep $INTERVAL
done

