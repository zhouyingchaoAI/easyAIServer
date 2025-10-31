# 并发处理问题修复说明

## 问题诊断

### 症状
- 队列积压严重：1967/2000 (98.35%)
- 丢弃率：14.88%
- 虽然配置了 `max_concurrent_infer = 100`，但并发处理能力上不来

### 根本原因
**只有1个 worker goroutine 在处理队列**

虽然配置了 `max_concurrent_infer = 100`，但代码中只启动了一个 `inferenceProcessLoop()` goroutine：

```go
// 修复前：只有1个worker
go s.inferenceProcessLoop()
```

`semaphore` 只限制同时进行的 HTTP 请求数，不限制 worker 数量。因为只有一个 worker，它必须串行处理，每次只能处理一个请求。

## 修复方案

### 修改内容
启动多个 worker goroutine 并行处理队列：

```go
// 修复后：启动多个worker
workerCount := s.cfg.MaxConcurrentInfer
for i := 0; i < workerCount; i++ {
    go s.inferenceProcessLoop()
}
```

### 工作原理

1. **Worker 数量** = `max_concurrent_infer` (当前为 100)
2. **每个 worker** 独立从队列取数据并处理
3. **Semaphore 限流** 继续限制同时进行的 HTTP 请求数（防止过多并发请求）

### 架构说明

```
队列 (2000)
  ↓
  ├─→ Worker 1 → Semaphore → HTTP Request → 算法服务
  ├─→ Worker 2 → Semaphore → HTTP Request → 算法服务
  ├─→ Worker 3 → Semaphore → HTTP Request → 算法服务
  ...
  └─→ Worker 100 → Semaphore → HTTP Request → 算法服务
```

- **Worker 并行度**：100个worker同时从队列取数据
- **HTTP 并发限制**：semaphore 限制最多100个同时进行的HTTP请求
- **队列吞吐量**：从串行处理提升到并行处理，理论上可提升100倍

## 配置说明

### 当前配置
```toml
[ai_analysis]
max_concurrent_infer = 100  # 同时启动100个worker
max_queue_size = 2000        # 队列容量2000
```

### 调整建议

**Worker数量（max_concurrent_infer）**：
- 小规模：10-20（1-2路视频）
- 中等规模：50-100（5-10路视频）
- 大规模：100-200（20+路视频）

**注意事项**：
- Worker数量过多可能导致goroutine过多，增加调度开销
- 实际并发受算法服务处理能力限制
- 建议根据实际推理速度调整

## 验证方法

重启服务后查看日志，应该看到：

```json
{
  "level": "info",
  "msg": "starting inference workers",
  "worker_count": 100,
  "max_concurrent_infer": 100
}
```

观察队列状态：
```bash
curl http://localhost:5066/api/v1/ai_analysis/inference_stats
```

预期效果：
- 队列利用率下降（多个worker同时消费）
- 丢弃率降低
- 处理速度显著提升

## 相关文件

- `internal/plugin/aianalysis/service.go` - 修改启动多个worker
- `internal/plugin/aianalysis/scheduler.go` - Semaphore限流（保持不变）
- `configs/config.toml` - 配置 `max_concurrent_infer`

## 历史记录

- **2025-10-31**: 修复并发处理问题，启动多个worker goroutine
  - 之前：只有1个worker串行处理
  - 现在：根据配置启动多个worker并行处理
