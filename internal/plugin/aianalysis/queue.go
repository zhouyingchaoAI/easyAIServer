package aianalysis

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
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

// InferenceQueue 智能推理队列
type InferenceQueue struct {
	images           []ImageInfo
	imageSet         map[string]bool // 用于快速检查图片是否已在队列中（path -> bool）
	maxSize          int
	strategy         QueueStrategy
	mu               sync.RWMutex
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

// NewInferenceQueue 创建智能队列
func NewInferenceQueue(maxSize int, strategy QueueStrategy, alertThreshold int, minioClient *minio.Client, bucket string, deleteDropped bool, logger *slog.Logger) *InferenceQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	if alertThreshold <= 0 {
		alertThreshold = maxSize / 2
	}
	
	return &InferenceQueue{
		images:         make([]ImageInfo, 0, maxSize),
		imageSet:       make(map[string]bool),
		maxSize:        maxSize,
		strategy:       strategy,
		alertThreshold: alertThreshold,
		alertInterval:  60 * time.Second,
		log:            logger,
		minio:          minioClient,
		bucket:         bucket,
		deleteDropped:  deleteDropped,
	}
}

// SetAlertCallback 设置告警回调
func (q *InferenceQueue) SetAlertCallback(callback func(AlertInfo)) {
	q.alertCallback = callback
}

// Add 添加图片到队列
func (q *InferenceQueue) Add(images []ImageInfo) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	added := 0
	duplicateCount := 0
	for _, img := range images {
		// 规范化路径格式（确保与MinIO key格式一致）
		normalizedPath := filepath.ToSlash(img.Path)
		
		// 检查图片是否已在队列中（去重）
		if q.imageSet[normalizedPath] {
			duplicateCount++
			continue
		}
		
		// 检查队列是否已满
		if len(q.images) >= q.maxSize {
			switch q.strategy {
			case StrategyDropOldest:
				// 丢弃最旧的
				dropped := q.images[0]
				// 从imageSet中移除（规范化路径）
				normalizedDroppedPath := filepath.ToSlash(dropped.Path)
				delete(q.imageSet, normalizedDroppedPath)
				q.images = q.images[1:]
				q.droppedCount++
				
				// 删除MinIO中的图片
				if q.deleteDropped {
					q.deleteImageFromMinIO(dropped)
				}
				
				q.log.Warn("queue full, dropped oldest image",
					slog.String("task_type", dropped.TaskType),
					slog.String("task_id", dropped.TaskID),
					slog.String("image", dropped.Filename),
					slog.Int("queue_size", len(q.images)),
					slog.Int64("total_dropped", q.droppedCount))
				
			case StrategyDropNewest:
				// 丢弃新的（不加入）
				q.droppedCount++
				
				// 删除MinIO中的图片
				if q.deleteDropped {
					q.deleteImageFromMinIO(img)
				}
				
				q.log.Warn("queue full, dropped newest image",
					slog.String("image", img.Filename))
				continue
				
			case StrategyLatestOnly:
				// 清空队列，只保留最新的
				oldImages := make([]ImageInfo, len(q.images))
				copy(oldImages, q.images)
				oldLen := len(q.images)
				// 清空imageSet
				q.imageSet = make(map[string]bool)
				q.images = q.images[:0]
				q.droppedCount += int64(oldLen)
				
				// 批量删除MinIO中的图片
				if q.deleteDropped {
					for _, droppedImg := range oldImages {
						q.deleteImageFromMinIO(droppedImg)
					}
				}
				
				q.log.Warn("queue full, cleared for latest images",
					slog.Int("cleared", oldLen))
			}
		}
		
		// 添加到队列和imageSet（使用规范化路径）
		q.images = append(q.images, img)
		q.imageSet[normalizedPath] = true
		added++
	}
	
	// 如果发现重复图片，记录日志
	if duplicateCount > 0 {
		q.log.Warn("duplicate images detected and skipped",
			slog.Int("duplicate_count", duplicateCount),
			slog.Int("added_count", added),
			slog.String("note", "images already in queue, preventing duplicate processing"))
	}
	
	// 检查积压告警
	q.checkBacklogAlertLocked()
	
	return added
}

