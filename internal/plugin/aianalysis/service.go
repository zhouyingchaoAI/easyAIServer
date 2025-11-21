package aianalysis

import (
	"context"
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
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
	cfg              *conf.AIAnalysisConfig
	fxCfg            *conf.FrameExtractorConfig // Frame Extractor配置
	registry         *AlgorithmRegistry
	scanner          *Scanner       // 保留用于兼容（可选，用于初始扫描）
	eventListener    *EventListener // 事件监听器（替代扫描器）
	scheduler        *Scheduler
	mq               MessageQueue
	queue            *InferenceQueue        // 智能队列
	monitor          *PerformanceMonitor    // 性能监控
	alertMgr         *AlertManager          // 告警管理
	alertBatchWriter *data.AlertBatchWriter // 批量写入告警
	log              *slog.Logger
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
		slog.Float64("scan_interval", s.cfg.ScanIntervalSec),
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

	// 初始化注册中心（优先初始化，确保注册功能可用）
	s.registry = NewRegistry(s.cfg.HeartbeatTimeoutSec, s.log)
	s.registry.StartHeartbeatChecker()

	// 设置注册回调：算法服务上线时自动启动已配置的任务
	s.registry.SetOnRegisterCallback(s.onAlgorithmServiceRegistered)

	// 设置注销回调：算法服务下线时记录日志
	s.registry.SetOnUnregisterCallback(s.onAlgorithmServiceUnregistered)

	// 初始化消息队列（如果失败，记录警告但不阻止启动）
	if err := s.initMessageQueue(); err != nil {
		s.log.Warn("failed to init message queue, continuing without MQ",
			slog.String("error", err.Error()),
			slog.String("note", "algorithm service registration will still work, but alerts will not be pushed to MQ"))
		// 不返回错误，允许服务继续启动（注册功能不依赖消息队列）
	}

	// 初始化智能队列
	maxQueueSize := s.cfg.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 100 // 默认值
	}
	alertThreshold := maxQueueSize / 2 // 告警阈值设为队列大小的一半

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
		5000, // 推理超过5秒告警
		s.log,
	)

	// 初始化告警管理器
	maxAlertsInDB := s.cfg.MaxAlertsInDB
	if maxAlertsInDB <= 0 {
		maxAlertsInDB = 1000 // 默认1000条
	}
	s.alertMgr = NewAlertManager(1000, maxAlertsInDB, s.log)

	// 设置告警回调
	s.queue.SetAlertCallback(func(alert AlertInfo) {
		s.alertMgr.SendAlert(SystemAlert{
			Type:    SystemAlertType(alert.Type),
			Level:   AlertLevel(alert.Level),
			Message: alert.Message,
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

	// 初始化批量写入器
	batchSize := s.cfg.AlertBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	batchInterval := s.cfg.AlertBatchIntervalSec
	if batchInterval <= 0 {
		batchInterval = 2
	}
	// maxAlertsInDB 已在上面声明，直接使用
	s.alertBatchWriter = data.NewAlertBatchWriter(
		batchSize,
		batchInterval,
		s.cfg.AlertBatchEnabled,
		maxAlertsInDB,
		s.log.With(slog.String("component", "alert_batch_writer")),
	)
	s.alertBatchWriter.Start()

	// 初始化事件监听器（替代扫描器，使用MinIO事件通知）
	s.eventListener = NewEventListener(minioClient, s.fxCfg.MinIO.Bucket, s.fxCfg.MinIO.BasePath, alertBasePath, s.log)

	// 保留扫描器用于兼容（可选，用于初始扫描或降级）
	s.scanner = NewScanner(minioClient, s.fxCfg.MinIO.Bucket, s.fxCfg.MinIO.BasePath, alertBasePath, s.log)

	// 初始化调度器
	moveConcurrent := s.cfg.AlertImageMoveConcurrent
	if moveConcurrent <= 0 {
		moveConcurrent = 50 // 默认50个并发
	}
	s.scheduler = NewScheduler(s.registry, minioClient, s.fxCfg.MinIO.Bucket, alertBasePath, s.mq, s.cfg.MaxConcurrentInfer, s.cfg.SaveOnlyWithDetection, s.alertBatchWriter, s.monitor, s.scanner, s.log, moveConcurrent)

	// 设置处理完成回调，用于增加processedCount
	s.scheduler.SetOnProcessedCallback(func() {
		s.queue.RecordProcessed()
	})

	// 启动智能推理循环
	s.startSmartInferenceLoop()

	// 设置全局实例
	globalService = s

	// 注册图片检查器到Frame Extractor，保护正在推理的图片、即将推理的图片和队列中等待的图片
	// 这样可以确保所有待处理的图片都不会被清理，避免时间窗口漏洞
	fxService := frameextractor.GetGlobal()
	if fxService != nil {
		fxService.SetQueueChecker(func(imagePath string) bool {
			// 保护队列中等待的图片
			if s.queue.Contains(imagePath) {
				return true
			}
			// 保护即将推理的图片（在Pop之后、ScheduleInference执行之前）
			if s.scheduler.IsImagePendingInference(imagePath) {
				return true
			}
			// 保护正在推理的图片
			if s.scheduler.IsImageInferring(imagePath) {
				return true
			}
			return false
		})
		s.log.Info("image checker registered to frame extractor",
			slog.String("note", "protecting images in queue, pending inference, and currently being inferred"))
	}

	s.log.Info("AI analysis plugin started successfully",
		slog.Int("queue_max_size", maxQueueSize),
		slog.Int("alert_threshold", alertThreshold),
		slog.String("queue_strategy", "drop_oldest"),
		slog.Int64("slow_threshold_ms", 5000))

	return nil
}

// startSmartInferenceLoop 启动智能推理循环
func (s *Service) startSmartInferenceLoop() {
	// 启动事件监听器（替代定时扫描）
	s.eventListener.Start(
		// 新图片回调
		func(img ImageInfo) {
			// 添加到智能队列（Add方法内部会去重）
			queueAddStart := time.Now()
			added := s.queue.Add([]ImageInfo{img})
			queueAddDuration := time.Since(queueAddStart)

			if added > 0 {
				s.log.Info("image added to queue via event",
					slog.String("path", img.Path),
					slog.String("task_type", img.TaskType),
					slog.String("task_id", img.TaskID),
					slog.Int("queue_size", s.queue.Size()),
					slog.Duration("queue_add_duration_ms", queueAddDuration))
			} else {
				s.log.Debug("image skipped (duplicate or queue full)",
					slog.String("path", img.Path))
			}
		},
		// 图片删除回调
		func(imagePath string) {
			// 从队列中移除已删除的图片
			removed := s.queue.Remove(imagePath)
			if removed {
				s.log.Info("image removed from queue (deleted from MinIO)",
					slog.String("path", imagePath),
					slog.Int("remaining_queue_size", s.queue.Size()))
			}
		},
	)

	// 可选：执行一次初始扫描，处理启动前已存在的图片
	// 注意：这可能导致重复处理，但可以确保不遗漏启动前的图片
	// 如果不需要，可以注释掉这部分代码
	go func() {
		time.Sleep(2 * time.Second) // 等待事件监听器启动
		s.log.Info("performing initial scan for existing images")
		s.scanner.Start(s.cfg.ScanIntervalSec, func(images []ImageInfo) {
			// 只执行一次扫描
			queueAddStart := time.Now()
			added := s.queue.Add(images)
			queueAddDuration := time.Since(queueAddStart)

			if added > 0 {
				s.log.Info("initial scan: images added to queue",
					slog.Int("added", added),
					slog.Int("queue_size", s.queue.Size()),
					slog.Duration("queue_add_duration_ms", queueAddDuration))
			}

			// 标记已处理
			for _, img := range images {
				s.scanner.MarkProcessed(img.Path)
			}

			// 停止扫描器（只执行一次）
			// 注意：如果scanner已经被Stop()过，这里会安全处理（不会panic）
			if s.scanner != nil {
			s.scanner.Stop()
			}
		})
	}()

	// 启动多个推理处理worker以提升并发处理能力
	workerCount := s.cfg.MaxConcurrentInfer
	if workerCount <= 0 {
		workerCount = 5 // 默认5个worker
	}
	// 移除硬编码限制，允许使用配置的并发数
	// 注意：过高的并发数可能导致资源耗尽，建议根据实际情况调整

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
	emptyQueueCount := 0
	for {
		// 在队列取数前，先确认算法并发是否有空位
		if s.scheduler != nil {
			maxConcurrent := s.scheduler.GetMaxConcurrent()
			if maxConcurrent > 0 {
				active := s.scheduler.GetActiveInferenceCount()
				if int(active) >= maxConcurrent {
					// 算法已满载，等待后再尝试
					time.Sleep(10 * time.Millisecond)
					continue
				}
			}
		}

		// 关键修复：从队列Pop后立即标记为"即将推理"，避免时间窗口漏洞
		// 这样可以确保图片在Pop之后、ScheduleInference实际执行之前就受到保护
		popStart := time.Now()
		img, ok := s.queue.Pop()
		popDuration := time.Since(popStart)

		if ok {
			// 立即标记为"即将推理"，避免时间窗口漏洞
			// 标记操作很快（只是一个map操作），时间窗口已经非常小
			marked := s.scheduler.MarkPendingInference(img.Path)
			if !marked {
				// 如果已经标记过，说明可能有重复，记录警告
				s.log.Warn("image already marked as pending inference",
					slog.String("path", img.Path))
			}
		}
		if !ok {
			emptyQueueCount++
			// 每100次空队列才记录一次日志，避免日志过多
			if emptyQueueCount%100 == 0 {
				s.log.Debug("queue empty, waiting",
					slog.Int("empty_count", emptyQueueCount),
					slog.Duration("pop_duration_ms", popDuration))
			}
			// 优化：减少sleep时间从100ms到10ms，提高响应速度
			// 这样可以更快地处理新加入队列的图片，减少等待时间
			time.Sleep(10 * time.Millisecond)
			continue
		}
		emptyQueueCount = 0

		// 在调度推理前，先检查图片是否还存在（避免处理已被清理的图片）
		// 注意：现在队列中的图片已被保护，但为了安全起见仍然检查
		// 如果图片不存在，可能是被其他原因删除的
		exists, statErr := s.scheduler.CheckImageExists(img.Path)

		if statErr != nil || !exists {
			// 图片不存在，取消pending标记并跳过处理（可能已被清理）
			s.scheduler.UnmarkPendingInference(img.Path)

			s.log.Debug("image not found before inference, skipping",
				slog.String("task_id", img.TaskID),
				slog.String("image", img.Filename),
				slog.String("path", img.Path),
				slog.String("err", statErr.Error()),
				slog.String("note", "image may have been deleted while waiting in queue"))

			// 标记为已处理，避免重复扫描
			// 注意：图片不存在不算处理，不增加processedCount
			if s.scanner != nil {
				s.scanner.MarkProcessed(img.Path)
			}
			continue
		}

		// 调度推理（同步调用，调度器内部已有并发控制）
		// 修复：移除额外的goroutine，避免goroutine泄漏
		// ScheduleInference内部已经有并发控制（通过activeInferences和maxConcurrent），
		// 不需要为每个图片都启动一个goroutine，这会导致goroutine数量无限增长
			scheduleStart := time.Now()
		s.scheduler.ScheduleInference(img)
			totalDuration := time.Since(scheduleStart)

			// 记录调度耗时（仅在Debug级别，避免日志过多）
		s.log.Debug("inference scheduled",
			slog.String("task_id", img.TaskID),
			slog.String("image", img.Filename),
				slog.Duration("schedule_duration_ms", totalDuration))

		// 注意：worker不再等待推理完成，立即处理下一张图片
		// ScheduleInference内部会异步执行推理，不会阻塞worker
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

		// 获取实际并发数
		var activeInferences int32
		var maxConcurrent int
		if s.scheduler != nil {
			activeInferences = s.scheduler.GetActiveInferenceCount()
			maxConcurrent = s.scheduler.GetMaxConcurrent()
		}

		// 记录统计日志（包含实际并发数）
		s.log.Info("performance statistics",
			slog.Any("queue", queueStats),
			slog.Any("performance", perfStats),
			slog.Int("active_inferences", int(activeInferences)),
			slog.Int("max_concurrent", maxConcurrent),
			slog.Float64("concurrency_utilization", float64(activeInferences)/float64(maxConcurrent)))

		// 检查丢弃率
		dropRate := s.queue.GetDropRate()
		if dropRate > 0.3 { // 丢弃率超过30%
			s.alertMgr.SendAlert(SystemAlert{
				Type:    AlertTypeHighDrop,
				Level:   LevelError,
				Message: "图片丢弃率过高，推理能力严重不足",
				Data: map[string]interface{}{
					"drop_rate":       dropRate,
					"dropped_total":   queueStats["dropped_total"],
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

	if s.eventListener != nil {
		s.eventListener.Stop()
	}
	if s.scanner != nil {
		s.scanner.Stop()
	}

	if s.registry != nil {
		s.registry.StopHeartbeatChecker()
	}

	// 停止批量写入器（会刷新剩余数据）
	if s.alertBatchWriter != nil {
		s.log.Info("stopping alert batch writer and flushing remaining alerts")
		s.alertBatchWriter.Stop()
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

// GetQueue 获取推理队列
func (s *Service) GetQueue() *InferenceQueue {
	return s.queue
}

// InferenceStats 推理统计信息
type InferenceStats struct {
	QueueSize          int     `json:"queue_size"`           // 当前队列大小
	QueueMaxSize       int     `json:"queue_max_size"`       // 队列最大容量
	QueueUtilization   float64 `json:"queue_utilization"`    // 队列使用率
	ActiveInferences   int32   `json:"active_inferences"`    // 当前正在推理的数量
	MaxConcurrentInfer int     `json:"max_concurrent_infer"` // 最大并发推理数
	DroppedTotal       int64   `json:"dropped_total"`        // 累计丢弃数
	ProcessedTotal     int64   `json:"processed_total"`      // 累计处理数
	DropRate           float64 `json:"drop_rate"`            // 丢弃率
	Strategy           string  `json:"strategy"`             // 队列策略
	AvgInferenceMs     float64 `json:"avg_inference_ms"`     // 平均推理时间(ms)
	MaxInferenceMs     int64   `json:"max_inference_ms"`     // 最大推理时间(ms)
	TotalInferences    int64   `json:"total_inferences"`     // 总推理次数
		SuccessInferences  int64   `json:"success_inferences"`   // 成功推理次数
		FailedInferences   int64   `json:"failed_inferences"`    // 失败次数
		SuccessRatePerSec  float64 `json:"success_rate_per_sec"` // 每秒推理成功数（张/秒）
		RequestRatePerSec  float64 `json:"request_rate_per_sec"` // 每秒请求发送数（次/秒）
		ResponseRatePerSec float64 `json:"response_rate_per_sec"` // 每秒响应数（次/秒）
	
	// MinIO操作监控（图片移动）
	MinIOMoveTotal       int64   `json:"minio_move_total"`        // 总移动次数
	MinIOMoveSuccess     int64   `json:"minio_move_success"`      // 成功次数
	MinIOMoveFailed      int64   `json:"minio_move_failed"`       // 失败次数
	MinIOMoveAvgTimeMs   float64 `json:"minio_move_avg_time_ms"`  // 平均耗时（毫秒）
	MinIOMoveMaxTimeMs   int64   `json:"minio_move_max_time_ms"`  // 最大耗时（毫秒）
	MinIOMoveSuccessRate float64 `json:"minio_move_success_rate"` // 成功率（0.0-1.0）
	
	UpdatedAt string `json:"updated_at"` // 更新时间
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

	successCount := int64(0)
	if sc, ok := perfStats["success_count"].(int64); ok {
		successCount = sc
	}

	successRatePerSec := 0.0
	if sr, ok := perfStats["success_rate_per_sec"].(float64); ok {
		successRatePerSec = sr
	}

	requestRatePerSec := 0.0
	if rr, ok := perfStats["request_rate_per_sec"].(float64); ok {
		requestRatePerSec = rr
	}

	responseRatePerSec := 0.0
	if rr, ok := perfStats["response_rate_per_sec"].(float64); ok {
		responseRatePerSec = rr
	}

	// 获取MinIO操作监控指标
	minIOMoveTotal := int64(0)
	minIOMoveSuccess := int64(0)
	minIOMoveFailed := int64(0)
	minIOMoveAvgTimeMs := 0.0
	minIOMoveMaxTimeMs := int64(0)
	minIOMoveSuccessRate := 0.0

	if mt, ok := perfStats["minio_move_total"].(int64); ok {
		minIOMoveTotal = mt
	}
	if ms, ok := perfStats["minio_move_success"].(int64); ok {
		minIOMoveSuccess = ms
	}
	if mf, ok := perfStats["minio_move_failed"].(int64); ok {
		minIOMoveFailed = mf
	}
	if mat, ok := perfStats["minio_move_avg_time_ms"].(float64); ok {
		minIOMoveAvgTimeMs = mat
	}
	if mmt, ok := perfStats["minio_move_max_time_ms"].(int64); ok {
		minIOMoveMaxTimeMs = mmt
	}
	if msr, ok := perfStats["minio_move_success_rate"].(float64); ok {
		minIOMoveSuccessRate = msr
	}

	var activeCount int32
	if s.scheduler != nil {
		activeCount = s.scheduler.GetActiveInferenceCount()
	}
	maxConcurrent := s.cfg.MaxConcurrentInfer
	if maxConcurrent <= 0 && s.scheduler != nil {
		maxConcurrent = s.scheduler.GetMaxConcurrent()
	}

	return InferenceStats{
		QueueSize:          queueStats["queue_size"].(int),
		QueueMaxSize:       queueStats["max_size"].(int),
		QueueUtilization:   queueStats["utilization"].(float64),
		ActiveInferences:   activeCount,
		MaxConcurrentInfer: maxConcurrent,
		DroppedTotal:       queueStats["dropped_total"].(int64),
		ProcessedTotal:     queueStats["processed_total"].(int64),
		DropRate:           dropRate,
		Strategy:           queueStats["strategy"].(string),
		AvgInferenceMs:     perfStats["avg_inference_ms"].(float64),
		MaxInferenceMs:     perfStats["max_inference_ms"].(int64),
		TotalInferences:    perfStats["total_count"].(int64),
		SuccessInferences:  successCount,
		FailedInferences:   perfStats["failed_count"].(int64),
		SuccessRatePerSec:  successRatePerSec,
		RequestRatePerSec:  requestRatePerSec,
		ResponseRatePerSec: responseRatePerSec,
		
		// MinIO操作监控（图片移动）
		MinIOMoveTotal:       minIOMoveTotal,
		MinIOMoveSuccess:     minIOMoveSuccess,
		MinIOMoveFailed:      minIOMoveFailed,
		MinIOMoveAvgTimeMs:   minIOMoveAvgTimeMs,
		MinIOMoveMaxTimeMs:   minIOMoveMaxTimeMs,
		MinIOMoveSuccessRate: minIOMoveSuccessRate,
		
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// ResetInferenceStats 重置推理统计数据
func (s *Service) ResetInferenceStats() error {
	if s.queue == nil || s.monitor == nil {
		return fmt.Errorf("inference service not initialized")
	}

	s.queue.ResetStats()
	s.monitor.Reset()

	// 同时重置抽帧统计数据
	fxService := frameextractor.GetGlobal()
	if fxService != nil {
		fxService.ResetFrameStats()
		s.log.Info("frame extraction statistics also reset")
	}

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
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
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

// onAlgorithmServiceUnregistered 算法服务注销时的回调
func (s *Service) onAlgorithmServiceUnregistered(serviceID string, reason string) {
	s.log.Warn("algorithm service offline",
		slog.String("service_id", serviceID),
		slog.String("reason", reason))

	// 可以在这里添加额外的处理逻辑，例如：
	// - 通知管理员服务下线
	// - 暂停相关的抽帧任务（可选）
	// - 记录服务中断事件

	// 注意：不要自动停止抽帧任务，因为：
	// 1. 服务可能只是临时故障，很快会恢复
	// 2. 可能有其他算法服务可以处理相同的任务类型
	// 3. 图片会继续抽帧并存储，等待服务恢复后处理
}

// getFrameExtractorService 获取抽帧服务实例
func (s *Service) getFrameExtractorService() *frameextractor.Service {
	return frameextractor.GetGlobal()
}

// GeneratePresignedURL 为图片路径生成预签名URL
func (s *Service) GeneratePresignedURL(imagePath string) (string, error) {
	if s.scheduler == nil {
		return "", fmt.Errorf("scheduler not initialized")
	}

	return s.scheduler.generatePresignedURL(imagePath)
}
