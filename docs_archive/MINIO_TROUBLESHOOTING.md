# MinIO 连接问题排查与解决

## 📊 问题现状

### ✅ 已确认正常
1. ✅ MinIO服务运行正常（10.1.6.230:9000）
2. ✅ 使用mc工具可以正常连接、上传、下载
3. ✅ Bucket `images` 存在且可访问
4. ✅ 认证信息正确（admin/admin123）
5. ✅ 已上传测试文件成功

### ❌ 存在的问题
- ❌ yanying平台报告 "502 Bad Gateway" 错误
- ❌ AI分析模块"list object error"
- ❌ 抽帧模块启动失败

## 🔍 问题分析

从日志中看到：

```json
{"level":"warn","ts":"2025-10-16 14:11:00.016","msg":"list object error","module":"aianalysis","err":"502 Bad Gateway"}
{"level":"error","ts":"2025-10-16 14:10:26.418","msg":"frame extractor start failed","err":"502 Bad Gateway"}
```

**502 Bad Gateway** 错误通常意味着：
1. MinIO返回了非预期的响应
2. HTTP代理或负载均衡器问题
3. API版本不兼容
4. SSL/TLS配置问题

## 🔧 解决方案

### 方案1：修改MinIO配置（推荐）

编辑配置文件：

```bash
vi /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350/configs/config.toml
```

**关键配置检查：**

```toml
[frame_extractor.minio]
endpoint = '10.1.6.230:9000'  # 不要使用 http:// 前缀
bucket = 'images'              # bucket名称
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false                # 必须设置为false
base_path = ''                 # 基础路径，留空表示根路径
```

### 方案2：使用不同的MinIO Endpoint格式

尝试以下格式之一：

**选项A：IP和端口**
```toml
endpoint = '10.1.6.230:9000'
```

**选项B：带HTTP协议**
```toml
endpoint = 'http://10.1.6.230:9000'
```

**选项C：使用域名（如果有）**
```toml
endpoint = 'minio.example.com:9000'
```

### 方案3：检查MinIO API版本

MinIO有两种API签名版本：S3v2和S3v4。

修改配置，尝试指定API版本：

```toml
[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
# 添加region设置（某些MinIO版本需要）
region = 'us-east-1'
```

### 方案4：使用本地存储（临时方案）

如果MinIO问题难以解决，可以临时使用本地存储：

```toml
[frame_extractor]
enable = true
interval_ms = 1000
output_dir = './snapshots'
store = 'local'  # 改为local
# store = 'minio'  # 注释掉minio
```

### 方案5：重启yanying服务

修改配置后，必须重启服务：

```bash
# 停止服务
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350
./stop.sh

# 或者kill进程
pkill -f easydarwin

# 启动服务
./easydarwin &

# 查看日志
tail -f logs/20251016_08_00_00.log
```

## 🧪 测试步骤

### 1. 手动测试MinIO连接

使用我们的测试脚本：

```bash
cd /code/EasyDarwin
./test_minio.sh
```

### 2. 检查yanying日志

```bash
tail -f /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350/logs/20251016_08_00_00.log | grep -i minio
```

查找是否还有502错误。

### 3. 测试AI分析扫描

```bash
# 创建测试图片
/tmp/mc cp /tmp/test.jpg test-minio/images/人数统计/task_test/frame_001.jpg

# 等待10秒（扫描间隔）
# 查看日志是否有扫描记录
```

### 4. 查询API

```bash
# 查询抽帧配置
curl http://localhost:5066/api/v1/frame_extractor/config

# 查询已注册的AI服务
curl http://localhost:5066/api/v1/ai_analysis/services

# 查询告警
curl http://localhost:5066/api/v1/ai_analysis/alerts
```

## 🐛 常见错误及解决

### 错误1: "502 Bad Gateway"

**原因**：
- MinIO返回了非标准HTTP响应
- endpoint格式不正确
- SSL配置不匹配

**解决**：
1. 确保`use_ssl = false`
2. 确保endpoint不包含http://前缀
3. 尝试重启MinIO服务

### 错误2: "Access Denied"

**原因**：认证失败

**解决**：
```bash
# 检查MinIO用户
/tmp/mc admin user list test-minio

# 创建新用户（如果需要）
/tmp/mc admin user add test-minio newuser newpass123
```

### 错误3: "Bucket does not exist"

**原因**：bucket不存在

**解决**：
```bash
# 创建bucket
/tmp/mc mb test-minio/images

# 设置bucket策略（允许读写）
/tmp/mc policy set public test-minio/images
```

### 错误4: "Connection timeout"

**原因**：网络问题

**解决**：
```bash
# 测试网络连接
ping 10.1.6.230
telnet 10.1.6.230 9000

# 检查防火墙
sudo firewall-cmd --list-ports
sudo ufw status
```

## 💡 最佳实践

### 1. 生产环境配置

```toml
[frame_extractor.minio]
endpoint = 'minio-lb.internal:9000'  # 使用负载均衡地址
bucket = 'yanying-frames'            # 专用bucket
access_key = 'yanying-app'           # 专用用户
secret_key = 'strong_password_here'
use_ssl = true                       # 生产环境启用SSL
region = 'us-east-1'
```

### 2. MinIO服务配置

```bash
# 设置合理的bucket策略
/tmp/mc policy set download test-minio/images

# 设置bucket生命周期（自动清理旧图片）
/tmp/mc ilm add test-minio/images --expiry-days 7

# 启用版本控制
/tmp/mc version enable test-minio/images
```

### 3. 监控和告警

```bash
# 监控MinIO磁盘使用
/tmp/mc admin info test-minio

# 设置webhook告警
/tmp/mc admin config set test-minio notify_webhook:1 \
  endpoint="http://yanying:5066/api/v1/webhook/minio"
```

## 📞 获取帮助

### 查看详细日志

```bash
# yanying日志
tail -f /code/EasyDarwin/build/*/logs/20251016_08_00_00.log

# MinIO日志（如果使用Docker）
docker logs minio -f --tail=100
```

### 联系支持

如果问题仍未解决：

1. 收集日志文件
2. 记录配置信息
3. 提供MinIO版本信息
4. GitHub Issues: https://github.com/zhouyingchaoAI/easyAIServer/issues

## 🎯 快速修复（推荐尝试顺序）

### 1. 最简单的修复

```bash
# 1. 确保配置正确
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350/configs
grep -A 6 "\[frame_extractor.minio\]" config.toml

# 2. 重启服务
cd ..
pkill -f easydarwin
./easydarwin &

# 3. 等待30秒，查看日志
tail -n 50 logs/20251016_08_00_00.log | grep -i "minio\|502"
```

### 2. 如果还是失败

**切换到本地存储**：

```bash
# 修改配置
sed -i 's/store = .minio./store = '\''local'\''/' configs/config.toml

# 重启
pkill -f easydarwin
./easydarwin &
```

### 3. 长期解决方案

1. 升级MinIO到最新版本
2. 配置MinIO网关模式
3. 使用MinIO集群

## ✅ 当前可用的功能

即使MinIO连接有问题，以下功能仍然可用：

1. ✅ 流媒体服务（RTSP/RTMP/HLS等）
2. ✅ Web界面访问
3. ✅ AI服务注册和管理
4. ✅ 告警查看（SQLite数据库）
5. ✅ 抽帧到本地文件系统

MinIO主要用于：
- 存储抽取的视频帧
- AI分析模块扫描图片

如果使用本地存储模式，这些功能同样可以工作，只是图片存储在本地而不是对象存储中。

---

**最后更新**: 2024-10-16  
**版本**: 1.0

