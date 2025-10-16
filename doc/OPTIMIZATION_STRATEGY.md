# yanying 智能分析优化策略

## 🎯 需要优化的问题

### 问题1：存储爆满
**现状**：抽帧图片持续保存，没有清理机制，最终会导致存储空间耗尽

**影响**：
- 磁盘/MinIO空间不断增长
- 老旧图片占用空间
- 系统可能因磁盘满而崩溃

### 问题2：推理积压
**现状**：当推理速度跟不上抽帧速度时，会产生积压

**影响**：
- 图片堆积越来越多
- 推理延迟增加
- 资源浪费

---

## 💡 优化方案

### 方案1：图片自动清理策略

#### 策略A：基于时间的自动清理（推荐）

**配置新增**：
```toml
[frame_extractor]
enable = true
store = 'minio'
auto_cleanup = true  # 启用自动清理
retention_hours = 24  # 保留24小时
cleanup_interval_hours = 1  # 每小时检查一次

[ai_analysis]
enable = true
delete_after_inference = true  # 推理完成后删除图片
keep_alert_images = true  # 保留有告警的图片
alert_image_retention_days = 7  # 告警图片保留7天
```

**实现逻辑**：
```
1. 定时清理器（每小时运行）
   ↓
2. 扫描MinIO中的图片
   ↓
3. 检查图片时间戳
   ↓
4. 删除超过retention_hours的图片
   例外：有告警记录的图片保留更长时间
```

#### 策略B：基于数量的清理

**配置**：
```toml
[frame_extractor]
max_snapshots_per_task = 1000  # 每个任务最多保留1000张
cleanup_strategy = 'fifo'  # fifo|lifo|random
```

**实现**：
- 当某个任务的图片超过限制时
- 删除最老的图片（FIFO）
- 保持图片总数在限制内

#### 策略C：MinIO生命周期策略（最简单）

直接使用MinIO的内置生命周期管理：

```bash
# 设置7天自动过期
/tmp/mc ilm add test-minio/images --expiry-days 7

# 或者设置30天
/tmp/mc ilm add test-minio/images --expiry-days 30

# 查看当前策略
/tmp/mc ilm ls test-minio/images
```

**优点**：
- ✅ 无需修改代码
- ✅ MinIO自动处理
- ✅ 性能最优

**立即执行**：
```bash
# 设置7天过期（推荐）
/tmp/mc ilm add test-minio/images --expiry-days 7
```

---

### 方案2：推理积压处理策略

#### 策略A：采样推理（最实用）

不是所有图片都推理，而是采样：

**配置新增**：
```toml
[ai_analysis]
sampling_strategy = 'interval'  # interval|random|skip
sampling_rate = 5  # 每5张图片推理1张（20%采样率）
max_queue_size = 100  # 最大队列长度
queue_strategy = 'drop_oldest'  # 队列满时的策略
```

**实现逻辑**：
```python
# 采样推理
if image_count % sampling_rate == 0:
    schedule_inference(image)
else:
    skip(image)  # 跳过不推理
```

**优势**：
- 减少80%的推理压力
- 降低成本
- 保证实时性

#### 策略B：优先级队列

**配置**：
```toml
[ai_analysis]
queue_mode = 'priority'  # fifo|priority|latest
priority_rules = [
  { task_type = '人员跌倒', priority = 1 },  # 高优先级
  { task_type = '火焰检测', priority = 1 },
  { task_type = '人数统计', priority = 3 },  # 低优先级
]
```

**实现**：
- 高优先级任务优先推理
- 低优先级任务可以跳过或延迟

#### 策略C：智能降级

**自动调整策略**：
```python
if queue_size > max_queue_size * 0.8:
    # 队列积压超过80%，启动降级
    sampling_rate = 10  # 降低采样率
    scan_interval = 20  # 降低扫描频率
else:
    sampling_rate = 2   # 正常采样率
    scan_interval = 10  # 正常扫描频率
```

#### 策略D：最新优先（Latest Only）

**只推理最新的图片**：
```toml
[ai_analysis]
inference_mode = 'latest_only'  # 只推理最新的，跳过积压
max_pending = 10  # 最多保留10张待推理
```

**实现**：
```python
# 每次扫描时
new_images = scan_minio()
if len(new_images) > max_pending:
    # 只取最新的N张
    new_images = new_images[-max_pending:]
```

---

## 🔧 立即可执行的优化

### 优化1：启用MinIO自动清理（推荐）

