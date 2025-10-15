# 任务类型分类使用示例

本文档提供抽帧任务类型分类功能的实际使用示例。

---

## 快速开始

### 1. 查看可用任务类型

```bash
curl http://localhost:5066/api/v1/frame_extractor/task_types | jq

# 返回
{
  "task_types": [
    "人数统计",
    "人员跌倒",
    "人员离岗",
    "吸烟检测",
    "区域入侵",
    "徘徊检测",
    "物品遗留",
    "安全帽检测"
  ]
}
```

### 2. 通过UI创建任务

访问：`http://localhost:5066/#/frame-extractor`

**填写表单**：
- 任务ID: `商场入口`
- 任务类型: 选择 `人数统计`（下拉框）
- RTSP地址: 选择直播流或输入 `rtsp://localhost:15544/live/stream_1`
- 间隔: `5000`
- 输出路径: `商场入口`（可选，默认使用任务ID）

点击"添加任务"。

### 3. 验证目录结构

```bash
# 本地存储
ls -la ./snapshots/人数统计/商场入口/
# 输出：
# 20250115-150001.123.jpg
# 20250115-150006.456.jpg
# ...

# MinIO存储（如果配置为minio）
# 路径：images/人数统计/商场入口/20250115-150001.123.jpg
```

---

## 场景示例

### 场景1：商场多场景监控

**需求**：
- 3个入口做客流统计
- 2个电梯做跌倒检测
- 1个吸烟区做吸烟检测

**配置**：

```toml
[[frame_extractor.tasks]]
id = '东门入口'
task_type = '人数统计'
rtsp_url = 'rtsp://localhost:15544/live/stream_1'
interval_ms = 5000
enabled = true

[[frame_extractor.tasks]]
id = '西门入口'
task_type = '人数统计'
rtsp_url = 'rtsp://localhost:15544/live/stream_2'
interval_ms = 5000
enabled = true

[[frame_extractor.tasks]]
id = '南门入口'
task_type = '人数统计'
rtsp_url = 'rtsp://localhost:15544/live/stream_3'
interval_ms = 5000
enabled = true

[[frame_extractor.tasks]]
id = '1号电梯'
task_type = '人员跌倒'
rtsp_url = 'rtsp://localhost:15544/live/stream_4'
interval_ms = 3000
enabled = true

[[frame_extractor.tasks]]
id = '2号电梯'
task_type = '人员跌倒'
rtsp_url = 'rtsp://localhost:15544/live/stream_5'
interval_ms = 3000
enabled = true

[[frame_extractor.tasks]]
id = '吸烟区监控'
task_type = '吸烟检测'
rtsp_url = 'rtsp://localhost:15544/live/stream_6'
interval_ms = 10000
enabled = true
```

**目录结构**：
```
snapshots/
├── 人数统计/
│   ├── 东门入口/
│   ├── 西门入口/
│   └── 南门入口/
├── 人员跌倒/
│   ├── 1号电梯/
│   └── 2号电梯/
└── 吸烟检测/
    └── 吸烟区监控/
```

### 场景2：工地安全监控

**需求**：
- 入口做安全帽检测
- 高空区域做人员离岗检测
- 禁入区域做区域入侵检测

**通过UI创建**：

1. 创建任务1：
   - 任务ID: `工地入口`
   - 任务类型: `安全帽检测`
   - RTSP地址: 从直播列表选择
   - 间隔: `3000`

2. 创建任务2：
   - 任务ID: `3号楼10层`
   - 任务类型: `人员离岗`
   - RTSP地址: 从直播列表选择
   - 间隔: `5000`

3. 创建任务3：
   - 任务ID: `材料堆放区`
   - 任务类型: `区域入侵`
   - RTSP地址: 从直播列表选择
   - 间隔: `2000`

**目录结构**：
```
snapshots/
├── 安全帽检测/
│   └── 工地入口/
├── 人员离岗/
│   └── 3号楼10层/
└── 区域入侵/
    └── 材料堆放区/
```

### 场景3：MinIO存储 + 智能分析

**配置MinIO**：

通过UI配置存储：
1. 访问抽帧管理页面
2. 存储配置卡片
3. 存储类型选择：`MinIO对象存储`
4. 填写MinIO信息：
   - Endpoint: `10.1.6.230:9000`
   - Bucket: `images`
   - Access Key: `admin`
   - Secret Key: `admin123`
5. 点击"保存配置"

**创建任务**：

```bash
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "客流分析1",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 5000
  }'
```

**MinIO中的存储**：
```
Bucket: images
路径：人数统计/客流分析1/20250115-150001.123.jpg
```

**智能分析服务监听**：

