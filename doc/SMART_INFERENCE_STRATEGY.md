# 智能推理策略 - 自适应推理和告警

## 🎯 设计目标

### 核心需求

1. **根据抽帧频率配置推理频率**
   - 抽帧快 → 推理也要快（采样或增加并发）
   - 抽帧慢 → 推理可以慢（节省资源）

2. **推理太慢时告警**
   - 监测推理队列积压情况
   - 超过阈值发送告警
   - 记录性能指标

3. **自动丢弃处理不完的图片**
   - 设置队列最大长度
   - 超过容量丢弃旧图片
   - 保留最新的图片优先处理

---

## 🏗️ 系统架构设计

### 当前架构

```
抽帧器 → MinIO → 扫描器 → 调度器 → 推理服务
         (无限堆积)   (全部处理)  (可能积压)
```

**问题**：
- ❌ 图片无限堆积
- ❌ 推理队列无限增长
- ❌ 无法感知积压

### 优化架构

```
抽帧器 → MinIO → 扫描器 → 智能队列 → 调度器 → 推理服务
   ↓        ↓        ↓         ↓         ↓
 5张/秒   自动清理  采样过滤   优先队列   性能监控
                              最大100张   ↓
                              超过丢弃   告警系统
```

**优势**：
- ✅ 队列有上限，不会无限增长
- ✅ 自动丢弃旧图片
- ✅ 实时性能监控和告警
- ✅ 自适应调整

---

## 💡 实现方案

### 方案1：配置化的智能队列（推荐）

#### 配置参数

```toml
[ai_analysis]
enable = true
scan_interval_sec = 1  # 扫描间隔

# 智能队列配置
queue_mode = 'smart'  # smart|fifo|priority
max_queue_size = 100  # 队列最大长度
queue_full_strategy = 'drop_oldest'  # drop_oldest|drop_newest|skip
sampling_enabled = true  # 启用采样
sampling_rate = 1  # 采样率（1=全部，2=50%，5=20%）

# 性能监控和告警
performance_monitoring = true  # 启用性能监控
backlog_alert_threshold = 50  # 积压超过50张告警
slow_inference_threshold_ms = 5000  # 推理超过5秒告警
alert_interval_sec = 60  # 告警间隔（避免频繁告警）

max_concurrent_infer = 50  # 最大并发数
```

#### 工作逻辑

```python
class SmartInferenceQueue:
    def __init__(self, max_size=100, strategy='drop_oldest'):
        self.max_size = max_size
        self.strategy = strategy
        self.queue = []
        self.dropped_count = 0
        self.last_alert_time = 0
        
    def add_images(self, images):
        """添加图片到队列"""
        for img in images:
            if len(self.queue) >= self.max_size:
                # 队列已满，执行丢弃策略
                if self.strategy == 'drop_oldest':
                    dropped = self.queue.pop(0)  # 丢弃最旧的
                    self.dropped_count += 1
                    self.log_dropped(dropped)
                elif self.strategy == 'drop_newest':
                    # 丢弃新的（不加入队列）
                    self.dropped_count += 1
                    self.log_dropped(img)
                    continue
                elif self.strategy == 'skip':
                    continue
            
            self.queue.append(img)
        
        # 检查是否需要告警
        self.check_backlog_alert()
    
    def check_backlog_alert(self):
        """检查积压并告警"""
        if len(self.queue) > self.backlog_alert_threshold:
            now = time.time()
            if now - self.last_alert_time > self.alert_interval_sec:
                self.send_alert({
                    'type': 'backlog',
                    'queue_size': len(self.queue),
                    'threshold': self.backlog_alert_threshold,
                    'dropped_total': self.dropped_count,
                    'message': f'推理队列积压{len(self.queue)}张，已丢弃{self.dropped_count}张'
                })
                self.last_alert_time = now
```

---

### 方案2：采样推理（按抽帧频率）

#### 自动计算采样率

```python
def calculate_sampling_rate(frame_interval_ms, avg_inference_ms):
    """
    根据抽帧频率和推理速度自动计算采样率
    
    例如：
    - 抽帧：每200ms = 5张/秒
    - 推理：每500ms = 2张/秒
    - 采样率：5/2 ≈ 3（每3张处理1张）
    """
    frames_per_sec = 1000 / frame_interval_ms
    infer_per_sec = 1000 / avg_inference_ms
    
    if infer_per_sec >= frames_per_sec:
        return 1  # 推理够快，全部处理
    else:
        return int(frames_per_sec / infer_per_sec) + 1
```

