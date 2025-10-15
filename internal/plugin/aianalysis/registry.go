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
}

// NewRegistry 创建注册中心
func NewRegistry(timeoutSec int, logger *slog.Logger) *AlgorithmRegistry {
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	return &AlgorithmRegistry{
		services:  make(map[string][]conf.AlgorithmService),
		log:       logger,
		timeout:   time.Duration(timeoutSec) * time.Second,
		stopCheck: make(chan struct{}),
	}
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
		// 移除同ID的旧服务
		r.removeServiceByIDLocked(service.ServiceID, taskType)
		
		// 添加新服务
		r.services[taskType] = append(r.services[taskType], service)
	}

	r.log.Info("algorithm service registered",
		slog.String("service_id", service.ServiceID),
		slog.String("name", service.Name),
		slog.Any("task_types", service.TaskTypes),
		slog.String("endpoint", service.Endpoint))

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

// Heartbeat 更新心跳时间
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

