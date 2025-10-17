package aianalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
	"easydarwin/internal/data/model"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

// Scheduler 推理调度器
type Scheduler struct {
	registry              *AlgorithmRegistry
	minio                 *minio.Client
	bucket                string
	mq                    MessageQueue
	log                   *slog.Logger
	semaphore             chan struct{} // 限制并发数
	saveOnlyWithDetection bool          // 只保存有检测结果的告警
}

// NewScheduler 创建调度器
func NewScheduler(registry *AlgorithmRegistry, minioClient *minio.Client, bucket string, mq MessageQueue, maxConcurrent int, saveOnlyWithDetection bool, logger *slog.Logger) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &Scheduler{
		registry:              registry,
		minio:                 minioClient,
		bucket:                bucket,
		mq:                    mq,
		log:                   logger,
		semaphore:             make(chan struct{}, maxConcurrent),
		saveOnlyWithDetection: saveOnlyWithDetection,
	}
}

// ScheduleInference 调度推理
func (s *Scheduler) ScheduleInference(image ImageInfo) {
	// 获取该任务类型的所有算法
	algorithms := s.registry.GetAlgorithms(image.TaskType)
	if len(algorithms) == 0 {
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
		slog.Int("algorithms", len(algorithms)))

	// 并发调用所有匹配的算法
	var wg sync.WaitGroup
	for _, algo := range algorithms {
		wg.Add(1)
		go func(algorithm conf.AlgorithmService) {
			defer wg.Done()
			
			// 限流
			s.semaphore <- struct{}{}
			defer func() { <-s.semaphore }()
			
			s.inferAndSave(image, algorithm)
		}(algo)
	}
	wg.Wait()
}

// inferAndSave 调用算法推理并保存结果
func (s *Scheduler) inferAndSave(image ImageInfo, algorithm conf.AlgorithmService) {
	// 生成预签名URL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	presignedURL, err := s.minio.PresignedGetObject(ctx, s.bucket, image.Path, 1*time.Hour, nil)
	if err != nil {
		s.log.Error("failed to generate presigned URL",
			slog.String("path", image.Path),
			slog.String("err", err.Error()))
		// 预签名失败，删除图片避免积压
		s.deleteImageWithReason(image.Path, "presign_failed")
		return
	}

	// 构建推理请求
	req := conf.InferenceRequest{
		ImageURL:  presignedURL.String(),
		TaskID:    image.TaskID,
		TaskType:  image.TaskType,
		ImagePath: image.Path,
	}

	// 记录推理开始时间
	inferStartTime := time.Now()
	
	// 调用算法服务
	resp, err := s.callAlgorithm(algorithm, req)
	if err != nil {
		s.log.Error("algorithm inference failed",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("image", image.Path),
			slog.String("err", err.Error()))
		// 推理失败，不删除图片（可能是算法服务临时故障）
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
	
	// 如果启用了只保存有检测结果的功能，且没有检测结果，则删除图片并跳过保存
	if s.saveOnlyWithDetection && detectionCount == 0 {
		s.log.Info("no detection result, deleting image",
			slog.String("image", image.Path),
			slog.String("task_id", image.TaskID),
			slog.String("task_type", image.TaskType),
			slog.String("algorithm", algorithm.ServiceID))
		
		// 删除MinIO中的图片（检测对象为0）
		if err := s.deleteImageWithReason(image.Path, "no_detection"); err != nil {
			s.log.Error("failed to delete image with no detection",
				slog.String("path", image.Path),
				slog.String("err", err.Error()))
		} else {
			s.log.Info("image deleted successfully (no detection)",
				slog.String("path", image.Path),
				slog.String("task_id", image.TaskID))
		}
		
		return // 不保存告警，不推送消息
	}
	
	// 有检测结果，保存告警到数据库
	resultJSON, _ := json.Marshal(resp.Result)
	alert := &model.Alert{
		TaskID:          image.TaskID,
		TaskType:        image.TaskType,
		ImagePath:       image.Path,
		ImageURL:        presignedURL.String(),
		AlgorithmID:     algorithm.ServiceID,
		AlgorithmName:   algorithm.Name,
		Result:          string(resultJSON),
		Confidence:      resp.Confidence,
		DetectionCount:  detectionCount,
		InferenceTimeMs: int(actualInferenceTime),
		CreatedAt:       time.Now(),
	}

	if err := data.CreateAlert(alert); err != nil {
		s.log.Error("failed to save alert",
			slog.String("task_id", image.TaskID),
			slog.String("err", err.Error()))
		return
	}

	s.log.Info("inference completed and saved",
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("task_id", image.TaskID),
		slog.String("task_type", image.TaskType),
		slog.Int("detection_count", detectionCount),
		slog.Uint64("alert_id", uint64(alert.ID)),
		slog.Float64("confidence", resp.Confidence),
		slog.Int64("inference_time_ms", actualInferenceTime))

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
}

// callAlgorithm HTTP调用算法服务
func (s *Scheduler) callAlgorithm(algorithm conf.AlgorithmService, req conf.InferenceRequest) (*conf.InferenceResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", algorithm.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	var resp conf.InferenceResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
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
	
	// 优先从 total_count 字段获取（最高优先级）
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

