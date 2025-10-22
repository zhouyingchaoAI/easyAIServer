# 快速指南：只保存有检测结果的告警

## 📝 一句话说明

**启用此功能后，系统只保存检测到目标的告警，无检测结果的图片会被自动删除，节省 70-90% 的存储空间。**

## 🚀 5秒钟启用

### 1. 编辑配置文件

```bash
vim configs/config.toml
```

### 2. 添加/修改配置

```toml
[ai_analysis]
save_only_with_detection = true  # ← 添加这一行
```

### 3. 重启服务

```bash
pkill -f easydarwin
./easydarwin
```

✅ **完成！** 现在只会保存有检测结果的告警了。

## 🎯 效果对比

### 关闭功能（默认行为）

```
抽取 100 帧图片
├─ 有检测结果：10 帧
│  ├─ 保存到数据库 ✅
│  ├─ 推送到 Kafka ✅
│  └─ MinIO 保留图片 ✅
│
└─ 无检测结果：90 帧
   ├─ 保存到数据库 ✅
   ├─ 推送到 Kafka ✅
   └─ MinIO 保留图片 ✅

结果：
- 数据库记录：100 条
- MinIO 图片：100 张
- Kafka 消息：100 条
- 存储空间：30 MB
```

### 启用功能

```
抽取 100 帧图片
├─ 有检测结果：10 帧
│  ├─ 保存到数据库 ✅
│  ├─ 推送到 Kafka ✅
│  └─ MinIO 保留图片 ✅
│
└─ 无检测结果：90 帧
   ├─ 保存到数据库 ❌ 不保存
   ├─ 推送到 Kafka ❌ 不推送
   └─ MinIO 删除图片 🗑️ 自动删除

结果：
- 数据库记录：10 条
- MinIO 图片：10 张
- Kafka 消息：10 条
- 存储空间：3 MB
节省：90%！
```

## 📊 存储节省计算器

### 场景 1：24小时监控（每秒1帧）

```
参数：
- 抽帧频率：1 帧/秒
- 运行时间：24 小时
- 图片大小：300 KB/张
- 有目标时间：10%

关闭功能：
- 总图片数：86,400 张
- 存储空间：25.9 GB

启用功能：
- 保存图片：8,640 张
- 存储空间：2.6 GB
- 节省：23.3 GB (90%)
```

### 场景 2：高频抽帧（每秒5帧）

```
参数：
- 抽帧频率：5 帧/秒
- 运行时间：24 小时
- 图片大小：300 KB/张
- 有目标时间：20%

关闭功能：
- 总图片数：432,000 张
- 存储空间：129.6 GB

启用功能：
- 保存图片：86,400 张
- 存储空间：25.9 GB
- 节省：103.7 GB (80%)
```

## 🎨 配置完整示例

```toml
[frame_extractor]
enable = true
interval_ms = 1000  # 每秒1帧
store = 'minio'

[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'

[ai_analysis]
enable = true
scan_interval_sec = 5
max_concurrent_infer = 20
save_only_with_detection = true  # ← 关键配置
```

## 🔍 验证功能是否生效

### 方法 1：查看日志

```bash
# 实时查看日志
tail -f logs/sugar.log | grep "no detection result"

# 如果看到类似日志，说明功能已生效：
# DEBUG no detection result, deleting image image=人数统计/task_1/frame_001.jpg
# DEBUG image deleted from MinIO path=人数统计/task_1/frame_001.jpg
```

### 方法 2：查看启动日志

```bash
tail -f logs/sugar.log | grep "starting AI analysis"

# 应该看到：
# INFO starting AI analysis plugin 
#   scan_interval=5 
#   save_only_with_detection=true  ← 确认已启用
```

### 方法 3：统计告警数量

```bash
# 查询最近1小时的告警
curl http://localhost:5066/api/v1/alerts?page=1&page_size=100

# 如果功能生效，告警数量应该明显减少
```

## 💡 使用建议

### ✅ 推荐启用的场景

1. **人数统计**
   - 大部分时间无人的区域
   - 只关心有人时的情况

2. **车辆检测**
   - 空旷时间较长的道路
   - 只记录有车的画面

3. **安全监控**
   - 工地、仓库等大部分时间无人区域
   - 节省存储，降低成本

4. **存储受限环境**
   - 磁盘空间有限
   - 需要长期保留数据

### ❌ 不推荐启用的场景

1. **需要完整记录**
   - 法律合规要求保留所有画面
   - 需要分析空场景的频率

2. **研究分析用途**
   - 需要统计空场景的时长
   - 分析无目标时的环境变化

3. **测试阶段**
   - 验证算法准确性
   - 调试推理流程

## 🐛 常见问题

### Q1：启用后告警数量为什么变少了？

**A**：这是正常现象！功能生效后，只保存有检测结果的告警，无检测结果的不再保存。

**验证：**
```bash
# 查看删除的图片数量
cat logs/sugar.log | grep "no detection result" | wc -l
```

### Q2：如何临时查看无检测结果的记录？

**A**：临时关闭功能：

```toml
save_only_with_detection = false  # 临时关闭
```

重启服务后，会保存所有结果。

### Q3：删除的图片能恢复吗？

**A**：不能。图片一旦删除就无法恢复。建议先测试确认效果后再长期启用。

### Q4：算法返回格式需要调整吗？

**A**：不需要。系统自动识别多种格式：

```json
// 支持的格式
{
  "result": {
    "detections": [...],  // 方式1（推荐）
    "objects": [...],     // 方式2
    "count": 5,          // 方式3
    "num": 5             // 方式4
  }
}
```

## 📈 监控与统计

### 查看今天删除的图片数

```bash
cat logs/sugar.log | grep "no detection result" | grep $(date +%Y-%m-%d) | wc -l
```

### 查看存储空间节省

```bash
# 查看 MinIO 使用情况
mc du local/images

# 对比启用前后的存储大小
```

### 统计有效告警比例

```sql
SELECT 
    DATE(created_at) as date,
    COUNT(*) as total_alerts,
    AVG(detection_count) as avg_detections
FROM alerts
WHERE created_at >= datetime('now', '-7 days')
GROUP BY DATE(created_at)
ORDER BY date DESC;
```

## 🔄 回滚方法

如果需要恢复原有行为：

```toml
[ai_analysis]
save_only_with_detection = false  # 或者删除这一行
```

重启服务即可。

## 📚 相关文档

- [详细功能说明](doc/FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [检测个数功能](doc/FEATURE_UPDATE_DETECTION_COUNT.md)
- [完整更新日志](CHANGELOG_DETECTION_COUNT.md)

---

**记住：启用此功能可以节省 70-90% 的存储空间！** 🎉

**版本**：v1.2.0  
**更新日期**：2024-10-17

