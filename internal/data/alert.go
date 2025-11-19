package data

import (
	"easydarwin/internal/data/model"
	"log/slog"
	"sync"
	"time"
)

// CreateAlert 创建告警记录
func CreateAlert(alert *model.Alert) error {
	return GetDatabase().Create(alert).Error
}

// ListAlerts 查询告警列表
func ListAlerts(filter model.AlertFilter) ([]model.Alert, int64, error) {
	var alerts []model.Alert
	var total int64

	db := GetDatabase().Model(&model.Alert{})

	// 筛选条件
	if filter.TaskID != "" {
		db = db.Where("task_id = ?", filter.TaskID)
	}
	if filter.TaskType != "" {
		db = db.Where("task_type = ?", filter.TaskType)
	}
	if filter.MinDetections > 0 {
		db = db.Where("detection_count >= ?", filter.MinDetections)
	}
	if filter.MaxDetections > 0 {
		db = db.Where("detection_count <= ?", filter.MaxDetections)
	}
	if !filter.StartTime.IsZero() {
		db = db.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		db = db.Where("created_at <= ?", filter.EndTime)
	}

	// 计数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	// 确保最新告警在第一页：按创建时间倒序，如果时间相同则按ID倒序（ID越大越新）
	if err := db.Order("created_at DESC, id DESC").Limit(pageSize).Offset(offset).Find(&alerts).Error; err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}

// GetAlertByID 根据ID获取告警
func GetAlertByID(id uint) (*model.Alert, error) {
	var alert model.Alert
	if err := GetDatabase().First(&alert, id).Error; err != nil {
		return nil, err
	}
	return &alert, nil
}

// DeleteAlert 删除告警
func DeleteAlert(id uint) error {
	return GetDatabase().Delete(&model.Alert{}, id).Error
}

// BatchDeleteAlerts 批量删除告警
func BatchDeleteAlerts(ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	
	// 使用事务批量删除
	result := GetDatabase().Delete(&model.Alert{}, ids)
	if result.Error != nil {
		return 0, result.Error
	}
	
	return int(result.RowsAffected), nil
}

// GetDistinctTaskIDs 获取所有不重复的任务ID列表
func GetDistinctTaskIDs() ([]string, error) {
	var taskIDs []string
	err := GetDatabase().Model(&model.Alert{}).
		Distinct("task_id").
		Where("task_id != ''").
		Order("task_id ASC").
		Pluck("task_id", &taskIDs).Error
	
	if err != nil {
		return nil, err
	}
	
	return taskIDs, nil
}

// AutoMigrate 自动迁移alert表
func MigrateAlertTable() error {
	return GetDatabase().AutoMigrate(&model.Alert{})
}

// AlertBatchWriter 批量写入告警记录
type AlertBatchWriter struct {
	buffer    []*model.Alert
	mu        sync.Mutex
	batchSize int
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
	log       *slog.Logger
	enabled   bool
}

// NewAlertBatchWriter 创建批量写入器
func NewAlertBatchWriter(batchSize int, intervalSec int, enabled bool, logger *slog.Logger) *AlertBatchWriter {
	if batchSize <= 0 {
		batchSize = 100
	}
	if intervalSec <= 0 {
		intervalSec = 2
	}
	
	return &AlertBatchWriter{
		buffer:    make([]*model.Alert, 0, batchSize),
		batchSize: batchSize,
		interval:  time.Duration(intervalSec) * time.Second,
		stopCh:    make(chan struct{}),
		log:       logger,
		enabled:   enabled,
	}
}

// Start 启动批量写入器
func (w *AlertBatchWriter) Start() {
	if !w.enabled {
		w.log.Info("alert batch writer is disabled, using direct write mode")
		return
	}
	
	w.wg.Add(1)
	go w.flushLoop()
	w.log.Info("alert batch writer started",
		slog.Int("batch_size", w.batchSize),
		slog.Duration("interval", w.interval))
}

// Stop 停止批量写入器并刷新剩余数据
func (w *AlertBatchWriter) Stop() {
	if !w.enabled {
		return
	}
	
	close(w.stopCh)
	w.wg.Wait()
	
	// 最后刷新一次
	w.flush()
	w.log.Info("alert batch writer stopped")
}

// Add 添加告警到批量队列
func (w *AlertBatchWriter) Add(alert *model.Alert) error {
	if !w.enabled {
		// 批量写入未启用，直接写入
		return CreateAlert(alert)
	}
	
	w.mu.Lock()
	w.buffer = append(w.buffer, alert)
	needFlush := len(w.buffer) >= w.batchSize
	w.mu.Unlock()
	
	// 达到批量大小，立即刷新
	if needFlush {
		w.flush()
	}
	
	return nil
}

// flushLoop 定时刷新循环
func (w *AlertBatchWriter) flushLoop() {
	defer w.wg.Done()
	
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.stopCh:
			return
		}
	}
}

// flush 批量写入数据库
func (w *AlertBatchWriter) flush() {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	
	// 复制缓冲区并清空
	toFlush := make([]*model.Alert, len(w.buffer))
	copy(toFlush, w.buffer)
	w.buffer = w.buffer[:0]
	w.mu.Unlock()
	
	// 批量插入
	startTime := time.Now()
	err := GetDatabase().CreateInBatches(toFlush, len(toFlush)).Error
	duration := time.Since(startTime)
	
	if err != nil {
		w.log.Error("failed to batch insert alerts",
			slog.Int("count", len(toFlush)),
			slog.Duration("duration", duration),
			slog.String("err", err.Error()))
		
		// 失败时尝试逐条插入（降级处理）
		w.log.Info("trying to insert alerts one by one as fallback")
		successCount := 0
		for _, alert := range toFlush {
			if err := CreateAlert(alert); err == nil {
				successCount++
			}
		}
		w.log.Info("fallback insert completed",
			slog.Int("success", successCount),
			slog.Int("failed", len(toFlush)-successCount))
	} else {
		w.log.Info("batch insert alerts succeeded",
			slog.Int("count", len(toFlush)),
			slog.Duration("duration", duration),
			slog.Float64("avg_ms_per_alert", float64(duration.Milliseconds())/float64(len(toFlush))))
	}
}

// GetQueueSize 获取当前队列大小
func (w *AlertBatchWriter) GetQueueSize() int {
	if !w.enabled {
		return 0
	}
	
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buffer)
}

