package aianalysis

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
)

// QueueStrategy 队列策略
type QueueStrategy string

const (
	StrategyDropOldest QueueStrategy = "drop_oldest" // 丢弃最旧的
	StrategyDropNewest QueueStrategy = "drop_newest" // 丢弃最新的
	StrategyLatestOnly QueueStrategy = "latest_only" // 只保留最新的N张
)

// InferenceQueue 智能推理队列（使用Channel+Map+原子计数器混合方案）
type InferenceQueue struct {
	// Channel作为主队列（无锁Pop）
	ch chan ImageInfo
	
	// Map用于快速查找（用于Contains、Remove、GetImagePaths）
	imageSet    map[string]bool // 用于快速检查图片是否已在队列中（path -> bool）
	imageSetMu  sync.RWMutex    // 保护imageSet
	
	// 已删除标记（用于Remove操作）
	deletedSet    map[string]bool // 标记为已删除的图片路径
	deletedSetMu  sync.RWMutex    // 保护deletedSet
	
	// 原子计数器（用于Size统计）
	sizeCounter int64 // 队列大小计数器（原子操作）
	
	maxSize          int
	strategy         QueueStrategy
	droppedCount     int64
	processedCount   int64 // 已处理的图片数量（包括成功和失败的推理）
	lastAlertTime    time.Time
	alertThreshold   int
	alertInterval    time.Duration
	log              *slog.Logger
	alertCallback    func(AlertInfo)
	minio            *minio.Client // MinIO客户端
	bucket           string         // MinIO bucket
	deleteDropped    bool           // 是否删除丢弃的图片
}

// AlertInfo 告警信息
type AlertInfo struct {
	Type      string
	Level     string
	Message   string
	QueueSize int
	Dropped   int64
	Timestamp time.Time
}

// NewInferenceQueue 创建智能队列（使用Channel+Map+原子计数器）
func NewInferenceQueue(maxSize int, strategy QueueStrategy, alertThreshold int, minioClient *minio.Client, bucket string, deleteDropped bool, logger *slog.Logger) *InferenceQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	if alertThreshold <= 0 {
		alertThreshold = maxSize / 2
	}
	
	return &InferenceQueue{
		ch:            make(chan ImageInfo, maxSize), // buffered channel，容量=maxSize
		imageSet:      make(map[string]bool),
		deletedSet:    make(map[string]bool),
		sizeCounter:   0,
		maxSize:       maxSize,
		strategy:      strategy,
		alertThreshold: alertThreshold,
		alertInterval: 60 * time.Second,
		log:           logger,
		minio:         minioClient,
		bucket:        bucket,
		deleteDropped: deleteDropped,
	}
}

// SetAlertCallback 设置告警回调
func (q *InferenceQueue) SetAlertCallback(callback func(AlertInfo)) {
	q.alertCallback = callback
}

