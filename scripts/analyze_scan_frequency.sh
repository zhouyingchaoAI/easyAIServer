#!/bin/bash

# MinIO 抽帧文件夹扫描频率和图片数量分析脚本

LOG_FILE="${1:-build/EasyDarwin-aarch64-v8.3.3-202511141638/logs/20251114_16_00_00.log}"

if [ ! -f "$LOG_FILE" ]; then
    echo "错误: 日志文件不存在: $LOG_FILE"
    echo "用法: $0 [日志文件路径]"
    exit 1
fi

echo "分析日志文件: $LOG_FILE"
echo ""

python3 << 'EOF'
import sys
import json
from datetime import datetime

events = []
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        data = json.loads(line)
        if 'found new images' in data.get('msg', ''):
            events.append(data)
    except:
        pass

print("=" * 80)
print("MinIO 抽帧文件夹扫描分析报告")
print("=" * 80)
print()

if len(events) == 0:
    print("未找到扫描日志")
    sys.exit(0)

print(f"总扫描次数: {len(events)}")
print()

# 显示最近20次扫描
print("最近20次扫描详情:")
print(f"{'序号':<6} {'时间':<20} {'图片数':<10} {'实际间隔(ms)':<15} {'配置间隔(s)':<12} {'间隔比值':<12} {'扫描耗时(ms)':<12}")
print("-" * 95)
for i, event in enumerate(events[-20:], 1):
    ts = event.get('ts', '')
    ts_short = ts.split('T')[1][:12] if 'T' in ts else ts[:19]
    count = event.get('count', 0)
    actual_ms = event.get('actual_interval_ms', 0)
    config_sec = event.get('configured_interval_sec', 0)
    ratio = event.get('interval_ratio', 0)
    scan_duration = event.get('scan_duration_ms', 0)
    print(f"{i:<6} {ts_short:<20} {count:<10} {actual_ms:<15.2f} {config_sec:<12.2f} {ratio:<12.2f} {scan_duration:<12.2f}")

print()

# 统计
actual_intervals = [e.get('actual_interval_ms', 0) for e in events if e.get('actual_interval_ms', 0) > 0]
found_counts = [e.get('count', 0) for e in events if e.get('count', 0) > 0]
ratios = [e.get('interval_ratio', 0) for e in events if e.get('interval_ratio', 0) > 0]
scan_durations = [e.get('scan_duration_ms', 0) for e in events if e.get('scan_duration_ms', 0) > 0]

