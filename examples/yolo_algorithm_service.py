#!/usr/bin/env python3
"""
YOLO算法服务 - 真实推理实现

演示如何使用YOLO模型进行真实的目标检测，并返回符合EasyDarwin规范的结果。

功能：
1. 使用ultralytics YOLO进行目标检测
2. 下载MinIO图片并推理
3. 返回包含 total_count 的结果
4. 检测对象为0时，图片会被自动删除

依赖安装：
    pip install ultralytics opencv-python pillow requests

运行：
    python3 yolo_algorithm_service.py --easydarwin http://localhost:5066 --model yolov8n.pt
"""

import argparse
import json
import logging
import os
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib import request as urllib_request
from urllib.error import URLError

try:
    from ultralytics import YOLO
    import cv2
    from PIL import Image
    YOLO_AVAILABLE = True
except ImportError:
    YOLO_AVAILABLE = False
    print("⚠️  警告: ultralytics 未安装，将使用模拟推理")
    print("安装: pip install ultralytics opencv-python pillow")

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class YOLOAlgorithmService:
    """YOLO算法服务"""
    
    def __init__(self, service_id, name, model_path, task_types, port, easydarwin_url, confidence_threshold=0.5):
        self.service_id = service_id
        self.name = name
        self.model_path = model_path
        self.task_types = task_types
        self.port = port
        self.easydarwin_url = easydarwin_url
        self.endpoint = f"http://localhost:{port}/infer"
        self.registered = False
        self.confidence_threshold = confidence_threshold
        
        # 加载YOLO模型
        self.model = None
        if YOLO_AVAILABLE:
            try:
                logger.info(f"正在加载YOLO模型: {model_path}")
                self.model = YOLO(model_path)
                logger.info(f"✓ YOLO模型加载成功")
            except Exception as e:
                logger.error(f"✗ YOLO模型加载失败: {e}")
                logger.info("将使用模拟推理")
        else:
            logger.warning("ultralytics未安装，使用模拟推理")
        
        # 任务类型到YOLO类别的映射
        self.task_class_mapping = {
            '人数统计': ['person'],
            '人员跌倒': ['person'],  # 跌倒检测需要姿态估计，这里简化为检测人
            '车辆检测': ['car', 'truck', 'bus', 'motorcycle', 'bicycle'],
            '安全帽检测': ['person'],  # 需要自定义模型检测安全帽
            '吸烟检测': ['person'],    # 需要自定义模型检测吸烟
        }
        
    def register(self):
        """注册到EasyDarwin"""
        data = {
            "service_id": self.service_id,
            "name": self.name,
            "task_types": self.task_types,
            "endpoint": self.endpoint,
            "version": "1.0.0"
        }
        
        url = f"{self.easydarwin_url}/api/v1/ai_analysis/register"
        try:
            req = urllib_request.Request(
                url,
                data=json.dumps(data).encode('utf-8'),
                headers={'Content-Type': 'application/json'},
                method='POST'
            )
            with urllib_request.urlopen(req, timeout=5) as response:
                result = json.loads(response.read().decode('utf-8'))
                if result.get('ok'):
                    self.registered = True
                    logger.info(f"✓ 注册成功: {self.service_id}")
                    return True
                else:
                    logger.error(f"✗ 注册失败: {result}")
                    return False
        except Exception as e:
            logger.error(f"✗ 注册失败: {e}")
            return False
    
    def heartbeat(self):
        """发送心跳"""
        if not self.registered:
            return
        
        url = f"{self.easydarwin_url}/api/v1/ai_analysis/heartbeat/{self.service_id}"
        try:
            req = urllib_request.Request(url, method='POST')
            with urllib_request.urlopen(req, timeout=5) as response:
                result = json.loads(response.read().decode('utf-8'))
                if result.get('ok'):
                    logger.debug(f"♥ 心跳成功: {self.service_id}")
                else:
                    logger.warn(f"心跳失败: {result}")
        except Exception as e:
            logger.error(f"心跳失败: {e}")
    
    def start_heartbeat_loop(self):
        """启动心跳循环"""
        def loop():
            while self.registered:
                time.sleep(30)
                self.heartbeat()
        
        thread = threading.Thread(target=loop, daemon=True)
        thread.start()
        logger.info("心跳线程已启动（每30秒）")
    
    def download_image(self, image_url):
        """下载图片到临时文件"""
        try:
            # 创建临时文件
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix='.jpg')
            temp_path = temp_file.name
            temp_file.close()
            
            # 下载图片
            urllib_request.urlretrieve(image_url, temp_path)
            logger.debug(f"图片下载成功: {temp_path}")
            return temp_path
        except Exception as e:
            logger.error(f"图片下载失败: {e}")
            return None
    
    def infer_with_yolo(self, image_path, task_type):
        """使用YOLO模型进行推理"""
        try:
            # 执行推理
            results = self.model.predict(
                image_path,
                conf=self.confidence_threshold,
                verbose=False
            )
            
            if not results or len(results) == 0:
                return {
                    "total_count": 0,
                    "message": "推理失败或无结果"
                }
            
            result = results[0]
            boxes = result.boxes
            
            # 获取该任务类型关注的类别
            target_classes = self.task_class_mapping.get(task_type, None)
            
            detections = []
            for box in boxes:
                cls_id = int(box.cls[0])
                class_name = result.names[cls_id]
                confidence = float(box.conf[0])
                bbox = box.xyxy[0].tolist()  # [x1, y1, x2, y2]
                
                # 如果指定了目标类别，只返回匹配的
                if target_classes is None or class_name in target_classes:
                    detections.append({
                        "class": class_name,
                        "confidence": confidence,
                        "bbox": [int(x) for x in bbox]
                    })
            
            # 特殊任务处理
            if task_type == '人员跌倒':
                # 简化：这里只是检测人，实际跌倒检测需要姿态估计
                # 可以接入姿态估计模型判断跌倒
                return {
                    "total_count": 0,  # 假设未检测到跌倒
                    "fall_detected": False,
                    "persons": len(detections),
                    "message": "未检测到跌倒（需要姿态估计模型）"
                }
            
            return {
                "total_count": len(detections),
                "detections": detections,
                "message": f"检测到{len(detections)}个{task_type}目标"
            }
            
        except Exception as e:
            logger.error(f"YOLO推理失败: {e}")
            return {
                "total_count": 0,
                "error": str(e),
                "message": "推理异常"
            }
    
    def infer_simulated(self, task_type):
        """模拟推理（当YOLO不可用时）"""
        import random
        
        # 随机生成检测结果
        if random.random() < 0.3:  # 30%概率无检测结果
            return {
                "total_count": 0,
                "message": f"未检测到{task_type}目标（模拟）"
            }
        
        # 随机生成1-5个检测结果
        count = random.randint(1, 5)
        detections = []
        for i in range(count):
            detections.append({
                "class": task_type,
                "confidence": round(random.uniform(0.6, 0.95), 2),
                "bbox": [
                    random.randint(0, 400),
                    random.randint(0, 300),
                    random.randint(400, 800),
                    random.randint(300, 600)
                ]
            })
        
        return {
            "total_count": count,
            "detections": detections,
            "message": f"检测到{count}个{task_type}目标（模拟）"
        }
    
    def infer(self, image_url, task_id, task_type):
        """执行推理"""
        logger.info(f"开始推理: task_id={task_id}, task_type={task_type}")
        
        # 如果没有YOLO模型，使用模拟推理
        if self.model is None:
            logger.warning("使用模拟推理")
            return self.infer_simulated(task_type)
        
        # 下载图片
        image_path = self.download_image(image_url)
        if image_path is None:
            return {
                "total_count": 0,
                "error": "图片下载失败",
                "message": "无法获取图片"
            }
        
        try:
            # 使用YOLO推理
            result = self.infer_with_yolo(image_path, task_type)
            return result
        finally:
            # 清理临时文件
            try:
                if os.path.exists(image_path):
                    os.remove(image_path)
                    logger.debug(f"临时文件已删除: {image_path}")
            except Exception as e:
                logger.warning(f"临时文件删除失败: {e}")


