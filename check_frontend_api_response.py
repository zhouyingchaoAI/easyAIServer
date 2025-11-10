#!/usr/bin/env python3
"""
检查前端API返回的告警数据是否正确
"""

import json
import urllib.request
import urllib.error
import sqlite3

print("=" * 80)
print("检查前端API返回的告警数据")
print("=" * 80)
print()

# 1. 从数据库读取最近的告警
print("1. 从数据库读取最近10条告警:")
print("-" * 80)

db_path = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831/configs/data.db"
conn = sqlite3.connect(db_path)
cursor = conn.cursor()

cursor.execute("""
    SELECT id, task_id, task_type, image_path, detection_count, created_at
    FROM alerts
    ORDER BY created_at DESC
    LIMIT 10
""")

db_alerts = cursor.fetchall()
conn.close()

print(f"数据库中的告警记录:\n")
for alert_id, task_id, task_type, image_path, count, created_at in db_alerts:
    print(f"ID:{alert_id:4d} | TaskID:{task_id:12s} | Type:{task_type:12s} | Count:{count}")
    print(f"       | Path: {image_path}")
    print()

# 2. 从API读取告警数据
print("\n2. 从API读取相同的告警数据:")
print("-" * 80)

try:
    # 获取告警列表
    req = urllib.request.Request('http://localhost:5066/api/v1/alerts?page=1&page_size=10')
    with urllib.request.urlopen(req, timeout=5) as response:
        data = json.loads(response.read().decode())
        api_alerts = data.get('items', [])
        
        print(f"API返回了 {len(api_alerts)} 条告警\n")
        
        # 3. 对比数据库和API返回的数据
        print("\n3. 对比数据库和API数据的一致性:")
        print("-" * 80)
        
        mismatches = []
        
        for i, api_alert in enumerate(api_alerts):
            api_id = api_alert.get('ID', 0)
            api_task_id = api_alert.get('TaskID', '')
            api_task_type = api_alert.get('TaskType', '')
            api_image_path = api_alert.get('ImagePath', '')
            api_image_url = api_alert.get('ImageURL', '')
            api_count = api_alert.get('DetectionCount', 0)
            
            # 从数据库中找到对应的记录
            db_record = None
            for db_alert in db_alerts:
                if db_alert[0] == api_id:
                    db_record = db_alert
                    break
            
            if db_record:
                db_id, db_task_id, db_task_type, db_image_path, db_count, db_time = db_record
                
                # 检查一致性
                task_id_match = (api_task_id == db_task_id)
                task_type_match = (api_task_type == db_task_type)
                image_path_match = (api_image_path == db_image_path)
                count_match = (api_count == db_count)
                
                # 从image_path解析task_id
                path_parts = api_image_path.split('/')
                if len(path_parts) >= 3:
                    if path_parts[0] == 'alerts':
                        path_task_id = path_parts[2]
                        path_task_type = path_parts[1]
                    else:
                        path_task_id = path_parts[1]
                        path_task_type = path_parts[0]
                else:
                    path_task_id = path_task_type = "?"
                
                path_task_id_match = (api_task_id == path_task_id)
                path_task_type_match = (api_task_type == path_task_type)
                
                if not (task_id_match and task_type_match and image_path_match and count_match and path_task_id_match):
                    mismatches.append({
                        'id': api_id,
                        'api_task_id': api_task_id,
                        'db_task_id': db_task_id,
                        'path_task_id': path_task_id,
                        'api_image_path': api_image_path,
                        'db_image_path': db_image_path,
                        'api_image_url': api_image_url
                    })
                    
                    print(f"✗ ID:{api_id:4d} 发现不一致:")
                    if not task_id_match:
                        print(f"   TaskID不匹配: API={api_task_id} vs DB={db_task_id}")
                    if not path_task_id_match:
                        print(f"   路径TaskID不匹配: API={api_task_id} vs Path={path_task_id}")
                    if not task_type_match:
                        print(f"   TaskType不匹配: API={api_task_type} vs DB={db_task_type}")
                    if not image_path_match:
                        print(f"   ImagePath不匹配:")
                        print(f"     API: {api_image_path}")
                        print(f"     DB:  {db_image_path}")
                    if not count_match:
                        print(f"   Count不匹配: API={api_count} vs DB={db_count}")
                    print(f"   ImageURL: {api_image_url[:80]}..." if len(api_image_url) > 80 else f"   ImageURL: {api_image_url}")
                    print()
                else:
                    print(f"✓ ID:{api_id:4d} | TaskID:{api_task_id:12s} | PathTaskID:{path_task_id:12s} | 完全匹配")
        
        if not mismatches:
            print("\n" + "=" * 80)
            print("✓ 所有API返回的数据与数据库完全一致")
            print("=" * 80)
        else:
            print("\n" + "=" * 80)
            print(f"✗ 发现 {len(mismatches)} 条不一致的记录")
            print("=" * 80)
            
            # 详细分析不一致的原因
            print("\n不一致原因分析:")
            for mismatch in mismatches:
                print(f"\nID {mismatch['id']}:")
                print(f"  API TaskID: {mismatch['api_task_id']}")
                print(f"  DB TaskID:  {mismatch['db_task_id']}")
                print(f"  Path TaskID: {mismatch['path_task_id']}")
                print(f"  ImagePath:  {mismatch['api_image_path']}")
                print(f"  ImageURL:   {mismatch['api_image_url'][:100]}..." if len(mismatch['api_image_url']) > 100 else f"  ImageURL:   {mismatch['api_image_url']}")

except urllib.error.URLError as e:
    print(f"✗ 无法连接到API: {e}")
    print("  请确保EasyDarwin服务正在运行")
except Exception as e:
    print(f"✗ 错误: {e}")
    import traceback
    traceback.print_exc()

print("\n" + "=" * 80)
print("检查完成")
print("=" * 80)

