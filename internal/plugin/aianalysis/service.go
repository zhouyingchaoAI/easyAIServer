package aianalysis

import (
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"

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
		slog.String("mq_address", s.cfg.MQAddress))

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

	// 初始化调度器
	s.scheduler = NewScheduler(s.registry, minioClient, s.fxCfg.MinIO.Bucket, s.mq, s.cfg.MaxConcurrentInfer, s.log)

	// 初始化扫描器
	s.scanner = NewScanner(minioClient, s.fxCfg.MinIO.Bucket, s.fxCfg.MinIO.BasePath, s.log)

	// 启动扫描器
	s.scanner.Start(s.cfg.ScanIntervalSec, func(images []ImageInfo) {
		for _, img := range images {
			s.scheduler.ScheduleInference(img)
			s.scanner.MarkProcessed(img.Path)
		}
	})

	// 设置全局实例
	globalService = s

	s.log.Info("AI analysis plugin started successfully")
	return nil
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

