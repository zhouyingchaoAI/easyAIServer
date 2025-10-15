# AI分析插件快速入门

本文档提供智能分析插件的快速入门指南，从零开始配置并运行完整的AI分析流程。

---

## 前置条件

### 必需组件

1. **EasyDarwin** - 已安装并运行
2. **MinIO** - 对象存储服务
3. **Kafka**（可选）- 消息队列
4. **Python 3.8+**（可选）- 运行算法服务示例

### 安装MinIO

```bash
# Docker方式
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  --name minio \
  -e "MINIO_ROOT_USER=admin" \
  -e "MINIO_ROOT_PASSWORD=admin123" \
  minio/minio server /data --console-address ":9001"

# 访问控制台
# http://localhost:9001
# 用户名: admin
# 密码: admin123
```

### 安装Kafka（可选）

```bash
# Docker方式
docker run -d \
  -p 9092:9092 \
  --name kafka \
  -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  wurstmeister/kafka
```

---

## 5分钟快速开始

### 步骤1：配置Frame Extractor使用MinIO

编辑 `configs/config.toml`：

```toml
[frame_extractor]
enable = false
interval_ms = 1000
output_dir = './snapshots'
store = 'minio'  # ← 改为minio

[frame_extractor.minio]
endpoint = 'localhost:9000'  # ← MinIO地址
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
base_path = ''
```

### 步骤2：启用AI分析插件

继续编辑 `configs/config.toml`：

```toml
[ai_analysis]
enable = true  # ← 启用
scan_interval_sec = 10
mq_type = 'kafka'
mq_address = 'localhost:9092'  # ← Kafka地址（如果没有Kafka可留空）
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 5
```

### 步骤3：创建抽帧任务

访问：`http://localhost:5066/#/frame-extractor`

创建任务：
- 任务ID: `测试任务1`
- 任务类型: `人数统计`
- RTSP地址: 从直播列表选择
- 间隔: `5000`

### 步骤4：启动算法服务

```bash
cd /code/EasyDarwin/examples

# 直接运行（使用模拟推理）
python3 algorithm_service.py \
  --service-id test_algo \
  --name "测试算法" \
  --task-types 人数统计 \
  --port 8000

# 输出
# 正在注册到 http://localhost:5066...
# ✓ 注册成功: test_algo
# 心跳线程已启动（每30秒）
# 算法服务已启动
# 等待推理请求...
```

### 步骤5：查看告警

访问：`http://localhost:5066/#/alerts`

你会看到：
- 告警列表（每张抽帧图片的推理结果）
- 任务ID、任务类型、算法名称
- 置信度、推理时间
- 点击"查看"可预览图片和完整推理结果

访问：`http://localhost:5066/#/ai-services`

你会看到：
- 注册的算法服务列表
- 服务状态（正常/心跳超时）
- 支持的任务类型

---

## 完整流程演示

### 1. 启动所有服务

```bash
# 终端1：MinIO（如果用Docker）
docker start minio

# 终端2：Kafka（可选）
docker start kafka

# 终端3：EasyDarwin
cd /code/EasyDarwin
./server -conf ./configs

# 终端4：算法服务
cd /code/EasyDarwin/examples
python3 algorithm_service.py --task-types 人数统计
```

### 2. 配置抽帧任务

```bash
# 通过API创建任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "商场入口",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 5000
  }'
```

### 3. 等待图片生成

```bash
# 查看EasyDarwin日志
tail -f logs/sugar.log | grep -E "frame extractor|AI analysis"

# 应该看到：
# frame extractor: snapshot saved ...
# found new images count=1
# scheduling inference image=人数统计/商场入口/20250115-150001.jpg
# inference completed and saved task_id=商场入口 alert_id=1
```

### 4. 查看告警

```bash
# API查询
curl http://localhost:5066/api/v1/alerts | jq

# 或访问UI
# http://localhost:5066/#/alerts
```

### 5. 消费Kafka消息（可选）

```bash
# Kafka命令行消费
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic easydarwin.alerts \
  --from-beginning

# 或Python消费
python3 - <<EOF
from kafka import KafkaConsumer
import json

consumer = KafkaConsumer(
    'easydarwin.alerts',
    bootstrap_servers=['localhost:9092'],
    value_deserializer=lambda m: json.loads(m.decode('utf-8'))
)

for msg in consumer:
    alert = msg.value
    print(f"告警: {alert['task_id']} - {alert['task_type']}")
    print(f"  结果: {alert['result']}")
    print(f"  置信度: {alert['confidence']}")
EOF
```

---

## 实际AI模型集成

### 使用YOLOv8

