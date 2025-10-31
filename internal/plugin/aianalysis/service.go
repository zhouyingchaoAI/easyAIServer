package aianalysis

import (
	"context"
	"easydarwin/internal/conf"
	"easydarwin/internal/plugin/frameextractor"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Service AI分析主服务
type Service struct {
	cfg       *conf.AIAnalysisConfig
	fxCfg     *conf.FrameExtractorConfig // Frame Extractor配置
	registry  *AlgorithmRegistry
	scanner   *Scanner
	scheduler *Scheduler
	mq        MessageQueue
	queue     *InferenceQueue     // 智能队列
	monitor   *PerformanceMonitor // 性能监控
	alertMgr  *AlertManager       // 告警管理
	log       *slog.Logger
}

var globalService *Service

// NewService 创建AI分析服务
func NewService(aiCfg *conf.AIAnalysisConfig, fxCfg *conf.FrameExtractorConfig, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		cfg:   aiCfg,
		fxCfg: fxCfg,
		log:   logger.With(slog.String("module", "aianalysis")),
	}
}

// Start 启动AI分析服务
func (s *Service) Start() error {
	if !s.cfg.Enable {
		s.log.Info("AI analysis plugin disabled")
		return nil
	}

	s.log.Info("starting AI analysis plugin",
		slog.Int("scan_interval", s.cfg.ScanIntervalSec),
		slog.String("mq_type", s.cfg.MQType),
		slog.String("mq_address", s.cfg.MQAddress),
		slog.Bool("save_only_with_detection", s.cfg.SaveOnlyWithDetection))

	// 检查Frame Extractor是否使用MinIO
	if s.fxCfg.Store != "minio" {
		return fmt.Errorf("AI analysis requires frame_extractor.store = 'minio'")
	}

	// 初始化MinIO客户端
	minioClient, err := s.initMinIO()
	if err != nil {
		return fmt.Errorf("failed to init minio: %w", err)
	}

	// 运行 MinIO 诊断（自动排查 502 问题）
	debugger := NewMinIODebugger(minioClient, s.fxCfg.MinIO.Bucket, s.log)
	if err := debugger.DiagnoseWithRetry(2, 2*time.Second); err != nil {
		s.log.Error("MinIO 诊断发现问题，但继续启动",
			slog.String("error", err.Error()))
		// 不阻止启动，但记录警告
	} else {
		s.log.Info("MinIO 诊断通过，连接正常")
	}

	// 初始化消息队列
	if err := s.initMessageQueue(); err != nil {
		return fmt.Errorf("failed to init message queue: %w", err)
	}

	// 初始化注册中心
	s.registry = NewRegistry(s.cfg.HeartbeatTimeoutSec, s.log)
	s.registry.StartHeartbeatChecker()
	
	// 设置注册回调：算法服务上线时自动启动已配置的任务
	s.registry.SetOnRegisterCallback(s.onAlgorithmServiceRegistered)

	// 初始化智能队列
	maxQueueSize := s.cfg.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 100  // 默认值
	}
	alertThreshold := maxQueueSize / 2  // 告警阈值设为队列大小的一半
	
	s.queue = NewInferenceQueue(
		maxQueueSize,         // 最大队列容量（可配置）
		StrategyDropOldest,   // 丢弃最旧的策略
		alertThreshold,       // 积压告警阈值
		minioClient,          // MinIO客户端
		s.fxCfg.MinIO.Bucket, // MinIO bucket
		true,                 // 丢弃图片时删除MinIO文件
		s.log,
	)

	// 初始化性能监控器
	s.monitor = NewPerformanceMonitor(
		5000,  // 推理超过5秒告警
		s.log,
	)

	// 初始化告警管理器
	s.alertMgr = NewAlertManager(1000, s.log)

	// 设置告警回调
	s.queue.SetAlertCallback(func(alert AlertInfo) {
		s.alertMgr.SendAlert(SystemAlert{
			Type:      SystemAlertType(alert.Type),
			Level:     AlertLevel(alert.Level),
			Message:   alert.Message,
			Data: map[string]interface{}{
				"queue_size": alert.QueueSize,
				"dropped":    alert.Dropped,
			},
			Timestamp: alert.Timestamp,
		})
	})

	s.monitor.SetAlertCallback(func(alert AlertInfo) {
		s.alertMgr.SendAlert(SystemAlert{
			Type:      SystemAlertType(alert.Type),
			Level:     AlertLevel(alert.Level),
			Message:   alert.Message,
			Timestamp: alert.Timestamp,
		})
	})

	// 获取告警路径前缀
	alertBasePath := s.cfg.AlertBasePath
	if alertBasePath == "" {
		alertBasePath = "alerts/"
	}
	
	// 初始化调度器
	s.scheduler = NewScheduler(s.registry, minioClient, s.fxCfg.MinIO.Bucket, alertBasePath, s.mq, s.cfg.MaxConcurrentInfer, s.cfg.SaveOnlyWithDetection, s.log)

	// 初始化扫描器
	s.scanner = NewScanner(minioClient, s.fxCfg.MinIO.Bucket, s.fxCfg.MinIO.BasePath, alertBasePath, s.log)

	// 启动智能推理循环
	s.startSmartInferenceLoop()

	// 设置全局实例
	globalService = s

	s.log.Info("AI analysis plugin started successfully",
		slog.Int("queue_max_size", maxQueueSize),
		slog.Int("alert_threshold", alertThreshold),
		slog.String("queue_strategy", "drop_oldest"),
		slog.Int64("slow_threshold_ms", 5000))
	
	return nil
}

