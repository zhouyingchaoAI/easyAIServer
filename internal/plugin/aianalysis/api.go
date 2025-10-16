package aianalysis

import (
	"encoding/json"
	"net/http"
)

// PerformanceStatsResponse 性能统计响应
type PerformanceStatsResponse struct {
	Queue       map[string]interface{} `json:"queue"`
	Performance map[string]interface{} `json:"performance"`
	DropRate    float64                `json:"drop_rate"`
	Healthy     bool                   `json:"healthy"`
}

// HandlePerformanceStats 处理性能统计请求
func HandlePerformanceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service := GetGlobal()
	if service == nil {
		http.Error(w, "AI analysis service not running", http.StatusServiceUnavailable)
		return
	}

	if service.queue == nil || service.monitor == nil {
		http.Error(w, "Smart inference not initialized", http.StatusInternalServerError)
		return
	}

	queueStats := service.queue.GetStats()
	perfStats := service.monitor.GetStats()
	dropRate := service.queue.GetDropRate()

	// 判断系统是否健康
	healthy := true
	if dropRate > 0.3 {
		healthy = false // 丢弃率过高
	}
	if avgTime, ok := perfStats["avg_time_ms"].(int64); ok && avgTime > 3000 {
		healthy = false // 平均推理时间过长
	}

	response := PerformanceStatsResponse{
		Queue:       queueStats,
		Performance: perfStats,
		DropRate:    dropRate,
		Healthy:     healthy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleQueueReset 处理队列重置请求
func HandleQueueReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service := GetGlobal()
	if service == nil {
		http.Error(w, "AI analysis service not running", http.StatusServiceUnavailable)
		return
	}

	if service.queue == nil {
		http.Error(w, "Smart inference not initialized", http.StatusInternalServerError)
		return
	}

	// 清空队列
	service.queue.Clear()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Queue cleared successfully",
	})
}

