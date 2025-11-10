# 算法服务问题修复报告

**日期**: 2025-11-04  
**修复版本**: v8.3.4  
**修复人**: AI Assistant

---

## 📋 问题概述

### 问题1：调用次数统计不准确 ❌

**现象**：
- 算法服务面板显示"调用成功次数"为1506次
- 但实际上所有推理请求都失败了（connection refused）
- 日志显示大量 `algorithm call failed after 3 retries` 错误

**根本原因**：
- 负载均衡在**选择算法时**就增加了调用计数
- 即使后续HTTP调用失败，计数器也已经增加了
- 导致显示的是"尝试次数"而不是"成功次数"

### 问题2：服务注册成功但推理端点不可用 ⚠️

**现象**：
- 服务注册成功，心跳正常
- 但推理端点 `http://172.17.0.2:7901/infer` 无法连接
- 系统没有自动剔除不可用的服务

---

## 🔧 修复方案

### 修复1：分离负载均衡选择和计数逻辑 ✅

**文件**: `internal/plugin/aianalysis/registry.go`

**修改内容**：
```go
// 修改前：在选择时就增加计数
func GetAlgorithmWithLoadBalance(taskType string) *AlgorithmService {
    // ... 选择逻辑 ...
    r.callCounters[endpoint]++  // ❌ 问题：无论后续是否成功都增加了
    return selected
}

// 修改后：只负责选择，不增加计数
func GetAlgorithmWithLoadBalance(taskType string) *AlgorithmService {
    // ... 选择逻辑 ...
    return selected  // ✅ 不增加计数，由调用方决定
}
```

**新增方法**：
- `RecordInferenceSuccess(endpoint)`: 记录推理成功（增加成功计数，重置失败计数）
- `RecordInferenceFailure(endpoint, serviceID)`: 记录推理失败（增加失败计数）

### 修复2：在推理成功后才增加计数 ✅

**文件**: `internal/plugin/aianalysis/scheduler.go`

**修改内容**：
```go
// 调用算法服务
resp, err := s.callAlgorithm(algorithm, req)
if err != nil {
    // ❌ 失败：记录失败次数
    s.registry.RecordInferenceFailure(algorithm.Endpoint, algorithm.ServiceID)
    return
}

if !resp.Success {
    // ❌ 失败：不增加计数
    return
}

// ✅ 成功：才增加调用计数
s.registry.RecordInferenceSuccess(algorithm.Endpoint)
```

### 修复3：添加服务健康检查机制 ✅

**新增功能**：
- 记录每个服务的连续失败次数
- 连续失败**5次**后自动注销服务
- 成功一次即重置失败计数

**新增字段**：
```go
type AlgorithmRegistry struct {
    // ...
    failureCounters map[string]int  // endpoint -> consecutive failure count
    maxFailures     int             // 默认5次
}
```

**工作流程**：
```
第1次失败 → 记录失败 (1/5)
第2次失败 → 记录失败 (2/5)
第3次失败 → 记录失败 (3/5)
第4次失败 → 记录失败 (4/5)
第5次失败 → 自动注销服务 ⚠️  → 从服务列表中移除 → 不再分配新请求

成功一次 → 重置失败计数 (0/5)
```

---

## 📊 修复效果

### 修复前 ❌
```
- 调用次数显示: 1506次（错误）
- 实际成功: 0次
- 不可用服务: 一直保留在列表中
- 负载均衡: 继续分配请求给失败的服务
```

### 修复后 ✅
```
- 调用次数显示: 只统计成功的调用（准确）
- 失败服务: 连续5次失败后自动注销
- 服务列表: 实时更新，自动剔除不可用服务
- 负载均衡: 只在可用服务间分配
```

---

## 🚀 部署步骤

### 1. 停止当前服务
```bash
# 停止EasyDarwin
pkill easydarwin

# 或者使用systemd
sudo systemctl stop easydarwin
```

### 2. 备份旧版本
```bash
cd /code/EasyDarwin
cp easydarwin easydarwin.bak.20251104
```

### 3. 替换新版本
```bash
# 使用修复后的版本
cp easydarwin_fixed easydarwin
chmod +x easydarwin
```

### 4. 启动服务
```bash
# 直接启动
./easydarwin

# 或者使用systemd
sudo systemctl start easydarwin
```

### 5. 验证修复效果
```bash
# 查看日志
tail -f ./build/EasyDarwin-aarch64-v8.3.3-202511030857/logs/20251104_00_00_00.log

# 检查服务列表
curl http://localhost:5066/api/v1/ai_analysis/services | jq
```

---

## 📝 验证要点

### 1. 调用次数准确性
- ✅ 只有推理成功时才增加计数
- ✅ 失败的调用不计入"调用成功次数"

### 2. 服务健康检查
- ✅ 连续5次失败后自动注销
- ✅ 日志输出：`service auto-unregistering due to consecutive failures`
- ✅ 服务列表实时更新

### 3. 负载均衡即时生效
- ✅ 服务上线：立即加入负载均衡
- ✅ 服务下线：立即从负载均衡中移除
- ✅ 服务失败：累计5次后自动剔除

---

## 🔍 日志关键词

### 成功场景
```
✅ "inference succeeded, call count incremented"
✅ "inference success recorded"
```

### 失败场景
```
⚠️  "inference failure recorded"
⚠️  "consecutive_failures: 1"
⚠️  "consecutive_failures: 5"
❌ "service auto-unregistering due to consecutive failures"
❌ "service auto-unregistered"
```

---

## ⚙️ 配置项

### 心跳超时
```toml
[ai_analysis]
heartbeat_timeout_sec = 10  # 心跳超时时间（秒）
```

### 失败阈值
当前硬编码为5次，后续可配置化：
```go
// 在 registry.go 中
maxFailures: 5  // 连续失败5次后自动注销
```

---

## 🐛 已知问题处理

### 问题：算法服务注册成功但端点不可用

**原因**：
- 算法服务进程发送心跳，但推理端点（7901端口）没有监听
- 可能是服务进程异常，或者端口配置错误

**解决方案**：
1. 检查算法服务进程是否正常运行
2. 检查端口是否正确监听
3. 查看算法服务日志排查问题
4. 系统会在5次失败后自动注销该服务

---

## 📌 最佳实践

### 算法服务开发建议

1. **健康检查端点**：
```python
@app.route('/health')
def health():
    return {"status": "ok", "service_id": SERVICE_ID}
```

2. **推理端点**：
```python
@app.route('/infer', methods=['POST'])
def infer():
    # 确保端点可用，能正确处理请求
    pass
```

3. **心跳机制**：
```python
# 每30秒发送一次心跳
while True:
    send_heartbeat()
    time.sleep(30)
```

4. **监控日志**：
```bash
# 监控推理请求
tail -f algorithm_service.log | grep "infer"
```

---

## 📞 技术支持

如有问题，请查看：
1. EasyDarwin日志: `./logs/*.log`
2. 算法服务日志: 各服务的日志文件
3. 系统日志: `journalctl -u easydarwin -f`

---

## ✅ 修复确认清单

- [x] 调用次数只统计成功的调用
- [x] 失败的调用不计入统计
- [x] 连续失败5次后自动注销
- [x] 服务列表实时更新
- [x] 负载均衡即时生效
- [x] 没有linter错误
- [x] 代码编译通过

**修复完成时间**: 2025-11-04  
**状态**: ✅ 已完成，待测试验证

