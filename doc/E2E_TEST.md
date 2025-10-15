# 端到端测试指南

本文档提供EasyDarwin完整功能（直播→抽帧→AI分析→告警）的端到端测试流程。

---

## 测试环境准备

### 必需服务

```bash
# 1. MinIO
docker run -d -p 9000:9000 -p 9001:9001 --name minio \
  -e "MINIO_ROOT_USER=admin" \
  -e "MINIO_ROOT_PASSWORD=admin123" \
  minio/minio server /data --console-address ":9001"

# 2. Kafka（可选，用于告警推送）
docker run -d -p 2181:2181 --name zookeeper wurstmeister/zookeeper
docker run -d -p 9092:9092 --name kafka \
  --link zookeeper \
  -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  wurstmeister/kafka

# 3. 测试RTSP流（使用FFmpeg生成）
ffmpeg -re -stream_loop -1 -i test_video.mp4 \
  -c copy -f rtsp rtsp://localhost:8554/test_stream &
```

---

## 完整测试流程

### 步骤1：配置EasyDarwin

编辑 `configs/config.toml`：

```toml
# 启用Frame Extractor
[frame_extractor]
enable = false  # 先不启用，通过API启用
store = 'minio'
task_types = ['人数统计', '人员跌倒', '吸烟检测']

[frame_extractor.minio]
endpoint = 'localhost:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
base_path = ''

# 启用AI Analysis
[ai_analysis]
enable = true
scan_interval_sec = 5  # 测试时缩短为5秒
mq_type = 'kafka'
mq_address = 'localhost:9092'
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 10
```

### 步骤2：启动EasyDarwin

```bash
cd /code/EasyDarwin

# 编译
go build -o server ./cmd/server

# 启动
./server -conf ./configs

# 验证启动
curl http://localhost:5066/api/v1/version
```

### 步骤3：配置直播流

```bash
# 添加拉流（如果没有）
curl -X POST http://localhost:5066/api/v1/live/pull \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试摄像头",
    "url": "rtsp://source_camera:554/stream"
  }'

# 获取流ID
STREAM_ID=$(curl -s http://localhost:5066/api/v1/live | jq '.items[0].id')
echo "Stream ID: $STREAM_ID"
```

### 步骤4：创建抽帧任务

```bash
# 通过API创建
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "e2e_test",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 3000
  }'

# 启动任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks/e2e_test/start

# 验证任务状态
curl http://localhost:5066/api/v1/frame_extractor/tasks/e2e_test/status
```

### 步骤5：启动算法服务

```bash
cd /code/EasyDarwin/examples

# 启动算法服务
python3 algorithm_service.py \
  --service-id e2e_test_algo \
  --name "E2E测试算法" \
  --task-types 人数统计 \
  --port 8000 \
  --easydarwin http://localhost:5066 &

# 等待注册
sleep 2

# 验证注册
curl -s http://localhost:5066/api/v1/ai_analysis/services | jq
```

### 步骤6：等待并验证

```bash
echo "等待完整流程..."
echo "  - 抽帧：3秒一张"
echo "  - 扫描：5秒一次"
echo "  - 推理：自动调度"
echo ""
echo "等待30秒..."
sleep 30

# 检查MinIO中的图片
echo "MinIO图片："
# 访问 http://localhost:9001 查看bucket: images

# 检查告警
echo "告警数量："
curl -s http://localhost:5066/api/v1/alerts | jq '.total'

echo "最新告警："
curl -s http://localhost:5066/api/v1/alerts | jq '.items[0] | {task_id, task_type, algorithm_name, confidence, created_at}'
```

### 步骤7：查看UI

访问以下页面验证：

1. **直播管理**: `http://localhost:5066/#/live`
   - 验证直播流在线

2. **抽帧管理**: `http://localhost:5066/#/frame-extractor`
   - 验证任务状态为"运行中"

3. **抽帧结果**: `http://localhost:5066/#/frame-extractor/gallery`
   - 验证有抽帧图片

4. **智能告警**: `http://localhost:5066/#/alerts`
   - 验证有告警记录
   - 点击"查看"预览图片和推理结果

5. **算法服务**: `http://localhost:5066/#/ai-services`
   - 验证算法服务状态为"正常"

### 步骤8：Kafka消费验证（可选）

```bash
# 终端1：消费告警消息
kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic easydarwin.alerts \
  --from-beginning

# 应该看到JSON格式的告警消息
```

---

## 自动化测试脚本

