# 抽帧插件故障排查指南

## 问题1：配置不持久化（重启后丢失）

### 症状
- UI修改MinIO配置后，重启服务配置恢复默认
- 添加的任务重启后消失
- 启停状态不保存

### 排查步骤

#### 1. 检查配置文件是否可写
```bash
# 查看配置文件权限
ls -la configs/config.toml

# 确保可写
chmod 644 configs/config.toml
```

#### 2. 查看配置保存日志
```bash
# 实时查看日志
tail -f logs/sugar.log | grep -E "updating config|config persisted|failed to persist"
```

**正常输出**：
```
updating config enable=true store=minio minio_endpoint=xxx minio_bucket=xxx
config persisted path=/path/to/configs/config.toml
```

**异常输出**：
```
failed to persist config path=... err=permission denied
```

#### 3. 验证配置文件内容
修改配置后立即检查：
```bash
cat configs/config.toml | grep -A 30 "\[frame_extractor\]"
```

应该看到最新的MinIO配置和任务列表。

#### 4. 测试完整流程
```bash
# 1. 启动服务
./build/easydarwin -conf ./configs

# 2. UI添加MinIO配置（endpoint/bucket/keys）并保存

# 3. 立即检查配置文件
cat configs/config.toml | grep -A 15 "\[frame_extractor.minio\]"

# 应该看到:
# [frame_extractor.minio]
# endpoint = 'minio.example.com:9000'
# bucket = 'snapshots'
# access_key = 'xxx'
# secret_key = 'xxx'
# use_ssl = false
# base_path = 'camera-frames'

# 4. 重启服务
pkill -f easydarwin
./build/easydarwin -conf ./configs

# 5. UI查看配置是否恢复
# 访问抽帧管理页，检查MinIO配置是否还在
```

### 解决方案

#### 方案1：检查配置目录参数
```bash
# 确保使用正确的配置目录启动
./build/easydarwin -conf /code/EasyDarwin/configs

# 或使用绝对路径
./build/easydarwin -conf $(pwd)/configs
```

#### 方案2：手动验证持久化
```bash
# 1. 备份当前配置
cp configs/config.toml configs/config.toml.bak

# 2. 通过API修改配置
curl -X POST http://localhost:10086/api/v1/frame_extractor/config \
  -H 'Content-Type: application/json' \
  -d '{
    "enable": true,
    "interval_ms": 2000,
    "store": "minio",
    "minio": {
      "endpoint": "test.minio.com:9000",
      "bucket": "test-bucket",
      "access_key": "test",
      "secret_key": "test",
      "use_ssl": false,
      "base_path": "test"
    }
  }'

# 3. 对比配置文件
diff configs/config.toml.bak configs/config.toml
```

#### 方案3：权限问题
```bash
# 检查程序运行用户
ps aux | grep easydarwin

# 确保配置目录对该用户可写
chown -R <user>:<group> configs/
chmod 755 configs
chmod 644 configs/config.toml
```

---

## 问题2：图片没有保存

### 症状
- 日志显示"starting ffmpeg"
- 但snapshots目录为空或没有对应任务ID子目录

### 排查步骤

#### 1. 检查FFmpeg是否正常工作
```bash
# 测试FFmpeg是否可执行（优先使用系统 PATH，或仓库内的 deploy/ffmpeg）
ffmpeg -version || ./deploy/ffmpeg -version

# 手动测试抽帧命令（从日志复制完整命令）
ffmpeg -y -rtsp_transport tcp -stimeout 5000000 \
  -i "rtsp://user:pass@ip:554/..." \
  -vf fps=1/1.0 -f image2 -strftime 1 \
  "/path/to/snapshots/cam1/%Y%m%d-%H%M%S.jpg"
```

#### 2. 查看详细日志
```bash
tail -f logs/sugar.log | grep -E "starting ffmpeg|snapshot progress|ffmpeg exited"
```