// Add 添加图片到队列（使用Channel+Map）
func (q *InferenceQueue) Add(images []ImageInfo) int {
	added := 0
	duplicateCount := 0
	
	for _, img := range images {
		// 规范化路径格式（确保与MinIO key格式一致）
		normalizedPath := filepath.ToSlash(img.Path)
		
		// 检查图片是否已在队列中（去重）- 使用Map
		q.imageSetMu.RLock()
		if q.imageSet[normalizedPath] {
			q.imageSetMu.RUnlock()
			duplicateCount++
			continue
		}
		q.imageSetMu.RUnlock()
		
		// 检查队列是否已满（使用原子计数器）
		currentSize := atomic.LoadInt64(&q.sizeCounter)
		if int(currentSize) >= q.maxSize {
			switch q.strategy {
			case StrategyDropOldest:
				// 丢弃最旧的：从Channel中Pop一个（非阻塞）
				select {
				case dropped := <-q.ch:
					// 从imageSet中移除
					normalizedDroppedPath := filepath.ToSlash(dropped.Path)
					q.imageSetMu.Lock()
					delete(q.imageSet, normalizedDroppedPath)
					q.imageSetMu.Unlock()
					atomic.AddInt64(&q.sizeCounter, -1)
					atomic.AddInt64(&q.droppedCount, 1)
					
					// 删除MinIO中的图片
					if q.deleteDropped {
						q.deleteImageFromMinIO(dropped)
					}
					
					q.log.Warn("queue full, dropped oldest image",
						slog.String("task_type", dropped.TaskType),
						slog.String("task_id", dropped.TaskID),
						slog.String("image", dropped.Filename),
						slog.Int64("queue_size", atomic.LoadInt64(&q.sizeCounter)),
						slog.Int64("total_dropped", atomic.LoadInt64(&q.droppedCount)))
				default:
					// Channel为空，直接添加
				}
				
			case StrategyDropNewest:
				// 丢弃新的（不加入）
				atomic.AddInt64(&q.droppedCount, 1)
				
				// 删除MinIO中的图片
				if q.deleteDropped {
					q.deleteImageFromMinIO(img)
				}
				
				q.log.Warn("queue full, dropped newest image",
					slog.String("image", img.Filename))
				continue
				
			case StrategyLatestOnly:
				// 清空队列，只保留最新的
				cleared := 0
				for {
					select {
					case dropped := <-q.ch:
						// 从imageSet中移除
						normalizedDroppedPath := filepath.ToSlash(dropped.Path)
						q.imageSetMu.Lock()
						delete(q.imageSet, normalizedDroppedPath)
						q.imageSetMu.Unlock()
						atomic.AddInt64(&q.sizeCounter, -1)
						cleared++
						
						// 删除MinIO中的图片
						if q.deleteDropped {
							q.deleteImageFromMinIO(dropped)
						}
					default:
						// Channel已空
						goto cleared
					}
				}
			cleared:
				atomic.AddInt64(&q.droppedCount, int64(cleared))
				q.log.Warn("queue full, cleared for latest images",
					slog.Int("cleared", cleared))
			}
		}
		
		// 添加到Channel（非阻塞）
		select {
		case q.ch <- img:
			// 成功添加到Channel
			// 更新imageSet
			q.imageSetMu.Lock()
			q.imageSet[normalizedPath] = true
			q.imageSetMu.Unlock()
			atomic.AddInt64(&q.sizeCounter, 1)
			added++
		default:
			// Channel已满（理论上不应该发生，因为上面已经处理了）
			// 如果发生，按策略处理
			if q.strategy == StrategyDropNewest {
				atomic.AddInt64(&q.droppedCount, 1)
				if q.deleteDropped {
					q.deleteImageFromMinIO(img)
				}
			}
		}
	}
	
	// 如果发现重复图片，记录日志
	if duplicateCount > 0 {
		q.log.Warn("duplicate images detected and skipped",
			slog.Int("duplicate_count", duplicateCount),
			slog.Int("added_count", added),
			slog.String("note", "images already in queue, preventing duplicate processing"))
	}
	
	// 检查积压告警
	q.checkBacklogAlert()
	
	return added
}

// Pop 取出一张图片（无锁！使用Channel）
func (q *InferenceQueue) Pop() (ImageInfo, bool) {
	// 使用Channel非阻塞读取（无锁！）
	for {
		select {
		case img := <-q.ch:
			// 检查是否已标记为删除
			normalizedPath := filepath.ToSlash(img.Path)
			q.deletedSetMu.RLock()
			if q.deletedSet[normalizedPath] {
				// 已删除，从deletedSet中移除，继续Pop下一个
				q.deletedSetMu.RUnlock()
				q.deletedSetMu.Lock()
				delete(q.deletedSet, normalizedPath)
				q.deletedSetMu.Unlock()
				atomic.AddInt64(&q.sizeCounter, -1)
				// 从imageSet中移除
				q.imageSetMu.Lock()
				delete(q.imageSet, normalizedPath)
				q.imageSetMu.Unlock()
				continue // 继续Pop下一个
			}
			q.deletedSetMu.RUnlock()
			
			// 正常处理：从imageSet中移除
			q.imageSetMu.Lock()
			delete(q.imageSet, normalizedPath)
			q.imageSetMu.Unlock()
			atomic.AddInt64(&q.sizeCounter, -1)
			
			// 注意：不在Pop时增加processedCount，只在推理成功或失败后增加
			// 这样可以确保processedCount更准确地反映实际推理的数量
			
			return img, true
		default:
			// Channel为空，返回false
			return ImageInfo{}, false
		}
	}
}

