# EasyDarwin AI算法服务对接指南

**版本**: v1.0  
**更新时间**: 2025-10-20  
**适用对象**: 算法开发者、AI服务提供商

---

## 📖 目录

1. [概述](#概述)
2. [服务注册](#服务注册)
3. [推理接口](#推理接口)
4. [配置文件格式](#配置文件格式)
5. [响应格式](#响应格式)
6. [心跳机制](#心跳机制)
7. [完整示例](#完整示例)
8. [调试指南](#调试指南)

---

## 概述

### 系统架构

```
┌──────────────┐
│   EasyDarwin │
│   主系统     │
└──────┬───────┘
       │
       │ HTTP
       ↓
┌──────────────┐
│  您的算法服务 │
│  (需要实现)  │
└──────────────┘
```

### 工作流程

```
1. 服务启动 → 注册到EasyDarwin
   ↓
2. EasyDarwin抽取视频帧 → 上传MinIO
   ↓
3. 扫描到新图片 → 发送推理请求
   ↓
4. 您的算法服务 → 处理请求 → 返回结果
   ↓
5. EasyDarwin → 保存告警 → 显示在前端
```

---

## 服务注册

### 1. 注册API

**端点**: `POST http://{easydarwin_host}:5066/api/v1/ai_analysis/register`

**请求体**:
```json
{
  "service_id": "tripwire_service_001",
  "name": "绊线人数统计服务",
  "task_types": ["绊线人数统计", "人数统计"],
  "endpoint": "http://your-algorithm-server:8000/infer",
  "version": "1.0.0"
}
```

**字段说明**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| service_id | string | ✅ | 服务唯一标识，建议使用UUID |
| name | string | ✅ | 服务名称，便于识别 |
| task_types | []string | ✅ | 支持的任务类型列表 |
| endpoint | string | ✅ | 推理接口的完整URL |
| version | string | ✅ | 服务版本号 |

**响应**:
```json
{
  "ok": true,
  "service_id": "tripwire_service_001"
}
```

### 2. 支持的任务类型

当前系统中的任务类型：

```
- 人数统计
- 绊线人数统计 ⭐ (新增)
- 人员跌倒
- 人员离岗
- 吸烟检测
- 区域入侵
- 徘徊检测
- 物品遗留
- 安全帽检测
```

**注意**: 您的服务可以支持一种或多种任务类型

### 3. Python注册示例

```python
import requests
import uuid

def register_to_easydarwin():
    """注册算法服务到EasyDarwin"""
    
    # EasyDarwin主服务地址
    easydarwin_url = "http://10.1.6.230:5066"
    
    # 您的算法服务信息
    service_info = {
        "service_id": str(uuid.uuid4()),  # 生成唯一ID
        "name": "绊线人数统计服务",
        "task_types": [
            "绊线人数统计",
            "人数统计"
        ],
        "endpoint": "http://192.168.1.100:8000/infer",  # 您的服务地址
        "version": "1.0.0"
    }
    
    # 发送注册请求
    response = requests.post(
        f"{easydarwin_url}/api/v1/ai_analysis/register",
        json=service_info,
        timeout=10
    )
    
    if response.status_code == 200:
        print("✅ 服务注册成功!")
        print(f"Service ID: {service_info['service_id']}")
        return service_info['service_id']
    else:
        print(f"❌ 注册失败: {response.text}")
        return None

# 在服务启动时调用
if __name__ == "__main__":
    service_id = register_to_easydarwin()
```

---

## 推理接口

### 1. 接口规范

**您需要实现的HTTP接口**:

```
POST http://your-server:port/infer
Content-Type: application/json
```

### 2. 推理请求格式

EasyDarwin会向您的服务发送如下请求：

```json
{
  "image_url": "http://10.1.6.230:9000/images/绊线人数统计/公司入口统计/20251020-094708.979.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=admin%2F20251020%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251020T014708Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=...",
  "task_id": "公司入口统计",
  "task_type": "绊线人数统计",
  "image_path": "绊线人数统计/公司入口统计/20251020-094708.979.jpg",
  "algo_config": {
    "task_id": "公司入口统计",
    "task_type": "绊线人数统计",
    "config_version": "1.0",
    "regions": [
      {
        "id": "region_1729411234567",
        "name": "入口检测线",
        "type": "line",
        "enabled": true,
        "points": [[100, 300], [700, 300]],
        "properties": {
          "direction": "in",
          "color": "#00FF00",
          "thickness": 3
        }
      }
    ],
    "algorithm_params": {
      "confidence_threshold": 0.7,
      "iou_threshold": 0.5
    }
  },
  "algo_config_url": "http://10.1.6.230:9000/images/绊线人数统计/公司入口统计/algo_config.json?X-Amz-Algorithm=..."
}
```

### 3. 字段详解

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| **image_url** | string | MinIO预签名URL，可直接下载图片 | `http://...` |
| **task_id** | string | 任务唯一标识 | `"公司入口统计"` |
| **task_type** | string | 任务类型，匹配注册时的类型 | `"绊线人数统计"` |
| **image_path** | string | 图片在MinIO中的路径 | `"绊线人数统计/..."` |
| **algo_config** | object | 算法配置内容（JSON对象）| `{...}` |
| **algo_config_url** | string | 配置文件URL（可选使用） | `http://...` |

### 4. 配置获取方式

**方式1: 使用请求中的配置内容（推荐）**

```python
def infer(request_data):
    # 直接从请求体获取
    algo_config = request_data.get('algo_config', {})
    
    # 解析配置
    regions = algo_config.get('regions', [])
    params = algo_config.get('algorithm_params', {})
    
    confidence_threshold = params.get('confidence_threshold', 0.7)
    iou_threshold = params.get('iou_threshold', 0.5)
    
    # 使用配置进行推理...
```

**方式2: 通过URL下载配置（备用）**

```python
import requests

def infer(request_data):
    # 从URL下载
    config_url = request_data.get('algo_config_url')
    
    if config_url:
        try:
            response = requests.get(config_url, timeout=10)
            algo_config = response.json()
        except Exception as e:
            print(f"下载配置失败，使用默认配置: {e}")
            algo_config = get_default_config()
    else:
        algo_config = request_data.get('algo_config', {})
    
    # 使用配置进行推理...
```

---

## 配置文件格式

### 1. 完整配置结构

```json
{
  "task_id": "公司入口统计",
  "task_type": "绊线人数统计",
  "config_version": "1.0",
  "created_at": "2025-10-20T10:00:00Z",
  "updated_at": "2025-10-20T10:30:00Z",
  "regions": [
    {
      "id": "region_1729411234567",
      "name": "入口检测线",
      "type": "line",
      "enabled": true,
      "points": [[100, 300], [700, 300]],
      "properties": {
        "direction": "in",
        "color": "#00FF00",
        "thickness": 3
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "iou_threshold": 0.5
  }
}
```

### 2. 区域类型

#### 线条（line）- 用于绊线检测

```json
{
  "type": "line",
  "points": [[x1, y1], [x2, y2]],  // 起点和终点
  "properties": {
    "direction": "in",  // "in"|"out"|"in_out"
    "color": "#00FF00",
    "thickness": 3
  }
}
```

**direction 字段说明**:
- `"in"` - 进入方向（从上方穿过线条到下方）
- `"out"` - 离开方向（从下方穿过线条到上方）
- `"in_out"` - 双向统计

#### 矩形（rectangle）- 用于区域检测

```json
{
  "type": "rectangle",
  "points": [[x1, y1], [x2, y2]],  // 左上角和右下角
  "properties": {
    "color": "#00FF00",
    "opacity": 0.3,
    "threshold": 0.5
  }
}
```

#### 多边形（polygon）- 用于不规则区域

```json
{
  "type": "polygon",
  "points": [[x1, y1], [x2, y2], [x3, y3], ...],
  "properties": {
    "color": "#0000FF",
    "opacity": 0.3,
    "threshold": 0.5
  }
}
```

### 3. 坐标系统

```
原点 (0, 0) 在图片左上角

  0 ────────→ X轴
  │
  │
  ↓ Y轴

示例图片分辨率: 1920x1080
X范围: 0 - 1920
Y范围: 0 - 1080
```

---

## 响应格式

### 1. 成功响应

**您的算法服务需要返回**:

```json
{
  "success": true,
  "result": {
    "total_count": 2,
    "detections": [
      {
        "class": "person",
        "confidence": 0.95,
        "bbox": [350, 150, 100, 250],
        "track_id": "track_42"
      },
      {
        "class": "person",
        "confidence": 0.88,
        "bbox": [600, 180, 95, 240],
        "track_id": "track_43"
      }
    ],
    "crossings": [
      {
        "line_name": "入口检测线",
        "direction": "in",
        "person_id": "track_42",
        "cross_point": [400, 300],
        "confidence": 0.95
      }
    ]
  },
  "confidence": 0.915,
  "inference_time_ms": 85
}
```

### 2. 失败响应

```json
{
  "success": false,
  "error": "Image download failed",
  "result": null,
  "confidence": 0.0,
  "inference_time_ms": 0
}
```

### 3. 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| **success** | boolean | ✅ | 推理是否成功 |
| **result** | object/null | ✅ | 推理结果（自定义结构） |
| **confidence** | float | ✅ | 平均置信度（0.0-1.0） |
| **inference_time_ms** | int | ✅ | 推理耗时（毫秒） |
| **error** | string | ❌ | 失败时的错误信息 |

### 4. result 结构建议

**绊线人数统计**:
```json
{
  "total_count": 2,           // 检测到的人数
  "detections": [...],        // 所有检测到的人员
  "crossings": [...]          // 穿越事件
}
```

**区域入侵检测**:
```json
{
  "total_count": 1,           // 入侵目标数
  "intrusions": [             // 入侵详情
    {
      "region_name": "禁区A",
      "object_class": "person",
      "confidence": 0.92
    }
  ]
}
```

**重要**: EasyDarwin会提取 `total_count` 或 `detections.length` 作为检测个数

---

## 推理接口实现

### Python Flask 示例

```python
from flask import Flask, request, jsonify
import requests
import cv2
import numpy as np
import time

app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    """
    推理接口
    """
    try:
        # 1. 解析请求
        data = request.json
        image_url = data.get('image_url')
        task_id = data.get('task_id')
        task_type = data.get('task_type')
        algo_config = data.get('algo_config', {})
        
        print(f"收到推理请求:")
        print(f"  任务ID: {task_id}")
        print(f"  任务类型: {task_type}")
        print(f"  图片URL: {image_url}")
        
        # 2. 下载图片
        image = download_image(image_url)
        if image is None:
            return jsonify({
                "success": False,
                "error": "Failed to download image",
                "result": None,
                "confidence": 0.0,
                "inference_time_ms": 0
            })
        
        # 3. 执行推理
        start_time = time.time()
        
        if task_type == "绊线人数统计":
            result = tripwire_counting(image, algo_config)
        elif task_type == "人数统计":
            result = person_counting(image, algo_config)
        else:
            result = default_inference(image, algo_config)
        
        inference_time = int((time.time() - start_time) * 1000)
        
        # 4. 返回结果
        return jsonify({
            "success": True,
            "result": result,
            "confidence": result.get('avg_confidence', 0.0),
            "inference_time_ms": inference_time
        })
        
    except Exception as e:
        print(f"推理异常: {str(e)}")
        return jsonify({
            "success": False,
            "error": str(e),
            "result": None,
            "confidence": 0.0,
            "inference_time_ms": 0
        })

def download_image(url):
    """下载图片"""
    try:
        response = requests.get(url, timeout=10)
        if response.status_code == 200:
            # 转换为OpenCV格式
            arr = np.frombuffer(response.content, np.uint8)
            img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
            return img
        return None
    except Exception as e:
        print(f"下载图片失败: {e}")
        return None

def tripwire_counting(image, config):
    """
    绊线人数统计算法
    """
    # 1. 获取检测线配置
    regions = config.get('regions', [])
    lines = [r for r in regions if r['type'] == 'line' and r.get('enabled', True)]
    
    # 2. 获取算法参数
    params = config.get('algorithm_params', {})
    conf_threshold = params.get('confidence_threshold', 0.7)
    iou_threshold = params.get('iou_threshold', 0.5)
    
    # 3. 人员检测（使用您的检测模型）
    persons = detect_persons(image, conf_threshold, iou_threshold)
    
    # 4. 绊线判断
    crossings = []
    for line in lines:
        direction = line['properties']['direction']
        points = line['points']  # [[x1, y1], [x2, y2]]
        
        # 检查每个人是否穿越（您需要实现轨迹跟踪）
        for person in persons:
            if check_line_crossing(person, points, direction):
                crossings.append({
                    "line_name": line.get('name', 'unknown'),
                    "direction": direction,
                    "person_id": person.get('track_id', 'unknown'),
                    "cross_point": person.get('center', [0, 0]),
                    "confidence": person['confidence']
                })
    
    # 5. 构建结果
    avg_conf = sum([p['confidence'] for p in persons]) / len(persons) if persons else 0.0
    
    return {
        "total_count": len(crossings),
        "detections": persons,
        "crossings": crossings,
        "avg_confidence": round(avg_conf, 3)
    }

def detect_persons(image, conf_threshold, iou_threshold):
    """
    人员检测（使用您的模型）
    
    返回格式:
    [
      {
        "class": "person",
        "confidence": 0.95,
        "bbox": [x, y, w, h],
        "track_id": "track_42"
      }
    ]
    """
    # TODO: 实现您的检测逻辑
    # 示例：使用YOLO模型
    # results = model(image)
    # persons = parse_results(results)
    return []

def check_line_crossing(person, line_points, direction):
    """
    检查人员是否穿越检测线
    
    Args:
        person: 人员检测结果
        line_points: [[x1, y1], [x2, y2]]
        direction: "in"|"out"|"in_out"
    
    Returns:
        bool: 是否穿越
    """
    # TODO: 实现穿越判断逻辑
    # 需要维护人员轨迹，判断是否跨越线条
    return False

if __name__ == '__main__':
    # 启动服务
    app.run(host='0.0.0.0', port=8000)
```

### Go (Gin) 示例

```go
package main

import (
    "encoding/json"
    "github.com/gin-gonic/gin"
    "io"
    "net/http"
    "time"
)

type InferenceRequest struct {
    ImageURL      string                 `json:"image_url"`
    TaskID        string                 `json:"task_id"`
    TaskType      string                 `json:"task_type"`
    ImagePath     string                 `json:"image_path"`
    AlgoConfig    map[string]interface{} `json:"algo_config"`
    AlgoConfigURL string                 `json:"algo_config_url"`
}

type InferenceResponse struct {
    Success         bool        `json:"success"`
    Result          interface{} `json:"result"`
    Confidence      float64     `json:"confidence"`
    InferenceTimeMs int         `json:"inference_time_ms"`
    Error           string      `json:"error,omitempty"`
}

func main() {
    r := gin.Default()
    
    r.POST("/infer", handleInference)
    
    r.Run(":8000")
}

func handleInference(c *gin.Context) {
    var req InferenceRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, InferenceResponse{
            Success: false,
            Error:   err.Error(),
        })
        return
    }
    
    // 下载图片
    image, err := downloadImage(req.ImageURL)
    if err != nil {
        c.JSON(200, InferenceResponse{
            Success: false,
            Error:   "Failed to download image: " + err.Error(),
        })
        return
    }
    
    // 执行推理
    startTime := time.Now()
    result := runInference(image, req.AlgoConfig, req.TaskType)
    inferTime := int(time.Since(startTime).Milliseconds())
    
    // 返回结果
    c.JSON(200, InferenceResponse{
        Success:         true,
        Result:          result,
        Confidence:      calculateConfidence(result),
        InferenceTimeMs: inferTime,
    })
}

func downloadImage(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}
```

---

## 心跳机制

### 1. 心跳API

**端点**: `POST http://{easydarwin_host}:5066/api/v1/ai_analysis/heartbeat/{service_id}`

**说明**: 每30-60秒发送一次心跳，保持服务在线状态

### 2. Python心跳示例

```python
import requests
import threading
import time

class HeartbeatThread(threading.Thread):
    def __init__(self, easydarwin_url, service_id):
        super().__init__(daemon=True)
        self.easydarwin_url = easydarwin_url
        self.service_id = service_id
        self.running = True
    
    def run(self):
        """每45秒发送一次心跳"""
        while self.running:
            try:
                response = requests.post(
                    f"{self.easydarwin_url}/api/v1/ai_analysis/heartbeat/{self.service_id}",
                    timeout=5
                )
                if response.status_code == 200:
                    print("✅ 心跳发送成功")
                else:
                    print(f"⚠️ 心跳失败: {response.status_code}")
            except Exception as e:
                print(f"❌ 心跳异常: {e}")
            
            time.sleep(45)  # 45秒间隔
    
    def stop(self):
        self.running = False

# 使用示例
heartbeat = HeartbeatThread(
    easydarwin_url="http://10.1.6.230:5066",
    service_id="your_service_id"
)
heartbeat.start()
```

### 3. 注销服务

**端点**: `DELETE http://{easydarwin_host}:5066/api/v1/ai_analysis/unregister/{service_id}`

```python
def unregister_service(service_id):
    """注销服务"""
    response = requests.delete(
        f"http://10.1.6.230:5066/api/v1/ai_analysis/unregister/{service_id}"
    )
    if response.status_code == 200:
        print("服务已注销")
```

---

## 完整示例

### Python完整实现

```python
#!/usr/bin/env python3
"""
EasyDarwin算法服务 - 绊线人数统计
"""

from flask import Flask, request, jsonify
import requests
import cv2
import numpy as np
import time
import uuid
import threading

app = Flask(__name__)

# 全局变量
SERVICE_ID = str(uuid.uuid4())
EASYDARWIN_URL = "http://10.1.6.230:5066"
SERVICE_PORT = 8000

# 配置缓存
config_cache = {}

# ============= 推理接口 =============

@app.route('/infer', methods=['POST'])
def infer():
    """推理接口"""
    try:
        data = request.json
        
        # 解析请求
        image_url = data['image_url']
        task_id = data['task_id']
        task_type = data['task_type']
        algo_config = data.get('algo_config', {})
        algo_config_url = data.get('algo_config_url', '')
        
        print(f"\n{'='*50}")
        print(f"收到推理请求:")
        print(f"  任务ID: {task_id}")
        print(f"  任务类型: {task_type}")
        print(f"  图片URL: {image_url[:80]}...")
        if algo_config_url:
            print(f"  配置URL: {algo_config_url[:80]}...")
        print(f"{'='*50}\n")
        
        # 下载图片
        image = download_image(image_url)
        if image is None:
            return error_response("图片下载失败")
        
        # 执行推理
        start_time = time.time()
        
        if task_type in ["绊线人数统计", "人数统计"]:
            result = tripwire_counting(image, algo_config)
        else:
            result = default_detection(image, algo_config)
        
        inference_time = int((time.time() - start_time) * 1000)
        
        print(f"✅ 推理完成: 耗时{inference_time}ms, 检测数={result.get('total_count', 0)}")
        
        # 返回结果
        return jsonify({
            "success": True,
            "result": result,
            "confidence": result.get('avg_confidence', 0.0),
            "inference_time_ms": inference_time
        })
        
    except Exception as e:
        print(f"❌ 推理失败: {str(e)}")
        import traceback
        traceback.print_exc()
        return error_response(str(e))

def error_response(error_msg):
    """错误响应"""
    return jsonify({
        "success": False,
        "error": error_msg,
        "result": None,
        "confidence": 0.0,
        "inference_time_ms": 0
    })

# ============= 核心算法 =============

def download_image(url):
    """下载图片"""
    try:
        response = requests.get(url, timeout=10)
        if response.status_code == 200:
            arr = np.frombuffer(response.content, np.uint8)
            img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
            return img
        return None
    except Exception as e:
        print(f"下载失败: {e}")
        return None

def tripwire_counting(image, config):
    """
    绊线人数统计算法
    
    Args:
        image: OpenCV图片 (numpy array)
        config: 算法配置
            {
                "regions": [
                    {
                        "type": "line",
                        "points": [[x1, y1], [x2, y2]],
                        "properties": {
                            "direction": "in"|"out"|"in_out"
                        }
                    }
                ],
                "algorithm_params": {
                    "confidence_threshold": 0.7,
                    "iou_threshold": 0.5
                }
            }
    
    Returns:
        {
            "total_count": int,
            "detections": [...],
            "crossings": [...]
        }
    """
    # 获取配置
    regions = config.get('regions', [])
    params = config.get('algorithm_params', {})
    
    conf_threshold = params.get('confidence_threshold', 0.7)
    iou_threshold = params.get('iou_threshold', 0.5)
    
    # 提取检测线
    lines = [r for r in regions if r['type'] == 'line' and r.get('enabled', True)]
    
    print(f"  配置: {len(lines)}条检测线, 置信度阈值={conf_threshold}")
    
    # TODO: 实现您的检测逻辑
    # 1. 人员检测
    persons = detect_persons_yolo(image, conf_threshold, iou_threshold)
    
    # 2. 轨迹跟踪
    tracks = update_tracks(persons)
    
    # 3. 绊线判断
    crossings = []
    for line in lines:
        direction = line['properties']['direction']
        points = line['points']
        
        for track in tracks:
            if is_crossing_line(track, points, direction):
                crossings.append({
                    "line_name": line.get('name', 'unknown'),
                    "direction": direction,
                    "person_id": track['id'],
                    "cross_point": track['center'],
                    "confidence": track['confidence']
                })
    
    # 计算平均置信度
    if persons:
        avg_conf = sum([p['confidence'] for p in persons]) / len(persons)
    else:
        avg_conf = 0.0
    
    return {
        "total_count": len(crossings),
        "detections": persons,
        "crossings": crossings,
        "avg_confidence": round(avg_conf, 3)
    }

def detect_persons_yolo(image, conf_threshold, iou_threshold):
    """
    人员检测（示例）
    
    TODO: 替换为您的实际检测模型
    """
    # 示例返回格式
    return [
        {
            "class": "person",
            "confidence": 0.95,
            "bbox": [350, 150, 100, 250],  # [x, y, w, h]
            "center": [400, 275],
            "track_id": "track_1"
        }
    ]

def is_crossing_line(track, line_points, direction):
    """
    判断是否穿越检测线
    
    TODO: 实现穿越判断逻辑
    需要维护轨迹历史，判断轨迹是否跨越线段
    """
    # 简化示例（实际需要轨迹跟踪）
    return False

# ============= 服务注册和心跳 =============

def register_service():
    """注册服务"""
    service_info = {
        "service_id": SERVICE_ID,
        "name": "绊线人数统计服务",
        "task_types": ["绊线人数统计", "人数统计"],
        "endpoint": f"http://192.168.1.100:{SERVICE_PORT}/infer",  # 改为您的实际IP
        "version": "1.0.0"
    }
    
    try:
        response = requests.post(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/register",
            json=service_info,
            timeout=10
        )
        
        if response.status_code == 200:
            print(f"✅ 服务注册成功!")
            print(f"   Service ID: {SERVICE_ID}")
            print(f"   Endpoint: {service_info['endpoint']}")
            return True
        else:
            print(f"❌ 注册失败: {response.text}")
            return False
    except Exception as e:
        print(f"❌ 注册异常: {e}")
        return False

def heartbeat_loop():
    """心跳循环"""
    while True:
        try:
            time.sleep(45)  # 每45秒
            response = requests.post(
                f"{EASYDARWIN_URL}/api/v1/ai_analysis/heartbeat/{SERVICE_ID}",
                timeout=5
            )
            if response.status_code == 200:
                print("💓 心跳正常")
            else:
                print(f"⚠️ 心跳失败: {response.status_code}")
        except Exception as e:
            print(f"❌ 心跳异常: {e}")

# ============= 主程序 =============

if __name__ == '__main__':
    print("="*60)
    print("EasyDarwin算法服务 - 绊线人数统计")
    print("="*60)
    
    # 注册服务
    if register_service():
        # 启动心跳线程
        heartbeat_thread = threading.Thread(target=heartbeat_loop, daemon=True)
        heartbeat_thread.start()
        
        # 启动Flask服务
        print(f"\n🚀 服务启动在端口 {SERVICE_PORT}")
        print(f"📡 等待推理请求...\n")
        app.run(host='0.0.0.0', port=SERVICE_PORT, debug=False)
    else:
        print("❌ 服务注册失败，请检查EasyDarwin是否运行")
```

---

## 调试指南

### 1. 测试注册

```bash
# 使用curl测试注册
curl -X POST http://10.1.6.230:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "test_service_001",
    "name": "测试服务",
    "task_types": ["绊线人数统计"],
    "endpoint": "http://192.168.1.100:8000/infer",
    "version": "1.0.0"
  }'

# 预期响应
# {"ok":true,"service_id":"test_service_001"}
```

### 2. 查看已注册服务

```bash
# 查询所有注册的服务
curl http://10.1.6.230:5066/api/v1/ai_analysis/services

# 响应示例
{
  "services": [
    {
      "service_id": "test_service_001",
      "name": "测试服务",
      "task_types": ["绊线人数统计"],
      "endpoint": "http://192.168.1.100:8000/infer",
      "version": "1.0.0",
      "register_at": 1698765432000,
      "last_heartbeat": 1698765480000
    }
  ],
  "total": 1
}
```

### 3. 模拟推理请求

```bash
# 测试您的推理接口
curl -X POST http://192.168.1.100:8000/infer \
  -H "Content-Type: application/json" \
  -d '{
    "image_url": "http://example.com/test.jpg",
    "task_id": "test_001",
    "task_type": "绊线人数统计",
    "algo_config": {
      "regions": [
        {
          "type": "line",
          "points": [[100, 300], [700, 300]],
          "properties": {"direction": "in"}
        }
      ],
      "algorithm_params": {
        "confidence_threshold": 0.7
      }
    }
  }'
```

### 4. 查看EasyDarwin日志

```bash
# 实时查看日志
tail -f logs/sugar.log

# 筛选推理相关日志
tail -f logs/sugar.log | grep "推理请求\|inference"

# 应该能看到：
# INFO 收到推理请求 
#   任务ID=公司入口统计 
#   配置文件URL=http://...
```

---

## 常见问题

### Q1: 服务注册失败？

**检查项**:
```
□ EasyDarwin是否正在运行
□ 网络是否连通（ping测试）
□ 端口是否正确（默认5066）
□ JSON格式是否正确
```

### Q2: 收不到推理请求？

**检查项**:
```
□ 服务是否注册成功
□ task_types是否匹配
□ 心跳是否正常
□ 是否有对应类型的任务在运行
```

### Q3: 配置文件URL无法访问？

**原因**:
```
- URL已过期（>1小时）
- MinIO服务不可访问
- 网络问题
```

**解决**:
```
✅ 使用请求中的algo_config字段
✅ 检查MinIO服务状态
✅ 验证网络连通性
```

### Q4: 图片下载失败？

**检查项**:
```
□ image_url是否有效
□ MinIO是否可访问
□ 网络超时设置是否合理
□ URL签名是否有效
```

---

## 性能优化建议

### 1. 图片下载优化

```python
# 使用连接池
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

session = requests.Session()
retry = Retry(total=3, backoff_factor=0.3)
adapter = HTTPAdapter(max_retries=retry, pool_maxsize=10)
session.mount('http://', adapter)

def download_image(url):
    response = session.get(url, timeout=10)
    # ...
```

### 2. 配置缓存

```python
# 缓存配置，避免重复下载
config_cache = {}

def get_config(task_id, algo_config, algo_config_url):
    # 优先使用请求中的配置
    if algo_config:
        return algo_config
    
    # 检查缓存
    if task_id in config_cache:
        return config_cache[task_id]
    
    # 下载并缓存
    if algo_config_url:
        config = download_config(algo_config_url)
        config_cache[task_id] = config
        return config
    
    return {}
```

### 3. 批处理推理

```python
# 如果算法支持批量推理，可以累积请求
batch_queue = []

def batch_infer():
    """批量推理"""
    if len(batch_queue) >= BATCH_SIZE:
        images = [item['image'] for item in batch_queue]
        results = model.predict(images)  # 批量推理
        # 分别返回结果...
```

---

## 部署检查清单

### 服务启动前

```
□ 算法模型已加载
□ 依赖库已安装
□ 配置文件已准备
□ 端口未被占用
□ 网络连接正常
```

### 注册前检查

```
□ EasyDarwin已启动
□ 服务端点URL正确
□ task_types列表正确
□ service_id唯一
```

### 运行中监控

```
□ 心跳正常发送
□ 推理请求正常接收
□ 响应时间在合理范围
□ 错误率在可接受范围
□ 日志正常输出
```

---

## API端点汇总

### EasyDarwin提供的API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/ai_analysis/register` | POST | 注册服务 |
| `/api/v1/ai_analysis/unregister/{id}` | DELETE | 注销服务 |
| `/api/v1/ai_analysis/heartbeat/{id}` | POST | 发送心跳 |
| `/api/v1/ai_analysis/services` | GET | 查询已注册服务 |

### 您需要实现的API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/infer` | POST | 推理接口（必须） |
| `/health` | GET | 健康检查（可选） |
| `/metrics` | GET | 性能指标（可选） |

---

## 测试流程

### 1. 单元测试

```python
def test_infer():
    """测试推理接口"""
    test_request = {
        "image_url": "http://test.com/test.jpg",
        "task_id": "test_001",
        "task_type": "绊线人数统计",
        "algo_config": {
            "regions": [
                {
                    "type": "line",
                    "points": [[100, 300], [700, 300]],
                    "properties": {"direction": "in"}
                }
            ]
        }
    }
    
    response = requests.post(
        "http://localhost:8000/infer",
        json=test_request
    )
    
    assert response.status_code == 200
    result = response.json()
    assert result['success'] == True
    print("✅ 测试通过")
```

### 2. 集成测试

```
步骤1: 启动您的算法服务
步骤2: 注册到EasyDarwin
步骤3: 在EasyDarwin创建任务
步骤4: 配置检测线
步骤5: 启动任务
步骤6: 查看是否收到推理请求
步骤7: 检查告警是否生成
```

---

## 快速启动模板

将以下代码保存为 `algorithm_service.py`:

```python
#!/usr/bin/env python3
"""
EasyDarwin算法服务启动模板
请根据您的实际情况修改配置和算法实现
"""

from flask import Flask, request, jsonify
import requests
import cv2
import numpy as np
import time
import uuid
import threading

# ========== 配置区域（请修改） ==========
EASYDARWIN_URL = "http://10.1.6.230:5066"  # EasyDarwin地址
SERVICE_HOST = "0.0.0.0"                    # 服务监听地址
SERVICE_PORT = 8000                         # 服务端口
SERVICE_NAME = "绊线人数统计服务"           # 服务名称
TASK_TYPES = ["绊线人数统计", "人数统计"]   # 支持的任务类型
# ========================================

SERVICE_ID = str(uuid.uuid4())
app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    """推理接口"""
    try:
        data = request.json
        image_url = data['image_url']
        task_type = data['task_type']
        algo_config = data.get('algo_config', {})
        
        # 下载图片
        image = download_image(image_url)
        if image is None:
            return jsonify({"success": False, "error": "图片下载失败"})
        
        # 执行推理
        start = time.time()
        result = run_algorithm(image, algo_config, task_type)
        infer_time = int((time.time() - start) * 1000)
        
        return jsonify({
            "success": True,
            "result": result,
            "confidence": result.get('avg_confidence', 0.0),
            "inference_time_ms": infer_time
        })
    except Exception as e:
        return jsonify({"success": False, "error": str(e)})

def download_image(url):
    """下载图片"""
    try:
        resp = requests.get(url, timeout=10)
        arr = np.frombuffer(resp.content, np.uint8)
        return cv2.imdecode(arr, cv2.IMREAD_COLOR)
    except:
        return None

def run_algorithm(image, config, task_type):
    """
    运行算法（TODO: 实现您的算法逻辑）
    """
    # 示例返回
    return {
        "total_count": 0,
        "detections": [],
        "avg_confidence": 0.0
    }

def register_service():
    """注册服务"""
    import socket
    local_ip = socket.gethostbyname(socket.gethostname())
    
    service_info = {
        "service_id": SERVICE_ID,
        "name": SERVICE_NAME,
        "task_types": TASK_TYPES,
        "endpoint": f"http://{local_ip}:{SERVICE_PORT}/infer",
        "version": "1.0.0"
    }
    
    try:
        resp = requests.post(
            f"{EASYDARWIN_URL}/api/v1/ai_analysis/register",
            json=service_info,
            timeout=10
        )
        if resp.status_code == 200:
            print(f"✅ 服务注册成功: {SERVICE_ID}")
            return True
    except Exception as e:
        print(f"❌ 注册失败: {e}")
    return False

def heartbeat_loop():
    """心跳循环"""
    while True:
        time.sleep(45)
        try:
            requests.post(
                f"{EASYDARWIN_URL}/api/v1/ai_analysis/heartbeat/{SERVICE_ID}",
                timeout=5
            )
        except:
            pass

if __name__ == '__main__':
    if register_service():
        threading.Thread(target=heartbeat_loop, daemon=True).start()
        print(f"🚀 服务启动: http://0.0.0.0:{SERVICE_PORT}")
        app.run(host=SERVICE_HOST, port=SERVICE_PORT)
```

**使用方法**:
```bash
# 1. 安装依赖
pip install flask requests opencv-python numpy

# 2. 修改配置（代码中的配置区域）

# 3. 运行服务
python algorithm_service.py

# 4. 查看输出
# ✅ 服务注册成功: xxx-xxx-xxx
# 🚀 服务启动: http://0.0.0.0:8000
```

---

## 附录

### A. 完整的推理请求示例

详见上文"推理接口"章节

### B. 配置文件完整示例

详见"配置文件格式"章节

### C. 相关文档

- `TRIPWIRE_COUNTING_ALGORITHM.md` - 绊线统计算法说明
- `LINE_DIRECTION_PERPENDICULAR_ARROWS.md` - 线条方向配置
- `INFERENCE_CONFIG_URL_FEATURE.md` - 配置URL功能

### D. 技术支持

- **项目地址**: https://github.com/EasyDarwin/EasyDarwin
- **问题反馈**: GitHub Issues
- **文档更新**: 本文档会持续更新

---

**祝您对接顺利！** 🎉

如有问题，请参考文档或联系技术支持。



