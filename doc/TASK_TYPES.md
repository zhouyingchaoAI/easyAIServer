# 抽帧任务类型分类功能

## 概述

为了支持后续的智能视频分析，抽帧插件现在支持任务类型分类功能。通过任务类型，可以将抽取的图片按照分析场景进行分类存储，便于智能分析服务识别和处理。

---

## 目录结构

### 新的目录结构

```
snapshots/
├── 人数统计/
│   ├── 客流分析1/
│   │   ├── 20250115-150001.123.jpg
│   │   └── 20250115-150006.456.jpg
│   └── 商场入口/
│       └── 20250115-150010.789.jpg
├── 人员跌倒/
│   └── 电梯监控/
│       └── 20250115-150015.000.jpg
├── 吸烟检测/
│   └── 仓库监控/
│       └── 20250115-150020.111.jpg
└── 未分类/
    └── 临时任务1/
        └── 20250115-150025.222.jpg
```

**路径格式**：`{output_dir}/{任务类型}/{任务ID}/{时间戳}.jpg`

### 旧的目录结构（已废弃）

```
snapshots/
├── 客流分析1/
│   └── 20250115-150001.123.jpg
└── 电梯监控/
    └── 20250115-150015.000.jpg
```

**路径格式**：`{output_dir}/{任务ID}/{时间戳}.jpg`

---

## 配置

### config.toml

```toml
[frame_extractor]
enable = false
interval_ms = 1000
output_dir = './snapshots'
store = 'local'
# 任务类型列表，用于智能分析分类
task_types = ['人数统计', '人员跌倒', '人员离岗', '吸烟检测', '区域入侵', '徘徊检测', '物品遗留', '安全帽检测']

[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
base_path = ''

[[frame_extractor.tasks]]
id = '客流分析1'
task_type = '人数统计'  # 必填：任务类型
rtsp_url = 'rtsp://127.0.0.1:15544/live/stream_1'
interval_ms = 5000
output_path = '客流分析1'
enabled = true

[[frame_extractor.tasks]]
id = '电梯监控'
task_type = '人员跌倒'  # 必填：任务类型
rtsp_url = 'rtsp://127.0.0.1:15544/live/stream_2'
interval_ms = 3000
output_path = '电梯监控'
enabled = true
```

### 配置说明

#### task_types（任务类型列表）

**类型**：字符串数组  
**默认值**：8个预定义类型  
**作用**：定义可用的任务类型，供创建任务时选择

**预定义类型**：
1. 人数统计 - 用于客流分析、人数统计场景
2. 人员跌倒 - 用于跌倒检测、安全监控场景
3. 人员离岗 - 用于岗位监控、在岗检测场景
4. 吸烟检测 - 用于禁烟区域监控
5. 区域入侵 - 用于禁入区域监控
6. 徘徊检测 - 用于异常行为检测
7. 物品遗留 - 用于遗留物检测
8. 安全帽检测 - 用于施工安全监控

**自定义类型**：
```toml
task_types = ['人数统计', '人员跌倒', '自定义类型1', '自定义类型2']
```

#### task_type（任务类型）

**类型**：字符串  
**必填**：是  
**作用**：指定任务的分析类型，决定图片存储的父目录

**默认值规则**：
- 如果不填写，自动使用 `task_types` 列表的第一个类型
- 如果 `task_types` 为空，自动使用 "未分类"

---

## API

### 获取任务类型列表

**Endpoint**: `GET /api/v1/frame_extractor/task_types`

**返回**:
```json
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

### 创建任务（包含任务类型）

**Endpoint**: `POST /api/v1/frame_extractor/tasks`

**请求体**:
```json
{
  "id": "客流分析1",
  "task_type": "人数统计",
  "rtsp_url": "rtsp://localhost:15544/live/stream_1",
  "interval_ms": 5000,
  "output_path": "客流分析1"
}
```

**字段说明**:
- `id`: 任务唯一标识
- `task_type`: **必填**，任务类型（从 task_types 列表选择）
- `rtsp_url`: RTSP流地址
- `interval_ms`: 抽帧间隔（毫秒）
- `output_path`: 输出子目录名（可选，默认为任务ID）

---

## UI操作

### 创建任务

1. 访问抽帧管理页面：`http://localhost:5066/#/frame-extractor`
2. 填写表单：
   - **任务ID**：如 `客流分析1`
   - **任务类型**：从下拉框选择（如 `人数统计`）
   - **RTSP地址**：从直播流列表选择或手动输入
   - **间隔**：抽帧间隔（毫秒）
   - **输出路径**：可选，默认使用任务ID
3. 点击 "添加任务"

### 任务列表

任务列表显示：
- **任务ID**（蓝色标签）
- **任务类型**（紫色标签）
- **状态**（运行中/已停止）
- **RTSP地址**
- **间隔**
- **操作按钮**（启动/停止、查看快照、编辑、删除）

