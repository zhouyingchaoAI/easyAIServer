# 抽帧插件使用文档

## 概述

EasyDarwin 抽帧插件是一个强大的视频流抽帧工具，支持从RTSP流中按指定间隔提取关键帧，并保存到本地文件系统或MinIO对象存储。

## 核心功能

### ✨ 主要特性

- 🎥 **RTSP流抽帧**：持续拉取RTSP视频流，按间隔提取JPEG图片
- 💾 **双存储支持**：支持本地文件系统和MinIO对象存储
- 🔄 **自动重连**：网络断开自动重连，指数退避策略
- 🎛️ **UI管理**：Web界面可视化配置和管理
- 💪 **配置持久化**：任务配置自动保存到config.toml
- 🗂️ **智能路径管理**：MinIO模式下自动创建/删除bucket和子路径

---

## 快速开始

### 1. 启用插件

编辑 `configs/config.toml`：

```toml
[frame_extractor]
enable = true
interval_ms = 1000
output_dir = './snapshots'
store = 'local'  # 或 'minio'
```

或使用 Makefile：
```bash
make fx-enable
```

### 2. 启动服务

```bash
# 构建
make build/linux

# 运行
./build/easydarwin_linux_amd64 -conf ./configs
```

### 3. 访问管理界面

浏览器打开：`http://<服务器IP>:10086/#/frame-extractor`

---

## 配置说明

### 本地存储模式

```toml
[frame_extractor]
enable = true
interval_ms = 1000        # 默认抽帧间隔（毫秒）
output_dir = './snapshots' # 本地输出根目录
store = 'local'

[[frame_extractor.tasks]]
id = 'cam1'
rtsp_url = 'rtsp://admin:password@192.168.1.100:554/stream'
interval_ms = 1000        # 任务级间隔，覆盖全局
output_path = 'cam1'      # 输出子路径
```

**输出路径**：`output_dir/output_path/YYYYMMDD-HHMMSS.jpg`  
**示例**：`./snapshots/cam1/20250114-153045.jpg`

---

### MinIO对象存储模式

```toml
[frame_extractor]
enable = true
interval_ms = 1000
store = 'minio'

[frame_extractor.minio]
endpoint = 'minio.example.com:9000'  # MinIO地址
bucket = 'snapshots'                  # Bucket名称（不存在会自动创建）
access_key = 'minioadmin'
secret_key = 'minioadmin'
use_ssl = false                       # 是否使用HTTPS
base_path = 'camera-frames'           # 可选，存储桶内前缀

[[frame_extractor.tasks]]
id = 'cam1'
rtsp_url = 'rtsp://admin:password@192.168.1.100:554/stream'
interval_ms = 2000
output_path = 'cam1'
```

**MinIO路径**：`<bucket>/<base_path>/<output_path>/YYYYMMDD-HHMMSS.jpg`  
**示例**：`snapshots/camera-frames/cam1/20250114-153045.jpg`

**自动管理**：
- ✅ 添加任务时自动创建子路径（上传.keep文件）
- ✅ 删除任务时自动清理对应路径下所有文件
- ✅ Bucket不存在时自动创建

---

## UI管理界面

### 存储配置区域

#### 字段说明

| 字段 | 说明 | 示例 |
|------|------|------|
| 存储类型 | local（本地）或 minio（对象存储） | minio |
| 默认抽帧间隔 | 全局默认值，毫秒 | 1000 |
| 启用状态 | 插件总开关 | 已启用 |

#### MinIO配置（仅当存储类型=minio）

| 字段 | 说明 | 必填 |
|------|------|------|
| Endpoint | MinIO服务地址 | ✅ |
| Bucket | 存储桶名称（自动创建） | ✅ |
| Access Key | 访问密钥 | ✅ |
| Secret Key | 私密密钥 | ✅ |
| Base Path | 桶内前缀路径 | ❌ |
| 使用SSL | 是否HTTPS连接 | ❌ |

### 任务管理区域

#### 添加任务

| 字段 | 说明 | 示例 |
|------|------|------|
| 任务ID | 唯一标识，用作MinIO子路径 | cam1 |
| RTSP地址 | 完整RTSP URL | rtsp://user:pass@ip:554/stream |
| 间隔(ms) | 抽帧间隔，覆盖全局默认 | 1000 |
| 输出路径 | 存储子路径 | cam1 |

#### 任务列表

- 📊 **表格展示**：ID、RTSP地址、间隔、输出路径
- ✏️ **编辑功能**：点击编辑按钮快速修改
- 🗑️ **删除确认**：MinIO模式提示会删除所有文件

---

## API接口

### 获取配置

```bash
GET /api/v1/frame_extractor/config
```

**响应**：
```json
{
  "enable": true,
  "interval_ms": 1000,
  "output_dir": "./snapshots",
  "store": "minio",
  "minio": {
    "endpoint": "minio.example.com:9000",
    "bucket": "snapshots",
    "access_key": "xxx",
    "secret_key": "xxx",
    "use_ssl": false,
    "base_path": "camera-frames"
  }
}
```

