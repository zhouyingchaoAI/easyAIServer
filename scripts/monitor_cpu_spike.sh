#!/bin/bash
# CPU使用率暴涨监控脚本

# 配置参数
CHECK_INTERVAL=2          # 检查间隔（秒）
CPU_THRESHOLD=50          # CPU使用率阈值（%），超过此值触发告警
SPIKE_THRESHOLD=30        # CPU突然增长阈值（%），短时间内增长超过此值触发告警
LOG_FILE="./cpu_spike_monitor.log"
TOP_N=10                  # 显示前N个CPU占用最高的进程

# 创建日志文件
mkdir -p "$(dirname "$LOG_FILE")"
LOG_FILE=$(readlink -f "$LOG_FILE")

echo "=== CPU使用率暴涨监控 ==="
echo "开始时间: $(date)"
echo "检查间隔: ${CHECK_INTERVAL}秒"
echo "CPU阈值: ${CPU_THRESHOLD}%"
echo "突增阈值: ${SPIKE_THRESHOLD}%"
echo "日志文件: $LOG_FILE"
echo ""

# 初始化
last_total_cpu=0
last_idle_cpu=0
last_check_time=$(date +%s)

# 获取CPU使用率的函数
get_cpu_usage() {
    # 读取/proc/stat获取CPU信息
    cpu_info=$(grep "^cpu " /proc/stat)
    
    # 解析CPU时间
    user=$(echo $cpu_info | awk '{print $2}')
    nice=$(echo $cpu_info | awk '{print $3}')
    system=$(echo $cpu_info | awk '{print $4}')
    idle=$(echo $cpu_info | awk '{print $5}')
    iowait=$(echo $cpu_info | awk '{print $6}')
    irq=$(echo $cpu_info | awk '{print $7}')
    softirq=$(echo $cpu_info | awk '{print $8}')
    
    # 计算总CPU时间
    total=$((user + nice + system + idle + iowait + irq + softirq))
    
    echo "$total $idle"
}

# 获取进程CPU使用率
get_top_processes() {
    # 使用ps命令获取进程信息，按CPU使用率排序
    ps aux --sort=-%cpu | head -n $((TOP_N + 1)) | tail -n +2 | awk '{printf "%-8s %-6s %-6s %-s\n", $2, $3, $4, $11" "$12" "$13" "$14" "$15" "$16" "$17" "$18" "$19" "$20" "$21}'
}

# 主监控循环
while true; do
    current_time=$(date +%s)
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 获取当前CPU使用率
    read total_cpu idle_cpu <<< $(get_cpu_usage)
    
    # 计算CPU使用率（相对于上次检查）
    if [ $last_total_cpu -gt 0 ]; then
        total_diff=$((total_cpu - last_total_cpu))
        idle_diff=$((idle_cpu - last_idle_cpu))
        
        if [ $total_diff -gt 0 ]; then
            # 计算CPU使用率百分比
            cpu_usage=$((100 * (total_diff - idle_diff) / total_diff))
            
            # 计算时间差
            time_diff=$((current_time - last_check_time))
            
            # 检查是否超过阈值
            if [ $cpu_usage -gt $CPU_THRESHOLD ]; then
                echo "[$timestamp] ⚠️  CPU使用率过高: ${cpu_usage}% (阈值: ${CPU_THRESHOLD}%)" | tee -a "$LOG_FILE"
                
                # 获取占用CPU最高的进程
                echo "[$timestamp] 占用CPU最高的进程:" | tee -a "$LOG_FILE"
                echo "PID      CPU%   MEM%   命令" | tee -a "$LOG_FILE"
                get_top_processes | tee -a "$LOG_FILE"
                echo "" | tee -a "$LOG_FILE"
            fi
            
            # 检查CPU使用率是否突然增长
            if [ -n "$last_cpu_usage" ] && [ $last_cpu_usage -lt $((cpu_usage - SPIKE_THRESHOLD)) ]; then
                spike_amount=$((cpu_usage - last_cpu_usage))
                echo "[$timestamp] 🚨 CPU使用率突然暴涨: ${last_cpu_usage}% → ${cpu_usage}% (增长 ${spike_amount}%)" | tee -a "$LOG_FILE"
                
                # 获取占用CPU最高的进程
                echo "[$timestamp] 占用CPU最高的进程:" | tee -a "$LOG_FILE"
                echo "PID      CPU%   MEM%   命令" | tee -a "$LOG_FILE"
                get_top_processes | tee -a "$LOG_FILE"
                echo "" | tee -a "$LOG_FILE"
            fi
            
            # 显示当前CPU使用率（可选，取消注释以启用）
            # echo "[$timestamp] CPU使用率: ${cpu_usage}%"
            
            last_cpu_usage=$cpu_usage
        fi
    fi
    
    # 更新上次的值
    last_total_cpu=$total_cpu
    last_idle_cpu=$idle_cpu
    last_check_time=$current_time
    
    # 等待下次检查
    sleep $CHECK_INTERVAL
done