**正常输出**（每10秒）：
```
starting ffmpeg task=cam1 task_output_path=cam1 full_output_dir=/path/to/snapshots/cam1
snapshot progress task=cam1 total_files=5 output_dir=/path/to/snapshots/cam1
snapshot progress task=cam1 total_files=10 output_dir=/path/to/snapshots/cam1
```

**异常输出**：
```
ffmpeg exited task=cam1 err=exit status 1 stderr=Connection refused
```

#### 3. 检查目录权限
```bash
# 检查snapshots目录
ls -la snapshots/

# 检查任务子目录
ls -la snapshots/cam1/

# 确保可写
chmod 755 snapshots
chmod 755 snapshots/cam1
```

#### 4. 验证RTSP连接
```bash
# 测试RTSP地址
ffmpeg -rtsp_transport tcp -i "rtsp://user:pass@ip:554/..." -frames:v 1 -f mjpeg test.jpg

# 成功会生成test.jpg
ls -lh test.jpg
```

---

## 问题3：快照列表为空

### 症状
- 图片已保存到snapshots目录
- UI快照列表显示"暂无快照数据"

### 排查步骤

#### 1. 检查静态文件服务
```bash
# 直接访问图片URL
curl -I http://localhost:10086/snapshots/cam1/20250114-153045.jpg

# 应该返回200 OK
```

#### 2. 查看API返回
```bash
# 调用列表API
curl http://localhost:10086/api/v1/frame_extractor/snapshots/cam1

# 应该返回:
# {"items":[...],"total":X}
```

#### 3. 检查日志
```bash
tail -f logs/sugar.log | grep "listing snapshots"
```

**正常输出**：
```
listing snapshots task=cam1 base_dir=/path/to/snapshots scan_dir=/path/to/snapshots/cam1
```

#### 4. 验证路径配置
```bash
# 检查output_path是否正确
curl http://localhost:10086/api/v1/frame_extractor/tasks | jq '.items[] | {id, output_path}'

# 应该返回:
# {"id":"cam1","output_path":"cam1"}
```

---

## 问题4：MinIO上传失败

### 症状
- 本地模式正常
- MinIO模式无图片

### 排查步骤

#### 1. 检查MinIO连接
```bash
# 测试MinIO端点
telnet minio.example.com 9000

# 或使用mc客户端
mc alias set myminio http://minio.example.com:9000 <access_key> <secret_key>
mc ls myminio/
```

#### 2. 查看上传日志
```bash
tail -f logs/sugar.log | grep -E "minio upload|creating minio|uploaded snapshot"
```

**正常输出**：
```
creating minio bucket bucket=snapshots
creating minio path task=cam1 key=camera-frames/cam1/.keep
uploaded snapshot task=cam1 key=camera-frames/cam1/20250114-153045.jpg size=128450
```

**异常输出**：
```
minio upload failed task=cam1 key=... err=Access Denied
```

#### 3. 验证Bucket权限
```bash
# 使用mc检查bucket策略
mc admin policy list myminio
mc admin user list myminio

# 确保用户有读写权限
```

---

## 使用新功能：批量删除快照

### 网格视图批量删除

1. 访问：`http://localhost:10086/#/frame-extractor/gallery`
2. 选择任务
3. 点击左上角"全选"复选框（或逐个勾选图片）
4. 点击右上角"删除选中 (N)"按钮
5. 确认删除

### 列表视图批量删除

1. 切换到列表视图
2. 使用表格左侧复选框选择多个图片
3. 点击"删除选中"按钮

### API批量删除

```bash
curl -X POST http://localhost:10086/api/v1/frame_extractor/snapshots/cam1/batch_delete \
  -H 'Content-Type: application/json' \
  -d '{
    "paths": [
      "cam1/20250114-153001.jpg",
      "cam1/20250114-153002.jpg",
      "cam1/20250114-153003.jpg"
    ]
  }'
```

**响应**：
```json
{"ok": true, "deleted": 3}
```

---

