# 直播流地址说明文档

## 概述

EasyDarwin 直播服务支持两种类型的RTSP地址：
1. **原始流地址**：推流端的RTSP源地址（存储在数据库中）
2. **转发流地址**：EasyDarwin服务器的统一播放地址（用于播放和分发）

抽帧插件使用的是**转发流地址**，确保稳定性和统一管理。

---

## 地址类型对比

| 类型 | 用途 | 格式示例 | 来源 |
|-----|------|---------|------|
| **原始流地址** | 推流配置 | `rtsp://admin:pass@192.168.1.100:554/stream` | 摄像头/推流端 |
| **转发流地址** | 播放/抽帧 | `rtsp://服务器IP:554/live/stream_1` | EasyDarwin服务器 |

---

## API工作流程

### 1. 获取直播流列表

```bash
GET /api/v1/live
```

**返回示例**：
```json
{
  "items": [
    {
      "id": 1,
      "name": "客流分析摄像头",
      "url": "rtsp://admin:pass@192.168.1.100:554/stream",
      "live_type": "pull",
      "online": 1,
      "enable": true
    },
    {
      "id": 2,
      "name": "门口监控",
      "url": "rtsp://192.168.1.101:554/h264",
      "live_type": "pull",
      "online": 1,
      "enable": true
    }
  ],
  "total": 2
}
```

### 2. 通过流ID获取转发播放地址

```bash
GET /api/v1/live/playurl/:id
```

**请求示例**：
```bash
curl http://localhost:5066/api/v1/live/playurl/1
```

**返回示例**：
```json
{
  "info": {
    "ID": 1,
    "Name": "",
    "RTSP": "rtsp://10.1.6.230:554/live/stream_1",
    "HttpFlv": "http://10.1.6.230:5066/flv/live/stream_1.flv",
    "HttpHls": "http://10.1.6.230:5066/ts/hls/stream_1/playlist.m3u8",
    "WsFlv": "ws://10.1.6.230:5066/ws_flv/live/stream_1.flv",
    "WEBRTC": "webrtc://10.1.6.230:5066/webrtc/play/live/stream_1"
  }
}
```

**关键字段说明**：
- `RTSP`: RTSP协议播放地址（用于抽帧）
- `HttpFlv`: HTTP-FLV协议播放地址
- `HttpHls`: HLS协议播放地址
- `WEBRTC`: WebRTC协议播放地址

---

## 前端实现

### 下拉列表加载

```javascript
const fetchLiveStreams = async () => {
  const { data } = await live.getLiveList({})
  liveStreams.value = data?.items || []
  
  // 构建选项：显示流名称和ID
  rtspOptions.value = liveStreams.value.map(stream => ({
    value: String(stream.id),
    label: `${stream.name} (ID: ${stream.id})`,
    streamId: stream.id,
    streamName: stream.name
  }))
}
```

### 选择流后获取播放地址

```javascript
const onStreamSelect = async (streamId, option) => {
  const id = parseInt(streamId)
  
  // 调用API获取转发地址
  const { data } = await live.getPlayUrl(id)
  
  // 提取RTSP播放地址
  if (data?.info?.RTSP) {
    form.value.rtsp_url = data.info.RTSP
    message.success(`已选择: ${option.streamName}`)
  }
}
```

---

## 用户操作流程

### 方式1：从直播列表选择（推荐）

1. 打开抽帧管理页面
2. 点击 "RTSP地址" 输入框
3. 下拉列表显示：
   ```
   客流分析摄像头 (ID: 1)
   门口监控 (ID: 2)
   大厅监控 (ID: 5)
   ```
4. 选择一个流
5. 系统自动：
   - 调用 `GET /live/playurl/1`
   - 获取 `rtsp://10.1.6.230:554/live/stream_1`
   - 填充到表单
6. 提示"已选择: 客流分析摄像头"

### 方式2：手动输入

如果直播流列表中没有，可以直接输入RTSP地址：
```
rtsp://user:pass@192.168.1.200:554/stream
```

---

## 转发地址格式

EasyDarwin服务器的转发地址遵循统一格式：

```
rtsp://[服务器IP]:[RTSP端口]/live/stream_[流ID]
```

