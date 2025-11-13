# EasyDarwin 告警消息 Kafka 对接文档

## 概述

EasyDarwin 智能分析插件会将检测到的告警信息实时推送到 Kafka 消息队列，其他系统可以通过消费 Kafka 消息来获取告警信息。

## 配置信息

### Kafka 连接信息

- **Bootstrap Server**: `172.16.5.207:9092`（根据实际配置调整）
- **Topic**: `easyai.alerts`
- **消息格式**: JSON
- **消息 Key**: 任务ID（`task_id`）

### 配置说明

在 EasyDarwin 的 `config.toml` 中配置：

```toml
[ai_analysis]
enable = true
mq_type = 'kafka'
mq_address = '172.16.5.207:9092'  # Kafka 地址
mq_topic = 'easyai.alerts'         # 告警推送 topic
```

## 消息格式

### JSON Schema

```json
{
  "id": 123,                          // 告警ID（整数）
  "task_id": "任务ID",                // 任务标识（字符串）
  "task_type": "人数统计",            // 任务类型（字符串）
  "image_path": "alerts/任务ID/20251111-090000.jpg",  // 图片存储路径
  "image_url": "http://172.16.5.207:9000/images/alerts/任务ID/20251111-090000.jpg",  // 图片访问URL
  "algorithm_id": "yolo11x_head_detector_7901",      // 算法服务ID
  "algorithm_name": "YOLOv11x人头检测算法",          // 算法名称
  "result": "{\"person_count\": 2, \"objects\": []}", // 推理结果（JSON字符串）
  "confidence": 0.91,                 // 置信度（0.0-1.0）
  "detection_count": 2,               // 检测到的对象数量（整数）
  "inference_time_ms": 234,          // 推理耗时（毫秒）
  "created_at": "2025-11-11T09:00:00Z"  // 创建时间（ISO 8601格式，UTC时区）
}
```

### 字段说明

| 字段名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `id` | integer | 是 | 告警记录的唯一ID |
| `task_id` | string | 是 | 任务标识，对应抽帧任务的ID |
| `task_type` | string | 是 | 任务类型，如"人数统计"、"目标检测"等 |
| `image_path` | string | 是 | 告警图片在MinIO中的存储路径 |
| `image_url` | string | 是 | 告警图片的访问URL（预签名URL或公开URL） |
| `algorithm_id` | string | 是 | 执行推理的算法服务ID |
| `algorithm_name` | string | 是 | 算法服务的名称 |
| `result` | string | 是 | 推理结果，JSON格式字符串，内容根据算法类型而定 |
| `confidence` | float | 是 | 整体置信度，范围0.0-1.0 |
| `detection_count` | integer | 是 | 检测到的对象/实例数量 |
| `inference_time_ms` | integer | 是 | 算法推理耗时（毫秒） |
| `created_at` | string | 是 | 告警创建时间，ISO 8601格式（UTC时区） |

### 消息 Key

Kafka 消息的 Key 为任务ID（`task_id`），可用于消息分区和路由。

### 消息示例

```json
{
  "id": 12345,
  "task_id": "厕所10",
  "task_type": "人数统计",
  "image_path": "alerts/厕所10/20251112-143000.jpg",
  "image_url": "http://172.16.5.207:9000/images/alerts/厕所10/20251112-143000.jpg",
  "algorithm_id": "yolo11x_head_detector_7901",
  "algorithm_name": "YOLOv11x人头检测算法",
  "result": "{\"person_count\": 3, \"objects\": [{\"class\": \"person\", \"confidence\": 0.95, \"bbox\": [100, 200, 150, 300]}]}",
  "confidence": 0.92,
  "detection_count": 3,
  "inference_time_ms": 156,
  "created_at": "2025-11-12T14:30:00Z"
}
```

## 消费示例

### Python 示例（使用 kafka-python）

