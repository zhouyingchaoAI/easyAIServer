# 智能分析插件文档

## 概述

智能分析插件是EasyDarwin的AI能力扩展，支持外部算法服务注册、自动扫描抽帧图片、调度推理任务、存储告警结果并推送到消息队列。

---

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                      EasyDarwin                          │
├─────────────────────────────────────────────────────────┤
│  Frame Extractor Plugin                                  │
│   └→ 保存图片到MinIO: 任务类型/任务ID/时间戳.jpg         │
├─────────────────────────────────────────────────────────┤
│  AI Analysis Plugin                                      │
│   ├─ 算法服务注册中心                                     │
│   │   ├─ 服务注册/注销                                    │
│   │   ├─ 心跳检测                                         │
│   │   └─ 任务类型→算法映射                                 │
│   ├─ MinIO扫描器                                         │
│   │   ├─ 定时扫描新图片（默认10秒）                        │
│   │   └─ 去重（跟踪已处理图片）                            │
│   ├─ 推理调度器                                           │
│   │   ├─ 根据任务类型匹配算法                              │
│   │   ├─ 并发HTTP调用算法服务                             │
│   │   └─ 汇总推理结果                                      │
│   └─ 结果处理                                             │
│       ├─ 存储到SQLite数据库                               │
│       └─ 推送到Kafka消息队列                              │
└─────────────────────────────────────────────────────────┘
              ↓ HTTP API                    ↓ Kafka
┌──────────────────────┐        ┌─────────────────────────┐
│  外部算法服务         │        │  外部系统/前端           │
│  (Python/任何语言)    │        │  (消费告警消息)          │
│                      │        │                          │
│  - 注册到EasyDarwin   │        │  - 实时告警通知          │
│  - 接收推理请求       │        │  - 数据分析              │
│  - 返回推理结果       │        │  - 第三方集成            │
│  - 定时发送心跳       │        │                          │
└──────────────────────┘        └─────────────────────────┘
```

---

## 配置

### config.toml

```toml
[ai_analysis]
enable = false  # 启用智能分析插件
scan_interval_sec = 10  # MinIO扫描间隔（秒）
mq_type = 'kafka'  # 消息队列类型：kafka|rabbitmq
mq_address = 'localhost:9092'  # 消息队列地址
mq_topic = 'easydarwin.alerts'  # 告警推送topic
heartbeat_timeout_sec = 90  # 算法服务心跳超时（秒）
max_concurrent_infer = 5  # 最大并发推理任务数
```

### 配置说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enable` | bool | false | 是否启用AI分析插件 |
| `scan_interval_sec` | int | 10 | MinIO扫描间隔，单位秒 |
| `mq_type` | string | kafka | 消息队列类型 |
| `mq_address` | string | localhost:9092 | 消息队列地址 |
| `mq_topic` | string | easydarwin.alerts | 告警推送的topic |
| `heartbeat_timeout_sec` | int | 90 | 算法服务心跳超时，超时自动注销 |
| `max_concurrent_infer` | int | 5 | 最大并发推理数，防止资源耗尽 |

### 依赖检查

AI分析插件需要：
1. **Frame Extractor插件**启用且配置为MinIO存储
2. **Kafka**（可选）：如果需要推送告警消息

---

## API接口

### 算法服务注册

**Endpoint**: `POST /api/v1/ai_analysis/register`

**请求体**:
```json
{
  "service_id": "people_counter_v1",
  "name": "人数统计算法v1",
  "task_types": ["人数统计", "客流分析"],
  "endpoint": "http://10.1.6.230:8000/infer",
  "version": "1.0.0"
}
```

**响应**:
```json
{
  "ok": true,
  "service_id": "people_counter_v1"
}
```

### 算法服务注销

**Endpoint**: `DELETE /api/v1/ai_analysis/unregister/:service_id`

**响应**:
```json
{
  "ok": true
}
```

### 算法服务心跳

**Endpoint**: `POST /api/v1/ai_analysis/heartbeat/:service_id`

**说明**: 算法服务需要每30秒发送一次心跳，超过`heartbeat_timeout_sec`未收到心跳将自动注销。

**响应**:
```json
{
  "ok": true
}
```

### 查询注册的服务

**Endpoint**: `GET /api/v1/ai_analysis/services`

