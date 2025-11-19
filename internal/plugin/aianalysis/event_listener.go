package aianalysis

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/notification"
)

// EventListener MinIO事件监听器（替代扫描器）
type EventListener struct {
	minio         *minio.Client
	bucket        string
	basePath      string
	alertBasePath string // 告警图片路径前缀
	processed     map[string]time.Time // 已处理图片 path -> 处理时间
	mu            sync.RWMutex
	log           *slog.Logger
	stopListen    chan struct{}
	onNewImage    func(ImageInfo) // 新图片回调
	onImageDelete func(string)    // 图片删除回调（path）
	listening     bool            // 是否正在监听
	listeningMu   sync.Mutex      // 保护listening状态
	stopOnce      sync.Once       // 确保只关闭一次stopListen
	cleanupTicker *time.Ticker    // 定期清理ticker
}

// NewEventListener 创建事件监听器
func NewEventListener(minioClient *minio.Client, bucket, basePath, alertBasePath string, logger *slog.Logger) *EventListener {
	return &EventListener{
		minio:         minioClient,
		bucket:        bucket,
		basePath:      basePath,
		alertBasePath: alertBasePath,
		processed:     make(map[string]time.Time),
		log:           logger,
		stopListen:    make(chan struct{}),
	}
}

// Start 启动事件监听
func (e *EventListener) Start(onNewImage func(ImageInfo), onImageDelete func(string)) {
	e.onNewImage = onNewImage
	e.onImageDelete = onImageDelete

	go e.listenEvents()
	
	// 启动定期清理
	e.startProcessedCleanup()
}

// Stop 停止事件监听
func (e *EventListener) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopListen)
		if e.cleanupTicker != nil {
			e.cleanupTicker.Stop()
		}
		e.log.Info("event listener stop signal sent")
	})
}

// listenEvents 监听MinIO事件
func (e *EventListener) listenEvents() {
	// 检查是否已经在监听
	e.listeningMu.Lock()
	if e.listening {
		e.listeningMu.Unlock()
		e.log.Warn("event listener already running, skipping duplicate start")
		return
	}
	e.listening = true
	e.listeningMu.Unlock()
	
	// 添加panic恢复机制
	defer func() {
		e.listeningMu.Lock()
		e.listening = false
		e.listeningMu.Unlock()
		
		if r := recover(); r != nil {
			e.log.Error("panic in event listener, recovering",
				slog.Any("panic", r))
			// 检查是否应该停止
			select {
			case <-e.stopListen:
				e.log.Info("event listener stopped after panic")
				return
			default:
			}
			// 修复：不再递归启动新的监听器，避免goroutine泄漏
			// 如果监听器崩溃，应该由外部服务重新启动，而不是自动重启
			// 自动重启可能导致多个监听器同时运行，造成goroutine泄漏
			e.log.Error("event listener panic recovered, but not auto-restarting to prevent goroutine leak",
				slog.String("note", "service restart required"))
		}
	}()
	
	e.log.Info("starting MinIO event listener",
		slog.String("bucket", e.bucket),
		slog.String("base_path", e.basePath),
		slog.String("alert_base_path", e.alertBasePath))

	// 配置事件通知
	var ctx context.Context
	var cancel context.CancelFunc
	
	// 创建初始context
	ctx, cancel = context.WithCancel(context.Background())
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	// 监听对象创建和删除事件
	notificationCh := e.minio.ListenBucketNotification(ctx, e.bucket, e.basePath, "", []string{
		"s3:ObjectCreated:*",  // 对象创建事件（包括Put、Post、Copy等）
		"s3:ObjectRemoved:*",  // 对象删除事件（包括Delete、DeleteMarkerCreated等）
	})

	for {
		select {
		case <-e.stopListen:
			e.log.Info("MinIO event listener stopped")
			cancel()
			return
		case notificationInfo, ok := <-notificationCh:
			if !ok {
				// 通道已关闭，可能是连接断开，重新连接
				e.log.Warn("notification channel closed, reconnecting...")
				cancel()
				time.Sleep(5 * time.Second)
				
				// 检查是否应该停止
				select {
				case <-e.stopListen:
					e.log.Info("MinIO event listener stopped during reconnect")
					return
				default:
				}
				
				// 重新创建context和监听
				ctx, cancel = context.WithCancel(context.Background())
				notificationCh = e.minio.ListenBucketNotification(ctx, e.bucket, e.basePath, "", []string{
					"s3:ObjectCreated:*",
					"s3:ObjectRemoved:*",
				})
				e.log.Info("reconnected to MinIO event notification")
				continue
			}
			
			if notificationInfo.Err != nil {
				// 检查是否是context取消错误（正常关闭）
				if notificationInfo.Err == context.Canceled {
					e.log.Info("notification context canceled, stopping listener")
					return
				}
				
				e.log.Error("notification error",
					slog.String("err", notificationInfo.Err.Error()))
				// 发生错误时，等待一段时间后重新连接
				time.Sleep(5 * time.Second)
				
				// 检查是否应该停止
				select {
				case <-e.stopListen:
					e.log.Info("MinIO event listener stopped during error recovery")
					return
				default:
				}
				
				// 重新创建context和监听
				cancel()
				ctx, cancel = context.WithCancel(context.Background())
				notificationCh = e.minio.ListenBucketNotification(ctx, e.bucket, e.basePath, "", []string{
					"s3:ObjectCreated:*",
					"s3:ObjectRemoved:*",
				})
				e.log.Info("reconnected to MinIO event notification after error")
				continue
			}

			// 处理通知记录
			for _, record := range notificationInfo.Records {
				e.handleNotificationRecord(record)
			}
		}
	}
}

