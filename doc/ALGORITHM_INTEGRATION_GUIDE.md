# 算法对接说明书

## 📚 yanying智能视频分析平台 - 算法服务对接指南

**版本**: v1.0  
**更新日期**: 2024-10-17  
**适用对象**: 算法开发者、AI工程师

---

## 📋 目录

1. [概述](#概述)
2. [对接流程](#对接流程)
3. [接口规范](#接口规范)
4. [算法配置使用](#算法配置使用)
5. [代码示例](#代码示例)
6. [测试验证](#测试验证)
7. [常见问题](#常见问题)
8. [技术支持](#技术支持)

---

## 概述

### 系统架构

```
┌─────────────────┐
│  EasyDarwin     │
│  视频流媒体      │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│ Frame Extractor │  ← 抽帧插件
│  视频抽帧        │
└────────┬────────┘
         │ MinIO
         ↓
┌─────────────────┐
│  AI Analysis    │  ← 智能分析插件
│  推理调度        │
└────────┬────────┘
         │ HTTP
         ↓
┌─────────────────┐
│  Algorithm      │  ← **您的算法服务**
│  Service        │
└─────────────────┘
```

### 对接价值

- ✅ **零侵入**：算法服务独立部署，不影响主系统
- ✅ **灵活配置**：Web界面绘制区域配置，无需改代码
- ✅ **自动调度**：图片自动推送，算法专注推理逻辑
- ✅ **水平扩展**：支持多算法服务负载均衡
- ✅ **故障隔离**：算法服务故障不影响主系统

---

## 对接流程

### Step 1: 注册算法服务

```python
import requests
import json

# 注册信息
register_data = {
    "service_id": "my_algorithm_v1",      # 服务唯一ID
    "name": "我的目标检测算法",             # 服务名称
    "task_types": ["人数统计", "车辆检测"],  # 支持的任务类型
    "endpoint": "http://192.168.1.100:8000/infer",  # 推理接口地址
    "version": "1.0.0"                    # 版本号
}

# 注册到EasyDarwin
response = requests.post(
    "http://easydarwin-host:5066/api/v1/ai_analysis/register",
    json=register_data
)

print(response.json())
# {"ok": true, "message": "registered successfully"}
```

### Step 2: 发送心跳（每30秒）

```python
import time

service_id = "my_algorithm_v1"

while True:
    requests.post(
        f"http://easydarwin-host:5066/api/v1/ai_analysis/heartbeat/{service_id}"
    )
    time.sleep(30)
```

### Step 3: 实现推理接口

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    
    # 接收参数
    image_url = data['image_url']      # 图片URL（预签名，可直接下载）
    task_id = data['task_id']          # 任务ID
    task_type = data['task_type']      # 任务类型
    algo_config = data.get('algo_config', {})  # 算法配置（可选）
    
    # 执行推理
    result = your_algorithm_inference(image_url, task_type, algo_config)
    
    # 返回结果
    return jsonify({
        "success": True,
        "result": {
            "total_count": len(result['detections']),  # ⚠️ 必须返回！
            "detections": result['detections'],
            "message": f"检测到{len(result['detections'])}个对象"
        },
        "confidence": 0.95,
        "inference_time_ms": 45
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)
```

---

## 接口规范

### 推理请求（POST）

**URL**: 您的算法服务地址（如 `http://192.168.1.100:8000/infer`）

**Headers**:
```
Content-Type: application/json
```

**Request Body**:
```json
{
  "image_url": "http://minio:9000/images/frames/...?X-Amz-...",
  "task_id": "cam_entrance_001",
  "task_type": "人数统计",
  "image_path": "frames/人数统计/cam_entrance_001/20241017-143520.jpg",
  "algo_config": {
    "task_id": "cam_entrance_001",
    "task_type": "人数统计",
    "config_version": "1.0",
    "regions": [
      {
        "id": "region_001",
        "name": "入口区域",
        "type": "polygon",
        "enabled": true,
        "points": [[100, 200], [300, 200], [300, 400], [100, 400]],
        "properties": {
          "color": "#FF0000",
          "threshold": 0.5
        }
      }
    ],
    "algorithm_params": {
      "confidence_threshold": 0.7,
      "iou_threshold": 0.5
    }
  }
}
```

**字段说明**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `image_url` | string | ✅ | MinIO预签名URL，有效期1小时，可直接下载 |
| `task_id` | string | ✅ | 任务唯一标识 |
| `task_type` | string | ✅ | 任务类型（人数统计、车辆检测等） |
| `image_path` | string | ✅ | MinIO对象路径 |
| `algo_config` | object | ❌ | 算法配置（可能为空，见下文） |

### 推理响应

**Response Body**:
```json
{
  "success": true,
  "result": {
    "total_count": 3,
    "detections": [
      {
        "class": "person",
        "confidence": 0.95,
        "bbox": [100, 200, 150, 300],
        "region_id": "region_001",
        "region_name": "入口区域"
      }
    ],
    "region_results": [
      {
        "region_id": "region_001",
        "region_name": "入口区域",
        "count": 3,
        "alert": false
      }
    ],
    "message": "检测到3个对象"
  },
  "confidence": 0.95,
  "inference_time_ms": 45
}
```

**返回字段说明**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `success` | boolean | ✅ | 是否成功 |
| `result` | object | ✅ | 推理结果 |
| `result.total_count` | number | ✅ | **检测对象总数（为0时图片会被自动删除）** |
| `result.detections` | array | ❌ | 检测详情列表 |
| `result.message` | string | ❌ | 描述信息 |
| `confidence` | number | ❌ | 平均置信度 |
| `inference_time_ms` | number | ❌ | 推理耗时（毫秒）|

⚠️ **重要**：`result.total_count` 必须返回！当值为0时，系统会自动删除该图片。

---

## 算法配置使用

### 配置结构

```json
{
  "regions": [
    {
      "id": "region_001",
      "name": "区域名称",
      "type": "polygon|line|rectangle",
      "enabled": true,
      "points": [[x1,y1], [x2,y2], ...],
      "properties": {
        "color": "#FF0000",
        "threshold": 0.5,
        "custom_key": "custom_value"
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "iou_threshold": 0.5,
    "custom_params": {}
  }
}
```

### 区域类型

#### 1. 多边形（polygon）

```json
{
  "type": "polygon",
  "points": [[100, 150], [400, 150], [400, 450], [100, 450]]
}
```

**用途**：区域计数、区域入侵检测

**检测方法**：判断检测框中心点是否在多边形内

```python
import cv2
import numpy as np

def point_in_polygon(point, polygon):
    """判断点是否在多边形内"""
    x, y = point
    poly = np.array(polygon, dtype=np.int32)
    return cv2.pointPolygonTest(poly, (x, y), False) >= 0

# 使用示例
center_x = (bbox[0] + bbox[2]) / 2
center_y = (bbox[1] + bbox[3]) / 2
if point_in_polygon((center_x, center_y), region['points']):
    print("在区域内")
```

#### 2. 线（line）

```json
{
  "type": "line",
  "points": [[300, 200], [600, 200]],
  "properties": {
    "direction": "bidirectional"  // "in" | "out" | "bidirectional"
  }
}
```

**用途**：越线检测、人流统计

**检测方法**：判断轨迹是否穿越线段

```python
def check_line_crossing(prev_point, curr_point, line_points):
    """检测是否越线"""
    def ccw(A, B, C):
        return (C[1]-A[1]) * (B[0]-A[0]) > (B[1]-A[1]) * (C[0]-A[0])
    
    A, B = line_points[0], line_points[1]
    C, D = prev_point, curr_point
    
    return ccw(A,C,D) != ccw(B,C,D) and ccw(A,B,C) != ccw(A,B,D)
```

#### 3. 矩形（rectangle）

```json
{
  "type": "rectangle",
  "points": [[100, 100], [500, 400]]  // [左上角, 右下角]
}
```

**用途**：快速区域检测

**检测方法**：简单的坐标判断

```python
def point_in_rectangle(point, rect_points):
    """判断点是否在矩形内"""
    x, y = point
    x1, y1 = rect_points[0]
    x2, y2 = rect_points[1]
    return x1 <= x <= x2 and y1 <= y <= y2
```

---

## 代码示例

### 完整Python示例（Flask）

```python
#!/usr/bin/env python3
import requests
import json
import time
import threading
from flask import Flask, request, jsonify
from PIL import Image
import cv2
import numpy as np
from ultralytics import YOLO

app = Flask(__name__)

# 加载YOLO模型
model = YOLO('yolov8n.pt')

# 配置
EASYDARWIN_HOST = "http://192.168.1.10:5066"
SERVICE_ID = "my_algorithm_v1"
SERVICE_NAME = "我的目标检测算法"
TASK_TYPES = ["人数统计", "车辆检测", "安全帽检测"]
ENDPOINT = "http://192.168.1.100:8000/infer"

def register_service():
    """注册算法服务"""
    data = {
        "service_id": SERVICE_ID,
        "name": SERVICE_NAME,
        "task_types": TASK_TYPES,
        "endpoint": ENDPOINT,
        "version": "1.0.0"
    }
    
    try:
        resp = requests.post(f"{EASYDARWIN_HOST}/api/v1/ai_analysis/register", json=data)
        print(f"✅ 注册成功: {resp.json()}")
        return True
    except Exception as e:
        print(f"❌ 注册失败: {e}")
        return False

def heartbeat_loop():
    """心跳循环"""
    while True:
        try:
            requests.post(f"{EASYDARWIN_HOST}/api/v1/ai_analysis/heartbeat/{SERVICE_ID}")
            print("♥ 心跳成功")
        except Exception as e:
            print(f"心跳失败: {e}")
        time.sleep(30)

def download_image(image_url):
    """下载图片"""
    import urllib.request
    import tempfile
    
    temp_file = tempfile.NamedTemporaryFile(delete=False, suffix='.jpg')
    urllib.request.urlretrieve(image_url, temp_file.name)
    return temp_file.name

def point_in_polygon(point, polygon):
    """判断点是否在多边形内"""
    x, y = point
    poly = np.array(polygon, dtype=np.int32)
    return cv2.pointPolygonTest(poly, (x, y), False) >= 0

def detect_in_region(detections, region):
    """在指定区域内检测"""
    result_detections = []
    
    for det in detections:
        bbox = det['bbox']  # [x1, y1, x2, y2]
        center_x = (bbox[0] + bbox[2]) / 2
        center_y = (bbox[1] + bbox[3]) / 2
        
        in_region = False
        if region['type'] == 'polygon':
            in_region = point_in_polygon((center_x, center_y), region['points'])
        elif region['type'] == 'rectangle':
            x1, y1 = region['points'][0]
            x2, y2 = region['points'][1]
            in_region = x1 <= center_x <= x2 and y1 <= center_y <= y2
        
        if in_region:
            det['region_id'] = region['id']
            det['region_name'] = region['name']
            result_detections.append(det)
    
    return result_detections

@app.route('/infer', methods=['POST'])
def infer():
    """推理接口"""
    data = request.json
    
    image_url = data['image_url']
    task_id = data['task_id']
    task_type = data['task_type']
    algo_config = data.get('algo_config', {})
    
    print(f"收到推理请求: task_id={task_id}, task_type={task_type}")
    
    try:
        # 1. 下载图片
        image_path = download_image(image_url)
        
        # 2. YOLO推理
        results = model.predict(image_path, conf=0.7)
        
        # 3. 解析结果
        all_detections = []
        for result in results:
            boxes = result.boxes
            for box in boxes:
                cls_id = int(box.cls[0])
                class_name = result.names[cls_id]
                confidence = float(box.conf[0])
                bbox = box.xyxy[0].tolist()
                
                all_detections.append({
                    "class": class_name,
                    "confidence": confidence,
                    "bbox": [int(x) for x in bbox]
                })
        
        # 4. 应用区域配置
        final_detections = []
        region_results = []
        
        regions = algo_config.get('regions', [])
        if regions:
            # 有区域配置，按区域过滤
            for region in regions:
                if not region.get('enabled', True):
                    continue
                
                region_dets = detect_in_region(all_detections, region)
                final_detections.extend(region_dets)
                
                region_results.append({
                    "region_id": region['id'],
                    "region_name": region['name'],
                    "count": len(region_dets),
                    "alert": len(region_dets) > 0
                })
        else:
            # 无区域配置，返回所有检测
            final_detections = all_detections
        
        # 5. 构建响应
        response = {
            "success": True,
            "result": {
                "total_count": len(final_detections),
                "detections": final_detections,
                "region_results": region_results,
                "message": f"检测到{len(final_detections)}个对象"
            },
            "confidence": sum([d['confidence'] for d in final_detections]) / len(final_detections) if final_detections else 0,
            "inference_time_ms": 45  # 实际应测量
        }
        
        print(f"推理完成: total_count={response['result']['total_count']}")
        return jsonify(response)
        
    except Exception as e:
        print(f"推理失败: {e}")
        return jsonify({
            "success": False,
            "error": str(e)
        }), 500

if __name__ == '__main__':
    # 注册服务
    if register_service():
        # 启动心跳线程
        thread = threading.Thread(target=heartbeat_loop, daemon=True)
        thread.start()
        
        # 启动HTTP服务
        print(f"🚀 算法服务已启动")
        print(f"   服务ID: {SERVICE_ID}")
        print(f"   端点: {ENDPOINT}")
        print(f"   支持类型: {TASK_TYPES}")
        app.run(host='0.0.0.0', port=8000)
```

---

## 测试验证

### 1. 手动测试推理接口

```bash
curl -X POST http://192.168.1.100:8000/infer \
  -H "Content-Type: application/json" \
  -d '{
    "image_url": "https://example.com/test.jpg",
    "task_id": "test_001",
    "task_type": "人数统计",
    "algo_config": {
      "regions": [],
      "algorithm_params": {"confidence_threshold": 0.7}
    }
  }'
```

### 2. 验证注册状态

```bash
curl http://easydarwin-host:5066/api/v1/ai_analysis/services
```

### 3. 查看告警记录

```bash
curl http://easydarwin-host:5066/api/v1/ai_analysis/alerts?task_id=cam_entrance_001
```

---

## 常见问题

### Q1: 如何处理无算法配置的情况？

**A**: 检查 `algo_config` 是否为空，为空时对全图进行检测：

```python
algo_config = data.get('algo_config', {})
regions = algo_config.get('regions', [])

if not regions:
    # 无区域配置，检测全图
    final_detections = all_detections
else:
    # 有区域配置，按区域过滤
    final_detections = filter_by_regions(all_detections, regions)
```

### Q2: total_count 必须准确吗？

**A**: 是的！`total_count` 直接影响图片是否被删除：
- `total_count = 0` → 图片被删除，不保存告警
- `total_count > 0` → 图片保留，保存告警

### Q3: 算法服务崩溃后怎么办？

**A**: 
1. 重新启动算法服务
2. 再次调用注册接口
3. 系统会自动恢复推理任务

### Q4: 如何支持多任务类型？

**A**: 在注册时指定 `task_types` 数组：

```python
"task_types": ["人数统计", "车辆检测", "安全帽检测"]
```

推理时根据 `task_type` 参数选择对应逻辑。

### Q5: 图片下载失败怎么处理？

**A**: 
1. 检查网络连接
2. 预签名URL有效期1小时，过期需重新请求
3. 实现重试机制

```python
import time

def download_with_retry(url, max_retries=3):
    for i in range(max_retries):
        try:
            return download_image(url)
        except Exception as e:
            if i == max_retries - 1:
                raise
            time.sleep(1)
```

### Q6: 如何优化推理性能？

**A**:
1. **批量推理**：积累多张图片一起推理
2. **GPU加速**：使用CUDA
3. **模型量化**：INT8/FP16
4. **缓存配置**：避免每次解析JSON
5. **异步处理**：使用消息队列

### Q7: 坐标系统是什么？

**A**: 
- 原点 (0,0) 在图像左上角
- x轴向右递增
- y轴向下递增
- 单位：像素
- 基于图像原始分辨率

---

## 技术支持

### 文档

- **算法配置规范**: `doc/ALGORITHM_CONFIG_SPEC.md`
- **项目README**: `README_CN.md`
- **API文档**: `doc/EasyDarwin.api.html`

### 示例代码

- **简单示例**: `examples/algorithm_service.py`
- **YOLO示例**: `examples/yolo_algorithm_service.py`
- **测试脚本**: `test_auto_delete.py`

### 联系方式

- **项目地址**: https://github.com/zhouyingchaoAI/easyAIServer
- **Issues**: 提交问题和建议

---

## 快速检查清单

部署前检查：

- [ ] 算法服务实现了 `/infer` 接口
- [ ] 返回结果包含 `total_count` 字段
- [ ] 实现了服务注册逻辑
- [ ] 实现了心跳机制（每30秒）
- [ ] 能够处理算法配置（regions）
- [ ] 能够正确下载和处理图片
- [ ] 错误处理完善
- [ ] 日志记录完整

---

**祝您对接顺利！** 🎉

如有问题，请随时查阅文档或提交Issue。

