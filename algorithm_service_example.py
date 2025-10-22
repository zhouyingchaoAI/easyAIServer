#!/usr/bin/env python3
"""
EasyDarwin算法服务对接示例代码
功能: 绊线人数统计算法服务

使用方法:
1. 安装依赖: pip install flask requests opencv-python numpy
2. 修改配置: 修改下方的配置变量
3. 运行服务: python algorithm_service_example.py
4. 查看日志: 确认注册成功并等待推理请求

作者: EasyDarwin Team
版本: v1.0
日期: 2025-10-20
"""

from flask import Flask, request, jsonify
import requests
import cv2
import numpy as np
import time
import uuid
import threading
import socket
import json
from datetime import datetime

# ==================== 配置区域（请修改为您的实际配置） ====================

# EasyDarwin主系统地址
EASYDARWIN_URL = "http://10.1.6.230:5066"

# 您的算法服务配置
SERVICE_HOST = "0.0.0.0"                          # 服务监听地址（通常不需要改）
SERVICE_PORT = 8000                                # 服务端口
SERVICE_NAME = "绊线人数统计算法服务"              # 服务名称
SERVICE_VERSION = "1.0.0"                         # 版本号

# 支持的任务类型（根据您的算法能力配置）
TASK_TYPES = [
    "绊线人数统计",
    "人数统计"
]

# 心跳间隔（秒）
HEARTBEAT_INTERVAL = 45

# ============================================================================

# 全局变量
SERVICE_ID = str(uuid.uuid4())
app = Flask(__name__)

# ==================== 核心接口实现 ====================

@app.route('/infer', methods=['POST'])
def infer():
    """
    推理接口（核心）
    
    EasyDarwin会向此接口发送推理请求
    """
    try:
        # 解析请求
        data = request.json
        
        image_url = data.get('image_url')
        task_id = data.get('task_id')
        task_type = data.get('task_type')
        image_path = data.get('image_path')
        algo_config = data.get('algo_config', {})
        algo_config_url = data.get('algo_config_url', '')
        
        # 打印请求信息
        print(f"\n{'='*60}")
        print(f"📨 收到推理请求 [{datetime.now().strftime('%H:%M:%S')}]")
        print(f"  任务ID: {task_id}")
        print(f"  任务类型: {task_type}")
        print(f"  图片路径: {image_path}")
        print(f"  图片URL: {image_url[:80]}...")
        if algo_config_url:
            print(f"  配置URL: {algo_config_url[:80]}...")
        print(f"  检测线数: {len([r for r in algo_config.get('regions', []) if r['type'] == 'line'])}")
        print(f"{'='*60}\n")
        
        # 下载图片
        print("📥 正在下载图片...")
        image = download_image(image_url)
        if image is None:
            print("❌ 图片下载失败")
            return error_response("图片下载失败")
        
        print(f"✅ 图片下载成功: {image.shape}")
        
        # 执行推理
        print("🔍 开始推理...")
        start_time = time.time()
        
        if task_type in ["绊线人数统计", "人数统计"]:
            result = tripwire_counting_algorithm(image, algo_config)
        else:
            result = default_algorithm(image, algo_config)
        
        inference_time = int((time.time() - start_time) * 1000)
        
        print(f"✅ 推理完成: 耗时{inference_time}ms")
        print(f"   检测数: {result.get('total_count', 0)}")
        print(f"   置信度: {result.get('avg_confidence', 0.0):.3f}")
        
        # 返回结果
        return jsonify({
            "success": True,
            "result": result,
            "confidence": result.get('avg_confidence', 0.0),
            "inference_time_ms": inference_time
        })
        
    except Exception as e:
        print(f"❌ 推理异常: {str(e)}")
        import traceback
        traceback.print_exc()
        return error_response(str(e))

@app.route('/health', methods=['GET'])
def health():
    """健康检查接口（可选）"""
    return jsonify({
        "status": "healthy",
        "service_id": SERVICE_ID,
        "uptime": time.time()
    })

def error_response(error_msg):
    """构建错误响应"""
    return jsonify({
        "success": False,
        "error": error_msg,
        "result": None,
        "confidence": 0.0,
        "inference_time_ms": 0
    })