```python
#!/usr/bin/env python3
import json
from kafka import KafkaConsumer

# Kafka 配置
BOOTSTRAP_SERVERS = ['172.16.5.207:9092']
TOPIC = 'easyai.alerts'
GROUP_ID = 'alert_consumer_group'  # 消费者组ID

# 创建消费者
consumer = KafkaConsumer(
    TOPIC,
    bootstrap_servers=BOOTSTRAP_SERVERS,
    group_id=GROUP_ID,
    value_deserializer=lambda m: json.loads(m.decode('utf-8')),
    key_deserializer=lambda k: k.decode('utf-8') if k else None,
    auto_offset_reset='latest',  # 从最新消息开始消费，使用 'earliest' 从头开始
    enable_auto_commit=True,
    consumer_timeout_ms=1000  # 超时时间（毫秒）
)

print(f"开始消费 Kafka Topic: {TOPIC}")

try:
    for message in consumer:
        alert = message.value
        task_id = message.key
        
        print(f"\n收到告警消息:")
        print(f"  Key (Task ID): {task_id}")
        print(f"  Partition: {message.partition}")
        print(f"  Offset: {message.offset}")
        print(f"  告警ID: {alert['id']}")
        print(f"  任务类型: {alert['task_type']}")
        print(f"  检测数量: {alert['detection_count']}")
        print(f"  置信度: {alert['confidence']}")
        print(f"  图片URL: {alert['image_url']}")
        print(f"  创建时间: {alert['created_at']}")
        
        # 解析推理结果
        result = json.loads(alert['result'])
        print(f"  推理结果: {result}")
        
        # 在这里处理告警业务逻辑
        # process_alert(alert)
        
except KeyboardInterrupt:
    print("\n停止消费")
finally:
    consumer.close()
```

### Go 示例（使用 segmentio/kafka-go）

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/segmentio/kafka-go"
)

type Alert struct {
    ID              uint      `json:"id"`
    TaskID          string    `json:"task_id"`
    TaskType        string    `json:"task_type"`
    ImagePath       string    `json:"image_path"`
    ImageURL        string    `json:"image_url"`
    AlgorithmID     string    `json:"algorithm_id"`
    AlgorithmName   string    `json:"algorithm_name"`
    Result          string    `json:"result"`
    Confidence      float64   `json:"confidence"`
    DetectionCount  int       `json:"detection_count"`
    InferenceTimeMs int      `json:"inference_time_ms"`
    CreatedAt       time.Time `json:"created_at"`
}

func main() {
    broker := "172.16.5.207:9092"
    topic := "easyai.alerts"
    groupID := "alert_consumer_group"

    // 创建消费者
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:  []string{broker},
        Topic:    topic,
        GroupID:  groupID,
        MinBytes: 10e3, // 10KB
        MaxBytes: 10e6, // 10MB
    })
    defer reader.Close()

    fmt.Printf("开始消费 Kafka Topic: %s\n", topic)

    ctx := context.Background()
    for {
        msg, err := reader.ReadMessage(ctx)
        if err != nil {
            log.Printf("读取消息失败: %v", err)
            continue
        }

        var alert Alert
        if err := json.Unmarshal(msg.Value, &alert); err != nil {
            log.Printf("解析消息失败: %v", err)
            continue
        }

        fmt.Printf("\n收到告警消息:\n")
        fmt.Printf("  Key (Task ID): %s\n", string(msg.Key))
        fmt.Printf("  Partition: %d\n", msg.Partition)
        fmt.Printf("  Offset: %d\n", msg.Offset)
        fmt.Printf("  告警ID: %d\n", alert.ID)
        fmt.Printf("  任务类型: %s\n", alert.TaskType)
        fmt.Printf("  检测数量: %d\n", alert.DetectionCount)
        fmt.Printf("  置信度: %.2f\n", alert.Confidence)
        fmt.Printf("  图片URL: %s\n", alert.ImageURL)
        fmt.Printf("  创建时间: %s\n", alert.CreatedAt.Format(time.RFC3339))

        // 解析推理结果
        var result map[string]interface{}
        if err := json.Unmarshal([]byte(alert.Result), &result); err == nil {
            fmt.Printf("  推理结果: %+v\n", result)
        }

        // 在这里处理告警业务逻辑
        // processAlert(alert)
    }
}
```

### Java 示例（使用 Spring Kafka）

```java
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.support.KafkaHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.stereotype.Component;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Map;

@Component
public class AlertConsumer {
    
