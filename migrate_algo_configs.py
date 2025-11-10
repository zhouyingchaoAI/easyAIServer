#!/usr/bin/env python3
"""
检查并迁移算法配置文件到正确路径
"""
import json
import sys

# 从API获取所有任务
import urllib.request

def get_tasks():
    try:
        with urllib.request.urlopen('http://localhost:5066/api/v1/frame_extractor/tasks') as response:
            data = json.loads(response.read().decode())
            return data.get('tasks', [])
    except Exception as e:
        print(f"获取任务列表失败: {e}")
        return []

def check_task_config(task):
    task_id = task.get('id', '')
    task_type = task.get('task_type', '')
    output_path = task.get('output_path', '')
    
    print(f"\n任务: {task_id}")
    print(f"  类型: {task_type}")
    print(f"  OutputPath: {output_path}")
    print(f"  ID==OutputPath: {task_id == output_path}")
    
    # 检查配置是否能读取
    try:
        url = f'http://localhost:5066/api/v1/frame_extractor/tasks/{task_id}/config'
        with urllib.request.urlopen(url) as response:
            config = json.loads(response.read().decode())
            print(f"  ✓ 配置文件可读取 (regions: {len(config.get('regions', []))})")
            return True
    except urllib.error.HTTPError as e:
        if e.code == 404:
            print(f"  ✗ 配置文件不存在")
        else:
            print(f"  ✗ 读取失败: {e}")
        return False
    except Exception as e:
        print(f"  ✗ 错误: {e}")
        return False

def main():
    print("=" * 60)
    print("检查算法配置文件路径")
    print("=" * 60)
    
    tasks = get_tasks()
    if not tasks:
        print("没有找到任务")
        return
    
    print(f"找到 {len(tasks)} 个任务\n")
    
    config_ok = []
    config_missing = []
    config_mismatch = []
    
    for task in tasks:
        task_id = task.get('id', '')
        output_path = task.get('output_path', '')
        
        if task_id != output_path:
            config_mismatch.append(task_id)
        
        if check_task_config(task):
            config_ok.append(task_id)
        else:
            config_missing.append(task_id)
    
    # 统计报告
    print("\n" + "=" * 60)
    print("统计报告")
    print("=" * 60)
    print(f"配置正常: {len(config_ok)} 个")
    print(f"配置缺失: {len(config_missing)} 个")
    print(f"ID!=OutputPath: {len(config_mismatch)} 个")
    
    if config_missing:
        print(f"\n缺失配置的任务: {', '.join(config_missing)}")
    
    if config_mismatch:
        print(f"\nID不匹配的任务: {', '.join(config_mismatch)}")
        print("\n⚠️  建议：")
        print("  1. 停止这些任务")
        print("  2. 重新配置算法参数")
        print("  3. 重新启动任务")

if __name__ == "__main__":
    main()