// handleNotificationRecord 处理通知记录
func (e *EventListener) handleNotificationRecord(record notification.Event) {
	// 添加panic恢复机制，防止单个事件处理失败导致整个监听器崩溃
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("panic in handleNotificationRecord, recovered",
				slog.Any("panic", r),
				slog.String("event_name", record.EventName),
				slog.String("object_key", record.S3.Object.Key))
		}
	}()
	
	// 解析事件名称
	eventName := record.EventName
	objectKey := record.S3.Object.Key

	// 规范化路径
	normalizedPath := filepath.ToSlash(objectKey)

	e.log.Debug("received MinIO event",
		slog.String("event_name", eventName),
		slog.String("object_key", normalizedPath),
		slog.String("bucket", record.S3.Bucket.Name))

	// 跳过告警路径中的图片
	if e.alertBasePath != "" && strings.HasPrefix(normalizedPath, e.alertBasePath) {
		e.log.Debug("skipping alert path image",
			slog.String("path", normalizedPath))
		return
	}

	// 处理对象创建事件
	if strings.HasPrefix(eventName, "s3:ObjectCreated:") {
		e.handleObjectCreated(normalizedPath, record)
		return
	}

	// 处理对象删除事件
	if strings.HasPrefix(eventName, "s3:ObjectRemoved:") {
		e.handleObjectRemoved(normalizedPath)
		return
	}

	e.log.Debug("unhandled event type",
		slog.String("event_name", eventName),
		slog.String("object_key", normalizedPath))
}

