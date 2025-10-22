package data

import (
	"easydarwin/internal/data/model"
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
	if err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&alerts).Error; err != nil {
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