```bash
#!/bin/bash
# e2e_test.sh - 端到端自动化测试

set -e

echo "=== EasyDarwin E2E测试 ==="
echo ""

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 1. 检查服务
echo "1. 检查必需服务..."
curl -sf http://localhost:5066/api/v1/version > /dev/null || {
  echo -e "${RED}✗ EasyDarwin未运行${NC}"
  exit 1
}
echo -e "${GREEN}✓ EasyDarwin运行中${NC}"

curl -sf http://localhost:9000/minio/health/live > /dev/null || {
  echo -e "${RED}✗ MinIO未运行${NC}"
  exit 1
}
echo -e "${GREEN}✓ MinIO运行中${NC}"

# 2. 启动算法服务
echo ""
echo "2. 启动算法服务..."
cd /code/EasyDarwin/examples
python3 algorithm_service.py \
  --service-id e2e_test \
  --task-types 人数统计 \
  --port 8000 > /tmp/algo.log 2>&1 &
ALGO_PID=$!
sleep 3

# 验证注册
SERVICE_COUNT=$(curl -s http://localhost:5066/api/v1/ai_analysis/services | jq '.total')
if [ "$SERVICE_COUNT" -gt "0" ]; then
  echo -e "${GREEN}✓ 算法服务注册成功 (count=$SERVICE_COUNT)${NC}"
else
  echo -e "${RED}✗ 算法服务注册失败${NC}"
  kill $ALGO_PID
  exit 1
fi

# 3. 创建抽帧任务
echo ""
echo "3. 创建抽帧任务..."
curl -sS -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "e2e_test",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://localhost:15544/live/stream_1",
    "interval_ms": 2000
  }' | jq '.ok'
echo -e "${GREEN}✓ 抽帧任务创建成功${NC}"

# 4. 等待流程
echo ""
echo "4. 等待完整流程（抽帧→扫描→推理→保存）..."
for i in {1..6}; do
  echo "  等待... ${i}/6 (5秒)"
  sleep 5
  
  # 检查告警
  ALERT_COUNT=$(curl -s http://localhost:5066/api/v1/alerts?task_id=e2e_test | jq '.total')
  if [ "$ALERT_COUNT" -gt "0" ]; then
    echo -e "${GREEN}✓ 已生成 $ALERT_COUNT 条告警！${NC}"
    break
  fi
done

# 5. 验证结果
echo ""
echo "5. 验证结果..."

FINAL_ALERT_COUNT=$(curl -s http://localhost:5066/api/v1/alerts?task_id=e2e_test | jq '.total')
if [ "$FINAL_ALERT_COUNT" -gt "0" ]; then
  echo -e "${GREEN}✓ E2E测试成功！${NC}"
  echo ""
  echo "最新告警："
  curl -s http://localhost:5066/api/v1/alerts?task_id=e2e_test | \
    jq '.items[0] | {id, task_id, task_type, algorithm_name, confidence, inference_time_ms}'
else
  echo -e "${RED}✗ E2E测试失败：未生成告警${NC}"
  echo "请检查日志："
  echo "  tail -f /code/EasyDarwin/logs/sugar.log"
fi

# 6. 清理
echo ""
echo "6. 清理测试数据..."
curl -sS -X DELETE http://localhost:5066/api/v1/frame_extractor/tasks/e2e_test
kill $ALGO_PID
echo -e "${GREEN}✓ 清理完成${NC}"

echo ""
echo "=== 测试完成 ==="
echo "UI访问："
echo "  告警列表: http://localhost:5066/#/alerts"
echo "  算法服务: http://localhost:5066/#/ai-services"
```

保存为 `tests/e2e_test.sh` 并运行：

```bash
chmod +x tests/e2e_test.sh
./tests/e2e_test.sh
```

---

## 性能测试

### 负载测试

```bash
# 创建10个任务（不同任务ID）
for i in {1..10}; do
  curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"load_test_$i\",
      \"task_type\": \"人数统计\",
      \"rtsp_url\": \"rtsp://localhost:15544/live/stream_$i\",
      \"interval_ms\": 2000
    }"
done

# 监控资源
top -p $(pgrep -f easydarwin)

# 监控告警生成速率
watch -n 1 'curl -s http://localhost:5066/api/v1/alerts | jq .total'
```

### 并发测试

```bash
# 启动多个算法服务
for port in 8001 8002 8003; do
  python3 examples/algorithm_service.py \
    --service-id algo_$port \
    --task-types 人数统计 \
    --port $port &
done

# 验证并发推理
# 每张图片会被3个算法同时处理
# 查看告警数量应该是图片数的3倍
```

---

## 故障注入测试

### 测试1：算法服务崩溃

```bash
# 1. 启动算法服务
python3 examples/algorithm_service.py --port 8000 &
ALGO_PID=$!

# 2. 创建任务并等待几张图片
# ...

# 3. 杀死算法服务
kill $ALGO_PID

# 4. 观察：
# - 90秒后服务会被标记为失联
# - 新图片不再调用该算法
# - 已有的告警仍然保留
```

### 测试2：MinIO断开

```bash
# 1. 停止MinIO
docker stop minio

# 2. 观察：
# - 抽帧任务报错
# - AI分析扫描失败
# - 告警不再生成

# 3. 恢复MinIO
docker start minio

# 4. 验证：
# - 抽帧任务自动恢复
# - AI分析继续扫描
# - 告警正常生成
```

### 测试3：网络延迟

```bash
# 模拟算法服务慢响应（修改algorithm_service.py）
def do_POST(self):
    time.sleep(2)  # 模拟2秒延迟
    # ...

# 观察：
# - 推理任务排队
# - 并发控制生效
# - 超时后失败
```

---

## 回归测试清单

### 功能测试

