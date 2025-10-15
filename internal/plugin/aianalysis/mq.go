package aianalysis

import (
	"easydarwin/internal/data/model"
)

// MessageQueue 消息队列接口
type MessageQueue interface {
	// Connect 连接到消息队列
	Connect() error
	
	// PublishAlert 发布告警消息
	PublishAlert(alert model.Alert) error
	
	// Close 关闭连接
	Close() error
}