### 更新配置

```bash
POST /api/v1/frame_extractor/config
Content-Type: application/json

{
  "enable": true,
  "interval_ms": 1000,
  "store": "minio",
  "minio": {
    "endpoint": "minio.example.com:9000",
    "bucket": "snapshots",
    "access_key": "minioadmin",
    "secret_key": "minioadmin",
    "use_ssl": false,
    "base_path": "frames"
  }
}
```

### 获取任务列表

```bash
GET /api/v1/frame_extractor/tasks
```

**响应**：
```json
{
  "items": [
    {
      "id": "cam1",
      "rtsp_url": "rtsp://...",
      "interval_ms": 1000,
      "output_path": "cam1"
    }
  ],
  "total": 1
}
```

### 添加任务

```bash
POST /api/v1/frame_extractor/tasks
Content-Type: application/json

{
  "id": "cam1",
  "rtsp_url": "rtsp://admin:password@192.168.1.100:554/stream",
  "interval_ms": 1000,
  "output_path": "cam1"
}
```

**MinIO模式**：会自动创建 `<bucket>/<base_path>/cam1/.keep` 文件

> ⚠️ 对于 `task_type = "绊线人数统计"` 的任务，还必须设置 `preferred_algorithm_endpoint` 字段来绑定唯一算法服务端点：
> ```json
> {
>   "task_type": "绊线人数统计",
>   "preferred_algorithm_endpoint": "http://tripwire-algo:8000/infer",
>   ...
> }
> ```

### 删除任务

```bash
DELETE /api/v1/frame_extractor/tasks/:id
```

**MinIO模式**：会自动删除 `<bucket>/<base_path>/<output_path>/` 下所有对象

---

## Makefile 命令

### 启用插件

```bash
make fx-enable
```
自动将 `config.toml` 中 `enable` 设为 `true`

### 检查FFmpeg

```bash
make fx-check-ffmpeg
```
验证 ffmpeg 是否可用（系统PATH或项目根目录）

### 添加任务（API方式）

```bash
make fx-add ID=cam1 RTSP='rtsp://user:pass@ip:554/...' INTERVAL=1000 OUT=cam1 SERVER=127.0.0.1:10086
```

### 运行示例

```bash
make fx-run-example RTSP='rtsp://admin:admin@192.168.1.100:554/stream'
```
自动构建、启用插件、启动服务并添加示例任务

---

## 工作原理

### 抽帧流程

1. **RTSP拉流**：使用FFmpeg持续拉取RTSP视频流
2. **帧过滤**：通过FFmpeg的fps滤镜按间隔提取关键帧
3. **格式转换**：输出MJPEG格式
4. **存储**：
   - 本地模式：直接写入文件系统
   - MinIO模式：实时上传到对象存储

### FFmpeg命令（本地存储）

```bash
ffmpeg -rtsp_transport tcp -stimeout 5000000 -i <rtsp_url> \
  -vf fps=1/1.0 -f image2 -strftime 1 \
  /path/to/output/%Y%m%d-%H%M%S.jpg
```

### FFmpeg命令（MinIO存储）

```bash
ffmpeg -rtsp_transport tcp -stimeout 5000000 -i <rtsp_url> \
  -vf fps=1/1.0 -f image2pipe -vcodec mjpeg pipe:1
```
输出到stdout，Go程序实时读取并上传

### 容错机制

- ⚡ **自动重连**：FFmpeg进程退出后自动重启
- 📈 **指数退避**：失败后等待时间从1s逐渐增加到30s
- 🔍 **健康监控**：日志记录所有启动、退出和错误事件

---

## 目录结构

### 本地存储

```
snapshots/
├── cam1/
│   ├── 20250114-153045.jpg
│   ├── 20250114-153046.jpg
│   └── ...
├── cam2/
│   └── ...
└── .../
```

### MinIO存储

```
<bucket>/
└── <base_path>/
    ├── cam1/
    │   ├── .keep
    │   ├── 20250114-153045.jpg
    │   ├── 20250114-153046.jpg
    │   └── ...
    ├── cam2/
    │   └── ...
    └── .../
```

---

## 常见问题

### Q: 如何验证抽帧是否正常？

**本地存储**：
```bash
ls -lh snapshots/cam1/
```

**MinIO存储**：
- 使用MinIO Console查看
- 或使用mc客户端：
  ```bash
  mc ls myminio/snapshots/camera-frames/cam1/
  ```

### Q: 如何调整抽帧间隔？

- UI修改：在任务列表点击"编辑"，修改间隔后保存
- 配置修改：编辑 `config.toml` 中 `interval_ms`
- API修改：POST 到 `/api/v1/frame_extractor/tasks`

### Q: 删除任务会删除已有图片吗？

- **本地存储**：不会删除，需手动清理
- **MinIO存储**：会自动删除对应路径下所有对象

### Q: MinIO连接失败怎么办？

检查：
1. Endpoint是否可达：`ping <endpoint>`
2. 端口是否开放：`telnet <endpoint> 9000`
3. Access Key/Secret Key是否正确
4. 是否需要SSL：检查 `use_ssl` 配置

