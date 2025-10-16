# yanying 一键启动指南

## 🚀 三种启动方式

### 方式1：一键启动（最简单）⭐

直接使用默认配置启动所有服务：

```bash
cd /code/EasyDarwin
./一键启动.sh
```

**包含的功能**：
- ✅ 自动配置MinIO（权限、清理策略）
- ✅ 生成优化的配置文件
- ✅ 启动yanying服务
- ✅ 注册5个算法服务实例
- ✅ 启动心跳循环
- ✅ 验证运行状态

**默认配置**：
- 抽帧：5张/秒
- 扫描：每秒
- 并发：50个
- 清理：1天过期

---

### 方式2：配置向导（自定义配置）

交互式配置所有参数：

```bash
cd /code/EasyDarwin
./快速配置向导.sh
```

会引导您配置：
1. MinIO连接信息
2. 性能参数（1/5/10张/秒）
3. 视频源地址
4. 任务类型
5. 自动启动服务

---

### 方式3：手动配置

```bash
# 1. 编辑配置文件
vi /code/EasyDarwin/build/EasyDarwin-lin-*/configs/config.toml

# 2. 配置MinIO
/tmp/mc anonymous set public test-minio/images
/tmp/mc ilm add test-minio/images --expiry-days 1

# 3. 启动服务
cd /code/EasyDarwin/build/EasyDarwin-lin-*
./easydarwin &

# 4. 注册算法服务
# （手动执行注册API调用）
```

---

## 📋 启动后检查清单

### ✅ 服务状态

```bash
# 检查进程
ps aux | grep easydarwin

# 检查日志
tail -f /code/EasyDarwin/build/*/logs/20251016_08_00_00.log

# 检查API
curl http://localhost:5066/api/v1/health
```

### ✅ MinIO状态

```bash
# 查看存储
/tmp/mc du yanying-minio/images

# 查看图片
/tmp/mc ls yanying-minio/images --recursive | head -20

# 查看清理策略
/tmp/mc ilm ls yanying-minio/images
```

### ✅ AI服务状态

```bash
# 查看已注册服务
curl http://localhost:5066/api/v1/ai_analysis/services

# 查看告警
curl http://localhost:5066/api/v1/ai_analysis/alerts
```

---

## 🔧 配置参数说明

### 一键启动.sh 配置参数

在脚本开头可以修改这些参数：

```bash
# MinIO配置
MINIO_ENDPOINT="10.1.6.230:9000"  # MinIO地址
MINIO_ACCESS_KEY="admin"           # 用户名
MINIO_SECRET_KEY="admin123"        # 密码
MINIO_BUCKET="images"              # Bucket名称
RETENTION_DAYS=1                   # 保留天数

# 性能参数
FRAME_INTERVAL_MS=200   # 抽帧间隔（200=5张/秒）
SCAN_INTERVAL_SEC=1     # 扫描间隔
MAX_CONCURRENT=50       # 最大并发数
NUM_ALGO_INSTANCES=5    # 算法实例数

# RTSP配置
RTSP_URL="rtsp://127.0.0.1:15544/live/stream_2"
TASK_TYPE="人数统计"
TASK_ID="high_performance_task"
```

---

## 🎯 不同场景的配置

### 场景1：实时监控（人员跌倒、火灾）

```bash
# 编辑 一键启动.sh
FRAME_INTERVAL_MS=100   # 10张/秒
SCAN_INTERVAL_SEC=1
MAX_CONCURRENT=100
RETENTION_DAYS=1

# 运行
./一键启动.sh
```

### 场景2：标准监控（人数统计、客流）⭐

```bash
# 使用默认配置
FRAME_INTERVAL_MS=200   # 5张/秒
SCAN_INTERVAL_SEC=1
MAX_CONCURRENT=50
RETENTION_DAYS=1

./一键启动.sh
```

### 场景3：定期巡检（设备检查）

```bash
FRAME_INTERVAL_MS=10000  # 0.1张/秒
SCAN_INTERVAL_SEC=60
MAX_CONCURRENT=5
RETENTION_DAYS=30

./一键启动.sh
```

---

## 🛠️ 常用命令

### 启动相关

```bash
# 一键启动
./一键启动.sh

# 停止服务
pkill -9 easydarwin

# 重启服务
pkill -9 easydarwin && sleep 2 && cd build/EasyDarwin-lin-* && ./easydarwin &

# 查看状态
ps aux | grep easydarwin
```

### 监控相关

```bash
# 实时日志
tail -f build/*/logs/20251016_08_00_00.log | grep "found new"

# 性能统计
tail -n 200 build/*/logs/20251016_08_00_00.log | grep "found new" | wc -l

# 存储查看
/tmp/mc du yanying-minio/images

# 图片数量
/tmp/mc ls yanying-minio/images --recursive | wc -l
```

### MinIO管理

```bash
# 查看bucket
/tmp/mc ls yanying-minio

# 查看清理策略
/tmp/mc ilm ls yanying-minio/images

# 修改清理策略
/tmp/mc ilm remove yanying-minio/images --all
/tmp/mc ilm add yanying-minio/images --expiry-days 3

# 手动清理
/tmp/mc find yanying-minio/images --older-than 7d --exec "mc rm {}"
```

### AI服务管理

```bash
# 注册新服务
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "my_service",
    "name": "我的算法服务",
    "task_types": ["人数统计"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
  }'

# 查看所有服务
curl http://localhost:5066/api/v1/ai_analysis/services

# 发送心跳
curl -X POST http://localhost:5066/api/v1/ai_analysis/heartbeat/my_service
```

---

## ❓ 常见问题

### Q1: 启动失败怎么办？

**检查步骤**：
1. 查看日志：`tail -100 build/*/logs/20251016_08_00_00.log`
2. 检查端口：`lsof -i :5066`
3. 检查MinIO：`curl http://10.1.6.230:9000/minio/health/live`

### Q2: MinIO连接失败？

```bash
# 运行诊断
./debug_minio_502.sh

# 或切换到本地存储
# 编辑 config.toml
[frame_extractor]
store = 'local'
```

### Q3: 性能不够？

**提升性能**：
1. 增加并发数：`max_concurrent_infer = 100`
2. 使用GPU加速算法
3. 部署更多算法实例
4. 增加服务器CPU/内存

### Q4: 存储增长太快？

**降低存储**：
1. 增加抽帧间隔：`interval_ms = 1000`（1张/秒）
2. 缩短保留时间：`/tmp/mc ilm add ... --expiry-days 1`
3. 降低图片质量（在抽帧脚本中）

---

## 📚 相关文档

- [高性能配置方案.md](高性能配置方案.md) - 详细性能优化
- [优化配置建议.md](优化配置建议.md) - 配置建议
- [doc/OPTIMIZATION_STRATEGY.md](doc/OPTIMIZATION_STRATEGY.md) - 优化策略
- [性能达标报告.md](性能达标报告.md) - 性能验证

---

## 🎯 快速参考

### 一行命令启动

```bash
cd /code/EasyDarwin && ./一键启动.sh
```

### 交互式配置启动

```bash
cd /code/EasyDarwin && ./快速配置向导.sh
```

### 自定义启动

编辑 `一键启动.sh` 修改参数，然后运行。

---

<div align="center">

## 🎊 选择您的方式，立即开始！

**新手**: 使用配置向导 `./快速配置向导.sh`  
**快速**: 使用一键启动 `./一键启动.sh`  
**专家**: 手动配置

**访问系统**: http://localhost:5066

</div>

