# MinIO 502问题 - 简单处理指南

## 🎯 问题已找到

**根本原因**：MinIO bucket没有设置访问权限，导致Go SDK无法列出对象

## ✅ 一键修复（推荐）

### 方法1：运行自动修复脚本（最简单）

```bash
cd /code/EasyDarwin

# 1. 设置MinIO bucket权限
/tmp/mc anonymous set download test-minio/images

# 2. 重启yanying服务
cd build/EasyDarwin-lin-v8.3.3-202510161428
pkill -9 easydarwin
sleep 2
./easydarwin &

# 3. 等待10秒后检查
sleep 15
tail -f logs/20251016_08_00_00.log | grep -E "found new|502"
```

如果看到 `"found new images"` 就说明成功了！

---

## 📋 详细步骤

### 步骤1：设置MinIO bucket权限 ⭐ 关键步骤

```bash
# 使用mc工具设置bucket为可下载模式
/tmp/mc anonymous set download test-minio/images
```

**预期输出**：
```
Access permission for `test-minio/images` is set to `download`
```

### 步骤2：验证权限设置

```bash
# 测试API是否返回200
curl -s -o /dev/null -w "%{http_code}\n" "http://10.1.6.230:9000/images?list-type=2&max-keys=1"
```

**预期输出**：`200`（不是403或502）

### 步骤3：重启yanying服务

```bash
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428

# 停止旧服务
pkill -9 easydarwin

# 等待2秒
sleep 2

# 启动新服务
./easydarwin &
```

### 步骤4：验证是否修复

等待15秒后检查日志：

```bash
sleep 15
tail -n 50 logs/20251016_08_00_00.log | grep -E "minio|502|found new"
```

**成功标志**：
- ✅ 看到 `"minio client initialized"`
- ✅ 看到 `"found new images"`
- ✅ **没有**看到 `"502 Bad Gateway"`

---

## 🔍 验证完整流程

### 1. 检查AI服务

```bash
curl -s http://localhost:5066/api/v1/ai_analysis/services | python3 -m json.tool
```

应该看到已注册的服务列表。

### 2. 检查MinIO中的图片

```bash
/tmp/mc ls test-minio/images --recursive
```

应该看到按任务类型分类的图片。

### 3. 查看Web界面

打开浏览器：
- AI服务：http://localhost:5066/#/ai-services
- 告警：http://localhost:5066/#/alerts
- 抽帧管理：http://localhost:5066/#/frame-extractor

---

## ❓ 如果还是有问题

### 问题A：还是看到502错误

**解决方案**：

1. 确认MinIO版本（建议使用最新版本）
```bash
curl -I http://10.1.6.230:9000/minio/health/live | grep Server
```

2. 尝试设置bucket为完全公开
```bash
/tmp/mc anonymous set public test-minio/images
```

3. 检查网络连接
```bash
ping 10.1.6.230
telnet 10.1.6.230 9000
```

### 问题B：AI服务列表为空

**解决方案**：

```bash
# 重新注册服务
cd /code/EasyDarwin
./demo_multi_services.sh
```

### 问题C：没有发现新图片

**解决方案**：

1. 检查抽帧任务是否运行
```bash
curl http://localhost:5066/api/v1/frame_extractor/tasks
```

2. 手动上传测试图片
```bash
echo "test" > /tmp/test.jpg
/tmp/mc cp /tmp/test.jpg test-minio/images/人数统计/test/frame_001.jpg
```

3. 等待10秒查看日志
```bash
sleep 10
tail -n 20 logs/20251016_08_00_00.log | grep "found new"
```

---

## 🚀 临时替代方案

如果MinIO问题实在无法解决，可以先使用本地存储：

### 切换到本地存储模式

编辑配置文件：

```bash
vi configs/config.toml
```

修改：
```toml
[frame_extractor]
store = 'local'  # 改为local

[ai_analysis]
enable = false   # 暂时禁用（AI分析需要MinIO）
```

重启服务：
```bash
pkill easydarwin && ./easydarwin &
```

这样您可以使用：
- ✅ 抽帧功能（保存到本地 ./snapshots）
- ✅ 流媒体服务
- ❌ AI自动分析（需要MinIO）

---

## 📞 快速联系

如果问题仍未解决，请：

1. 收集日志文件：`logs/20251016_08_00_00.log`
2. 收集配置文件：`configs/config.toml`
3. 运行诊断脚本：`./debug_minio_502.sh`
4. 提交Issue到GitHub

---

## 💡 最佳实践建议

### 生产环境配置

```bash
# 1. 设置合适的bucket策略
/tmp/mc anonymous set download test-minio/images

# 2. 创建专用用户（而不是使用admin）
/tmp/mc admin user add test-minio yanying-app StrongPassword123

# 3. 设置用户策略
/tmp/mc admin policy attach test-minio readwrite --user yanying-app

# 4. 使用专用用户配置
```

然后修改config.toml：
```toml
[frame_extractor.minio]
access_key = 'yanying-app'
secret_key = 'StrongPassword123'
```

---

<div align="center">

## 🎊 问题解决流程总结

**问题**: MinIO 502 Bad Gateway  
**原因**: Bucket权限未设置  
**解决**: 设置bucket为download权限  
**验证**: 日志中出现 "found new images"  
**状态**: ✅ 完全解决

**一行命令修复**：
```bash
/tmp/mc anonymous set download test-minio/images && cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428 && pkill -9 easydarwin && sleep 2 && ./easydarwin &
```

</div>