// startSmartInferenceLoop 启动智能推理循环
func (s *Service) startSmartInferenceLoop() {
	// 启动扫描器
	s.scanner.Start(s.cfg.ScanIntervalSec, func(images []ImageInfo) {
		// 添加到智能队列
		added := s.queue.Add(images)
		
		if added > 0 {
			s.log.Info("images added to queue",
				slog.Int("added", added),
				slog.Int("queue_size", s.queue.Size()))
		}
		
		// 标记所有图片为已扫描
		for _, img := range images {
			s.scanner.MarkProcessed(img.Path)
		}
	})
	
	// 启动多个推理处理worker以提升并发处理能力
	workerCount := s.cfg.MaxConcurrentInfer
	if workerCount <= 0 {
		workerCount = 5  // 默认5个worker
	}
	// 限制worker数量上限，避免过多goroutine
	if workerCount > 200 {
		workerCount = 200
	}
	
	s.log.Info("starting inference workers",
		slog.Int("worker_count", workerCount),
		slog.Int("max_concurrent_infer", s.cfg.MaxConcurrentInfer))
	
	// 启动多个worker并行处理队列
	for i := 0; i < workerCount; i++ {
		go s.inferenceProcessLoop()
	}
	
	// 启动定期统计和检查
	go s.periodicStatsLoop()
}

// inferenceProcessLoop 推理处理循环
func (s *Service) inferenceProcessLoop() {
	for {
		// 从队列取出图片
		img, ok := s.queue.Pop()
		if !ok {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		
		// 记录开始时间
		startTime := time.Now()
		
		// 调度推理
		s.scheduler.ScheduleInference(img)
		
		// 记录推理时间
		inferenceTime := time.Since(startTime).Milliseconds()
		s.monitor.RecordInference(inferenceTime, true)
	}
}

// periodicStatsLoop 定期统计循环
func (s *Service) periodicStatsLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// 获取统计信息
		queueStats := s.queue.GetStats()
		perfStats := s.monitor.GetStats()
		
		// 记录统计日志
		s.log.Info("performance statistics",
			slog.Any("queue", queueStats),
			slog.Any("performance", perfStats))
		
		// 检查丢弃率
		dropRate := s.queue.GetDropRate()
		if dropRate > 0.3 {  // 丢弃率超过30%
			s.alertMgr.SendAlert(SystemAlert{
				Type:    AlertTypeHighDrop,
				Level:   LevelError,
				Message: "图片丢弃率过高，推理能力严重不足",
				Data: map[string]interface{}{
					"drop_rate":      dropRate,
					"dropped_total":  queueStats["dropped_total"],
					"processed_total": queueStats["processed_total"],
				},
				Timestamp: time.Now(),
			})
		}
	}
}