**响应**:
```json
{
  "services": [
    {
      "service_id": "people_counter_v1",
      "name": "人数统计算法v1",
      "task_types": ["人数统计"],
      "endpoint": "http://10.1.6.230:8000/infer",
      "version": "1.0.0",
      "register_at": 1705305600,
      "last_heartbeat": 1705305630
    }
  ],
  "total": 1
}
```

### 查询告警列表

**Endpoint**: `GET /api/v1/alerts`

**参数**:
- `task_id`: 任务ID（可选）
- `task_type`: 任务类型（可选）
- `page`: 页码（默认1）
- `page_size`: 每页数量（默认20，最大100）

**响应**:
```json
{
  "items": [
    {
      "id": 1,
      "task_id": "客流分析1",
      "task_type": "人数统计",
      "image_path": "人数统计/客流分析1/20250115-150001.123.jpg",
      "image_url": "http://minio.../presigned_url",
      "algorithm_id": "people_counter_v1",
      "algorithm_name": "人数统计算法v1",
      "result": "{\"person_count\":23,\"objects\":[...]}",
      "confidence": 0.95,
      "inference_time_ms": 45,
      "created_at": "2025-01-15T15:00:01Z"
    }
  ],
  "total": 100
}
```

### 删除告警

**Endpoint**: `DELETE /api/v1/alerts/:id`

---

## 算法服务开发指南

### 基本要求

算法服务需要实现以下功能：
1. **HTTP推理接口**：接收推理请求，返回结果
2. **主动注册**：启动时向EasyDarwin注册
3. **定时心跳**：每30秒发送心跳
4. **优雅退出**：停止时注销服务

### 推理接口规范

**Endpoint**: 自定义（注册时提供）

**请求方法**: POST

**请求体**:
```json
{
  "image_url": "http://minio-server/bucket/presigned_url?token=...",
  "task_id": "客流分析1",
  "task_type": "人数统计",
  "image_path": "人数统计/客流分析1/20250115-150001.123.jpg"
}
```

**响应**:
```json
{
  "success": true,
  "result": {
    "person_count": 23,
    "objects": [
      {
        "class": "person",
        "confidence": 0.95,
        "bbox": [100, 200, 150, 300]
      }
    ]
  },
  "confidence": 0.95,
  "inference_time_ms": 45
}
```

**错误响应**:
```json
{
  "success": false,
  "error": "推理失败原因",
  "confidence": 0.0,
  "inference_time_ms": 0
}
```

### Python示例

参考 `examples/algorithm_service.py`：

```python
# 启动算法服务
python3 examples/algorithm_service.py \
  --service-id people_counter_v1 \
  --name "人数统计算法v1" \
  --task-types 人数统计 客流分析 \
  --port 8000 \
  --easydarwin http://localhost:5066

# 输出
# 正在注册到 http://localhost:5066...
# ✓ 注册成功: people_counter_v1
# 心跳线程已启动（每30秒）
# 算法服务已启动
#   服务ID: people_counter_v1
#   服务名称: 人数统计算法v1
#   支持类型: ['人数统计', '客流分析']
#   监听端口: 8000
#   推理端点: http://localhost:8000/infer
# 等待推理请求...
```

### 实际AI模型集成

使用YOLOv8示例：

```python
from ultralytics import YOLO
import urllib.request

class RealInferenceHandler(BaseHTTPRequestHandler):
    # 加载模型（全局，启动时加载一次）
    MODEL = YOLO('yolov8n.pt')
    
    def infer(self, image_url, task_type):
        # 下载图片
        image_path = '/tmp/image.jpg'
        urllib.request.urlretrieve(image_url, image_path)
        
        # 执行推理
        results = self.MODEL.predict(image_path, conf=0.5)
        
        # 解析结果
        objects = []
        for r in results[0].boxes:
            objects.append({
                'class': self.MODEL.names[int(r.cls[0])],
                'confidence': float(r.conf[0]),
                'bbox': r.xyxy[0].tolist()
            })
        
        # 根据任务类型定制返回
        if task_type == '人数统计':
            person_count = sum(1 for obj in objects if obj['class'] == 'person')
            return {
                'person_count': person_count,
                'objects': objects
            }
        else:
            return {'objects': objects}
```

---

## 工作流程

### 1. 算法服务注册

