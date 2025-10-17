# 功能优化：队列丢弃图片同步删除MinIO文件

## 📝 功能概述

优化推理队列的图片丢弃逻辑：当队列满了需要丢弃图片时，同步删除MinIO中对应的图片文件，避免存储空间浪费。

## ✨ 优化内容

### 问题描述

**原有问题：**
- 当推理队列积压严重时，会丢弃部分图片
- 丢弃的图片只是从队列中移除，但MinIO中的文件依然保留
- 长期运行会积累大量未处理的图片文件，浪费存储空间

**优化方案：**
- 丢弃图片时，同步删除MinIO中的图片文件
- 异步删除，不阻塞队列操作
- 支持所有丢弃策略（丢弃最旧/最新/清空队列）

### 丢弃策略说明

#### 1. StrategyDropOldest（丢弃最旧的）- 默认策略

**触发条件**：队列已满，新图片到来

**行为：**
1. 移除队列中最旧的图片
2. 删除MinIO中对应的图片文件
3. 新图片加入队列

```
队列: [图1, 图2, 图3] (已满)
新图片: 图4
↓
丢弃: 图1
删除: MinIO中的图1文件
队列: [图2, 图3, 图4]
```

#### 2. StrategyDropNewest（丢弃最新的）

**触发条件**：队列已满，新图片到来

**行为：**
1. 拒绝新图片加入队列
2. 删除MinIO中新图片文件
3. 队列保持不变

```
队列: [图1, 图2, 图3] (已满)
新图片: 图4
↓
丢弃: 图4
删除: MinIO中的图4文件
队列: [图1, 图2, 图3] (不变)
```

#### 3. StrategyLatestOnly（只保留最新的）

**触发条件**：队列已满，需要清空

**行为：**
1. 清空队列中所有图片
2. 批量删除MinIO中对应的图片文件
3. 新图片加入空队列

```
队列: [图1, 图2, 图3, ..., 图100] (已满)
新图片: 图101
↓
清空: 图1~图100
批量删除: MinIO中的图1~图100文件
队列: [图101]
```

## 🔧 技术实现

### 队列结构增强

```go
type InferenceQueue struct {
    images           []ImageInfo
    maxSize          int
    strategy         QueueStrategy
    // ... 其他字段
    
    // 新增字段
    minio            *minio.Client  // MinIO客户端
    bucket           string          // MinIO bucket
    deleteDropped    bool            // 是否删除丢弃的图片
}
```

### 删除函数

```go
// 异步删除MinIO中的图片
func (q *InferenceQueue) deleteImageFromMinIO(img ImageInfo) {
    if q.minio == nil || q.bucket == "" {
        return
    }
    
    // 使用goroutine异步删除，不阻塞队列操作
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        
        err := q.minio.RemoveObject(ctx, q.bucket, img.Path, minio.RemoveObjectOptions{})
        if err != nil {
            q.log.Warn("failed to delete dropped image from MinIO",
                slog.String("path", img.Path),
                slog.String("err", err.Error()))
            return
        }
        
        q.log.Debug("dropped image deleted from MinIO",
            slog.String("path", img.Path))
    }()
}
```

### 丢弃逻辑

```go
switch q.strategy {
case StrategyDropOldest:
    dropped := q.images[0]
    q.images = q.images[1:]
    q.droppedCount++
    
    // 删除MinIO中的图片
    if q.deleteDropped {
        q.deleteImageFromMinIO(dropped)
    }
    
case StrategyDropNewest:
    q.droppedCount++
    
    // 删除MinIO中的图片
    if q.deleteDropped {
        q.deleteImageFromMinIO(img)
    }
    continue
    
case StrategyLatestOnly:
    oldImages := make([]ImageInfo, len(q.images))
    copy(oldImages, q.images)
    q.images = q.images[:0]
    q.droppedCount += int64(len(oldImages))
    
    // 批量删除MinIO中的图片
    if q.deleteDropped {
        for _, droppedImg := range oldImages {
            q.deleteImageFromMinIO(droppedImg)
        }
    }
}
```

## 📊 存储空间节省

### 场景分析

假设：
- 队列容量：100张
- 抽帧频率：每秒5帧
- 推理速度：每秒2张（慢于抽帧）
- 持续时间：1小时
- 图片大小：300KB/张

#### 优化前

```
总抽帧数：18,000 张
队列处理：7,200 张
队列丢弃：10,800 张

MinIO存储：
- 保留：18,000 张图片（所有抽帧）
- 大小：5.4 GB

问题：
❌ 10,800张丢弃的图片仍占用存储空间
❌ MinIO中大量未处理的图片
❌ 浪费 3.24 GB 存储空间
```

#### 优化后

```
总抽帧数：18,000 张
队列处理：7,200 张
队列丢弃：10,800 张

MinIO存储：
- 保留：7,200 张图片（已处理的）
- 删除：10,800 张图片（丢弃的）
- 大小：2.16 GB

优势：
✅ 丢弃的图片立即删除
✅ MinIO只保留有效图片
✅ 节省 3.24 GB 存储空间（60%）
```

## 📋 日志示例

### 丢弃最旧的图片

```log
WARN queue full, dropped oldest image 
  task_type=人数统计 
  task_id=task_1 
  image=frame_001.jpg 
  queue_size=99 
  total_dropped=1

DEBUG dropped image deleted from MinIO 
  path=人数统计/task_1/frame_001.jpg 
  task_type=人数统计 
  task_id=task_1
```