**示例**：
- 服务器IP: `10.1.6.230`
- RTSP端口: `554` (来自 `config.toml` 的 `rtsp.addr`)
- 流ID: `1`
- **转发地址**: `rtsp://10.1.6.230:554/live/stream_1`

---

## 端口配置

转发地址的端口来自 `config.toml`：

```toml
[rtsp]
addr = 554
rtsps_addr = 322
```

- **HTTP端口**: `default_http_config.http_listen_addr = 5066`
- **RTSP端口**: `rtsp.addr = 554`
- **HTTPS端口**: `default_http_config.https_listen_addr = 10443`

---

## 抽帧插件中的使用

抽帧任务配置示例：

```toml
[[frame_extractor.tasks]]
id = '客流分析1'
rtsp_url = 'rtsp://10.1.6.230:554/live/stream_1'  # 使用转发地址
interval_ms = 5000
output_path = '客流分析1'
enabled = true
```

**为什么使用转发地址**：
- ✅ 统一管理：所有流都通过EasyDarwin转发
- ✅ 稳定性：服务器端负责重连和错误处理
- ✅ 安全性：无需直接暴露摄像头密码
- ✅ 多协议：同一个流可以RTSP/HLS/FLV多协议播放

---

## 调试技巧

### 1. 查看直播流列表
```bash
curl http://localhost:5066/api/v1/live | jq
```

### 2. 获取指定流的播放地址
```bash
curl http://localhost:5066/api/v1/live/playurl/1 | jq
```

### 3. 测试RTSP播放
```bash
ffplay rtsp://10.1.6.230:554/live/stream_1
```

### 4. 查看抽帧日志
```bash
tail -f logs/sugar.log | grep "frame extractor"
```

---

## 常见问题

### Q1: 为什么不直接使用原始RTSP地址？

**A**: 原始地址可能：
- 包含敏感信息（用户名密码）
- 不同摄像头格式不统一
- 没有服务器端重连机制
- 无法利用EasyDarwin的流管理功能

### Q2: 转发地址获取失败怎么办？

**A**: 检查：
1. 直播流是否已启动（`enable = true`）
2. 直播流是否在线（`online = 1`）
3. RTSP服务是否正常运行
4. 端口配置是否正确

### Q3: 可以混用原始地址和转发地址吗？

**A**: 可以，但建议：
- 优先使用转发地址（从列表选择）
- 仅在测试或特殊情况下手动输入原始地址

### Q4: RTSP端口号不正确怎么办？

**A**: 确保配置正确加载：
1. 检查 `config.toml` 中 `[rtspconfig]` 的 `addr` 配置
2. 确认格式为 `addr = ':15544'` （带冒号）
3. 重启服务后验证：
   ```bash
   curl http://localhost:5066/api/v1/live/playurl/1 | jq '.info.RTSP'
   # 应该返回: "rtsp://服务器IP:15544/live/stream_1"
   ```

**已知问题修复**（v1.2.0+）：
- 添加了所有配置字段的 `mapstructure` 标签
- 确保 viper 正确映射 `[rtspconfig]` 到 `RtspConfig` 结构体
- 端口号现在正确包含在转发地址中

---

## API参考

### 获取播放地址 API

**Endpoint**: `GET /api/v1/live/playurl/:id`

**参数**:
- `id` (路径参数): 直播流ID

**返回**:
```typescript
{
  info: {
    ID: number
    Name: string
    RTSP: string       // RTSP播放地址
    HttpFlv: string    // HTTP-FLV播放地址
    HttpHls: string    // HLS播放地址
    WsFlv: string      // WebSocket-FLV播放地址
    WEBRTC: string     // WebRTC播放地址
  }
}
```

**错误码**:
- `400`: 参数错误或流不存在
- `500`: 服务器内部错误

---

## 总结

抽帧插件通过以下方式获取RTSP地址：

1. **推荐方式**：从直播列表选择 → 自动获取转发地址
2. **备用方式**：手动输入RTSP地址（用于特殊场景）

转发地址格式：`rtsp://服务器IP:554/live/stream_流ID`

这种设计确保了系统的稳定性、安全性和可维护性。