```bash
# 算法服务启动
python3 algorithm_service.py

# 自动注册到EasyDarwin
POST http://localhost:5066/api/v1/ai_analysis/register
{
  "service_id": "people_counter_v1",
  "task_types": ["人数统计"]
}
```

### 2. MinIO扫描

```
每10秒（scan_interval_sec）：
1. 列举MinIO中的所有图片
2. 过滤已处理的图片
3. 返回新图片列表
```

### 3. 推理调度

```
对于每张新图片：
1. 从路径提取任务类型：人数统计/客流分析1/20250115-150001.jpg
2. 查询注册中心：获取"人数统计"类型的所有算法
3. 并发调用所有匹配算法的HTTP接口
4. 收集推理结果
```

### 4. 结果处理

```
推理成功后：
1. 保存到数据库（alerts表）
2. 推送到Kafka（topic: easydarwin.alerts）
```

### 5. 前端查看

```
访问：http://localhost:5066/#/alerts
- 查看告警列表
- 筛选（任务类型、任务ID）
- 查看详情（图片+推理结果）
```

---

## 使用示例

### 场景1：人数统计

**1. 配置抽帧任务**

```bash
# 通过UI或API创建
POST /api/v1/frame_extractor/tasks
{
  "id": "商场入口",
  "task_type": "人数统计",
  "rtsp_url": "rtsp://localhost:15544/live/stream_1",
  "interval_ms": 5000
}
```

**2. 启动算法服务**

```bash
python3 examples/algorithm_service.py \
  --service-id people_counter \
  --task-types 人数统计 \
  --port 8000
```

**3. 启用AI分析**

编辑`config.toml`：
```toml
[ai_analysis]
enable = true
```

重启EasyDarwin。

**4. 查看结果**

- 访问告警页面：`http://localhost:5066/#/alerts`
- 告警会显示每张图片的人数统计结果
- Kafka消费者可实时接收告警消息

### 场景2：多算法协同

一个任务类型可以注册多个算法：

```bash
# 算法1：人数统计
python3 algorithm_service.py \
  --service-id people_counter \
  --task-types 人数统计 \
  --port 8001

# 算法2：热力图分析（同样支持人数统计）
python3 heatmap_service.py \
  --service-id heatmap_analyzer \
  --task-types 人数统计 \
  --port 8002
```

当"人数统计"任务生成图片时：
- 同时调用 `people_counter` 和 `heatmap_analyzer`
- 两个算法并发执行
- 生成两条告警记录

---

## 数据库表结构

### alerts表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| task_id | VARCHAR(100) | 任务ID |
| task_type | VARCHAR(50) | 任务类型 |
| image_path | VARCHAR(500) | MinIO对象路径 |
| image_url | VARCHAR(1000) | 预签名URL |
| algorithm_id | VARCHAR(100) | 算法服务ID |
| algorithm_name | VARCHAR(100) | 算法服务名称 |
| result | TEXT | 推理结果JSON |
| confidence | REAL | 置信度 |
| inference_time_ms | INTEGER | 推理耗时（毫秒） |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

---

## Kafka消息格式

推送到Kafka的告警消息格式：

```json
{
  "id": 1,
  "task_id": "客流分析1",
  "task_type": "人数统计",
  "image_path": "人数统计/客流分析1/20250115-150001.123.jpg",
  "image_url": "http://minio.../presigned_url",
  "algorithm_id": "people_counter_v1",
  "algorithm_name": "人数统计算法v1",
  "result": "{\"person_count\":23,\"objects\":[...]}",
  "confidence": 0.95,
  "inference_time_ms": 45,
  "created_at": "2025-01-15T15:00:01Z"
}
```

### Kafka消费示例

```python
from kafka import KafkaConsumer
import json

consumer = KafkaConsumer(
    'easydarwin.alerts',
    bootstrap_servers=['localhost:9092'],
    value_deserializer=lambda m: json.loads(m.decode('utf-8'))
)

for message in consumer:
    alert = message.value
    print(f"收到告警: {alert['task_id']} - {alert['task_type']}")
    
    result = json.loads(alert['result'])
    if alert['task_type'] == '人数统计':
        print(f"  人数: {result.get('person_count')}")
    elif alert['task_type'] == '人员跌倒':
        print(f"  跌倒检测: {result.get('fall_detected')}")
```

