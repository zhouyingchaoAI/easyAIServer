# 算法服务返回格式规范

## 📝 概述

本文档详细说明算法服务应该返回的数据格式，以确保 yanying 平台能够正确识别检测结果并做出相应处理。

## ⚠️ 重要提示

**算法服务必须在返回结果中包含 `total_count` 字段！**

- ✅ `total_count > 0`：保存告警，保留图片
- ❌ `total_count = 0`：**删除图片**，不保存告警（当启用 `save_only_with_detection` 时）

## ✨ 推荐格式（最佳实践）

### 标准返回格式

```json
{
  "success": true,
  "result": {
    "total_count": 5,  // ← 检测到的目标总数（最高优先级，必需）
    "detections": [
      {
        "class_name": "person",
        "confidence": 0.95,
        "bbox": [100, 150, 200, 350]
      },
      {
        "class_name": "person",
        "confidence": 0.88,
        "bbox": [300, 150, 400, 350]
      }
      // ... 更多检测结果
    ],
    "image_size": [1920, 1080],
    "inference_time": 120
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```

**关键字段说明：**
- `success`: 必需，布尔值，表示推理是否成功
- `result.total_count`: **强烈推荐**，检测到的目标总数
- `result.detections`: 推荐，详细的检测结果数组
- `confidence`: 推荐，整体置信度
- `inference_time_ms`: 推荐，推理耗时（毫秒）

## 🎯 检测个数提取逻辑

yanying 平台按以下优先级提取检测个数：

### 优先级顺序

```
1. total_count   ← 最高优先级（推荐使用）
2. count
3. num
4. detections 数组长度
5. objects 数组长度
```

### 示例说明

#### ✅ 推荐：使用 total_count

```json
{
  "success": true,
  "result": {
    "total_count": 3,  // ← 系统优先读取此字段
    "detections": [...]
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```

**优势：**
- 明确表达检测总数
- 性能最优（无需计算数组长度）
- 支持复杂场景（如分组统计）

#### ✅ 无检测结果

```json
{
  "success": true,
  "result": {
    "total_count": 0,  // ← total_count = 0
    "detections": [],
    "message": "未检测到目标"
  },
  "confidence": 0.0,
  "inference_time_ms": 80
}
```

**处理逻辑：**
- `total_count = 0` → 图片被删除
- 不保存告警到数据库
- 不推送到 Kafka

## 📊 支持的所有格式

### 格式 1：total_count（推荐）⭐

```json
{
  "success": true,
  "result": {
    "total_count": 5
  }
}
```
→ `detection_count = 5`

### 格式 2：count

```json
{
  "success": true,
  "result": {
    "count": 5
  }
}
```
→ `detection_count = 5`

### 格式 3：num

```json
{
  "success": true,
  "result": {
    "num": 5
  }
}
```
→ `detection_count = 5`

### 格式 4：detections 数组

```json
{
  "success": true,
  "result": {
    "detections": [
      {"class_name": "person", "confidence": 0.95},
      {"class_name": "person", "confidence": 0.88},
      {"class_name": "person", "confidence": 0.92}
    ]
  }
}
```
→ `detection_count = 3` (数组长度)

### 格式 5：objects 数组

```json
{
  "success": true,
  "result": {
    "objects": [
      {"label": "helmet", "score": 0.92},
      {"label": "no_helmet", "score": 0.89}
    ]
  }
}
```
→ `detection_count = 2` (数组长度)

## 🔄 完整示例

### 人数统计算法

```json
{
  "success": true,
  "result": {
    "total_count": 8,
    "detections": [
      {
        "class_name": "person",
        "confidence": 0.95,
        "bbox": [100, 150, 200, 350],
        "track_id": 1
      },
      {
        "class_name": "person",
        "confidence": 0.92,
        "bbox": [300, 150, 400, 350],
        "track_id": 2
      }
      // ... 6 more persons
    ],
    "zones": {
      "zone_1": 3,
      "zone_2": 5
    },
    "alert": true,
    "alert_message": "检测到8人，超过阈值5人"
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```

### 安全帽检测算法

```json
{
  "success": true,
  "result": {
    "total_count": 5,
    "detections": [
      {
        "class_name": "helmet",
        "confidence": 0.96,
        "bbox": [100, 50, 180, 150]
      },
      {
        "class_name": "helmet",
        "confidence": 0.94,
        "bbox": [300, 60, 380, 160]
      },
      {
        "class_name": "no_helmet",
        "confidence": 0.91,
        "bbox": [500, 70, 580, 170]
      }
      // ... more
    ],
    "statistics": {
      "helmet": 3,
      "no_helmet": 2
    },
    "alert": true,
    "alert_message": "检测到2人未佩戴安全帽"
  },
  "confidence": 0.94,
  "inference_time_ms": 150
}
```

