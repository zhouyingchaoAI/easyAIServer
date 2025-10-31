package aianalysis

import (
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AlgorithmRegistry 算法服务注册中心
type AlgorithmRegistry struct {
	// services maps task_type -> list of algorithm services
	services  map[string][]conf.AlgorithmService
	mu        sync.RWMutex
	log       *slog.Logger
	timeout   time.Duration // heartbeat timeout
	stopCheck chan struct{}
	onRegisterCallback func(serviceID string, taskTypes []string) // 注册回调
	
	// 负载均衡：记录每个算法实例的调用次数
	// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
	callCounters map[string]int // algorithm endpoint -> call count
	
	// Round-Robin索引：每个任务类型的当前选择索引
	rrIndexes map[string]int // task_type -> round-robin index
}

// NewRegistry 创建注册中心
func NewRegistry(timeoutSec int, logger *slog.Logger) *AlgorithmRegistry {
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	return &AlgorithmRegistry{
		services:      make(map[string][]conf.AlgorithmService),
		log:           logger,
		timeout:       time.Duration(timeoutSec) * time.Second,
		stopCheck:     make(chan struct{}),
		callCounters:  make(map[string]int),
		rrIndexes:     make(map[string]int),
	}
}

// SetOnRegisterCallback 设置注册回调
func (r *AlgorithmRegistry) SetOnRegisterCallback(callback func(serviceID string, taskTypes []string)) {
	r.onRegisterCallback = callback
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

	r.log.Info("algorithm service registered",
		slog.String("service_id", service.ServiceID),
		slog.String("name", service.Name),
		slog.Any("task_types", service.TaskTypes),
		slog.String("endpoint", service.Endpoint))

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
	// 从所有任务类型中移除
	for taskType := range r.services {
		if r.removeServiceByIDLocked(serviceID, taskType) {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("service not found")
	}

	r.log.Info("algorithm service unregistered", slog.String("service_id", serviceID))
	return nil
}

// Heartbeat 更新心跳时间（按ServiceID）
func (r *AlgorithmRegistry) Heartbeat(serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	found := false

	// 更新所有匹配服务的心跳时间
	for taskType, services := range r.services {
		for i := range services {
			if services[i].ServiceID == serviceID {
				services[i].LastHeartbeat = now
				found = true
			}
		}
		r.services[taskType] = services
	}

	if !found {
		return fmt.Errorf("service not found")
	}

	return nil
}

// HeartbeatByEndpoint 更新心跳时间（按Endpoint）
// 用于支持多个相同ServiceID但不同Endpoint的实例
func (r *AlgorithmRegistry) HeartbeatByEndpoint(endpoint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	found := false

	// 更新所有匹配服务的心跳时间
	for taskType, services := range r.services {
		for i := range services {
			if services[i].Endpoint == endpoint {
				services[i].LastHeartbeat = now
				found = true
			}
		}
		r.services[taskType] = services
	}

	if !found {
		return fmt.Errorf("service not found by endpoint: %s", endpoint)
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
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-r.stopCheck:
				ticker.Stop()
				return
			case <-ticker.C:
				r.checkAndRemoveExpired()
			}
		}
	}()
}

// StopHeartbeatChecker 停止心跳检测
func (r *AlgorithmRegistry) StopHeartbeatChecker() {
	close(r.stopCheck)
}

// checkAndRemoveExpired 检查并移除超时服务
func (r *AlgorithmRegistry) checkAndRemoveExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	timeoutSec := int64(r.timeout.Seconds())

	for taskType, services := range r.services {
		var alive []conf.AlgorithmService
		for _, svc := range services {
			if now-svc.LastHeartbeat < timeoutSec {
				alive = append(alive, svc)
			} else {
				r.log.Warn("algorithm service expired",
					slog.String("service_id", svc.ServiceID),
					slog.String("name", svc.Name),
					slog.Int64("last_heartbeat", svc.LastHeartbeat),
					slog.Int64("now", now))
			}
		}
		r.services[taskType] = alive
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

// GetAlgorithmWithLoadBalance 使用负载均衡策略选择一个算法实例并增加调用计数
// 策略：自适应负载均衡
// 1. 如果所有实例调用次数相同，使用Round-Robin轮询
// 2. 否则选择调用次数最少的实例
// 注意：此函数内部会同时选择实例并递增计数，保证原子性
func (r *AlgorithmRegistry) GetAlgorithmWithLoadBalance(taskType string) *conf.AlgorithmService {
	r.mu.Lock()
	defer r.mu.Unlock()

	services, ok := r.services[taskType]
	if !ok || len(services) == 0 {
		return nil
	}

	if len(services) == 1 {
		// 只有一个实例，直接返回并增加计数
		selected := &services[0]
		r.callCounters[selected.Endpoint]++
		return selected
	}

	// 收集所有实例的调用次数
	counts := make([]int, len(services))
	for i, svc := range services {
		counts[i] = r.callCounters[svc.Endpoint]
	}

	// 检查是否所有实例调用次数相同
	allEqual := true
	for i := 1; i < len(counts); i++ {
		if counts[i] != counts[0] {
			allEqual = false
			break
		}
	}

	var selected *conf.AlgorithmService
	var endpoint string
	
	if allEqual {
		// 所有实例负载相同，使用Round-Robin轮询
		idx := r.rrIndexes[taskType]
		selected = &services[idx % len(services)]
		endpoint = selected.Endpoint
		r.rrIndexes[taskType] = (idx + 1) % len(services)
		
		r.log.Debug("load balance: using round-robin",
			slog.String("task_type", taskType),
			slog.String("endpoint", endpoint),
			slog.Int("round_robin_index", idx),
			slog.Int("call_count", counts[idx]))
	} else {
		// 存在负载差异，选择调用次数最少的实例
		minCount := -1
		minIdx := 0
		for i := range services {
			count := counts[i]
			if minCount == -1 || count < minCount {
				minCount = count
				minIdx = i
			}
		}
		selected = &services[minIdx]
		endpoint = selected.Endpoint
		
		r.log.Debug("load balance: using least-load",
			slog.String("task_type", taskType),
			slog.String("endpoint", endpoint),
			slog.Int("call_count", minCount))
	}

	// 增加选中实例的调用计数
	r.callCounters[endpoint]++
	
	return selected
}

// IncrementCallCount 增加调用计数
// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
func (r *AlgorithmRegistry) IncrementCallCount(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callCounters[endpoint]++
}

// GetCallCount 获取调用次数
// 使用endpoint作为key，因为同一service_id可能有多个不同的endpoint实例
func (r *AlgorithmRegistry) GetCallCount(endpoint string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.callCounters[endpoint]
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

