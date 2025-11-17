# 队列为空但算法无请求 - 不改代码的解决方案

## 问题根本原因

1. **时间窗口问题**：
   - Worker从队列Pop图片后，检查图片存在
   - 启动goroutine异步处理
   - Goroutine获取semaphore时可能等待很久（300个并发都被占用）
   - 在等待期间，图片被Frame Extractor清理（max_frame_count=200，只保留40秒）
   - 获取到semaphore后，发现图片不存在，直接return
   - 导致大量goroutine在等待处理不存在的图片

2. **队列为空的原因**：
   - 新图片入队后，立即被300个worker取走
   - Worker检查时图片存在，启动goroutine
   - 但goroutine在等待semaphore时，图片已被清理
   - 队列看起来为空，但实际有大量积压的异步任务

## 解决方案（不改代码）

### 方案1：增加图片保留时间（推荐，立即生效）

**原理**：增加max_frame_count，让图片保留更长时间，给推理留出足够时间。

**优点**：
- 简单直接，无需改代码
- 立即生效
- 给推理留出足够时间

**缺点**：
- 增加MinIO存储压力
- 治标不治本

**实现步骤**：

1. 修改 `configs/config.toml`：
```toml
# 找到 [frame_extractor] 部分
[frame_extractor]
max_frame_count = 1000  # 从200增加到1000，保留约200秒的图片
```

2. 如果任务级也有配置，也需要修改：
```toml
[[frame_extractor.tasks]]
id = '测试1'
max_frame_count = 1000  # 从默认值增加到1000
```

3. 重启服务：
```bash
cd /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511171346
./stop.sh
./start.sh
```

**预期效果**：
- 图片保留时间从40秒增加到200秒
- 给推理留出足够时间
- 减少"image not found"错误

### 方案2：降低抽帧频率（临时方案）

**原理**：降低抽帧频率，减少图片产生速度，让推理能跟上。

**优点**：
- 简单直接
- 减少系统负载

**缺点**：
- 降低监控频率
- 可能遗漏重要事件

**实现步骤**：

1. 修改 `configs/config.toml`：
```toml
[[frame_extractor.tasks]]
id = '测试1'
interval_ms = 1000  # 从200增加到1000，降低抽帧频率
```

2. 重启服务

**预期效果**：
- 抽帧速度从5fps降低到1fps
- 减少图片产生速度
- 让推理能跟上

### 方案3：重启服务，清空积压（紧急恢复）

**原理**：重启服务，清空当前队列和积压的异步任务，让系统重新开始。

**优点**：
- 立即清空积压
- 让系统重新开始

**缺点**：
- 会丢失当前队列中的图片
- 需要重启服务

**实现步骤**：

1. 停止服务：
```bash
cd /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511171346
./stop.sh
```

2. 等待几秒，确保服务完全停止

3. 启动服务：
```bash
./start.sh
```

**预期效果**：
- 清空当前队列
- 清空积压的异步任务
- 系统重新开始处理新图片

### 方案4：增加算法服务数量（长期方案）

**原理**：增加算法服务实例数，提高处理速度。

**优点**：
- 提高处理速度
- 减少积压

**缺点**：
- 需要额外的硬件资源
- 需要部署更多算法服务

**实现步骤**：

1. 部署更多算法服务实例
2. 确保算法服务注册到系统
3. 系统会自动负载均衡

**预期效果**：
- 提高推理处理速度
- 减少队列积压
- 减少图片被清理的情况

## 推荐执行顺序

1. **立即执行**：方案1（增加max_frame_count到1000）
2. **如果方案1不够**：方案2（降低抽帧频率）
3. **紧急情况**：方案3（重启服务）
4. **长期优化**：方案4（增加算法服务数量）

## 验证方法

执行方案后，检查以下指标：

1. **队列大小**：
```bash
curl -s http://localhost:5066/api/v1/ai_analysis/inference_stats | python3 -m json.tool | grep queue_size
```
应该看到队列大小逐渐增加，而不是一直为0

2. **推理成功率**：
```bash
curl -s http://localhost:5066/api/v1/ai_analysis/inference_stats | python3 -m json.tool | grep success_rate_per_sec
```
应该看到推理速度提升

3. **图片缺失错误**：
```bash
grep -c "image not found in MinIO" logs/20251117_08_00_00.log.4
```
应该看到错误数量减少

4. **算法服务调用**：
```bash
curl -s http://localhost:5066/api/v1/ai_analysis/services | python3 -m json.tool | grep call_count
```
应该看到call_count持续增长

