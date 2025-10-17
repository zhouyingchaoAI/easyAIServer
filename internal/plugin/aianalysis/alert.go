package aianalysis

import (
	"easydarwin/internal/data"
	"easydarwin/internal/data/model"
	"log/slog"
	"sync"
	"time"
)

// SystemAlertType 系统告警类型
type SystemAlertType string

const (
	AlertTypeBacklog     SystemAlertType = "queue_backlog"      // 队列积压
	AlertTypeSlowInfer   SystemAlertType = "slow_inference"     // 推理慢
	AlertTypeHighDrop    SystemAlertType = "high_drop_rate"     // 高丢弃率
	AlertTypeNoAlgorithm SystemAlertType = "no_algorithm"       // 无可用算法
)

// AlertLevel 告警级别
type AlertLevel string

const (
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelError    AlertLevel = "error"
	LevelCritical AlertLevel = "critical"
)

// SystemAlert 系统告警
type SystemAlert struct {
	Type      SystemAlertType
	Level     AlertLevel
	Message   string
	Data      map[string]interface{}
	Timestamp time.Time
}

// AlertManager 告警管理器
type AlertManager struct {
	alerts        []SystemAlert
	maxAlerts     int
	mu            sync.RWMutex
	log           *slog.Logger
	webhookURL    string
	emailAddress  string
}

// NewAlertManager 创建告警管理器
func NewAlertManager(maxAlerts int, logger *slog.Logger) *AlertManager {
	if maxAlerts <= 0 {
		maxAlerts = 1000
	}
	
	return &AlertManager{
		alerts:    make([]SystemAlert, 0, maxAlerts),
		maxAlerts: maxAlerts,
		log:       logger,
	}
}

// SendAlert 发送告警
func (am *AlertManager) SendAlert(alert SystemAlert) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// 添加时间戳
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}
	
	// 保存到内存（用于API查询）
	am.alerts = append(am.alerts, alert)
	if len(am.alerts) > am.maxAlerts {
		am.alerts = am.alerts[1:]  // 保留最新的
	}
	
	// 记录日志
	am.log.Error("system alert",
		slog.String("type", string(alert.Type)),
		slog.String("level", string(alert.Level)),
		slog.String("message", alert.Message),
		slog.Any("data", alert.Data))
	
	// 保存到数据库
	am.saveToDatabase(alert)
	
	// 发送通知（webhook/邮件等）
	am.sendNotification(alert)
}

// saveToDatabase 保存告警到数据库
func (am *AlertManager) saveToDatabase(alert SystemAlert) {
	// 创建系统告警记录
	dbAlert := &model.Alert{
		TaskID:         "system",
		TaskType:       string(alert.Type),
		AlgorithmID:    "system_monitor",
		AlgorithmName:  "系统监控",
		Result:         alert.Message,
		Confidence:     1.0,
		DetectionCount: 0, // 系统告警没有检测个数
		CreatedAt:      alert.Timestamp,
	}
	
	if err := data.CreateAlert(dbAlert); err != nil {
		am.log.Error("failed to save system alert to database",
			slog.String("err", err.Error()))
	}
}

// sendNotification 发送通知
func (am *AlertManager) sendNotification(alert SystemAlert) {
	// TODO: 实现webhook通知
	if am.webhookURL != "" {
		// HTTP POST to webhook
		am.log.Info("sending alert to webhook",
			slog.String("url", am.webhookURL),
			slog.String("type", string(alert.Type)))
	}
	
	// TODO: 实现邮件通知
	if am.emailAddress != "" {
		am.log.Info("sending alert email",
			slog.String("email", am.emailAddress),
			slog.String("type", string(alert.Type)))
	}
}

// GetRecentAlerts 获取最近的告警
func (am *AlertManager) GetRecentAlerts(n int) []SystemAlert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if n <= 0 || n > len(am.alerts) {
		n = len(am.alerts)
	}
	
	// 返回最新的N个
	result := make([]SystemAlert, n)
	start := len(am.alerts) - n
	copy(result, am.alerts[start:])
	
	return result
}

// ClearAlerts 清空告警历史
func (am *AlertManager) ClearAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.alerts = am.alerts[:0]
}

