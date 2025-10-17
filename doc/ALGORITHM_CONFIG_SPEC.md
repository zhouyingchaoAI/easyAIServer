# 算法配置规范文档 (Algorithm Configuration Specification)

## 版本信息

- **版本**: v1.0
- **日期**: 2024-10-17
- **适用范围**: yanying智能视频分析平台

---

## 📋 概述

本文档定义了算法配置的标准JSON格式，用于在视频分析任务中配置检测区域、算法参数等信息。

### 设计目标

1. **通用性**：支持多种算法类型（人数统计、跌倒检测、越线检测等）
2. **灵活性**：支持多个区域、多种形状（线、矩形、多边形）
3. **扩展性**：预留自定义参数字段
4. **易用性**：结构清晰，易于理解和实现

---

## 📐 JSON标准结构

### 完整示例

```json
{
  "task_id": "cam_entrance_001",
  "task_type": "人数统计",
  "config_version": "1.0",
  "created_at": "2024-10-17T14:35:20Z",
  "updated_at": "2024-10-17T14:35:20Z",
  "regions": [
    {
      "id": "region_001",
      "name": "入口区域",
      "type": "polygon",
      "enabled": true,
      "points": [
        [100, 200],
        [300, 200],
        [300, 400],
        [100, 400]
      ],
      "properties": {
        "color": "#FF0000",
        "opacity": 0.3,
        "threshold": 0.5,
        "alert_type": "count"
      }
    },
    {
      "id": "region_002",
      "name": "越线检测",
      "type": "line",
      "enabled": true,
      "points": [
        [500, 100],
        [500, 600]
      ],
      "properties": {
        "color": "#00FF00",
        "opacity": 0.5,
        "direction": "bidirectional",
        "thickness": 5
      }
    },
    {
      "id": "region_003",
      "name": "禁止区域",
      "type": "rectangle",
      "enabled": true,
      "points": [
        [700, 150],
        [900, 350]
      ],
      "properties": {
        "color": "#0000FF",
        "opacity": 0.4,
        "alert_type": "intrusion"
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "iou_threshold": 0.5,
    "min_detection_size": 50,
    "max_detection_size": 500,
    "frame_skip": 0,
    "custom_params": {
      "track_enabled": true,
      "track_max_age": 30,
      "min_dwell_time": 3
    }
  }
}
```

---

## 📖 字段说明

### 根级字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `task_id` | string | ✅ | 任务唯一标识，与抽帧任务ID对应 |
| `task_type` | string | ✅ | 任务类型（人数统计、人员跌倒、吸烟检测等） |
| `config_version` | string | ✅ | 配置版本号，当前为 "1.0" |
| `created_at` | string | ✅ | 配置创建时间（ISO 8601格式） |
| `updated_at` | string | ✅ | 配置更新时间（ISO 8601格式） |
| `regions` | array | ✅ | 检测区域列表，可以为空数组 |
| `algorithm_params` | object | ❌ | 算法参数，可选 |

---

### regions 数组元素

每个region对象包含以下字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | ✅ | 区域唯一标识，格式：region_XXX |
| `name` | string | ✅ | 区域名称，便于识别 |
| `type` | string | ✅ | 区域类型：`"line"` / `"rectangle"` / `"polygon"` |
| `enabled` | boolean | ✅ | 是否启用该区域 |
| `points` | array | ✅ | 坐标点数组，格式见下方说明 |
| `properties` | object | ❌ | 区域属性，可选 |

#### points 格式说明

**线（line）**：
```json
"points": [[x1, y1], [x2, y2]]
```
- 两个点定义一条线段
- 用于越线检测、绊线检测等

**矩形（rectangle）**：
```json
"points": [[x1, y1], [x2, y2]]
```
- 第一个点：左上角坐标
- 第二个点：右下角坐标
- 用于区域入侵、区域计数等

**多边形（polygon）**：
```json
"points": [[x1, y1], [x2, y2], [x3, y3], ...]
```
- 多个点按顺序连接形成封闭多边形
- 至少3个点
- 用于不规则区域检测

**坐标系统**：
- 原点(0,0)在图像左上角
- x轴向右递增
- y轴向下递增
- 单位：像素

#### properties 对象

常用属性（可选）：

| 字段 | 类型 | 说明 | 示例值 |
|------|------|------|--------|
| `color` | string | 区域颜色（十六进制） | `"#FF0000"` |
| `opacity` | number | 透明度（0.0-1.0） | `0.3` |
| `threshold` | number | 检测阈值 | `0.5` |
| `direction` | string | 方向（仅line）：`"in"` / `"out"` / `"bidirectional"` | `"bidirectional"` |
| `thickness` | number | 线宽（仅line） | `5` |
| `alert_type` | string | 告警类型 | `"count"` / `"intrusion"` / `"cross"` |

