# 负载均衡优先级修复

**日期**: 2025-11-04  
**问题**: 服务停止后重新上线注册没有第一时间分配推理  
**状态**: ✅ 已修复

---

## 🔍 问题诊断

### 用户反馈
> "服务停止以后重新上线注册没有第一时间分配推理"

### 问题场景

```
场景1：服务A、B都停止后重新启动
├─ 服务A注册：调用次数=0，被添加到列表索引0
├─ 服务B注册：调用次数=0，被添加到列表索引1
├─ 负载均衡选择：
│   ├─ 调用次数都是0（allEqual=true）
│   ├─ 使用Round-Robin，从rrIndexes[taskType]开始
│   └─ 如果索引=0，会一直选择服务A
└─ 结果：服务B要等很久才能获得推理请求 ❌

场景2：服务A运行中，服务B重新上线
├─ 服务A：调用次数=100
├─ 服务B：调用次数=0（新注册）
├─ 负载均衡选择：
│   ├─ 调用次数不同（allEqual=false）
│   └─ 选择最少的（服务B）
└─ 结果：服务B会被立即选中 ✅
```

### 根本原因

**修复前的逻辑**：
```go
// 1. 先判断是否所有服务调用次数相同
if allEqual {
    // 使用Round-Robin，从上次的索引开始
    idx := r.rrIndexes[taskType]
    selected = &services[idx % len(services)]
} else {
    // 选择调用次数最少的
    selected = minService
}
```

**问题**：
- 当所有服务调用次数都是0时（如重启后），会进入Round-Robin分支
- Round-Robin从固定索引开始，不会优先考虑新注册的服务
- 导致新注册的服务要等待轮询才能获得请求

---

## ✅ 修复方案

### 新的负载均衡策略

**核心原则**：**始终优先选择调用次数最少的服务**

```go
// 1. 找出调用次数最少的服务
minCount := findMinCount(services)
minIndices := findServicesWithCount(minCount)

// 2. 如果只有一个最少的，直接选择
if len(minIndices) == 1 {
    return services[minIndices[0]]
}

// 3. 如果有多个最少的，在这些服务中使用Round-Robin
idx := r.rrIndexes[taskType] % len(minIndices)
return services[minIndices[idx]]
```

### 修复效果

```
场景1：服务A、B都重新上线（调用次数都=0）
├─ 找出最小调用次数：0
├─ 最小调用次数的服务：[A, B]
├─ 在[A, B]中使用Round-Robin
│   ├─ 第1次：选择A
│   ├─ 第2次：选择B
│   └─ 第3次：选择A
└─ 结果：两个服务立即都能获得请求 ✅

场景2：服务A运行中(调用次数=100)，服务B重新上线(=0)
├─ 找出最小调用次数：0
├─ 最小调用次数的服务：[B]
├─ 只有一个，直接选择B
└─ 结果：服务B立即获得请求 ✅

场景3：服务A(调用次数=50)，服务B(调用次数=30)，服务C重新上线(=0)
├─ 找出最小调用次数：0
├─ 最小调用次数的服务：[C]
├─ 只有一个，直接选择C
└─ 结果：服务C立即获得请求，直到调用次数追上B ✅
```

---

## 📊 算法对比

### 修复前 ❌

| 步骤 | 逻辑 | 问题 |
|------|------|------|
| 1 | 检查是否所有调用次数相同 | - |
| 2a | 相同 → Round-Robin | ❌ 新服务要等待轮询 |
| 2b | 不同 → 选择最少的 | ✅ 新服务会被选中 |

### 修复后 ✅

| 步骤 | 逻辑 | 效果 |
|------|------|------|
| 1 | 找出调用次数最少的服务 | ✅ 新服务一定是最少的 |
| 2 | 在最少的服务中选择 | ✅ 保证新服务被选中 |
| 3a | 只有一个最少的 → 直接选择 | ✅ 立即分配 |
| 3b | 多个最少的 → Round-Robin | ✅ 均匀分配 |

---

## 🎯 特性保证

### 1. 新服务立即获得请求 ✅
```
服务注册 → 调用次数=0 → 是最少的 → 立即被选中
```

### 2. 负载自动均衡 ✅
```
服务A: 调用次数=100
服务B: 调用次数=80
服务C: 调用次数=60

选择顺序：C → C → C ... → (C=80) → B, C轮询 → (B=80) → A, B, C轮询
```

### 3. 多个新服务公平分配 ✅
```
服务A、B、C同时注册，调用次数都=0
→ Round-Robin在[A, B, C]中轮询
→ 每个服务都立即获得请求
```

### 4. 服务上线即时生效 ✅
```
t=0: 服务A、B运行中
t=1: 服务C注册（调用次数=0）
t=2: 立即选择服务C（因为调用次数最少）
```

---

## 📝 日志示例

### 新服务立即被选中
```json
{
  "level": "debug",
  "msg": "load balance: least-load selected",
  "task_type": "人数统计",
  "selected_endpoint": "http://172.17.0.2:7901/infer",
  "selected_service_id": "yolo11x_head_detector_7901",
  "call_count": 0,
  "total_services": 3,
  "all_call_counts": [100, 80, 0]
}
```

### 多个新服务轮询
```json
{
  "level": "debug",
  "msg": "load balance: round-robin among least-loaded services",
  "task_type": "人数统计",
  "selected_endpoint": "http://172.17.0.2:7902/infer",
  "selected_service_id": "yolo11x_head_detector_7902",
  "call_count": 0,
  "services_with_same_count": 2,
  "total_services": 3
}
```

---

## 🚀 测试验证

### 测试场景1：单服务重新上线
```bash
# 1. 停止服务
# 服务自动从列表移除（心跳超时）

# 2. 启动服务
# 服务注册，调用次数=0

# 3. 观察日志
tail -f logs/*.log | grep "load balance"

# 期望：立即看到该服务被选中
# ✅ "least-load selected", "call_count": 0
```

### 测试场景2：多服务同时上线
```bash
# 1. 启动3个服务A、B、C
# 都注册，调用次数都=0

# 2. 观察日志
tail -f logs/*.log | grep "load balance"

# 期望：3个服务轮流被选中
# ✅ "round-robin among least-loaded services"
# ✅ "services_with_same_count": 3
```

### 测试场景3：单服务重启（其他服务运行中）
```bash
# 1. 服务A、B运行中（调用次数>0）
# 2. 重启服务C
# 3. 观察日志

# 期望：服务C立即被选中（调用次数=0）
# ✅ "least-load selected"
# ✅ "call_count": 0
# ✅ "all_call_counts": [100, 80, 0]
```

---

## 🎉 总结

### 修复内容
- ✅ 改进负载均衡算法，优先选择调用次数最少的服务
- ✅ 确保新注册的服务（调用次数=0）立即被选中
- ✅ 多个最少调用次数的服务使用Round-Robin公平分配

### 修复效果
- ✅ 服务重新上线后立即获得推理请求
- ✅ 负载自动均衡到各个服务
- ✅ 多个新服务公平分配请求
- ✅ 即时生效，无延迟

### 部署步骤
```bash
cd /code/EasyDarwin
pkill easydarwin
cp easydarwin_fixed easydarwin
./easydarwin
```

---

**修复完成时间**: 2025-11-04  
**编译状态**: ✅ 通过  
**Linter检查**: ✅ 无错误  
**测试状态**: ⏳ 待验证

