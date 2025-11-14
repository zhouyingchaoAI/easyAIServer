package aianalysis

import (
	"context"
	"fmt"
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
	lastScanTime time.Time // 上次扫描时间（用于计算实际间隔）
	lastScanMu   sync.Mutex // 保护lastScanTime
	configuredInterval float64 // 配置的扫描间隔（秒）
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
func (s *Scanner) Start(intervalSec float64, onNewImages func([]ImageInfo)) {
	if intervalSec <= 0 {
		intervalSec = 10
	}

	// 保存配置的间隔
	s.configuredInterval = intervalSec

	// 支持小数秒，转换为毫秒
	intervalMs := int64(intervalSec * 1000)
	if intervalMs < 100 {
		intervalMs = 100 // 最小100毫秒，避免过于频繁
	}
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	go func() {
		// 立即扫描一次
		firstScanStart := time.Now()
		images, err := s.scanNewImages()
		firstScanDuration := time.Since(firstScanStart)
		if err == nil && len(images) > 0 {
			onNewImages(images)
		}
		
		// 记录首次扫描
		s.lastScanMu.Lock()
		s.lastScanTime = firstScanStart
		s.lastScanMu.Unlock()
		
		if err == nil {
			s.log.Info("scanner started, first scan completed",
				slog.Float64("configured_interval_sec", intervalSec),
				slog.Int64("configured_interval_ms", intervalMs),
				slog.Int("images_found", len(images)),
				slog.Duration("scan_duration_ms", firstScanDuration))
		}

		for {
			select {
			case <-s.stopScan:
				ticker.Stop()
				return
			case <-ticker.C:
				// 计算实际扫描间隔
				s.lastScanMu.Lock()
				actualInterval := time.Since(s.lastScanTime)
				s.lastScanTime = time.Now()
				s.lastScanMu.Unlock()
				
				scanStart := time.Now()
				images, err := s.scanNewImages()
				scanDuration := time.Since(scanStart)
				if err != nil {
					s.log.Error("scan minio failed", 
						slog.String("err", err.Error()),
						slog.Duration("scan_duration_ms", scanDuration),
						slog.Duration("actual_interval_ms", actualInterval),
						slog.Float64("configured_interval_sec", intervalSec))
					continue
				}
				
				// 记录每次扫描的详细信息（包括实际间隔和图片数量）
				if len(images) > 0 {
					queueAddStart := time.Now()
					onNewImages(images)
					queueAddDuration := time.Since(queueAddStart)
					
					// Info级别：记录实际扫描间隔、配置间隔、发现的图片数量
					s.log.Info("found new images", 
						slog.Int("count", len(images)),
						slog.Duration("scan_duration_ms", scanDuration),
						slog.Duration("queue_add_duration_ms", queueAddDuration),
						slog.Duration("actual_interval_ms", actualInterval),
						slog.Float64("configured_interval_sec", intervalSec),
						slog.Float64("interval_ratio", actualInterval.Seconds()/intervalSec),
						slog.String("note", "interval_ratio shows actual/configured, >1 means delayed, <1 means early"))
				} else {
					// 即使没有新图片，也记录扫描间隔（Info级别，便于分析）
					s.log.Info("scan completed, no new images found",
						slog.Duration("scan_duration_ms", scanDuration),
						slog.Duration("actual_interval_ms", actualInterval),
						slog.Float64("configured_interval_sec", intervalSec),
						slog.Float64("interval_ratio", actualInterval.Seconds()/intervalSec),
						slog.String("note", "interval_ratio shows actual/configured, >1 means delayed, <1 means early"))
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
	scanStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 列举所有对象（只扫描basePath下的，不扫描告警路径）
	listStart := time.Now()
	objectCh := s.minio.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    s.basePath,
		Recursive: true,
	})
	listDuration := time.Since(listStart)

	var newImages []ImageInfo
	var totalScanned int
	var skippedProcessed int
	var skippedInvalidPath int
	var skippedPreview int
	var skippedOther int
	var skippedAlertPath int // 跳过告警路径的数量
	
	// 按路径统计扫描的图片数量
	pathStats := make(map[string]int) // path -> count
	
	for object := range objectCh {
		totalScanned++
		if object.Err != nil {
			s.log.Warn("list object error", slog.String("err", object.Err.Error()))
			continue
		}

		// 首先跳过告警路径中的图片（已经推理过了，不需要扫描）
		// 这样可以避免遍历告警图片，提高扫描效率
		if s.alertBasePath != "" && strings.HasPrefix(object.Key, s.alertBasePath) {
			skippedAlertPath++
			// 统计告警路径下的图片数量（用于日志）
			alertPath := s.alertBasePath
			if idx := strings.Index(strings.TrimPrefix(object.Key, s.alertBasePath), "/"); idx > 0 {
				alertPath = s.alertBasePath + strings.TrimPrefix(object.Key, s.alertBasePath)[:idx+1]
			}
			pathStats[alertPath]++
			continue
		}

		// 过滤非图片文件
		if !isImageFile(object.Key) {
			skippedOther++
			continue
		}

		// 跳过.keep等标记文件
		if strings.Contains(object.Key, "/.") {
			skippedOther++
			continue
		}
		
		// 跳过配置文件
		if strings.HasSuffix(object.Key, "algo_config.json") || strings.HasSuffix(object.Key, ".json") {
			skippedOther++
			continue
		}

		// 检查是否已处理
		if s.isProcessed(object.Key) {
			skippedProcessed++
			// 统计已处理图片的路径
			path := s.getPathPrefix(object.Key)
			pathStats[path]++
			continue
		}

		// 跳过预览图（preview开头的图片不参与推理）
		filename := object.Key[strings.LastIndex(object.Key, "/")+1:]
		if strings.HasPrefix(filename, "preview_") {
			skippedPreview++
			continue
		}

		// 解析路径：任务类型/任务ID/文件名
		taskType, taskID, filename := parseImagePath(object.Key, s.basePath)
		if taskType == "" || taskID == "" {
			skippedInvalidPath++
			s.log.Warn("skipping image with invalid path structure",
				slog.String("path", object.Key),
				slog.String("base_path", s.basePath),
				slog.String("note", "expected format: basePath/taskType/taskID/filename.jpg"))
			continue
		}

		// 统计新图片的路径
		path := s.getPathPrefix(object.Key)
		pathStats[path]++

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

	scanDuration := time.Since(scanStart)
	
	// 构建路径统计信息（用于日志）
	var pathStatsList []string
	for path, count := range pathStats {
		if count > 0 {
			pathStatsList = append(pathStatsList, fmt.Sprintf("%s:%d", path, count))
		}
	}
	
	// 记录扫描统计信息（包括路径统计）
	if len(newImages) > 0 || skippedInvalidPath > 0 || skippedAlertPath > 0 || (totalScanned > 0 && len(newImages) == 0 && skippedProcessed < totalScanned) {
		s.log.Info("scan statistics",
			slog.Int("total_scanned", totalScanned),
			slog.Int("new_images", len(newImages)),
			slog.Int("skipped_processed", skippedProcessed),
			slog.Int("skipped_invalid_path", skippedInvalidPath),
			slog.Int("skipped_preview", skippedPreview),
			slog.Int("skipped_alert_path", skippedAlertPath),
			slog.Int("skipped_other", skippedOther),
			slog.String("base_path", s.basePath),
			slog.String("alert_base_path", s.alertBasePath),
			slog.String("path_stats", strings.Join(pathStatsList, ", ")),
			slog.Duration("list_objects_duration_ms", listDuration),
			slog.Duration("total_scan_duration_ms", scanDuration))
	}

	return newImages, nil
}

// getPathPrefix 获取路径前缀（用于统计）
// 例如: "人数统计/门口2/20251114-164712.618.jpg" -> "人数统计/门口2/"
func (s *Scanner) getPathPrefix(fullPath string) string {
	// 移除basePath前缀
	path := fullPath
	if s.basePath != "" {
		path = strings.TrimPrefix(path, s.basePath)
		path = strings.TrimPrefix(path, "/")
	}
	
	// 获取任务类型/任务ID/部分
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1] + "/"
	} else if len(parts) == 1 {
		return parts[0] + "/"
	}
	return "unknown/"
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

