#!/bin/bash

# 诊断图片移动过程中的文件名变化或错位问题
# 2025-11-06

echo "========================================"
echo "图片移动问题诊断脚本"
echo "========================================"
echo ""

LOG_FILE="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/logs/20251106_08_00_00.log"

# 1. 提取最近的移动记录，分析源路径和目标路径的对应关系
echo "1. 分析最近50次图片移动记录..."
echo "="*70

python3 << 'EOF'
import json
import re
from collections import defaultdict

log_file = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/logs/20251106_08_00_00.log"

# 读取日志文件
with open(log_file, 'r') as f:
    lines = f.readlines()

# 查找移动记录
move_records = []
for line in lines:
    if "async image move succeeded" in line or "async image move failed" in line:
        try:
            log_data = json.loads(line)
            move_records.append(log_data)
        except:
            pass

print(f"找到 {len(move_records)} 条移动记录\n")
print("检查移动前后的路径对应关系:")
print("="*80)

# 分析最近50条记录
issues = []
for i, record in enumerate(move_records[-50:], 1):
    task_id = record.get('task_id', 'N/A')
    task_type = record.get('task_type', 'N/A')
    filename = record.get('filename', 'N/A')
    src = record.get('src', '')
    dst = record.get('dst', '')
    status = "✓ 成功" if "succeeded" in record.get('msg', '') else "✗ 失败"
    
    # 从源路径解析信息
    src_parts = src.split('/')
    if len(src_parts) >= 3:
        src_task_type = src_parts[0]
        src_task_id = src_parts[1]
        src_filename = src_parts[-1]
    else:
        src_task_type = src_task_id = src_filename = "?"
    
    # 从目标路径解析信息
    dst_parts = dst.split('/')
    if len(dst_parts) >= 4 and dst_parts[0] == 'alerts':
        dst_task_type = dst_parts[1]
        dst_task_id = dst_parts[2]
        dst_filename = dst_parts[-1]
    elif len(dst_parts) >= 3:
        dst_task_type = dst_parts[0]
        dst_task_id = dst_parts[1]
        dst_filename = dst_parts[-1]
    else:
        dst_task_type = dst_task_id = dst_filename = "?"
    
    # 检查一致性
    task_id_match = (task_id == src_task_id == dst_task_id)
    task_type_match = (task_type == src_task_type == dst_task_type)
    filename_match = (filename == src_filename == dst_filename)
    
    if not (task_id_match and task_type_match and filename_match):
        issues.append({
            'index': i,
            'task_id': task_id,
            'src_task_id': src_task_id,
            'dst_task_id': dst_task_id,
            'filename': filename,
            'src_filename': src_filename,
            'dst_filename': dst_filename,
            'src': src,
            'dst': dst
        })
        
        print(f"\n⚠️  记录 #{i}: 发现不一致")
        print(f"   记录TaskID: {task_id:15s} | 源TaskID: {src_task_id:15s} | 目标TaskID: {dst_task_id:15s} | 匹配: {'✓' if task_id_match else '✗'}")
        print(f"   记录Filename: {filename:30s}")
        print(f"   源Filename:   {src_filename:30s}")
        print(f"   目标Filename: {dst_filename:30s} | 匹配: {'✓' if filename_match else '✗'}")
        print(f"   源路径: {src}")
        print(f"   目标路径: {dst}")

if not issues:
    print("\n✓ 所有移动记录的路径对应关系正确")
else:
    print(f"\n✗ 发现 {len(issues)} 条有问题的移动记录")

# 统计问题类型
if issues:
    print("\n问题分类:")
    task_id_issues = sum(1 for issue in issues if issue['task_id'] != issue['dst_task_id'])
    filename_issues = sum(1 for issue in issues if issue['filename'] != issue['dst_filename'])
    print(f"  - TaskID不匹配: {task_id_issues} 条")
    print(f"  - 文件名不匹配: {filename_issues} 条")

EOF

echo ""
echo "="*70
echo ""

# 2. 检查构建路径和实际移动是否一致
echo "2. 检查路径构建和实际移动的时序关系..."
echo "="*70

python3 << 'EOF'
import json
from datetime import datetime

log_file = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/logs/20251106_08_00_00.log"

# 读取日志
with open(log_file, 'r') as f:
    lines = f.readlines()

# 提取构建和移动记录
constructions = []  # 路径构建记录
moves = []          # 移动记录

for line in lines:
    try:
        log_data = json.loads(line)
        msg = log_data.get('msg', '')
        
        if "constructing alert image path" in msg:
            constructions.append({
                'time': log_data.get('ts', ''),
                'task_id': log_data.get('task_id', ''),
                'task_type': log_data.get('task_type', ''),
                'filename': log_data.get('filename', ''),
                'src_path': log_data.get('src_path', ''),
                'target_path': log_data.get('target_path', '')
            })
        elif "async image move" in msg:
            moves.append({
                'time': log_data.get('ts', ''),
                'task_id': log_data.get('task_id', ''),
                'task_type': log_data.get('task_type', ''),
                'filename': log_data.get('filename', ''),
                'src': log_data.get('src', ''),
                'dst': log_data.get('dst', ''),
                'success': 'succeeded' in msg
            })
    except:
        pass

print(f"找到 {len(constructions)} 条路径构建记录")
print(f"找到 {len(moves)} 条移动记录\n")

# 匹配构建和移动记录
print("检查构建和移动的对应关系（最近20条）:")
print("="*80)