### Q: FFmpeg找不到怎么办？

- 系统安装：`apt-get install ffmpeg`
- 或将ffmpeg二进制放到项目根目录
- 验证：`make fx-check-ffmpeg`

---

## 性能建议

### 抽帧间隔

| 场景 | 推荐间隔 |
|------|----------|
| 实时监控 | 500-1000ms |
| 定期快照 | 5000-10000ms |
| 低频归档 | 30000-60000ms |

### MinIO优化

- 使用SSD存储提升上传速度
- 启用MinIO压缩节省空间
- 设置生命周期策略自动清理旧文件：
  ```bash
  mc ilm add myminio/snapshots --expiry-days 7
  ```

### 并发任务

- 单台服务器建议 ≤ 10个并发任务
- 每个任务占用一个FFmpeg进程
- 监控CPU/内存使用率

---

## 高级用法

### 1. 通过API批量添加任务

```bash
#!/bin/bash
CAMS=(
  "cam1:rtsp://192.168.1.101:554/stream"
  "cam2:rtsp://192.168.1.102:554/stream"
  "cam3:rtsp://192.168.1.103:554/stream"
)

for cam in "${CAMS[@]}"; do
  IFS=: read -r id url <<< "$cam"
  curl -X POST http://127.0.0.1:10086/api/v1/frame_extractor/tasks \
    -H 'Content-Type: application/json' \
    -d "{\"id\":\"$id\",\"rtsp_url\":\"$url\",\"interval_ms\":1000,\"output_path\":\"$id\"}"
done
```

### 2. 使用MinIO SDK清理旧文件

参考 MinIO lifecycle policies 或自定义脚本：
```python
from minio import Minio
from datetime import datetime, timedelta

client = Minio('minio.example.com:9000',
               access_key='xxx',
               secret_key='xxx',
               secure=False)

# 删除7天前的快照
cutoff = datetime.now() - timedelta(days=7)
for obj in client.list_objects('snapshots', prefix='camera-frames/', recursive=True):
    if obj.last_modified < cutoff:
        client.remove_object('snapshots', obj.object_name)
```

---

## 故障排查

### 日志位置

```bash
tail -f logs/sugar.log
```

### 常见错误

#### 1. "snapshot failed"

**原因**：RTSP拉流失败  
**解决**：
- 检查RTSP URL是否正确
- 验证摄像头网络连通性
- 确认用户名/密码正确

#### 2. "minio not initialized"

**原因**：MinIO配置不完整或连接失败  
**解决**：
- 检查 `endpoint`、`bucket`、`access_key`、`secret_key` 是否正确
- 验证MinIO服务是否运行
- 查看日志中具体错误信息

#### 3. "failed to persist config"

**原因**：配置文件写入权限不足  
**解决**：
- 确保程序对 `configs/config.toml` 有写权限
- 检查磁盘空间是否充足

---

## 架构设计

```
+-------------------------+
|    EasyDarwin Core      |
|-------------------------|
| RTSP/HLS 流管理         |
+-------------------------+
          │
          ▼
+-------------------------+
|  Frame Extractor Plugin |
|-------------------------|
| ┌─────────────────────┐ |
| │  Config Manager     │ |
| │  - UI配置接口       │ |
| │  - TOML持久化       │ |
| └─────────────────────┘ |
| ┌─────────────────────┐ |
| │  Task Manager       │ |
| │  - 运行时增删       │ |
| │  - 生命周期管理     │ |
| └─────────────────────┘ |
| ┌─────────────────────┐ |
| │  Stream Worker      │ |
| │  - FFmpeg拉流       │ |
| │  - 帧解码抽取       │ |
| │  - 自动重连         │ |
| └─────────────────────┘ |
| ┌─────────────────────┐ |
| │  Storage Sink       │ |
| │  - Local FS         │ |
| │  - MinIO Uploader   │ |
| │  - 路径管理         │ |
| └─────────────────────┘ |
+-------------------------+
          │
          ▼
+-------------------------+
|   Storage Backend       |
|-------------------------|
| 本地: ./snapshots/      |
| MinIO: bucket/path/     |
+-------------------------+
```

---

## 开发与扩展

### 添加新存储后端

1. 在 `worker.go` 添加新的 `run<Storage>SinkLoop` 方法
2. 在 `service.go` 的 `startTask` 中添加分支
3. 在 `config.toml` 和 `model.go` 添加配置
4. 前端UI添加对应配置表单

### 自定义帧处理

修改 `buildContinuousArgs` 添加FFmpeg滤镜：
```go
// 示例：缩放图片
args = append(args, "-vf", fmt.Sprintf("fps=1/%.6f,scale=640:480", sec))

// 示例：添加水印
args = append(args, "-vf", fmt.Sprintf("fps=1/%.6f,drawtext=text='%s':x=10:y=10", sec, taskID))
```

---

## 许可与支持

- 项目：EasyDarwin
- 官网：www.easydarwin.org
- 开源协议：遵循主项目

