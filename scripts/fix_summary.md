# 修复总结

## 问题1：清零统计功能需要把总抽帧数量一块清零

### 修复内容

1. **在 `internal/plugin/frameextractor/service.go` 中添加 `ResetFrameStats()` 方法**：
   - 重置 `totalFrames`（总抽帧数量）
   - 重置 `framesPerSec`（每秒抽帧数）
   - 重置 `countInWindow`（窗口内计数）
   - 重置时间窗口

2. **在 `internal/plugin/aianalysis/service.go` 的 `ResetInferenceStats()` 方法中调用抽帧统计重置**：
   - 当清零推理统计时，同时清零抽帧统计
   - 确保两个统计系统同步重置

### 修改文件
- `internal/plugin/frameextractor/service.go` - 添加 `ResetFrameStats()` 方法
- `internal/plugin/aianalysis/service.go` - 在 `ResetInferenceStats()` 中调用抽帧统计重置

### 效果
- 点击"清零统计"按钮时，会同时清零：
  - AI推理统计数据（processed_total, dropped_total等）
  - 抽帧统计数据（total_frames, frames_per_sec等）

---

## 问题2：排查算法服务掉线问题

### 问题分析

从日志分析发现：
1. **心跳超时**：算法服务心跳超时（默认60秒）会被自动移除
2. **服务未找到**：心跳请求到达时，服务可能已被移除，导致"service not found"错误
3. **注册去重**：按endpoint去重可能导致服务重新注册时旧服务被移除

### 修复内容

1. **增强心跳错误日志**：
   - 当心跳请求到达但服务未找到时，记录警告日志
   - 提示服务需要重新注册

2. **增强注册日志**：
   - 记录按endpoint去重的操作
   - 说明这是正常现象（服务重新注册时）

3. **增强心跳超时检查**：
   - 添加接近超时预警（80%超时时间）
   - 记录更详细的超时信息（注册时间、最后心跳时间）
   - 提示服务可以重新注册

4. **增强健康状态报告**：
   - 记录接近超时的服务数量
   - 提供更详细的诊断信息

### 修改文件
- `internal/plugin/aianalysis/registry.go` - 增强心跳和注册日志

### 新增日志

1. **心跳未找到服务**：
   ```
   "heartbeat received but service not found by endpoint/service_id"
   "service may need to re-register"
   ```

2. **服务重新注册**：
   ```
   "removed duplicate service by endpoint before registering new one"
   "this is normal when service re-registers"
   ```

3. **接近超时预警**：
   ```
   "algorithm service heartbeat age is high (near timeout)"
   "service may be experiencing network issues or high load"
   ```

4. **超时详情**：
   ```
   "register_time": "2025-11-04 03:20:00"
   "last_heartbeat_time": "2025-11-04 03:21:00"
   ```

### 排查建议

1. **检查算法服务心跳间隔**：
   - 确保算法服务每30秒发送一次心跳（建议间隔为超时时间的50%）
   - 检查网络延迟和丢包情况

2. **检查算法服务注册**：
   - 确保所有算法服务都正确注册
   - 检查注册时的endpoint是否正确

3. **监控日志**：
   - 关注"heartbeat age is high"警告
   - 关注"service expired"日志
   - 检查是否有网络问题导致心跳丢失

4. **配置建议**：
   - 如果网络不稳定，可以增加 `heartbeat_timeout_sec`（默认60秒）
   - 确保算法服务的心跳发送间隔小于超时时间的一半

### 可能原因

1. **网络问题**：网络延迟或丢包导致心跳丢失
2. **算法服务负载过高**：处理请求时无法及时发送心跳
3. **算法服务崩溃**：服务异常退出，无法发送心跳
4. **时间同步问题**：服务器时间不同步导致心跳时间判断错误

### 验证方法

1. **查看服务状态**：
   ```bash
   curl http://localhost:5066/api/v1/ai_analysis/services | python3 -m json.tool
   ```

2. **查看日志**：
   ```bash
   grep -E "(heartbeat|register|expired)" logs/20251117_*.log | tail -50
   ```

3. **监控心跳年龄**：
   - 关注"heartbeat age is high"警告
   - 如果频繁出现，说明服务可能有问题