```python
# Python示例：监听MinIO事件
from minio import Minio

client = Minio('10.1.6.230:9000',
               access_key='admin',
               secret_key='admin123',
               secure=False)

# 监听新图片上传事件
events = client.listen_bucket_notification(
    'images',
    suffix='.jpg',
    events=['s3:ObjectCreated:*']
)

for event in events:
    for record in event['Records']:
        object_key = record['s3']['object']['key']
        # 解析：人数统计/客流分析1/20250115-150001.123.jpg
        
        parts = object_key.split('/')
        task_type = parts[0]  # '人数统计'
        task_id = parts[1]    # '客流分析1'
        
        # 根据任务类型选择AI模型
        if task_type == '人数统计':
            result = yolo_detect_persons(object_key)
            push_result(task_id, result)
        elif task_type == '人员跌倒':
            result = detect_fall(object_key)
            push_result(task_id, result)
```

---

## API使用示例

### 创建不同类型的任务

```bash
# 1. 人数统计任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "商场入口",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 5000
  }'

# 2. 人员跌倒检测任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "电梯监控",
    "task_type": "人员跌倒",
    "rtsp_url": "rtsp://localhost:15544/live/stream_2",
    "interval_ms": 3000
  }'

# 3. 吸烟检测任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "禁烟区",
    "task_type": "吸烟检测",
    "rtsp_url": "rtsp://localhost:15544/live/stream_3",
    "interval_ms": 10000
  }'
```

### 查询任务列表

```bash
curl http://localhost:5066/api/v1/frame_extractor/tasks | jq

# 返回
{
  "items": [
    {
      "id": "商场入口",
      "task_type": "人数统计",
      "rtsp_url": "rtsp://localhost:15544/live/stream_1",
      "interval_ms": 5000,
      "output_path": "商场入口",
      "enabled": true
    },
    {
      "id": "电梯监控",
      "task_type": "人员跌倒",
      "rtsp_url": "rtsp://localhost:15544/live/stream_2",
      "interval_ms": 3000,
      "output_path": "电梯监控",
      "enabled": true
    }
  ]
}
```

---

## 自定义任务类型

### 添加新类型

编辑 `configs/config.toml`：

```toml
[frame_extractor]
task_types = [
  '人数统计', 
  '人员跌倒', 
  '人员离岗', 
  '吸烟检测',
  '区域入侵',
  '徘徊检测',
  '物品遗留',
  '安全帽检测',
  '车辆违停',      # 新增
  '消防通道阻塞',  # 新增
  '口罩佩戴检测'   # 新增
]
```

重启服务：

```bash
pkill server
./server -conf ./configs
```

验证：

```bash
curl http://localhost:5066/api/v1/frame_extractor/task_types | jq

# 返回应包含新增的类型
{
  "task_types": [
    ...,
    "车辆违停",
    "消防通道阻塞",
    "口罩佩戴检测"
  ]
}
```

### 使用新类型创建任务

通过UI：
- 任务类型下拉框会自动显示新增的类型
- 选择并创建任务

通过API：
```bash
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "停车场监控",
    "task_type": "车辆违停",
    "rtsp_url": "rtsp://localhost:15544/live/stream_10",
    "interval_ms": 15000
  }'
```

---

## 批量操作示例

### 批量创建同类型任务

```bash
#!/bin/bash
# 批量创建人数统计任务

TASK_TYPE="人数统计"
BASE_URL="http://localhost:5066/api/v1/frame_extractor/tasks"

# 商场4个入口
for i in {1..4}; do
  curl -X POST $BASE_URL \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"入口${i}\",
      \"task_type\": \"${TASK_TYPE}\",
      \"rtsp_url\": \"rtsp://localhost:15544/live/stream_${i}\",
      \"interval_ms\": 5000
    }"
  echo ""
done
```

### 按类型查看任务

```bash
# 获取所有任务
curl http://localhost:5066/api/v1/frame_extractor/tasks | jq

# 过滤人数统计任务
curl http://localhost:5066/api/v1/frame_extractor/tasks | \
  jq '.items[] | select(.task_type == "人数统计")'
```

---

## 智能分析服务示例

### 简单的Python分析服务