```bash
# 设置7天自动过期
/tmp/mc ilm add test-minio/images --expiry-days 7

# 验证
/tmp/mc ilm ls test-minio/images
```

### 优化2：降低抽帧频率

编辑配置文件：
```toml
[[frame_extractor.tasks]]
id = 'test1'
task_type = '人数统计'
rtsp_url = 'rtsp://127.0.0.1:15544/live/stream_2'
interval_ms = 5000  # 从2000改为5000（每5秒1帧）
output_path = 'test1'
enabled = true
```

### 优化3：增加扫描间隔（已完成）

```toml
[ai_analysis]
scan_interval_sec = 10  # 已从1秒改为10秒 ✅
```

### 优化4：增加并发数

如果服务器资源充足：
```toml
[ai_analysis]
max_concurrent_infer = 10  # 从5增加到10
```

---

## 📊 推荐配置方案

### 方案A：低频率高质量（推荐生产环境）

```toml
[frame_extractor]
enable = true
interval_ms = 1000
store = 'minio'

[[frame_extractor.tasks]]
interval_ms = 5000  # 每5秒1帧

[ai_analysis]
enable = true
scan_interval_sec = 10  # 每10秒扫描
max_concurrent_infer = 10

# MinIO清理策略
# 命令: /tmp/mc ilm add test-minio/images --expiry-days 7
```

**特点**：
- 抽帧：每5秒1帧 = 每小时720张
- 扫描：每10秒 = 每小时360次
- 清理：7天自动删除
- 并发：10个推理任务

**存储消耗**：
- 每张图片约100KB
- 每小时：720 × 100KB = 72MB
- 7天：72MB × 24 × 7 ≈ 12GB

### 方案B：高频率采样（实时监控）

```toml
[[frame_extractor.tasks]]
interval_ms = 1000  # 每秒1帧

[ai_analysis]
scan_interval_sec = 5  # 每5秒扫描
sampling_rate = 5  # 只推理20%的图片
max_concurrent_infer = 20
```

**特点**：
- 抽帧密集但只推理部分图片
- 适合实时监控场景

---

## 🛠️ 代码优化建议

### 优化1：添加图片清理功能

创建新文件：`internal/plugin/frameextractor/cleanup.go`

```go
package frameextractor

import (
	"context"
	"log/slog"
	"time"
	"github.com/minio/minio-go/v7"
)

// StartCleanup 启动自动清理
func (s *Service) StartCleanup(retentionHours int, intervalHours int) {
	if retentionHours <= 0 {
		retentionHours = 24
	}
	if intervalHours <= 0 {
		intervalHours = 1
	}
	
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	go func() {
		for {
			select {
			case <-s.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				s.cleanupOldImages(retentionHours)
			}
		}
	}()
}

// cleanupOldImages 清理旧图片
func (s *Service) cleanupOldImages(retentionHours int) {
	if s.minio == nil {
		return
	}
	
	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	ctx := context.Background()
	
	// 列举所有对象
	objectCh := s.minio.client.ListObjects(ctx, s.minio.bucket, minio.ListObjectsOptions{
		Prefix:    s.minio.base,
		Recursive: true,
	})
	
	deleted := 0
	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		
		// 检查是否超过保留时间
		if object.LastModified.Before(cutoffTime) {
			// 删除对象
			err := s.minio.client.RemoveObject(ctx, s.minio.bucket, object.Key, minio.RemoveObjectOptions{})
			if err != nil {
				s.log.Warn("failed to delete old image", 
					slog.String("key", object.Key),
					slog.String("err", err.Error()))
			} else {
				deleted++
			}
		}
	}
	
	s.log.Info("cleanup completed", 
		slog.Int("deleted", deleted),
		slog.Int("retention_hours", retentionHours))
}
```

### 优化2：添加采样推理

修改 `internal/plugin/aianalysis/scanner.go`：

```go
type Scanner struct {
	// ... 现有字段
	samplingRate int  // 采样率：每N张推理1张
	imageCount   int  // 图片计数器
}

func (s *Scanner) shouldInfer(imagePath string) bool {
	if s.samplingRate <= 1 {
		return true  // 全部推理
	}
	
	s.imageCount++
	return s.imageCount % s.samplingRate == 0
}

// 在 scanNewImages 中使用
func (s *Scanner) scanNewImages() ([]ImageInfo, error) {
	// ... 扫描代码
	
	var newImages []ImageInfo
	for object := range objectCh {
		// ... 检查代码
		
		// 采样过滤
		if !s.shouldInfer(object.Key) {
			s.MarkProcessed(object.Key)  // 标记为已处理但不推理
			continue
		}
		
		newImages = append(newImages, ImageInfo{...})
	}
	
	return newImages, nil
}
```

