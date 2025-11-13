# 推理停止问题诊断报告

## 问题现象

推理服务在 `2025-11-12 04:07:41` 后停止，当前时间 `2025-11-12 07:59:36`，已停止约 4 小时。

## 根本原因

**所有算法服务因心跳超时被自动注销，导致没有可用的算法服务进行推理。**

### 证据

1. **API 查询结果**：`/api/v1/ai_analysis/services` 返回服务数量为 0
2. **日志分析**：
   - 最后推理日志：`2025-11-12 04:07:41.328`
   - 最后扫描日志：`2025-11-12 04:07:41.022`
   - 大量 `context canceled` 错误，说明算法服务响应超时
   - 错误信息提示：`service will be removed by heartbeat timeout if truly offline`

3. **推理流程**：
   - 扫描器仍在工作（添加图片到队列）
   - 推理循环仍在运行
   - 但 `GetAlgorithmWithLoadBalance()` 返回 `nil`（没有可用服务）
   - 图片被删除，推理停止

## 问题分析

### 1. 心跳超时机制

- **配置**：`heartbeat_timeout_sec = 60`（60秒）
- **机制**：算法服务需要每 30 秒发送一次心跳
- **超时处理**：超过 60 秒未收到心跳，服务自动注销

### 2. 算法服务状态

从日志看，以下服务都出现了 `context canceled` 错误：
- `yolo11x_head_detector_7901` ~ `yolo11x_head_detector_7912`

这些服务可能：
1. 已停止运行
2. 网络连接中断
3. 响应超时（60秒超时）
4. 心跳发送失败

### 3. 推理停止流程

```
扫描器 → 添加图片到队列 → 推理循环获取图片
  ↓
查找算法服务 → GetAlgorithmWithLoadBalance() 返回 nil
  ↓
没有可用服务 → 删除图片 → 继续等待
  ↓
推理停止（没有服务可调用）
```

## 解决方案

### 方案 1：重启算法服务（推荐）

1. **检查算法服务状态**：
   ```bash
   # 检查服务是否运行
   ps aux | grep yolo
   # 或
   netstat -tlnp | grep 790
   ```

2. **重启算法服务**：
   - 确保所有算法服务正常运行
   - 服务会自动重新注册到 EasyDarwin

3. **验证注册**：
   ```bash
   curl http://127.0.0.1:5066/api/v1/ai_analysis/services
   ```

### 方案 2：检查网络连接

1. **测试算法服务连通性**：
   ```bash
   for port in 7901 7902 7903 7904 7905 7906 7907 7908 7909 7910 7911 7912; do
     echo "Testing 172.16.5.207:$port"
     curl -m 5 http://172.16.5.207:$port/health 2>&1 || echo "Failed"
   done
   ```

2. **检查防火墙规则**：
   - 确保 EasyDarwin 可以访问算法服务
   - 确保算法服务可以访问 EasyDarwin

### 方案 3：调整心跳超时配置

如果算法服务响应较慢，可以增加心跳超时时间：

```toml
[ai_analysis]
heartbeat_timeout_sec = 120  # 增加到 120 秒
```

**注意**：需要重启 EasyDarwin 使配置生效。

### 方案 4：检查算法服务心跳发送

算法服务需要定期发送心跳：

```bash
# 每 30 秒发送一次心跳
curl -X POST http://127.0.0.1:5066/api/v1/ai_analysis/heartbeat/yolo11x_head_detector_7901
```

确保算法服务的心跳发送逻辑正常工作。

## 预防措施

### 1. 监控算法服务状态

定期检查已注册的服务：
```bash
curl http://127.0.0.1:5066/api/v1/ai_analysis/services | jq '.total'
```

### 2. 设置告警

- 当服务数量为 0 时发送告警
- 监控推理队列积压情况
- 监控算法服务响应时间

### 3. 日志监控

关注以下日志：
- `algorithm service unregistered` - 服务注销
- `heartbeat timeout` - 心跳超时
- `no algorithm for task type` - 没有可用服务

## 快速恢复步骤

1. **检查算法服务**：
   ```bash
   # 检查服务进程
   ps aux | grep -E "yolo|algorithm"
   ```

2. **重启算法服务**（如果已停止）

3. **等待服务注册**（通常几秒内完成）

4. **验证推理恢复**：
   ```bash
   # 查看最新日志
   tail -f /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511120326/logs/20251112_00_00_00.log | grep -E "scheduling inference|收到推理请求"
   ```

5. **检查服务注册**：
   ```bash
   curl http://127.0.0.1:5066/api/v1/ai_analysis/services | jq '.total'
   ```

## 相关配置

当前配置（`config.toml`）：
```toml
[ai_analysis]
enable = true
scan_interval_sec = 1
mq_type = 'kafka'
mq_address = '172.16.5.207:9092'
mq_topic = 'easyai.alerts'
heartbeat_timeout_sec = 60  # 心跳超时时间（秒）
max_concurrent_infer = 100
max_queue_size = 5000
```

## 总结

推理停止的根本原因是**所有算法服务因心跳超时被注销**。解决方案是：
1. 确保算法服务正常运行
2. 确保算法服务正确发送心跳
3. 确保网络连接正常
4. 必要时调整心跳超时配置

推理循环本身仍在运行，一旦有算法服务注册，推理会自动恢复。

