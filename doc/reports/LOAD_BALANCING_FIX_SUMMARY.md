# 负载均衡优化完成总结

## 问题描述
原始实现中，多个相同类型的算法实例（不同的endpoint）没有正确进行负载均衡，系统一直使用同一个推理端点。

## 根本原因

### 问题1：使用ServiceID而不是Endpoint作为唯一标识
- **原始设计**：`callCounters` 使用 `service_id` 作为key
- **问题**：同一算法的多个实例可能共享相同的 `service_id`，但拥有不同的 `endpoint`
- **结果**：多个实例的调用次数被统计在一起，无法正确负载均衡

### 问题2：计数和选择的竞态条件
- **原始设计**：先调用 `GetAlgorithmWithLoadBalance()` 选择实例，再调用 `IncrementCallCount()` 增加计数
- **问题**：这两个操作之间存在时间窗口，可能导致并发请求都选择了同一个实例
- **结果**：负载均衡失效

### 问题3：缺少自适应均衡策略
- **原始设计**：只使用"最小调用次数"策略
- **问题**：当所有实例调用次数相同时，总是选择第一个
- **结果**：无法实现真正的轮询分配

## 解决方案

### 修复1：使用Endpoint作为唯一标识
```go
// 负载均衡：记录每个算法实例的调用次数
// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
callCounters map[string]int // algorithm endpoint -> call count
```

**影响文件**：
- `internal/plugin/aianalysis/registry.go`
- `internal/plugin/aianalysis/scheduler.go`
- `internal/web/api/ai_analysis.go`

### 修复2：原子化选择+计数操作
```go
// GetAlgorithmWithLoadBalance 使用负载均衡策略选择一个算法实例并增加调用计数
// 注意：此函数内部会同时选择实例并递增计数，保证原子性
func (r *AlgorithmRegistry) GetAlgorithmWithLoadBalance(taskType string) *conf.AlgorithmService {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // ... 选择逻辑 ...
    
    // 增加选中实例的调用计数
    r.callCounters[endpoint]++
    
    return selected
}
```

**关键改进**：
- 在同一个锁内完成选择+计数操作
- 消除了竞态条件
- 保证负载统计的准确性

### 修复3：实现自适应负载均衡
```go
// 策略：自适应负载均衡
// 1. 如果所有实例调用次数相同，使用Round-Robin轮询
// 2. 否则选择调用次数最少的实例

// Round-Robin索引：每个任务类型的当前选择索引
rrIndexes map[string]int // task_type -> round-robin index
```

**策略说明**：
1. **初始化阶段**：所有实例负载为0，使用Round-Robin轮询，确保均匀分配
2. **运行阶段**：选择调用次数最少的实例，自动实现负载均衡
3. **重置机制**：服务注册/注销时重置Round-Robin索引，确保公平性

## 负载均衡机制

### Round-Robin轮询（负载相同时）
```
实例列表: [endpoint1, endpoint2, endpoint3]
调用序列: endpoint1 -> endpoint2 -> endpoint3 -> endpoint1 -> ...
```

### 最小负载选择（负载不同时）
```
实例负载: {endpoint1: 10, endpoint2: 8, endpoint3: 12}
选择结果: endpoint2 (最小)
```

## 日志输出

系统会输出详细的负载均衡日志，便于调试和监控：

```json
// Round-Robin模式
{
  "level": "debug",
  "msg": "load balance: using round-robin",
  "task_type": "人数统计",
  "endpoint": "http://172.17.0.2:7901/infer",
  "round_robin_index": 1,
  "call_count": 5
}

// 最小负载模式
{
  "level": "debug",
  "msg": "load balance: using least-load",
  "task_type": "人数统计",
  "endpoint": "http://172.17.0.3:7901/infer",
  "call_count": 3
}
```

## API变更

### 内部API
- `GetAlgorithmWithLoadBalance(taskType string)`: 现在内部会自动增加计数，无需外部调用
- `IncrementCallCount(endpoint string)`: 已废弃使用，但仍保留用于兼容性
- `GetCallCount(endpoint string)`: 参数从 `serviceID` 改为 `endpoint`

### 外部API
- `/api/v1/ai_analysis/services`: 返回每个endpoint的调用次数
- 前端显示：每个算法实例（按endpoint唯一标识）的调用次数

## 测试验证

### 测试场景
1. **单一实例**：直接使用，无负载均衡
2. **多个实例，相同负载**：使用Round-Robin轮询
3. **多个实例，不同负载**：使用最小负载选择
4. **动态添加/移除实例**：Round-Robin索引自动重置

### 验证方法
1. 查看日志中的 `load balance` 消息
2. 监控API返回的 `call_count` 字段
3. 观察多个实例的调用次数是否均匀分布

## 性能影响

- **锁竞争**：使用 `sync.RWMutex`，读多写少场景下性能优良
- **内存开销**：增加 `rrIndexes` map，每个 `task_type` 一个 int，开销可忽略
- **CPU开销**：负载均衡计算O(n)复杂度，n为实例数量，通常很小（<10个）

## 兼容性

- **向后兼容**：现有API和配置文件无需修改
- **数据兼容**：调用次数统计基于endpoint，旧数据的 `service_id` 映射会被自动废弃
- **前端兼容**：前端已更新为按endpoint显示调用次数

## 总结

通过以上三个关键修复，实现了：
1. ✅ 正确的多实例识别（基于endpoint）
2. ✅ 原子化的负载均衡操作（无竞态条件）
3. ✅ 自适应均衡策略（Round-Robin + 最小负载）

系统现在能够正确地在多个算法实例间进行负载均衡，确保推理请求的均匀分配和高效的资源利用率。