// Stop 停止AI分析服务
func (s *Service) Stop() error {
	s.log.Info("stopping AI analysis plugin")

	if s.scanner != nil {
		s.scanner.Stop()
	}

	if s.registry != nil {
		s.registry.StopHeartbeatChecker()
	}

	if s.mq != nil {
		if err := s.mq.Close(); err != nil {
			s.log.Error("failed to close MQ", slog.String("err", err.Error()))
		}
	}
	
	// 输出最终统计
	if s.queue != nil {
		s.log.Info("final queue stats", slog.Any("stats", s.queue.GetStats()))
	}
	if s.monitor != nil {
		s.log.Info("final performance stats", slog.Any("stats", s.monitor.GetStats()))
	}

	globalService = nil
	s.log.Info("AI analysis plugin stopped")
	return nil
}

// GetGlobal 获取全局AI分析服务实例
func GetGlobal() *Service {
	return globalService
}

// SetGlobal 设置全局AI分析服务实例
func SetGlobal(s *Service) {
	globalService = s
}

// GetRegistry 获取注册中心
func (s *Service) GetRegistry() *AlgorithmRegistry {
	return s.registry
}

// InferenceStats 推理统计信息
type InferenceStats struct {
	QueueSize       int     `json:"queue_size"`        // 当前队列大小
	QueueMaxSize    int     `json:"queue_max_size"`    // 队列最大容量
	QueueUtilization float64 `json:"queue_utilization"` // 队列使用率
	DroppedTotal    int64   `json:"dropped_total"`     // 累计丢弃数
	ProcessedTotal  int64   `json:"processed_total"`   // 累计处理数
	DropRate        float64 `json:"drop_rate"`         // 丢弃率
	Strategy        string  `json:"strategy"`          // 队列策略
	AvgInferenceMs  float64 `json:"avg_inference_ms"`  // 平均推理时间(ms)
	MaxInferenceMs  int64   `json:"max_inference_ms"`  // 最大推理时间(ms)
	TotalInferences int64   `json:"total_inferences"`  // 总推理次数
	FailedInferences int64  `json:"failed_inferences"` // 失败次数
	UpdatedAt       string  `json:"updated_at"`        // 更新时间
}

// GetInferenceStats 获取推理统计信息
func (s *Service) GetInferenceStats() InferenceStats {
	if s.queue == nil || s.monitor == nil {
		return InferenceStats{
			UpdatedAt: time.Now().Format(time.RFC3339),
		}
	}
	
	queueStats := s.queue.GetStats()
	perfStats := s.monitor.GetStats()
	dropRate := s.queue.GetDropRate()
	
	return InferenceStats{
		QueueSize:        queueStats["queue_size"].(int),
		QueueMaxSize:     queueStats["max_size"].(int),
		QueueUtilization: queueStats["utilization"].(float64),
		DroppedTotal:     queueStats["dropped_total"].(int64),
		ProcessedTotal:   queueStats["processed_total"].(int64),
		DropRate:         dropRate,
		Strategy:         queueStats["strategy"].(string),
		AvgInferenceMs:   perfStats["avg_inference_ms"].(float64),
		MaxInferenceMs:   perfStats["max_inference_ms"].(int64),
		TotalInferences:  perfStats["total_count"].(int64),
		FailedInferences: perfStats["failed_count"].(int64),
		UpdatedAt:        time.Now().Format(time.RFC3339),
	}
}

