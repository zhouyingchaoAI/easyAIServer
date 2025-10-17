#!/usr/bin/env python3
"""
算法服务示例

演示如何开发一个算法服务并注册到EasyDarwin AI分析插件。

功能：
1. 启动HTTP服务接收推理请求
2. 注册到EasyDarwin
3. 定时发送心跳
4. 执行推理并返回结果

运行：
    python3 algorithm_service.py --easydarwin http://localhost:5066
"""

import argparse
import json
import logging
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib import request as urllib_request
from urllib.error import URLError

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class AlgorithmService:
    """算法服务"""
    
    def __init__(self, service_id, name, task_types, port, easydarwin_url):
        self.service_id = service_id
        self.name = name
        self.task_types = task_types
        self.port = port
        self.easydarwin_url = easydarwin_url
        self.endpoint = f"http://localhost:{port}/infer"
        self.registered = False
        
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


class InferenceHandler(BaseHTTPRequestHandler):
    """推理请求处理器"""
    
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
            
            logger.info(f"收到推理请求: task_id={task_id}, task_type={task_type}")
            
            # 执行推理（这里是模拟）
            result = self.infer(image_url, task_type)
            
            # 返回结果
            response = {
                "success": True,
                "result": result,
                "confidence": 0.95,
                "inference_time_ms": 45
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps(response).encode('utf-8'))
            
            logger.info(f"推理完成: task_id={task_id}, result={result}")
            
        except Exception as e:
            logger.error(f"推理失败: {e}")
            self.send_error(500, str(e))
    
    def infer(self, image_url, task_type):
        """执行推理（示例实现）
        
        重要提示：
        1. 推理结果必须返回 total_count 字段表示检测对象数量
        2. 如果 total_count = 0，图片会被自动删除（启用 save_only_with_detection 时）
        3. 如果 total_count > 0，会保存告警记录并推送到消息队列
        
        实际应用中应该：
        1. 下载图片: urllib.request.urlretrieve(image_url, '/tmp/image.jpg')
        2. 加载模型: model = YOLO('yolov8n.pt')
        3. 执行推理: results = model.predict('/tmp/image.jpg')
        4. 解析结果并返回（必须包含 total_count）
        """
        
        # 模拟不同任务类型的推理结果
        if task_type == '人数统计':
            detections = [
                {"class": "person", "confidence": 0.95, "bbox": [100, 200, 150, 300]},
                {"class": "person", "confidence": 0.92, "bbox": [200, 220, 250, 320]},
                {"class": "person", "confidence": 0.89, "bbox": [300, 240, 350, 340]}
            ]
            return {
                "total_count": len(detections),  # ✅ 检测到3个对象
                "detections": detections,
                "message": f"检测到{len(detections)}人"
            }
        elif task_type == '人员跌倒':
            # 示例：未检测到跌倒 -> total_count = 0 -> 图片会被删除
            return {
                "total_count": 0,  # ❌ 未检测到跌倒（图片将被删除）
                "fall_detected": False,
                "persons": 3,  # 虽然有3个人，但没有跌倒
                "message": "未检测到跌倒"
            }
        elif task_type == '吸烟检测':
            # 示例：检测到1个吸烟行为 -> 保存告警
            return {
                "total_count": 1,  # ✅ 检测到1个吸烟行为
                "smoking_detected": True,
                "detections": [
                    {"location": {"x": 320, "y": 240}, "confidence": 0.87}
                ],
                "message": "检测到吸烟行为"
            }
        elif task_type == '车辆检测':
            # 示例：检测到多辆车
            vehicles = [
                {"class": "car", "confidence": 0.96, "bbox": [50, 100, 200, 300]},
                {"class": "truck", "confidence": 0.88, "bbox": [250, 120, 400, 320]},
            ]
            return {
                "total_count": len(vehicles),  # ✅ 检测到2辆车
                "detections": vehicles,
                "message": f"检测到{len(vehicles)}辆车"
            }
        elif task_type == '安全帽检测':
            # 示例：有人未戴安全帽
            violations = [
                {"person_id": 1, "has_helmet": False, "confidence": 0.93, "bbox": [100, 150, 180, 280]}
            ]
            return {
                "total_count": len(violations),  # ✅ 检测到1个违规
                "violations": violations,
                "message": f"检测到{len(violations)}人未戴安全帽"
            }
        else:
            # 未知任务类型 -> total_count = 0 -> 图片会被删除
            return {
                "total_count": 0,  # ❌ 未知任务类型（图片将被删除）
                "message": f"未支持的任务类型: {task_type}"
            }
    
    def log_message(self, format, *args):
        """禁用默认日志"""
        pass


def main():
    parser = argparse.ArgumentParser(description='算法服务示例')
    parser.add_argument('--service-id', default='demo_algo_v1', help='服务ID')
    parser.add_argument('--name', default='演示算法服务', help='服务名称')
    parser.add_argument('--task-types', nargs='+', default=['人数统计', '人员跌倒', '吸烟检测'], help='支持的任务类型')
    parser.add_argument('--port', type=int, default=8000, help='HTTP服务端口')
    parser.add_argument('--easydarwin', default='http://localhost:5066', help='EasyDarwin地址')
    
    args = parser.parse_args()
    
    # 创建算法服务
    service = AlgorithmService(
        service_id=args.service_id,
        name=args.name,
        task_types=args.task_types,
        port=args.port,
        easydarwin_url=args.easydarwin
    )
    
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
    
    logger.info(f"算法服务已启动")
    logger.info(f"  服务ID: {service.service_id}")
    logger.info(f"  服务名称: {service.name}")
    logger.info(f"  支持类型: {service.task_types}")
    logger.info(f"  监听端口: {args.port}")
    logger.info(f"  推理端点: {service.endpoint}")
    logger.info("等待推理请求...")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        logger.info("收到停止信号")
        httpd.shutdown()


if __name__ == '__main__':
    main()

