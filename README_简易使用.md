# yanying 平台 - 简易使用指南

## 快速启动

```bash
# 直接运行
./easydarwin

# 或使用启动脚本
./START.sh
```

## 默认配置

服务将使用 `configs/config.toml` 中的配置自动启动。

### 访问地址
- Web 界面: http://localhost:10008
- RTSP 端口: 15544
- RTMP 端口: 11935

### 默认账号
- 用户名: admin
- 密码: admin (首次登录后请修改)

## 配置文件

主配置文件位于: `configs/config.toml`

关键配置项：
```toml
[frame_extractor]
enable = true          # 启用抽帧
interval_ms = 200      # 抽帧间隔（每秒5帧）

[ai_analysis]
enable = true          # 启用智能分析
max_concurrent_infer = 50  # 最大并发推理数
```

## 查看日志

```bash
tail -f logs/sugar.log
```

## 性能监控

```bash
# 查看性能统计
curl http://localhost:10008/api/performance/stats

# 查看算法服务
curl http://localhost:10008/api/ai/services
```

## 停止服务

```bash
# 按 Ctrl+C 停止
# 或使用 pkill
pkill -f easydarwin
```

## 问题排查

### 1. 端口占用
```bash
lsof -i :10008
lsof -i :15544
```

### 2. MinIO 连接测试
```bash
./test_minio.sh
```

### 3. 查看实时日志
```bash
tail -f logs/sugar.log
```

## 高级功能

详细文档请参考：
- 智能推理使用: `doc/SMART_INFERENCE_USAGE.md`
- 技术设计: `doc/SMART_INFERENCE_STRATEGY.md`
- 中文文档: `README_CN.md`

---
简单易用，开箱即用 🚀