### 查看快照

快照按任务ID查看，但存储路径已包含任务类型：
- 本地存储：`./snapshots/人数统计/客流分析1/`
- MinIO存储：`images/人数统计/客流分析1/`

---

## 智能分析集成

### 路径识别规则

智能分析服务可以通过路径识别任务类型：

```python
# 示例：从MinIO路径提取任务类型
# 路径：人数统计/客流分析1/20250115-150001.123.jpg

import os

def get_task_info_from_path(object_path):
    parts = object_path.split('/')
    if len(parts) >= 2:
        task_type = parts[0]  # '人数统计'
        task_id = parts[1]    # '客流分析1'
        filename = parts[-1]  # '20250115-150001.123.jpg'
        
        return {
            'task_type': task_type,
            'task_id': task_id,
            'filename': filename
        }
```

### 推理模型映射

根据任务类型调用对应的AI模型：

```python
# 示例：任务类型到模型的映射
MODEL_MAP = {
    '人数统计': 'yolov8n.pt',
    '人员跌倒': 'fall_detection.pt',
    '人员离岗': 'absence_detection.pt',
    '吸烟检测': 'smoking_detection.pt',
    '区域入侵': 'intrusion_detection.pt',
    '徘徊检测': 'loitering_detection.pt',
    '物品遗留': 'abandoned_object.pt',
    '安全帽检测': 'helmet_detection.pt'
}

def infer_image(image_path, task_type):
    model_path = MODEL_MAP.get(task_type)
    if not model_path:
        return None
    
    # 加载模型并推理
    model = load_model(model_path)
    result = model.predict(image_path)
    return result
```

### MinIO事件监听

智能分析服务可以监听MinIO的事件通知：

```python
from minio import Minio
from minio.select import SelectRequest

# 监听MinIO新文件事件
client = Minio('10.1.6.230:9000',
               access_key='admin',
               secret_key='admin123',
               secure=False)

# 订阅bucket事件
events = client.listen_bucket_notification(
    'images',
    prefix='',
    suffix='.jpg',
    events=['s3:ObjectCreated:*']
)

for event in events:
    for record in event['Records']:
        object_key = record['s3']['object']['key']
        # 解析路径获取任务类型
        task_info = get_task_info_from_path(object_key)
        
        # 下载图片
        client.fget_object('images', object_key, '/tmp/image.jpg')
        
        # 执行推理
        result = infer_image('/tmp/image.jpg', task_info['task_type'])
        
        # 推送结果（WebSocket/HTTP回调/数据库）
        push_result(task_info, result)
```

---

## 使用示例

### 场景1：商场客流分析

```bash
# 1. 配置任务类型
# config.toml中已包含"人数统计"

# 2. 通过UI创建任务
- 任务ID: 商场入口
- 任务类型: 人数统计
- RTSP地址: rtsp://localhost:15544/live/stream_1
- 间隔: 5000ms

# 3. 启动任务后，图片保存到
./snapshots/人数统计/商场入口/20250115-150001.123.jpg
```

### 场景2：工地安全监控

```bash
# 1. 添加自定义类型到config.toml
task_types = [..., '安全帽检测', '高空作业监测']

# 2. 创建多个安全监控任务
- 任务1: 工地入口 → 安全帽检测
- 任务2: 脚手架区域 → 高空作业监测

# 3. 图片按类型分类存储
./snapshots/安全帽检测/工地入口/
./snapshots/高空作业监测/脚手架区域/
```

### 场景3：MinIO存储

```bash
# MinIO中的目录结构
images/
├── 人数统计/
│   └── 客流分析1/
│       └── 20250115-150001.123.jpg
└── 人员跌倒/
    └── 电梯监控/
        └── 20250115-150005.456.jpg

# 智能分析服务可以：
# 1. 监听images bucket的所有.jpg文件创建事件
# 2. 从路径提取任务类型
# 3. 调用对应的AI模型推理
# 4. 推送结果到前端/数据库
```

---

## 新增类型

### 通过配置文件

编辑 `configs/config.toml`：

```toml
[frame_extractor]
task_types = [
  '人数统计', 
  '人员跌倒', 
  '人员离岗', 
  '吸烟检测',
  '新增类型1',  # 新增
  '新增类型2'   # 新增
]
```

重启服务后生效。

### 验证任务类型

```bash
# 获取当前可用的任务类型
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

---

## 数据结构

### Go结构体

```go
type FrameExtractorConfig struct {
    Enable       bool     `json:"enable"`
    IntervalMs   int      `json:"interval_ms"`
    OutputDir    string   `json:"output_dir"`
    Store        string   `json:"store"`
    TaskTypes    []string `json:"task_types"`  // 新增
    MinIO        MinIOConfig
    Tasks        []FrameExtractTask
}