# ==================== 算法实现区域 ====================

def tripwire_counting_algorithm(image, config):
    """
    绊线人数统计算法（示例实现）
    
    TODO: 替换为您的实际算法实现
    
    Args:
        image: OpenCV图片 (numpy array, BGR格式)
        config: 算法配置
            {
                "regions": [...],
                "algorithm_params": {...}
            }
    
    Returns:
        {
            "total_count": int,
            "detections": [...],
            "crossings": [...]
        }
    """
    # 解析配置
    regions = config.get('regions', [])
    params = config.get('algorithm_params', {})
    
    conf_threshold = params.get('confidence_threshold', 0.7)
    iou_threshold = params.get('iou_threshold', 0.5)
    
    # 提取检测线
    lines = [r for r in regions if r['type'] == 'line' and r.get('enabled', True)]
    
    print(f"  算法参数: conf={conf_threshold}, iou={iou_threshold}")
    print(f"  检测线数: {len(lines)}")
    
    # ========== TODO: 实现您的算法 ==========
    
    # 示例1: 人员检测（需要替换为您的YOLO/其他模型）
    persons = detect_persons_mock(image, conf_threshold)
    
    # 示例2: 轨迹跟踪（需要实现DeepSORT或其他跟踪算法）
    tracks = track_persons_mock(persons)
    
    # 示例3: 绊线判断（需要实现线段交叉检测）
    crossings = []
    for line in lines:
        direction = line['properties']['direction']
        points = line['points']
        line_name = line.get('name', 'line')
        
        # 判断穿越（示例）
        for track in tracks:
            if check_line_crossing_mock(track, points, direction):
                crossings.append({
                    "line_name": line_name,
                    "direction": direction,
                    "person_id": track['id'],
                    "cross_point": track['center'],
                    "confidence": track['confidence']
                })
    
    # ==========================================
    
    # 计算平均置信度
    avg_conf = 0.0
    if persons:
        avg_conf = sum([p['confidence'] for p in persons]) / len(persons)
    
    return {
        "total_count": len(crossings),
        "detections": persons,
        "crossings": crossings,
        "avg_confidence": round(avg_conf, 3),
        "lines_checked": len(lines)
    }

def default_algorithm(image, config):
    """默认算法（示例）"""
    return {
        "total_count": 0,
        "detections": [],
        "avg_confidence": 0.0
    }

# ==================== 辅助函数（示例实现，需替换） ====================

def detect_persons_mock(image, conf_threshold):
    """
    人员检测（Mock实现）
    
    TODO: 替换为您的实际YOLO或其他检测模型
    """
    # 示例：随机生成检测结果用于测试
    return [
        {
            "class": "person",
            "confidence": 0.95,
            "bbox": [350, 150, 100, 250],  # [x, y, w, h]
            "center": [400, 275],
            "track_id": "track_1"
        },
        {
            "class": "person",
            "confidence": 0.88,
            "bbox": [600, 180, 95, 240],
            "center": [647, 300],
            "track_id": "track_2"
        }
    ]

def track_persons_mock(persons):
    """
    轨迹跟踪（Mock实现）
    
    TODO: 实现DeepSORT或其他跟踪算法
    """
    # 示例：直接返回检测结果
    return [{
        "id": p['track_id'],
        "confidence": p['confidence'],
        "center": p['center'],
        "history": [p['center']]  # 历史轨迹点
    } for p in persons]

def check_line_crossing_mock(track, line_points, direction):
    """
    绊线判断（Mock实现）
    
    TODO: 实现实际的线段交叉判断
    需要检查轨迹是否从一侧穿过到另一侧
    """
    # 示例：随机返回（实际需要准确的几何计算）
    import random
    return random.random() > 0.7

# ==================== 工具函数 ====================