---

## 前端界面

### 智能告警页面

访问：`http://localhost:5066/#/alerts`

功能：
- 告警列表（表格展示）
- 筛选器（任务类型、任务ID）
- 查看详情（图片预览+推理结果）
- 删除告警
- 分页

### 算法服务页面

访问：`http://localhost:5066/#/ai-services`

功能：
- 查看所有注册的算法服务
- 服务状态（正常/心跳超时/已失联）
- 支持的任务类型
- 注册时间和最后心跳时间
- 自动刷新（每30秒）

---

## 故障排查

### 问题1：AI分析插件未启动

**检查**:
```bash
# 1. 确认配置启用
cat configs/config.toml | grep -A 5 "\[ai_analysis\]"
# enable应该为true

# 2. 确认Frame Extractor使用MinIO
cat configs/config.toml | grep -A 2 "\[frame_extractor\]"
# store应该为'minio'

# 3. 查看日志
tail -f logs/sugar.log | grep "AI analysis"
# 应该看到：AI analysis plugin started successfully
```

### 问题2：算法服务注册失败

**检查**:
```bash
# 1. 确认EasyDarwin运行中
curl http://localhost:5066/api/v1/version

# 2. 确认AI分析插件启用
curl http://localhost:5066/api/v1/ai_analysis/services

# 3. 检查算法服务日志
# 应该看到"✓ 注册成功"
```

### 问题3：没有告警生成

**检查流程**:
```bash
# 1. 确认有抽帧图片
# MinIO: images/人数统计/客流分析1/*.jpg

# 2. 确认有算法服务注册
curl http://localhost:5066/api/v1/ai_analysis/services | jq

# 3. 查看扫描日志
tail -f logs/sugar.log | grep "found new images"
# 应该看到扫描到的图片数量

# 4. 查看推理日志
tail -f logs/sugar.log | grep "inference completed"

# 5. 查询告警
curl http://localhost:5066/api/v1/alerts | jq
```

### 问题4：Kafka推送失败

**检查**:
```bash
# 1. 确认Kafka运行
nc -zv localhost 9092

# 2. 查看连接日志
tail -f logs/sugar.log | grep "kafka"

# 3. 测试topic
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic easydarwin.alerts \
  --from-beginning
```

---

## 性能优化

### 扫描间隔调整

根据图片生成频率调整：

| 抽帧间隔 | 推荐扫描间隔 |
|---------|-------------|
| 1-5秒   | 5-10秒      |
| 5-10秒  | 10-15秒     |
| >10秒   | 15-30秒     |

### 并发控制

```toml
[ai_analysis]
max_concurrent_infer = 10  # 根据服务器性能调整
```

建议值：
- 低配服务器（2核4G）：5
- 中配服务器（4核8G）：10
- 高配服务器（8核16G+）：20

### 已处理图片缓存

- 内存缓存最多10000条
- 超过24小时自动清理
- 重启服务会重新处理所有图片

---

## 开发清单

- [x] 算法服务注册中心
- [x] MinIO图片扫描器
- [x] 推理调度器
- [x] Kafka消息队列集成
- [x] 告警数据存储
- [x] 告警查询API
- [x] 前端告警界面
- [x] 算法服务示例（Python）
- [ ] RabbitMQ消息队列（未实现）
- [ ] 推理结果可视化（画框）
- [ ] 告警统计Dashboard
- [ ] WebSocket实时推送

---

## 下一步

可以基于此插件扩展：

1. **实时告警**
   - WebSocket推送到前端
   - 异常情况弹窗提醒

2. **结果可视化**
   - 在图片上叠加bbox检测框
   - 热力图展示
   - 轨迹追踪

3. **数据分析**
   - 按任务类型统计告警数量
   - 按时间段统计趋势
   - 异常行为分析报表

4. **告警规则**
   - 配置阈值（如人数>50触发告警）
   - 告警去重（短时间内相同告警合并）
   - 告警升级（连续N次触发升级）

参考文档：
- [doc/FRAME_EXTRACTOR.md](FRAME_EXTRACTOR.md) - 抽帧插件文档
- [doc/TASK_TYPES.md](TASK_TYPES.md) - 任务类型分类
- [examples/algorithm_service.py](../examples/algorithm_service.py) - 算法服务示例

