package aianalysis

import (
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ResponseTimeWindow 响应时间滑动窗口大小
const ResponseTimeWindow = 10

// AlgorithmRegistry 算法服务注册中心
type AlgorithmRegistry struct {
	// services maps task_type -> list of algorithm services
	services  map[string][]conf.AlgorithmService
	mu        sync.RWMutex
	log       *slog.Logger
	timeout   time.Duration // heartbeat timeout
	stopCheck chan struct{}
	onRegisterCallback   func(serviceID string, taskTypes []string)  // 注册回调
	onUnregisterCallback func(serviceID string, reason string)       // 注销回调
	
	// 负载均衡：记录每个算法实例的调用次数
	// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
	callCounters map[string]int // algorithm endpoint -> call count
	
	// 性能统计：记录每个算法实例的响应时间
	responseTimes map[string][]int64 // algorithm endpoint -> response times (ms) in sliding window
	
	// 加权轮询：每个任务类型的当前权重计数器
	weightCounters map[string]int // task_type -> current weight counter
	
	// Round-Robin索引：作为兜底策略
	rrIndexes map[string]int // task_type -> round-robin index
}

// NewRegistry 创建注册中心
func NewRegistry(timeoutSec int, logger *slog.Logger) *AlgorithmRegistry {
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	return &AlgorithmRegistry{
		services:       make(map[string][]conf.AlgorithmService),
		log:            logger,
		timeout:        time.Duration(timeoutSec) * time.Second,
		stopCheck:      make(chan struct{}),
		callCounters:   make(map[string]int),
		responseTimes:  make(map[string][]int64),
		weightCounters: make(map[string]int),
		rrIndexes:      make(map[string]int),
	}
}

// SetOnRegisterCallback 设置注册回调
func (r *AlgorithmRegistry) SetOnRegisterCallback(callback func(serviceID string, taskTypes []string)) {
	r.onRegisterCallback = callback
}

// SetOnUnregisterCallback 设置注销回调
func (r *AlgorithmRegistry) SetOnUnregisterCallback(callback func(serviceID string, reason string)) {
	r.onUnregisterCallback = callback
}

// Register 注册算法服务
func (r *AlgorithmRegistry) Register(service conf.AlgorithmService) error {
	if service.ServiceID == "" || service.Endpoint == "" {
		return fmt.Errorf("service_id and endpoint required")
	}
	if len(service.TaskTypes) == 0 {
		return fmt.Errorf("task_types required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	service.RegisterAt = now
	service.LastHeartbeat = now

	// 为每个支持的任务类型注册
	for _, taskType := range service.TaskTypes {
		// 移除相同endpoint的旧服务（按endpoint去重）
		removed := r.removeServiceByEndpointLocked(service.Endpoint, taskType)
		
		// 添加新服务
		r.services[taskType] = append(r.services[taskType], service)
		
		// 如果移除了旧服务，重置Round-Robin索引以确保公平分配
		if removed {
			r.rrIndexes[taskType] = 0
		}
	}

	// 获取当前所有唯一endpoint列表（用于调试）
	allInstances := r.ListAllServiceInstancesLocked()
	totalServices := len(allInstances)
	
	// 收集所有endpoint用于日志
	endpoints := make([]string, 0, totalServices)
	for _, inst := range allInstances {
		endpoints = append(endpoints, inst.Endpoint)
	}
	
	r.log.Info("algorithm service registered successfully",
		slog.String("service_id", service.ServiceID),
		slog.String("name", service.Name),
		slog.Any("task_types", service.TaskTypes),
		slog.String("endpoint", service.Endpoint),
		slog.String("version", service.Version),
		slog.Int("total_services", totalServices),
		slog.Any("all_endpoints", endpoints))

	// 触发注册回调（异步）
	if r.onRegisterCallback != nil {
		go r.onRegisterCallback(service.ServiceID, service.TaskTypes)
	}

	return nil
}

// Unregister 注销算法服务
func (r *AlgorithmRegistry) Unregister(serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	var removedEndpoint string
	
	// 先找到endpoint（在删除之前）
	for _, services := range r.services {
		for _, svc := range services {
			if svc.ServiceID == serviceID {
				removedEndpoint = svc.Endpoint
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	
	if !found {
		return fmt.Errorf("service not found")
	}
	
	// 从所有任务类型中移除
	for taskType := range r.services {
		r.removeServiceByIDLocked(serviceID, taskType)
	}
	
	totalServices := len(r.ListAllServiceInstancesLocked())

	r.log.Info("algorithm service unregistered successfully",
		slog.String("service_id", serviceID),
		slog.String("endpoint", removedEndpoint),
		slog.Int("remaining_services", totalServices))
	return nil
}

// Heartbeat 更新心跳时间（按ServiceID）
func (r *AlgorithmRegistry) Heartbeat(serviceID string) error {
	return r.HeartbeatWithStats(serviceID, nil)
}

// HeartbeatWithStats 更新心跳时间并更新性能统计（按ServiceID）
func (r *AlgorithmRegistry) HeartbeatWithStats(serviceID string, stats *conf.HeartbeatRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	found := false
	var endpoint string

	// 更新所有匹配服务的心跳时间和性能统计
	for taskType, services := range r.services {
		for i := range services {
			if services[i].ServiceID == serviceID {
				services[i].LastHeartbeat = now
				endpoint = services[i].Endpoint
				
				// 更新性能统计（如果提供）
				if stats != nil {
					services[i].TotalRequests = stats.TotalRequests
					services[i].AvgInferenceTimeMs = stats.AvgInferenceTimeMs
					services[i].LastInferenceTimeMs = stats.LastInferenceTimeMs
					services[i].LastTotalTimeMs = stats.LastTotalTimeMs
				}
				
				found = true
			}
		}
		r.services[taskType] = services
	}

	if !found {
		return fmt.Errorf("service not found: %s", serviceID)
	}
	
	if stats != nil && stats.TotalRequests > 0 {
		r.log.Debug("heartbeat with stats received",
			slog.String("service_id", serviceID),
			slog.String("endpoint", endpoint),
			slog.Int64("total_requests", stats.TotalRequests),
			slog.Float64("avg_inference_ms", stats.AvgInferenceTimeMs),
			slog.Float64("last_inference_ms", stats.LastInferenceTimeMs),
			slog.Float64("last_total_ms", stats.LastTotalTimeMs))
	} else {
		r.log.Debug("heartbeat received",
			slog.String("service_id", serviceID),
			slog.String("endpoint", endpoint))
	}

	return nil
}

// HeartbeatByEndpoint 更新心跳时间（按Endpoint）
// 用于支持多个相同ServiceID但不同Endpoint的实例
func (r *AlgorithmRegistry) HeartbeatByEndpoint(endpoint string) error {
	return r.HeartbeatByEndpointWithStats(endpoint, nil)
}

// HeartbeatByEndpointWithStats 更新心跳时间并更新性能统计（按Endpoint）
func (r *AlgorithmRegistry) HeartbeatByEndpointWithStats(endpoint string, stats *conf.HeartbeatRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	found := false
	var serviceID string

	// 更新所有匹配服务的心跳时间和性能统计
	for taskType, services := range r.services {
		for i := range services {
			if services[i].Endpoint == endpoint {
				services[i].LastHeartbeat = now
				serviceID = services[i].ServiceID
				
				// 更新性能统计（如果提供）
				if stats != nil {
					services[i].TotalRequests = stats.TotalRequests
					services[i].AvgInferenceTimeMs = stats.AvgInferenceTimeMs
					services[i].LastInferenceTimeMs = stats.LastInferenceTimeMs
					services[i].LastTotalTimeMs = stats.LastTotalTimeMs
				}
				
				found = true
			}
		}
		r.services[taskType] = services
	}

	if !found {
		return fmt.Errorf("service not found by endpoint: %s", endpoint)
	}
	
	if stats != nil && stats.TotalRequests > 0 {
		r.log.Debug("heartbeat with stats received by endpoint",
			slog.String("service_id", serviceID),
			slog.String("endpoint", endpoint),
			slog.Int64("total_requests", stats.TotalRequests),
			slog.Float64("avg_inference_ms", stats.AvgInferenceTimeMs),
			slog.Float64("last_inference_ms", stats.LastInferenceTimeMs),
			slog.Float64("last_total_ms", stats.LastTotalTimeMs))
	} else {
		r.log.Debug("heartbeat received by endpoint",
			slog.String("service_id", serviceID),
			slog.String("endpoint", endpoint))
	}

	return nil
}

// GetAlgorithms 获取指定任务类型的所有算法服务
func (r *AlgorithmRegistry) GetAlgorithms(taskType string) []conf.AlgorithmService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services, ok := r.services[taskType]
	if !ok {
		return nil
	}

	// 返回副本
	result := make([]conf.AlgorithmService, len(services))
	copy(result, services)
	return result
}

// ListAllServices 列出所有注册的服务
func (r *AlgorithmRegistry) ListAllServices() []conf.AlgorithmService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []conf.AlgorithmService
	seen := make(map[string]bool)

	for _, services := range r.services {
		for _, svc := range services {
			if !seen[svc.ServiceID] {
				all = append(all, svc)
				seen[svc.ServiceID] = true
			}
		}
	}

	return all
}

// StartHeartbeatChecker 启动心跳检测
func (r *AlgorithmRegistry) StartHeartbeatChecker() {
	checkTicker := time.NewTicker(30 * time.Second)
	reportTicker := time.NewTicker(5 * time.Minute)  // 每5分钟报告一次健康状态
	
	r.log.Info("algorithm service heartbeat checker started",
		slog.Int("check_interval_sec", 30),
		slog.Int("timeout_sec", int(r.timeout.Seconds())),
		slog.Int("health_report_interval_min", 5))
	
	go func() {
		for {
			select {
			case <-r.stopCheck:
				checkTicker.Stop()
				reportTicker.Stop()
				r.log.Info("heartbeat checker stopped")
				return
			case <-checkTicker.C:
				r.checkAndRemoveExpired()
			case <-reportTicker.C:
				r.logHealthStatus()
			}
		}
	}()
}

// StopHeartbeatChecker 停止心跳检测
func (r *AlgorithmRegistry) StopHeartbeatChecker() {
	close(r.stopCheck)
}

// ClearAllServices 清空所有注册的服务（用于清理测试数据）
func (r *AlgorithmRegistry) ClearAllServices() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 统计清理前的服务数
	totalBefore := len(r.ListAllServiceInstancesLocked())
	
	// 清空所有数据
	r.services = make(map[string][]conf.AlgorithmService)
	r.callCounters = make(map[string]int)
	r.rrIndexes = make(map[string]int)
	
	r.log.Warn("all algorithm services cleared",
		slog.Int("cleared_count", totalBefore))
	
	return totalBefore
}

// logHealthStatus 输出服务健康状态报告
func (r *AlgorithmRegistry) logHealthStatus() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	now := time.Now().Unix()
	totalServices := len(r.ListAllServiceInstancesLocked())
	
	if totalServices == 0 {
		r.log.Info("algorithm services health report - no services registered")
		return
	}
	
	// 统计各任务类型的服务数量
	taskTypeCount := make(map[string]int)
	for taskType, services := range r.services {
		taskTypeCount[taskType] = len(services)
	}
	
	r.log.Info("algorithm services health report",
		slog.Int("total_services", totalServices),
		slog.Any("task_type_distribution", taskTypeCount))
		
	// 详细列出每个服务的状态
	for _, svc := range r.ListAllServiceInstancesLocked() {
		age := now - svc.LastHeartbeat
		r.log.Info("  service status",
			slog.String("service_id", svc.ServiceID),
			slog.String("endpoint", svc.Endpoint),
			slog.Int64("heartbeat_age_sec", age),
			slog.Int("call_count", r.callCounters[svc.Endpoint]))
	}
}

// checkAndRemoveExpired 检查并移除超时服务
func (r *AlgorithmRegistry) checkAndRemoveExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	timeoutSec := int64(r.timeout.Seconds())
	
	totalExpired := 0
	totalAlive := 0

	var expiredServices []conf.AlgorithmService
	
	for taskType, services := range r.services {
		var alive []conf.AlgorithmService
		for _, svc := range services {
			age := now - svc.LastHeartbeat
			if age < timeoutSec {
				alive = append(alive, svc)
				totalAlive++
			} else {
				totalExpired++
				expiredServices = append(expiredServices, svc)
				r.log.Warn("algorithm service expired - auto removing",
					slog.String("service_id", svc.ServiceID),
					slog.String("name", svc.Name),
					slog.String("endpoint", svc.Endpoint),
					slog.String("task_type", taskType),
					slog.Int64("heartbeat_age_sec", age),
					slog.Int64("timeout_threshold_sec", timeoutSec))
			}
		}
		r.services[taskType] = alive
	}
	
	// 解锁后触发注销回调（避免死锁）
	r.mu.Unlock()
	
	// 触发注销回调
	if r.onUnregisterCallback != nil && len(expiredServices) > 0 {
		for _, svc := range expiredServices {
			go r.onUnregisterCallback(svc.ServiceID, "heartbeat_timeout")
		}
	}
	
	// 重新加锁
	r.mu.Lock()
	
	// 定期输出服务健康状态摘要
	if totalExpired > 0 {
		r.log.Info("heartbeat check completed - services expired",
			slog.Int("total_alive", totalAlive),
			slog.Int("total_expired", totalExpired))
	}
}

// removeServiceByIDLocked 移除指定ID的服务（需要已加锁）
func (r *AlgorithmRegistry) removeServiceByIDLocked(serviceID, taskType string) bool {
	services, ok := r.services[taskType]
	if !ok {
		return false
	}

	var filtered []conf.AlgorithmService
	found := false
	for _, svc := range services {
		if svc.ServiceID != serviceID {
			filtered = append(filtered, svc)
		} else {
			found = true
		}
	}

	r.services[taskType] = filtered
	return found
}

// removeServiceByEndpointLocked 移除指定endpoint的服务（需要已加锁）
func (r *AlgorithmRegistry) removeServiceByEndpointLocked(endpoint, taskType string) bool {
	services, ok := r.services[taskType]
	if !ok {
		return false
	}

	var filtered []conf.AlgorithmService
	found := false
	for _, svc := range services {
		if svc.Endpoint != endpoint {
			filtered = append(filtered, svc)
		} else {
			found = true
		}
	}

	r.services[taskType] = filtered
	return found
}

// ListAllServiceInstances 列出所有注册的服务实例（按endpoint去重，每个endpoint只返回一次）
func (r *AlgorithmRegistry) ListAllServiceInstances() []conf.AlgorithmService {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ListAllServiceInstancesLocked()
}

// ListAllServiceInstancesLocked 列出所有服务实例（需要已加锁）
func (r *AlgorithmRegistry) ListAllServiceInstancesLocked() []conf.AlgorithmService {
	var all []conf.AlgorithmService
	seenEndpoints := make(map[string]bool)  // 按endpoint去重

	for _, services := range r.services {
		for _, svc := range services {
			// 按endpoint去重，每个endpoint只显示一次
			if !seenEndpoints[svc.Endpoint] {
				all = append(all, svc)
				seenEndpoints[svc.Endpoint] = true
			}
		}
	}

	return all
}

// GetAlgorithmWithLoadBalance 使用负载均衡策略选择一个算法实例（不增加计数）
// 策略：自适应负载均衡
// 1. 如果所有实例调用次数相同，使用Round-Robin轮询
// 2. 否则选择调用次数最少的实例
// 注意：此函数只负责选择，不增加计数。计数应该在调用成功后通过 IncrementCallCount 增加
func (r *AlgorithmRegistry) GetAlgorithmWithLoadBalance(taskType string) *conf.AlgorithmService {
	r.mu.Lock()
	defer r.mu.Unlock()

	services, ok := r.services[taskType]
	if !ok || len(services) == 0 {
		r.log.Warn("no algorithm service available for task type",
			slog.String("task_type", taskType),
			slog.Int("total_registered_endpoints", len(r.ListAllServiceInstancesLocked())))
		return nil
	}
	
	// 记录可用服务列表（debug）
	endpoints := make([]string, len(services))
	callCounts := make([]int, len(services))
	for i, svc := range services {
		endpoints[i] = svc.Endpoint
		callCounts[i] = r.callCounters[svc.Endpoint]
	}
	
	r.log.Debug("load balance: available services",
		slog.String("task_type", taskType),
		slog.Int("service_count", len(services)),
		slog.Any("endpoints", endpoints),
		slog.Any("call_counts", callCounts))

	if len(services) == 1 {
		// 只有一个实例，直接返回（不增加计数）
		selected := &services[0]
		
		r.log.Debug("load balance: single service",
			slog.String("task_type", taskType),
			slog.String("endpoint", selected.Endpoint))
		
		return selected
	}

	// 负载均衡策略：加权轮询（Weighted Round Robin）
	// 核心原则：
	// 1. 保证每个服务都能获得请求（公平性）
	// 2. 根据总耗时动态调整分配权重（性能优化）
	// 3. 响应越快，权重越高，获得更多请求
	
	type serviceWeight struct {
		index       int
		endpoint    string
		serviceID   string
		avgRespTime int64  // 平均响应时间（毫秒）
		weight      int    // 计算出的权重
		hasData     bool   // 是否有响应时间数据
	}
	
	// 计算每个服务的权重
	weights := make([]serviceWeight, len(services))
	totalWeight := 0
	newServiceCount := 0
	
	for i, svc := range services {
		times := r.responseTimes[svc.Endpoint]
		
		if len(times) == 0 {
			// 新服务：给予中等权重，让它们快速参与并收集数据
			weights[i] = serviceWeight{
				index:       i,
				endpoint:    svc.Endpoint,
				serviceID:   svc.ServiceID,
				avgRespTime: 0,
				weight:      10,  // 新服务默认权重10
				hasData:     false,
			}
			newServiceCount++
			totalWeight += 10
		} else {
			// 计算平均响应时间
			var sum int64
			for _, t := range times {
				sum += t
			}
			avgTime := sum / int64(len(times))
			
			// 计算权重：响应时间越短，权重越高
			// 权重公式：weight = max(1, 1000 / avgTime)
			// 例如：50ms → 权重20，100ms → 权重10，200ms → 权重5
			var weight int
			if avgTime > 0 {
				weight = int(1000 / avgTime)
				if weight < 1 {
					weight = 1  // 最小权重1，保证每个服务都能获得请求
				}
				if weight > 100 {
					weight = 100  // 最大权重100，避免极端情况
				}
			} else {
				weight = 10  // 默认权重
			}
			
			weights[i] = serviceWeight{
				index:       i,
				endpoint:    svc.Endpoint,
				serviceID:   svc.ServiceID,
				avgRespTime: avgTime,
				weight:      weight,
				hasData:     true,
			}
			totalWeight += weight
		}
	}
	
	// 使用加权轮询选择服务
	counter := r.weightCounters[taskType]
	r.weightCounters[taskType] = (counter + 1) % totalWeight
	
	// 找出counter对应的服务
	var selected *conf.AlgorithmService
	cumulative := 0
	selectedIdx := 0
	
	for i, w := range weights {
		cumulative += w.weight
		if counter < cumulative {
			selected = &services[i]
			selectedIdx = i
			break
		}
	}
	
	// 如果没有找到（理论上不应该发生），使用Round-Robin兜底
	if selected == nil {
		idx := r.rrIndexes[taskType] % len(services)
		selected = &services[idx]
		selectedIdx = idx
		r.rrIndexes[taskType] = (r.rrIndexes[taskType] + 1) % len(services)
		
		r.log.Warn("load balance: fallback to round-robin",
			slog.String("task_type", taskType),
			slog.String("selected_endpoint", selected.Endpoint))
	} else {
		// 记录选择结果
		w := weights[selectedIdx]
		
		// 构建所有服务的权重信息（用于调试）
		weightInfo := make([]map[string]interface{}, len(weights))
		for i, weight := range weights {
			weightInfo[i] = map[string]interface{}{
				"endpoint":      weight.endpoint,
				"weight":        weight.weight,
				"avg_time_ms":   weight.avgRespTime,
				"has_data":      weight.hasData,
			}
		}
		
		if w.hasData {
			r.log.Debug("load balance: weighted round-robin selected",
				slog.String("task_type", taskType),
				slog.String("selected_endpoint", selected.Endpoint),
				slog.String("selected_service_id", selected.ServiceID),
				slog.Int64("avg_response_time_ms", w.avgRespTime),
				slog.Int("weight", w.weight),
				slog.Int("total_weight", totalWeight),
				slog.Int("counter", counter),
				slog.Int("total_services", len(services)),
				slog.Any("all_weights", weightInfo))
		} else {
			r.log.Debug("load balance: new service selected for data collection",
				slog.String("task_type", taskType),
				slog.String("selected_endpoint", selected.Endpoint),
				slog.String("selected_service_id", selected.ServiceID),
				slog.Int("weight", w.weight),
				slog.Int("new_services_count", newServiceCount),
				slog.Int("total_services", len(services)))
		}
	}
	
	return selected
}

// IncrementCallCount 增加调用计数（仅用于成功的调用）
// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
func (r *AlgorithmRegistry) IncrementCallCount(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callCounters[endpoint]++
}

// RecordInferenceSuccess 记录推理成功（增加调用计数，记录响应时间）
// 注意：推理成功只增加计数，不影响服务在线状态（服务状态由心跳决定）
// responseTimeMs: 推理响应时间（毫秒）
func (r *AlgorithmRegistry) RecordInferenceSuccess(endpoint string, responseTimeMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 增加成功计数
	r.callCounters[endpoint]++
	
	// 记录响应时间（使用滑动窗口，只保留最近N次）
	times := r.responseTimes[endpoint]
	times = append(times, responseTimeMs)
	if len(times) > ResponseTimeWindow {
		times = times[len(times)-ResponseTimeWindow:]
	}
	r.responseTimes[endpoint] = times
	
	// 计算平均响应时间
	var sum int64
	for _, t := range times {
		sum += t
	}
	avgTime := sum / int64(len(times))
	
	r.log.Debug("inference success recorded",
		slog.String("endpoint", endpoint),
		slog.Int("success_count", r.callCounters[endpoint]),
		slog.Int64("response_time_ms", responseTimeMs),
		slog.Int64("avg_response_time_ms", avgTime),
		slog.Int("sample_count", len(times)))
}

// RecordInferenceFailure 记录推理失败（仅记录日志，不注销服务）
// 注意：推理失败不影响服务在线状态（服务状态由心跳决定）
// 服务的注销完全由心跳超时机制处理
func (r *AlgorithmRegistry) RecordInferenceFailure(endpoint, serviceID string) {
	// 只记录日志，不做任何状态改变
	r.log.Warn("inference call failed",
		slog.String("endpoint", endpoint),
		slog.String("service_id", serviceID),
		slog.String("note", "service status is managed by heartbeat, not by inference failures"))
}

// GetCallCount 获取调用次数
// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
func (r *AlgorithmRegistry) GetCallCount(endpoint string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.callCounters[endpoint]
}

// LoadBalanceInfo 负载均衡信息
type LoadBalanceInfo struct {
	TaskType      string                   `json:"task_type"`
	TotalServices int                      `json:"total_services"`
	Services      []ServiceLoadBalanceInfo `json:"services"`
	TotalWeight   int                      `json:"total_weight"`
	UpdatedAt     string                   `json:"updated_at"`
}

// ServiceLoadBalanceInfo 服务负载均衡信息
type ServiceLoadBalanceInfo struct {
	Endpoint        string  `json:"endpoint"`
	ServiceID       string  `json:"service_id"`
	Name            string  `json:"name"`
	AvgResponseMs   int64   `json:"avg_response_ms"`    // 平均响应时间
	Weight          int     `json:"weight"`             // 当前权重
	CallCount       int     `json:"call_count"`         // 调用次数
	AllocationRatio float64 `json:"allocation_ratio"`   // 分配比例（%）
	HasData         bool    `json:"has_data"`           // 是否有性能数据
}

// GetLoadBalanceInfo 获取指定任务类型的负载均衡信息
func (r *AlgorithmRegistry) GetLoadBalanceInfo(taskType string) *LoadBalanceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	services, ok := r.services[taskType]
	if !ok || len(services) == 0 {
		return nil
	}
	
	// 计算权重（与GetAlgorithmWithLoadBalance中的逻辑一致）
	totalWeight := 0
	serviceInfos := make([]ServiceLoadBalanceInfo, len(services))
	
	for i, svc := range services {
		times := r.responseTimes[svc.Endpoint]
		var avgTime int64
		var weight int
		hasData := false
		
		if len(times) == 0 {
			// 新服务：默认权重10
			weight = 10
		} else {
			// 计算平均响应时间
			var sum int64
			for _, t := range times {
				sum += t
			}
			avgTime = sum / int64(len(times))
			hasData = true
			
			// 计算权重：weight = max(1, min(100, 1000 / avgTime))
			if avgTime > 0 {
				weight = int(1000 / avgTime)
				if weight < 1 {
					weight = 1
				}
				if weight > 100 {
					weight = 100
				}
			} else {
				weight = 10
			}
		}
		
		serviceInfos[i] = ServiceLoadBalanceInfo{
			Endpoint:      svc.Endpoint,
			ServiceID:     svc.ServiceID,
			Name:          svc.Name,
			AvgResponseMs: avgTime,
			Weight:        weight,
			CallCount:     r.callCounters[svc.Endpoint],
			HasData:       hasData,
		}
		totalWeight += weight
	}
	
	// 计算分配比例
	for i := range serviceInfos {
		if totalWeight > 0 {
			serviceInfos[i].AllocationRatio = float64(serviceInfos[i].Weight) / float64(totalWeight) * 100
		}
	}
	
	return &LoadBalanceInfo{
		TaskType:      taskType,
		TotalServices: len(services),
		Services:      serviceInfos,
		TotalWeight:   totalWeight,
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}
}

