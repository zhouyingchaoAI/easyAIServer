package aianalysis

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

// ImageInfo 图片信息
type ImageInfo struct {
	Path     string    // MinIO对象路径
	TaskType string    // 任务类型
	TaskID   string    // 任务ID
	Filename string    // 文件名
	Size     int64     // 文件大小
	ModTime  time.Time // 修改时间
}

// Scanner MinIO图片扫描器
type Scanner struct {
	minio        *minio.Client
	bucket       string
	basePath     string
	alertBasePath string // 告警图片路径前缀
	processed    map[string]time.Time // 已处理图片 path -> 处理时间
	mu           sync.RWMutex
	log          *slog.Logger
	stopScan     chan struct{}
}

// NewScanner 创建扫描器
func NewScanner(minioClient *minio.Client, bucket, basePath, alertBasePath string, logger *slog.Logger) *Scanner {
	return &Scanner{
		minio:         minioClient,
		bucket:        bucket,
		basePath:      basePath,
		alertBasePath: alertBasePath,
		processed:     make(map[string]time.Time),
		log:           logger,
		stopScan:      make(chan struct{}),
	}
}

// Start 启动定时扫描
func (s *Scanner) Start(intervalSec int, onNewImages func([]ImageInfo)) {
	if intervalSec <= 0 {
		intervalSec = 10
	}

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	go func() {
		// 立即扫描一次
		if images, err := s.scanNewImages(); err == nil && len(images) > 0 {
			onNewImages(images)
		}

		for {
			select {
			case <-s.stopScan:
				ticker.Stop()
				return
			case <-ticker.C:
				images, err := s.scanNewImages()
				if err != nil {
					s.log.Error("scan minio failed", slog.String("err", err.Error()))
					continue
				}
				if len(images) > 0 {
					s.log.Info("found new images", slog.Int("count", len(images)))
					onNewImages(images)
				}
			}
		}
	}()
}

// Stop 停止扫描
func (s *Scanner) Stop() {
	close(s.stopScan)
}

// scanNewImages 扫描MinIO中的新图片
func (s *Scanner) scanNewImages() ([]ImageInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 列举所有对象
	objectCh := s.minio.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    s.basePath,
		Recursive: true,
	})

	var newImages []ImageInfo
	for object := range objectCh {
		if object.Err != nil {
			s.log.Warn("list object error", slog.String("err", object.Err.Error()))
			continue
		}

		// 过滤非图片文件
		if !isImageFile(object.Key) {
			continue
		}

		// 跳过.keep等标记文件
		if strings.Contains(object.Key, "/.") {
			continue
		}
		
		// 跳过预览图（preview开头的图片不参与推理）
		filename := object.Key[strings.LastIndex(object.Key, "/")+1:]
		if strings.HasPrefix(filename, "preview_") {
			s.log.Debug("skipping preview image", slog.String("path", object.Key))
			continue
		}
		
		// 跳过配置文件
		if strings.HasSuffix(object.Key, "algo_config.json") || strings.HasSuffix(object.Key, ".json") {
			continue
		}
		
		// 跳过告警路径中的图片（已经推理过了）
		if s.alertBasePath != "" && strings.HasPrefix(object.Key, s.alertBasePath) {
			s.log.Debug("skipping alert image", slog.String("path", object.Key))
			continue
		}

		// 检查是否已处理
		if s.isProcessed(object.Key) {
			continue
		}

		// 解析路径：任务类型/任务ID/文件名
		taskType, taskID, filename := parseImagePath(object.Key, s.basePath)
		if taskType == "" || taskID == "" {
			s.log.Debug("skipping image with invalid path structure",
				slog.String("path", object.Key),
				slog.String("base_path", s.basePath))
			continue
		}

		s.log.Debug("parsed image path",
			slog.String("full_path", object.Key),
			slog.String("task_type", taskType),
			slog.String("task_id", taskID),
			slog.String("filename", filename))

		newImages = append(newImages, ImageInfo{
			Path:     object.Key,
			TaskType: taskType,
			TaskID:   taskID,
			Filename: filename,
			Size:     object.Size,
			ModTime:  object.LastModified,
		})
	}

	return newImages, nil
}

// MarkProcessed 标记图片已处理
func (s *Scanner) MarkProcessed(imagePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[imagePath] = time.Now()

	// 清理过期记录（超过24小时）
	if len(s.processed) > 10000 {
		s.cleanupProcessedLocked()
	}
}

// isProcessed 检查图片是否已处理
func (s *Scanner) isProcessed(imagePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.processed[imagePath]
	return exists
}

// cleanupProcessedLocked 清理过期的已处理记录（需要已加锁）
func (s *Scanner) cleanupProcessedLocked() {
	now := time.Now()
	for path, processedTime := range s.processed {
		if now.Sub(processedTime) > 24*time.Hour {
			delete(s.processed, path)
		}
	}
	s.log.Info("cleaned up processed images cache", slog.Int("remaining", len(s.processed)))
}

// isImageFile 判断是否为图片文件
func isImageFile(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasSuffix(lower, ".jpg") || 
	       strings.HasSuffix(lower, ".jpeg") || 
	       strings.HasSuffix(lower, ".png")
}

// parseImagePath 解析图片路径：任务类型/任务ID/文件名
func parseImagePath(objectKey, basePath string) (taskType, taskID, filename string) {
	// 移除base path前缀
	path := objectKey
	if basePath != "" {
		path = strings.TrimPrefix(path, basePath)
		path = strings.TrimPrefix(path, "/")
	}

	// 分割路径
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", ""
	}

	taskType = parts[0]
	taskID = parts[1]
	filename = parts[len(parts)-1]
	return
}

