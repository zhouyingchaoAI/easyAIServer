package aianalysis

import (
	"bytes"
	"context"
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
	"easydarwin/internal/data/model"
	"easydarwin/internal/plugin/frameextractor"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
)

// Scheduler 推理调度器
type Scheduler struct {
	registry              *AlgorithmRegistry
	minio                 *minio.Client
	bucket                string
	alertBasePath         string // 告警图片存储路径前缀
	mq                    MessageQueue
	log                   *slog.Logger
	semaphore             chan struct{}          // 限制并发数
	saveOnlyWithDetection bool                   // 只保存有检测结果的告警
	httpClient            *http.Client           // 优化的HTTP客户端
	alertBatchWriter      *data.AlertBatchWriter // 批量写入告警
	monitor               *PerformanceMonitor    // 性能监控器（用于记录推理时间）
	scanner               *Scanner               // 扫描器（用于标记图片已处理）

	// 移动锁：确保同一task_id的图片按顺序移动，避免并发错位
	moveLocks       map[string]*sync.Mutex
	moveLockLastUse map[string]time.Time // 记录每个锁的最后使用时间，用于清理
	moveLockMu      sync.Mutex

	// 图片移动并发控制：限制同时进行的图片移动操作数
	moveSemaphore chan struct{}

	// 正在推理的图片集合（用于清理时保护，只保护正在推理的图片，不保护队列中等待的）
	inferringImages  map[string]bool
	inferringMu      sync.RWMutex
	activeInferences int32 // 当前正在推理的数量（用于监控）

	// 即将推理的图片集合（用于清理时保护，在Pop之前标记，避免时间窗口漏洞）
	pendingInferringImages map[string]time.Time // 改为记录时间，用于超时清理
	pendingMu              sync.RWMutex

	// 处理完成回调（用于通知service增加processedCount）
	onProcessedCallback func()
}

const tripwireTaskType = "绊线人数统计"

// InferenceResult 推理结果（用于异步保存告警）
type InferenceResult struct {
	Alert         *model.Alert
	ImagePath     string
	AlertBasePath string
}

// NewScheduler 创建调度器
func NewScheduler(registry *AlgorithmRegistry, minioClient *minio.Client, bucket, alertBasePath string, mq MessageQueue, maxConcurrent int, saveOnlyWithDetection bool, alertBatchWriter *data.AlertBatchWriter, monitor *PerformanceMonitor, scanner *Scanner, logger *slog.Logger, moveConcurrent int) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	// 图片移动并发数配置
	if moveConcurrent <= 0 {
		moveConcurrent = 50 // 默认50个并发
	}

	// 优化HTTP客户端配置 - 启用连接复用以提高性能，避免连接数持续增长导致变慢
	transport := &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       30 * time.Second, // 缩短到30秒，更快释放空闲连接
		DisableCompression:    false,
		ResponseHeaderTimeout: 30 * time.Second, // 缩短到30秒，快速失败
		ExpectContinueTimeout: 1 * time.Second,
		// 关键：启用连接复用，避免每次请求都创建新连接，提高性能并减少资源消耗
		DisableKeepAlives: false, // 改为false，启用连接复用
		// 优化连接超时配置
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // 缩短到5秒，快速失败
			KeepAlive: 30 * time.Second, // 启用keep-alive，30秒保活
		}).DialContext,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // 缩短到60秒，快速失败，避免长时间卡死
	}

	scheduler := &Scheduler{
		registry:               registry,
		minio:                  minioClient,
		bucket:                 bucket,
		alertBasePath:          alertBasePath,
		mq:                     mq,
		log:                    logger,
		semaphore:              make(chan struct{}, maxConcurrent),
		saveOnlyWithDetection:  saveOnlyWithDetection,
		httpClient:             httpClient,
		alertBatchWriter:       alertBatchWriter,
		monitor:                monitor,
		scanner:                scanner,
		moveLocks:              make(map[string]*sync.Mutex),
		moveLockLastUse:        make(map[string]time.Time),
		inferringImages:        make(map[string]bool),
		pendingInferringImages: make(map[string]time.Time),
		onProcessedCallback:    nil,
		moveSemaphore:          make(chan struct{}, moveConcurrent),
	}

	// 启动移动锁定期清理
	scheduler.startMoveLockCleanup()

	// 启动pending标记超时清理
	scheduler.startPendingCleanup()

	return scheduler
}

// cleanupPendingInferences 清理超过3分钟未处理的pending标记
func (s *Scheduler) cleanupPendingInferences() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	now := time.Now()
	cleanupThreshold := 3 * time.Minute // 缩短到3分钟
	cleanedCount := 0

	for path, markTime := range s.pendingInferringImages {
		if now.Sub(markTime) > cleanupThreshold {
			delete(s.pendingInferringImages, path)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		s.log.Warn("cleaned up stale pending inference marks",
			slog.Int("cleaned_count", cleanedCount),
			slog.Int("remaining_count", len(s.pendingInferringImages)),
			slog.String("note", "these images were marked as pending but never processed"))
	}
}

// startPendingCleanup 启动pending标记定期清理
func (s *Scheduler) startPendingCleanup() {
	ticker := time.NewTicker(1 * time.Minute) // 缩短到每1分钟清理一次
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			s.cleanupPendingInferences()
		}
	}()
}

// SetOnProcessedCallback 设置处理完成回调
func (s *Scheduler) SetOnProcessedCallback(callback func()) {
	s.onProcessedCallback = callback
}