#### 配置示例

```toml
[ai_analysis]
auto_sampling = true  # 启用自动采样
target_inference_ratio = 0.8  # 目标：推理能力应≥抽帧速度的80%
adjust_interval_sec = 60  # 每60秒调整一次采样率
```

#### 工作流程

```
1. 启动时：采样率=1（全部处理）
   ↓
2. 每60秒评估：
   - 统计抽帧速度：5张/秒
   - 统计推理速度：2张/秒
   - 计算采样率：5/2 ≈ 3
   ↓
3. 应用新采样率：
   - 每3张图片只推理1张
   - 其他2张标记为"已跳过"
   ↓
4. 继续监控，动态调整
```

---

### 方案3：优先级丢弃

#### 配置

```toml
[ai_analysis]
queue_mode = 'priority'
max_queue_size = 100

# 优先级规则
[[ai_analysis.priority_rules]]
task_type = '人员跌倒'
priority = 1  # 最高优先级（永不丢弃）

[[ai_analysis.priority_rules]]
task_type = '火焰检测'
priority = 1

[[ai_analysis.priority_rules]]
task_type = '人数统计'
priority = 3  # 低优先级（可以丢弃）
```

#### 丢弃逻辑

```python
def drop_low_priority_images(queue, max_size):
    """丢弃低优先级图片"""
    if len(queue) <= max_size:
        return
    
    # 按优先级排序
    queue.sort(key=lambda x: (x.priority, x.timestamp))
    
    # 保留高优先级和最新的
    keep_count = max_size
    to_keep = []
    to_drop = []
    
    # 优先保留priority=1的
    for img in queue:
        if img.priority == 1:
            to_keep.append(img)
        elif len(to_keep) < keep_count:
            to_keep.append(img)
        else:
            to_drop.append(img)
    
    # 丢弃低优先级
    for img in to_drop:
        log.info(f'丢弃低优先级图片: {img.path}')
    
    return to_keep
```

---

## 🔧 代码实现建议

### 修改文件：`internal/plugin/aianalysis/queue.go`（新建）

```go
package aianalysis

import (
	"log/slog"
	"sync"
	"time"
)

// InferenceQueue 智能推理队列
type InferenceQueue struct {
	images          []ImageInfo
	maxSize         int
	strategy        string // drop_oldest|drop_newest|skip
	mu              sync.RWMutex
	droppedCount    int64
	lastAlertTime   time.Time
	alertThreshold  int
	alertInterval   time.Duration
	log             *slog.Logger
}

// NewInferenceQueue 创建队列
func NewInferenceQueue(maxSize int, strategy string, alertThreshold int, logger *slog.Logger) *InferenceQueue {
	return &InferenceQueue{
		images:         make([]ImageInfo, 0, maxSize),
		maxSize:        maxSize,
		strategy:       strategy,
		alertThreshold: alertThreshold,
		alertInterval:  60 * time.Second,
		log:            logger,
	}
}

// Add 添加图片到队列
func (q *InferenceQueue) Add(images []ImageInfo) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	for _, img := range images {
		// 检查队列是否已满
		if len(q.images) >= q.maxSize {
			switch q.strategy {
			case "drop_oldest":
				// 丢弃最旧的
				dropped := q.images[0]
				q.images = q.images[1:]
				q.droppedCount++
				q.log.Warn("queue full, dropped oldest image",
					slog.String("dropped", dropped.Path),
					slog.Int("queue_size", len(q.images)))
			case "drop_newest":
				// 丢弃新的（不加入）
				q.droppedCount++
				q.log.Warn("queue full, dropped newest image",
					slog.String("dropped", img.Path))
				continue
			case "skip":
				// 跳过
				continue
			}
		}
		
		q.images = append(q.images, img)
	}
	
	// 检查积压告警
	q.checkBacklogAlert()
}

// Pop 取出一张图片
func (q *InferenceQueue) Pop() (ImageInfo, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if len(q.images) == 0 {
		return ImageInfo{}, false
	}
	
	img := q.images[0]
	q.images = q.images[1:]
	return img, true
}

// Size 获取队列大小
func (q *InferenceQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.images)
}

// checkBacklogAlert 检查积压告警
func (q *InferenceQueue) checkBacklogAlert() {
	if len(q.images) <= q.alertThreshold {
		return
	}
	
	now := time.Now()
	if now.Sub(q.lastAlertTime) < q.alertInterval {
		return  // 避免频繁告警
	}
	
	q.lastAlertTime = now
	q.log.Error("inference backlog alert",
		slog.Int("queue_size", len(q.images)),
		slog.Int("threshold", q.alertThreshold),
		slog.Int64("dropped_total", q.droppedCount),
		slog.String("message", "推理队列积压，请增加并发数或降低抽帧频率"))
	
	// TODO: 发送系统告警（邮件/短信/webhook）
}

// GetStats 获取统计信息
func (q *InferenceQueue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	return map[string]interface{}{
		"queue_size":     len(q.images),
		"max_size":       q.maxSize,
		"dropped_total":  q.droppedCount,
		"utilization":    float64(len(q.images)) / float64(q.maxSize),
	}
}
```

