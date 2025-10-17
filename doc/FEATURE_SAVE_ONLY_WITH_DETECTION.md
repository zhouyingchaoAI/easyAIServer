# 功能说明：只保存有检测结果的告警

## 📝 功能概述

新增智能过滤功能：**只保存和推送有检测结果的告警**，没有检测到目标的图片将被自动删除，避免存储空间浪费和无效告警。

## ✨ 主要特性

### 1. 智能检测结果过滤

- **自动判断**：根据算法返回的检测结果自动判断是否有目标
- **只保存有效告警**：只有检测到目标（检测个数 > 0）的告警才会被保存到数据库
- **删除无效图片**：检测个数为 0 的图片会被自动从 MinIO 删除
- **不推送空告警**：没有检测结果的不会推送到消息队列

### 2. 节省存储空间

- **减少冗余**：避免保存大量无检测结果的图片
- **降低成本**：显著减少 MinIO 存储空间占用
- **提高效率**：数据库中只保存有价值的告警记录

### 3. 可配置开关

- **灵活控制**：通过配置文件开启或关闭此功能
- **默认启用**：建议启用以节省资源
- **兼容性好**：关闭后恢复原有行为，保存所有告警

## 🎯 适用场景

### 场景 1：人数统计

**需求**：只关心有人的时候，空场景不需要记录

**配置：**
```toml
[ai_analysis]
save_only_with_detection = true
```

**效果：**
- ✅ 有人经过：保存告警，记录人数
- ❌ 无人场景：删除图片，不保存告警

### 场景 2：车辆检测

**需求**：只记录有车辆的画面

**效果：**
- ✅ 检测到车辆：保存记录
- ❌ 空旷道路：删除图片

### 场景 3：安全帽检测

**需求**：只记录有工人的场景

**效果：**
- ✅ 有工人：检测安全帽，保存告警
- ❌ 无人工地：删除图片

## 🔧 配置说明

### 配置文件

编辑 `configs/config.toml`：

```toml
[ai_analysis]
enable = true
scan_interval_sec = 5
mq_type = 'kafka'
mq_address = ''
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 20
save_only_with_detection = true  # ← 新增配置项
```

### 配置项说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `save_only_with_detection` | bool | false | 是否只保存有检测结果的告警 |

### 配置值

- **`true`** (推荐)：启用功能，只保存有检测结果的告警
- **`false`**：关闭功能，保存所有推理结果（包括无检测结果的）

## 📊 工作流程

### 启用功能时的流程

```
┌─────────────────────────────────────────────────────────┐
│ 1. Frame Extractor 抽取视频帧                            │
│    → 保存到 MinIO: /人数统计/task_1/frame_001.jpg        │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 2. AI Scanner 扫描 MinIO 发现新图片                      │
│    → 添加到推理队列                                      │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 3. Scheduler 调度算法服务推理                            │
│    → 调用算法服务 HTTP API                               │
│    → 返回推理结果                                        │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ 4. 提取检测个数 (detection_count)                        │
│    → 从 result.detections 数组长度获取                   │
└─────────────────────────────────────────────────────────┘
                          ↓
                    ┌──────────┐
                    │ 判断检测个数│
                    └──────────┘
                    ↙          ↘
        检测个数 = 0             检测个数 > 0
                ↓                      ↓
    ┌────────────────────┐    ┌────────────────────┐
    │ 5a. 删除图片        │    │ 5b. 保存告警        │
    │ - 从MinIO删除       │    │ - 保存到数据库      │
    │ - 不保存告警        │    │ - 推送到Kafka       │
    │ - 不推送消息        │    │ - 保留图片          │
    │ - 记录debug日志     │    │ - 记录info日志      │
    └────────────────────┘    └────────────────────┘
```

### 关闭功能时的流程

所有推理结果都会被保存，包括检测个数为 0 的。

## 💾 存储空间对比

### 示例场景：24小时监控

假设：
- 抽帧频率：每秒 1 帧
- 每张图片：300KB
- 有人时间：平均 10%

| 项目 | 关闭功能 | 启用功能 | 节省 |
|------|----------|----------|------|
| 总抽帧数 | 86,400 张 | 86,400 张 | - |
| 保存图片数 | 86,400 张 | 8,640 张 | 90% |
| 存储空间 | 25.9 GB | 2.6 GB | **23.3 GB** |
| 告警记录数 | 86,400 条 | 8,640 条 | 90% |

**结论**：在大多数监控场景下，可以节省 **70-90%** 的存储空间。

## 📋 日志示例

### 有检测结果

```log
INFO inference completed and saved 
  algorithm=people_counter_v1 
  task_id=task_1 
  detection_count=5 
  alert_id=12345 
  confidence=0.95
```

### 无检测结果

```log
DEBUG no detection result, deleting image 
  image=人数统计/task_1/frame_001.jpg 
  algorithm=people_counter_v1

DEBUG image deleted from MinIO 
  path=人数统计/task_1/frame_001.jpg
```

### 删除失败

```log
WARN failed to delete image with no detection 
  path=人数统计/task_1/frame_001.jpg 
  err=object not found
```

## 🎨 算法服务适配

### 确保返回正确的检测结果

算法服务需要确保返回的 `result` 包含检测结果数组：

#### 标准格式（推荐）⭐

