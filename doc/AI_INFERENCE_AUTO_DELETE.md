# AI推理自动删除MinIO图片功能说明

## 📋 功能概述

EasyDarwin AI分析插件已完善算法推理结果返回和自动删除MinIO图片的功能。

### 核心功能

1. **算法推理后返回完整结果**：记录检测对象数量、置信度、推理耗时等详细信息
2. **自动删除无检测结果的图片**：当检测对象数量为 0 时，自动删除MinIO中的图片
3. **智能图片管理**：避免MinIO存储无用图片，节省存储空间

---

## 🎯 工作流程

### 1. 推理流程

```
图片上传到MinIO
    ↓
扫描器扫描新图片
    ↓
添加到推理队列
    ↓
调度器分配算法服务
    ↓
算法服务推理并返回结果
    ↓
检查 total_count 字段
    ├─ = 0 → 删除图片，不保存告警
    └─ > 0 → 保存告警，推送到MQ
```

### 2. 删除策略

图片会在以下情况被自动删除：

| 情况 | 删除原因 | 说明 |
|------|---------|------|
| `total_count = 0` | `no_detection` | 未检测到目标对象 |
| 预签名URL失败 | `presign_failed` | 无法生成访问URL |
| 推理返回失败 | `inference_failed` | 算法返回 success=false |
| 无可用算法 | `no_algorithm` | 该任务类型没有算法服务 |

---

## 🔧 配置

### config.toml 配置

```toml
[ai_analysis]
enable = true
scan_interval_sec = 5
max_concurrent_infer = 10
heartbeat_timeout_sec = 60

# 🔑 关键配置：只保存有检测结果的告警
save_only_with_detection = true  # true=自动删除无检测结果图片，false=保留所有图片

# 消息队列配置
mq_type = "kafka"
mq_address = "localhost:9092"
mq_topic = "ai_alerts"

[frame_extractor]
enable = true
store = "minio"  # 必须使用 minio

[frame_extractor.minio]
endpoint = "10.1.6.230:9000"
access_key = "admin"
secret_key = "admin123"
bucket = "images"
use_ssl = false
base_path = "frames"
```

---

## 📊 推理结果格式规范

### 算法服务必须返回的字段

```json
{
  "success": true,
  "result": {
    "total_count": 3,         // ⚠️ 必填！检测对象数量
    "detections": [...],      // 可选：检测详情
    "message": "检测到3人"    // 可选：描述信息
  },
  "confidence": 0.95,         // 置信度
  "inference_time_ms": 45     // 推理耗时（毫秒）
}
```

### total_count 提取规则

系统按以下优先级提取检测对象数量：

1. **total_count** (最高优先级)
2. **count**
3. **num**
4. **detections 数组长度**
5. **objects 数组长度**

### 示例1：人数统计（有检测结果）

```json
{
  "success": true,
  "result": {
    "total_count": 3,
    "detections": [
      {"class": "person", "confidence": 0.95, "bbox": [100, 200, 150, 300]},
      {"class": "person", "confidence": 0.92, "bbox": [200, 220, 250, 320]},
      {"class": "person", "confidence": 0.89, "bbox": [300, 240, 350, 340]}
    ],
    "message": "检测到3人"
  },
  "confidence": 0.95,
  "inference_time_ms": 45
}
```

**结果**：✅ 保存告警，推送到MQ，保留图片

---

### 示例2：人员跌倒检测（无检测结果）

```json
{
  "success": true,
  "result": {
    "total_count": 0,
    "fall_detected": false,
    "persons": 3,
    "message": "未检测到跌倒"
  },
  "confidence": 0.98,
  "inference_time_ms": 52
}
```

**结果**：❌ 不保存告警，删除图片（`save_only_with_detection=true` 时）

---

### 示例3：吸烟检测（有检测结果）

```json
{
  "success": true,
  "result": {
    "total_count": 1,
    "smoking_detected": true,
    "detections": [
      {"location": {"x": 320, "y": 240}, "confidence": 0.87}
    ],
    "message": "检测到吸烟行为"
  },
  "confidence": 0.87,
  "inference_time_ms": 38
}
```

**结果**：✅ 保存告警，推送到MQ，保留图片

---

## 📝 日志说明

### 推理成功日志

```
[INFO] inference result received 
  image=frames/人数统计/task_001/20241017-143520.000.jpg
  algorithm=demo_algo_v1
  detection_count=3
  confidence=0.95
  inference_time_ms=45
  result=map[detections:[...] message:检测到3人 total_count:3]

[INFO] inference completed and saved
  algorithm=demo_algo_v1
  task_id=task_001
  task_type=人数统计
  detection_count=3
  alert_id=12345
  confidence=0.95
  inference_time_ms=45
```

### 无检测结果删除日志

```
[INFO] no detection result, deleting image
  image=frames/人员跌倒/task_002/20241017-143521.000.jpg
  task_id=task_002
  task_type=人员跌倒
  algorithm=demo_algo_v1

[INFO] image deleted from MinIO
  path=frames/人员跌倒/task_002/20241017-143521.000.jpg
  reason=no_detection

[INFO] image deleted successfully (no detection)
  path=frames/人员跌倒/task_002/20241017-143521.000.jpg
  task_id=task_002
```

### 错误日志

```
[ERROR] failed to delete image from MinIO
  path=frames/xxx/test.jpg
  reason=no_detection
  err=context deadline exceeded
```

---

## 🧪 测试

### 1. 启动算法服务（示例）