// handleObjectCreated 处理对象创建事件
func (e *EventListener) handleObjectCreated(objectKey string, record notification.Event) {
	// 过滤非图片文件
	if !isImageFile(objectKey) {
		return
	}

	// 跳过.keep等标记文件
	if strings.Contains(objectKey, "/.") {
		return
	}

	// 跳过配置文件
	if strings.HasSuffix(objectKey, "algo_config.json") || strings.HasSuffix(objectKey, ".json") {
		return
	}

	// 跳过预览图
	filename := objectKey[strings.LastIndex(objectKey, "/")+1:]
	if strings.HasPrefix(filename, "preview_") {
		return
	}

	// 检查是否已处理
	if e.isProcessed(objectKey) {
		e.log.Debug("image already processed, skipping",
			slog.String("path", objectKey))
		return
	}

	// 解析路径：任务类型/任务ID/文件名
	taskType, taskID, filename := parseImagePath(objectKey, e.basePath)
	if taskType == "" || taskID == "" {
		e.log.Warn("skipping image with invalid path structure",
			slog.String("path", objectKey),
			slog.String("base_path", e.basePath),
			slog.String("note", "expected format: basePath/taskType/taskID/filename.jpg"))
		return
	}

	// 获取对象信息
	// 优先从MinIO获取完整信息（包括大小和修改时间）
	objInfo, err := e.minio.StatObject(context.Background(), e.bucket, objectKey, minio.StatObjectOptions{})
	var size int64
	var modTime time.Time
	
	if err != nil {
		// 如果无法获取对象信息，使用事件中的信息
		e.log.Warn("failed to stat object, using event data",
			slog.String("path", objectKey),
			slog.String("err", err.Error()))
		
		// 使用事件中的大小
		if record.S3.Object.Size > 0 {
			size = record.S3.Object.Size
		}
		// 使用当前时间作为默认值
		modTime = time.Now()
	} else {
		// 使用从MinIO获取的完整信息
		size = objInfo.Size
		modTime = objInfo.LastModified
	}

	imageInfo := ImageInfo{
		Path:     objectKey,
		TaskType: taskType,
		TaskID:   taskID,
		Filename: filename,
		Size:     size,
		ModTime:  modTime,
	}

	e.log.Info("new image detected via event",
		slog.String("path", objectKey),
		slog.String("task_type", taskType),
		slog.String("task_id", taskID),
		slog.String("filename", filename),
		slog.Int64("size", size))

	// 标记为已处理（防止重复处理）
	e.MarkProcessed(objectKey)

	// 调用回调
	if e.onNewImage != nil {
		e.onNewImage(imageInfo)
	}
}

// handleObjectRemoved 处理对象删除事件
func (e *EventListener) handleObjectRemoved(objectKey string) {
	e.log.Info("image deleted via event",
		slog.String("path", objectKey))

	// 从已处理列表中移除
	e.mu.Lock()
	delete(e.processed, objectKey)
	e.mu.Unlock()

	// 调用删除回调（从队列中移除）
	if e.onImageDelete != nil {
		e.onImageDelete(objectKey)
	}
}

// MarkProcessed 标记图片已处理
func (e *EventListener) MarkProcessed(imagePath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.processed[imagePath] = time.Now()

	// 清理过期记录（超过24小时）
	if len(e.processed) > 10000 {
		e.cleanupProcessedLocked()
	}
}

// isProcessed 检查图片是否已处理
func (e *EventListener) isProcessed(imagePath string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.processed[imagePath]
	return exists
}

// cleanupProcessedLocked 清理过期的已处理记录（需要已加锁）
func (e *EventListener) cleanupProcessedLocked() {
	now := time.Now()
	cleanupThreshold := 30 * time.Minute // 缩短到只保留最近30分钟的记录
	cleanedCount := 0
	
	for path, processedTime := range e.processed {
		if now.Sub(processedTime) > cleanupThreshold {
			delete(e.processed, path)
			cleanedCount++
		}
	}
	
	if cleanedCount > 0 {
		e.log.Info("cleaned up processed images cache",
			slog.Int("cleaned_count", cleanedCount),
			slog.Int("remaining", len(e.processed)))
	}
}

// cleanupProcessed 清理过期的已处理记录（自动加锁）
func (e *EventListener) cleanupProcessed() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cleanupProcessedLocked()
}

// startProcessedCleanup 启动定期清理
func (e *EventListener) startProcessedCleanup() {
	e.cleanupTicker = time.NewTicker(5 * time.Minute) // 缩短到每5分钟清理一次
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.log.Error("panic in processed cleanup", slog.Any("panic", r))
			}
		}()
		for range e.cleanupTicker.C {
			select {
			case <-e.stopListen:
				return
			default:
				e.cleanupProcessed()
			}
		}
	}()
}

// GetProcessedCount 获取已处理图片数量（用于统计）
func (e *EventListener) GetProcessedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.processed)
}

