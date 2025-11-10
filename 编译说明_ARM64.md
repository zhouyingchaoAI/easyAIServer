# EasyDarwin ARM64编译说明

## 🎯 系统架构识别

你的系统：**aarch64 (ARM64)**

## ✅ 正确的编译命令

### 推荐方式1：ARM64专用编译
```bash
make build/arm64
```

**输出**：`/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-{日期时间}/`

### 推荐方式2：自动检测架构
```bash
make build/local
```

**输出**：根据当前系统自动选择架构编译

### ❌ 不要使用
```bash
make build/linux  # ← 强制编译amd64，不适合ARM系统
```

---

## 📁 编译产物

### 最新编译版本
```
/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511030747/
├── easydarwin.com  # 主程序 (ARM64)
├── ffmpeg          # FFmpeg (ARM64)
├── configs/        # 配置文件
├── web/            # Web界面
├── start.sh        # 启动脚本
└── stop.sh         # 停止脚本
```

### 架构验证
```bash
# EasyDarwin: 267 = ARM aarch64 ✓
# FFmpeg:     267 = ARM aarch64 ✓
```

---

## 🆕 本次编译包含的功能优化

### 1. 算法服务自动注册
- ✅ 详细的注册成功日志
- ✅ 显示当前总服务数
- ✅ 记录版本信息

### 2. 服务断开自动清理
- ✅ 每30秒自动检测心跳
- ✅ 90秒无心跳自动注销
- ✅ 详细的清理日志（含心跳年龄）
- ✅ 清理后统计报告

### 3. 健康状态报告（新增）
- ✅ 每5分钟自动输出
- ✅ 显示所有服务状态
- ✅ 任务类型分布统计
- ✅ 调用次数统计

### 4. 注销回调机制（新增）
- ✅ 服务下线触发回调
- ✅ 记录下线原因
- ✅ 可扩展的处理逻辑

---

## 🚀 启动新版本

### 步骤1：停止旧版本
```bash
pkill easydarwin
```

### 步骤2：启动新版本
```bash
cd /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511030747
./easydarwin.com
```

### 步骤3：验证功能

#### 查看启动日志
```bash
tail -f logs/sugar.log | grep "heartbeat checker started"
```

**预期输出**：
```
algorithm service heartbeat checker started
  check_interval_sec=30
  timeout_sec=90
  health_report_interval_min=5
```

#### 注册一个测试服务
```bash
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "test_service",
    "name": "测试服务",
    "task_types": ["人数统计"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
  }'
```

**预期日志**：
```
algorithm service registered successfully
  service_id=test_service
  endpoint=http://localhost:8000/infer
  total_services=1
```

#### 等待5分钟查看健康报告
```bash
tail -f logs/sugar.log | grep "health report"
```

**预期输出**：
```
algorithm services health report
  total_services=5
  task_type_distribution={...}

  service status
    service_id=test_service
    heartbeat_age_sec=30
    call_count=0
```

---

## 📊 监控关键日志

### 启动时
```
algorithm service heartbeat checker started
```

### 服务注册时
```
algorithm service registered successfully
  service_id=xxx
  total_services=5
```

### 服务清理时
```
algorithm service expired - auto removing
  service_id=xxx
  heartbeat_age_sec=95

algorithm service offline
  service_id=xxx
  reason=heartbeat_timeout
```

### 每5分钟
```
algorithm services health report
  total_services=5
```

---

## 🔧 配置说明

### AI分析配置
文件：`configs/config.toml`

```toml
[ai_analysis]
enable = true                    # 启用AI分析
heartbeat_timeout_sec = 90       # 心跳超时时间（秒）
scan_interval_sec = 5            # MinIO扫描间隔
max_concurrent_infer = 100       # 最大并发推理
```

**关键参数**：
- `heartbeat_timeout_sec`: 控制自动清理的超时时间
  - 设置90秒 = 允许丢失2次心跳（30秒间隔）
  - 可根据网络情况调整

---

## 📖 相关文档

- 平台功能优化总结：`/code/EasyDarwin/平台功能优化总结.md`
- 算法服务集成指南：`/code/EasyDarwin/doc/ALGORITHM_SERVICE_INTEGRATION_GUIDE.md`

---

**编译时间**：2025-11-03 07:47  
**版本**：v8.3.3  
**架构**：aarch64 (ARM64)  
**状态**：✅ 包含所有功能优化

