package videortsp

import (
	"gorm.io/gorm"
	"strings"
)

// Store 数据存储实现
type Store struct {
	db *gorm.DB
}

// NewStore 创建存储实例
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// AutoMigrate 自动迁移数据库表
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&StreamTask{})
}

// Create 创建任务
func (s *Store) Create(task *StreamTask) error {
	return s.db.Create(task).Error
}

// Update 更新任务
func (s *Store) Update(task *StreamTask) error {
	return s.db.Save(task).Error
}

// Delete 删除任务
func (s *Store) Delete(id string) error {
	return s.db.Delete(&StreamTask{}, "id = ?", id).Error
}

// GetByID 根据ID获取任务
func (s *Store) GetByID(id string) (*StreamTask, error) {
	var task StreamTask
	err := s.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByStreamName 根据流名称获取任务
func (s *Store) GetByStreamName(streamName string) (*StreamTask, error) {
	var task StreamTask
	err := s.db.Where("stream_name = ?", streamName).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// List 列出任务
func (s *Store) List(limit, offset int, search string) ([]*StreamTask, int64, error) {
	var tasks []*StreamTask
	var total int64

	query := s.db.Model(&StreamTask{})

	// 搜索
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(stream_name) LIKE ?", searchPattern, searchPattern)
	}

	// 获取总数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取列表
	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tasks).Error
	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// Storer 接口实现检查
var _ Storer = (*Store)(nil)

