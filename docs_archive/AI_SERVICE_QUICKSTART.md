# yanying AI服务快速入门

## ✅ 问题已解决

您的AI服务现在已经成功注册并可以被发现了！

## 📊 当前已注册的服务

目前系统中有 **5个** AI算法服务：

| 服务ID | 服务名称 | 支持的任务类型 | 推理端点 |
|--------|---------|---------------|---------|
| people_counter | 人数统计服务 | 人数统计、客流分析 | http://localhost:8001/infer |
| helmet_detector | 安全帽检测服务 | 安全帽检测、施工安全 | http://localhost:8002/infer |
| fall_detector | 跌倒检测服务 | 人员跌倒、老人监护 | http://localhost:8003/infer |
| smoke_detector | 吸烟检测服务 | 吸烟检测、禁烟区监控 | http://localhost:8004/infer |
| test_service_001 | 测试算法服务 | 人数统计、人员跌倒 | http://localhost:8000/infer |

## 🌐 Web界面查看

打开浏览器访问：

```
http://localhost:5066/#/ai-services
```

或者

```
http://10.1.4.246:5066/#/ai-services
```

您将看到所有已注册的算法服务及其状态。

## 🔧 API调用示例

### 1. 查询所有服务

```bash
curl http://localhost:5066/api/v1/ai_analysis/services
```

### 2. 手动注册新服务

```bash
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d '{
    "service_id": "my_custom_service",
    "name": "我的自定义服务",
    "task_types": ["人数统计", "行为分析"],
    "endpoint": "http://your-server:8000/infer",
    "version": "1.0.0"
  }'
```

### 3. 发送心跳

```bash
curl -X POST http://localhost:5066/api/v1/ai_analysis/heartbeat/my_custom_service
```

### 4. 查询告警

```bash
curl http://localhost:5066/api/v1/ai_analysis/alerts
```

## 🚀 使用演示脚本

我已经为您创建了几个演示脚本：

### 单服务演示

```bash
cd /code/EasyDarwin
./demo_ai_service.sh
```

这将注册一个演示服务并保持心跳运行。

### 多服务演示（推荐）

```bash
cd /code/EasyDarwin
./demo_multi_services.sh
```

这将注册4个不同类型的算法服务，模拟真实的多算法协同场景。

## 📝 完整工作流程

### 步骤1：启动yanying平台

```bash
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350
./easydarwin
```

### 步骤2：创建抽帧任务

访问Web界面：http://localhost:5066/#/frame-extractor

1. 点击"新增抽帧任务"
2. 选择任务类型：人数统计
3. 填写RTSP地址
4. 设置抽帧间隔：5000ms（5秒）
5. 点击"启动抽帧"

### 步骤3：注册算法服务

运行演示脚本：

```bash
cd /code/EasyDarwin
./demo_multi_services.sh
```

或者使用您自己的算法服务（Python示例）：

```python
import requests
import time

# 注册服务
response = requests.post('http://localhost:5066/api/v1/ai_analysis/register', json={
    'service_id': 'my_yolo_service',
    'name': 'YOLO人数统计',
    'task_types': ['人数统计'],
    'endpoint': 'http://localhost:8000/infer',
    'version': '1.0.0'
})
print(f"注册结果: {response.json()}")

# 保持心跳
while True:
    time.sleep(30)
    response = requests.post('http://localhost:5066/api/v1/ai_analysis/heartbeat/my_yolo_service')
    print(f"心跳: {response.json()}")
```

### 步骤4：查看结果

1. **查看算法服务**：http://localhost:5066/#/ai-services
2. **查看告警结果**：http://localhost:5066/#/alerts
3. **查看快照图库**：http://localhost:5066/#/frame-extractor/gallery

## 🔍 故障排查

### 问题1：服务列表为空

**原因**：
- yanying平台未启动
- AI分析插件未启用

**解决方案**：

1. 检查配置文件 `configs/config.toml`：

```toml
[ai_analysis]
enable = true  # 必须为 true
```

2. 重启yanying服务

### 问题2：心跳失败（400错误）

**原因**：服务ID不存在或未注册

**解决方案**：
1. 先注册服务
2. 使用正确的service_id发送心跳

### 问题3：MinIO连接错误

**原因**：MinIO配置不正确或MinIO未启动

**解决方案**：

1. 检查MinIO配置：

```toml
[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
access_key = 'admin'
secret_key = 'admin123'
bucket = 'images'
```

2. 测试MinIO连接：

```bash
curl http://10.1.6.230:9000
```

3. 确保bucket存在：

```bash
# 使用mc工具
mc alias set myminio http://10.1.6.230:9000 admin admin123
mc mb myminio/images
```

## 📚 相关文档

- [AI分析插件完整文档](doc/AI_ANALYSIS.md)
- [任务类型说明](doc/TASK_TYPES.md)
- [抽帧插件文档](doc/FRAME_EXTRACTOR.md)
- [部署指南](doc/DEPLOYMENT_GUIDE_CN.md)

## 🎯 下一步

1. ✅ **算法服务已注册** - 5个演示服务正在运行
2. 📸 **配置抽帧任务** - 从摄像头抽取图片
3. 🤖 **等待AI分析** - 系统会自动调度推理
4. 📊 **查看告警结果** - 在Web界面查看分析结果

## 💡 提示

- 心跳间隔建议：30秒
- 心跳超时时间：90秒（可配置）
- 服务注册后立即可用，无需重启平台
- 支持动态增减算法服务

## 📞 获取帮助

如有问题，请查看：

1. 平台日志：`/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350/logs/`
2. 项目文档：`/code/EasyDarwin/doc/`
3. GitHub Issues

---

**现在您的AI服务已经可以正常工作了！** 🎉