// GetAllLoadBalanceInfo 获取所有任务类型的负载均衡信息
func (r *AlgorithmRegistry) GetAllLoadBalanceInfo() map[string]*LoadBalanceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make(map[string]*LoadBalanceInfo)
	
	for taskType := range r.services {
		r.mu.RUnlock()
		info := r.GetLoadBalanceInfo(taskType)
		r.mu.RLock()
		
		if info != nil {
			result[taskType] = info
		}
	}
	
	return result
}

// ServiceStat 服务统计信息
type ServiceStat struct {
	ServiceID     string   `json:"service_id"`
	Name          string   `json:"name"`
	Endpoint      string   `json:"endpoint"`
	Version       string   `json:"version"`
	TaskTypes     []string `json:"task_types"`
	CallCount     int      `json:"call_count"`
	LastHeartbeat int64    `json:"last_heartbeat"`
	RegisterAt    int64    `json:"register_at"`
}

// GetServiceStats 获取服务统计信息
func (r *AlgorithmRegistry) GetServiceStats(taskType string) []ServiceStat {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services, ok := r.services[taskType]
	if !ok {
		return nil
	}

	stats := make([]ServiceStat, len(services))
	for i, svc := range services {
		stats[i] = ServiceStat{
			ServiceID:     svc.ServiceID,
			Name:          svc.Name,
			Endpoint:      svc.Endpoint,
			Version:       svc.Version,
			TaskTypes:     svc.TaskTypes,
			CallCount:     r.callCounters[svc.Endpoint], // 使用endpoint作为key
			LastHeartbeat: svc.LastHeartbeat,
			RegisterAt:    svc.RegisterAt,
		}
	}

	return stats
}