def download_image(url):
    """下载MinIO图片"""
    try:
        response = requests.get(url, timeout=10)
        if response.status_code == 200:
            # 将字节转换为OpenCV图片
            arr = np.frombuffer(response.content, np.uint8)
            img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
            
            if img is None:
                print("⚠️ 图片解码失败")
                return None
            
            return img
        else:
            print(f"⚠️ HTTP {response.status_code}")
            return None
    except Exception as e:
        print(f"⚠️ 下载异常: {e}")
        return None

def get_local_ip():
    """获取本机IP地址"""
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("8.8.8.8", 80))
        ip = s.getsockname()[0]
        s.close()
        return ip
    except:
        return "127.0.0.1"

# ==================== 服务注册和心跳 ====================

def register_to_easydarwin():
    """注册服务到EasyDarwin"""
    local_ip = get_local_ip()
    
    service_info = {
        "service_id": SERVICE_ID,
        "name": SERVICE_NAME,
        "task_types": TASK_TYPES,
        "endpoint": f"http://{local_ip}:{SERVICE_PORT}/infer",
        "version": SERVICE_VERSION
    }
    
    print(f"\n{'='*60}")
    print(f"📡 正在注册服务到EasyDarwin...")
    print(f"  EasyDarwin地址: {EASYDARWIN_URL}")
    print(f"  服务ID: {SERVICE_ID}")
    print(f"  服务名称: {SERVICE_NAME}")
    print(f"  推理端点: {service_info['endpoint']}")
    print(f"  支持类型: {', '.join(TASK_TYPES)}")
    print(f"{'='*60}\n")
    
    try:
        response = requests.post(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/register",
            json=service_info,
            timeout=10
        )
        
        if response.status_code == 200:
            print("✅ 服务注册成功!")
            print(f"   可在EasyDarwin前端查看已注册服务")
            return True
        else:
            print(f"❌ 注册失败: HTTP {response.status_code}")
            print(f"   响应: {response.text}")
            return False
            
    except requests.exceptions.ConnectionError:
        print(f"❌ 无法连接到EasyDarwin: {EASYDARWIN_URL}")
        print(f"   请检查:")
        print(f"   1. EasyDarwin是否正在运行")
        print(f"   2. 地址和端口是否正确")
        print(f"   3. 网络连接是否正常")
        return False
    except Exception as e:
        print(f"❌ 注册异常: {e}")
        return False

def heartbeat_loop():
    """心跳循环线程"""
    print(f"💓 心跳线程已启动（间隔{HEARTBEAT_INTERVAL}秒）\n")
    
    while True:
        try:
            time.sleep(HEARTBEAT_INTERVAL)
            
            response = requests.post(
                f"{EASYDARWIN_URL}/api/v1/ai_analysis/heartbeat/{SERVICE_ID}",
                timeout=5
            )
            
            if response.status_code == 200:
                timestamp = datetime.now().strftime('%H:%M:%S')
                print(f"💓 [{timestamp}] 心跳发送成功")
            else:
                print(f"⚠️ 心跳失败: HTTP {response.status_code}")
                
        except Exception as e:
            print(f"❌ 心跳异常: {e}")

def unregister_service():
    """注销服务"""
    try:
        print(f"\n📡 正在注销服务...")
        response = requests.delete(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/unregister/{SERVICE_ID}",
            timeout=5
        )
        if response.status_code == 200:
            print("✅ 服务已注销")
    except:
        pass

# ==================== 主程序 ====================

def print_banner():
    """打印启动横幅"""
    print("\n" + "="*60)
    print("  EasyDarwin 算法服务")
    print("  绊线人数统计示例")
    print("="*60)
    print(f"  版本: {SERVICE_VERSION}")
    print(f"  服务ID: {SERVICE_ID}")
    print(f"  监听端口: {SERVICE_PORT}")
    print("="*60 + "\n")

def print_help():
    """打印使用帮助"""
    print("\n📖 使用说明:")
    print("  1. 确保EasyDarwin正在运行")
    print("  2. 修改代码中的EASYDARWIN_URL配置")
    print("  3. 运行此脚本")
    print("  4. 在EasyDarwin中创建任务（类型选'绊线人数统计'）")
    print("  5. 配置检测线并启动任务")
    print("  6. 查看此窗口的推理请求日志\n")
    
    print("💡 调试技巧:")
    print("  - 查看EasyDarwin日志: tail -f logs/sugar.log")
    print("  - 查看已注册服务: curl http://10.1.6.230:5066/api/v1/ai_analysis/services")
    print("  - 测试推理接口: curl -X POST http://localhost:8000/infer -H 'Content-Type: application/json' -d '{...}'\n")