- [ ] 直播流拉取和转发
- [ ] 获取直播流播放地址
- [ ] 创建抽帧任务（带任务类型）
- [ ] 图片保存到MinIO（任务类型/任务ID目录）
- [ ] 算法服务注册
- [ ] 算法服务心跳
- [ ] MinIO扫描新图片
- [ ] 推理调度（类型匹配）
- [ ] 告警保存到数据库
- [ ] 告警推送到Kafka
- [ ] 前端告警列表展示
- [ ] 前端告警详情查看
- [ ] 前端算法服务监控
- [ ] 删除告警
- [ ] 算法服务注销

### API测试

```bash
# 1. 获取任务类型
curl http://localhost:5066/api/v1/frame_extractor/task_types

# 2. 注册算法
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{"service_id":"test","name":"test","task_types":["人数统计"],"endpoint":"http://localhost:8000/infer","version":"1.0"}'

# 3. 查询服务
curl http://localhost:5066/api/v1/ai_analysis/services

# 4. 心跳
curl -X POST http://localhost:5066/api/v1/ai_analysis/heartbeat/test

# 5. 查询告警
curl http://localhost:5066/api/v1/alerts

# 6. 删除告警
curl -X DELETE http://localhost:5066/api/v1/alerts/1

# 7. 注销算法
curl -X DELETE http://localhost:5066/api/v1/ai_analysis/unregister/test
```

### UI测试

访问所有页面并验证：

1. `/` - 主页
2. `/live` - 直播管理
3. `/frame-extractor` - 抽帧管理
4. `/frame-extractor/gallery` - 抽帧结果
5. `/alerts` - 智能告警（新）
6. `/ai-services` - 算法服务（新）

### 性能测试

```bash
# 1. CPU使用率
top -p $(pgrep -f easydarwin)

# 2. 内存使用
ps aux | grep easydarwin | awk '{print $6}'

# 3. 告警生成速率
watch -n 1 'curl -s http://localhost:5066/api/v1/alerts | jq .total'

# 4. Kafka消息速率
kafka-run-class.sh kafka.tools.GetOffsetShell \
  --broker-list localhost:9092 \
  --topic easydarwin.alerts
```

---

## 预期结果

### 成功标准

✅ **直播流**：在线且可播放  
✅ **抽帧任务**：运行中，MinIO中有图片  
✅ **算法服务**：注册成功，心跳正常  
✅ **MinIO扫描**：日志显示"found new images"  
✅ **推理调度**：日志显示"inference completed"  
✅ **告警保存**：数据库中有记录  
✅ **Kafka推送**：消费者收到消息  
✅ **UI展示**：所有页面正常显示数据  

### 性能指标

- 抽帧任务（3秒间隔）：约20张图片/分钟
- AI扫描（5秒间隔）：每次扫描找到4-5张新图片
- 推理耗时：<100ms/张（模拟）
- 端到端延迟：<15秒（抽帧→推理→告警）
- 内存占用：<200MB（EasyDarwin + AI插件）

---

## 清理测试数据

```bash
# 1. 停止算法服务
pkill -f algorithm_service.py

# 2. 删除抽帧任务
curl -X DELETE http://localhost:5066/api/v1/frame_extractor/tasks/e2e_test

# 3. 清理MinIO数据
# 访问 http://localhost:9001 手动删除bucket中的文件

# 4. 清理数据库告警
sqlite3 configs/data.db "DELETE FROM alerts WHERE task_id LIKE 'e2e_%' OR task_id = 'test%';"

# 5. 重启EasyDarwin
pkill -f easydarwin
./server -conf ./configs
```

---

## 常见测试问题

### Q: 30秒后仍无告警？

**排查步骤**:
```bash
# 1. 检查抽帧是否正常
tail -f logs/sugar.log | grep "frame extractor"
# 应该看到：snapshot saved

# 2. 检查MinIO中是否有图片
# 访问 http://localhost:9001
# 查看bucket: images/人数统计/e2e_test/

# 3. 检查AI扫描
tail -f logs/sugar.log | grep "found new images"
# 应该看到：found new images count=N

# 4. 检查推理调用
tail -f logs/sugar.log | grep "scheduling inference"
# 应该看到：scheduling inference image=...

# 5. 检查算法服务日志
cat /tmp/algo.log
# 应该看到：收到推理请求
```

### Q: Kafka推送失败？

**解决方案**:
- Kafka可以留空，不影响告警保存到数据库
- 如果需要Kafka，确保：
  - Kafka运行中: `nc -zv localhost 9092`
  - Topic存在或自动创建
  - 网络连通

### Q: 算法服务心跳超时？

**原因**:
- Python进程意外退出
- 网络问题
- EasyDarwin重启（注册信息丢失）

**解决**:
- 重新启动算法服务（会自动重新注册）
- 查看算法服务日志
- 确保服务ID唯一

---

## 总结

端到端测试验证了完整的AI视频分析流程：

```
RTSP流 → EasyDarwin直播 → 抽帧插件 → MinIO存储 → 
AI扫描 → 算法服务推理 → 告警保存 → Kafka推送 → 
前端展示/外部系统消费
```

所有组件协同工作，提供完整的智能视频分析解决方案！