class InferenceHandler(BaseHTTPRequestHandler):
    """推理请求处理器"""
    
    # 类变量，用于访问算法服务实例
    algorithm_service = None
    
    def do_POST(self):
        if self.path != '/infer':
            self.send_error(404, "Not Found")
            return
        
        # 读取请求体
        content_length = int(self.headers['Content-Length'])
        body = self.rfile.read(content_length)
        
        try:
            req_data = json.loads(body.decode('utf-8'))
            image_url = req_data.get('image_url')
            task_id = req_data.get('task_id')
            task_type = req_data.get('task_type')
            
            # 记录开始时间
            start_time = time.time()
            
            # 执行推理
            result = self.algorithm_service.infer(image_url, task_id, task_type)
            
            # 计算推理时间
            inference_time_ms = int((time.time() - start_time) * 1000)
            
            # 返回结果
            response = {
                "success": True,
                "result": result,
                "confidence": result.get('detections', [{}])[0].get('confidence', 0.0) if result.get('detections') else 0.0,
                "inference_time_ms": inference_time_ms
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(response).encode('utf-8'))
            
            logger.info(f"推理完成: task_id={task_id}, total_count={result.get('total_count', 0)}, time={inference_time_ms}ms")
            
        except Exception as e:
            logger.error(f"推理失败: {e}", exc_info=True)
            self.send_error(500, str(e))
    
    def log_message(self, format, *args):
        """禁用默认日志"""
        pass


