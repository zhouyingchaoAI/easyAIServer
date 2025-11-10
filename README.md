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
# 安装前端依赖并构建
make build/local

# 启动后端服务
./build/easydarwin -conf ./configs/config.toml
```

构建完成后，日志会提示服务端口；默认 Web 控制台监听在 `http://127.0.0.1:10086`。
如需直接使用预编译版本，可运行 `deploy/easydarwin`（对应平台）并保持 `configs/` 在同一级目录。

### 访问地址

- **Web 管理界面**: http://localhost:10086
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
├── build/                 # 本地构建产物（make build/*）
├── cmd/                   # Go 服务入口
├── configs/               # 主配置（config.toml 等）
├── deploy/                # 预打包的发行版与脚本
├── doc/
│   ├── reports/           # 历史修复与排查报告
│   └── *.md               # 功能/特性文档
├── examples/              # 示例算法/脚本
├── scripts/
│   ├── deploy/            # 发布相关脚本
│   ├── diagnostics/       # 诊断排查脚本
│   └── maintenance/       # 日常维护脚本
├── tests/                 # 手动/集成测试脚本
├── web/                   # 前端编译结果
├── web-src/               # 前端源码
└── README.md
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
curl http://localhost:10086/api/performance/stats

# 查看算法服务
curl http://localhost:10086/api/ai/services

# 实时日志
tail -f logs/sugar.log
```

## 📖 文档

- [README_CN.md](README_CN.md) - 中文总览
- [doc/PROJECT_INTRO_CN.md](doc/PROJECT_INTRO_CN.md) - 平台介绍
- [doc/FRAME_EXTRACTOR.md](doc/FRAME_EXTRACTOR.md) - 抽帧模块说明
- [doc/SMART_INFERENCE_USAGE.md](doc/SMART_INFERENCE_USAGE.md) - 智能推理使用手册
- [doc/reports/](doc/reports/) - 历史修复报告与上线总结

## 🛠️ 辅助工具

| 场景 | 脚本 |
| --- | --- |
| MinIO 连接测试 | `scripts/test_minio.sh` |
| 算法服务演示 | `scripts/demo_multi_services.sh` |
| 一键启动（示例） | `scripts/一键启动.sh` |
| 告警排查工具 | `scripts/diagnostics/check_alert_issue.py` |
| 清理抽帧冗余 | `scripts/diagnostics/cleanup_old_frames.py` |
| 配置迁移与修复 | `scripts/maintenance/` 下的脚本 |
| 快速部署发布 | `scripts/deploy/deploy_new_version.sh` |

## 🔧 常见问题

### 端口被占用

```bash
lsof -i :10086   # 检查Web端口
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