### 无检测结果

```json
{
  "success": true,
  "result": {
    "total_count": 0,
    "detections": [],
    "message": "画面中未检测到目标"
  },
  "confidence": 0.0,
  "inference_time_ms": 80
}
```

**处理逻辑：**
```
total_count = 0 
  ↓
删除图片 🗑️
  ↓
不保存告警 ❌
  ↓
不推送消息 ❌
```

## 🎨 Python 算法服务示例

### 使用 total_count（推荐）

```python
@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    image_url = data['image_url']
    
    # 下载图片
    response = requests.get(image_url)
    image = cv2.imdecode(np.frombuffer(response.content, np.uint8), cv2.IMREAD_COLOR)
    
    # 运行检测模型
    results = model(image)
    detections = results.pandas().xyxy[0].to_dict('records')
    
    # 计算检测总数
    total_count = len(detections)
    
    # 返回结果
    return jsonify({
        'success': True,
        'result': {
            'total_count': total_count,  # ← 明确返回总数
            'detections': detections,
            'image_size': image.shape[:2]
        },
        'confidence': max([d['confidence'] for d in detections]) if detections else 0.0,
        'inference_time_ms': int((time.time() - start_time) * 1000)
    })
```

### 无检测结果处理

```python
@app.route('/infer', methods=['POST'])
def infer():
    # ... 推理逻辑
    
    if len(detections) == 0:
        # 明确返回 total_count = 0
        return jsonify({
            'success': True,
            'result': {
                'total_count': 0,  # ← 重要：明确标记无检测
                'detections': [],
                'message': '未检测到目标'
            },
            'confidence': 0.0,
            'inference_time_ms': inference_time
        })
    
    # 有检测结果
    return jsonify({
        'success': True,
        'result': {
            'total_count': len(detections),
            'detections': detections
        },
        'confidence': max_confidence,
        'inference_time_ms': inference_time
    })
```

## ⚠️ 注意事项

### 1. total_count vs detections 数组

**场景：** 当检测结果很多，但只需要统计总数时

```json
{
  "success": true,
  "result": {
    "total_count": 100,  // ← 总数
    "detections": [      // ← 可以只返回部分（如前10个）
      // ... 仅返回前10个检测结果
    ],
    "note": "总共检测到100个，仅返回前10个详情"
  }
}
```

**建议：** 始终确保 `total_count` 准确反映实际检测总数。

### 2. total_count = 0 的影响

当 `total_count = 0` 时（或其他字段都为0），系统会：

```
✅ 如果 save_only_with_detection = true：
   - 删除MinIO中的图片
   - 不保存告警到数据库
   - 不推送到Kafka

❌ 如果 save_only_with_detection = false：
   - 保留图片
   - 保存告警（detection_count = 0）
   - 推送到Kafka
```

### 3. 推理失败处理

```json
{
  "success": false,
  "error": "图片格式不支持",
  "result": null
}
```

**处理：** 不删除图片，记录错误日志。

## 🔍 调试技巧

### 查看提取的检测个数

```bash
# 查看日志中的检测个数
tail -f logs/sugar.log | grep "detection_count"

# 输出示例：
# INFO inference completed and saved 
#   algorithm=people_counter_v1 
#   detection_count=5  ← 提取的检测个数
```

### 测试算法返回格式

```bash
# 测试算法服务
curl -X POST http://localhost:8000/infer \
  -H "Content-Type: application/json" \
  -d '{
    "image_url": "http://example.com/test.jpg",
    "task_id": "test_1",
    "task_type": "人数统计"
  }' | jq .

# 检查返回的 total_count 字段
```

## 📋 检查清单

在部署算法服务前，请确认：

- [ ] 返回格式包含 `success` 字段
- [ ] 返回格式包含 `result.total_count` 字段（推荐）
- [ ] `total_count = 0` 时明确返回 0
- [ ] 有检测结果时 `total_count > 0`
- [ ] 返回格式为有效的 JSON
- [ ] 测试了无检测结果的场景
- [ ] 测试了有检测结果的场景

## 📖 相关文档

- [只保存有检测结果功能](FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [检测个数统计功能](FEATURE_UPDATE_DETECTION_COUNT.md)
- [算法服务示例](../examples/algorithm_service.py)

---

**版本**：v1.2.1  
**更新日期**：2024-10-17  
**作者**：yanying team