```bash
cd /code/EasyDarwin/examples
python3 algorithm_service.py \
  --service-id demo_algo_v1 \
  --name "演示算法服务" \
  --task-types "人数统计" "人员跌倒" "吸烟检测" "车辆检测" "安全帽检测" \
  --port 8000 \
  --easydarwin http://localhost:5066
```

### 2. 验证自动删除功能

#### 场景1：有检测结果（不删除）

```bash
# 模拟"人数统计"任务，会检测到3人 (total_count=3)
# 图片会被保留，告警会被保存
```

**预期**：
- ✅ 图片保留在MinIO
- ✅ 告警保存到数据库
- ✅ 消息推送到Kafka

#### 场景2：无检测结果（自动删除）

```bash
# 模拟"人员跌倒"任务，未检测到跌倒 (total_count=0)
# 图片会被自动删除
```

**预期**：
- ❌ 图片从MinIO删除
- ❌ 不保存告警
- ❌ 不推送消息

### 3. 查看MinIO存储

访问 MinIO 控制台：
```
http://10.1.6.230:9000
用户名: admin
密码: admin123
```

查看 `images` bucket 中的 `frames/` 目录：
- 有检测结果的图片会保留
- 无检测结果的图片会被删除

---

## 📈 性能监控

### 队列统计

```
[INFO] performance statistics
  queue=map[
    added_total:1523
    dropped_total:45
    processed_total:1478
    current_size:12
  ]
  performance=map[
    total_inferences:1478
    success_count:1450
    failed_count:28
    avg_time_ms:52.3
    max_time_ms:234
    min_time_ms:15
  ]
```

### 删除统计

通过日志统计删除原因：

```bash
# 统计各种删除原因
grep "image deleted from MinIO" easydarwin.log | grep -o 'reason=[a-z_]*' | sort | uniq -c

# 输出示例：
#   1245 reason=no_detection        # 无检测结果删除
#      5 reason=presign_failed      # 预签名失败
#      2 reason=inference_failed    # 推理失败
#     12 reason=no_algorithm        # 无算法服务
```

---

## 🔍 故障排查

### 问题1：图片被错误删除

**可能原因**：
- 算法服务未正确返回 `total_count` 字段
- `save_only_with_detection` 配置为 `true`

**解决方法**：
1. 检查算法服务返回的JSON格式
2. 确保 `result.total_count` 存在且类型正确
3. 或设置 `save_only_with_detection = false`

### 问题2：图片没有被删除

**可能原因**：
- `save_only_with_detection` 配置为 `false`
- MinIO权限不足

**解决方法**：
1. 检查配置：`save_only_with_detection = true`
2. 验证MinIO账号有删除权限
3. 查看错误日志

### 问题3：删除失败

**可能原因**：
- MinIO连接超时
- 图片已被其他进程删除

**解决方法**：
1. 检查MinIO服务状态
2. 增加超时时间
3. 查看详细错误日志

---

## 💡 最佳实践

### 1. 算法开发建议

```python
def infer(image_url, task_type):
    """推理函数"""
    # 1. 下载图片
    # 2. 加载模型
    # 3. 执行推理
    results = model.predict(image)
    
    # 4. 构建返回结果（必须包含 total_count）
    return {
        "total_count": len(results),  # ⚠️ 必须返回！
        "detections": results,
        "message": f"检测到{len(results)}个对象"
    }
```

### 2. 存储优化

- **启用自动删除**：`save_only_with_detection = true`
- **合理设置扫描间隔**：`scan_interval_sec = 5`（根据业务调整）
- **控制并发数**：`max_concurrent_infer = 10`（根据算力调整）

### 3. 监控告警

定期检查：
- 图片删除率（正常应该在30-70%）
- MinIO存储使用率
- 推理失败率

### 4. 数据保留策略

```toml
# 只保存有价值的告警数据
save_only_with_detection = true

# 建议配合定期清理旧数据
# - 数据库告警记录：保留30天
# - MinIO图片：保留7天
```

---

## 🆚 对比

### 功能对比

| 功能 | 之前 | 现在 |
|-----|------|------|
| 推理结果 | ✅ 返回 | ✅ 返回（增强日志） |
| 检测对象数量 | ✅ 提取 | ✅ 提取（支持多字段） |
| 无检测图片 | ❌ 保留 | ✅ 自动删除 |
| 删除原因记录 | ❌ 无 | ✅ 详细记录 |
| 错误处理 | ⚠️ 简单 | ✅ 完善 |
| 推理耗时 | ⚠️ 使用算法返回 | ✅ 实际测量 |

---

## 📚 相关文件

- `internal/plugin/aianalysis/scheduler.go` - 推理调度和图片删除逻辑
- `internal/plugin/aianalysis/service.go` - AI分析服务主入口
- `examples/algorithm_service.py` - 算法服务示例（含多种场景）
- `config.toml` - 配置文件

---

## ✅ 总结

### 核心改进

1. ✅ **完善推理结果返回**：详细记录检测对象数量、置信度、推理耗时等信息
2. ✅ **自动删除无检测图片**：`total_count = 0` 时自动删除MinIO图片
3. ✅ **删除原因追踪**：记录每次删除的原因（no_detection、presign_failed等）
4. ✅ **错误处理增强**：更完善的错误处理和日志记录
5. ✅ **性能优化**：避免存储无用图片，节省存储空间

### 使用建议

- 生产环境建议开启：`save_only_with_detection = true`
- 算法服务必须返回：`result.total_count` 字段
- 定期监控删除日志和MinIO存储使用率

---

**文档版本**: v1.0  
**更新日期**: 2024-10-17  
**维护者**: EasyDarwin Team