## 完整测试流程

### 测试1：配置持久化

```bash
# 1. 启动服务
./build/easydarwin -conf ./configs

# 2. 保存配置（UI或API）
curl -X POST http://localhost:10086/api/v1/frame_extractor/config \
  -H 'Content-Type: application/json' \
  -d '{
    "enable": true,
    "store": "local",
    "interval_ms": 1500,
    "output_dir": "./snapshots"
  }'

# 3. 验证文件已更新
grep "interval_ms" configs/config.toml
# 应该显示: interval_ms = 1500

# 4. 重启服务
pkill easydarwin && ./build/easydarwin -conf ./configs

# 5. 读取配置
curl http://localhost:10086/api/v1/frame_extractor/config | jq '.interval_ms'
# 应该返回: 1500
```

### 测试2：任务持久化

```bash
# 1. 添加任务
curl -X POST http://localhost:10086/api/v1/frame_extractor/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "test_cam",
    "rtsp_url": "rtsp://test",
    "interval_ms": 2000,
    "output_path": "test_cam"
  }'

# 2. 验证配置文件
cat configs/config.toml | grep -A 6 "id = 'test_cam'"
# 应该看到完整任务定义

# 3. 重启
pkill easydarwin && ./build/easydarwin -conf ./configs

# 4. 查询任务
curl http://localhost:10086/api/v1/frame_extractor/tasks | jq '.items[] | select(.id=="test_cam")'
# 应该返回任务信息
```

---

## 常见错误及解决

### 错误1：permission denied

**原因**：配置文件无写权限  
**解决**：
```bash
chmod 644 configs/config.toml
chown $USER configs/config.toml
```

### 错误2：no such file or directory

**原因**：配置文件路径错误  
**解决**：
```bash
# 使用绝对路径启动
./build/easydarwin -conf /absolute/path/to/configs

# 或相对当前目录
cd /code/EasyDarwin
./build/easydarwin -conf ./configs
```

### 错误3：failed to persist config

**原因**：磁盘空间不足或文件被锁定  
**解决**：
```bash
# 检查磁盘空间
df -h

# 检查文件是否被占用
lsof configs/config.toml

# 重新启动服务
```

---

## 调试技巧

### 启用Debug日志

编辑 `configs/config.toml`：
```toml
[baselog]
level = 'debug'  # 从 'info' 改为 'debug'
```

重启后会看到更详细的日志：
```
DEBUG ffmpeg command cmd="..."
DEBUG listing snapshots task=cam1 base_dir=... scan_dir=...
DEBUG uploaded snapshot task=cam1 key=... size=...
```

### 监控配置文件变化

```bash
# 监控配置文件修改
watch -n 1 'ls -l configs/config.toml'

# 或使用inotify
inotifywait -m configs/config.toml
```

### 测试API响应

```bash
# 保存配置后立即读取
curl -X POST http://localhost:10086/api/v1/frame_extractor/config -d '{...}' && \
curl http://localhost:10086/api/v1/frame_extractor/config | jq '.'
```

---

## 性能优化建议

### 批量删除优化
- 一次删除建议 ≤ 100个文件
- MinIO模式批量删除会逐个调用API
- 大量文件建议分批删除

### 配置保存优化
- 避免频繁修改配置（每次都会重写文件）
- 批量修改后一次性保存
- 定期备份config.toml

---

## 需要帮助？

如果问题仍未解决，请提供以下信息：

1. 完整日志（最后100行）：
   ```bash
   tail -100 logs/sugar.log > debug.log
   ```

2. 配置文件：
   ```bash
   cat configs/config.toml | grep -A 50 "\[frame_extractor\]" > config_dump.txt
   ```

3. 任务状态：
   ```bash
   curl http://localhost:10086/api/v1/frame_extractor/tasks > tasks.json
   ```

4. 错误复现步骤

5. 运行环境：
   - OS版本
   - Go版本
   - 启动命令
   - 配置目录路径

