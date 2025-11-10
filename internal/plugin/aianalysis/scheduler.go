package aianalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
	"easydarwin/internal/data/model"
	"easydarwin/internal/plugin/frameextractor"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	
	// 移动锁：确保同一task_id的图片按顺序移动，避免并发错位
	moveLocks  map[string]*sync.Mutex
	moveLockMu sync.Mutex
}

// NewScheduler 创建调度器
func NewScheduler(registry *AlgorithmRegistry, minioClient *minio.Client, bucket, alertBasePath string, mq MessageQueue, maxConcurrent int, saveOnlyWithDetection bool, alertBatchWriter *data.AlertBatchWriter, logger *slog.Logger) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	
	// 优化HTTP客户端配置
	transport := &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	
	return &Scheduler{
		registry:              registry,
		minio:                 minioClient,
		bucket:                bucket,
		alertBasePath:         alertBasePath,
		mq:                    mq,
		log:                   logger,
		semaphore:             make(chan struct{}, maxConcurrent),
		saveOnlyWithDetection: saveOnlyWithDetection,
		httpClient:            httpClient,
		alertBatchWriter:      alertBatchWriter,
		moveLocks:             make(map[string]*sync.Mutex),
	}
}

// ScheduleInference 调度推理
func (s *Scheduler) ScheduleInference(image ImageInfo) {
	// 使用负载均衡选择一个算法实例
	algorithm := s.registry.GetAlgorithmWithLoadBalance(image.TaskType)
	if algorithm == nil {
		s.log.Debug("no algorithm for task type, deleting image",
			slog.String("task_type", image.TaskType),
			slog.String("task_id", image.TaskID),
			slog.String("image", image.Path))
		
		// 没有算法服务，删除图片避免积压
		if err := s.deleteImage(image.Path); err != nil {
			s.log.Warn("failed to delete image without algorithm",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		}
		
		return
	}

	s.log.Info("scheduling inference",
		slog.String("image", image.Path),
		slog.String("task_type", image.TaskType),
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("endpoint", algorithm.Endpoint))

	// 限流
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 调用选中的算法实例
	s.inferAndSave(image, *algorithm)
}

// inferAndSave 调用算法推理并保存结果
func (s *Scheduler) inferAndSave(image ImageInfo, algorithm conf.AlgorithmService) {
	// 生成预签名URL（带重试机制）
	var presignedURL *url.URL
	var err error
	maxRetries := 3
	retryDelay := 1 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		
		presignedURL, err = s.minio.PresignedGetObject(ctx, s.bucket, image.Path, 1*time.Hour, nil)
		cancel()
		
		if err == nil {
			if i > 0 {
				s.log.Info("presigned URL generated after retry",
					slog.Int("attempt", i+1),
					slog.String("path", image.Path))
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
	
	if err != nil {
		s.log.Error("failed to generate presigned URL after retries",
			slog.String("path", image.Path),
			slog.String("err", err.Error()),
			slog.String("err_type", fmt.Sprintf("%T", err)))
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
			configPath := fxService.GetAlgorithmConfigPath(image.TaskID)
			if configPath != "" {
				configURLCtx, configURLCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer configURLCancel()
				
				presignedConfigURL, err := s.minio.PresignedGetObject(configURLCtx, s.bucket, configPath, 1*time.Hour, nil)
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
		slog.String("配置文件URL", algoConfigURL))
	
	// 记录推理开始时间
	inferStartTime := time.Now()
	
	// 调用算法服务
	resp, err := s.callAlgorithm(algorithm, req)
	if err != nil {
		// ❌ 推理调用失败，记录日志（不注销服务，服务状态由心跳管理）
		s.registry.RecordInferenceFailure(algorithm.Endpoint, algorithm.ServiceID)
		
		s.log.Error("algorithm inference failed",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("endpoint", algorithm.Endpoint),
			slog.String("image", image.Path),
			slog.String("err", err.Error()),
			slog.String("note", "service will be removed by heartbeat timeout if truly offline"))
		
		// 推理失败，删除图片（避免积压，图片已尝试推理过）
		if delErr := s.deleteImageWithReason(image.Path, "inference_call_failed"); delErr != nil {
			s.log.Error("failed to delete image after inference failure",
				slog.String("path", image.Path),
				slog.String("err", delErr.Error()))
		} else {
			s.log.Info("image deleted after inference failure",
				slog.String("path", image.Path),
				slog.String("algorithm", algorithm.ServiceID))
		}
		return
	}

	// 计算实际推理耗时
	actualInferenceTime := time.Since(inferStartTime).Milliseconds()
	
	if !resp.Success {
		s.log.Warn("inference not successful",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("image", image.Path),
			slog.String("error", resp.Error))
		// 推理失败，删除图片
		s.deleteImageWithReason(image.Path, "inference_failed")
		return
	}

	// ✅ 推理成功，记录成功（增加调用计数，记录响应时间）
	// 使用算法服务返回的推理时间，如果为0则使用实际测量的时间
	reportedTimeMs := int64(resp.InferenceTimeMs)
	if reportedTimeMs <= 0 {
		reportedTimeMs = actualInferenceTime
	}
	s.registry.RecordInferenceSuccess(algorithm.Endpoint, reportedTimeMs)
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
        }

        return
    }
	
	// 有检测结果，准备告警图片路径
	var alertImagePath string
	var alertImageURL string
	
    if s.alertBasePath != "" && detectionCount > 0 {
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
		go func(srcPath, dstPath, taskID, taskType, filename string) {
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
		
	} else {
		// 未配置告警路径，使用原路径
		alertImagePath = image.Path
		// 不预先生成URL，节省时间（API返回时按需生成）
		alertImageURL = ""
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
	
	// 验证任务ID与图片路径的一致性
	if strings.Contains(alertImagePath, "/") {
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
	if err := s.alertBatchWriter.Add(alert); err != nil {
		s.log.Error("failed to add alert to batch writer",
			slog.String("task_id", image.TaskID),
			slog.String("err", err.Error()))
		return
	}
	
	s.log.Debug("alert record prepared for batch save",
		slog.String("task_id", alert.TaskID),
		slog.String("task_type", alert.TaskType),
		slog.String("image_path", alert.ImagePath),
		slog.String("original_path", image.Path))

	s.log.Info("inference completed and queued for batch save",
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("task_id", image.TaskID),
		slog.String("task_type", image.TaskType),
		slog.Int("detection_count", detectionCount),
		slog.Float64("confidence", resp.Confidence),
		slog.Int64("inference_time_ms", actualInferenceTime),
		slog.Int("batch_queue_size", s.alertBatchWriter.GetQueueSize()))

	// 推送到消息队列
	if s.mq != nil {
		if err := s.mq.PublishAlert(*alert); err != nil {
			s.log.Error("failed to publish alert to MQ",
				slog.String("task_id", image.TaskID),
				slog.String("err", err.Error()))
		} else {
			s.log.Debug("alert published to MQ",
				slog.Uint64("alert_id", uint64(alert.ID)),
				slog.String("task_id", image.TaskID))
		}
	}
	
	// 如果未配置告警路径（使用了原路径），告警已保存后删除原文件
	// 注意：删除后alert记录中的ImagePath会失效，但用户要求总是删除原路径
	if s.alertBasePath == "" && alertImagePath == image.Path {
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

// callAlgorithm HTTP调用算法服务（带重试机制）
func (s *Scheduler) callAlgorithm(algorithm conf.AlgorithmService, req conf.InferenceRequest) (*conf.InferenceResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	maxRetries := 3
	retryDelay := 2 * time.Second
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		
		httpReq, err := http.NewRequestWithContext(ctx, "POST", algorithm.Endpoint, bytes.NewReader(reqBody))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create request failed: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, err := s.httpClient.Do(httpReq)
		cancel()
		
		if err == nil {
			defer httpResp.Body.Close()

			if httpResp.StatusCode == http.StatusOK {
				var resp conf.InferenceResponse
				if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
					return nil, fmt.Errorf("decode response failed: %w", err)
				}
				
				if i > 0 {
					s.log.Info("algorithm call succeeded after retry",
						slog.Int("attempt", i+1),
						slog.String("endpoint", algorithm.Endpoint))
				}
				return &resp, nil
			}
			
			// 非200状态码
			body, _ := io.ReadAll(httpResp.Body)
			lastErr = fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
		} else {
			lastErr = err
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

// moveImageToAlertPathInternal 内部移动操作
func (s *Scheduler) moveImageToAlertPathInternal(srcPath, dstPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// 复制对象到新路径
	src := minio.CopySrcOptions{
		Bucket: s.bucket,
		Object: srcPath,
	}
	dst := minio.CopyDestOptions{
		Bucket: s.bucket,
		Object: dstPath,
	}
	
	_, err := s.minio.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("copy object failed: %w", err)
	}
	
	// 删除原文件（等待复制完成后再删除）
	if err := s.minio.RemoveObject(ctx, s.bucket, srcPath, minio.RemoveObjectOptions{}); err != nil {
		// 复制成功但删除失败，不返回错误（原文件留着也无妨）
		s.log.Warn("failed to remove original image after copy (not critical)",
			slog.String("path", srcPath),
			slog.String("err", err.Error()))
	}
	
	return nil
}

// getFrameExtractorService 获取抽帧服务实例
func (s *Scheduler) getFrameExtractorService() *frameextractor.Service {
	return frameextractor.GetGlobal()
}

// generatePresignedURL 生成图片的预签名URL
func (s *Scheduler) generatePresignedURL(imagePath string) (string, error) {
	if imagePath == "" {
		return "", nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
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
	
	return s.moveLocks[taskID]
}