// PopBatch 批量取出（使用Channel，无锁）
func (q *InferenceQueue) PopBatch(n int) []ImageInfo {
	batch := make([]ImageInfo, 0, n)
	
	// 从Channel中批量读取
	for i := 0; i < n; i++ {
		select {
		case img := <-q.ch:
			// 检查是否已标记为删除
			normalizedPath := filepath.ToSlash(img.Path)
			q.deletedSetMu.RLock()
			if q.deletedSet[normalizedPath] {
				// 已删除，跳过
				q.deletedSetMu.RUnlock()
				q.deletedSetMu.Lock()
				delete(q.deletedSet, normalizedPath)
				q.deletedSetMu.Unlock()
				atomic.AddInt64(&q.sizeCounter, -1)
				// 从imageSet中移除
				q.imageSetMu.Lock()
				delete(q.imageSet, normalizedPath)
				q.imageSetMu.Unlock()
				continue
			}
			q.deletedSetMu.RUnlock()
			
			// 正常处理
			q.imageSetMu.Lock()
			delete(q.imageSet, normalizedPath)
			q.imageSetMu.Unlock()
			atomic.AddInt64(&q.sizeCounter, -1)
			batch = append(batch, img)
		default:
			// Channel为空，返回已读取的
			break
		}
	}
	
	// 注意：不在PopBatch时增加processedCount，只在推理成功或失败后增加
	// 这样可以确保processedCount更准确地反映实际推理的数量
	
	if len(batch) == 0 {
		return nil
	}
	return batch
}

// Size 获取当前队列大小（使用原子计数器）
func (q *InferenceQueue) Size() int {
	return int(atomic.LoadInt64(&q.sizeCounter))
}

// GetImagePaths 获取队列中所有图片的路径集合（用于清理时排除）
func (q *InferenceQueue) GetImagePaths() map[string]bool {
	q.imageSetMu.RLock()
	defer q.imageSetMu.RUnlock()
	
	// 返回imageSet的副本，避免外部修改
	paths := make(map[string]bool, len(q.imageSet))
	for path := range q.imageSet {
		paths[path] = true
	}
	return paths
}

// Contains 检查图片是否在队列中（使用Map）
func (q *InferenceQueue) Contains(imagePath string) bool {
	q.imageSetMu.RLock()
	defer q.imageSetMu.RUnlock()
	// 规范化路径格式（确保与MinIO key格式一致）
	normalizedPath := filepath.ToSlash(imagePath)
	return q.imageSet[normalizedPath]
}

// Clear 清空队列（循环读取Channel）
func (q *InferenceQueue) Clear() int {
	cleared := 0
	
	// 循环读取Channel直到为空
	for {
		select {
		case <-q.ch:
			cleared++
			atomic.AddInt64(&q.sizeCounter, -1)
		default:
			// Channel已空
			goto done
		}
	}
	
done:
	// 清空Map
	q.imageSetMu.Lock()
	q.imageSet = make(map[string]bool)
	q.imageSetMu.Unlock()
	
	q.deletedSetMu.Lock()
	q.deletedSet = make(map[string]bool)
	q.deletedSetMu.Unlock()
	
	return cleared
}

// RecordProcessed 记录一次处理（推理成功或失败后调用）
func (q *InferenceQueue) RecordProcessed() {
	atomic.AddInt64(&q.processedCount, 1)
}

