# 视频转流问题修复总结

## 问题描述

从日志分析，发现两个主要问题：

1. **"in stream already exist"** - 流已存在错误
2. **"invalid video and audio info, sdp:v=0"** - SDP信息无效错误

## 已实施的修复

### 1. 改进流直播功能的 `AddSession` 方法
**文件：** `internal/core/source/source_client.go`

- 如果Session已存在，先清理再创建
- 添加重试机制（最多3次）
- 每次重试前等待200ms

### 2. 优化FFmpeg推流参数
**文件：** `internal/plugin/videortsp/ffmpeg.go`

**新增参数：**
- `-fflags +genpts` - 确保生成PTS时间戳
- `-force_key_frames expr:gte(n,n_forced*1)` - 强制第一个帧是关键帧
- `-x264-params keyint=50:min-keyint=50:scenecut=0` - 确保SPS/PPS在每个关键帧前发送（仅libx264）
- `-bsf:a aac_adtstoasc` - 确保AAC序列头正确发送

### 3. 增加等待时间
**文件：** `internal/plugin/videortsp/core.go`

- 停止进程后等待时间：从5秒增加到8秒
- 启动流后等待时间：从3秒增加到8秒
- 确保RTMP服务器有足够时间清理Session和处理流

### 4. 改进进程停止逻辑
**文件：** `internal/plugin/videortsp/ffmpeg.go`

- 停止FFmpeg进程后，额外等待1秒
- 确保RTMP服务器完全清理Session

## 问题分析

从日志看，错误发生在RTMP转RTSP时（`rtmp2rtsp.go:210`），说明：
- RTMP流可能还没有收到视频/音频序列头
- 或者RTMP服务器还没有处理完序列头
- 客户端在流还没完全就绪时就尝试通过RTSP播放

## 建议的测试步骤

1. **重启服务**：确保所有修改生效
   ```bash
   # 停止当前服务
   # 重新编译
   make build/local
   # 启动服务
   ```

2. **测试视频转流**：
   - 创建一个新的视频转流任务
   - 等待流启动完成
   - 尝试通过RTSP播放

3. **观察日志**：
   - 检查是否还有"流已存在"错误
   - 检查是否还有"invalid video and audio info"错误
   - 如果问题仍然存在，可能需要进一步增加等待时间

## 如果问题仍然存在

如果等待8秒后问题仍然存在，可能的原因：

1. **RTMP服务器处理延迟**：可能需要更长的等待时间（10-15秒）
2. **FFmpeg推流问题**：检查FFmpeg日志，确认序列头是否正确发送
3. **RTMP服务器配置**：检查RTMP服务器配置，确认是否正确处理序列头
4. **网络延迟**：如果RTMP服务器和客户端不在同一机器，可能有网络延迟

## 进一步优化建议

如果问题持续存在，可以考虑：

1. **添加流就绪检查**：通过API检查RTMP流是否真的就绪（是否有序列头）
2. **增加重试机制**：如果RTSP播放失败，自动重试
3. **改进错误处理**：提供更详细的错误信息，便于排查问题