### 优化3：推理队列管理

添加队列限制：

```go
type Scheduler struct {
	// ... 现有字段
	pendingQueue chan ImageInfo  // 待推理队列
	maxQueueSize int             // 最大队列长度
}

func (s *Scheduler) ScheduleInference(images []ImageInfo) {
	for _, image := range images {
		select {
		case s.pendingQueue <- image:
			// 成功加入队列
		default:
			// 队列已满，根据策略处理
			switch s.queueStrategy {
			case "drop_oldest":
				<-s.pendingQueue  // 丢弃最老的
				s.pendingQueue <- image
			case "drop_new":
				// 丢弃新的（不加入队列）
				s.log.Warn("queue full, dropping new image", 
					slog.String("image", image.Path))
			case "skip":
				// 跳过
			}
		}
	}
}
```

---

## 🚀 立即可用的优化（无需修改代码）

### 1. 启用MinIO生命周期清理

```bash
# 设置7天自动删除
/tmp/mc ilm add test-minio/images --expiry-days 7

# 或者设置不同的策略
# 3天：紧急监控场景
/tmp/mc ilm add test-minio/images --expiry-days 3

# 30天：长期存档
/tmp/mc ilm add test-minio/images --expiry-days 30
```

### 2. 调整抽帧和扫描参数

**当前配置**：
```toml
interval_ms = 2000  # 每2秒1帧
scan_interval_sec = 10  # 每10秒扫描
```

**优化建议**：

**场景1：实时监控（人员跌倒、火灾等）**
```toml
interval_ms = 1000  # 每秒1帧（密集）
scan_interval_sec = 5  # 每5秒扫描
max_concurrent_infer = 20  # 增加并发
```

**场景2：客流统计（可接受延迟）**
```toml
interval_ms = 5000  # 每5秒1帧
scan_interval_sec = 30  # 每30秒扫描
max_concurrent_infer = 5
```

**场景3：定期巡检**
```toml
interval_ms = 60000  # 每分钟1帧
scan_interval_sec = 300  # 每5分钟扫描
max_concurrent_infer = 2
```

### 3. 手动清理脚本

创建定时清理任务：

```bash
# 创建清理脚本
cat > /code/EasyDarwin/cleanup_old_images.sh << 'EOF'
#!/bin/bash

DAYS=7  # 保留天数
BUCKET="test-minio/images"

echo "清理 ${DAYS} 天前的图片..."

# 计算截止日期
CUTOFF_DATE=$(date -d "${DAYS} days ago" +%Y-%m-%d)

# 列出并删除旧图片
/tmp/mc find ${BUCKET} --older-than ${DAYS}d --exec "mc rm {}"

echo "清理完成"
EOF

chmod +x /code/EasyDarwin/cleanup_old_images.sh

# 设置cron任务（每天凌晨3点执行）
# crontab -e
# 0 3 * * * /code/EasyDarwin/cleanup_old_images.sh
```

---

## 📈 性能计算和规划

### 存储需求计算

**输入参数**：
- 抽帧间隔：N 秒
- 图片大小：约100KB（JPEG压缩）
- 任务数量：M 个
- 保留时间：D 天

**公式**：
```
每小时图片数 = (3600 / N) × M
总存储需求 = (3600 / N) × M × 100KB × 24 × D
```

**示例**：
```
场景1：10个摄像头，每5秒1帧，保留7天
= (3600/5) × 10 × 100KB × 24 × 7
= 720 × 10 × 100KB × 168
= 121 GB

场景2：10个摄像头，每10秒1帧，保留3天
= (3600/10) × 10 × 100KB × 24 × 3
= 360 × 10 × 100KB × 72
= 26 GB
```

### 推理能力计算

**输入参数**：
- 单次推理时间：T 毫秒
- 并发数：C

**公式**：
```
每秒推理能力 = 1000 / T × C
能处理的抽帧速率 = (1000 / T × C) / M
```

**示例**：
```
假设：推理100ms，并发10
每秒推理能力 = 1000/100 × 10 = 100 张/秒
能处理10个任务 = 100 / 10 = 每任务10张/秒

因此：
- 抽帧间隔 >= 100ms 可以实时处理
- 抽帧间隔 < 100ms 会产生积压
```