// IsImageInferring 检查图片是否正在推理中（用于清理时保护）
func (s *Scheduler) IsImageInferring(imagePath string) bool {
	s.inferringMu.RLock()
	defer s.inferringMu.RUnlock()
	// 规范化路径格式（使用filepath.ToSlash确保格式一致）
	normalizedPath := filepath.ToSlash(imagePath)
	return s.inferringImages[normalizedPath]
}

// MarkPendingInference 标记图片为即将推理（在Pop之前调用，避免时间窗口漏洞）
func (s *Scheduler) MarkPendingInference(imagePath string) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	// 规范化路径格式
	normalizedPath := filepath.ToSlash(imagePath)
	// 检查是否已经标记（避免重复标记）
	if _, exists := s.pendingInferringImages[normalizedPath]; exists {
		return false // 已经标记过
	}
	s.pendingInferringImages[normalizedPath] = time.Now()
	return true // 成功标记
}

// IsImagePendingInference 检查图片是否即将推理（用于清理时保护）
func (s *Scheduler) IsImagePendingInference(imagePath string) bool {
	s.pendingMu.RLock()
	defer s.pendingMu.RUnlock()
	// 规范化路径格式
	normalizedPath := filepath.ToSlash(imagePath)
	_, exists := s.pendingInferringImages[normalizedPath]
	return exists
}

// UnmarkPendingInference 取消标记图片为即将推理（在ScheduleInference开始时调用，转换为正在推理）
func (s *Scheduler) UnmarkPendingInference(imagePath string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	// 规范化路径格式
	normalizedPath := filepath.ToSlash(imagePath)
	delete(s.pendingInferringImages, normalizedPath)
}

// IsImageProtected 检查图片是否受到保护（即将推理或正在推理）
func (s *Scheduler) IsImageProtected(imagePath string) bool {
	// 检查是否即将推理
	if s.IsImagePendingInference(imagePath) {
		return true
	}
	// 检查是否正在推理
	if s.IsImageInferring(imagePath) {
		return true
	}
	return false
}