---

### 修改文件：`internal/plugin/aianalysis/monitor.go`（新建）

```go
package aianalysis

import (
	"log/slog"
	"sync"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	frameRate         float64  // 抽帧速率（张/秒）
	inferenceRate     float64  // 推理速率（张/秒）
	avgInferenceTime  float64  // 平均推理时间（毫秒）
	
	totalInferences   int64
	totalInferenceTime int64
	mu                sync.RWMutex
	
	slowThresholdMs   int64
	lastSlowAlert     time.Time
	log               *slog.Logger
}

// NewPerformanceMonitor 创建监控器
func NewPerformanceMonitor(slowThresholdMs int64, logger *slog.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		slowThresholdMs: slowThresholdMs,
		log:            logger,
	}
}

// RecordInference 记录一次推理
func (m *PerformanceMonitor) RecordInference(inferenceTimeMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalInferences++
	m.totalInferenceTime += inferenceTimeMs
	m.avgInferenceTime = float64(m.totalInferenceTime) / float64(m.totalInferences)
	
	// 检查是否推理太慢
	if inferenceTimeMs > m.slowThresholdMs {
		now := time.Now()
		if now.Sub(m.lastSlowAlert) > 60*time.Second {
			m.lastSlowAlert = now
			m.log.Warn("slow inference detected",
				slog.Int64("inference_time_ms", inferenceTimeMs),
				slog.Int64("threshold_ms", m.slowThresholdMs),
				slog.Float64("avg_time_ms", m.avgInferenceTime))
			// TODO: 发送告警
		}
	}
}

// CalculateSamplingRate 计算推荐的采样率
func (m *PerformanceMonitor) CalculateSamplingRate(frameIntervalMs int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.avgInferenceTime == 0 {
		return 1  // 初始全部处理
	}
	
	framesPerSec := 1000.0 / float64(frameIntervalMs)
	inferPerSec := 1000.0 / m.avgInferenceTime
	
	if inferPerSec >= framesPerSec {
		return 1  // 推理够快，全部处理
	}
	
	// 计算采样率：需要跳过多少张
	ratio := framesPerSec / inferPerSec
	samplingRate := int(ratio) + 1
	
	m.log.Info("calculated sampling rate",
		slog.Float64("frames_per_sec", framesPerSec),
		slog.Float64("infer_per_sec", inferPerSec),
		slog.Int("sampling_rate", samplingRate))
	
	return samplingRate
}

// GetStats 获取统计信息
func (m *PerformanceMonitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"total_inferences":   m.totalInferences,
		"avg_inference_ms":   m.avgInferenceTime,
		"frame_rate":         m.frameRate,
		"inference_rate":     m.inferenceRate,
	}
}
```

---

### 修改文件：`internal/plugin/aianalysis/service.go`

在Start方法中集成智能队列：

