package videortsp

import (
	"time"
)

// StreamTask 视频转RTSP流任务
type StreamTask struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`        // 任务名称
	VideoPath   string    `gorm:"not null" json:"videoPath"`   // 视频文件路径
	StreamName  string    `gorm:"not null;uniqueIndex" json:"streamName"` // RTSP流名称，如 "loop"
	RTSPURL     string    `gorm:"-" json:"rtspUrl"`            // RTSP播放地址（计算字段）
	Status      string    `gorm:"default:'stopped'" json:"status"` // 状态: stopped, running, error
	Enabled     bool      `gorm:"default:false" json:"enabled"`    // 是否启用
	Loop        bool      `gorm:"default:true" json:"loop"`        // 是否循环播放
	VideoCodec  string    `json:"videoCodec"`                     // 视频编码: libx264
	AudioCodec  string    `json:"audioCodec"`                     // 音频编码: aac
	Preset      string    `json:"preset"`                         // 编码预设: ultrafast
	Tune        string    `json:"tune"`                           // 编码调优: zerolatency
	Error       string    `gorm:"type:text" json:"error"`         // 错误信息
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (StreamTask) TableName() string {
	return "video_rtsp_stream"
}

const (
	StatusStopped = "stopped"
	StatusRunning = "running"
	StatusError   = "error"
)