```python
#!/usr/bin/env python3
from ultralytics import YOLO
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
import json

class YOLOInferenceHandler(BaseHTTPRequestHandler):
    MODEL = YOLO('yolov8n.pt')  # 加载模型
    
    def do_POST(self):
        if self.path != '/infer':
            self.send_error(404)
            return
        
        # 读取请求
        content_length = int(self.headers['Content-Length'])
        body = self.rfile.read(content_length)
        req_data = json.loads(body.decode('utf-8'))
        
        # 下载图片
        image_url = req_data['image_url']
        image_path = '/tmp/infer_image.jpg'
        urllib.request.urlretrieve(image_url, image_path)
        
        # 执行推理
        start_time = time.time()
        results = self.MODEL.predict(image_path, conf=0.5)
        inference_time = int((time.time() - start_time) * 1000)
        
        # 解析结果
        objects = []
        for r in results[0].boxes:
            objects.append({
                'class': self.MODEL.names[int(r.cls[0])],
                'confidence': float(r.conf[0]),
                'bbox': r.xyxy[0].tolist()
            })
        
        # 统计人数
        person_count = sum(1 for obj in objects if obj['class'] == 'person')
        
        # 返回结果
        response = {
            'success': True,
            'result': {
                'person_count': person_count,
                'objects': objects
            },
            'confidence': max([obj['confidence'] for obj in objects], default=0.0),
            'inference_time_ms': inference_time
        }
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(response).encode('utf-8'))
    
    def log_message(self, format, *args):
        pass

if __name__ == '__main__':
    print("Loading YOLOv8 model...")
    # 预热模型
    YOLOInferenceHandler.MODEL.predict('https://ultralytics.com/images/bus.jpg')
    print("Model loaded!")
    
    # 启动HTTP服务
    server = HTTPServer(('0.0.0.0', 8000), YOLOInferenceHandler)
    print("YOLOv8 inference service running on port 8000")
    server.serve_forever()
```

运行：
```bash
pip install ultralytics
python3 yolo_service.py
```

然后注册到EasyDarwin：
```bash
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "yolov8_detector",
    "name": "YOLOv8目标检测",
    "task_types": ["人数统计", "区域入侵"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
  }'
```

---

## 高级配置

### 多算法并行

同一个任务类型可以注册多个算法：

```bash
# 算法1：基础检测
python3 basic_detector.py --port 8001

# 算法2：高级分析
python3 advanced_analyzer.py --port 8002

# 两个都注册为"人数统计"类型
# 图片会同时被两个算法处理
# 生成两条告警记录
```

### 扫描间隔优化

根据抽帧频率调整：

```toml
[frame_extractor]
interval_ms = 5000  # 5秒抽一帧

[ai_analysis]
scan_interval_sec = 10  # 10秒扫描一次（足够覆盖）
```

推荐配置：
- 抽帧间隔1-5秒 → 扫描间隔5-10秒
- 抽帧间隔5-10秒 → 扫描间隔10-15秒
- 抽帧间隔>10秒 → 扫描间隔15-30秒

### 并发控制

```toml
[ai_analysis]
max_concurrent_infer = 10  # 最多10个并发推理
```

如果有多个快速生成图片的任务：
- 增加并发数（需要更多服务器资源）
- 或增加扫描间隔（延迟处理）

---

## 监控和调试

### 查看注册服务

```bash
curl http://localhost:5066/api/v1/ai_analysis/services | jq
```

### 查看最新告警

```bash
curl 'http://localhost:5066/api/v1/alerts?page=1&page_size=5' | jq '.items[] | {id, task_id, confidence}'
```

### 按任务类型筛选

```bash
curl 'http://localhost:5066/api/v1/alerts?task_type=人数统计' | jq
```

### 实时日志

```bash
# 扫描日志
tail -f logs/sugar.log | grep "found new images"

# 推理日志
tail -f logs/sugar.log | grep "inference completed"

# 注册日志
tail -f logs/sugar.log | grep "algorithm service"
```

### Kafka消息监控

```bash
# 查看告警消息
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic easydarwin.alerts \
  --from-beginning \
  | jq
```

---

## 常见问题

### Q1: AI分析插件未启动？

**A**: 检查：
1. `config.toml`中`[ai_analysis] enable = true`
2. `[frame_extractor] store = 'minio'`（必须）
3. MinIO配置正确且可连接
4. 查看日志：`tail -f logs/sugar.log | grep "AI analysis"`

### Q2: 算法服务注册失败？

**A**: 检查：
1. EasyDarwin运行正常
2. `service_id`唯一
3. `endpoint`可访问
4. `task_types`不为空

### Q3: 没有生成告警？

**A**: 检查流程：
1. 抽帧任务是否运行？MinIO中是否有图片？
2. 算法服务是否注册成功？
3. 任务类型是否匹配？（task_type要完全一致）
4. 查看日志是否有错误

