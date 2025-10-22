# 算法服务对接 - 快速参考

## 🚀 3步完成对接

```
第1步: 实现推理接口
第2步: 注册服务
第3步: 发送心跳
```

---

## 📋 API速查表

### 1. 服务注册

```http
POST http://10.1.6.230:5066/api/v1/ai_analysis/register
Content-Type: application/json

{
  "service_id": "your_service_id",
  "name": "绊线人数统计服务",
  "task_types": ["绊线人数统计"],
  "endpoint": "http://192.168.1.100:8000/infer",
  "version": "1.0.0"
}
```

### 2. 推理接口（您需要实现）

```http
POST http://192.168.1.100:8000/infer
Content-Type: application/json

请求 ↓
```

### 3. 心跳保活

```http
POST http://10.1.6.230:5066/api/v1/ai_analysis/heartbeat/{service_id}

每45秒发送一次
```

---

## 📥 推理请求格式

```json
{
  "image_url": "http://10.1.6.230:9000/images/...",
  "task_id": "公司入口统计",
  "task_type": "绊线人数统计",
  "image_path": "绊线人数统计/公司入口统计/xxx.jpg",
  "algo_config": {
    "regions": [
      {
        "type": "line",
        "points": [[100, 300], [700, 300]],
        "properties": {
          "direction": "in"  // "in"|"out"|"in_out"
        }
      }
    ],
    "algorithm_params": {
      "confidence_threshold": 0.7,
      "iou_threshold": 0.5
    }
  },
  "algo_config_url": "http://10.1.6.230:9000/images/.../algo_config.json"
}
```

**关键字段**:
- `image_url` - 图片预签名URL，直接下载
- `algo_config` - 配置内容（推荐使用）
- `algo_config_url` - 配置文件URL（备用）

---

## 📤 推理响应格式

```json
{
  "success": true,
  "result": {
    "total_count": 2,
    "detections": [...],
    "crossings": [...]
  },
  "confidence": 0.915,
  "inference_time_ms": 85
}
```

**必填字段**:
- `success` - 是否成功（boolean）
- `result` - 推理结果（object/null）
- `confidence` - 置信度（float, 0.0-1.0）
- `inference_time_ms` - 推理耗时（int, 毫秒）

---

## 💻 Python最小实现

```python
from flask import Flask, request, jsonify
import requests
import cv2
import numpy as np

app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    
    # 下载图片
    resp = requests.get(data['image_url'])
    arr = np.frombuffer(resp.content, np.uint8)
    image = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    
    # TODO: 运行您的算法
    result = your_algorithm(image, data.get('algo_config', {}))
    
    return jsonify({
        "success": True,
        "result": result,
        "confidence": 0.9,
        "inference_time_ms": 100
    })

if __name__ == '__main__':
    # 1. 先注册服务（见完整示例）
    # 2. 启动Flask
    app.run(host='0.0.0.0', port=8000)
```

---

## 📐 配置解析

### 检测线配置

```python
def parse_line_config(algo_config):
    """解析检测线配置"""
    regions = algo_config.get('regions', [])
    lines = [r for r in regions if r['type'] == 'line']
    
    for line in lines:
        name = line.get('name', 'unknown')
        points = line['points']  # [[x1, y1], [x2, y2]]
        direction = line['properties']['direction']
        
        print(f"检测线: {name}")
        print(f"  起点: {points[0]}")
        print(f"  终点: {points[1]}")
        print(f"  方向: {direction}")
        
        # 使用配置进行检测...
```

### 方向说明

```python
direction_map = {
    "in": "进入（从上方穿过到下方）",
    "out": "离开（从下方穿过到上方）",
    "in_out": "双向（任意方向穿过）"
}
```

---

## 🔍 调试技巧

### 1. 测试注册

```bash
curl -X POST http://10.1.6.230:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{"service_id":"test","name":"测试","task_types":["绊线人数统计"],"endpoint":"http://192.168.1.100:8000/infer","version":"1.0"}'
```

### 2. 查看服务列表

```bash
curl http://10.1.6.230:5066/api/v1/ai_analysis/services | jq
```

### 3. 测试推理接口

```python
# 模拟EasyDarwin发送的请求
test_data = {
    "image_url": "http://example.com/test.jpg",
    "task_id": "test_001",
    "task_type": "绊线人数统计",
    "algo_config": {...}
}

response = requests.post("http://localhost:8000/infer", json=test_data)
print(response.json())
```

### 4. 查看EasyDarwin日志

```bash
# 实时查看
tail -f logs/sugar.log | grep "推理请求"

# 应该看到：
# INFO 收到推理请求 任务ID=xxx 配置文件URL=http://...
```

---

## ⚠️ 常见错误

### 错误1: 服务注册失败

```
原因: EasyDarwin未运行或地址错误
解决: ping测试、检查端口
```

### 错误2: 收不到推理请求

```
原因: task_types不匹配
解决: 检查注册的task_types是否包含任务的类型
```

### 错误3: 图片下载失败

```
原因: URL过期或MinIO不可访问
解决: 检查网络、验证URL有效性
```

### 错误4: 配置文件访问失败

```
原因: algo_config_url过期
解决: 使用algo_config字段（推荐）
```

---

## 📊 支持的任务类型

```
✅ 人数统计
✅ 绊线人数统计 ⭐
✅ 人员跌倒
✅ 人员离岗
✅ 吸烟检测
✅ 区域入侵
✅ 徘徊检测
✅ 物品遗留
✅ 安全帽检测
```

**您可以选择支持一种或多种**

---

## 📞 技术支持

### 详细文档

```
ALGORITHM_SERVICE_INTEGRATION_GUIDE.md - 完整对接指南
TRIPWIRE_COUNTING_ALGORITHM.md - 绊线统计算法说明
LINE_DIRECTION_PERPENDICULAR_ARROWS.md - 线条方向配置
```

### 快速测试

```bash
# 1. 下载启动模板
# 见上文"快速启动模板"

# 2. 修改配置
# EASYDARWIN_URL、SERVICE_PORT等

# 3. 运行
python algorithm_service.py

# 4. 验证
curl http://localhost:8000/infer -X POST -H "Content-Type: application/json" -d '{...}'
```

---

## ✅ 对接检查清单

```
□ 实现了 /infer 接口
□ 能下载MinIO图片
□ 能解析algo_config
□ 返回格式正确
□ 已注册到EasyDarwin
□ 心跳正常发送
□ task_types匹配
□ 能收到推理请求
□ 推理结果正确
□ 告警正常生成
```

---

**版本**: v1.0  
**适用**: 算法服务对接  
**完整文档**: ALGORITHM_SERVICE_INTEGRATION_GUIDE.md