// checkBacklogAlert 检查积压告警（使用原子计数器）
func (q *InferenceQueue) checkBacklogAlert() {
	currentSize := int(atomic.LoadInt64(&q.sizeCounter))
	if currentSize <= q.alertThreshold {
		return
	}
	
	now := time.Now()
	if now.Sub(q.lastAlertTime) < q.alertInterval {
		return  // 避免频繁告警
	}
	
	q.lastAlertTime = now
	
	alert := AlertInfo{
		Type:      "queue_backlog",
		Level:     "warning",
		Message:   "推理队列积压严重，建议增加并发数或降低抽帧频率",
		QueueSize: currentSize,
		Dropped:   atomic.LoadInt64(&q.droppedCount),
		Timestamp: now,
	}
	
	q.log.Error("inference backlog alert",
		slog.Int("queue_size", currentSize),
		slog.Int("threshold", q.alertThreshold),
		slog.Int64("dropped_total", atomic.LoadInt64(&q.droppedCount)),
		slog.String("message", alert.Message))
	
	// 触发告警回调
	if q.alertCallback != nil {
		q.alertCallback(alert)
	}
}

// Remove 从队列中移除指定路径的图片（使用标记删除法）
func (q *InferenceQueue) Remove(imagePath string) bool {
	// 规范化路径格式
	normalizedPath := filepath.ToSlash(imagePath)
	
	// 检查图片是否在队列中
	q.imageSetMu.RLock()
	if !q.imageSet[normalizedPath] {
		q.imageSetMu.RUnlock()
		return false
	}
	q.imageSetMu.RUnlock()
	
	// 标记为已删除（Pop时会跳过）
	q.deletedSetMu.Lock()
	q.deletedSet[normalizedPath] = true
	q.deletedSetMu.Unlock()
	
	// 从imageSet中移除（立即移除，避免Contains返回true）
	q.imageSetMu.Lock()
	delete(q.imageSet, normalizedPath)
	q.imageSetMu.Unlock()
	
	// 注意：不在这里减少sizeCounter，因为图片还在Channel中
	// Pop时会检查deletedSet，如果已删除则跳过并减少计数器
	// 这样可以确保sizeCounter的准确性
	
	q.log.Debug("removed image from queue (marked as deleted)",
		slog.String("path", imagePath),
		slog.Int64("remaining_queue_size", atomic.LoadInt64(&q.sizeCounter)))
	
	return true
}

// GetStats 获取统计信息（使用原子计数器）
func (q *InferenceQueue) GetStats() map[string]interface{} {
	currentSize := int(atomic.LoadInt64(&q.sizeCounter))
	utilization := 0.0
	if q.maxSize > 0 {
		utilization = float64(currentSize) / float64(q.maxSize)
	}
	
	return map[string]interface{}{
		"queue_size":     currentSize,
		"max_size":       q.maxSize,
		"dropped_total":  atomic.LoadInt64(&q.droppedCount),
		"processed_total": atomic.LoadInt64(&q.processedCount),
		"utilization":    utilization,
		"strategy":       string(q.strategy),
	}
}

// GetDropRate 获取丢弃率（使用原子操作）
func (q *InferenceQueue) GetDropRate() float64 {
	processed := atomic.LoadInt64(&q.processedCount)
	dropped := atomic.LoadInt64(&q.droppedCount)
	total := processed + dropped
	if total == 0 {
		return 0
	}
	
	return float64(dropped) / float64(total)
}

// ResetStats 重置统计数据（使用原子操作）
func (q *InferenceQueue) ResetStats() {
	atomic.StoreInt64(&q.droppedCount, 0)
	atomic.StoreInt64(&q.processedCount, 0)
	q.lastAlertTime = time.Time{}
	
	q.log.Info("inference queue stats reset",
		slog.Int64("remaining_queue_size", atomic.LoadInt64(&q.sizeCounter)))
}

// deleteImageFromMinIO 删除MinIO中的图片
func (q *InferenceQueue) deleteImageFromMinIO(img ImageInfo) {
	if q.minio == nil || q.bucket == "" {
		return
	}
	
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
			slog.String("path", img.Path),
			slog.String("task_type", img.TaskType),
			slog.String("task_id", img.TaskID))
	}()
}