// Pop 取出一张图片
func (q *InferenceQueue) Pop() (ImageInfo, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if len(q.images) == 0 {
		return ImageInfo{}, false
	}
	
	img := q.images[0]
	// 从imageSet中移除（规范化路径）
	normalizedPath := filepath.ToSlash(img.Path)
	delete(q.imageSet, normalizedPath)
	q.images = q.images[1:]
	// 注意：不在Pop时增加processedCount，只在推理成功或失败后增加
	// 这样可以确保processedCount更准确地反映实际推理的数量
	
	return img, true
}

// PopBatch 批量取出
func (q *InferenceQueue) PopBatch(n int) []ImageInfo {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if len(q.images) == 0 {
		return nil
	}
	
	if n > len(q.images) {
		n = len(q.images)
	}
	
	batch := make([]ImageInfo, n)
	copy(batch, q.images[:n])
	// 从imageSet中移除
	for i := 0; i < n; i++ {
		delete(q.imageSet, q.images[i].Path)
	}
	q.images = q.images[n:]
	// 注意：不在PopBatch时增加processedCount，只在推理成功或失败后增加
	// 这样可以确保processedCount更准确地反映实际推理的数量
	
	return batch
}

// Size 获取当前队列大小
func (q *InferenceQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.images)
}

// GetImagePaths 获取队列中所有图片的路径集合（用于清理时排除）
func (q *InferenceQueue) GetImagePaths() map[string]bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	// 返回imageSet的副本，避免外部修改
	paths := make(map[string]bool, len(q.imageSet))
	for path := range q.imageSet {
		paths[path] = true
	}
	return paths
}

// Contains 检查图片是否在队列中
func (q *InferenceQueue) Contains(imagePath string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	// 规范化路径格式（确保与MinIO key格式一致）
	normalizedPath := filepath.ToSlash(imagePath)
	return q.imageSet[normalizedPath]
}

// Clear 清空队列
func (q *InferenceQueue) Clear() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	cleared := len(q.images)
	q.images = q.images[:0]
	q.imageSet = make(map[string]bool)
	return cleared
}

// RecordProcessed 记录一次处理（推理成功或失败后调用）
func (q *InferenceQueue) RecordProcessed() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.processedCount++
}

// checkBacklogAlertLocked 检查积压告警（需要已加锁）
func (q *InferenceQueue) checkBacklogAlertLocked() {
	if len(q.images) <= q.alertThreshold {
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
		QueueSize: len(q.images),
		Dropped:   q.droppedCount,
		Timestamp: now,
	}
	
	q.log.Error("inference backlog alert",
		slog.Int("queue_size", len(q.images)),
		slog.Int("threshold", q.alertThreshold),
		slog.Int64("dropped_total", q.droppedCount),
		slog.String("message", alert.Message))
	
	// 触发告警回调
	if q.alertCallback != nil {
		q.alertCallback(alert)
	}
}

// GetStats 获取统计信息
func (q *InferenceQueue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	utilization := 0.0
	if q.maxSize > 0 {
		utilization = float64(len(q.images)) / float64(q.maxSize)
	}
	
	return map[string]interface{}{
		"queue_size":     len(q.images),
		"max_size":       q.maxSize,
		"dropped_total":  q.droppedCount,
		"processed_total": q.processedCount,
		"utilization":    utilization,
		"strategy":       string(q.strategy),
	}
}

// GetDropRate 获取丢弃率
func (q *InferenceQueue) GetDropRate() float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	total := q.processedCount + q.droppedCount
	if total == 0 {
		return 0
	}
	
	return float64(q.droppedCount) / float64(total)
}

// ResetStats 重置统计数据
func (q *InferenceQueue) ResetStats() {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.droppedCount = 0
	q.processedCount = 0
	q.lastAlertTime = time.Time{}
	
	q.log.Info("inference queue stats reset",
		slog.Int("remaining_queue_size", len(q.images)))
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