if __name__ == '__main__':
    try:
        # 打印横幅
        print_banner()
        print_help()
        
        # 注册服务
        if not register_to_easydarwin():
            print("\n❌ 服务注册失败，无法继续")
            print("   请检查EasyDarwin是否运行并重试\n")
            exit(1)
        
        # 启动心跳线程
        heartbeat_thread = threading.Thread(target=heartbeat_loop, daemon=True)
        heartbeat_thread.start()
        
        # 启动Flask服务
        print(f"\n🚀 算法服务已启动")
        print(f"   监听地址: http://0.0.0.0:{SERVICE_PORT}")
        print(f"   健康检查: http://localhost:{SERVICE_PORT}/health")
        print(f"   推理接口: http://localhost:{SERVICE_PORT}/infer")
        print(f"\n📡 等待推理请求...\n")
        
        app.run(
            host=SERVICE_HOST, 
            port=SERVICE_PORT,
            debug=False,  # 生产环境设为False
            threaded=True
        )
        
    except KeyboardInterrupt:
        print("\n\n⚠️ 收到退出信号...")
        unregister_service()
        print("👋 服务已停止\n")
    except Exception as e:
        print(f"\n❌ 启动失败: {e}\n")

# ==================== 示例算法实现（需要替换为真实实现） ====================

"""
⚠️ 注意: 以下是Mock实现，仅用于演示！

实际使用时，请替换为您的真实算法：
1. detect_persons_mock → 使用YOLO/Faster-RCNN等检测模型
2. track_persons_mock → 使用DeepSORT/ByteTrack等跟踪算法
3. check_line_crossing_mock → 实现准确的线段交叉判断
"""

def detect_persons_mock(image, conf_threshold):
    """
    Mock人员检测
    
    TODO: 替换为真实的YOLO/检测模型
    """
    # 示例：生成随机检测结果
    import random
    
    height, width = image.shape[:2]
    num_persons = random.randint(0, 3)
    
    detections = []
    for i in range(num_persons):
        x = random.randint(50, width - 150)
        y = random.randint(50, height - 300)
        w = random.randint(80, 120)
        h = random.randint(200, 280)
        
        detections.append({
            "class": "person",
            "confidence": round(random.uniform(conf_threshold, 1.0), 3),
            "bbox": [x, y, w, h],
            "center": [x + w//2, y + h//2],
            "track_id": f"track_{i+1}"
        })
    
    return detections

def track_persons_mock(persons):
    """Mock轨迹跟踪"""
    return [{
        "id": p['track_id'],
        "confidence": p['confidence'],
        "center": p['center'],
        "history": [p['center']]
    } for p in persons]

def check_line_crossing_mock(track, line_points, direction):
    """Mock绊线判断"""
    # 示例：30%概率判定为穿越
    import random
    return random.random() > 0.7

# ==================== 提示信息 ====================

"""
🎯 实现您的算法的步骤:

1. 替换 detect_persons_mock
   - 加载您的检测模型（YOLO/Faster-RCNN/等）
   - 实现人员检测逻辑
   - 返回检测框和置信度

2. 替换 track_persons_mock
   - 实现多目标跟踪（DeepSORT/ByteTrack/等）
   - 为每个人分配唯一ID
   - 维护轨迹历史

3. 替换 check_line_crossing_mock
   - 实现线段交叉判断
   - 判断穿越方向（上→下 或 下→上）
   - 与配置的direction匹配

4. 优化性能
   - 使用GPU加速
   - 批量推理
   - 配置缓存
   - 连接池复用

推荐库:
  - 检测: ultralytics (YOLO), detectron2
  - 跟踪: deep_sort_realtime, norfair
  - 图像: opencv-python, pillow
  - 几何计算: shapely
"""



