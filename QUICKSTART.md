# EasyDarwin AI 分析快速启动指南

## 问题说明

### 问题1：算法服务加载失败
**原因**：AI Analysis 插件未正确启动，因为配置问题。

### 问题2：MinIO 配置不持久化
**原因**：从 Web UI 保存 MinIO 配置时，会重置 `frame_extractor.enable = false`。

---

## 解决方案

### 方法1：使用自动化脚本（推荐）

#### 1. 完整重启并启用插件

```bash
cd /code/EasyDarwin

# 停止所有进程
pkill -9 easydarwin

# 确保配置正确
cat > /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/fix_config.sh << 'EOF'
#!/bin/bash
CONFIG_FILE="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/config.toml"

# 启用 Frame Extractor
sed -i '/^\[frame_extractor\]/,/^\[/ s/^enable = false/enable = true/' "$CONFIG_FILE"

# 设置存储为 minio
sed -i '/^\[frame_extractor\]/,/^\[/ s/^store = .*/store = '\''minio'\''/' "$CONFIG_FILE"

# 启用 AI Analysis
sed -i '/^\[ai_analysis\]/,/^\[/ s/^enable = false/enable = true/' "$CONFIG_FILE"

# 修复 mq_type
sed -i '/^\[ai_analysis\]/,/^$/ s/^mq_type = .*/mq_type = '\''kafka'\''/' "$CONFIG_FILE"

# 清空 mq_address
sed -i '/^\[ai_analysis\]/,/^$/ s/^mq_address = .*/mq_address = '\'''\''/' "$CONFIG_FILE"

echo "✅ 配置已修复"
EOF

chmod +x /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/fix_config.sh
/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/fix_config.sh

# 启动服务
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136
./easydarwin &

# 等待启动
sleep 5

# 查看日志验证
tail -30 logs/sugar.log | grep -E "AI analysis|minio"
```

#### 2. 启动算法服务

```bash
cd /code/EasyDarwin
python3 examples/algorithm_service.py \
  --service-id yolo11x_head_detector \
  --name "YOLO11X头部检测" \
  --task-types 人数统计 \
  --port 8000 \
  --easydarwin http://10.1.6.230:5066
```

---

### 方法2：手动配置

#### 1. 编辑配置文件

```bash
vim /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/config.toml
```

找到并修改：

```toml
[frame_extractor]
enable = true  # ← 改为 true
interval_ms = 1000
output_dir = './snapshots'
store = 'minio'  # ← 确保是 minio

[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
base_path = ''

[ai_analysis]
enable = true  # ← 改为 true
scan_interval_sec = 10
mq_type = 'kafka'  # ← 不能是空字符串 ''
mq_address = ''  # ← 如果没有 Kafka，保持为空
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 5
```

#### 2. 重启服务

```bash
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136
pkill -9 easydarwin
./easydarwin &
```

#### 3. 验证启动

```bash
# 查看日志
tail -f logs/sugar.log

# 应该看到：
# ✅ "minio client initialized"
# ✅ "AI analysis plugin started successfully"

# 不应该看到：
# ❌ "AI analysis start failed"
# ❌ "unknown mq_type"
```

---

## 重要提示

### ⚠️ 使用 Web UI 的注意事项

**如果从 Web UI 修改 MinIO 配置：**

1. 保存后，`frame_extractor.enable` 会被重置为 `false`
2. **必须重启服务并重新启用**

**避免问题的方法：**
- 方案A：只在启动时修改配置文件，不使用 Web UI
- 方案B：使用 Web UI 后，立即执行修复脚本

```bash
# 从 Web UI 保存配置后，立即执行：
sed -i '/^\[frame_extractor\]/,/^\[/ s/^enable = false/enable = true/' \
  /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136/configs/config.toml

# 重启服务
pkill -9 easydarwin
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136 && ./easydarwin &
```

---

## 验证清单

启动后检查：

- [ ] MinIO 服务运行正常：`curl -I http://10.1.6.230:9000`
- [ ] Bucket 存在：`mc ls myminio/images`
- [ ] EasyDarwin 运行：`ps aux | grep easydarwin`
- [ ] AI 插件启动：`tail logs/sugar.log | grep "AI analysis"`
- [ ] 算法服务注册：`curl http://10.1.6.230:5066/api/v1/ai_analysis/services`
- [ ] 抽帧任务运行：检查 MinIO 中是否有新图片

---

## 访问地址

- **主界面**: http://10.1.6.230:5066
- **算法服务**: http://10.1.6.230:5066/#/ai-services
- **告警列表**: http://10.1.6.230:5066/#/alerts
- **抽帧管理**: http://10.1.6.230:5066/#/frame-extractor

---

## 故障排查

### 如果算法服务显示"暂无"

1. 检查 AI Analysis 是否启动：
   ```bash
   curl http://10.1.6.230:5066/api/v1/ai_analysis/services
   ```

2. 如果返回 500 错误，查看日志：
   ```bash
   grep "AI analysis" logs/sugar.log
   ```

3. 常见错误：
   - `AI analysis requires frame_extractor.store = 'minio'` → 修改 store 配置
   - `unknown mq_type: ""` → 修改 mq_type 为 'kafka'
   - `registry not ready` → AI Analysis 未启动

### 如果 MinIO 连接失败

```bash
# 测试 MinIO
curl -I http://10.1.6.230:9000

# 测试 bucket
mc ls myminio/images

# 如果 bucket 不存在
mc mb myminio/images
```

---

## 完整工作流程

```
1. RTSP 流 → 2. 抽帧 → 3. 上传 MinIO → 4. AI 扫描 → 5. 算法推理 → 6. 存储告警
                  ↓                       ↓                ↓
           (每4秒)              (images/人数统计/...)  (yolo11x)       (SQLite)
```