// CheckImageExists 检查图片是否存在于MinIO（用于Pop后检查）
func (s *Scheduler) CheckImageExists(imagePath string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.minio.StatObject(ctx, s.bucket, imagePath, minio.StatObjectOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

// ScheduleInference 调度推理
func (s *Scheduler) ScheduleInference(image ImageInfo) {
	// 根据任务类型选择算法实例（绊线任务需要绑定端点）
	algorithm, selectErr := s.selectAlgorithmForImage(image)
	if algorithm == nil {
		logArgs := []any{
			slog.String("task_type", image.TaskType),
			slog.String("task_id", image.TaskID),
			slog.String("image", image.Path),
		}
		if selectErr != nil {
			logArgs = append(logArgs, slog.String("reason", selectErr.Error()))
			s.log.Error("failed to select algorithm, deleting image", logArgs...)
		} else {
			s.log.Debug("no algorithm for task type, deleting image", logArgs...)
		}

		// 没有算法服务，删除图片避免积压
		if err := s.deleteImage(image.Path); err != nil {
			s.log.Warn("failed to delete image without algorithm",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		} else {
			// 图片已删除，标记为已处理（避免重复扫描）
			if s.scanner != nil {
				s.scanner.MarkProcessed(image.Path)
			}
		}

		return
	}

	scheduleStart := time.Now()

	// 限流
	semaphoreWaitStart := time.Now()
	s.semaphore <- struct{}{}
	semaphoreWaitDuration := time.Since(semaphoreWaitStart)
	atomic.AddInt32(&s.activeInferences, 1)
	defer func() {
		<-s.semaphore
		atomic.AddInt32(&s.activeInferences, -1)
	}()

	// 将"即将推理"转换为"正在推理"（如果之前已标记为pending）
	normalizedPath := filepath.ToSlash(image.Path)
	wasPending := s.IsImagePendingInference(image.Path)
	if wasPending {
		s.UnmarkPendingInference(image.Path)
	}

	// 标记图片正在推理（用于清理时保护）
	s.inferringMu.Lock()
	s.inferringImages[normalizedPath] = true
	s.inferringMu.Unlock()

	// 确保推理完成后移除标记
	defer func() {
		s.inferringMu.Lock()
		delete(s.inferringImages, normalizedPath)
		s.inferringMu.Unlock()
	}()

	s.log.Info("scheduling inference",
		slog.String("image", image.Path),
		slog.String("task_type", image.TaskType),
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("endpoint", algorithm.Endpoint),
		slog.Duration("semaphore_wait_ms", semaphoreWaitDuration))

	// 调用选中的算法实例
	inferStart := time.Now()
	s.inferAndSave(image, *algorithm)
	totalScheduleDuration := time.Since(scheduleStart)
	inferDuration := time.Since(inferStart)

	// Debug级别，避免日志过多
	s.log.Debug("inference scheduled completed",
		slog.String("task_id", image.TaskID),
		slog.String("image", image.Filename),
		slog.Duration("inference_duration_ms", inferDuration),
		slog.Duration("total_schedule_duration_ms", totalScheduleDuration))
}

// inferAndSave 调用算法推理并保存结果
func (s *Scheduler) inferAndSave(image ImageInfo, algorithm conf.AlgorithmService) {
	inferStart := time.Now()

	// 处理前检查图片是否存在（避免处理已删除的图片）
	statStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, statErr := s.minio.StatObject(ctx, s.bucket, image.Path, minio.StatObjectOptions{})
	cancel()
	statDuration := time.Since(statStart)

	if statErr != nil {
		// 图片不存在，跳过处理（可能是被丢弃时删除了）
		s.log.Warn("image not found in MinIO, skipping inference",
			slog.String("path", image.Path),
			slog.String("task_id", image.TaskID),
			slog.String("err", statErr.Error()),
			slog.String("note", "image may have been deleted when dropped from queue"))

		// 标记为已处理，避免重复扫描
		if s.scanner != nil {
			s.scanner.MarkProcessed(image.Path)
		}
		return
	}

	// 生成预签名URL（带重试机制）
	// 注意：MinIO SDK生成签名时使用UTC时间，但MinIO服务器验证时使用CST时间
	// 时差8小时，因此需要增加有效期以补偿时区差
	// 1小时有效期 + 8小时时差 + 1小时缓冲 = 10小时
	presignedExpiry := 10 * time.Hour

	var presignedURL *url.URL
	var err error
	maxRetries := 3
	retryDelay := 1 * time.Second

	presignStart := time.Now()
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		presignedURL, err = s.minio.PresignedGetObject(ctx, s.bucket, image.Path, presignedExpiry, nil)
		cancel()
		presignDuration := time.Since(presignStart)

		if err == nil {
			if i > 0 {
				s.log.Info("presigned URL generated after retry",
					slog.Int("attempt", i+1),
					slog.String("path", image.Path),
					slog.Duration("presign_duration_ms", presignDuration))
			}
			break
		}

		// 记录错误详情
		s.log.Warn("failed to generate presigned URL, retrying...",
			slog.Int("attempt", i+1),
			slog.Int("max_retries", maxRetries),
			slog.String("path", image.Path),
			slog.String("err", err.Error()),
			slog.String("err_type", fmt.Sprintf("%T", err)))

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
	}

	presignDuration := time.Since(presignStart)
	if err != nil {
		s.log.Error("failed to generate presigned URL after retries",
			slog.String("path", image.Path),
			slog.String("err", err.Error()),
			slog.String("err_type", fmt.Sprintf("%T", err)),
			slog.Duration("presign_duration_ms", presignDuration),
			slog.Duration("stat_duration_ms", statDuration))
		// 预签名失败，删除图片避免积压
		s.deleteImageWithReason(image.Path, "presign_failed")
		return
	}

	// 读取算法配置（如果存在）
	var algoConfig map[string]interface{}
	var algoConfigURL string
	if fxService := s.getFrameExtractorService(); fxService != nil {
		if configBytes, err := fxService.GetAlgorithmConfig(image.TaskID); err == nil {
			if err := json.Unmarshal(configBytes, &algoConfig); err != nil {
				s.log.Warn("failed to parse algo config",
					slog.String("task_id", image.TaskID),
					slog.String("err", err.Error()))
			}

			// 生成配置文件的预签名URL
			// 同样需要补偿时区差（10小时有效期）
			configPath := fxService.GetAlgorithmConfigPath(image.TaskID)
			if configPath != "" {
				configURLCtx, configURLCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer configURLCancel()

				presignedConfigURL, err := s.minio.PresignedGetObject(configURLCtx, s.bucket, configPath, presignedExpiry, nil)
				if err == nil {
					algoConfigURL = presignedConfigURL.String()
				} else {
					s.log.Warn("failed to generate config presigned URL",
						slog.String("config_path", configPath),
						slog.String("err", err.Error()))
				}
			}
		}
	}

	// 构建推理请求
	req := conf.InferenceRequest{
		ImageURL:      presignedURL.String(),
		TaskID:        image.TaskID,
		TaskType:      image.TaskType,
		ImagePath:     image.Path,
		AlgoConfig:    algoConfig,
		AlgoConfigURL: algoConfigURL,
	}

	// 记录推理请求详情
	s.log.Info("收到推理请求",
		slog.String("任务ID", image.TaskID),
		slog.String("任务类型", image.TaskType),
		slog.String("图片路径", image.Path),
		slog.String("图片URL", presignedURL.String()),
		slog.String("配置文件URL", algoConfigURL),
		slog.Duration("stat_duration_ms", statDuration),
		slog.Duration("presign_duration_ms", presignDuration))

	// 记录推理开始时间
	algorithmCallStart := time.Now()

	// 记录请求发送
	if s.monitor != nil {
		s.monitor.RecordRequestSent()
	}

	// 调用算法服务
	resp, err := s.callAlgorithm(algorithm, req)
	algorithmCallDuration := time.Since(algorithmCallStart)

	// 记录响应接收（无论成功或失败）
	if s.monitor != nil {
		s.monitor.RecordResponseReceived()
	}

	if err != nil {
		// 检查是否是404错误（图片不存在）
		is404Error := strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found")

		// ❌ 推理调用失败，记录日志（不注销服务，服务状态由心跳管理）
		s.registry.RecordInferenceFailure(algorithm.Endpoint, algorithm.ServiceID)

		// 记录失败到监控器
		if s.monitor != nil {
			// 计算失败前的耗时
			failedTime := time.Since(algorithmCallStart).Milliseconds()
			s.monitor.RecordInference(failedTime, false)
		}

		// 通知处理完成（推理失败也算处理）
		if s.onProcessedCallback != nil {
			s.onProcessedCallback()
		}

		if is404Error {
			// 404错误：图片不存在，跳过处理，不删除（可能已经被删除）
			s.log.Warn("algorithm inference failed: image not found (404)",
				slog.String("algorithm", algorithm.ServiceID),
				slog.String("endpoint", algorithm.Endpoint),
				slog.String("image", image.Path),
				slog.String("err", err.Error()),
				slog.Duration("algorithm_call_duration_ms", algorithmCallDuration),
				slog.String("note", "image may have been deleted from MinIO, skipping"))
		} else {
			s.log.Error("algorithm inference failed",
				slog.String("algorithm", algorithm.ServiceID),
				slog.String("endpoint", algorithm.Endpoint),
				slog.String("image", image.Path),
				slog.String("err", err.Error()),
				slog.Duration("algorithm_call_duration_ms", algorithmCallDuration),
				slog.String("note", "service will be removed by heartbeat timeout if truly offline"))

			// 非404错误，删除图片（避免积压，图片已尝试推理过）
			if delErr := s.deleteImageWithReason(image.Path, "inference_call_failed"); delErr != nil {
				s.log.Error("failed to delete image after inference failure",
					slog.String("path", image.Path),
					slog.String("err", delErr.Error()))
			} else {
				s.log.Info("image deleted after inference failure",
					slog.String("path", image.Path),
					slog.String("algorithm", algorithm.ServiceID))
			}
		}

		// 标记为已处理（避免重复扫描）
		if s.scanner != nil {
			s.scanner.MarkProcessed(image.Path)
		}
		return
	}

	// 计算实际推理耗时
	actualInferenceTime := time.Since(algorithmCallStart).Milliseconds()

	if !resp.Success {
		// 记录失败到监控器
		if s.monitor != nil {
			actualInferenceTime := time.Since(algorithmCallStart).Milliseconds()
			s.monitor.RecordInference(actualInferenceTime, false)
		}

		// 通知处理完成（推理失败也算处理）
		if s.onProcessedCallback != nil {
			s.onProcessedCallback()
		}

		s.log.Warn("inference not successful",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("image", image.Path),
			slog.String("error", resp.Error))
		// 推理失败，删除图片
		if err := s.deleteImageWithReason(image.Path, "inference_failed"); err != nil {
			s.log.Error("failed to delete image after inference failure",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		} else {
			// 图片已删除，标记为已处理（避免重复扫描）
			if s.scanner != nil {
				s.scanner.MarkProcessed(image.Path)
			}
		}
		return
	}

	// ✅ 推理成功，记录成功（增加调用计数，记录响应时间）
	// 使用算法服务返回的推理时间，如果为0则使用实际测量的时间
	reportedTimeMs := int64(resp.InferenceTimeMs)
	if reportedTimeMs <= 0 {
		reportedTimeMs = actualInferenceTime
	}
	s.registry.RecordInferenceSuccess(algorithm.Endpoint, reportedTimeMs)

	// 记录到性能监控器（使用算法服务返回的推理时间，而不是总处理时间）
	if s.monitor != nil {
		s.monitor.RecordInference(reportedTimeMs, true)
	}

	// 通知处理完成（推理成功）
	if s.onProcessedCallback != nil {
		s.onProcessedCallback()
	}

	s.log.Debug("inference succeeded, call count incremented and response time recorded",
		slog.String("endpoint", algorithm.Endpoint),
		slog.String("service_id", algorithm.ServiceID),
		slog.Int64("response_time_ms", reportedTimeMs),
		slog.Int64("actual_time_ms", actualInferenceTime))

	// 提取检测个数
	detectionCount := extractDetectionCount(resp.Result)

	// 记录推理结果详情
	s.log.Info("inference result received",
		slog.String("image", image.Path),
		slog.String("algorithm", algorithm.ServiceID),
		slog.Int("detection_count", detectionCount),
		slog.Float64("confidence", resp.Confidence),
		slog.Int64("inference_time_ms", actualInferenceTime),
		slog.Duration("algorithm_call_duration_ms", algorithmCallDuration),
		slog.Any("result", resp.Result))

	// 无检测结果：直接删除原路径图片并返回（不保存告警，不推送消息）
	if detectionCount == 0 {
		s.log.Info("no detection result, deleting image",
			slog.String("image", image.Path),
			slog.String("task_id", image.TaskID),
			slog.String("task_type", image.TaskType),
			slog.String("algorithm", algorithm.ServiceID))

		if err := s.deleteImageWithReason(image.Path, "no_detection"); err != nil {
			s.log.Error("failed to delete image with no detection",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		} else {
			s.log.Info("image deleted successfully (no detection)",
				slog.String("path", image.Path),
				slog.String("task_id", image.TaskID))
			// 图片已删除，标记为已处理（避免重复扫描）
			if s.scanner != nil {
				s.scanner.MarkProcessed(image.Path)
			}
		}

		return
	}

	// 检查是否保存告警图片（不影响告警信息的保存和推送）
	shouldSaveImage := s.shouldSaveAlertImage(image.TaskID, algoConfig)

	// 准备告警图片路径
	var alertImagePath string
	var alertImageURL string

	// 只有配置为保存图片时才移动/保存图片
	if shouldSaveImage && s.alertBasePath != "" && detectionCount > 0 {
		// 构建目标告警路径（保存告警时使用目标路径）
		// 使用 ImageInfo 中已解析的 Filename，避免重复解析导致混淆
		targetAlertPath := fmt.Sprintf("%s%s/%s/%s", s.alertBasePath, image.TaskType, image.TaskID, image.Filename)

		s.log.Info("constructing alert image path",
			slog.String("task_id", image.TaskID),
			slog.String("task_type", image.TaskType),
			slog.String("filename", image.Filename),
			slog.String("src_path", image.Path),
			slog.String("target_path", targetAlertPath))

		// 使用目标告警路径保存（确保URL可以访问）
		alertImagePath = targetAlertPath
		// 不预先生成URL，节省时间（API返回时按需生成）
		alertImageURL = ""

		// 在后台异步执行图片移动
		// 注意：传递所有必要的参数到闭包，避免并发问题
		// 使用移动锁确保同一task_id的图片按顺序移动，避免内容错位
		// 使用并发控制限制同时进行的移动操作数
		go func(srcPath, dstPath, taskID, taskType, filename string) {
			// 获取并发控制信号量
			s.moveSemaphore <- struct{}{}
			defer func() { <-s.moveSemaphore }()

			// 获取该task_id的移动锁，确保顺序移动
			lock := s.getMoveLock(taskID)
			lock.Lock()
			defer lock.Unlock()

			if err := s.moveImageToAlertPathAsync(srcPath, dstPath); err != nil {
				s.log.Error("async image move failed",
					slog.String("task_id", taskID),
					slog.String("task_type", taskType),
					slog.String("filename", filename),
					slog.String("src", srcPath),
					slog.String("dst", dstPath),
					slog.String("err", err.Error()))
				// 移动失败不影响告警，原路径图片仍然可用
			} else {
				s.log.Info("async image move succeeded",
					slog.String("task_id", taskID),
					slog.String("task_type", taskType),
					slog.String("filename", filename),
					slog.String("src", srcPath),
					slog.String("dst", dstPath))
			}
		}(image.Path, targetAlertPath, image.TaskID, image.TaskType, image.Filename)

	} else if shouldSaveImage {
		// 未配置告警路径，但需要保存图片，使用原路径
		alertImagePath = image.Path
		// 不预先生成URL，节省时间（API返回时按需生成）
		alertImageURL = ""
	} else {
		// 不保存图片，路径为空
		alertImagePath = ""
		alertImageURL = ""

		// 删除原图片（不保存告警图片）
		s.log.Info("alert image saving disabled for task, deleting original image",
			slog.String("task_id", image.TaskID),
			slog.String("task_type", image.TaskType),
			slog.String("image", image.Path),
			slog.String("note", "alert will be saved without image"))

		if err := s.deleteImageWithReason(image.Path, "alert_image_save_disabled"); err != nil {
			s.log.Error("failed to delete image after alert image disabled",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		}
	}

	// 保存告警到数据库
	resultJSON, _ := json.Marshal(resp.Result)
	alert := &model.Alert{
		TaskID:          image.TaskID,
		TaskType:        image.TaskType,
		ImagePath:       alertImagePath,
		ImageURL:        alertImageURL,
		AlgorithmID:     algorithm.ServiceID,
		AlgorithmName:   algorithm.Name,
		Result:          string(resultJSON),
		Confidence:      resp.Confidence,
		DetectionCount:  detectionCount,
		InferenceTimeMs: int(actualInferenceTime),
		CreatedAt:       time.Now(),
	}

	// 验证任务ID与图片路径的一致性（只在有图片路径时验证）
	if alertImagePath != "" && strings.Contains(alertImagePath, "/") {
		pathParts := strings.Split(alertImagePath, "/")
		if len(pathParts) >= 3 {
			pathTaskID := pathParts[len(pathParts)-2] // 倒数第二个部分应该是task_id
			if pathTaskID != image.TaskID {
				s.log.Error("task_id mismatch detected!",
					slog.String("alert_task_id", image.TaskID),
					slog.String("path_task_id", pathTaskID),
					slog.String("image_path", alertImagePath),
					slog.String("original_path", image.Path))
			}
		}
	}

	// 使用批量写入器添加告警
	saveStart := time.Now()
	if err := s.alertBatchWriter.Add(alert); err != nil {
		s.log.Error("failed to add alert to batch writer",
			slog.String("task_id", image.TaskID),
			slog.String("err", err.Error()),
			slog.Duration("save_duration_ms", time.Since(saveStart)))
		return
	}
	saveDuration := time.Since(saveStart)

	s.log.Debug("alert record prepared for batch save",
		slog.String("task_id", alert.TaskID),
		slog.String("task_type", alert.TaskType),
		slog.String("image_path", alert.ImagePath),
		slog.String("original_path", image.Path),
		slog.Duration("save_duration_ms", saveDuration))

	// 推送到消息队列
	mqStart := time.Now()
	var mqDuration time.Duration
	if s.mq != nil {
		if err := s.mq.PublishAlert(*alert); err != nil {
			mqDuration = time.Since(mqStart)
			s.log.Error("failed to publish alert to MQ",
				slog.String("task_id", image.TaskID),
				slog.String("err", err.Error()),
				slog.Duration("mq_duration_ms", mqDuration))
		} else {
			mqDuration = time.Since(mqStart)
			s.log.Debug("alert published to MQ",
				slog.Uint64("alert_id", uint64(alert.ID)),
				slog.String("task_id", image.TaskID),
				slog.Duration("mq_duration_ms", mqDuration))
		}
	}

	// 记录完整的推理流程耗时
	totalInferDuration := time.Since(inferStart)
	// Info级别，但只记录关键信息，详细耗时在Debug级别
	s.log.Info("inference completed and queued for batch save",
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("task_id", image.TaskID),
		slog.String("task_type", image.TaskType),
		slog.Int("detection_count", detectionCount),
		slog.Float64("confidence", resp.Confidence),
		slog.Int64("inference_time_ms", actualInferenceTime),
		slog.Int("batch_queue_size", s.alertBatchWriter.GetQueueSize()),
		slog.Duration("algorithm_call_duration_ms", algorithmCallDuration)) // 只记录主要耗时

	// 详细耗时记录在Debug级别
	s.log.Debug("inference detailed timing",
		slog.String("task_id", image.TaskID),
		slog.String("image", image.Filename),
		slog.Duration("stat_duration_ms", statDuration),
		slog.Duration("presign_duration_ms", presignDuration),
		slog.Duration("algorithm_call_duration_ms", algorithmCallDuration),
		slog.Duration("save_duration_ms", saveDuration),
		slog.Duration("mq_duration_ms", mqDuration),
		slog.Duration("total_infer_duration_ms", totalInferDuration))

	// 推理成功并已保存告警，标记图片为已处理
	if s.scanner != nil {
		s.scanner.MarkProcessed(image.Path)
	}

	// 如果未配置告警路径且需要保存图片（使用了原路径），告警已保存后删除原文件
	// 注意：删除后alert记录中的ImagePath会失效，但用户要求总是删除原路径
	// 如果shouldSaveImage为false，图片已经在上面删除了，这里不需要再删除
	if shouldSaveImage && s.alertBasePath == "" && alertImagePath == image.Path && alertImagePath != "" {
		if err := s.deleteImageWithReason(image.Path, "after_inference_no_alert_path"); err != nil {
			s.log.Error("failed to delete original image after inference (no alert path)",
				slog.String("path", image.Path),
				slog.String("task_id", image.TaskID),
				slog.String("err", err.Error()))
		} else {
			s.log.Info("original image deleted after inference (no alert path)",
				slog.String("path", image.Path),
				slog.String("task_id", image.TaskID),
				slog.String("alert_id", fmt.Sprintf("%d", alert.ID)))
		}
	}
}

// GetActiveInferenceCount 返回当前正在进行推理的数量（即semaphore占用数）
func (s *Scheduler) GetActiveInferenceCount() int32 {
	return atomic.LoadInt32(&s.activeInferences)
}

// GetMaxConcurrent 返回调度器允许的最大并发数
func (s *Scheduler) GetMaxConcurrent() int {
	return cap(s.semaphore)
}

// callAlgorithm HTTP调用算法服务（带重试机制）
func (s *Scheduler) callAlgorithm(algorithm conf.AlgorithmService, req conf.InferenceRequest) (*conf.InferenceResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	maxRetries := 2               // 减少到2次重试（总共3次尝试），避免长时间卡死
	retryDelay := 1 * time.Second // 初始重试延迟1秒
	var lastErr error

	// 判断是否为连接错误的辅助函数
	isConnectionError := func(err error) bool {
		if err == nil {
			return false
		}
		errStr := err.Error()
		return strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "connection timeout") ||
			strings.Contains(errStr, "no such host") ||
			strings.Contains(errStr, "dial tcp") ||
			strings.Contains(errStr, "network is unreachable")
	}

	for i := 0; i < maxRetries; i++ {
		// 使用全局优化的HTTP客户端，启用连接复用以提高性能
		// 注意：全局客户端已配置连接复用，可以大幅提高并发请求速度
		httpReq, err := http.NewRequest("POST", algorithm.Endpoint, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("create request failed: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		// 不设置Connection: close，使用连接复用

		httpResp, err := s.httpClient.Do(httpReq)

		if err == nil {
			defer httpResp.Body.Close()

			if httpResp.StatusCode == http.StatusOK {
				var resp conf.InferenceResponse
				// 先读取全部响应体，避免在读取时连接被关闭导致错误
				bodyBytes, readErr := io.ReadAll(httpResp.Body)
				if readErr != nil {
					lastErr = fmt.Errorf("read response failed: %w", readErr)
					continue // 继续重试
				}

				// 解析JSON响应
				if err := json.Unmarshal(bodyBytes, &resp); err != nil {
					bodyPreview := string(bodyBytes)
					if len(bodyPreview) > 200 {
						bodyPreview = bodyPreview[:200] + "..."
					}
					lastErr = fmt.Errorf("decode response failed: %w (body: %s)", err, bodyPreview)
					continue // 继续重试
				}

				if i > 0 {
					s.log.Info("algorithm call succeeded after retry",
						slog.Int("attempt", i+1),
						slog.String("endpoint", algorithm.Endpoint))
				}
				// 成功时，defer cancel()会在函数返回时执行
				return &resp, nil
			}

			// 非200状态码
			body, _ := io.ReadAll(httpResp.Body)
			bodyStr := string(body)
			lastErr = fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, bodyStr)

			// 404错误（图片不存在）不重试，直接返回
			if httpResp.StatusCode == http.StatusNotFound || strings.Contains(bodyStr, "404") || strings.Contains(bodyStr, "Not Found") {
				s.log.Warn("image not found (404), skipping retry",
					slog.String("endpoint", algorithm.Endpoint),
					slog.String("image_url", req.ImageURL),
					slog.String("error", lastErr.Error()),
					slog.String("note", "image may have been deleted from MinIO"))
				return nil, lastErr
			}
		} else {
			lastErr = err
		}

		// 智能重试：连接错误不重试，快速失败
		if lastErr != nil && isConnectionError(lastErr) {
			s.log.Warn("connection error detected, skipping retry (fast fail)",
				slog.Int("attempt", i+1),
				slog.String("endpoint", algorithm.Endpoint),
				slog.String("error", lastErr.Error()),
				slog.String("error_type", fmt.Sprintf("%T", lastErr)),
				slog.String("note", "connection errors indicate service is offline, no point retrying"))
			// 连接错误，不重试，直接返回
			return nil, fmt.Errorf("connection error (service likely offline): %w", lastErr)
		}

		// 记录错误详情
		s.log.Warn("algorithm call failed, retrying...",
			slog.Int("attempt", i+1),
			slog.Int("max_retries", maxRetries),
			slog.String("endpoint", algorithm.Endpoint),
			slog.String("error", lastErr.Error()),
			slog.String("error_type", fmt.Sprintf("%T", lastErr)))

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}

		// 重新marshal请求体（bytes.NewReader可能已被读取）
		reqBody, _ = json.Marshal(req)
	}

	return nil, fmt.Errorf("algorithm call failed after %d retries: %w", maxRetries, lastErr)
}

// extractDetectionCount 从推理结果中提取检测个数
func extractDetectionCount(result interface{}) int {
	if result == nil {
		return 0
	}

	// 尝试将result转换为map
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return 0
	}

	// 特殊处理：绊线统计算法 - 优先从 line_crossing 获取穿越统计数
	if lineCrossing, ok := resultMap["line_crossing"]; ok {
		if lineCrossingMap, ok := lineCrossing.(map[string]interface{}); ok {
			// 遍历所有区域，累加穿越统计数
			totalCrossingCount := 0
			for _, regionData := range lineCrossingMap {
				if regionMap, ok := regionData.(map[string]interface{}); ok {
					if count, ok := regionMap["count"]; ok {
						switch v := count.(type) {
						case int:
							totalCrossingCount += v
						case float64:
							totalCrossingCount += int(v)
						}
					}
				}
			}
			// 如果有穿越统计数，优先返回（这才是绊线算法的核心数据）
			if totalCrossingCount > 0 {
				return totalCrossingCount
			}
		}
	}

	// 优先从 total_count 字段获取
	if totalCount, ok := resultMap["total_count"]; ok {
		switch v := totalCount.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}

	// 尝试从 count 字段获取
	if count, ok := resultMap["count"]; ok {
		switch v := count.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}

	// 尝试从 num 字段获取
	if num, ok := resultMap["num"]; ok {
		switch v := num.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}

	// 尝试从 detections 数组长度获取
	if detections, ok := resultMap["detections"]; ok {
		if detectionsArray, ok := detections.([]interface{}); ok {
			return len(detectionsArray)
		}
	}

	// 尝试从 objects 数组长度获取
	if objects, ok := resultMap["objects"]; ok {
		if objectsArray, ok := objects.([]interface{}); ok {
			return len(objectsArray)
		}
	}

	return 0
}

func parseBoolFromConfig(val interface{}) (bool, bool) {
	switch v := val.(type) {
	case bool:
		return v, true
	case string:
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed, true
		}
	case float64:
		return v != 0, true
	}
	return false, false
}

func (s *Scheduler) shouldSaveAlertImage(taskID string, algoConfig map[string]interface{}) bool {
	// 优先从任务配置中读取
	if fxService := s.getFrameExtractorService(); fxService != nil {
		if task := fxService.GetTaskByID(taskID); task != nil {
			// 如果任务配置了SaveAlertImage，使用任务配置
			if task.SaveAlertImage != nil {
				return *task.SaveAlertImage
			}
		}
	}

	// 其次从算法配置中读取（兼容旧逻辑）
	if algoConfig != nil {
		if val, ok := parseBoolFromConfig(algoConfig["save_alert_image"]); ok {
			return val
		}
		if params, ok := algoConfig["algorithm_params"].(map[string]interface{}); ok {
			if val, ok := parseBoolFromConfig(params["save_alert_image"]); ok {
				return val
			}
		}
	}

	// 默认返回true（保存告警图片）
	return true
}

// deleteImage 删除MinIO中的图片
func (s *Scheduler) deleteImage(imagePath string) error {
	return s.deleteImageWithReason(imagePath, "unknown")
}

// deleteImageWithReason 删除MinIO中的图片（带删除原因）
func (s *Scheduler) deleteImageWithReason(imagePath, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.minio.RemoveObject(ctx, s.bucket, imagePath, minio.RemoveObjectOptions{})
	if err != nil {
		s.log.Error("failed to delete image from MinIO",
			slog.String("path", imagePath),
			slog.String("reason", reason),
			slog.String("err", err.Error()))
		return fmt.Errorf("remove object failed: %w", err)
	}

	s.log.Info("image deleted from MinIO",
		slog.String("path", imagePath),
		slog.String("reason", reason))

	return nil
}

// moveImageToAlertPath 将图片移动到告警路径（同步版本，已废弃）
func (s *Scheduler) moveImageToAlertPath(imagePath, taskType, taskID string) (string, error) {
	// 解析原文件名
	parts := strings.Split(imagePath, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid image path: %s", imagePath)
	}
	filename := parts[len(parts)-1]

	// 构建告警路径：alerts/{task_type}/{task_id}/filename
	alertPath := fmt.Sprintf("%s%s/%s/%s", s.alertBasePath, taskType, taskID, filename)

	return alertPath, s.moveImageToAlertPathAsync(imagePath, alertPath)
}

// moveImageToAlertPathAsync 异步移动图片到告警路径（带重试）
func (s *Scheduler) moveImageToAlertPathAsync(srcPath, dstPath string) error {
	// 重试配置
	maxRetries := 3
	retryDelay := 500 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			s.log.Debug("retrying image move",
				slog.Int("attempt", attempt+1),
				slog.String("src", srcPath),
				slog.String("dst", dstPath))
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}

		// 执行移动操作
		if err := s.moveImageToAlertPathInternal(srcPath, dstPath); err != nil {
			lastErr = err
			continue
		}

		// 成功
		return nil
	}

	// 所有重试都失败
	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// moveImageToAlertPathInternal 内部移动操作（带监控）
func (s *Scheduler) moveImageToAlertPathInternal(srcPath, dstPath string) error {
	// 记录开始时间
	startTime := time.Now()

	// 优化超时时间：从15秒降到5秒，快速失败
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 复制对象到新路径
	copyStart := time.Now()
	src := minio.CopySrcOptions{
		Bucket: s.bucket,
		Object: srcPath,
	}
	dst := minio.CopyDestOptions{
		Bucket: s.bucket,
		Object: dstPath,
	}

	_, err := s.minio.CopyObject(ctx, dst, src)
	copyDuration := time.Since(copyStart)

	if err != nil {
		// 记录失败
		if s.monitor != nil {
			s.monitor.RecordMinIOMove(false, time.Since(startTime).Milliseconds())
		}
		return fmt.Errorf("copy object failed: %w", err)
	}

	// 删除原文件（等待复制完成后再删除）
	removeStart := time.Now()
	if err := s.minio.RemoveObject(ctx, s.bucket, srcPath, minio.RemoveObjectOptions{}); err != nil {
		// 复制成功但删除失败，不返回错误（原文件留着也无妨）
		s.log.Warn("failed to remove original image after copy (not critical)",
			slog.String("path", srcPath),
			slog.String("err", err.Error()))
	}
	removeDuration := time.Since(removeStart)
	totalDuration := time.Since(startTime)

	// 记录成功和响应时间
	if s.monitor != nil {
		s.monitor.RecordMinIOMove(true, totalDuration.Milliseconds())
	}

	// 记录性能日志（Debug级别，避免日志过多）
	s.log.Debug("image move completed",
		slog.String("src", srcPath),
		slog.String("dst", dstPath),
		slog.Duration("copy_duration_ms", copyDuration),
		slog.Duration("remove_duration_ms", removeDuration),
		slog.Duration("total_duration_ms", totalDuration))

	return nil
}

// getFrameExtractorService 获取抽帧服务实例
func (s *Scheduler) getFrameExtractorService() *frameextractor.Service {
	return frameextractor.GetGlobal()
}

func (s *Scheduler) selectAlgorithmForImage(image ImageInfo) (*conf.AlgorithmService, error) {
	if image.TaskType != tripwireTaskType {
		return s.registry.GetAlgorithmWithLoadBalance(image.TaskType), nil
	}

	fxService := s.getFrameExtractorService()
	if fxService == nil {
		return nil, fmt.Errorf("frame extractor service unavailable")
	}

	task := fxService.GetTaskByID(image.TaskID)
	if task == nil {
		return nil, fmt.Errorf("frame extractor task not found")
	}

	preferredEndpoint := strings.TrimSpace(task.PreferredAlgorithmEndpoint)
	if preferredEndpoint == "" {
		return nil, fmt.Errorf("preferred_algorithm_endpoint not configured for task")
	}

	algorithm := s.registry.GetAlgorithmByEndpoint(image.TaskType, preferredEndpoint)
	if algorithm == nil {
		return nil, fmt.Errorf("preferred algorithm endpoint %s not registered", preferredEndpoint)
	}

	return algorithm, nil
}

// generatePresignedURL 生成图片的预签名URL
// 注意：MinIO SDK生成签名时使用UTC时间，但MinIO服务器验证时使用CST时间
// 时差8小时，24小时有效期已经足够覆盖时区差
func (s *Scheduler) generatePresignedURL(imagePath string) (string, error) {
	if imagePath == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 24小时有效期已经足够覆盖8小时时差，保持原值
	presignedURL, err := s.minio.PresignedGetObject(ctx, s.bucket, imagePath, 24*time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// getMoveLock 获取或创建指定task_id的移动锁
// 确保同一任务的图片按顺序移动，避免并发导致的内容错位
func (s *Scheduler) getMoveLock(taskID string) *sync.Mutex {
	s.moveLockMu.Lock()
	defer s.moveLockMu.Unlock()

	if _, ok := s.moveLocks[taskID]; !ok {
		s.moveLocks[taskID] = &sync.Mutex{}
	}

	// 更新最后使用时间
	s.moveLockLastUse[taskID] = time.Now()

	return s.moveLocks[taskID]
}

// cleanupMoveLocks 清理超过3分钟未使用的移动锁
func (s *Scheduler) cleanupMoveLocks() {
	s.moveLockMu.Lock()
	defer s.moveLockMu.Unlock()

	now := time.Now()
	cleanupThreshold := 3 * time.Minute // 缩短到3分钟
	cleanedCount := 0

	for taskID, lastUse := range s.moveLockLastUse {
		if now.Sub(lastUse) > cleanupThreshold {
			delete(s.moveLocks, taskID)
			delete(s.moveLockLastUse, taskID)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		s.log.Debug("cleaned up unused move locks",
			slog.Int("cleaned_count", cleanedCount),
			slog.Int("remaining_count", len(s.moveLocks)))
	}
}

// startMoveLockCleanup 启动移动锁定期清理
func (s *Scheduler) startMoveLockCleanup() {
	ticker := time.NewTicker(2 * time.Minute) // 缩短到每2分钟清理一次
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			s.cleanupMoveLocks()
		}
	}()
}