```go
func (s *Service) Start() error {
	// ... 现有代码
	
	// 创建智能队列
	queue := NewInferenceQueue(
		100,              // 最大100张
		"drop_oldest",    // 丢弃旧的
		50,               // 积压50张告警
		s.log,
	)
	
	// 创建性能监控器
	monitor := NewPerformanceMonitor(
		5000,  // 推理超过5秒告警
		s.log,
	)
	
	// 启动扫描和处理循环
	go func() {
		ticker := time.NewTicker(time.Duration(s.cfg.ScanIntervalSec) * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				// 扫描MinIO
				newImages, err := s.scanner.scanNewImages()
				if err != nil {
					continue
				}
				
				// 添加到队列（自动丢弃）
				queue.Add(newImages)
				
				// 处理队列中的图片
				for {
					img, ok := queue.Pop()
					if !ok {
						break  // 队列为空
					}
					
					// 调度推理
					start := time.Now()
					s.scheduler.ScheduleInference(img)
					inferenceTime := time.Since(start).Milliseconds()
					
					// 记录性能
					monitor.RecordInference(inferenceTime)
				}
				
				// 定期输出统计
				queueStats := queue.GetStats()
				perfStats := monitor.GetStats()
				s.log.Info("performance stats",
					slog.Any("queue", queueStats),
					slog.Any("performance", perfStats))
			}
		}
	}()
	
	return nil
}
```

---

## 🛠️ 无需修改代码的临时方案

### 方案A：配置文件实现智能采样

**在扫描器中添加逻辑**（修改scanner.go）：

```go
// 在scanNewImages中添加
var imageCounter int

func (s *Scanner) scanNewImages() ([]ImageInfo, error) {
	// ... 现有扫描代码
	
	var newImages []ImageInfo
	samplingRate := 5  // 每5张处理1张
	
	for object := range objectCh {
		// ... 检查代码
		
		imageCounter++
		
		// 采样过滤
		if imageCounter % samplingRate != 0 {
			s.MarkProcessed(object.Key)  // 标记已处理但不推理
			continue
		}
		
		newImages = append(newImages, ImageInfo{...})
	}
	
	// 限制返回数量（避免积压）
	maxReturn := 20
	if len(newImages) > maxReturn {
		s.log.Warn("too many new images, limiting",
			slog.Int("found", len(newImages)),
			slog.Int("limit", maxReturn))
		newImages = newImages[len(newImages)-maxReturn:]  // 保留最新的
	}
	
	return newImages, nil
}
```

---

### 方案B：通过配置控制（立即可用）

**配置调整策略**：

```bash
# 场景1：抽帧快，推理也要快
# 抽帧：5张/秒
interval_ms = 200
scan_interval_sec = 1
max_concurrent_infer = 50

# 场景2：抽帧快，但推理慢
# 使用采样：只推理20%
interval_ms = 200  # 仍然5张/秒
scan_interval_sec = 5  # 降低扫描频率
max_concurrent_infer = 20
# 效果：每5秒扫描一次，每次处理25张中的部分

# 场景3：推理太慢，降低抽帧
interval_ms = 1000  # 降为1张/秒
scan_interval_sec = 3
max_concurrent_infer = 10
```

---

## 📊 推荐的配置组合

### 配置1：自适应推理（需要代码修改）

```toml
[ai_analysis]
queue_mode = 'smart'
max_queue_size = 100
queue_full_strategy = 'drop_oldest'
auto_sampling = true
backlog_alert_threshold = 50
slow_inference_threshold_ms = 5000
alert_interval_sec = 60
```

### 配置2：当前可用（无需改代码）

```toml
# 根据您的抽帧频率选择：

# 抽帧5张/秒（每200ms）
[[frame_extractor.tasks]]
interval_ms = 200

# 选项A：全部推理（需要快速算法）
[ai_analysis]
scan_interval_sec = 1
max_concurrent_infer = 50  # 够大才能不积压

# 选项B：降低扫描频率（部分推理）
[ai_analysis]
scan_interval_sec = 5  # 每5秒处理一批
max_concurrent_infer = 20

# 选项C：降低抽帧（匹配推理速度）
[[frame_extractor.tasks]]
interval_ms = 1000  # 1张/秒（推理慢时）
[ai_analysis]
scan_interval_sec = 3
max_concurrent_infer = 10
```

---

## 🚨 告警系统设计

### 告警类型

```go
type AlertType string

const (
	AlertBacklog      AlertType = "queue_backlog"      // 队列积压
	AlertSlowInfer    AlertType = "slow_inference"     // 推理太慢
	AlertHighDrop     AlertType = "high_drop_rate"     // 丢弃率过高
	AlertStorageFull  AlertType = "storage_full"       // 存储满
)

type SystemAlert struct {
	Type       AlertType
	Level      string  // warning|error|critical
	Message    string
	Data       map[string]interface{}
	Timestamp  time.Time
}
```

### 告警触发条件