### Q4: Kafka推送失败？

**A**: 
1. Kafka可以不配置，告警仍会保存到数据库
2. 如果配置了Kafka但连接失败，检查：
   - Kafka服务是否运行
   - `mq_address`是否正确
   - 防火墙/网络是否通畅

---

## 测试验证

### 完整测试脚本

```bash
#!/bin/bash

echo "=== AI分析插件测试 ==="

# 1. 启动算法服务
echo "1. 启动算法服务..."
cd /code/EasyDarwin/examples
python3 algorithm_service.py \
  --service-id test_algo \
  --task-types 人数统计 \
  --port 8000 &
ALGO_PID=$!
sleep 2

# 2. 验证注册
echo "2. 验证算法服务注册..."
curl -s http://localhost:5066/api/v1/ai_analysis/services | jq '.total'

# 3. 创建抽帧任务
echo "3. 创建抽帧任务..."
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test1",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 3000
  }'

# 4. 等待抽帧和推理
echo "4. 等待30秒（抽帧+扫描+推理）..."
sleep 30

# 5. 查询告警
echo "5. 查询告警..."
ALERT_COUNT=$(curl -s http://localhost:5066/api/v1/alerts | jq '.total')
echo "  告警数量: $ALERT_COUNT"

if [ "$ALERT_COUNT" -gt "0" ]; then
  echo "✓ 测试成功！AI分析正常工作"
  curl -s http://localhost:5066/api/v1/alerts | jq '.items[0] | {task_id, algorithm_name, confidence}'
else
  echo "✗ 测试失败：未生成告警"
fi

# 清理
kill $ALGO_PID
echo "测试完成"
```

---

## 生产环境部署

### 1. 使用真实AI模型

```python
# production_algorithm_service.py
from ultralytics import YOLO

class ProductionService:
    def __init__(self):
        # 根据任务类型加载不同模型
        self.models = {
            '人数统计': YOLO('yolov8n.pt'),
            '人员跌倒': YOLO('fall_detection.pt'),
            '吸烟检测': YOLO('smoking_detection.pt'),
        }
    
    def infer(self, image_url, task_type):
        model = self.models.get(task_type)
        if not model:
            return {'error': f'No model for {task_type}'}
        
        # 下载并推理
        # ...
```

### 2. 配置监控告警

```python
# 监控Kafka消息并发送钉钉/邮件告警
from kafka import KafkaConsumer
import requests

consumer = KafkaConsumer('easydarwin.alerts', ...)

for message in consumer:
    alert = message.value
    result = json.loads(alert['result'])
    
    # 异常检测
    if alert['task_type'] == '人员跌倒' and result.get('fall_detected'):
        # 发送钉钉告警
        send_dingtalk_alert(f"检测到跌倒: {alert['task_id']}")
    
    elif alert['task_type'] == '人数统计' and result.get('person_count', 0) > 100:
        # 人数过多告警
        send_email_alert(f"人流告警: {result['person_count']}人")
```

### 3. 高可用部署

```bash
# 算法服务集群（负载均衡）
# 服务器1
python3 algorithm_service.py --service-id algo_1 --port 8001

# 服务器2
python3 algorithm_service.py --service-id algo_2 --port 8002

# 服务器3
python3 algorithm_service.py --service-id algo_3 --port 8003

# 所有服务注册到EasyDarwin
# EasyDarwin会并发调用所有注册的服务
```

---

## 性能指标

### 处理能力

配置：
- 扫描间隔: 10秒
- 并发推理: 5
- 推理耗时: 50ms/张

理论处理能力：
- 每次扫描最多处理5张图片
- 每秒处理：5张 × (1000ms / 50ms) = 100张/秒
- 每10秒批次：50-100张

### 资源消耗

- EasyDarwin AI插件：约50MB内存（含10000条已处理缓存）
- 算法服务（YOLOv8）：约2GB内存（模型加载）
- Kafka客户端：约10MB内存

---

## 下一步

AI分析插件已完成基础功能。可以扩展：

1. **RabbitMQ支持** - 实现RabbitMQQueue
2. **结果可视化** - 在图片上画bbox检测框
3. **WebSocket推送** - 实时告警到前端
4. **告警规则引擎** - 配置化的告警条件
5. **统计Dashboard** - 告警趋势图表
6. **批量重新推理** - 对历史图片重新分析

参考文档：
- [doc/AI_ANALYSIS.md](AI_ANALYSIS.md) - 完整API文档
- [doc/TASK_TYPES.md](TASK_TYPES.md) - 任务类型说明
- [examples/algorithm_service.py](../examples/algorithm_service.py) - 示例代码

