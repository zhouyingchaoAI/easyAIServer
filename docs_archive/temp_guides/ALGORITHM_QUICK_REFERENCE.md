# 算法服务开发快速参考

## 🎯 一分钟上手

### 返回格式模板（复制使用）

```json
{
  "success": true,
  "result": {
    "total_count": 检测到的目标总数,
    "detections": [详细检测结果],
    "message": "检测描述"
  },
  "confidence": 最高置信度,
  "inference_time_ms": 推理耗时毫秒数
}
```

## ⚠️ 关键规则

### total_count = 0 会删除图片！

```json
{
  "result": {
    "total_count": 0  // ← 图片将被从MinIO删除！
  }
}
```

### total_count > 0 会保存告警

```json
{
  "result": {
    "total_count": 5  // ← 告警被保存到数据库
  }
}
```

## 📋 Python 代码模板

### 最简模板

```python
@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    
    # TODO: 你的推理逻辑
    detections = []  # 你的检测结果
    
    # 返回标准格式
    return jsonify({
        'success': True,
        'result': {
            'total_count': len(detections),  # ← 必需
            'detections': detections
        },
        'confidence': 0.95,
        'inference_time_ms': 120
    })
```

### 完整模板（推荐）

```python
import requests
import cv2
import numpy as np
from flask import Flask, request, jsonify
import time

app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    start_time = time.time()
    
    try:
        # 1. 解析请求
        data = request.json
        image_url = data['image_url']
        task_id = data.get('task_id', '')
        task_type = data.get('task_type', '')
        
        # 2. 下载图片
        response = requests.get(image_url, timeout=10)
        img_array = np.frombuffer(response.content, np.uint8)
        image = cv2.imdecode(img_array, cv2.IMREAD_COLOR)
        
        # 3. 执行推理
        # TODO: 替换为你的模型
        # results = your_model.predict(image)
        detections = []  # 你的检测结果
        
        # 4. 计算总数
        total_count = len(detections)
        
        # 5. 计算推理时间
        inference_time = int((time.time() - start_time) * 1000)
        
        # 6. 返回结果
        return jsonify({
            'success': True,
            'result': {
                'total_count': total_count,  # ← 必需
                'detections': detections,
                'message': f'检测到{total_count}个目标' if total_count > 0 else '未检测到目标'
            },
            'confidence': max([d['confidence'] for d in detections]) if detections else 0.0,
            'inference_time_ms': inference_time
        })
        
    except Exception as e:
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)
```

## 🎨 不同任务类型示例

### 人数统计

```python
# 检测到人
return jsonify({
    'success': True,
    'result': {
        'total_count': 8,
        'detections': [
            {'class': 'person', 'confidence': 0.95, 'bbox': [...]},
            {'class': 'person', 'confidence': 0.92, 'bbox': [...]},
            # ... 8 persons
        ]
    }
})

# 无人
return jsonify({
    'success': True,
    'result': {
        'total_count': 0,  # ← 图片将被删除
        'detections': [],
        'message': '画面中无人'
    }
})
```

### 安全帽检测

```python
# 检测到未佩戴
return jsonify({
    'success': True,
    'result': {
        'total_count': 2,  # 2人未佩戴
        'detections': [
            {'class': 'no_helmet', 'confidence': 0.91, 'bbox': [...]},
            {'class': 'no_helmet', 'confidence': 0.88, 'bbox': [...]}
        ],
        'statistics': {
            'helmet': 5,
            'no_helmet': 2
        },
        'alert': True,
        'alert_message': '检测到2人未佩戴安全帽'
    }
})

# 全部佩戴（无违规）
return jsonify({
    'success': True,
    'result': {
        'total_count': 0,  # ← 无违规，图片删除
        'statistics': {
            'helmet': 7,
            'no_helmet': 0
        },
        'message': '全员已佩戴安全帽'
    }
})
```

### 车辆检测

```python
# 检测到车辆
return jsonify({
    'success': True,
    'result': {
        'total_count': 3,
        'vehicles': [
            {'type': 'car', 'plate': '京A12345'},
            {'type': 'truck', 'plate': '京B67890'},
            {'type': 'bus', 'plate': '京C11111'}
        ]
    }
})

# 无车辆
return jsonify({
    'success': True,
    'result': {
        'total_count': 0,  # ← 空旷道路，删除图片
        'message': '未检测到车辆'
    }
})
```

## 🔍 验证测试

### 测试脚本

```python
import requests
import json

def test_algorithm_service():
    url = 'http://localhost:8000/infer'
    
    # 测试请求
    payload = {
        'image_url': 'http://example.com/test.jpg',
        'task_id': 'test_1',
        'task_type': '人数统计'
    }
    
    response = requests.post(url, json=payload)
    result = response.json()
    
    # 验证格式
    assert 'success' in result, "缺少 success 字段"
    assert 'result' in result, "缺少 result 字段"
    assert 'total_count' in result['result'], "缺少 total_count 字段！"
    
    print(f"✅ total_count = {result['result']['total_count']}")
    print("✅ 格式验证通过")

if __name__ == '__main__':
    test_algorithm_service()
```

### 手动测试

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

## 📊 字段对照表

| 字段名 | 优先级 | 类型 | 说明 |
|--------|--------|------|------|
| `total_count` | ⭐⭐⭐⭐⭐ | 数字 | 检测总数（强烈推荐） |
| `count` | ⭐⭐⭐⭐ | 数字 | 备选字段 |
| `num` | ⭐⭐⭐ | 数字 | 备选字段 |
| `detections` | ⭐⭐ | 数组 | 通过长度计算 |
| `objects` | ⭐ | 数组 | 通过长度计算 |

## 🐛 常见错误

### 错误 1：忘记返回 total_count

```python
# ❌ 错误
return jsonify({
    'success': True,
    'result': {
        'detections': detections  # 缺少 total_count
    }
})

# ✅ 正确
return jsonify({
    'success': True,
    'result': {
        'total_count': len(detections),  # ← 添加此行
        'detections': detections
    }
})
```

### 错误 2：total_count 与实际不符

```python
# ❌ 错误：total_count 与 detections 数量不一致
return jsonify({
    'result': {
        'total_count': 5,
        'detections': [...]  # 实际只有3个
    }
})

# ✅ 正确：保持一致
detections = [...]
return jsonify({
    'result': {
        'total_count': len(detections),  # 自动计算
        'detections': detections
    }
})
```

### 错误 3：推理失败返回 total_count = 0

```python
# ❌ 错误：推理失败时不应返回 total_count = 0
return jsonify({
    'success': True,  # 应该是 False
    'result': {
        'total_count': 0  # 会导致图片被删除！
    }
})

# ✅ 正确：推理失败使用 success = false
return jsonify({
    'success': False,
    'error': '图片格式不支持'
}), 500
```

## 📖 完整文档

- [total_count 参数详细说明](TOTAL_COUNT_PARAMETER.md)
- [算法返回格式规范](ALGORITHM_RESPONSE_FORMAT.md)
- [只保存有检测结果功能](FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [示例代码](../examples/algorithm_service.py)

---

## 💡 记住

**三个关键点：**
1. 必须返回 `total_count` 字段
2. `total_count = 0` 会删除图片
3. 推理失败用 `success: false`，不要返回 `total_count = 0`

**版本**：v1.2.1  
**更新日期**：2024-10-17