```go
func (s *Service) checkSystemAlerts() {
	queueStats := s.queue.GetStats()
	perfStats := s.monitor.GetStats()
	
	// 1. 队列积压告警
	if queueStats["queue_size"].(int) > 50 {
		s.sendAlert(SystemAlert{
			Type:    AlertBacklog,
			Level:   "warning",
			Message: fmt.Sprintf("推理队列积压%d张图片", queueStats["queue_size"]),
			Data:    queueStats,
		})
	}
	
	// 2. 推理慢告警
	if perfStats["avg_inference_ms"].(float64) > 5000 {
		s.sendAlert(SystemAlert{
			Type:    AlertSlowInfer,
			Level:   "warning",
			Message: fmt.Sprintf("平均推理时间%.0fms，超过阈值5000ms", perfStats["avg_inference_ms"]),
			Data:    perfStats,
		})
	}
	
	// 3. 高丢弃率告警
	dropRate := float64(queueStats["dropped_total"].(int64)) / float64(perfStats["total_inferences"].(int64))
	if dropRate > 0.3 {  // 丢弃率超过30%
		s.sendAlert(SystemAlert{
			Type:    AlertHighDrop,
			Level:   "error",
			Message: fmt.Sprintf("图片丢弃率%.1f%%，推理能力不足", dropRate*100),
			Data:    map[string]interface{}{
				"drop_rate": dropRate,
				"dropped":   queueStats["dropped_total"],
				"total":     perfStats["total_inferences"],
			},
		})
	}
}
```

---

## 📋 API接口设计

### 查询性能统计

**GET** `/api/v1/ai_analysis/performance`

**响应**：
```json
{
  "queue": {
    "current_size": 25,
    "max_size": 100,
    "dropped_total": 123,
    "utilization": 0.25
  },
  "performance": {
    "total_inferences": 1523,
    "avg_inference_ms": 350,
    "frame_rate": 5.0,
    "inference_rate": 2.8,
    "recommended_sampling": 2
  },
  "alerts": [
    {
      "type": "queue_backlog",
      "level": "warning",
      "message": "推理队列积压50张",
      "timestamp": "2024-10-16T15:47:30Z"
    }
  ]
}
```

### 调整采样率

**POST** `/api/v1/ai_analysis/sampling`

**请求**：
```json
{
  "enabled": true,
  "sampling_rate": 3,  // 每3张处理1张
  "auto_adjust": true
}
```

---

## 🎯 实施计划

### 阶段1：配置层面优化（今天可完成）

**不修改代码，通过配置实现**：

```toml
# 方案：根据推理能力调整抽帧和扫描

# 如果推理慢（比如500ms/张）
[[frame_extractor.tasks]]
interval_ms = 500  # 匹配推理速度

[ai_analysis]
scan_interval_sec = 3  # 每3秒扫描一批
max_concurrent_infer = 20

# 如果推理快（比如100ms/张）
[[frame_extractor.tasks]]
interval_ms = 100  # 10张/秒

[ai_analysis]
scan_interval_sec = 1
max_concurrent_infer = 100
```

### 阶段2：代码层面优化（本周）

**需要修改的文件**：
1. ✅ 新建 `internal/plugin/aianalysis/queue.go` - 智能队列
2. ✅ 新建 `internal/plugin/aianalysis/monitor.go` - 性能监控
3. ✅ 修改 `internal/plugin/aianalysis/service.go` - 集成队列和监控
4. ✅ 修改 `internal/plugin/aianalysis/scanner.go` - 添加采样逻辑
5. ✅ 新增 API接口 - 性能统计和控制

### 阶段3：告警系统（本月）

**功能**：
- 队列积压告警
- 推理慢告警
- 高丢弃率告警
- 存储满告警

**通知方式**：
- Web界面通知
- 邮件/短信
- Webhook
- Kafka消息

---

## 💻 立即可用的简化方案

由于代码修改需要重新编译，我为您创建一个**配置文件驱动的方案**：

### 创建配置文件：`smart_inference_config.toml`