    private ObjectMapper objectMapper = new ObjectMapper();
    
    @KafkaListener(
        topics = "easyai.alerts",
        groupId = "alert_consumer_group",
        containerFactory = "kafkaListenerContainerFactory"
    )
    public void consumeAlert(
        @Payload String message,
        @Header(KafkaHeaders.RECEIVED_KEY) String taskId,
        @Header(KafkaHeaders.RECEIVED_PARTITION_ID) int partition,
        @Header(KafkaHeaders.OFFSET) long offset
    ) {
        try {
            Map<String, Object> alert = objectMapper.readValue(message, Map.class);
            
            System.out.println("\n收到告警消息:");
            System.out.println("  Key (Task ID): " + taskId);
            System.out.println("  Partition: " + partition);
            System.out.println("  Offset: " + offset);
            System.out.println("  告警ID: " + alert.get("id"));
            System.out.println("  任务类型: " + alert.get("task_type"));
            System.out.println("  检测数量: " + alert.get("detection_count"));
            System.out.println("  置信度: " + alert.get("confidence"));
            System.out.println("  图片URL: " + alert.get("image_url"));
            System.out.println("  创建时间: " + alert.get("created_at"));
            
            // 解析推理结果
            String resultStr = (String) alert.get("result");
            Map<String, Object> result = objectMapper.readValue(resultStr, Map.class);
            System.out.println("  推理结果: " + result);
            
            // 在这里处理告警业务逻辑
            // processAlert(alert);
            
        } catch (Exception e) {
            System.err.println("处理消息失败: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
```

## 注意事项

### 1. 消息大小限制

- Kafka 默认消息大小限制为 1MB
- 如果告警消息较大，需要调整 Kafka 配置：
  - `message.max.bytes`（broker 级别）
  - `max.message.bytes`（topic 级别）
  - `replica.fetch.max.bytes`（broker 级别）

### 2. 消费者组

- 使用不同的 `group_id` 可以实现多个消费者同时消费
- 同一个 `group_id` 的多个消费者会负载均衡消费消息

### 3. 消息顺序

- 同一 `task_id` 的消息会发送到同一个分区，保证顺序
- 不同 `task_id` 的消息可能在不同分区，不保证全局顺序

### 4. 消息可靠性

- EasyDarwin 使用 `RequiredAcks=RequireOne`，确保消息至少被 leader 确认
- 建议消费者使用 `enable_auto_commit=false` 手动提交 offset，确保消息处理完成后再提交

### 5. 图片访问

- `image_url` 字段包含图片的完整访问URL
- 如果使用 MinIO 预签名URL，URL 有时效性，需要及时访问
- 建议消费系统收到告警后立即下载图片保存

### 6. 推理结果格式

- `result` 字段是 JSON 字符串，需要二次解析
- 不同算法类型的 `result` 格式可能不同，需要根据 `algorithm_id` 或 `task_type` 进行适配

## 测试工具

### 使用 kafka-console-consumer 测试

```bash
/opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server 172.16.5.207:9092 \
  --topic easyai.alerts \
  --from-beginning \
  --property print.key=true \
  --property print.timestamp=true
```

### 使用 Python 脚本测试

参考本文档的 Python 示例代码，可以快速验证消息消费是否正常。

## 故障排查

### 1. 无法连接 Kafka

- 检查 Kafka 地址和端口是否正确
- 检查网络连通性：`telnet 172.16.5.207 9092`
- 检查 Kafka 服务是否正常运行

### 2. 无法消费消息

- 检查 Topic 是否存在：`kafka-topics.sh --list --bootstrap-server 172.16.5.207:9092`
- 检查消费者组状态：`kafka-consumer-groups.sh --bootstrap-server 172.16.5.207:9092 --group alert_consumer_group --describe`
- 检查 offset 位置：可能需要重置 offset 从头开始消费

### 3. 消息格式错误

- 确认消息是有效的 JSON 格式
- 检查字段类型是否匹配
- 查看 EasyDarwin 日志确认消息发送是否成功

## 联系支持

如有问题，请查看 EasyDarwin 日志文件或联系技术支持。

---

**文档版本**: v1.0  
**最后更新**: 2025-11-12