type FrameExtractTask struct {
    ID         string `json:"id"`
    TaskType   string `json:"task_type"`  // 新增
    RtspURL    string `json:"rtsp_url"`
    IntervalMs int    `json:"interval_ms"`
    OutputPath string `json:"output_path"`
    Enabled    bool   `json:"enabled"`
}
```

### 前端数据

```javascript
const form = {
  id: '客流分析1',
  task_type: '人数统计',  // 新增
  rtsp_url: 'rtsp://...',
  interval_ms: 5000,
  output_path: '客流分析1'
}
```

---

## 默认值处理

### 后端逻辑

```go
// AddTask 方法会自动处理默认值
if strings.TrimSpace(t.TaskType) == "" {
    if len(s.cfg.TaskTypes) > 0 {
        t.TaskType = s.cfg.TaskTypes[0]  // 使用第一个类型
    } else {
        t.TaskType = "未分类"  // 没有配置类型时使用"未分类"
    }
}
```

### 目录创建

所有保存路径都会包含任务类型：

```go
// 本地存储
taskType := task.TaskType
if taskType == "" {
    taskType = "未分类"
}
dir := filepath.Join(baseDir, taskType, task.OutputPath)

// MinIO存储
key := filepath.ToSlash(filepath.Join(basePath, taskType, task.OutputPath, filename))
```

---

## 前端展示

### 任务列表

| 任务ID | 任务类型 | 状态 | RTSP地址 | 间隔 | 操作 |
|--------|---------|------|----------|------|------|
| 客流分析1 | 人数统计 | 运行中 | rtsp://... | 5000ms | 启停/查看/编辑/删除 |
| 电梯监控 | 人员跌倒 | 已停止 | rtsp://... | 3000ms | 启停/查看/编辑/删除 |

### 任务类型标签

- 显示为紫色标签
- 未配置时显示 "未分类"

---

## 智能分析服务集成（未来）

### 服务架构

```
EasyDarwin 抽帧插件
    ↓ (保存图片到 MinIO/本地)
    ↓ 路径：任务类型/任务ID/图片
    ↓
智能分析服务（Python/Go）
    ↓ (监听新图片 + 路径识别)
    ↓ (根据任务类型加载对应AI模型)
    ↓ (执行推理)
    ↓
推理结果推送
    ↓ (WebSocket/HTTP回调/数据库)
    ↓
前端/第三方系统
```

### 推理结果格式（示例）

```json
{
  "task_id": "客流分析1",
  "task_type": "人数统计",
  "image_path": "人数统计/客流分析1/20250115-150001.123.jpg",
  "timestamp": "2025-01-15T15:00:01.123Z",
  "result": {
    "person_count": 23,
    "objects": [
      {"class": "person", "confidence": 0.95, "bbox": [100, 200, 150, 300]},
      {"class": "person", "confidence": 0.92, "bbox": [200, 220, 250, 320]}
    ]
  },
  "inference_time_ms": 45
}
```

---

## 迁移指南

### 从旧结构迁移

如果你有使用旧目录结构的任务：

```bash
# 旧结构
snapshots/
└── 客流分析1/

# 需要迁移为新结构
snapshots/
└── 人数统计/
    └── 客流分析1/
```

**迁移脚本**：

```bash
#!/bin/bash
# 迁移现有任务到新结构

cd snapshots

# 为每个任务创建任务类型目录
mkdir -p 人数统计
mv 客流分析1 人数统计/

mkdir -p 人员跌倒
mv 电梯监控 人员跌倒/

# 更新config.toml中的任务配置，添加task_type字段
```

---

## 故障排查

### 问题1：任务类型未显示

**检查**：
```bash
# 1. 确认config.toml中有task_types配置
cat configs/config.toml | grep task_types

# 2. 确认服务已重启
# 3. 确认API返回正确
curl http://localhost:5066/api/v1/frame_extractor/task_types | jq
```

### 问题2：图片保存路径不对

**检查日志**：
```bash
tail -f logs/sugar.log | grep "created minio path\|任务类型"

# 应该显示：
# created minio path task=客流分析1 type=人数统计 key=人数统计/客流分析1/.keep
```

### 问题3：旧任务无法查看快照

**原因**：旧任务没有 task_type 字段  
**解决**：
1. 停止旧任务
2. 编辑任务添加 task_type
3. 或手动迁移文件到新目录结构

---

## 总结

任务类型分类功能为智能视频分析提供了基础：

✅ **分类存储**：图片按任务类型分目录存储  
✅ **配置化**：任务类型可在config.toml中配置和扩展  
✅ **UI支持**：前端下拉框选择任务类型  
✅ **API接口**：获取任务类型列表  
✅ **路径规范**：统一的目录结构便于分析服务识别  
✅ **兼容性**：未分类任务自动归类到"未分类"目录  

为后续添加智能分析服务做好了准备！

