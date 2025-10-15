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
	registry *AlgorithmRegistry
	minio    *minio.Client
	bucket   string
	mq       MessageQueue
	log      *slog.Logger
	semaphore chan struct{} // 限制并发数
}

// NewScheduler 创建调度器
func NewScheduler(registry *AlgorithmRegistry, minioClient *minio.Client, bucket string, mq MessageQueue, maxConcurrent int, logger *slog.Logger) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &Scheduler{
		registry:  registry,
		minio:     minioClient,
		bucket:    bucket,
		mq:        mq,
		log:       logger,
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

// ScheduleInference 调度推理
func (s *Scheduler) ScheduleInference(image ImageInfo) {
	// 获取该任务类型的所有算法
	algorithms := s.registry.GetAlgorithms(image.TaskType)
	if len(algorithms) == 0 {
		s.log.Debug("no algorithm for task type",
			slog.String("task_type", image.TaskType),
			slog.String("task_id", image.TaskID))
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
		return
	}

	// 构建推理请求
	req := conf.InferenceRequest{
		ImageURL:  presignedURL.String(),
		TaskID:    image.TaskID,
		TaskType:  image.TaskType,
		ImagePath: image.Path,
	}

	// 调用算法服务
	resp, err := s.callAlgorithm(algorithm, req)
	if err != nil {
		s.log.Error("algorithm inference failed",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("image", image.Path),
			slog.String("err", err.Error()))
		return
	}

	if !resp.Success {
		s.log.Warn("inference not successful",
			slog.String("algorithm", algorithm.ServiceID),
			slog.String("error", resp.Error))
		return
	}

	// 保存告警到数据库
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
		InferenceTimeMs: resp.InferenceTimeMs,
		CreatedAt:       time.Now(),
	}

	if err := data.CreateAlert(alert); err != nil {
		s.log.Error("failed to save alert",
			slog.String("err", err.Error()))
		return
	}

	s.log.Info("inference completed and saved",
		slog.String("algorithm", algorithm.ServiceID),
		slog.String("task_id", image.TaskID),
		slog.Uint64("alert_id", uint64(alert.ID)),
		slog.Float64("confidence", resp.Confidence))

	// 推送到消息队列
	if s.mq != nil {
		if err := s.mq.PublishAlert(*alert); err != nil {
			s.log.Error("failed to publish alert to MQ",
				slog.String("err", err.Error()))
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