```toml
# yanying 智能推理配置

[inference]
# 基础配置
frame_interval_ms = 200  # 抽帧间隔
target_fps = 5  # 目标：5张/秒

# 智能队列
queue_enabled = true
queue_max_size = 100
queue_strategy = "drop_oldest"  # drop_oldest|drop_newest|latest_only

# 采样配置
sampling_enabled = true
sampling_mode = "auto"  # auto|fixed|adaptive
fixed_sampling_rate = 5  # fixed模式：每5张处理1张
auto_target_ratio = 0.8  # auto模式：推理能力应>=抽帧速度的80%

# 性能监控
monitoring_enabled = true
slow_inference_ms = 5000  # 超过5秒告警
backlog_threshold = 50  # 积压50张告警
alert_interval_sec = 60  # 告警间隔

# 告警通知
alert_webhook = ""  # Webhook URL
alert_email = ""  # 邮件地址
```

---

## 🔍 监控和调优

### 实时监控脚本

```bash
cat > /code/EasyDarwin/monitor_inference.sh << 'EOF'
#!/bin/bash

# 推理性能实时监控

LOG_FILE="/code/EasyDarwin/build/EasyDarwin-lin-*/logs/20251016_08_00_00.log"

echo "监控推理性能（Ctrl+C停止）..."
echo ""

LAST_FOUND=0
LAST_SCHEDULED=0

while true; do
    sleep 10
    
    # 统计最近10秒
    FOUND=$(tail -n 100 $LOG_FILE | grep "found new" | grep -o '"count":[0-9]*' | cut -d':' -f2 | awk '{sum+=$1} END {print sum}')
    SCHEDULED=$(tail -n 100 $LOG_FILE | grep "scheduling inference" | wc -l)
    
    FOUND=${FOUND:-0}
    SCHEDULED=${SCHEDULED:-0}
    
    FOUND_RATE=$(echo "scale=1; ($FOUND - $LAST_FOUND) / 10" | bc)
    SCHED_RATE=$(echo "scale=1; ($SCHEDULED - $LAST_SCHEDULED) / 10" | bc)
    
    echo "[$(date +%H:%M:%S)] 发现: ${FOUND_RATE}张/秒, 调度: ${SCHED_RATE}次/秒"
    
    # 检查积压
    if (( $(echo "$FOUND_RATE > $SCHED_RATE * 1.5" | bc -l) )); then
        echo "  ⚠️  推理速度跟不上抽帧速度！"
    fi
    
    LAST_FOUND=$FOUND
    LAST_SCHEDULED=$SCHEDULED
done
EOF

chmod +x /code/EasyDarwin/monitor_inference.sh
```

---

## 🎯 根据您需求的最终方案

### 配置：自动适配5张/秒

```toml
[[frame_extractor.tasks]]
interval_ms = 200  # 抽帧：5张/秒

[ai_analysis]
scan_interval_sec = 1  # 每秒扫描
max_concurrent_infer = 50  # 足够的并发

# （未来添加）
queue_max_size = 100  # 队列最多100张
queue_strategy = 'drop_oldest'  # 超过则丢弃旧的
backlog_alert = 50  # 积压50张告警
slow_inference_alert_ms = 5000  # 推理超5秒告警
```

### 当前可用的解决方案

**1. 队列限制**：通过扫描间隔控制
```toml
scan_interval_sec = 1  # 每秒最多处理一批
# 如果每批5张，则队列不会超过5张
```

**2. 自动丢弃**：MinIO清理策略
```bash
# 1天过期 = 自动丢弃旧图片
/tmp/mc ilm add yanying-minio/images --expiry-days 1
```

**3. 性能监控**：日志分析
```bash
# 实时监控脚本
./monitor_inference.sh
```

**4. 手动调优**：根据日志调整参数
- 如果积压 → 增加`max_concurrent_infer`
- 如果推理慢 → 降低`interval_ms`

---

## 📝 总结

### 您的需求

1. ✅ **根据抽帧频率配置推理**
   - 当前：抽帧5张/秒，推理也是5张/秒
   
2. ⚠️ **推理太慢时告警**
   - 临时方案：通过日志监控
   - 完整方案：需要代码添加告警系统

3. ⚠️ **自动丢弃处理不完的图片**
   - 临时方案：MinIO自动清理
   - 完整方案：需要代码添加智能队列

### 当前状态

✅ **配置层面已优化**：
- 抽帧：5张/秒
- 扫描：每秒
- 并发：50个
- 清理：1天过期

⚠️ **代码层面待开发**：
- 智能队列管理
- 性能监控告警
- 自动采样调整

---

**我建议：先使用当前的配置方案，同时我可以帮您实现代码层面的智能队列和告警功能。需要我继续开发吗？**