```json
{
  "success": true,
  "result": {
    "total_count": 2,  // ← 检测总数（最高优先级，强烈推荐）
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
→ detection_count = 2 (从 total_count 获取)

#### 无检测结果

```json
{
  "success": true,
  "result": {
    "total_count": 0,  // ← 明确标记无检测（推荐）
    "detections": [],  // 空数组
    "message": "未检测到目标"
  },
  "confidence": 0.0,
  "inference_time_ms": 80
}
```

#### 其他支持的格式

**提取优先级：**
1. **total_count** ← 最高优先级（推荐使用）
2. count
3. num
4. detections 数组长度
5. objects 数组长度

```json
// 方式 1：total_count（推荐）⭐
{
  "success": true,
  "result": {
    "total_count": 0
  }
}

// 方式 2：count
{
  "success": true,
  "result": {
    "count": 0
  }
}

// 方式 3：num
{
  "success": true,
  "result": {
    "num": 0
  }
}

// 方式 4：detections 数组
{
  "success": true,
  "result": {
    "detections": []  // 空数组，长度 = 0
  }
}

// 方式 5：objects 数组
{
  "success": true,
  "result": {
    "objects": []  // 空数组，长度 = 0
  }
}
```

## ⚙️ 技术实现

### 代码逻辑

```go
// scheduler.go

// 提取检测个数
detectionCount := extractDetectionCount(resp.Result)

// 如果启用了只保存有检测结果的功能，且没有检测结果
if s.saveOnlyWithDetection && detectionCount == 0 {
    s.log.Debug("no detection result, deleting image",
        slog.String("image", image.Path),
        slog.String("algorithm", algorithm.ServiceID))
    
    // 删除MinIO中的图片
    if err := s.deleteImage(image.Path); err != nil {
        s.log.Warn("failed to delete image with no detection",
            slog.String("path", image.Path),
            slog.String("err", err.Error()))
    }
    
    return // 不保存告警，不推送消息
}

// 有检测结果，继续保存...
```

### 删除图片函数

```go
func (s *Scheduler) deleteImage(imagePath string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    err := s.minio.RemoveObject(ctx, s.bucket, imagePath, minio.RemoveObjectOptions{})
    if err != nil {
        return fmt.Errorf("remove object failed: %w", err)
    }
    
    s.log.Debug("image deleted from MinIO", slog.String("path", imagePath))
    return nil
}
```

## 🔍 监控与统计

### 查看删除图片的日志

```bash
# 查看删除的图片数量
tail -f logs/sugar.log | grep "no detection result"

# 统计今天删除的图片数
cat logs/sugar.log | grep "no detection result" | grep $(date +%Y-%m-%d) | wc -l
```

### 查询有效告警比例

```sql
-- 查看各任务的有效告警比例
SELECT 
    task_id,
    COUNT(*) as total_alerts,
    SUM(CASE WHEN detection_count > 0 THEN 1 ELSE 0 END) as with_detection,
    ROUND(SUM(CASE WHEN detection_count > 0 THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as detection_rate
FROM alerts
WHERE created_at >= datetime('now', '-1 day')
GROUP BY task_id
ORDER BY detection_rate DESC;
```

## 💡 使用建议

### 1. 推荐启用

**适用场景：**
- 人数统计、车辆检测等目标检测场景
- 大多数时间为空场景的监控
- 存储空间有限的环境

**节省效果显著：**
- 减少 70-90% 的存储空间
- 降低数据库体积
- 提高查询效率

### 2. 考虑关闭

**适用场景：**
- 需要完整的视频记录（无论是否有目标）
- 用于分析空场景的频率和时长
- 需要保留所有推理历史

### 3. 测试建议

启用功能前建议先测试：

```bash
# 1. 先关闭功能运行一段时间
save_only_with_detection = false

# 2. 观察告警中无检测结果的比例
SELECT 
    COUNT(*) as total,
    SUM(CASE WHEN detection_count = 0 THEN 1 ELSE 0 END) as zero_detection,
    ROUND(SUM(CASE WHEN detection_count = 0 THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as zero_rate
FROM alerts
WHERE created_at >= datetime('now', '-1 hour');

# 3. 如果 zero_rate > 50%，建议启用功能
save_only_with_detection = true
```

## 🐛 故障排查

### 问题1：图片被误删

**原因**：算法返回的检测结果格式不正确

**解决：**
1. 检查算法服务返回的 JSON 格式
2. 确保 `result` 中包含 `detections` 数组
3. 临时关闭功能：`save_only_with_detection = false`

### 问题2：删除失败

**原因**：MinIO 权限不足或网络问题

**查看日志：**
```bash
tail -f logs/sugar.log | grep "failed to delete image"
```

**解决：**
1. 检查 MinIO 配置和权限
2. 测试 MinIO 连接：`./scripts/test_minio.sh`
3. 删除失败不影响推理流程，图片会保留

### 问题3：告警数量突然减少

**原因**：启用功能后，无检测结果的告警不再保存

**验证：**
```bash
# 查看日志中删除的图片数量
cat logs/sugar.log | grep "no detection result" | wc -l
```

## 📖 相关文档

- [检测个数功能](FEATURE_UPDATE_DETECTION_COUNT.md)
- [任务ID下拉选择](FEATURE_TASK_ID_DROPDOWN.md)
- [智能推理使用指南](SMART_INFERENCE_USAGE.md)

---

**版本**：v1.2.0  
**更新日期**：2024-10-17  
**作者**：yanying team

