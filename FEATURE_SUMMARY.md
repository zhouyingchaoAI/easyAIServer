# yanying 智能告警系统 - 功能汇总

## 🎉 最新功能总览

本文档汇总了 yanying 智能告警系统的所有增强功能。

---

## 📊 功能列表

### 1️⃣ 检测实例个数统计 (v1.1.0)

**功能**：告警记录中添加检测个数字段，支持按检测个数过滤

**使用场景**：
- 查询检测到5人以上的告警
- 过滤空场景（检测个数=0）
- 统计检测情况

**配置**：无需配置，自动生效

**详细文档**：[FEATURE_UPDATE_DETECTION_COUNT.md](doc/FEATURE_UPDATE_DETECTION_COUNT.md)

---

### 2️⃣ 任务ID自动下拉选择 (v1.1.1)

**功能**：任务ID筛选从手动输入改为自动下拉选择

**使用场景**：
- 快速选择任务ID，避免拼写错误
- 搜索过滤任务ID
- 提升用户体验

**配置**：无需配置，自动生效

**详细文档**：[FEATURE_TASK_ID_DROPDOWN.md](doc/FEATURE_TASK_ID_DROPDOWN.md)

---

### 3️⃣ 只保存有检测结果的告警 (v1.2.0) ⭐推荐

**功能**：自动过滤无检测结果的告警，删除无用图片

**使用场景**：
- 节省存储空间（70-90%）
- 减少无效告警
- 降低运营成本

**配置**：
```toml
[ai_analysis]
save_only_with_detection = true  # 启用功能
```