### 丢弃最新的图片

```log
WARN queue full, dropped newest image 
  image=frame_100.jpg

DEBUG dropped image deleted from MinIO 
  path=人数统计/task_1/frame_100.jpg
```

### 清空队列

```log
WARN queue full, cleared for latest images 
  cleared=100

DEBUG dropped image deleted from MinIO 
  path=人数统计/task_1/frame_001.jpg
DEBUG dropped image deleted from MinIO 
  path=人数统计/task_1/frame_002.jpg
... (批量删除)
```

### 删除失败

```log
WARN failed to delete dropped image from MinIO 
  path=人数统计/task_1/frame_001.jpg 
  err=object not found
```

## ⚙️ 配置说明

### 当前配置

队列丢弃时删除MinIO文件功能**默认启用**，在Service初始化时设置：

```go
// internal/plugin/aianalysis/service.go
s.queue = NewInferenceQueue(
    100,                    // 最大队列容量
    StrategyDropOldest,     // 丢弃最旧的策略
    50,                     // 积压50张告警
    minioClient,            // MinIO客户端
    s.fxCfg.MinIO.Bucket,   // MinIO bucket
    true,                   // ← 丢弃图片时删除MinIO文件
    s.log,
)
```

### 如需关闭功能

如果特殊场景需要保留丢弃的图片，可以修改代码：

```go
s.queue = NewInferenceQueue(
    100,
    StrategyDropOldest,
    50,
    minioClient,
    s.fxCfg.MinIO.Bucket,
    false,  // ← 关闭删除功能
    s.log,
)
```

## 🎯 适用场景

### ✅ 推荐启用（默认）

1. **生产环境**
   - 长期运行的系统
   - 存储空间有限
   - 关注成本控制

2. **推理速度慢于抽帧**
   - 队列经常积压
   - 频繁丢弃图片
   - 需要及时清理

3. **高并发场景**
   - 多路视频流
   - 大量图片积压
   - 需要快速清理

### ⚠️ 考虑关闭

1. **调试阶段**
   - 需要查看所有图片
   - 分析丢弃原因
   - 验证队列逻辑

2. **存储充足**
   - 磁盘空间大
   - 不关心成本
   - 需要完整记录

## 🔍 监控与验证

### 查看丢弃日志

```bash
# 实时查看丢弃的图片
tail -f logs/sugar.log | grep "queue full, dropped"

# 查看删除的图片
tail -f logs/sugar.log | grep "dropped image deleted"

# 统计今天丢弃的图片数
cat logs/sugar.log | grep "queue full, dropped" | grep $(date +%Y-%m-%d) | wc -l
```

### 验证MinIO清理

```bash
# 查看MinIO存储使用（删除前）
mc du local/images

# 等待一段时间后再次查看（删除后）
mc du local/images

# 应该看到存储空间减少
```

### 队列统计

```bash
# 通过API查看队列状态
curl http://localhost:5066/api/performance/stats | jq .queue

# 输出示例：
{
  "queue_size": 95,
  "max_size": 100,
  "dropped_total": 1250,  # 已丢弃总数
  "processed_total": 5400,
  "utilization": 0.95
}
```

## 💡 性能优化

### 异步删除

- 使用goroutine异步删除，不阻塞队列操作
- 删除失败不影响推理流程
- 超时时间10秒，避免长时间等待

### 批量删除

- `StrategyLatestOnly` 策略批量删除
- 每个删除操作独立goroutine
- 并发删除，提高效率

### 错误容忍

- 删除失败只记录警告日志
- 不影响队列正常运行
- 系统继续工作

## 🐛 故障排查

### 问题1：MinIO存储未减少

**原因**：删除失败或MinIO权限不足

**排查：**
```bash
# 查看删除失败日志
cat logs/sugar.log | grep "failed to delete dropped image"

# 测试MinIO连接
./scripts/test_minio.sh

# 检查MinIO权限
mc admin user list local
```

### 问题2：队列仍然满

**原因**：推理速度太慢，丢弃速度跟不上抽帧

**解决：**
1. 增加并发数：`max_concurrent_infer = 50`
2. 降低抽帧频率：`interval_ms = 2000`
3. 增加算法服务实例
4. 优化算法性能

### 问题3：日志中大量删除警告

**原因**：队列经常满，频繁丢弃

**解决：**
1. 增大队列容量（修改代码）
2. 优化推理性能
3. 调整抽帧策略

## 📊 对比总结

| 项目 | 优化前 | 优化后 | 改进 |
|------|--------|--------|------|
| 丢弃图片 | 保留在MinIO | 自动删除 | ✅ |
| 存储占用 | 包含丢弃图片 | 只有有效图片 | **节省60%+** |
| 性能影响 | 无 | 极小（异步） | 可忽略 |
| 操作便利 | 需手动清理 | 自动清理 | ✅ |

## 📖 相关文档

- [只保存有检测结果](FEATURE_SAVE_ONLY_WITH_DETECTION.md)
- [智能推理使用指南](SMART_INFERENCE_USAGE.md)
- [性能优化策略](OPTIMIZATION_STRATEGY.md)

---

**版本**：v1.2.1  
**更新日期**：2024-10-17  
**作者**：yanying team

