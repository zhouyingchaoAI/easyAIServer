#!/usr/bin/env python3
"""
检查告警混淆问题
"""
import json
import urllib.request
import sqlite3

print("=" * 70)
print("检查告警混淆问题")
print("=" * 70)

# 1. 检查最近的告警记录
print("\n1. 检查最近10条告警记录:")
print("-" * 70)
conn = sqlite3.connect('./configs/data.db')
cursor = conn.cursor()
cursor.execute("""
    SELECT id, task_id, image_path, created_at
    FROM alerts 
    ORDER BY created_at DESC 
    LIMIT 10
""")

alerts = cursor.fetchall()
for alert_id, task_id, image_path, created_at in alerts:
    # 从image_path解析出实际的task_id
    parts = image_path.split('/')
    if len(parts) >= 3:
        path_task_id = parts[2] if parts[0] == 'alerts' else parts[1]
    else:
        path_task_id = '?'
    
    match = "✓" if task_id == path_task_id else "✗"
    print(f"{match} ID:{alert_id:4d} | TaskID:{task_id:10s} | Path中的ID:{path_task_id:10s} | 时间:{created_at}")

conn.close()

# 2. 检查任务列表
print("\n2. 检查所有任务的ID和OutputPath:")
print("-" * 70)
try:
    with urllib.request.urlopen('http://localhost:5066/api/v1/frame_extractor/tasks') as response:
        data = json.loads(response.read().decode())
        tasks = data.get('items', [])
        
        for task in tasks:
            task_id = task.get('id', '')
            output_path = task.get('output_path', '')
            config_status = task.get('config_status', '')
            enabled = task.get('enabled', False)
            
            match = "✓" if task_id == output_path else "✗"
            status = "运行中" if enabled else "已停止"
            print(f"{match} {task_id:15s} | OutputPath:{output_path:15s} | {config_status:12s} | {status}")
except Exception as e:
    print(f"获取任务列表失败: {e}")

# 3. 检查几个任务的配置文件是否能读取
print("\n3. 检查任务配置文件:")
print("-" * 70)
test_tasks = ['测试1', '测试2', '测试3', '门口3']
for task_id in test_tasks:
    try:
        url = f'http://localhost:5066/api/v1/frame_extractor/tasks/{task_id}/config'
        with urllib.request.urlopen(url) as response:
            config = json.loads(response.read().decode())
            regions = len(config.get('regions', []))
            print(f"✓ {task_id:15s} | 配置文件存在 | {regions}个区域")
    except urllib.error.HTTPError as e:
        if e.code == 404:
            print(f"✗ {task_id:15s} | 配置文件不存在")
        else:
            print(f"✗ {task_id:15s} | 读取失败: HTTP {e.code}")
    except Exception as e:
        print(f"✗ {task_id:15s} | 错误: {e}")

print("\n" + "=" * 70)
print("诊断结果:")
print("=" * 70)

# 检查告警是否有不匹配的情况
conn = sqlite3.connect('./configs/data.db')
cursor = conn.cursor()
cursor.execute("""
    SELECT COUNT(*) 
    FROM alerts 
    WHERE task_id != SUBSTR(image_path, 
        CASE 
            WHEN image_path LIKE 'alerts/%' THEN LENGTH('alerts/') + INSTR(SUBSTR(image_path, LENGTH('alerts/')+1), '/') + 1
            ELSE INSTR(image_path, '/') + 1
        END,
        INSTR(SUBSTR(image_path, 
            CASE 
                WHEN image_path LIKE 'alerts/%' THEN LENGTH('alerts/') + INSTR(SUBSTR(image_path, LENGTH('alerts/')+1), '/') + 1
                ELSE INSTR(image_path, '/') + 1
            END
        ), '/') - 1
    )
""")
mismatch_count = cursor.fetchone()[0]
conn.close()

if mismatch_count > 0:
    print(f"⚠️  发现 {mismatch_count} 条告警的task_id与image_path不匹配")
    print("   原因可能是：")
    print("   1. 旧的告警数据（修复前产生的）")
    print("   2. 仍有旧图片被扫描并推理")
    print("\n   解决方法：")
    print("   1. 清空旧告警数据：sqlite3 configs/data.db 'DELETE FROM alerts;'")
    print("   2. 清理MinIO中的旧图片：python3 cleanup_old_frames.py")
    print("   3. 重启服务让新图片使用新路径")
else:
    print("✓ 所有告警的task_id与image_path都匹配")
    print("\n如果前端仍显示错乱，可能是以下原因：")
    print("1. 浏览器缓存问题 - 建议清除缓存或使用无痕模式")
    print("2. 配置文件路径问题 - 检查algo_config.json是否在正确位置")
    print("3. 图片内容本身的问题 - 检查是否是同一个摄像头")