def main():
    parser = argparse.ArgumentParser(description='YOLO算法服务')
    parser.add_argument('--service-id', default='yolo_detector_v1', help='服务ID')
    parser.add_argument('--name', default='YOLO目标检测服务', help='服务名称')
    parser.add_argument('--model', default='yolov8n.pt', help='YOLO模型路径')
    parser.add_argument('--task-types', nargs='+', 
                       default=['人数统计', '车辆检测', '人员跌倒', '安全帽检测', '吸烟检测'],
                       help='支持的任务类型')
    parser.add_argument('--port', type=int, default=8000, help='HTTP服务端口')
    parser.add_argument('--easydarwin', default='http://localhost:5066', help='EasyDarwin地址')
    parser.add_argument('--confidence', type=float, default=0.5, help='置信度阈值')
    
    args = parser.parse_args()
    
    # 创建算法服务
    service = YOLOAlgorithmService(
        service_id=args.service_id,
        name=args.name,
        model_path=args.model,
        task_types=args.task_types,
        port=args.port,
        easydarwin_url=args.easydarwin,
        confidence_threshold=args.confidence
    )
    
    # 将服务实例设置到Handler类变量
    InferenceHandler.algorithm_service = service
    
    # 注册到EasyDarwin
    logger.info(f"正在注册到 {args.easydarwin}...")
    if not service.register():
        logger.error("注册失败，退出")
        return
    
    # 启动心跳
    service.start_heartbeat_loop()
    
    # 启动HTTP服务
    server_address = ('', args.port)
    httpd = HTTPServer(server_address, InferenceHandler)
    
    logger.info(f"YOLO算法服务已启动")
    logger.info(f"  服务ID: {service.service_id}")
    logger.info(f"  服务名称: {service.name}")
    logger.info(f"  模型: {args.model}")
    logger.info(f"  支持类型: {service.task_types}")
    logger.info(f"  监听端口: {args.port}")
    logger.info(f"  推理端点: {service.endpoint}")
    logger.info(f"  置信度阈值: {args.confidence}")
    logger.info(f"  YOLO可用: {'是' if service.model else '否（使用模拟推理）'}")
    logger.info("等待推理请求...")
    logger.info("")
    logger.info("💡 提示:")
    logger.info("  - total_count = 0 时，图片会被自动删除")
    logger.info("  - total_count > 0 时，会保存告警并推送到MQ")
    logger.info("")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        logger.info("收到停止信号")
        httpd.shutdown()


if __name__ == '__main__':
    main()