---

### algorithm_params 对象

通用算法参数（可选）：

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `confidence_threshold` | number | 置信度阈值（0.0-1.0） | `0.7` |
| `iou_threshold` | number | IOU阈值（0.0-1.0） | `0.5` |
| `min_detection_size` | number | 最小检测尺寸（像素） | `50` |
| `max_detection_size` | number | 最大检测尺寸（像素） | `500` |
| `frame_skip` | number | 跳帧数（0表示不跳帧） | `0` |
| `custom_params` | object | 自定义参数对象 | `{}` |

**custom_params 示例**：
```json
{
  "track_enabled": true,
  "track_max_age": 30,
  "min_dwell_time": 3,
  "alert_interval": 5
}
```

---

## 🎯 不同任务类型的配置示例

### 1. 人数统计

```json
{
  "task_id": "cam_hall_001",
  "task_type": "人数统计",
  "config_version": "1.0",
  "created_at": "2024-10-17T10:00:00Z",
  "updated_at": "2024-10-17T10:00:00Z",
  "regions": [
    {
      "id": "region_001",
      "name": "大厅区域",
      "type": "polygon",
      "enabled": true,
      "points": [[100, 150], [500, 150], [500, 450], [100, 450]],
      "properties": {
        "color": "#FF6B6B",
        "opacity": 0.3,
        "alert_type": "count",
        "max_count": 50
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "min_detection_size": 80,
    "custom_params": {
      "count_mode": "current"
    }
  }
}
```

### 2. 越线检测

```json
{
  "task_id": "cam_gate_001",
  "task_type": "区域入侵",
  "config_version": "1.0",
  "created_at": "2024-10-17T10:00:00Z",
  "updated_at": "2024-10-17T10:00:00Z",
  "regions": [
    {
      "id": "region_001",
      "name": "入口检测线",
      "type": "line",
      "enabled": true,
      "points": [[300, 200], [600, 200]],
      "properties": {
        "color": "#4ECDC4",
        "thickness": 5,
        "direction": "in",
        "alert_on_cross": true
      }
    },
    {
      "id": "region_002",
      "name": "出口检测线",
      "type": "line",
      "enabled": true,
      "points": [[300, 400], [600, 400]],
      "properties": {
        "color": "#FFE66D",
        "thickness": 5,
        "direction": "out",
        "alert_on_cross": true
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.75,
    "custom_params": {
      "track_enabled": true,
      "cross_threshold": 0.5
    }
  }
}
```

### 3. 人员跌倒检测

```json
{
  "task_id": "cam_corridor_001",
  "task_type": "人员跌倒",
  "config_version": "1.0",
  "created_at": "2024-10-17T10:00:00Z",
  "updated_at": "2024-10-17T10:00:00Z",
  "regions": [
    {
      "id": "region_001",
      "name": "监控全区域",
      "type": "rectangle",
      "enabled": true,
      "points": [[0, 0], [1920, 1080]],
      "properties": {
        "color": "#FF4757",
        "opacity": 0.2,
        "alert_type": "fall"
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.8,
    "custom_params": {
      "fall_duration_threshold": 2,
      "aspect_ratio_threshold": 2.5
    }
  }
}
```

### 4. 安全帽检测

```json
{
  "task_id": "cam_site_001",
  "task_type": "安全帽检测",
  "config_version": "1.0",
  "created_at": "2024-10-17T10:00:00Z",
  "updated_at": "2024-10-17T10:00:00Z",
  "regions": [
    {
      "id": "region_001",
      "name": "作业区域",
      "type": "polygon",
      "enabled": true,
      "points": [[200, 100], [800, 100], [900, 500], [100, 500]],
      "properties": {
        "color": "#FFA502",
        "opacity": 0.3,
        "alert_type": "no_helmet"
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "custom_params": {
      "helmet_check_enabled": true,
      "alert_delay": 3
    }
  }
}
```

### 5. 吸烟检测

```json
{
  "task_id": "cam_office_001",
  "task_type": "吸烟检测",
  "config_version": "1.0",
  "created_at": "2024-10-17T10:00:00Z",
  "updated_at": "2024-10-17T10:00:00Z",
  "regions": [
    {
      "id": "region_001",
      "name": "禁烟区域1",
      "type": "rectangle",
      "enabled": true,
      "points": [[100, 100], [500, 400]],
      "properties": {
        "color": "#EA2027",
        "opacity": 0.3,
        "alert_type": "smoking"
      }
    },
    {
      "id": "region_002",
      "name": "禁烟区域2",
      "type": "polygon",
      "enabled": true,
      "points": [[600, 100], [900, 100], [900, 400], [600, 400]],
      "properties": {
        "color": "#EA2027",
        "opacity": 0.3,
        "alert_type": "smoking"
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.75,
    "custom_params": {
      "smoking_duration_threshold": 3
    }
  }
}
```

