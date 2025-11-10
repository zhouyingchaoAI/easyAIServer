#!/usr/bin/env python3
"""
算法服务示例 - 带性能统计功能
演示如何在心跳时报告性能指标
"""

import time
import threading
import requests
import json
from collections import deque
from datetime import datetime

# ==================== 配置 ====================

EASYDARWIN_URL = "http://172.16.5.207:5066"
SERVICE_ID = "example_algo_service_001"
SERVICE_NAME = "示例算法服务"
SERVICE_PORT = 8000
TASK_TYPES = ["人数统计", "客流分析"]
HEARTBEAT_INTERVAL = 30  # 心跳间隔（秒）

# ==================== 性能统计类 ====================

class PerformanceStats:
    """性能统计"""
    
    def __init__(self, window_size=50):
        self.window_size = window_size
        self.total_requests = 0
        self.inference_times = deque(maxlen=window_size)  # 保留最近N次推理时间
        self.last_inference_time_ms = 0
        self.last_total_time_ms = 0
        self.lock = threading.Lock()
    
    def record_inference(self, inference_time_ms, total_time_ms):
        """记录一次推理"""
        with self.lock:
            self.total_requests += 1
            self.last_inference_time_ms = inference_time_ms
            self.last_total_time_ms = total_time_ms
            self.inference_times.append(inference_time_ms)
    
    def get_avg_inference_time(self):
        """获取平均推理时间"""
        with self.lock:
            if len(self.inference_times) == 0:
                return 0.0
            return sum(self.inference_times) / len(self.inference_times)
    
    def get_stats_dict(self):
        """获取统计数据字典"""
        with self.lock:
            return {
                "total_requests": self.total_requests,
                "avg_inference_time_ms": round(self.get_avg_inference_time(), 2),
                "last_inference_time_ms": round(self.last_inference_time_ms, 2),
                "last_total_time_ms": round(self.last_total_time_ms, 2)
            }
    
    def reset(self):
        """重置统计"""
        with self.lock:
            self.total_requests = 0
            self.inference_times.clear()
            self.last_inference_time_ms = 0
            self.last_total_time_ms = 0

# ==================== 全局变量 ====================

stats = PerformanceStats(window_size=50)
registered = False

# ==================== 核心功能 ====================

def register_service():
    """注册算法服务"""
    global registered
    
    data = {
        "service_id": SERVICE_ID,
        "name": SERVICE_NAME,
        "task_types": TASK_TYPES,
        "endpoint": f"http://172.16.5.207:{SERVICE_PORT}/infer",
        "version": "1.0.0"
    }
    
    try:
        response = requests.post(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/register",
            json=data,
            timeout=5
        )
        
        if response.status_code == 200:
            result = response.json()
            if result.get('ok'):
                registered = True
                print(f"✅ 注册成功: {SERVICE_ID}")
                print(f"   端点: {data['endpoint']}")
                print(f"   任务类型: {', '.join(TASK_TYPES)}")
                return True
        
        print(f"❌ 注册失败: HTTP {response.status_code}")
        print(f"   响应: {response.text}")
        return False
        
    except Exception as e:
        print(f"❌ 注册异常: {e}")
        return False

def send_heartbeat():
    """发送心跳（携带性能统计）"""
    if not registered:
        return
    
    # 获取性能统计
    stats_data = stats.get_stats_dict()
    
    try:
        response = requests.post(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/heartbeat/{SERVICE_ID}",
            json=stats_data,  # 携带性能统计
            timeout=5
        )
        
        if response.status_code == 200:
            timestamp = datetime.now().strftime('%H:%M:%S')
            print(f"💓 [{timestamp}] 心跳成功")
            print(f"   累积请求: {stats_data['total_requests']}")
            print(f"   平均耗时: {stats_data['avg_inference_time_ms']:.2f}ms")
            print(f"   最近推理: {stats_data['last_inference_time_ms']:.2f}ms")
            print(f"   最近总耗: {stats_data['last_total_time_ms']:.2f}ms")
        else:
            print(f"⚠️ 心跳失败: HTTP {response.status_code}")
            
    except Exception as e:
        print(f"❌ 心跳异常: {e}")

def heartbeat_loop():
    """心跳循环线程"""
    print(f"💓 心跳线程已启动（间隔{HEARTBEAT_INTERVAL}秒）\n")
    
    while True:
        try:
            time.sleep(HEARTBEAT_INTERVAL)
            send_heartbeat()
            
        except Exception as e:
            print(f"❌ 心跳线程异常: {e}")

def simulate_inference(image_url, task_id, task_type):
    """模拟推理（实际应该替换为真实模型推理）"""
    total_start = time.time()
    
    # 1. 模拟下载图片
    time.sleep(0.02)  # 20ms
    
    # 2. 模拟模型推理
    inference_start = time.time()
    time.sleep(0.05)  # 50ms - 这是纯推理时间
    inference_time_ms = (time.time() - inference_start) * 1000
    
    # 3. 模拟后处理
    time.sleep(0.01)  # 10ms
    
    # 计算总耗时
    total_time_ms = (time.time() - total_start) * 1000
    
    # 记录性能统计
    stats.record_inference(inference_time_ms, total_time_ms)
    
    # 返回结果
    return {
        "success": True,
        "result": {
            "detections": [],
            "total_count": 5
        },
        "confidence": 0.95,
        "inference_time_ms": inference_time_ms  # 返回给EasyDarwin
    }

# ==================== 主程序 ====================

def main():
    print("=" * 60)
    print("🤖 算法服务示例（带性能统计）")
    print("=" * 60)
    print()
    
    # 1. 注册服务
    print("📝 正在注册服务...")
    if not register_service():
        print("❌ 注册失败，退出")
        return
    
    print()
    
    # 2. 启动心跳线程
    heartbeat_thread = threading.Thread(target=heartbeat_loop, daemon=True)
    heartbeat_thread.start()
    
    # 3. 模拟推理请求
    print("🔄 模拟推理请求（每5秒一次）")
    print("   观察心跳输出的性能统计变化\n")
    print("=" * 60)
    print()
    
    request_count = 0
    while True:
        try:
            time.sleep(5)
            
            # 模拟推理
            request_count += 1
            result = simulate_inference(
                image_url="http://example.com/image.jpg",
                task_id="test_001",
                task_type="人数统计"
            )
            
            current_stats = stats.get_stats_dict()
            print(f"📸 推理 #{request_count} 完成:")
            print(f"   推理时间: {current_stats['last_inference_time_ms']:.2f}ms")
            print(f"   总耗时: {current_stats['last_total_time_ms']:.2f}ms")
            print(f"   平均耗时: {current_stats['avg_inference_time_ms']:.2f}ms")
            print(f"   累积次数: {current_stats['total_requests']}")
            print()
            
        except KeyboardInterrupt:
            print("\n👋 正在退出...")
            break
        except Exception as e:
            print(f"❌ 错误: {e}")

if __name__ == "__main__":
    main()

