#!/usr/bin/env python3
"""
测试算法服务注册脚本
"""
import requests
import json
import time
import threading

# yanying平台地址
YANYING_HOST = "http://localhost:5066"

# 服务信息
SERVICE_CONFIG = {
    "service_id": "test_service_001",
    "name": "测试算法服务",
    "task_types": ["人数统计", "人员跌倒"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
}

def register_service():
    """注册算法服务"""
    url = f"{YANYING_HOST}/api/v1/ai_analysis/register"
    
    print(f"正在注册服务到: {url}")
    print(f"服务配置: {json.dumps(SERVICE_CONFIG, indent=2, ensure_ascii=False)}")
    
    try:
        response = requests.post(url, json=SERVICE_CONFIG, timeout=5)
        print(f"\n注册响应状态码: {response.status_code}")
        print(f"注册响应内容: {response.text}")
        
        if response.status_code == 200:
            print("✅ 服务注册成功！")
            return True
        else:
            print(f"❌ 服务注册失败: {response.text}")
            return False
    except Exception as e:
        print(f"❌ 注册请求失败: {e}")
        return False

def send_heartbeat():
    """发送心跳"""
    url = f"{YANYING_HOST}/api/v1/ai_analysis/heartbeat/{SERVICE_CONFIG['service_id']}"
    
    print(f"\n发送心跳到: {url}")
    
    try:
        response = requests.post(url, timeout=5)
        print(f"心跳响应状态码: {response.status_code}")
        print(f"心跳响应内容: {response.text}")
        
        if response.status_code == 200:
            print("✅ 心跳发送成功！")
            return True
        else:
            print(f"❌ 心跳发送失败: {response.text}")
            return False
    except Exception as e:
        print(f"❌ 心跳请求失败: {e}")
        return False

def get_services():
    """获取所有注册的服务"""
    url = f"{YANYING_HOST}/api/v1/ai_analysis/services"
    
    print(f"\n查询服务列表: {url}")
    
    try:
        response = requests.get(url, timeout=5)
        print(f"查询响应状态码: {response.status_code}")
        
        if response.status_code == 200:
            data = response.json()
            print(f"服务总数: {data.get('total', 0)}")
            
            if data.get('services'):
                print("\n已注册的服务:")
                for svc in data['services']:
                    print(f"  - ID: {svc.get('service_id')}")
                    print(f"    名称: {svc.get('name')}")
                    print(f"    状态: {svc.get('status')}")
                    print(f"    任务类型: {', '.join(svc.get('task_types', []))}")
                    print()
            else:
                print("暂无注册的服务")
            
            return True
        else:
            print(f"❌ 查询失败: {response.text}")
            return False
    except Exception as e:
        print(f"❌ 查询请求失败: {e}")
        return False

def heartbeat_loop():
    """心跳循环"""
    while True:
        time.sleep(30)  # 每30秒发送一次心跳
        send_heartbeat()

if __name__ == "__main__":
    print("="*60)
    print("yanying AI服务注册测试工具")
    print("="*60)
    
    # 1. 注册服务
    if register_service():
        # 2. 查询服务列表
        time.sleep(1)
        get_services()
        
        # 3. 发送心跳
        time.sleep(1)
        send_heartbeat()
        
        # 4. 再次查询服务列表
        time.sleep(1)
        get_services()
        
        print("\n✅ 测试完成！服务已注册并发送心跳。")
        print("您可以在Web界面查看: http://localhost:5066/#/ai-services")
        
        # 5. 启动心跳循环
        print("\n开始心跳循环（每30秒）...")
        print("按 Ctrl+C 停止")
        
        try:
            heartbeat_loop()
        except KeyboardInterrupt:
            print("\n\n停止心跳循环")
    else:
        print("\n❌ 服务注册失败，请检查yanying是否正常运行")

