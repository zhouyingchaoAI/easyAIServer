# RTSP播放问题修复

## 问题描述
- **错误**: `invalid video and audio info, sdp:v=0`
- **现象**: RTSP流无法播放，SDP信息中缺少视频和音频流信息
- **原因**: 序列头还没有准备好就被用于生成SDP

## 修复方案

### 1. 修复FFmpeg参数顺序
**文件**: `internal/plugin/videortsp/ffmpeg.go`

- 修复`-map`参数顺序：先视频后音频
- 确保音频映射是可选的（使用`0:a?`）

```go
// 修复前
args = append(args, "-map", "0:a?")
args = append(args, "-map", "0:v:0")

// 修复后
args = append(args, "-map", "0:v:0")
if task.AudioCodec != "" {
    args = append(args, "-map", "0:a?")
}
```

### 2. 增加等待时间
**文件**: `internal/plugin/videortsp/core.go`

- FFmpeg启动后等待时间：2秒 → 3秒
- 最小等待时间：3秒 → 5秒
- 超时时间：20秒 → 30秒
- 即使超时也再等待2秒

### 3. 改进流就绪检测
**文件**: `internal/plugin/videortsp/core.go`

- 需要连续2次检查都成功才认为流就绪
- 确保序列头稳定存在，而不是临时存在

```go
// 连续检查次数，确保序列头稳定存在
readyCount := 0
requiredReadyCount := 2 // 需要连续2次检查都成功

// 如果序列头不存在，重置计数
if videoHeader == nil {
    readyCount = 0
}
```

### 4. 优化x264参数
**文件**: `internal/plugin/videortsp/ffmpeg.go`

- 添加`force-cfr=1`确保恒定帧率
- 确保序列头在流开始时立即发送

## 测试建议

1. **重新编译并部署服务**
2. **创建新的视频转流任务**
3. **启动流并等待足够时间（约10-15秒）**
4. **使用RTSP播放器测试播放**
5. **检查日志确认没有"invalid video and audio info"错误**

## 预期效果

- ✅ SDP信息包含完整的视频和音频流信息
- ✅ RTSP流可以正常播放
- ✅ 没有"invalid video and audio info"错误
- ✅ 序列头在流开始时立即发送

## 注意事项

1. **启动延迟**: 修复增加了等待时间，流启动可能需要10-15秒
2. **序列头检测**: 需要连续2次检查成功，确保序列头稳定
3. **超时处理**: 即使超时也会再等待2秒，确保序列头被处理

