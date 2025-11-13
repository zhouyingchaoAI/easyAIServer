# 平均推理时间计算说明

## 概述

EasyDarwin 的 AI 分析插件会监控和统计推理性能，其中**平均推理时间**是一个重要的性能指标。

## 计算方式

### 1. 推理时间记录

推理时间在以下两个位置被记录：

#### 位置 1：推理处理循环（`service.go`）

```go
// inferenceProcessLoop 推理处理循环
func (s *Service) inferenceProcessLoop() {
    for {
        // 从队列取出图片
        img, ok := s.queue.Pop()
        if !ok {
            time.Sleep(100 * time.Millisecond)
            continue
        }
        
        // 记录开始时间
        startTime := time.Now()
        
        // 调度推理
        s.scheduler.ScheduleInference(img)
        
        // 记录推理时间（从调度开始到结束的总时间）
        inferenceTime := time.Since(startTime).Milliseconds()
        s.monitor.RecordInference(inferenceTime, true)
    }
}
```

**说明**：这里记录的是从调用 `ScheduleInference` 开始到返回的**总耗时**，包括：
- 算法服务调用时间
- 图片处理时间
- 告警保存时间
- 其他处理时间

#### 位置 2：算法服务调用（`scheduler.go`）

```go
// 记录推理开始时间
inferStartTime := time.Now()

// 调用算法服务
resp, err := s.callAlgorithm(algorithm, req)

// 计算实际推理耗时
actualInferenceTime := time.Since(inferStartTime).Milliseconds()

// 使用算法服务返回的推理时间，如果为0则使用实际测量的时间
reportedTimeMs := int64(resp.InferenceTimeMs)
if reportedTimeMs <= 0 {
    reportedTimeMs = actualInferenceTime
}
s.registry.RecordInferenceSuccess(algorithm.Endpoint, reportedTimeMs)
```

**说明**：这里记录的是**算法服务实际推理时间**，优先使用算法服务返回的 `inference_time_ms`，如果为 0 则使用实际测量的时间。

### 2. 平均推理时间计算

平均推理时间在 `monitor.go` 中通过**累积平均**的方式计算：

```go
// RecordInference 记录一次推理
func (m *PerformanceMonitor) RecordInference(inferenceTimeMs int64, success bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if !success {
        m.failedInferences++
        return  // 失败的不计入时间统计
    }
    
    m.totalInferences++
    m.totalInferenceTime += inferenceTimeMs
    
    // 计算平均推理时间：总时间 / 总次数
    m.avgInferenceTime = float64(m.totalInferenceTime) / float64(m.totalInferences)
    
    // ... 其他逻辑
}
```

**计算公式**：
```
平均推理时间 = 总推理时间 / 成功推理次数
```

**特点**：
- ✅ **累积统计**：从服务启动开始累积所有成功的推理
- ✅ **只统计成功**：失败的推理不计入时间统计，只记录失败次数
- ✅ **实时更新**：每次成功推理后立即更新平均值
- ✅ **线程安全**：使用互斥锁保护，支持并发访问

## 统计指标

### 性能监控器提供的指标

通过 `GetStats()` 可以获取以下统计信息：

```go
{
    "total_count":         总推理次数（成功）
    "failed_count":        失败次数
    "total_inference_time": 总推理时间（毫秒）
    "avg_inference_ms":    平均推理时间（毫秒）⭐
    "max_inference_ms":    最大推理时间（毫秒）
    "inference_per_sec":   推理速率（次/秒）
    "slow_count":          慢推理次数
    "slow_threshold_ms":   慢推理阈值（默认5000ms）
}
```

### 推理速率计算

```go
inferPerSec := 0.0
if m.avgInferenceTime > 0 {
    inferPerSec = 1000.0 / m.avgInferenceTime
}
```

**公式**：`推理速率 = 1000 / 平均推理时间（毫秒）`

例如：
- 平均推理时间 200ms → 推理速率 = 1000/200 = 5 次/秒
- 平均推理时间 100ms → 推理速率 = 1000/100 = 10 次/秒

## 使用场景

### 1. 性能监控

平均推理时间用于：
- 监控系统性能
- 识别性能瓶颈
- 优化算法服务

### 2. 慢推理告警

当推理时间超过阈值（默认 5000ms）时，会触发告警：

```go
if inferenceTimeMs > m.slowThresholdMs {
    m.slowCount++
    m.checkSlowInferenceAlertLocked(inferenceTimeMs)
}
```

### 3. 采样率计算

用于计算推荐的采样率，避免推理速度跟不上抽帧速度：

```go
// 计算抽帧速率和推理速率
framesPerSec := 1000.0 / float64(frameIntervalMs)
inferPerSec := 1000.0 / m.avgInferenceTime

if inferPerSec >= framesPerSec*0.9 {
    return 1  // 推理能力充足（>=90%），全部处理
}

// 计算需要的采样率
ratio := framesPerSec / inferPerSec
samplingRate := int(ratio) + 1
```

## 注意事项

### 1. 时间统计范围

- **监控器统计**：从调度开始到结束的**总时间**（包括所有处理步骤）
- **注册中心统计**：算法服务的**实际推理时间**（仅算法执行时间）

### 2. 失败不计入

失败的推理不计入平均时间统计，只记录失败次数。

### 3. 累积统计

平均推理时间是**累积统计**，从服务启动开始计算，不会自动重置。

如需重置统计，可以调用：
```go
service.monitor.Reset()
```

### 4. 线程安全

所有统计操作都是线程安全的，支持多个 worker 并发记录。

## API 查询

### 查询性能统计

```bash
curl http://127.0.0.1:5066/api/v1/ai_analysis/performance/stats
```

**响应示例**：
```json
{
  "queue": {
    "queue_size": 100,
    "max_size": 5000,
    "dropped_total": 50,
    "utilization": 0.02
  },
  "performance": {
    "total_count": 1000,
    "failed_count": 5,
    "total_inference_time": 200000,
    "avg_inference_ms": 200.0,
    "max_inference_ms": 1500,
    "inference_per_sec": 5.0,
    "slow_count": 2,
    "slow_threshold_ms": 5000
  },
  "drop_rate": 0.05,
  "healthy": true
}
```

## 总结

- **计算方式**：累积平均 = 总推理时间 / 成功推理次数
- **统计范围**：从调度开始到结束的总时间
- **更新时机**：每次成功推理后立即更新
- **失败处理**：失败不计入时间统计
- **线程安全**：支持并发访问

平均推理时间是系统性能的重要指标，可以帮助识别性能瓶颈和优化方向。

