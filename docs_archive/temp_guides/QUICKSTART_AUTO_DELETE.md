# 🚀 快速开始：AI推理自动删除MinIO图片

## 📝 功能概述

当算法推理返回**检测对象数量为0**时，系统会**自动删除MinIO中的图片**，避免存储无用数据。

---

## ⚡ 5分钟快速开始

### 1️⃣ 配置 (config.toml)

```toml
[ai_analysis]
enable = true
save_only_with_detection = true  # ← 开启自动删除功能

[frame_extractor]
store = "minio"

[frame_extractor.minio]
endpoint = "10.1.6.230:9000"
access_key = "admin"
secret_key = "admin123"
bucket = "images"
```

### 2️⃣ 启动算法服务

```bash
# 方式1: 使用示例服务（模拟推理）
cd /code/EasyDarwin
python3 examples/algorithm_service.py --easydarwin http://localhost:5066

# 方式2: 使用YOLO服务（真实推理）
python3 examples/yolo_algorithm_service.py \
  --model yolov8n.pt \
  --easydarwin http://localhost:5066
```

### 3️⃣ 验证功能

```bash
# 运行测试脚本
python3 test_auto_delete.py
```

---

## 🎯 核心逻辑

```
算法推理
    ↓
检查 total_count
    ├─ = 0 → ❌ 删除图片
    └─ > 0 → ✅ 保留图片 + 保存告警
```

### 示例1：检测到目标（保留图片）

```python
# 算法返回
{
    "success": true,
    "result": {
        "total_count": 3,  # ✅ 有检测结果
        "detections": [...]
    }
}

# 系统操作
✅ 保留图片
✅ 保存告警到数据库
✅ 推送到Kafka
```

### 示例2：未检测到目标（删除图片）

```python
# 算法返回
{
    "success": true,
    "result": {
        "total_count": 0,  # ❌ 无检测结果
        "message": "未检测到目标"
    }
}

# 系统操作
❌ 删除MinIO图片
❌ 不保存告警
❌ 不推送消息
```

---

## 📊 日志示例

### 有检测结果（保留）

```log
[INFO] inference result received
  detection_count=3
  confidence=0.95

[INFO] inference completed and saved
  alert_id=12345
  detection_count=3
```

### 无检测结果（删除）

```log
[INFO] inference result received
  detection_count=0

[INFO] no detection result, deleting image
  image=frames/人员跌倒/task_002/20241017-143521.jpg

[INFO] image deleted from MinIO
  path=frames/人员跌倒/task_002/20241017-143521.jpg
  reason=no_detection
```

---

## 🔧 算法服务开发

### 必须返回 total_count 字段

```python
def infer(image_url, task_type):
    # 1. 下载图片
    # 2. 模型推理
    results = model.predict(image)
    
    # 3. 返回结果（必须包含 total_count）
    return {
        "total_count": len(results),  # ⚠️ 必须！
        "detections": results,
        "message": f"检测到{len(results)}个对象"
    }
```

### total_count 支持的字段

系统会按优先级提取：
1. `total_count` ⭐ 推荐
2. `count`
3. `num`
4. `detections` 数组长度
5. `objects` 数组长度

---

## 🧪 测试验证

### 查看MinIO存储

```bash
# 访问MinIO控制台
http://10.1.6.230:9000
用户名: admin
密码: admin123

# 查看 images/frames/ 目录
# - 有检测结果的图片会保留
# - 无检测结果的图片会被删除
```

### 查看日志

```bash
# 查看删除日志
grep "image deleted from MinIO" easydarwin.log

# 统计删除原因
grep "image deleted from MinIO" easydarwin.log | grep -o 'reason=[a-z_]*' | sort | uniq -c
```

---

## ⚙️ 配置选项

| 配置项 | 说明 | 默认值 |
|-------|------|--------|
| `save_only_with_detection` | 只保存有检测结果的告警 | `false` |
| `scan_interval_sec` | 扫描间隔（秒） | `5` |
| `max_concurrent_infer` | 最大并发推理数 | `5` |

### 推荐配置

**生产环境（节省存储）：**
```toml
save_only_with_detection = true   # 自动删除无检测图片
scan_interval_sec = 5              # 快速扫描
max_concurrent_infer = 10          # 提高并发
```

**开发测试（保留所有）：**
```toml
save_only_with_detection = false  # 保留所有图片
scan_interval_sec = 10            # 降低频率
max_concurrent_infer = 3          # 降低并发
```

---

## 🎓 进阶使用

### 1. 自定义删除策略

修改 `scheduler.go` 中的删除逻辑：

```go
// 示例：只有连续3次检测为0才删除
if s.saveOnlyWithDetection && detectionCount == 0 {
    // 自定义逻辑
    if shouldDelete(image.Path) {
        s.deleteImageWithReason(image.Path, "no_detection")
    }
}
```

### 2. 批量清理历史图片

```python
# 清理7天前的图片
from minio import Minio
from datetime import datetime, timedelta

client = Minio("10.1.6.230:9000", ...)
cutoff = datetime.now() - timedelta(days=7)

for obj in client.list_objects("images", prefix="frames/", recursive=True):
    if obj.last_modified < cutoff:
        client.remove_object("images", obj.object_name)
```

### 3. 监控告警

设置监控脚本，当删除率过高时告警：

```python
# 删除率 > 80% 时告警
if deleted / total > 0.8:
    send_alert("图片删除率过高，请检查算法服务")
```

---

## ❓ 常见问题

### Q1: 图片被错误删除？

**A:** 检查算法服务是否正确返回 `total_count` 字段。

```bash
# 查看推理结果
grep "inference result received" easydarwin.log
```

### Q2: 图片没有被删除？

**A:** 检查配置：

```toml
[ai_analysis]
save_only_with_detection = true  # 必须为 true
```

### Q3: 删除失败？

**A:** 检查MinIO权限和连接：

```bash
# 测试MinIO连接
python3 test_minio_connection.py
```

---

## 📚 相关文档

- [详细功能说明](AI_INFERENCE_AUTO_DELETE.md)
- [算法服务示例](examples/algorithm_service.py)
- [YOLO服务示例](examples/yolo_algorithm_service.py)
- [测试脚本](test_auto_delete.py)

---

## ✅ 检查清单

部署前检查：

- [ ] `save_only_with_detection = true`
- [ ] MinIO连接正常
- [ ] 算法服务已注册
- [ ] 算法返回 `total_count` 字段
- [ ] 日志可以查看删除记录

---

## 💡 最佳实践

1. ✅ **生产环境开启自动删除**：`save_only_with_detection = true`
2. ✅ **算法必须返回 total_count**：确保字段存在且准确
3. ✅ **定期检查删除率**：正常应在30-70%
4. ✅ **监控MinIO存储**：避免存储满
5. ✅ **保留重要日志**：便于故障排查

---

**祝您使用愉快！** 🎉

如有问题，请查看 [AI_INFERENCE_AUTO_DELETE.md](AI_INFERENCE_AUTO_DELETE.md) 详细文档。