**详细文档**：
- [详细说明](doc/FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [快速指南](QUICK_GUIDE_SAVE_ONLY_WITH_DETECTION.md)

---

### 4️⃣ 队列丢弃优化 (v1.2.1) ⭐推荐

**功能**：队列丢弃图片时同步删除MinIO文件

**使用场景**：
- 推理速度慢于抽帧，队列经常积压
- 长期运行的生产环境
- 存储空间有限

**配置**：自动启用，无需配置

**详细文档**：[FEATURE_QUEUE_DROP_OPTIMIZATION.md](doc/FEATURE_QUEUE_DROP_OPTIMIZATION.md)

---

## 🎯 功能对比表

| 功能 | 版本 | 节省空间 | 需要配置 | 推荐指数 |
|------|------|----------|----------|----------|
| 检测个数统计 | v1.1.0 | - | 否 | ⭐⭐⭐⭐ |
| 任务ID下拉 | v1.1.1 | - | 否 | ⭐⭐⭐⭐ |
| 只保存有检测结果 | v1.2.0 | 70-90% | 是 | ⭐⭐⭐⭐⭐ |
| 队列丢弃优化 | v1.2.1 | 额外60% | 否 | ⭐⭐⭐⭐⭐ |

---

## 🚀 快速上手

### 所有功能一次性配置

```toml
# configs/config.toml

[frame_extractor]
enable = true
interval_ms = 1000
store = 'minio'
task_types = ['人数统计', '人员跌倒', '安全帽检测']

[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'

[ai_analysis]
enable = true
scan_interval_sec = 5
max_concurrent_infer = 20
heartbeat_timeout_sec = 90
save_only_with_detection = true  # ← v1.2.0 新增：只保存有检测结果
```

### 数据库迁移（仅首次需要）

```bash
# 添加 detection_count 字段
sqlite3 configs/data.db
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
.exit
```

### 重启服务

```bash
# 重新编译
make build/linux

# 启动服务
cd build/EasyDarwin-lin-*
./easydarwin
```

---

## 📈 使用示例

### 示例 1：查询特定检测个数的告警

**Web界面：**
1. 打开 http://localhost:5066/#/alerts
2. 任务类型：人数统计
3. 最少检测数：5
4. 最多检测数：20
5. 点击"查询"

**API：**
```bash
curl "http://localhost:5066/api/v1/alerts?task_type=人数统计&min_detections=5&max_detections=20"
```

### 示例 2：选择任务ID

**Web界面：**
1. 打开告警列表
2. 点击"任务ID"下拉框
3. 输入关键词搜索（如 "人数"）
4. 选择目标任务
5. 自动查询

### 示例 3：只保存有检测结果

**启用功能：**
```toml
[ai_analysis]
save_only_with_detection = true
```

**效果：**
- 检测到目标：保存告警 + 保留图片
- 未检测到：删除图片 + 不保存告警

---

## 💾 存储空间节省

### 场景：24小时监控，每秒1帧

| 项目 | 关闭功能 | 启用功能 | 节省 |
|------|----------|----------|------|
| 抽帧总数 | 86,400 | 86,400 | - |
| 保存图片 | 86,400 | 8,640 | 90% |
| 告警记录 | 86,400 | 8,640 | 90% |
| 存储空间 | 25.9 GB | 2.6 GB | **23.3 GB** |

**结论**：启用 `save_only_with_detection` 可节省 **70-90%** 的存储空间！

---

## 🔧 算法服务开发指南

### 返回格式要求

为了让所有功能正常工作，算法服务应返回以下格式：

```json
{
  "success": true,
  "result": {
    "detections": [
      {
        "class_name": "person",
        "confidence": 0.95,
        "bbox": [100, 150, 200, 350]
      }
    ]
  },
  "confidence": 0.95,
  "inference_time_ms": 120
}
```

**关键点：**
- `result.detections` 数组：用于统计检测个数
- 数组长度 = 检测个数
- 空数组 = 无检测结果（会被删除）

---

## 📊 监控与统计

### 查看系统状态

```bash
# 1. 查看告警列表
curl http://localhost:5066/api/v1/alerts?page=1&page_size=20

# 2. 查看算法服务
curl http://localhost:5066/api/v1/ai_analysis/services

# 3. 查看任务ID列表
curl http://localhost:5066/api/v1/alerts/task_ids

# 4. 查看实时日志
tail -f logs/sugar.log
```

### 统计查询示例

```sql
-- 查看各任务的检测统计
SELECT 
    task_id,
    task_type,
    COUNT(*) as total_alerts,
    AVG(detection_count) as avg_detections,
    MAX(detection_count) as max_detections
FROM alerts
WHERE created_at >= datetime('now', '-1 day')
GROUP BY task_id, task_type
ORDER BY avg_detections DESC;

-- 查看有效告警比例
SELECT 
    task_type,
    COUNT(*) as total,
    SUM(CASE WHEN detection_count > 0 THEN 1 ELSE 0 END) as with_detection,
    ROUND(SUM(CASE WHEN detection_count > 0 THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as rate
FROM alerts
WHERE created_at >= datetime('now', '-7 days')
GROUP BY task_type;
```

---

## 🐛 故障排查

### 问题 1：检测个数显示为 0

**原因**：算法返回格式不正确

**解决**：
1. 检查算法服务返回的 JSON
2. 确保包含 `detections` 数组
3. 参考上方的返回格式示例

### 问题 2：任务ID下拉列表为空

**原因**：数据库中没有告警记录

**解决**：
1. 确认有告警记录：`curl http://localhost:5066/api/v1/alerts`
2. 检查 API：`curl http://localhost:5066/api/v1/alerts/task_ids`

### 问题 3：图片被误删

**原因**：`save_only_with_detection = true` 且算法返回无检测结果

**解决**：
1. 临时关闭功能：`save_only_with_detection = false`
2. 检查算法服务返回的数据格式
3. 查看日志：`tail -f logs/sugar.log | grep "no detection result"`

---

## 📚 完整文档索引

### 功能文档
- [检测个数统计](doc/FEATURE_UPDATE_DETECTION_COUNT.md)
- [任务ID下拉](doc/FEATURE_TASK_ID_DROPDOWN.md)
- [只保存有检测结果](doc/FEATURE_SAVE_ONLY_WITH_DETECTION.md)

### 快速指南
- [任务ID下拉](QUICK_UPDATE_GUIDE.md)
- [只保存有检测结果](QUICK_GUIDE_SAVE_ONLY_WITH_DETECTION.md)

### 其他文档
- [数据库迁移](doc/DATABASE_MIGRATION.md)
- [更新日志](CHANGELOG_DETECTION_COUNT.md)
- [完整中文文档](README_CN.md)

---

## 🎓 学习路径

### 新手入门
1. 阅读 [README_简易使用.md](README_简易使用.md)
2. 启动系统，访问 Web 界面
3. 创建抽帧任务
4. 注册算法服务
5. 查看告警列表

### 进阶配置
1. 启用检测个数过滤
2. 配置任务ID筛选
3. 启用 `save_only_with_detection`
4. 监控系统性能

### 高级优化
1. 优化抽帧频率
2. 调整并发推理数
3. 配置消息队列
4. 自定义告警规则

---

## 💡 最佳实践

### 推荐配置（生产环境）

```toml
[frame_extractor]
interval_ms = 1000  # 每秒1帧，平衡性能和效果

[ai_analysis]
scan_interval_sec = 5  # 5秒扫描一次
max_concurrent_infer = 20  # 根据服务器性能调整
save_only_with_detection = true  # 强烈推荐启用
```

### 存储管理建议

1. **定期清理旧数据**
```sql
-- 删除30天前的告警
DELETE FROM alerts WHERE created_at < datetime('now', '-30 days');
```

2. **备份重要数据**
```bash
# 备份数据库
cp configs/data.db backups/data_$(date +%Y%m%d).db

# 备份MinIO数据
mc mirror local/images/ backup/images/
```

3. **监控存储使用**
```bash
# 查看磁盘使用
df -h

# 查看MinIO使用
mc du local/images
```

---

## 🎉 总结

yanying 智能告警系统通过三大功能增强，实现了：

✅ **更智能**：自动统计检测个数，智能过滤无效告警  
✅ **更易用**：任务ID下拉选择，搜索过滤  
✅ **更高效**：节省 70-90% 存储空间，降低运营成本

**立即体验这些强大功能，让视频AI分析更简单、更高效！** 🚀

---

**版本**：v1.2.0  
**更新日期**：2024-10-17  
**作者**：yanying team

