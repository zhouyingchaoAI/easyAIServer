package aianalysis

import (
	"context"
	"encoding/json"
	"easydarwin/internal/data/model"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaQueue Kafka消息队列实现
type KafkaQueue struct {
	writer *kafka.Writer
	topic  string
	log    *slog.Logger
}

// NewKafkaQueue 创建Kafka队列
func NewKafkaQueue(address, topic string, logger *slog.Logger) *KafkaQueue {
	return &KafkaQueue{
		topic: topic,
		log:   logger,
		writer: &kafka.Writer{
			Addr:            kafka.TCP(address),
			Topic:           topic,
			Balancer:        &kafka.LeastBytes{},
			WriteTimeout:    10 * time.Second,
			ReadTimeout:     10 * time.Second,
			Async:           false,             // 同步写入，确保消息发送成功
			RequiredAcks:    kafka.RequireOne, // 等待 leader 确认，避免消息丢失
			BatchSize:       1,                // 每条消息立即发送
			BatchTimeout:    time.Millisecond,
			MaxAttempts:     3, // 重试3次
			WriteBackoffMin: 100 * time.Millisecond,
			WriteBackoffMax: 1 * time.Second,
		},
	}
}

// Connect 连接到Kafka
func (k *KafkaQueue) Connect() error {
	// kafka-go的Writer会自动连接，这里只做验证
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 尝试写入一条测试消息验证连接
	testMsg := kafka.Message{
		Key:   []byte("test"),
		Value: []byte("connection test"),
	}
	
	err := k.writer.WriteMessages(ctx, testMsg)
	if err != nil {
		return err
	}

	k.log.Info("kafka connected", slog.String("topic", k.topic))
	return nil
}

// PublishAlert 发布告警消息
func (k *KafkaQueue) PublishAlert(alert model.Alert) error {
	alertJSON, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(alert.TaskID),
		Value: alertJSON,
		Time:  alert.CreatedAt,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = k.writer.WriteMessages(ctx, msg)
	if err != nil {
		return err
	}

	k.log.Debug("alert published to kafka",
		slog.Uint64("alert_id", uint64(alert.ID)),
		slog.String("task_id", alert.TaskID),
		slog.String("task_type", alert.TaskType))

	return nil
}

// Close 关闭Kafka连接
func (k *KafkaQueue) Close() error {
	if k.writer != nil {
		return k.writer.Close()
	}
	return nil
}

