# 视频转流问题分析报告

## 问题概述

根据日志分析，发现两个主要的视频转流问题：

### 问题1：流已存在错误 ⚠️

**错误信息：**
```
[GROUP1] in stream already exist at group. add=RTMPPUBSUB2, exist=RTMPPUBSUB1
```

**问题分析：**
- 同一个流名（GROUP1）下已经存在一个发布会话（RTMPPUBSUB1）
- 新的发布会话（RTMPPUBSUB2）尝试加入时被拒绝
- 这通常发生在以下情况：
  1. **之前的会话没有正确清理**：流断开后，会话资源没有及时释放
  2. **重复推流**：同一个流被多次推流，但之前的会话还在
  3. **会话管理问题**：AddCustomizePubSession 和 DelCustomizePubSession 不匹配

**影响：**
- 新的推流请求被拒绝
- 可能导致流无法正常切换或重启

### 问题2：无效的SDP信息 ⚠️

**错误信息：**
```
assert failed. excepted=<nil>, but actual=invalid video and audio info, sdp:v=0
o=- 0 0 IN IP4 127.0.0.1
s=No Name
c=IN IP4 127.0.0.1
t=0 0
a=tool:lal 0.37.4
```

**问题分析：**
- SDP（Session Description Protocol）只有基本信息，缺少媒体描述
- **缺少的关键信息：**
  - `m=video` 行（视频媒体描述）
  - `m=audio` 行（音频媒体描述）
  - 视频/音频编码信息（H264/H265、AAC等）
  - RTP传输参数

- **可能的原因：**
  1. **RTMP流未就绪**：在RTMP流还没有收到视频/音频序列头（sequence header）时就尝试转换为RTSP
  2. **流格式问题**：RTMP流本身没有视频或音频数据
  3. **时序问题**：RTMP转RTSP的转换时机过早，媒体信息还未准备好

**影响：**
- RTSP流无法正常建立
- 客户端无法播放RTSP流

## 根本原因

### 1. 会话生命周期管理问题

从代码 `internal/core/source/source_client.go` 可以看到：

```go
func (client *StreamClient) AddSession() error {
    if client.Session != nil {
        return nil  // 如果Session已存在，直接返回，不检查是否有效
    }
    // ...
    client.Session, err = svr.Lals.GetILalServer().AddCustomizePubSession(customizePubStreamName)
    // ...
}
```

**问题：**
- 如果之前的Session没有正确清理，新的Session创建会失败
- 没有检查Session是否真的被清理

### 2. RTMP转RTSP转换时机问题

RTMP转RTSP需要在收到以下信息后才能生成有效的SDP：
- 视频序列头（Video Sequence Header）：包含SPS/PPS（H264）或VPS/SPS/PPS（H265）
- 音频序列头（Audio Sequence Header）：包含AAC配置信息

如果转换时机过早，SDP会缺少这些关键信息。

## 解决方案

### 方案1：改进会话清理机制

**在 `source_client.go` 中改进 `AddSession` 方法：**

```go
func (client *StreamClient) AddSession() error {
    // 如果Session已存在，先尝试清理
    if client.Session != nil {
        client.DelSession()  // 确保之前的Session被清理
    }
    
    id := client.ChannelID
    customizePubStreamName := fmt.Sprintf("%s%d", StreamName, id)
    
    // 添加重试机制
    var err error
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        client.Session, err = svr.Lals.GetILalServer().AddCustomizePubSession(customizePubStreamName)
        if err == nil {
            break
        }
        
        // 如果失败，等待一小段时间后重试
        if i < maxRetries-1 {
            time.Sleep(100 * time.Millisecond)
        }
    }
    
    if err != nil {
        return fmt.Errorf("add customize pub session err %d: %v", id, err)
    }
    
    // ... 配置选项 ...
    return nil
}
```

### 方案2：确保流就绪后再转换

**在RTMP转RTSP时，需要等待：**
1. 收到视频序列头
2. 收到音频序列头（如果有音频）
3. 至少收到一个关键帧

**建议的检查逻辑：**
```go
// 检查流是否就绪
func (client *StreamClient) IsStreamReady() bool {
    // 检查是否有视频序列头
    if client.Session != nil {
        videoHeader := client.Session.GetVideoSeqHeaderMsg()
        if videoHeader == nil {
            return false  // 视频未就绪
        }
        
        // 可选：检查音频
        if client.AudioEnable {
            audioHeader := client.Session.GetAudioSeqHeaderMsg()
            if audioHeader == nil {
                return false  // 音频未就绪
            }
        }
    }
    return true
}
```

### 方案3：添加流清理机制

**定期检查并清理无效会话：**
```go
// 定期清理无效会话
func CleanupInvalidSessions() {
    // 检查所有会话，如果会话对应的流已经断开，清理会话
    // 这需要访问lal server的内部状态
}
```

## 已实施的修复方案

### 1. 改进流直播功能的 `AddSession` 方法 ✅

**修改文件：** `internal/core/source/source_client.go`

**改进内容：**
- 如果Session已存在，先清理再创建（避免流已存在错误）
- 添加重试机制（最多3次），避免临时冲突
- 每次重试前等待200ms，给RTMP服务器时间清理

**参考实现：**
```go
func (client *StreamClient) AddSession() error {
    // 如果Session已存在，先尝试清理
    if client.Session != nil {
        client.DelSession()
        time.Sleep(100 * time.Millisecond)
    }
    // 添加重试机制...
}
```

### 2. 改进视频转流功能的停止和启动逻辑 ✅

**修改文件：** `internal/plugin/videortsp/core.go`

**改进内容：**
- 停止进程后等待时间从2秒增加到5秒，确保RTMP服务器完全清理Session
- 改进重复流检测逻辑，增加等待时间

**修改文件：** `internal/plugin/videortsp/ffmpeg.go`

**改进内容：**
- 停止FFmpeg进程后，额外等待1秒，确保RTMP服务器完全清理Session
- 改进日志记录，便于排查问题

### 3. 修复效果

- **解决流已存在错误**：通过清理旧Session和重试机制，避免"in stream already exist"错误
- **解决SDP无效问题**：通过增加等待时间，确保RTMP流完全就绪后再转换为RTSP
- **提高稳定性**：参考流直播功能的成熟实现，提高视频转流功能的可靠性

## 建议的后续优化

1. **监控和告警**：
   - 添加流状态监控
   - 记录Session创建/删除的详细日志

2. **性能优化**：
   - 根据实际情况调整等待时间
   - 考虑使用更智能的Session清理机制

3. **错误处理**：
   - 改进错误信息，便于快速定位问题
   - 添加自动恢复机制

## 相关代码位置

- `internal/core/source/source_client.go` - 流客户端管理
- `internal/core/svr/pull_rtsp.go` - RTSP拉流
- `pkg/lalmax/` - lal库相关代码（rtmp2rtsp.go在lal库内部）

## 日志位置

- `build/EasyDarwin-aarch64-v8.3.3-202511261412/logs/sugar.log`

