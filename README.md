# yanying（燕影）智能视频分析平台

<p align="center">
  <img src="web-src/public/swallow.svg" width="120" alt="yanying logo">
</p>

<p align="center">
  <strong>开箱即用的智能视频分析平台</strong>
</p>

<p align="center">
  流媒体服务 + 视频抽帧 + AI智能分析 = 一体化解决方案
</p>

---

## 🚀 快速启动

```bash
./easydarwin
```

就这么简单！

### 访问地址

- **Web 管理界面**: http://localhost:10008
- **默认账号**: admin / admin

## 📋 功能特性

### 流媒体服务
- ✅ RTSP/RTMP/HLS/HTTP-FLV/WebRTC 多协议支持
- ✅ 低延迟推拉流
- ✅ 多路视频同时处理
- ✅ Web 实时预览

### 智能抽帧
- ✅ 可配置抽帧频率（默认每秒5帧）
- ✅ MinIO 对象存储
- ✅ 支持8种任务类型：人数统计、人员跌倒、人员离岗、吸烟检测、区域入侵、徘徊检测、物品遗留、安全帽检测

### AI 智能分析
- ✅ **智能自适应推理**：自动匹配抽帧与推理速度
- ✅ **智能队列管理**：防止存储爆满，自动丢弃策略
- ✅ **性能监控告警**：推理慢告警、高丢弃率告警
- ✅ 算法服务注册与发现
- ✅ 心跳检测与故障转移
- ✅ 支持 Kafka 告警推送
- ✅ 每秒处理 5 张图片

## 📦 项目结构

```
yanying/
├── easydarwin          # 主程序（直接运行）
├── START.sh            # 启动脚本（可选）
├── configs/            # 配置文件
│   └── config.toml     # 主配置
├── scripts/            # 辅助脚本
│   ├── 一键启动.sh
│   ├── test_minio.sh
│   └── ...
├── web/                # 前端资源
├── doc/                # 技术文档
└── 使用说明.txt        # 快速参考

```

## ⚙️ 配置说明

主配置文件：`configs/config.toml`

### 关键配置

```toml
# 抽帧配置
[frame_extractor]
enable = true
interval_ms = 200      # 每秒5帧

# 智能分析配置
[ai_analysis]
enable = true
scan_interval_sec = 1
max_concurrent_infer = 50
```

## 📊 性能监控

```bash
# 查看性能统计
curl http://localhost:10008/api/performance/stats

# 查看算法服务
curl http://localhost:10008/api/ai/services

# 实时日志
tail -f logs/sugar.log
```

## 📖 文档

- [使用说明.txt](使用说明.txt) - 快速参考
- [README_简易使用.md](README_简易使用.md) - 简易使用指南
- [README_CN.md](README_CN.md) - 完整中文文档
- [智能推理使用指南](doc/SMART_INFERENCE_USAGE.md) - 详细的智能推理文档

## 🛠️ 辅助工具

```bash
# MinIO 连接测试
scripts/test_minio.sh

# 算法服务演示
scripts/demo_multi_services.sh

# 完整自动配置启动
scripts/一键启动.sh
```

## 🔧 常见问题

### 端口被占用

```bash
lsof -i :10008   # 检查Web端口
lsof -i :15544   # 检查RTSP端口
```

### MinIO 连接问题

```bash
scripts/test_minio.sh  # 测试连接
```

### 查看实时日志

```bash
tail -f logs/sugar.log
```

## 📈 性能指标

- 🎯 **抽帧性能**: 每秒 5 帧
- 🎯 **推理性能**: 每秒 5 张图片
- 🎯 **并发能力**: 最大 50 并发推理
- 🎯 **队列容量**: 100 张智能队列
- 🎯 **丢弃率**: < 5% （健康状态）

## 🏗️ 技术架构

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│  视频源      │────▶│ EasyDarwin   │────▶│  Web 展示      │
│  RTSP/RTMP  │     │  流媒体服务   │     │                │
└─────────────┘     └──────────────┘     └────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ 抽帧插件      │
                    │ Frame Extract│
                    └──────────────┘
                           │
                           ▼ MinIO
                    ┌──────────────┐
                    │ 智能分析插件  │
                    │ AI Analysis  │
                    └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ 算法服务      │
                    │ AI Service   │
                    └──────────────┘
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

本项目基于 EasyDarwin 开发，遵循相应开源协议。

---

<p align="center">
  <strong>简单易用 | 性能强大 | 智能分析</strong>
</p>

<p align="center">
  Made with ❤️ by yanying team
</p>
