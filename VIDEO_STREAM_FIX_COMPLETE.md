# 视频转流功能完整修复方案

## 问题分析

从日志分析发现以下关键问题：

1. **"in stream already exist"** - RTMP服务器端会话没有完全清理
2. **"invalid video and audio info, sdp:v=0"** - RTSP转换时序列头还没准备好
3. **FFmpeg进程退出错误** - FFmpeg推流失败
4. **waitForStreamReady返回true但等待时间为0ms** - 检查逻辑有问题

## 修复方案

### 1. 增加会话清理等待时间

**文件**: `internal/plugin/videortsp/core.go`

- 在`waitForSessionCleanup`后增加2秒等待时间，确保RTMP服务器完全清理内部会话
- RTMP服务器可能需要额外时间清理内部状态

```go
// 额外等待时间，确保RTMP服务器完全清理内部会话
time.Sleep(2 * time.Second)
```

### 2. 增加FFmpeg启动后的等待时间

**文件**: `internal/plugin/videortsp/core.go`

- 在FFmpeg进程启动后，等待2秒让RTMP服务器注册会话
- 这给RTMP服务器时间创建hook session

```go
// 等待FFmpeg进程启动并让RTMP服务器注册会话
time.Sleep(2 * time.Second)
```

### 3. 改进waitForStreamReady逻辑

**文件**: `internal/plugin/videortsp/core.go`

- 增加最小等待时间（3秒），确保RTMP服务器有时间注册会话
- 检查间隔从200ms增加到500ms，给RTMP服务器更多时间
- 改进序列头检查逻辑，检查payload长度确保序列头有效
- 超时时间从15秒增加到20秒

```go
minWaitTime := 3 * time.Second // 最小等待时间
checkInterval := 500 * time.Millisecond // 检查间隔
// 检查payload长度确保序列头有效
if videoHeader != nil && len(videoHeader.Payload) > 0 {
    // ...
}
```

### 4. 增加停止进程后的等待时间

**文件**: `internal/plugin/videortsp/ffmpeg.go`

- 停止FFmpeg进程后，等待时间从1秒增加到3秒
- 确保RTMP服务器内部会话完全清理

```go
// 增加等待时间，确保RTMP服务器内部会话完全清理
time.Sleep(3 * time.Second)
```

## 测试脚本

创建了完整的测试脚本 `test_video_stream.sh`，包含以下测试：

1. **检查服务运行状态** - 验证API服务是否可访问
2. **创建视频转流任务** - 测试任务创建功能
3. **启动流** - 测试流启动功能
4. **检查流可用性** - 验证RTSP流是否可用
5. **停止流** - 测试流停止功能
6. **重复启动测试** - 测试会话清理功能（关键测试）
7. **检查日志错误** - 验证是否有相关错误日志

## 使用方法

### 运行测试脚本

```bash
cd /code/EasyDarwin
bash test_video_stream.sh
```

测试脚本会自动：
- 检查服务是否运行
- 创建测试任务
- 执行各项测试
- 输出测试结果
- 清理测试数据

### 手动测试

1. **创建任务**:
```bash
curl -X POST http://127.0.0.1:5066/api/v1/video_rtsp \
  -H "Content-Type: application/json" \
  -d '{
    "video_path": "/path/to/video.mp4",
    "video_codec": "libx264",
    "audio_codec": "aac",
    "enabled": false
  }'
```

2. **启动流**:
```bash
curl -X POST http://127.0.0.1:5066/api/v1/video_rtsp/{task_id}/start
```

3. **检查状态**:
```bash
curl http://127.0.0.1:5066/api/v1/video_rtsp/{task_id}
```

4. **停止流**:
```bash
curl -X POST http://127.0.0.1:5066/api/v1/video_rtsp/{task_id}/stop
```

## 预期效果

修复后应该解决以下问题：

1. ✅ **"in stream already exist"错误** - 通过增加等待时间确保会话完全清理
2. ✅ **"invalid video and audio info"错误** - 通过改进等待逻辑确保序列头准备好
3. ✅ **FFmpeg进程退出错误** - 通过增加等待时间确保流稳定
4. ✅ **重复启动失败** - 通过改进会话清理逻辑确保可以重复启动

## 注意事项

1. **等待时间** - 修复增加了等待时间，可能会略微增加启动延迟（约7-10秒），但提高了稳定性
2. **资源清理** - 确保停止流后有足够时间清理资源
3. **日志监控** - 建议监控日志文件，观察是否还有相关错误

## 修改的文件

- `internal/plugin/videortsp/core.go` - 核心逻辑修复
- `internal/plugin/videortsp/ffmpeg.go` - 进程管理修复
- `test_video_stream.sh` - 测试脚本（新增）

## 下一步

1. 重新编译并部署服务
2. 运行测试脚本验证修复效果
3. 监控日志确认问题已解决
4. 根据实际使用情况调整等待时间参数（如需要）