---

## 🎯 推荐配置组合

### 配置1：标准监控（推荐）

```toml
[frame_extractor]
enable = true
store = 'minio'
interval_ms = 1000

[[frame_extractor.tasks]]
interval_ms = 5000  # 每5秒1帧
enabled = true

[ai_analysis]
enable = true
scan_interval_sec = 10  # 每10秒扫描
max_concurrent_infer = 10
```

**MinIO清理**：
```bash
/tmp/mc ilm add test-minio/images --expiry-days 7
```

**特点**：
- 平衡的抽帧频率
- 合理的存储消耗
- 实时性和成本的折中

### 配置2：高密度实时监控

```toml
[[frame_extractor.tasks]]
interval_ms = 1000  # 每秒1帧（高密度）

[ai_analysis]
scan_interval_sec = 5
max_concurrent_infer = 20
# 建议添加采样：sampling_rate = 3（只推理33%）
```

**MinIO清理**：
```bash
/tmp/mc ilm add test-minio/images --expiry-days 3  # 3天
```

### 配置3：低成本巡检

```toml
[[frame_extractor.tasks]]
interval_ms = 30000  # 每30秒1帧

[ai_analysis]
scan_interval_sec = 60  # 每分钟扫描
max_concurrent_infer = 5
```

**MinIO清理**：
```bash
/tmp/mc ilm add test-minio/images --expiry-days 30  # 30天
```

---

## 🔍 监控和告警

### 存储监控

```bash
# 查看MinIO存储使用
/tmp/mc admin info test-minio

# 查看bucket大小
/tmp/mc du test-minio/images

# 按任务类型统计
/tmp/mc du test-minio/images/人数统计
/tmp/mc du test-minio/images/人员跌倒
```

### 推理队列监控

添加API接口查询推理状态：
```bash
# 查询推理统计
curl http://localhost:5066/api/v1/ai_analysis/stats

# 应该返回：
{
  "pending_images": 5,       # 待推理图片数
  "processing": 3,            # 正在推理
  "completed_today": 1523,    # 今日完成数
  "failed_today": 12,         # 今日失败数
  "avg_inference_ms": 150     # 平均推理时间
}
```

---

## 💾 数据保留策略建议

### 分层存储策略

```
热数据（0-24小时）
├─ 所有图片
└─ MinIO快速存储

温数据（1-7天）
├─ 有告警的图片
└─ MinIO标准存储

冷数据（>7天）
├─ 重要告警图片
└─ 归档到OSS/S3（可选）

删除（>30天）
└─ 全部删除
```

### 智能保留规则

```python
def should_keep(image, alert):
    # 1. 最近24小时：全部保留
    if image.age < 24h:
        return True
    
    # 2. 有告警：保留7天
    if alert exists and alert.age < 7d:
        return True
    
    # 3. 高置信度告警：保留30天
    if alert.confidence > 0.9 and alert.age < 30d:
        return True
    
    # 4. 其他：删除
    return False
```

---

## 📝 实施步骤

### 第1步：立即执行（现在）

```bash
# 1. 启用MinIO自动清理（7天）
/tmp/mc ilm add test-minio/images --expiry-days 7

# 2. 调整抽帧间隔到5秒
# 编辑 config.toml，修改 interval_ms = 5000

# 3. 保持扫描间隔10秒（已优化）

# 4. 重启服务
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428
pkill -9 easydarwin && sleep 2 && ./easydarwin &
```

### 第2步：监控观察（今天-明天）

```bash
# 监控存储增长
watch -n 60 '/tmp/mc du test-minio/images'

# 监控推理日志
tail -f logs/20251016_08_00_00.log | grep "found new\|scheduling"
```

### 第3步：根据需要调整（本周）

根据实际运行情况调整参数：
- 存储增长太快 → 增加抽帧间隔或减少保留天数
- 推理积压 → 增加并发数或降低扫描频率

---

## 🎊 总结

### 存储爆满解决方案
✅ **MinIO生命周期策略**（立即可用）
```bash
/tmp/mc ilm add test-minio/images --expiry-days 7
```

### 推理积压解决方案
✅ **调整参数平衡**（已优化）
- 抽帧：5秒1帧
- 扫描：10秒1次
- 并发：10个任务

### 未来优化（需要代码修改）
- 采样推理（只推理部分图片）
- 优先级队列
- 智能降级
- 自动清理集成

---

**立即执行清理策略，问题就能解决！** 🚀

