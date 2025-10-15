package model

import (
	"time"

	"gorm.io/gorm"
)

// Alert 告警记录
type Alert struct {
	ID              uint           `json:"id" gorm:"primarykey"`
	TaskID          string         `json:"task_id" gorm:"type:varchar(100);index"`
	TaskType        string         `json:"task_type" gorm:"type:varchar(50);index"`
	ImagePath       string         `json:"image_path" gorm:"type:varchar(500)"`
	ImageURL        string         `json:"image_url" gorm:"type:varchar(1000)"` // 预签名URL或本地URL
	AlgorithmID     string         `json:"algorithm_id" gorm:"type:varchar(100)"`
	AlgorithmName   string         `json:"algorithm_name" gorm:"type:varchar(100)"`
	Result          string         `json:"result" gorm:"type:text"` // JSON格式推理结果
	Confidence      float64        `json:"confidence"`
	InferenceTimeMs int            `json:"inference_time_ms"`
	CreatedAt       time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (Alert) TableName() string {
	return "alerts"
}

// AlertFilter 告警筛选条件
type AlertFilter struct {
	TaskID     string    `form:"task_id"`
	TaskType   string    `form:"task_type"`
	StartTime  time.Time `form:"start_time"`
	EndTime    time.Time `form:"end_time"`
	Page       int       `form:"page"`
	PageSize   int       `form:"page_size"`
}

