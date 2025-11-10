# 推理队列大小配置说明

## 概述

推理队列大小现在可以通过配置文件进行调整，以适配不同规模的使用场景。

## 配置方式

### 1. 配置文件位置

- 源配置: `configs/config.toml`
- 运行时配置: `build/EasyDarwin-*/configs/config.toml`

### 2. 配置项

在 `[ai_analysis]` 部分添加：

```toml
[ai_analysis]
enable = true
scan_interval_sec = 5
max_concurrent_infer = 100
max_queue_size = 500  # 推理队列最大容量
save_only_with_detection = true
alert_base_path = 'alerts/'
```

### 3. 配置说明

- **max_queue_size**: 推理队列的最大容量
  - 默认值: `100`（未配置或 ≤ 0 时使用）
  - 告警阈值: 自动设置为 `max_queue_size / 2`
  - 建议值: 根据实际视频路数和抽帧频率调整

## 配置建议

### 规模评估

```
小规模:  1-2路视频   → 100-200
中等规模: 5-10路视频  → 500-1000
大规模:  20+路视频   → 2000-5000
```

### 计算公式

```
max_queue_size = 抽帧频率(Hz) × 视频路数 × 扫描间隔(5s) × 2

示例:
  1Hz × 10路 × 5s × 2 = 100
  2Hz × 20路 × 5s × 2 = 400
```

## 生效方式

1. **编辑配置**: 修改 `configs/config.toml` 中的 `max_queue_size`
2. **重新编译**: 运行 `make` 或 `go build ./cmd/server`
3. **重启服务**: 启动新的可执行文件

## 验证方式

启动后查看日志，确认队列配置生效：

```json
{
  "level": "info",
  "msg": "AI analysis plugin started successfully",
  "queue_max_size": 500,
  "alert_threshold": 250,
  "queue_strategy": "drop_oldest"
}
```

## 注意事项

### ⚠️ 重要提示

1. **仅增大队列不能解决根本问题**
   - 如果算法服务不可达（connection refused），即使队列再大也会持续丢弃图片
   - 需要确保算法服务正常运行且网络连通

2. **内存占用**
   - 队列大小直接影响内存占用
   - 每张图片占用约为 50-200KB（取决于分辨率）
   - 500 张图片约占用 25-100MB

3. **响应延迟**
   - 队列过大可能导致处理延迟增加
   - 建议结合实际推理速度调整

### 当前环境问题

根据日志分析：
- ❌ 所有算法服务连接被拒绝 (`connection refused`)
- ⚠️  图片丢弃率高达 98.3%
- 📉 队列始终满载 (99/100)

**解决方案优先级**：
1. 🔴 **高优先级**: 启动算法服务或修复网络连接
2. 🟡 **中优先级**: 增大队列缓解临时压力
3. 🟢 **低优先级**: 优化推理性能

## 相关文件

- 配置结构: `internal/conf/model.go` → `AIAnalysisConfig`
- 队列初始化: `internal/plugin/aianalysis/service.go`
- 队列实现: `internal/plugin/aianalysis/queue.go`
- 配置文件: `configs/config.toml`

## 历史记录

- **2025-10-31**: 添加 `max_queue_size` 可配置支持
- 之前: 队列大小硬编码为 100
- 现在: 支持通过配置文件动态调整

