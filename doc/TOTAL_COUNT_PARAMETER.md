# total_count 参数说明

## 📝 重要说明

**算法服务必须在返回结果中包含 `total_count` 参数，表示检测到的目标总数。**

## ⭐ 关键规则

### 当 `total_count = 0` 时

系统会执行以下操作：

```
total_count = 0
    ↓
❌ 不保存告警到数据库
❌ 不推送消息到 Kafka
🗑️ 删除 MinIO 中的原始图片
```

### 当 `total_count > 0` 时

```
total_count > 0
    ↓
✅ 保存告警到数据库
✅ 推送消息到 Kafka
✅ 保留 MinIO 中的图片
```

## 📊 标准返回格式

### ✅ 正确格式

```json
{
  "success": true,
  "result": {
    "total_count": 5,  // ← 必需：检测到的目标总数
    "detections": [
      // ... 检测详情
    ]
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```

### ✅ 无检测结果

```json
{
  "success": true,
  "result": {
    "total_count": 0,  // ← 重要：明确返回 0
    "detections": [],
    "message": "未检测到目标"
  },
  "confidence": 0.0,
  "inference_time_ms": 80
}
```

**结果：** 图片被删除，不保存告警。

### ❌ 错误示例

```json
{
  "success": true,
  "result": {
    // ❌ 缺少 total_count
    "detections": [],
    "message": "无检测结果"
  }
}
```

**问题：** 系统会尝试从 `detections` 数组长度提取，虽然也能工作，但不如明确返回 `total_count`。

## 🎯 字段优先级

系统按以下顺序提取检测个数：

```
1. result.total_count    ← 最高优先级（推荐）
2. result.count
3. result.num
4. result.detections.length  ← 数组长度
5. result.objects.length     ← 数组长度
```

## 💡 最佳实践

### Python 示例

```python
import cv2
import numpy as np
from ultralytics import YOLO

@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    image_url = data['image_url']
    
    # 1. 下载图片
    response = requests.get(image_url)
    img_array = np.frombuffer(response.content, np.uint8)
    image = cv2.imdecode(img_array, cv2.IMREAD_COLOR)
    
    # 2. 执行推理
    model = YOLO('yolov8n.pt')
    results = model(image)
    
    # 3. 解析结果
    detections = []
    for result in results:
        boxes = result.boxes
        for box in boxes:
            detections.append({
                'class_name': result.names[int(box.cls)],
                'confidence': float(box.conf),
                'bbox': box.xyxy[0].tolist()
            })
    
    # 4. 计算总数
    total_count = len(detections)
    
    # 5. 返回结果
    return jsonify({
        'success': True,
        'result': {
            'total_count': total_count,  # ← 关键：明确返回总数
            'detections': detections,
            'message': f'检测到{total_count}个目标' if total_count > 0 else '未检测到目标'
        },
        'confidence': max([d['confidence'] for d in detections]) if detections else 0.0,
        'inference_time_ms': int(inference_time * 1000)
    })
```

### 关键代码

```python
# ✅ 推荐：始终返回 total_count
total_count = len(detections)

return jsonify({
    'success': True,
    'result': {
        'total_count': total_count,  # ← 必需
        'detections': detections
    }
})

# ❌ 不推荐：依赖系统自动计算
return jsonify({
    'success': True,
    'result': {
        'detections': detections  # 系统需要计算数组长度
    }
})
```

## 🔍 验证方法

### 测试算法返回格式

```bash
# 调用算法服务
curl -X POST http://localhost:8000/infer \
  -H "Content-Type: application/json" \
  -d '{
    "image_url": "http://example.com/test.jpg",
    "task_id": "test_1",
    "task_type": "人数统计"
  }' | jq .

# 检查输出
{
  "success": true,
  "result": {
    "total_count": 3,  # ← 确认有此字段
    "detections": [...]
  }
}
```

### 查看系统日志

```bash
# 查看提取的检测个数
tail -f logs/sugar.log | grep "detection_count"

# 输出示例：
# INFO inference completed and saved 
#   detection_count=5  ← 确认正确提取
```

## ⚠️ 重要警告

### total_count = 0 会删除图片

**当启用 `save_only_with_detection = true` 时：**

```json
{
  "result": {
    "total_count": 0  // ← 图片将被删除！
  }
}
```

**确保：**
1. 只在真正没有检测结果时返回 0
2. 不要因为算法错误返回 0
3. 推理失败时应返回 `success: false`

### 推理失败 vs 无检测结果

```json
// ✅ 推理成功，但无检测结果
{
  "success": true,
  "result": {
    "total_count": 0,  // 图片会被删除
    "message": "未检测到目标"
  }
}

// ✅ 推理失败
{
  "success": false,  // 图片不会被删除
  "error": "图片格式不支持"
}
```

## 📖 相关文档

- [算法返回格式规范](ALGORITHM_RESPONSE_FORMAT.md)
- [只保存有检测结果功能](FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [检测个数统计功能](FEATURE_UPDATE_DETECTION_COUNT.md)

---

**总结：** 算法服务应始终在 `result` 中返回 `total_count` 字段，明确标识检测到的目标总数。当 `total_count = 0` 时，原始图片将被删除（如果启用了 `save_only_with_detection`）。

**版本**：v1.2.1  
**更新日期**：2024-10-17

