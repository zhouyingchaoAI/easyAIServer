package aianalysis

import (
	"log/slog"
	"sync"
	"time"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	// 统计数据
	totalInferences    int64
	failedInferences   int64  // 失败次数
	totalInferenceTime int64  // 毫秒
	avgInferenceTime   float64 // 毫秒
	maxInferenceTime   int64  // 最大推理时间
	
	// 速率统计
	frameRate      float64 // 抽帧速率（张/秒）
	inferenceRate  float64 // 推理速率（张/秒）
	lastUpdateTime time.Time
	
	// 慢推理告警
	slowThresholdMs int64
	slowCount       int64
	lastSlowAlert   time.Time
	
	// 线程安全
	mu sync.RWMutex
	
	// 日志
	log *slog.Logger
	
	// 告警回调
	alertCallback func(AlertInfo)
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(slowThresholdMs int64, logger *slog.Logger) *PerformanceMonitor {
	if slowThresholdMs <= 0 {
		slowThresholdMs = 5000  // 默认5秒
	}
	
	return &PerformanceMonitor{
		slowThresholdMs: slowThresholdMs,
		lastUpdateTime:  time.Now(),
		log:             logger,
	}
}

// SetAlertCallback 设置告警回调
func (m *PerformanceMonitor) SetAlertCallback(callback func(AlertInfo)) {
	m.alertCallback = callback
}

// RecordInference 记录一次推理
func (m *PerformanceMonitor) RecordInference(inferenceTimeMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !success {
		m.failedInferences++
		return  // 失败的不计入时间统计
	}
	
	m.totalInferences++
	m.totalInferenceTime += inferenceTimeMs
	m.avgInferenceTime = float64(m.totalInferenceTime) / float64(m.totalInferences)
	
	// 更新最大推理时间
	if inferenceTimeMs > m.maxInferenceTime {
		m.maxInferenceTime = inferenceTimeMs
	}
	
	// 更新推理速率（每分钟更新一次）
	now := time.Now()
	if now.Sub(m.lastUpdateTime) > 60*time.Second {
		duration := now.Sub(m.lastUpdateTime).Seconds()
		m.inferenceRate = float64(m.totalInferences) / duration
		m.lastUpdateTime = now
		
		m.log.Info("performance updated",
			slog.Float64("avg_inference_ms", m.avgInferenceTime),
			slog.Float64("inference_rate", m.inferenceRate))
	}
	
	// 检查慢推理
	if inferenceTimeMs > m.slowThresholdMs {
		m.slowCount++
		m.checkSlowInferenceAlertLocked(inferenceTimeMs)
	}
}

// checkSlowInferenceAlertLocked 检查慢推理告警（需要已加锁）
func (m *PerformanceMonitor) checkSlowInferenceAlertLocked(inferenceTimeMs int64) {
	now := time.Now()
	if now.Sub(m.lastSlowAlert) < 60*time.Second {
		return  // 避免频繁告警
	}
	
	m.lastSlowAlert = now
	
	alert := AlertInfo{
		Type:      "slow_inference",
		Level:     "warning",
		Message:   "推理速度过慢，建议优化算法或使用GPU加速",
		Timestamp: now,
	}
	
	m.log.Warn("slow inference alert",
		slog.Int64("inference_time_ms", inferenceTimeMs),
		slog.Int64("threshold_ms", m.slowThresholdMs),
		slog.Float64("avg_time_ms", m.avgInferenceTime),
		slog.Int64("slow_count", m.slowCount))
	
	// 触发告警回调
	if m.alertCallback != nil {
		m.alertCallback(alert)
	}
}

// CalculateSamplingRate 计算推荐的采样率
func (m *PerformanceMonitor) CalculateSamplingRate(frameIntervalMs int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.avgInferenceTime == 0 || m.totalInferences < 10 {
		return 1  // 初始阶段全部处理
	}
	
	// 计算抽帧速率和推理速率
	framesPerSec := 1000.0 / float64(frameIntervalMs)
	inferPerSec := 1000.0 / m.avgInferenceTime
	
	if inferPerSec >= framesPerSec*0.9 {
		return 1  // 推理能力充足（>=90%），全部处理
	}
	
	// 计算需要的采样率
	ratio := framesPerSec / inferPerSec
	samplingRate := int(ratio) + 1
	
	// 限制采样率范围
	if samplingRate < 1 {
		samplingRate = 1
	}
	if samplingRate > 100 {
		samplingRate = 100
	}
	
	m.log.Info("calculated recommended sampling rate",
		slog.Float64("frames_per_sec", framesPerSec),
		slog.Float64("infer_per_sec", inferPerSec),
		slog.Int("recommended_sampling", samplingRate),
		slog.String("reason", "推理速度跟不上抽帧速度"))
	
	return samplingRate
}

// GetStats 获取统计信息
func (m *PerformanceMonitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	inferPerSec := 0.0
	if m.avgInferenceTime > 0 {
		inferPerSec = 1000.0 / m.avgInferenceTime
	}
	
	return map[string]interface{}{
		"total_count":         m.totalInferences,
		"failed_count":        m.failedInferences,
		"total_inference_time": m.totalInferenceTime,
		"avg_inference_ms":    m.avgInferenceTime,
		"max_inference_ms":    m.maxInferenceTime,
		"inference_per_sec":   inferPerSec,
		"slow_count":          m.slowCount,
		"slow_threshold_ms":   m.slowThresholdMs,
	}
}

// Reset 重置统计
func (m *PerformanceMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalInferences = 0
	m.failedInferences = 0
	m.totalInferenceTime = 0
	m.avgInferenceTime = 0
	m.maxInferenceTime = 0
	m.slowCount = 0
	m.lastUpdateTime = time.Now()
	m.lastSlowAlert = time.Time{}
	
	m.log.Info("performance monitor stats reset")
}

