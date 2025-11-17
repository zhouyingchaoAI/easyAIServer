# 队列为空但算法无请求 - 完整解决方案

## 问题根本原因

1. **时间窗口问题**：
   - Worker从队列Pop图片后，检查图片存在（第331行）
   - 启动goroutine异步处理（第353行）
   - Goroutine获取semaphore时可能等待很久（300个并发都被占用）
   - 在等待期间，图片被Frame Extractor清理（max_frame_count=200，只保留40秒）
   - 获取到semaphore后，发现图片不存在，直接return
   - 导致大量goroutine在等待处理不存在的图片

2. **队列为空的原因**：
   - 新图片入队后，立即被300个worker取走
   - Worker检查时图片存在，启动goroutine
   - 但goroutine在等待semaphore时，图片已被清理
   - 队列看起来为空，但实际有大量积压的异步任务

## 解决方案（按优先级）

### 方案1：在启动goroutine前立即标记为正在推理（推荐）

**原理**：在Pop后检查图片存在后，立即标记为"正在推理"，这样Frame Extractor清理时会跳过这些图片。

**优点**：
- 确保队列中的图片不会被清理
- 不需要修改清理逻辑
- 立即生效

**实现位置**：`internal/plugin/aianalysis/service.go` 的 `inferenceProcessLoop` 方法

**修改逻辑**：
```go
// 在检查图片存在后，立即标记为正在推理
if exists {
    // 立即标记为正在推理，保护图片不被清理
    s.scheduler.MarkImageInferring(img.Path)
    
    // 然后启动goroutine
    go func(image ImageInfo) {
        // ... 原有逻辑
    }(img)
}
```

### 方案2：增加图片保留时间（临时方案）

**原理**：增加max_frame_count，让图片保留更长时间。

**实现**：修改 `configs/config.toml`
```toml
max_frame_count = 1000  # 从200增加到1000，保留约200秒的图片
```

### 方案3：优化semaphore获取时机

**原理**：在goroutine内部，先检查图片是否存在，再获取semaphore。

**优点**：
- 避免占用semaphore处理不存在的图片
- 提高semaphore利用率

**实现位置**：`internal/plugin/aianalysis/scheduler.go` 的 `ScheduleInference` 方法

### 方案4：清空队列，重新开始（紧急恢复）

**原理**：清空当前队列，让系统重新开始处理新图片。

**实现**：需要添加API接口或重启服务

## 推荐执行顺序

1. **立即执行**：方案2（增加max_frame_count到1000）- 无需改代码
2. **短期修复**：方案1（在启动goroutine前立即标记）- 需要改代码
3. **长期优化**：方案3（优化semaphore获取时机）- 需要改代码

