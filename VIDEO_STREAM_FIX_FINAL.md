# 视频转流问题完整修复 - 最终版本

## ✅ 已完成的修复

### 1. 改进流直播功能的 `AddSession` 方法
**文件：** `internal/core/source/source_client.go`

**修复内容：**
- ✅ 如果Session已存在，先清理再创建
- ✅ 添加重试机制（最多3次），每次重试前等待200ms
- ✅ 避免"流已存在"错误

### 2. 优化FFmpeg推流参数
**文件：** `internal/plugin/videortsp/ffmpeg.go`

**新增参数：**
- ✅ `-fflags +genpts` - 确保生成PTS时间戳
- ✅ `-force_key_frames expr:gte(n,n_forced*1)` - 强制第一个帧是关键帧
- ✅ `-x264-params keyint=50:min-keyint=50:scenecut=0` - 确保SPS/PPS在每个关键帧前发送（仅libx264）
- ✅ `-bsf:a aac_adtstoasc` - 确保AAC序列头正确发送

### 3. 实现智能等待机制 ⭐ 核心修复
**文件：** `internal/plugin/videortsp/core.go`

#### 3.1 `waitForSessionCleanup` - 智能等待Session清理
- ✅ 检查hook session是否还存在
- ✅ 每200ms检查一次，最多等待10秒
- ✅ 确保旧的Session被完全清理，避免"流已存在"错误

#### 3.2 `waitForStreamReady` - 智能等待流就绪
- ✅ 检查流是否有视频序列头
- ✅ 每200ms检查一次，最多等待15秒
- ✅ 确保RTMP流完全就绪后再继续，避免"invalid video and audio info"错误

**关键优势：**
- 🚀 不再使用固定等待时间，流就绪后立即继续
- 🚀 如果超时仍未就绪，记录警告但继续执行（避免阻塞）
- 🚀 提高效率，减少不必要的等待

### 4. 改进进程停止逻辑
**文件：** `internal/plugin/videortsp/ffmpeg.go`

- ✅ 停止FFmpeg进程后，额外等待1秒
- ✅ 确保RTMP服务器完全清理Session

## 工作流程

### 启动流流程（优化后）：
```
1. 检查是否有重复的流任务 → 停止它们
2. 停止当前流名称的所有进程
3. ⭐ 智能等待Session清理（最多10秒，每200ms检查一次）
4. 启动FFmpeg推流
5. ⭐ 智能等待流就绪（检查序列头，最多15秒，每200ms检查一次）
6. 更新任务状态为运行中
```

### 停止流流程：
```
1. 停止FFmpeg进程
2. 等待1秒，确保RTMP服务器清理Session
3. 更新任务状态
```

## 技术实现细节

### RTMP流名称格式
- **RTMP推流URL**：`rtmp://host:port/video/streamName`
  - `video` 是app name
  - `streamName` 是流名称（如 `video_xxx`）
- **lal的hook session**：使用的流名称就是 `streamName`（不包含app name）

### 流就绪检查机制
```go
// 1. 获取hook session
ok, session := hook.GetHookSessionManagerInstance().GetHookSession(streamName)

// 2. 检查是否有视频序列头
videoHeader := session.GetVideoSeqHeaderMsg()
if videoHeader != nil {
    // 流已就绪！
}
```

## 预期效果

### ✅ 解决"流已存在"错误
- 通过智能等待Session清理，确保旧Session被完全清理
- 避免新流启动时与旧Session冲突

### ✅ 解决"SDP无效"错误
- 通过智能等待流就绪，确保RTMP流有序列头后再继续
- 避免RTMP转RTSP时SDP信息不完整

### ✅ 提高效率
- 不再使用固定等待时间（之前是8秒固定等待）
- 流就绪后立即继续，减少等待时间
- 如果流在1秒内就绪，只等待1秒；如果8秒内就绪，只等待8秒

## 修改的文件清单

1. ✅ `internal/core/source/source_client.go` - 改进AddSession方法
2. ✅ `internal/plugin/videortsp/core.go` - 实现智能等待机制
3. ✅ `internal/plugin/videortsp/ffmpeg.go` - 优化FFmpeg参数和停止逻辑
4. ✅ `internal/plugin/aianalysis/scheduler.go` - 修复编译错误（删除多余的x）

## 编译状态

✅ 所有模块编译通过
- ✅ 主程序编译成功
- ✅ 视频转流模块编译成功
- ✅ 流直播模块编译成功

## 下一步

1. **重启服务**：确保所有修改生效
2. **测试视频转流功能**：
   - 创建新的视频转流任务
   - 快速重启流（停止后立即启动）
   - 观察日志，确认问题是否解决
3. **如果问题仍然存在**：
   - 检查日志中的等待时间
   - 可能需要增加超时时间
   - 检查FFmpeg日志，确认序列头是否正确发送

## 关键改进点

### 之前的问题：
- ❌ 使用固定等待时间（8秒），无论流是否就绪
- ❌ 没有检查Session是否真的被清理
- ❌ 没有检查流是否有序列头

### 现在的解决方案：
- ✅ 智能等待Session清理（检查hook session是否存在）
- ✅ 智能等待流就绪（检查是否有视频序列头）
- ✅ 流就绪后立即继续，提高效率

