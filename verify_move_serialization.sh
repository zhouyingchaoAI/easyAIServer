#!/bin/bash

# 验证图片移动串行化效果
# 2025-11-06

echo "========================================"
echo "验证图片移动串行化效果"
echo "========================================"
echo ""

LOG_DIR="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/logs"

# 获取最新的日志文件
LATEST_LOG=$(ls -t $LOG_DIR/*.log | grep -v "crash\|sugar" | head -1)

if [ -z "$LATEST_LOG" ]; then
    echo "❌ 未找到日志文件"
    exit 1
fi

echo "分析日志文件: $LATEST_LOG"
echo ""

# 1. 检查移动锁是否生效（同一task_id的移动应该串行）
echo "1. 检查同一task_id的移动是否串行化..."
echo "="*70

python3 << EOF
import json
from collections import defaultdict
from datetime import datetime

log_file = "$LATEST_LOG"

# 读取移动记录
moves = []
with open(log_file, 'r') as f:
    for line in f:
        if "async image move" in line:
            try:
                data = json.loads(line)
                if 'ts' in data and 'task_id' in data:
                    # 解析时间戳（精确到毫秒）
                    ts_str = data['ts']
                    # 格式: 2025-11-06 09:17:29.123
                    try:
                        ts = datetime.strptime(ts_str, "%Y-%m-%d %H:%M:%S.%f")
                    except:
                        try:
                            ts = datetime.strptime(ts_str[:19], "%Y-%m-%d %H:%M:%S")
                        except:
                            continue
                    
                    moves.append({
                        'time': ts,
                        'task_id': data.get('task_id', ''),
                        'filename': data.get('filename', ''),
                        'status': 'succeeded' if 'succeeded' in data.get('msg', '') else 'failed'
                    })
            except:
                pass

if not moves:
    print("⚠️  未找到移动记录（可能服务刚启动）")
    exit(0)

print(f"分析最近 {len(moves)} 条移动记录\n")

# 按task_id分组
by_task = defaultdict(list)
for move in moves:
    by_task[move['task_id']].append(move)

# 检查每个task_id的移动是否有重叠
print("检查各任务的移动时序:")
print("-" * 80)

overlap_count = 0
serial_count = 0

for task_id, task_moves in sorted(by_task.items()):
    if len(task_moves) < 2:
        continue
    
    # 检查是否有时间重叠
    task_moves.sort(key=lambda x: x['time'])
    has_overlap = False
    
    for i in range(len(task_moves) - 1):
        time_diff = (task_moves[i+1]['time'] - task_moves[i]['time']).total_seconds()
        # 如果两次移动间隔小于0.001秒，认为是并发的
        if time_diff < 0.001:
            has_overlap = True
            break
    
    if has_overlap:
        overlap_count += 1
        print(f"⚠️  {task_id:15s}: 检测到并发移动 (移动次数: {len(task_moves)})")
    else:
        serial_count += 1
        if len(task_moves) >= 3:  # 只显示有多次移动的
            print(f"✓  {task_id:15s}: 串行移动正常 (移动次数: {len(task_moves)})")

print()
print("="* 80)
print(f"统计结果:")
print(f"  串行化任务: {serial_count} 个")
print(f"  仍有重叠: {overlap_count} 个")

if overlap_count == 0:
    print("\n✓ 移动锁生效，所有任务的移动都已串行化")
else:
    print(f"\n⚠️  仍有 {overlap_count} 个任务存在并发移动（可能需要进一步调查）")

EOF

echo ""

# 2. 统计最近的移动成功率
echo "2. 统计移动成功率..."
echo "="*70

python3 << EOF
import json

log_file = "$LATEST_LOG"

success_count = 0
failed_count = 0

with open(log_file, 'r') as f:
    for line in f:
        if "async image move succeeded" in line:
            success_count += 1
        elif "async image move failed" in line:
            failed_count += 1

total = success_count + failed_count

if total > 0:
    success_rate = (success_count / total) * 100
    print(f"总移动次数: {total}")
    print(f"成功: {success_count} ({success_rate:.2f}%)")
    print(f"失败: {failed_count} ({100-success_rate:.2f}%)")
    
    if success_rate >= 99:
        print("\n✓ 移动成功率良好")
    elif success_rate >= 95:
        print("\n⚠️  移动成功率可接受，但建议检查失败原因")
    else:
        print("\n✗ 移动成功率偏低，需要排查问题")
else:
    print("⚠️  暂无移动记录")

EOF

echo ""

# 3. 检查路径一致性
echo "3. 检查最近移动的路径一致性..."
echo "="*70

python3 << EOF
import json

log_file = "$LATEST_LOG"

# 读取最近20条移动记录
moves = []
with open(log_file, 'r') as f:
    for line in f:
        if "async image move" in line:
            try:
                data = json.loads(line)
                moves.append(data)
            except:
                pass

recent_moves = moves[-20:] if len(moves) > 20 else moves

if not recent_moves:
    print("⚠️  暂无移动记录")
else:
    print(f"检查最近 {len(recent_moves)} 条移动记录:\n")
    
    issues = 0
    for i, move in enumerate(recent_moves, 1):
        task_id = move.get('task_id', '')
        filename = move.get('filename', '')
        src = move.get('src', '')
        dst = move.get('dst', '')
        
        # 检查src和dst是否一致
        src_parts = src.split('/')
        dst_parts = dst.split('/')
        
        if len(src_parts) >= 3 and len(dst_parts) >= 4:
            src_task_id = src_parts[1]
            src_filename = src_parts[-1]
            dst_task_id = dst_parts[2]
            dst_filename = dst_parts[-1]
            
            if task_id == src_task_id == dst_task_id and filename == src_filename == dst_filename:
                print(f"✓ #{i:2d}: {task_id:12s} | {filename}")
            else:
                issues += 1
                print(f"✗ #{i:2d}: 不一致")
                print(f"     TaskID: {task_id} vs Src:{src_task_id} vs Dst:{dst_task_id}")
                print(f"     Filename: {filename} vs Src:{src_filename} vs Dst:{dst_filename}")
    
    print()
    if issues == 0:
        print("✓ 所有移动记录的路径都一致")
    else:
        print(f"✗ 发现 {issues} 条路径不一致的记录")

EOF

echo ""
echo "="*70
echo "验证完成"
echo "="*70