```python
#!/usr/bin/env python3
"""
简单的智能分析服务示例
监听MinIO新图片，根据任务类型执行推理
"""

import os
from minio import Minio
from ultralytics import YOLO
import requests

# MinIO配置
MINIO_CLIENT = Minio(
    '10.1.6.230:9000',
    access_key='admin',
    secret_key='admin123',
    secure=False
)

# 模型加载（根据任务类型）
MODELS = {
    '人数统计': YOLO('yolov8n.pt'),
    '人员跌倒': YOLO('fall_detection.pt'),
    '安全帽检测': YOLO('helmet_detection.pt'),
}

def analyze_image(task_type, task_id, image_path):
    """根据任务类型执行推理"""
    model = MODELS.get(task_type)
    if not model:
        print(f"No model for task type: {task_type}")
        return None
    
    results = model.predict(image_path)
    
    # 解析结果
    if task_type == '人数统计':
        person_count = sum(1 for r in results[0].boxes if r.cls == 0)
        return {'person_count': person_count}
    
    elif task_type == '人员跌倒':
        fall_detected = any(r.cls == 1 for r in results[0].boxes)
        return {'fall_detected': fall_detected}
    
    elif task_type == '安全帽检测':
        no_helmet_count = sum(1 for r in results[0].boxes if r.cls == 2)
        return {'no_helmet_count': no_helmet_count}
    
    return None

def push_result(task_id, task_type, image_path, result):
    """推送结果到WebSocket/HTTP"""
    payload = {
        'task_id': task_id,
        'task_type': task_type,
        'image_path': image_path,
        'result': result,
        'timestamp': datetime.now().isoformat()
    }
    
    # 选项1：HTTP回调
    requests.post('http://your-callback-url/api/analysis/result', json=payload)
    
    # 选项2：WebSocket推送
    # websocket.send(json.dumps(payload))
    
    # 选项3：保存到数据库
    # db.insert('analysis_results', payload)

def main():
    """监听MinIO事件并分析"""
    print("Starting AI analysis service...")
    
    events = MINIO_CLIENT.listen_bucket_notification(
        'images',
        suffix='.jpg',
        events=['s3:ObjectCreated:*']
    )
    
    for event in events:
        for record in event['Records']:
            object_key = record['s3']['object']['key']
            
            # 解析路径：任务类型/任务ID/文件名
            parts = object_key.split('/')
            if len(parts) < 3:
                continue
            
            task_type = parts[0]
            task_id = parts[1]
            filename = parts[2]
            
            print(f"Analyzing: {task_type}/{task_id}/{filename}")
            
            # 下载图片
            local_path = f'/tmp/{filename}'
            MINIO_CLIENT.fget_object('images', object_key, local_path)
            
            # 执行推理
            result = analyze_image(task_type, task_id, local_path)
            
            # 推送结果
            if result:
                push_result(task_id, task_type, object_key, result)
                print(f"Result: {result}")
            
            # 清理临时文件
            os.remove(local_path)

if __name__ == '__main__':
    main()
```

### 运行分析服务

```bash
# 安装依赖
pip install minio ultralytics requests

# 运行
python3 ai_analysis_service.py

# 输出示例
Starting AI analysis service...
Analyzing: 人数统计/商场入口/20250115-150001.123.jpg
Result: {'person_count': 23}
Analyzing: 人员跌倒/电梯监控/20250115-150005.456.jpg
Result: {'fall_detected': False}
Analyzing: 安全帽检测/工地入口/20250115-150010.789.jpg
Result: {'no_helmet_count': 2}
```

---

## 配置建议

### 间隔设置

根据任务类型设置合适的抽帧间隔：

| 任务类型 | 推荐间隔 | 原因 |
|---------|---------|------|
| 人数统计 | 5-10秒 | 人流变化较慢 |
| 人员跌倒 | 1-3秒 | 需要快速检测 |
| 人员离岗 | 30-60秒 | 状态变化较慢 |
| 吸烟检测 | 10-15秒 | 行为持续时间较长 |
| 区域入侵 | 2-5秒 | 需要及时检测 |
| 徘徊检测 | 5-10秒 | 需要连续观察 |
| 物品遗留 | 30-60秒 | 状态变化慢 |
| 安全帽检测 | 5-10秒 | 进出场景检测 |

### 存储选择

| 存储类型 | 适用场景 | 优点 | 缺点 |
|---------|---------|------|------|
| **本地** | 单机部署、测试环境 | 简单、无依赖 | 不易扩展、容量受限 |
| **MinIO** | 生产环境、多服务器 | 可扩展、支持事件通知 | 需要额外部署MinIO |

---

## 验证功能

### 测试任务类型

```bash
# 1. 创建测试任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test1",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 2000
  }'

# 2. 等待10秒
sleep 10

# 3. 检查目录
ls -la ./snapshots/人数统计/test1/
# 应该有几张图片

# 4. 查看日志
tail -f logs/sugar.log | grep "test1"

# 5. 删除测试任务
curl -X DELETE http://localhost:5066/api/v1/frame_extractor/tasks/test1
```

---

## 下一步

任务类型分类功能已完成，为智能分析做好准备。后续可以：

1. **开发智能分析服务**
   - 监听MinIO事件或定时扫描
   - 根据任务类型加载对应模型
   - 执行推理并推送结果

2. **前端展示分析结果**
   - 添加分析结果查询API
   - 在快照gallery页面显示推理结果
   - 支持画框、标签、统计图表

3. **告警通知**
   - 异常情况WebSocket实时推送
   - 邮件/短信告警
   - 第三方平台集成

4. **数据统计**
   - 按任务类型统计分析数量
   - 异常事件统计
   - 趋势分析

参考文档：
- [doc/FRAME_EXTRACTOR.md](FRAME_EXTRACTOR.md) - 抽帧插件完整文档
- [doc/TASK_TYPES.md](TASK_TYPES.md) - 任务类型功能详解
- [doc/LIVE_STREAM_URL.md](LIVE_STREAM_URL.md) - 直播流地址说明