// ResetInferenceStats 重置推理统计数据
func (s *Service) ResetInferenceStats() error {
	if s.queue == nil || s.monitor == nil {
		return fmt.Errorf("inference service not initialized")
	}
	
	s.queue.ResetStats()
	s.monitor.Reset()
	
	s.log.Info("inference statistics reset by user request")
	
	return nil
}

// initMinIO 初始化MinIO客户端
func (s *Service) initMinIO() (*minio.Client, error) {
	cfg := s.fxCfg.MinIO
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("minio endpoint and bucket required")
	}

	// 配置自定义的 HTTP Transport 以解决 502 错误
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:   false,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 创建自定义 HTTP 客户端
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Transport: transport,
		Region:    "", // MinIO 默认区域
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 使用自定义 HTTP 客户端（如果 MinIO SDK 支持）
	_ = httpClient // 保留引用以备将来使用

	// 测试连接并检查bucket是否存在，增加重试机制
	var exists bool
	maxRetries := 3
	retryDelay := 2 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		
		exists, err = client.BucketExists(ctx, cfg.Bucket)
		cancel()
		
		if err == nil {
			break
		}
		
		if i < maxRetries-1 {
			s.log.Warn("minio bucket check failed, retrying...",
				slog.Int("attempt", i+1),
				slog.Int("max_retries", maxRetries),
				slog.String("error", err.Error()))
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to check minio bucket after %d retries: %w", maxRetries, err)
	}
	
	if !exists {
		s.log.Info("creating minio bucket", slog.String("bucket", cfg.Bucket))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket %s: %w", cfg.Bucket, err)
		}
		s.log.Info("minio bucket created successfully", slog.String("bucket", cfg.Bucket))
	}

	s.log.Info("minio client initialized",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("bucket", cfg.Bucket),
		slog.Bool("bucket_exists", exists))

	return client, nil
}

// initMessageQueue 初始化消息队列
func (s *Service) initMessageQueue() error {
	if s.cfg.MQAddress == "" {
		s.log.Warn("MQ address not configured, alerts will not be pushed")
		return nil
	}

	switch s.cfg.MQType {
	case "kafka":
		s.mq = NewKafkaQueue(s.cfg.MQAddress, s.cfg.MQTopic, s.log)
	case "rabbitmq":
		return fmt.Errorf("rabbitmq not implemented yet")
	default:
		return fmt.Errorf("unknown mq_type: %s", s.cfg.MQType)
	}

	if err := s.mq.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MQ: %w", err)
	}

	s.log.Info("message queue initialized",
		slog.String("type", s.cfg.MQType),
		slog.String("address", s.cfg.MQAddress),
		slog.String("topic", s.cfg.MQTopic))

	return nil
}

// onAlgorithmServiceRegistered 算法服务注册时的回调
func (s *Service) onAlgorithmServiceRegistered(serviceID string, taskTypes []string) {
	s.log.Info("algorithm service online, checking tasks to auto-start",
		slog.String("service_id", serviceID),
		slog.Any("task_types", taskTypes))
	
	// 导入frameextractor包
	fxService := s.getFrameExtractorService()
	if fxService == nil {
		s.log.Warn("frame extractor service not available")
		return
	}
	
	// 查找所有匹配task_type且已配置的抽帧任务
	for _, taskType := range taskTypes {
		tasks := fxService.GetTasksByType(taskType)
		for _, task := range tasks {
			// 只启动已配置但未运行的任务
			if task.ConfigStatus == "configured" && !task.Enabled {
				if err := fxService.StartTaskByID(task.ID); err != nil {
					s.log.Error("failed to auto-start task",
						slog.String("task_id", task.ID),
						slog.String("err", err.Error()))
				} else {
					s.log.Info("auto-started task",
						slog.String("task_id", task.ID),
						slog.String("task_type", taskType),
						slog.String("reason", "algorithm_service_online"))
				}
			}
		}
	}
}

// getFrameExtractorService 获取抽帧服务实例
func (s *Service) getFrameExtractorService() *frameextractor.Service {
	return frameextractor.GetGlobal()
}