mismatches = []
for i, construction in enumerate(constructions[-20:], 1):
    # 查找对应的移动记录
    matching_move = None
    for move in moves:
        if (move['src'] == construction['src_path'] and 
            move['dst'] == construction['target_path']):
            matching_move = move
            break
    
    if matching_move:
        # 检查task_id和filename是否一致
        if (construction['task_id'] == matching_move['task_id'] and
            construction['filename'] == matching_move['filename']):
            print(f"✓ #{i}: TaskID={construction['task_id']:12s} Filename={construction['filename']}")
        else:
            print(f"✗ #{i}: 信息不匹配!")
            print(f"   构建: TaskID={construction['task_id']:12s} Filename={construction['filename']}")
            print(f"   移动: TaskID={matching_move['task_id']:12s} Filename={matching_move['filename']}")
            mismatches.append({
                'construction': construction,
                'move': matching_move
            })
    else:
        print(f"⚠ #{i}: TaskID={construction['task_id']:12s} - 未找到对应的移动记录")
        print(f"   预期源路径: {construction['src_path']}")
        print(f"   预期目标路径: {construction['target_path']}")

if mismatches:
    print(f"\n发现 {len(mismatches)} 条不匹配的记录")
else:
    print("\n✓ 构建和移动记录完全匹配")

EOF

echo ""
echo "="*70
echo ""

# 3. 检查数据库中最近的告警记录
echo "3. 检查数据库中告警记录的路径正确性..."
echo "="*70

python3 << 'EOF'
import sqlite3

db_path = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/configs/data.db"
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

cursor.execute("""
    SELECT id, task_id, task_type, image_path, created_at
    FROM alerts
    ORDER BY created_at DESC
    LIMIT 30
""")

alerts = cursor.fetchall()
conn.close()

print(f"检查最近30条告警记录的路径一致性:\n")

path_issues = []
for alert_id, task_id, task_type, image_path, created_at in alerts:
    # 解析路径
    parts = image_path.split('/')
    
    if len(parts) >= 4 and parts[0] == 'alerts':
        # alerts/task_type/task_id/filename 格式
        path_task_type = parts[1]
        path_task_id = parts[2]
        path_filename = parts[3]
    elif len(parts) >= 3:
        # task_type/task_id/filename 格式
        path_task_type = parts[0]
        path_task_id = parts[1]
        path_filename = parts[2]
    else:
        path_task_type = path_task_id = path_filename = "?"
    
    task_id_match = (task_id == path_task_id)
    task_type_match = (task_type == path_task_type)
    
    if not (task_id_match and task_type_match):
        path_issues.append({
            'id': alert_id,
            'task_id': task_id,
            'path_task_id': path_task_id,
            'task_type': task_type,
            'path_task_type': path_task_type,
            'image_path': image_path
        })
        
        print(f"✗ ID:{alert_id:4d} | 记录TaskID:{task_id:12s} vs 路径TaskID:{path_task_id:12s}")
        print(f"         | 记录Type:{task_type:12s} vs 路径Type:{path_task_type:12s}")
        print(f"         | Path: {image_path}")
        print()

if not path_issues:
    print("✓ 所有告警记录的路径都正确匹配")
else:
    print(f"✗ 发现 {len(path_issues)} 条路径不匹配的告警记录")

EOF

echo ""
echo "="*70
echo ""

# 4. 检查并发场景下的时间戳
echo "4. 检查并发移动的时间戳分布..."
echo "="*70

python3 << 'EOF'
import json
from datetime import datetime
from collections import defaultdict

log_file = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/logs/20251106_08_00_00.log"

with open(log_file, 'r') as f:
    lines = f.readlines()

# 提取移动记录并按秒分组
moves_by_second = defaultdict(list)

for line in lines:
    if "async image move" in line:
        try:
            log_data = json.loads(line)
            ts = log_data.get('ts', '')
            # 提取到秒
            ts_second = ts[:19] if len(ts) >= 19 else ts
            
            moves_by_second[ts_second].append({
                'task_id': log_data.get('task_id', ''),
                'filename': log_data.get('filename', ''),
                'src': log_data.get('src', ''),
                'dst': log_data.get('dst', '')
            })
        except:
            pass

# 找出并发移动的时间点
concurrent_moves = {ts: moves for ts, moves in moves_by_second.items() if len(moves) > 1}

print(f"发现 {len(concurrent_moves)} 个时间点有并发移动操作\n")

if concurrent_moves:
    print("并发移动详情（可能导致竞态条件）:")
    print("="*80)
    
    for ts, moves in sorted(concurrent_moves.items())[-10:]:  # 显示最近10个
        print(f"\n时间: {ts} | 并发数: {len(moves)}")
        
        # 检查是否有相同的task_id
        task_ids = [m['task_id'] for m in moves]
        if len(task_ids) != len(set(task_ids)):
            print("  ⚠️  警告: 同一秒内有相同task_id的多次移动!")
        
        for i, move in enumerate(moves[:5], 1):  # 只显示前5个
            print(f"  {i}. TaskID={move['task_id']:12s} File={move['filename']}")
else:
    print("✓ 未发现明显的并发移动竞态问题")

EOF

echo ""
echo "="*70
echo ""

echo "诊断完成！"
echo ""
echo "建议检查项:"
echo "  1. 如果发现TaskID不匹配，说明闭包捕获的变量有问题"
echo "  2. 如果发现文件名不匹配，说明filename解析有问题"
echo "  3. 如果有大量并发移动，可能导致竞态条件"
echo "  4. 检查日志中的完整移动链路: 构建 -> 移动 -> 数据库"

