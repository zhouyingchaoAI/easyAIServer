#!/usr/bin/env python3
"""
诊断并发异步问题
"""
import json
import urllib.request
import sqlite3
from datetime import datetime, timedelta

print("=" * 80)
print("并发异步问题诊断")
print("=" * 80)

# 1. 检查推理统计
print("\n1. 推理系统状态:")
print("-" * 80)
try:
    with urllib.request.urlopen('http://localhost:5066/api/v1/ai_analysis/inference_stats') as response:
        stats = json.loads(response.read().decode())
        print(f"已处理图片: {stats['processed_total']}")
        print(f"队列大小: {stats['queue_size']}/{stats['queue_max_size']}")
        print(f"推理成功: {stats['total_inferences']}")
        print(f"推理失败: {stats['failed_inferences']}")
        print(f"平均推理时间: {stats['avg_inference_ms']:.2f}ms")
        print(f"丢弃图片: {stats['dropped_total']}")
except Exception as e:
    print(f"无法获取推理统计: {e}")

# 2. 检查数据库
print("\n2. 数据库状态:")
print("-" * 80)
conn = sqlite3.connect('./configs/data.db')
cursor = conn.cursor()

# 最后一条告警
cursor.execute("SELECT MAX(created_at), COUNT(*) FROM alerts")
last_alert, total_alerts = cursor.fetchone()
print(f"总告警数: {total_alerts}")
print(f"最后告警时间: {last_alert}")

if last_alert:
    last_time = datetime.fromisoformat(last_alert.replace('+00:00', ''))
    hours_ago = (datetime.utcnow() - last_time).total_seconds() / 3600
    print(f"距今: {hours_ago:.1f} 小时")

conn.close()

# 3. 检查配置
print("\n3. 关键配置:")
print("-" * 80)
with open('./configs/config.toml', 'r') as f:
    content = f.read()
    for line in content.split('\n'):
        if any(key in line for key in ['save_only_with_detection', 'alert_batch_enabled', 'alert_batch_size', 'alert_batch_interval']):
            print(f"  {line.strip()}")

# 4. 诊断结论
print("\n" + "=" * 80)
print("诊断结论:")
print("=" * 80)

# 计算推理成功数和数据库告警数的差异
try:
    with urllib.request.urlopen('http://localhost:5066/api/v1/ai_analysis/inference_stats') as response:
        stats = json.loads(response.read().decode())
        inferences = stats['total_inferences']
        
        conn = sqlite3.connect('./configs/data.db')
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) FROM alerts")
        db_alerts = cursor.fetchone()[0]
        conn.close()
        
        diff = inferences - db_alerts
        
        print(f"推理成功: {inferences} 次")
        print(f"数据库告警: {db_alerts} 条")
        print(f"差异: {diff} 条")
        
        if diff > 100:
            print("\n⚠️  可能的问题:")
            print("  1. 批量写入器缓冲区未刷新（batch_size=100，可能有<100条在缓冲区）")
            print("  2. save_only_with_detection=true 导致无检测结果的图片不保存")
            print("  3. 批量写入器卡住或死锁")
            
            print("\n解决方案:")
            print("  1. 重启服务强制刷新批量写入缓冲区")
            print("  2. 或者临时降低batch_size配置触发写入")
            print("  3. 或者设置 save_only_with_detection = false 保存所有推理结果")
        
        elif hours_ago > 24:
            print("\n⚠️  29小时没有新告警，但推理在正常进行")
            print("  可能原因：检测不到目标（save_only_with_detection=true）")
            print("  建议：临时设置 save_only_with_detection = false 验证")
        
        else:
            print("\n✓ 系统运行正常")
            
except Exception as e:
    print(f"诊断失败: {e}")

print("\n" + "=" * 80)
print("并发安全检查:")
print("=" * 80)
print("代码审查发现:")
print("  ✓ ImageInfo在ScheduleInference时就携带了正确的TaskID")
print("  ✓ 推理请求req包含独立的image.TaskID副本")
print("  ✓ 批量写入使用了互斥锁保护")
print("  ✗ 但是：批量写入缓冲区可能长时间不刷新")
print("\n建议：")
print("  1. 降低alert_batch_size从100到10，加快刷新")
print("  2. 降低alert_batch_interval_sec从2秒到1秒")
print("  3. 重启服务强制刷新当前缓冲区")

