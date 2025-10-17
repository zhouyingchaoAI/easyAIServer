# 功能更新：检测实例个数统计

## 📝 更新概览

本次更新为智能告警系统增加了**检测实例个数**功能，可以统计和过滤每次推理检测到的目标数量。

## ✨ 新增功能

### 1. 检测个数字段

- **数据库字段**：`detection_count` - 记录每次推理检测到的实例个数
- **自动提取**：从算法服务返回的推理结果中自动提取检测个数
- **支持格式**：
  - `detections` 数组 - 最常见的格式
  - `objects` 数组
  - `count` 数值
  - `num` 数值

### 2. 告警列表增强

#### 新增列
- **检测数列**：直观显示每条告警检测到的实例个数
- **徽章样式**：
  - 有检测：绿色徽章
  - 无检测：灰色徽章

#### 新增过滤器
- **最少检测数**：过滤检测个数 >= 指定值的记录
- **最多检测数**：过滤检测个数 <= 指定值的记录
- **重置按钮**：一键清除所有过滤条件

### 3. 告警详情增强

在告警详情弹窗中显示检测个数，方便查看具体信息。

### 4. 算法服务界面优化

- **响应式布局**：自适应不同屏幕尺寸
- **横向滚动**：表格超出宽度时支持横向滚动
- **文本省略**：长文本自动省略并显示 Tooltip
- **任务类型换行**：多个任务类型标签自动换行显示
- **最大高度限制**：任务类型列表超过一定高度时显示滚动条

## 🎯 使用场景

### 场景 1：人数统计

```
筛选条件：
- 任务类型：人数统计
- 最少检测数：5
- 最多检测数：20

结果：找出检测到 5-20 人的所有告警记录
```

### 场景 2：异常监测

```
筛选条件：
- 任务类型：人员跌倒
- 最少检测数：1

结果：找出所有检测到跌倒的告警（至少1人）
```

### 场景 3：空场景过滤

```
筛选条件：
- 最少检测数：1

结果：排除所有未检测到目标的记录
```

## 📊 算法服务返回格式

推理结果支持多种格式，系统会自动提取检测个数：

### 格式 1：detections 数组（推荐）

```json
{
  "success": true,
  "result": {
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
    ]
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```
→ detection_count = 2

### 格式 2：objects 数组

```json
{
  "success": true,
  "result": {
    "objects": [
      {"label": "helmet", "score": 0.92},
      {"label": "helmet", "score": 0.87},
      {"label": "no_helmet", "score": 0.89}
    ]
  },
  "confidence": 0.92
}
```
→ detection_count = 3

### 格式 3：count 字段

```json
{
  "success": true,
  "result": {
    "count": 15,
    "message": "检测到15人"
  },
  "confidence": 0.90
}
```
→ detection_count = 15

### 格式 4：num 字段

```json
{
  "success": true,
  "result": {
    "num": 8,
    "description": "8辆车"
  },
  "confidence": 0.93
}
```
→ detection_count = 8

## 🔧 API 更新

### 查询告警接口

**请求：**
```bash
GET /api/v1/ai_analysis/alerts?min_detections=5&max_detections=20&page=1&page_size=20
```

**参数：**
- `min_detections` (可选): 最少检测个数
- `max_detections` (可选): 最多检测个数
- `task_type` (可选): 任务类型
- `task_id` (可选): 任务ID
- `page`: 页码
- `page_size`: 每页数量

**响应：**
```json
{
  "ok": true,
  "data": {
    "items": [
      {
        "id": 1,
        "task_id": "task_1",
        "task_type": "人数统计",
        "detection_count": 12,
        "confidence": 0.95,
        "created_at": "2024-10-17T10:30:00Z",
        ...
      }
    ],
    "total": 100
  }
}
```

## 📦 数据库变更

### 新增字段

```sql
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
```

详见：[数据库迁移说明](DATABASE_MIGRATION.md)

## 🎨 前端界面预览

### 告警列表
```
┌─────┬──────────┬─────────┬──────┬────────┬────────┐
│ ID  │ 任务类型  │ 任务ID  │ 检测数│ 置信度 │ 操作   │
├─────┼──────────┼─────────┼──────┼────────┼────────┤
│ 123 │ 人数统计  │ task_1  │  🏷12 │ ████95%│ 查看   │
│ 124 │ 人员跌倒  │ task_2  │  🏷1  │ ████92%│ 查看   │
│ 125 │ 吸烟检测  │ task_3  │  🏷3  │ ███85% │ 查看   │
└─────┴──────────┴─────────┴──────┴────────┴────────┘
```

### 筛选器
```
┌─────────────────────────────────────────────────────────┐
│ [任务类型▼] [任务ID🔍] [最少检测数] [最多检测数] [查询] [重置] │
└─────────────────────────────────────────────────────────┘
```

## 🚀 升级步骤

### 1. 备份数据库
```bash
cp configs/data.db configs/data.db.backup
```

### 2. 更新数据库结构
```bash
sqlite3 configs/data.db
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
.exit
```

### 3. 重新编译并启动
```bash
# 重新编译
make build/linux

# 启动服务
cd build/EasyDarwin-lin-*
./easydarwin
```

### 4. 验证功能
```bash
# 访问Web界面
http://localhost:5066/#/alerts

# 或通过API验证
curl http://localhost:5066/api/v1/ai_analysis/alerts?page=1
```

## 💡 最佳实践

### 1. 算法服务开发建议

推荐使用 `detections` 数组格式返回结果：

```python
@app.route('/infer', methods=['POST'])
def infer():
    # ... 推理逻辑
    
    return jsonify({
        'success': True,
        'result': {
            'detections': detections,  # 检测结果数组
            'image_size': [width, height],
            'message': f'检测到{len(detections)}个目标'
        },
        'confidence': max_confidence,
        'inference_time_ms': inference_time
    })
```

### 2. 性能优化

对于高频查询，建议使用检测个数索引：

```sql
-- 已自动创建
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);

-- 查询示例（会使用索引）
SELECT * FROM alerts WHERE detection_count >= 5 AND detection_count <= 20;
```

### 3. 数据分析

统计检测个数分布：

```sql
SELECT 
    task_type,
    AVG(detection_count) as avg_count,
    MIN(detection_count) as min_count,
    MAX(detection_count) as max_count
FROM alerts
WHERE created_at >= datetime('now', '-7 days')
GROUP BY task_type;
```

## 🐛 已知问题

无

## 📞 技术支持

如有问题，请查阅：
- [完整文档](README_CN.md)
- [API文档](EasyDarwin.api.html)
- [故障排查](TROUBLESHOOTING_FRAME_EXTRACTOR.md)

---

**版本**：v1.1.0  
**更新日期**：2024-10-17  
**作者**：yanying team