if actual_intervals:
    print("=" * 80)
    print("1. 实际扫描间隔分析")
    print("=" * 80)
    print(f"总扫描次数: {len(events)}")
    print(f"有间隔数据的扫描: {len(actual_intervals)}")
    print()
    print("实际扫描间隔（毫秒）:")
    print(f"  最小: {min(actual_intervals):.2f}ms")
    print(f"  最大: {max(actual_intervals):.2f}ms")
    print(f"  平均: {sum(actual_intervals)/len(actual_intervals):.2f}ms")
    sorted_intervals = sorted(actual_intervals)
    median = sorted_intervals[len(sorted_intervals)//2]
    print(f"  中位数: {median:.2f}ms")
    
    if events:
        config_val = events[0].get('configured_interval_sec', 0)
        print(f"配置的扫描间隔: {config_val*1000:.2f}ms ({config_val:.2f}秒)")
        avg_actual = sum(actual_intervals) / len(actual_intervals)
        print(f"实际平均间隔: {avg_actual:.2f}ms")
        if config_val > 0:
            print(f"间隔比值（实际/配置）: {avg_actual/(config_val*1000):.3f}")
        print()
        print(f"⚠️  关键发现:")
        if config_val > 0:
            if avg_actual < config_val * 1000 * 0.8:
                print(f"  - 实际扫描间隔 ({avg_actual:.0f}ms) 远小于配置值 ({config_val*1000:.0f}ms)")
                print(f"  - 扫描器运行速度比配置的要快 {config_val*1000/avg_actual:.1f} 倍")
            elif avg_actual > config_val * 1000 * 1.2:
                print(f"  - 实际扫描间隔 ({avg_actual:.0f}ms) 远大于配置值 ({config_val*1000:.0f}ms)")
                print(f"  - 扫描器运行速度比配置的要慢 {avg_actual/(config_val*1000):.1f} 倍")
            else:
                print(f"  - 实际扫描间隔 ({avg_actual:.0f}ms) 接近配置值 ({config_val*1000:.0f}ms)")
    print()
    
    # 间隔分布
    print("间隔分布:")
    ranges = {
        '0-50ms': 0,
        '50-100ms': 0,
        '100-200ms': 0,
        '200-500ms': 0,
        '500-1000ms': 0,
        '1000ms+': 0
    }
    for iv in actual_intervals:
        if iv < 50:
            ranges['0-50ms'] += 1
        elif iv < 100:
            ranges['50-100ms'] += 1
        elif iv < 200:
            ranges['100-200ms'] += 1
        elif iv < 500:
            ranges['200-500ms'] += 1
        elif iv < 1000:
            ranges['500-1000ms'] += 1
        else:
            ranges['1000ms+'] += 1
    
    for r, count in ranges.items():
        if count > 0:
            pct = count / len(actual_intervals) * 100
            print(f"  {r:12s}: {count:4d} 次 ({pct:5.1f}%)")
    print()

if found_counts:
    print("=" * 80)
    print("2. 每次扫描发现的图片数量分析")
    print("=" * 80)
    print(f"总发现次数: {len(found_counts)}")
    print()
    print("单次发现数量:")
    print(f"  最小: {min(found_counts)} 张")
    print(f"  最大: {max(found_counts)} 张")
    print(f"  平均: {sum(found_counts)/len(found_counts):.1f} 张")
    sorted_counts = sorted(found_counts)
    median_count = sorted_counts[len(sorted_counts)//2]
    print(f"  中位数: {median_count} 张")
    print()
    
    # 数量分布
    print("发现数量分布:")
    count_ranges = {
        '0-1000': 0,
        '1001-3000': 0,
        '3001-5000': 0,
        '5001-7000': 0,
        '7000+': 0
    }
    for cnt in found_counts:
        if cnt <= 1000:
            count_ranges['0-1000'] += 1
        elif cnt <= 3000:
            count_ranges['1001-3000'] += 1
        elif cnt <= 5000:
            count_ranges['3001-5000'] += 1
        elif cnt <= 7000:
            count_ranges['5001-7000'] += 1
        else:
            count_ranges['7000+'] += 1
    
    for r, count in count_ranges.items():
        if count > 0:
            pct = count / len(found_counts) * 100
            print(f"  {r:12s}: {count:4d} 次 ({pct:5.1f}%)")
    print()

# 计算扫描频率和图片积累分析
if actual_intervals and found_counts and len(actual_intervals) == len(found_counts):
    print("=" * 80)
    print("3. 扫描频率和图片积累分析")
    print("=" * 80)
    
    avg_interval = sum(actual_intervals) / len(actual_intervals)
    avg_count = sum(found_counts) / len(found_counts)
    scan_frequency = 1000 / avg_interval if avg_interval > 0 else 0
    
    print(f"平均扫描间隔: {avg_interval:.2f}ms ({avg_interval/1000:.3f}秒)")
    print(f"实际扫描频率: {scan_frequency:.2f} 次/秒")
    
    if events:
        config_val = events[0].get('configured_interval_sec', 0)
        if config_val > 0:
            expected_frequency = 1.0 / config_val
            print(f"配置的扫描频率: {expected_frequency:.2f} 次/秒 ({config_val*1000:.1f}ms/次)")
            print(f"频率比值: {scan_frequency/expected_frequency:.3f}")
    print()
    
    print("图片积累分析:")
    print(f"平均每次扫描发现: {avg_count:.1f} 张")
    print(f"平均扫描间隔: {avg_interval:.2f}ms ({avg_interval/1000:.3f}秒)")
    
    # 假设抽帧速率是70张/秒
    assumed_frame_rate = 70
    print(f"假设抽帧速率: {assumed_frame_rate} 张/秒")
    theoretical_count = (avg_interval / 1000) * assumed_frame_rate
    print(f"理论每次应发现: {theoretical_count:.1f} 张")
    print(f"实际每次发现: {avg_count:.1f} 张")
    
    if theoretical_count > 0:
        accumulation_ratio = avg_count / theoretical_count
        print(f"积累倍数: {accumulation_ratio:.1f}倍")
        print()
        print("⚠️  关键分析:")
        if accumulation_ratio > 1.5:
            print(f"  - 实际发现的图片数量 ({avg_count:.0f}张) 远大于理论值 ({theoretical_count:.0f}张)")
            print(f"  - 积累倍数: {accumulation_ratio:.1f}倍")
            print()
            print("  可能原因:")
            print("    1. 扫描间隔实际比配置的要长，导致图片积累")
            print("    2. 有多个任务同时抽帧，总抽帧速率高于70张/秒")
            print("    3. 扫描器一次性扫描了大量历史图片（积累的图片）")
            print("    4. 抽帧速率实际高于70张/秒")
        elif accumulation_ratio < 0.5:
            print(f"  - 实际发现的图片数量 ({avg_count:.0f}张) 小于理论值 ({theoretical_count:.0f}张)")
            print("  可能原因:")
            print("    1. 抽帧速率实际低于70张/秒")
            print("    2. 图片被清理速度很快")
        else:
            print(f"  - 实际发现的图片数量 ({avg_count:.0f}张) 接近理论值 ({theoretical_count:.0f}张)")
            print("  - 扫描和抽帧速率基本匹配")

if scan_durations:
    print()
    print("=" * 80)
    print("4. 扫描耗时分析")
    print("=" * 80)
    print(f"扫描耗时（毫秒）:")
    print(f"  最小: {min(scan_durations):.2f}ms")
    print(f"  最大: {max(scan_durations):.2f}ms")
    print(f"  平均: {sum(scan_durations)/len(scan_durations):.2f}ms")
    sorted_durations = sorted(scan_durations)
    median_duration = sorted_durations[len(sorted_durations)//2]
    print(f"  中位数: {median_duration:.2f}ms")
    print()
    print("扫描耗时占比（相对于扫描间隔）:")
    if actual_intervals and len(scan_durations) == len(actual_intervals):
        duration_ratios = [scan_durations[i] / actual_intervals[i] if actual_intervals[i] > 0 else 0 
                          for i in range(len(scan_durations))]
        avg_ratio = sum(duration_ratios) / len(duration_ratios)
        print(f"  平均占比: {avg_ratio*100:.1f}%")
        if avg_ratio > 0.5:
            print("  ⚠️  扫描耗时占用大量时间，可能影响扫描频率")

print()
print("=" * 80)
print("分析完成")
print("=" * 80)
EOF

