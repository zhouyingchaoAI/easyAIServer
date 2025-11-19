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
	successInferences  int64  // 成功次数（只统计成功的推理）
	totalInferenceTime int64  // 毫秒
	avgInferenceTime   float64 // 毫秒
	maxInferenceTime   int64  // 最大推理时间
	
	// 速率统计
	frameRate      float64 // 抽帧速率（张/秒）
	inferenceRate  float64 // 推理速率（张/秒）
	successRatePerSec float64 // 每秒推理成功数（张/秒）
	lastUpdateTime time.Time
	lastSuccessTime time.Time // 上次成功推理时间（用于计算每秒成功数）
	successCountInWindow int64 // 窗口内的成功推理数
	windowStartTime time.Time // 窗口开始时间
	
	// 请求发送和响应统计
	requestCountInWindow int64 // 窗口内的请求发送数
	responseCountInWindow int64 // 窗口内的响应数（包括成功和失败）
	requestRatePerSec float64 // 每秒请求发送数
	responseRatePerSec float64 // 每秒响应数
	requestWindowStartTime time.Time // 请求窗口开始时间
	responseWindowStartTime time.Time // 响应窗口开始时间
	lastRequestTime time.Time // 最后一次请求发送时间（用于超时检测）
	lastResponseTime time.Time // 最后一次响应接收时间（用于超时检测）
	
	// 慢推理告警
	slowThresholdMs int64
	slowCount       int64
	lastSlowAlert   time.Time
	
	// MinIO操作监控（图片移动）
	minIOMoveTotal      int64   // 总移动次数
	minIOMoveSuccess    int64   // 成功次数
	minIOMoveFailed     int64   // 失败次数
	minIOMoveTotalTime  int64   // 总耗时（毫秒）
	minIOMoveAvgTime    float64 // 平均耗时（毫秒）
	minIOMoveMaxTime    int64   // 最大耗时（毫秒）
	
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
	
	now := time.Now()
	return &PerformanceMonitor{
		slowThresholdMs: slowThresholdMs,
		lastUpdateTime:  now,
		lastSuccessTime: now,
		windowStartTime: now,
		requestWindowStartTime: now,
		responseWindowStartTime: now,
		lastRequestTime: now,
		lastResponseTime: now,
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
	m.successInferences++
	m.totalInferenceTime += inferenceTimeMs
	m.avgInferenceTime = float64(m.totalInferenceTime) / float64(m.totalInferences)
	
	// 更新最大推理时间
	if inferenceTimeMs > m.maxInferenceTime {
		m.maxInferenceTime = inferenceTimeMs
	}
	
	// 更新每秒推理成功数（使用滑动窗口，每秒更新一次）
	now := time.Now()
	m.successCountInWindow++
	
	// 计算窗口内的每秒成功数（使用最近1秒的数据）
	windowDuration := now.Sub(m.windowStartTime).Seconds()
	if windowDuration >= 1.0 {
		// 窗口已满1秒，计算每秒成功数
		m.successRatePerSec = float64(m.successCountInWindow) / windowDuration
		// 重置窗口
		m.successCountInWindow = 0
		m.windowStartTime = now
	} else if windowDuration > 0 {
		// 窗口未满1秒，使用当前数据估算
		m.successRatePerSec = float64(m.successCountInWindow) / windowDuration
	}
	
	m.lastSuccessTime = now
	
	// 更新推理速率（每分钟更新一次）
	if now.Sub(m.lastUpdateTime) > 60*time.Second {
		duration := now.Sub(m.lastUpdateTime).Seconds()
		m.inferenceRate = float64(m.totalInferences) / duration
		m.lastUpdateTime = now
		
		m.log.Info("performance updated",
			slog.Float64("avg_inference_ms", m.avgInferenceTime),
			slog.Float64("inference_rate", m.inferenceRate),
			slog.Float64("success_rate_per_sec", m.successRatePerSec))
	}
}

// RecordRequestSent 记录一次请求发送
func (m *PerformanceMonitor) RecordRequestSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	m.requestCountInWindow++
	m.lastRequestTime = now // 更新最后请求时间
	
	// 计算窗口内的每秒请求数（使用最近1秒的数据）
	windowDuration := now.Sub(m.requestWindowStartTime).Seconds()
	if windowDuration >= 1.0 {
		// 窗口已满1秒，计算每秒请求数
		m.requestRatePerSec = float64(m.requestCountInWindow) / windowDuration
		// 重置窗口
		m.requestCountInWindow = 0
		m.requestWindowStartTime = now
	} else if windowDuration > 0 {
		// 窗口未满1秒，使用当前数据估算
		m.requestRatePerSec = float64(m.requestCountInWindow) / windowDuration
	}
}

