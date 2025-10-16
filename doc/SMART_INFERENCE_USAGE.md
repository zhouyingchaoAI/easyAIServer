# 智能推理系统使用指南

## 概述

yanying平台已集成智能自适应推理系统，能够自动处理推理速度与抽帧速度不匹配的问题，确保系统稳定运行。

## 核心功能

### 1. 智能队列管理
- **自动丢弃策略**：当队列积压时，自动丢弃旧图片，避免存储爆满
- **队列容量限制**：默认最大100张，可配置
- **实时统计**：实时监控队列状态、丢弃率

### 2. 性能监控
- **推理速度监控**：实时统计推理速度（张/秒）
- **推理时间监控**：监控单张图片推理耗时
- **慢推理告警**：超过阈值（默认5秒）自动告警

### 3. 智能告警
- **队列积压告警**：队列积压超过50张时告警
- **推理过慢告警**：推理速度跟不上抽帧速度时告警
- **高丢弃率告警**：丢弃率超过30%时告警

## 配置说明

智能推理系统无需额外配置，已集成到 `ai_analysis` 模块中。关键配置参数：

```toml
[frame_extractor]
enable = true
interval_ms = 200  # 抽帧间隔（每秒5帧）

[ai_analysis]
enable = true
scan_interval_sec = 1     # MinIO扫描间隔
max_concurrent_infer = 50 # 最大并发推理数
```

## API接口

### 1. 查询性能统计

```bash
GET http://localhost:10008/api/performance/stats
```

**响应示例：**
```json
{
  "queue": {
    "current_size": 10,
    "max_size": 100,
    "total_added": 1000,
    "total_processed": 990,
    "dropped_total": 5,
    "dropped_oldest": 3,
    "dropped_newest": 2
  },
  "performance": {
    "total_inferences": 990,
    "avg_time_ms": 1200,
    "max_time_ms": 3500,
    "min_time_ms": 800,
    "success_rate": 0.98,
    "infer_per_second": 4.5
  },
  "drop_rate": 0.005,
  "healthy": true
}
```

### 2. 重置队列

```bash
POST http://localhost:10008/api/performance/reset
```

**响应示例：**
```json
{
  "success": true,
  "message": "Queue cleared successfully"
}
```

## 日志监控

### 启动日志
```
INFO AI analysis plugin started successfully queue_max_size=100 queue_strategy=drop_oldest slow_threshold_ms=5000
```

### 运行日志
```
INFO images added to queue added=5 queue_size=15
INFO performance statistics queue={"current_size":15, ...} performance={"avg_time_ms":1200, ...}
```

### 告警日志
```
WARN queue backlog alert queue_size=55 threshold=50
ERROR inference too slow duration_ms=5200 threshold_ms=5000
ERROR 图片丢弃率过高，推理能力严重不足 drop_rate=0.35
```

## 性能指标解读

### 1. 队列健康指标
- **current_size < 20**：系统运行良好 ✅
- **20 ≤ current_size < 50**：系统接近满载 ⚠️
- **current_size ≥ 50**：系统过载，触发告警 ❌

### 2. 丢弃率指标
- **drop_rate < 0.05**：偶尔丢弃，可接受 ✅
- **0.05 ≤ drop_rate < 0.3**：频繁丢弃，需关注 ⚠️
- **drop_rate ≥ 0.3**：严重丢弃，推理能力不足 ❌

### 3. 推理速度指标
- **infer_per_second ≥ 5**：达标（匹配抽帧速度） ✅
- **3 ≤ infer_per_second < 5**：接近阈值，需优化 ⚠️
- **infer_per_second < 3**：速度过慢，需扩容 ❌

## 优化建议

### 1. 推理速度不足
**症状：** `infer_per_second < 5`，`drop_rate` 持续上升

**解决方案：**
- 增加算法服务实例数
- 优化算法模型（轻量化、量化）
- 降低抽帧频率：`interval_ms = 400`（每秒2.5帧）
- 增加并发数：`max_concurrent_infer = 100`

### 2. 队列积压严重
**症状：** `current_size` 持续接近 `max_size`

**解决方案：**
- 减小MinIO扫描间隔：`scan_interval_sec = 0.5`
- 增加队列容量（修改代码中 `NewInferenceQueue(100, ...)` 参数）
- 增加算法服务实例

### 3. 告警频繁
**症状：** 频繁出现告警日志

**解决方案：**
- 调整告警阈值（修改代码）
- 优化系统性能
- 查看具体告警类型，针对性解决

## 监控脚本示例

```bash
#!/bin/bash
# 实时监控性能指标

while true; do
    echo "========== $(date) =========="
    curl -s http://localhost:10008/api/performance/stats | \
        python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f\"队列大小: {data['queue']['current_size']}/{data['queue']['max_size']}\")
print(f\"丢弃率: {data['drop_rate']*100:.2f}%\")
print(f\"推理速度: {data['performance']['infer_per_second']:.2f} 张/秒\")
print(f\"平均耗时: {data['performance']['avg_time_ms']} ms\")
print(f\"健康状态: {'✅ 正常' if data['healthy'] else '❌ 异常'}\")
"
    echo ""
    sleep 10
done
```

## 故障排查

### 问题1：推理不工作
1. 检查 `ai_analysis.enable = true`
2. 确认算法服务已注册：`curl http://localhost:10008/api/ai/services`
3. 查看日志：`tail -f build/.../logs/sugar.log`

### 问题2：丢弃率过高
1. 查看算法服务数量是否充足
2. 检查推理耗时是否过长
3. 考虑降低抽帧频率

### 问题3：队列总是空的
1. 检查 Frame Extractor 是否正常工作
2. 确认MinIO扫描正常：查看日志 `images added to queue`
3. 验证MinIO连接：`./test_minio.sh`

## 性能调优实践

### 场景1：高清视频（4K）
```toml
[frame_extractor]
interval_ms = 500  # 每秒2帧

[ai_analysis]
max_concurrent_infer = 30  # 控制并发
```

### 场景2：多路视频（>10路）
```toml
[frame_extractor]
interval_ms = 200  # 每秒5帧

[ai_analysis]
max_concurrent_infer = 100  # 提高并发
scan_interval_sec = 0.5     # 加快扫描
```

### 场景3：实时性要求高
```toml
[frame_extractor]
interval_ms = 100  # 每秒10帧

[ai_analysis]
max_concurrent_infer = 150
scan_interval_sec = 0.5
```

## 总结

智能推理系统已帮您解决：
✅ 抽帧速度与推理速度不匹配
✅ 存储空间无限增长
✅ 推理队列积压
✅ 缺乏性能监控和告警

开箱即用，无需额外配置！🚀

