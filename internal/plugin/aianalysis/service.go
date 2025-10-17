package aianalysis

import (
	"easydarwin/internal/conf"
	"easydarwin/internal/plugin/frameextractor"
	"fmt"
	"log/slog"
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
	s.queue = NewInferenceQueue(
		100,                    // 最大队列容量
		StrategyDropOldest,     // 丢弃最旧的策略
		50,                     // 积压50张告警
		minioClient,            // MinIO客户端
		s.fxCfg.MinIO.Bucket,   // MinIO bucket
		true,                   // 丢弃图片时删除MinIO文件
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

	// 初始化调度器
	s.scheduler = NewScheduler(s.registry, minioClient, s.fxCfg.MinIO.Bucket, s.mq, s.cfg.MaxConcurrentInfer, s.cfg.SaveOnlyWithDetection, s.log)

	// 初始化扫描器
	s.scanner = NewScanner(minioClient, s.fxCfg.MinIO.Bucket, s.fxCfg.MinIO.BasePath, s.log)

	// 启动智能推理循环
	s.startSmartInferenceLoop()

	// 设置全局实例
	globalService = s

	s.log.Info("AI analysis plugin started successfully",
		slog.Int("queue_max_size", 100),
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
	
	// 启动推理处理循环
	go s.inferenceProcessLoop()
	
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

// initMinIO 初始化MinIO客户端
func (s *Service) initMinIO() (*minio.Client, error) {
	cfg := s.fxCfg.MinIO
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("minio endpoint and bucket required")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	s.log.Info("minio client initialized",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("bucket", cfg.Bucket))

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