// RecordResponseReceived 记录一次响应接收（包括成功和失败）
func (m *PerformanceMonitor) RecordResponseReceived() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	m.responseCountInWindow++
	m.lastResponseTime = now // 更新最后响应时间
	
	// 计算窗口内的每秒响应数（使用最近1秒的数据）
	windowDuration := now.Sub(m.responseWindowStartTime).Seconds()
	if windowDuration >= 1.0 {
		// 窗口已满1秒，计算每秒响应数
		m.responseRatePerSec = float64(m.responseCountInWindow) / windowDuration
		// 重置窗口
		m.responseCountInWindow = 0
		m.responseWindowStartTime = now
	} else if windowDuration > 0 {
		// 窗口未满1秒，使用当前数据估算
		m.responseRatePerSec = float64(m.responseCountInWindow) / windowDuration
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

// RecordMinIOMove 记录MinIO图片移动操作
func (m *PerformanceMonitor) RecordMinIOMove(success bool, durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.minIOMoveTotal++
	if success {
		m.minIOMoveSuccess++
		m.minIOMoveTotalTime += durationMs
		if m.minIOMoveSuccess > 0 {
			m.minIOMoveAvgTime = float64(m.minIOMoveTotalTime) / float64(m.minIOMoveSuccess)
		}
		if durationMs > m.minIOMoveMaxTime {
			m.minIOMoveMaxTime = durationMs
		}
	} else {
		m.minIOMoveFailed++
	}
}

// GetStats 获取统计信息
func (m *PerformanceMonitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	inferPerSec := 0.0
	if m.avgInferenceTime > 0 {
		inferPerSec = 1000.0 / m.avgInferenceTime
	}
	
	// 计算实时每秒成功数（如果窗口未满1秒，使用当前数据估算）
	now := time.Now()
	windowDuration := now.Sub(m.windowStartTime).Seconds()
	currentSuccessRate := m.successRatePerSec
	if windowDuration > 0 && windowDuration < 1.0 {
		// 窗口未满1秒，使用当前数据估算
		currentSuccessRate = float64(m.successCountInWindow) / windowDuration
	} else if windowDuration >= 1.0 {
		// 窗口已满，使用已计算的速率
		currentSuccessRate = m.successRatePerSec
	}
	
	// 检查成功推理超时：如果超过3秒没有新的成功推理，置零
	if !m.lastSuccessTime.IsZero() {
		timeSinceLastSuccess := now.Sub(m.lastSuccessTime).Seconds()
		if timeSinceLastSuccess > 3.0 {
			currentSuccessRate = 0.0
			m.successRatePerSec = 0.0
			m.successCountInWindow = 0
		}
	}
	
	// 计算实时每秒请求数
	requestWindowDuration := now.Sub(m.requestWindowStartTime).Seconds()
	currentRequestRate := m.requestRatePerSec
	if requestWindowDuration > 0 && requestWindowDuration < 1.0 {
		currentRequestRate = float64(m.requestCountInWindow) / requestWindowDuration
	} else if requestWindowDuration >= 1.0 {
		currentRequestRate = m.requestRatePerSec
	}
	
	// 检查请求超时：如果超过3秒没有新请求，置零
	if !m.lastRequestTime.IsZero() {
		timeSinceLastRequest := now.Sub(m.lastRequestTime).Seconds()
		if timeSinceLastRequest > 3.0 {
			currentRequestRate = 0.0
			m.requestRatePerSec = 0.0
			m.requestCountInWindow = 0
		}
	}
	
	// 计算实时每秒响应数
	responseWindowDuration := now.Sub(m.responseWindowStartTime).Seconds()
	currentResponseRate := m.responseRatePerSec
	if responseWindowDuration > 0 && responseWindowDuration < 1.0 {
		currentResponseRate = float64(m.responseCountInWindow) / responseWindowDuration
	} else if responseWindowDuration >= 1.0 {
		currentResponseRate = m.responseRatePerSec
	}
	
	// 检查响应超时：如果超过3秒没有新响应，置零
	if !m.lastResponseTime.IsZero() {
		timeSinceLastResponse := now.Sub(m.lastResponseTime).Seconds()
		if timeSinceLastResponse > 3.0 {
			currentResponseRate = 0.0
			m.responseRatePerSec = 0.0
			m.responseCountInWindow = 0
		}
	}
	
	// 计算MinIO移动成功率
	minIOMoveSuccessRate := 0.0
	if m.minIOMoveTotal > 0 {
		minIOMoveSuccessRate = float64(m.minIOMoveSuccess) / float64(m.minIOMoveTotal)
	}

	return map[string]interface{}{
		"total_count":         m.totalInferences,
		"success_count":       m.successInferences,
		"failed_count":        m.failedInferences,
		"total_inference_time": m.totalInferenceTime,
		"avg_inference_ms":    m.avgInferenceTime,
		"max_inference_ms":    m.maxInferenceTime,
		"inference_per_sec":   inferPerSec,
		"success_rate_per_sec": currentSuccessRate, // 每秒推理成功数
		"request_rate_per_sec": currentRequestRate, // 每秒请求发送数
		"response_rate_per_sec": currentResponseRate, // 每秒响应数
		"slow_count":          m.slowCount,
		"slow_threshold_ms":   m.slowThresholdMs,
		
		// MinIO操作监控（图片移动）
		"minio_move_total":        m.minIOMoveTotal,
		"minio_move_success":      m.minIOMoveSuccess,
		"minio_move_failed":       m.minIOMoveFailed,
		"minio_move_avg_time_ms":  m.minIOMoveAvgTime,
		"minio_move_max_time_ms":  m.minIOMoveMaxTime,
		"minio_move_success_rate": minIOMoveSuccessRate,
	}
}

// Reset 重置统计
func (m *PerformanceMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	m.totalInferences = 0
	m.failedInferences = 0
	m.successInferences = 0
	m.totalInferenceTime = 0
	m.avgInferenceTime = 0
	m.maxInferenceTime = 0
	m.slowCount = 0
	m.lastUpdateTime = now
	m.lastSuccessTime = now
	m.windowStartTime = now
	m.successCountInWindow = 0
	m.successRatePerSec = 0
	m.requestCountInWindow = 0
	m.responseCountInWindow = 0
	m.requestRatePerSec = 0
	m.responseRatePerSec = 0
	m.requestWindowStartTime = now
	m.responseWindowStartTime = now
	m.lastRequestTime = now
	m.lastResponseTime = now
	m.lastSlowAlert = time.Time{}
	
	m.log.Info("performance monitor stats reset")
}