---

## 🔧 算法服务使用说明

### 接收配置

算法服务在推理请求中会收到 `algo_config` 字段：

```python
def infer(self, image_url, task_id, task_type, algo_config):
    """
    Args:
        image_url: 图片URL
        task_id: 任务ID
        task_type: 任务类型
        algo_config: 算法配置对象（dict）
    """
    # 1. 提取配置信息
    regions = algo_config.get('regions', [])
    params = algo_config.get('algorithm_params', {})
    
    # 2. 使用配置进行推理
    confidence_threshold = params.get('confidence_threshold', 0.7)
    
    # 3. 在指定区域内检测
    for region in regions:
        if not region['enabled']:
            continue
            
        region_type = region['type']
        points = region['points']
        
        if region_type == 'polygon':
            # 多边形区域检测
            results = self.detect_in_polygon(image, points, confidence_threshold)
        elif region_type == 'line':
            # 越线检测
            results = self.detect_line_crossing(image, points, ...)
        elif region_type == 'rectangle':
            # 矩形区域检测
            results = self.detect_in_rectangle(image, points, confidence_threshold)
```

### 区域检测辅助函数

```python
import cv2
import numpy as np

def point_in_polygon(point, polygon):
    """判断点是否在多边形内"""
    x, y = point
    poly = np.array(polygon, dtype=np.int32)
    return cv2.pointPolygonTest(poly, (x, y), False) >= 0

def point_in_rectangle(point, rect_points):
    """判断点是否在矩形内"""
    x, y = point
    x1, y1 = rect_points[0]
    x2, y2 = rect_points[1]
    return x1 <= x <= x2 and y1 <= y <= y2

def check_line_crossing(trajectory, line_points):
    """检测轨迹是否越过线"""
    # 实现越线检测逻辑
    pass
```

---

## 📤 推理结果格式

算法服务应该返回包含区域信息的结果：

```python
{
    "success": True,
    "result": {
        "total_count": 5,
        "detections": [
            {
                "class": "person",
                "confidence": 0.95,
                "bbox": [100, 200, 150, 300],
                "region_id": "region_001",  # 所属区域ID
                "region_name": "入口区域"    # 所属区域名称
            },
            # ... more detections
        ],
        "region_results": [
            {
                "region_id": "region_001",
                "region_name": "入口区域",
                "count": 3,
                "alert": False
            },
            {
                "region_id": "region_002",
                "region_name": "越线检测",
                "crossed": True,
                "direction": "in",
                "alert": True
            }
        ],
        "message": "检测到5个对象"
    },
    "confidence": 0.95,
    "inference_time_ms": 45
}
```

---

## ⚠️ 注意事项

### 1. 坐标系统
- 所有坐标基于图像原始分辨率
- 如果图像进行了缩放，算法需要自行处理坐标转换

### 2. 区域优先级
- 多个区域重叠时，按regions数组顺序处理
- 一个检测对象可以同时属于多个区域

### 3. 配置缓存
- 算法服务建议缓存配置，避免每次请求都解析
- 当配置更新时会在请求中体现

### 4. 错误处理
- 如果配置格式错误，应返回明确的错误信息
- 建议验证必填字段是否存在

### 5. 向后兼容
- 新版本可能增加新字段
- 算法服务应忽略未知字段，保持兼容性

---

## 📁 配置文件存储

### MinIO存储路径

```
frames/{task_type}/{task_id}/algo_config.json
```

示例：
```
frames/人数统计/cam_entrance_001/algo_config.json
```

### 访问方式

**保存配置**：
```
POST /frame_extractor/tasks/:task_id/config
Content-Type: application/json

{配置JSON}
```

**获取配置**：
```
GET /frame_extractor/tasks/:task_id/config
```

**推理时自动包含**：
```json
{
  "image_url": "...",
  "task_id": "cam_entrance_001",
  "task_type": "人数统计",
  "algo_config": {
    "regions": [...],
    "algorithm_params": {...}
  }
}
```

---

## 🔄 配置更新流程

1. **Web界面配置** → 绘制区域、设置参数
2. **保存到MinIO** → 生成algo_config.json文件
3. **推理请求携带** → AI分析插件读取配置并传递给算法服务
4. **算法服务使用** → 根据配置执行检测
5. **返回结果** → 包含区域相关信息

---

## 📞 技术支持

如有疑问或建议，请联系：
- 项目地址：https://github.com/zhouyingchaoAI/easyAIServer
- 文档版本：v1.0
- 更新日期：2024-10-17

---

**注意**：本规范持续更新中，请关注版本变化。

