# FFmpeg命令调试指南

## 问题
FFmpeg进程立即退出，需要调试FFmpeg命令以确保参数正确。

## 调试方法

### 方法1: 通过API获取FFmpeg命令

**API端点**: `GET /api/v1/video_rtsp/{id}/ffmpeg_command`

**示例**:
```bash
# 获取任务ID（从创建任务的响应中获取）
TASK_ID="1764140238784761200"

# 获取FFmpeg命令
curl http://127.0.0.1:5066/api/v1/video_rtsp/$TASK_ID/ffmpeg_command
```

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "ffmpegPath": "/path/to/ffmpeg",
    "rtmpUrl": "rtmp://127.0.0.1:21935/video/video_xxx",
    "command": "ffmpeg -stream_loop -1 -re ...",
    "formattedCmd": "ffmpeg \\\n  -stream_loop \\\n  -1 \\\n  ...",
    "args": ["-stream_loop", "-1", "-re", ...],
    "argsCount": 25
  }
}
```

### 方法2: 使用测试脚本

**脚本**: `test_ffmpeg_command.sh`

**用法**:
```bash
# 基本用法（使用默认参数）
bash test_ffmpeg_command.sh

# 指定参数
bash test_ffmpeg_command.sh \
  /path/to/video.mp4 \
  rtmp://127.0.0.1:21935/video/test_stream \
  libx264 \
  aac \
  true
```

**参数说明**:
1. 视频路径（必需）
2. RTMP URL（可选，默认: rtmp://127.0.0.1:21935/video/test_stream）
3. 视频编码（可选，默认: libx264）
4. 音频编码（可选，默认: aac）
5. 是否循环（可选，默认: true）

**脚本功能**:
- 自动检测FFmpeg路径
- 显示完整的FFmpeg命令
- 显示格式化的命令（每行一个参数）
- 询问是否执行命令
- 执行命令并显示输出

### 方法3: 查看日志

启动任务后，在日志中搜索 `ffmpeg command`：

```bash
# 查看日志
tail -f build/EasyDarwin-aarch64-v8.3.3-202511261444/logs/20251126_08_00_00.log | grep "ffmpeg command"
```

日志会显示：
```json
{
  "level": "info",
  "msg": "ffmpeg command",
  "stream_name": "video_xxx",
  "command": "ffmpeg -stream_loop -1 -re ...",
  "args_count": 25
}
```

## 手动测试FFmpeg命令

1. **获取命令**（使用上述任一方法）
2. **复制完整命令**
3. **在终端中执行**，观察输出和错误
4. **根据错误信息调整参数**

## 常见问题排查

### 1. 视频文件不存在
**错误**: `No such file or directory`
**解决**: 检查视频路径是否正确

### 2. 编码器不支持
**错误**: `Encoder 'xxx' not found`
**解决**: 检查FFmpeg是否支持指定的编码器，或使用 `copy`

### 3. 音频流不存在
**错误**: `Stream map '0:a?' matches no streams`
**解决**: 视频文件可能没有音频轨道，可以：
- 使用 `-an` 禁用音频
- 或者不指定音频编码

### 4. RTMP连接失败
**错误**: `Connection refused` 或 `Connection timed out`
**解决**: 
- 检查RTMP服务器是否运行
- 检查RTMP URL是否正确
- 检查防火墙设置

### 5. 参数错误
**错误**: `Invalid argument` 或 `Unrecognized option`
**解决**: 检查FFmpeg版本，某些参数可能不支持

## 调试步骤

1. **获取FFmpeg命令**（使用API或脚本）
2. **手动执行命令**，观察输出
3. **如果失败，查看错误信息**
4. **根据错误调整参数**
5. **重新测试直到成功**
6. **如果命令成功，但程序仍然失败，检查进程管理逻辑**

## 简化命令测试

如果完整命令太复杂，可以逐步简化：

```bash
# 1. 最简命令（测试基本功能）
ffmpeg -i video.mp4 -c:v copy -c:a copy -f flv rtmp://127.0.0.1:21935/video/test

# 2. 添加实时模式
ffmpeg -re -i video.mp4 -c:v copy -c:a copy -f flv rtmp://127.0.0.1:21935/video/test

# 3. 添加循环
ffmpeg -stream_loop -1 -re -i video.mp4 -c:v copy -c:a copy -f flv rtmp://127.0.0.1:21935/video/test

# 4. 逐步添加其他参数
```

## 修改后的文件

- `internal/plugin/videortsp/ffmpeg.go` - 添加BuildFFmpegCommand方法和日志输出
- `internal/plugin/videortsp/api.go` - 添加GetFFmpegCommandHandler API端点
- `internal/plugin/videortsp/core.go` - 添加GetRTMPHost方法
- `test_ffmpeg_command.sh` - 测试脚本（新增）

