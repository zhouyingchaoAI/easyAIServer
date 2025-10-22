# 算法服务对接文档 - 总览

**目标读者**: AI算法开发者、第三方算法服务提供商  
**最后更新**: 2025-10-20

---

## 📚 文档导航

### 🚀 快速开始

**如果您是第一次对接，请按顺序阅读：**

1. **[快速参考卡片](./ALGORITHM_INTEGRATION_QUICK_REFERENCE.md)** ⭐
   - 3步完成对接
   - API速查表
   - 最小实现示例
   - **阅读时间**: 5分钟

2. **[示例代码](./algorithm_service_example.py)** ⭐
   - 可直接运行的Python示例
   - 完整的注册和心跳实现
   - Mock算法实现
   - **上手时间**: 10分钟

3. **[完整对接指南](./ALGORITHM_SERVICE_INTEGRATION_GUIDE.md)**
   - 详细的接口说明
   - 多种语言示例
   - 调试和故障排查
   - **深入了解**: 30分钟

---

## 🎯 核心概念

### 系统架构

```
┌──────────────────┐
│   EasyDarwin     │  ← 主系统（已运行）
│   主服务系统      │
└────────┬─────────┘
         │ HTTP通信
         │
         ↓
┌──────────────────┐
│  您的算法服务     │  ← 需要您实现
│  - 注册          │
│  - 心跳          │
│  - 推理接口       │
└──────────────────┘
```

### 工作流程

```
1. 启动 → 注册服务
2. 心跳 → 保持在线
3. 接收 → 推理请求
4. 处理 → 返回结果
5. EasyDarwin → 保存告警
```

---

## 📋 3个必须实现的接口

### 1. 注册接口（调用EasyDarwin）

```http
POST http://10.1.6.230:5066/api/v1/ai_analysis/register

{
  "service_id": "唯一ID",
  "name": "服务名称",
  "task_types": ["绊线人数统计"],
  "endpoint": "http://your-ip:8000/infer",
  "version": "1.0.0"
}
```

### 2. 推理接口（您实现）

```http
POST http://your-ip:8000/infer

收到请求:
{
  "image_url": "http://...",
  "algo_config": {...},
  "algo_config_url": "http://..."
}

返回响应:
{
  "success": true,
  "result": {...},
  "confidence": 0.9,
  "inference_time_ms": 85
}
```

### 3. 心跳接口（调用EasyDarwin）

```http
POST http://10.1.6.230:5066/api/v1/ai_analysis/heartbeat/{service_id}

每45秒发送一次
```

---

## 💻 快速开始（5分钟）

### 步骤1: 下载示例代码

```bash
# 代码已在仓库中
cp algorithm_service_example.py my_algorithm_service.py
```

### 步骤2: 安装依赖

```bash
pip install flask requests opencv-python numpy
```

### 步骤3: 修改配置

```python
# 编辑 my_algorithm_service.py

# 修改EasyDarwin地址
EASYDARWIN_URL = "http://10.1.6.230:5066"  # 改为实际地址

# 修改服务端口
SERVICE_PORT = 8000  # 改为未占用的端口

# 修改支持的任务类型
TASK_TYPES = ["绊线人数统计", "人数统计"]
```

### 步骤4: 运行服务

```bash
python my_algorithm_service.py

# 预期输出:
# ✅ 服务注册成功!
# 🚀 算法服务已启动
# 📡 等待推理请求...
```

### 步骤5: 测试

```bash
# 在EasyDarwin前端:
# 1. 创建任务 → 类型选"绊线人数统计"
# 2. 配置检测线
# 3. 启动任务

# 在算法服务终端应该看到:
# 📨 收到推理请求
# ✅ 推理完成
```

---

## 📊 推理请求和响应

### 请求格式（您会收到）

```json
{
  "image_url": "http://10.1.6.230:9000/images/绊线人数统计/公司入口统计/20251020-094708.979.jpg?X-Amz-...",
  "task_id": "公司入口统计",
  "task_type": "绊线人数统计",
  "image_path": "绊线人数统计/公司入口统计/20251020-094708.979.jpg",
  "algo_config": {
    "regions": [
      {
        "type": "line",
        "points": [[100, 300], [700, 300]],
        "properties": {
          "direction": "in"  // "in"|"out"|"in_out"
        }
      }
    ],
    "algorithm_params": {
      "confidence_threshold": 0.7,
      "iou_threshold": 0.5
    }
  },
  "algo_config_url": "http://10.1.6.230:9000/images/.../algo_config.json?..."
}
```

### 响应格式（您需要返回）

```json
{
  "success": true,
  "result": {
    "total_count": 2,
    "detections": [...],
    "crossings": [...]
  },
  "confidence": 0.915,
  "inference_time_ms": 85
}
```

---

## 🎯 方向配置说明

### direction 字段含义

```
"in"     → ⬇ 进入（从上方穿过线条到下方）
"out"    → ⬆ 离开（从下方穿过线条到上方）  
"in_out" → ⬍ 双向（任意方向穿过线条）
```

### 视觉示意

```
进入 (in):
      ↓
  ═══════════
  
离开 (out):
  ═══════════
      ↑

双向 (in_out):
   ↓     ↑
  ═══════════
```

### 代码实现建议

```python
def check_crossing(track, line, direction):
    """
    检查穿越
    
    Args:
        track: 人员轨迹
        line: 检测线坐标 [[x1,y1], [x2,y2]]
        direction: "in"|"out"|"in_out"
    """
    # 计算穿越方向
    cross_dir = calculate_cross_direction(track, line)
    
    # 匹配配置
    if direction == "in_out":
        return True  # 双向都算
    elif direction == "in" and cross_dir == "down":
        return True
    elif direction == "out" and cross_dir == "up":
        return True
    
    return False
```

---

## 🔧 完整实现建议

### Python技术栈

```
推荐组合:
- Flask/FastAPI (Web框架)
- OpenCV (图像处理)
- YOLOv8 (目标检测)
- DeepSORT (多目标跟踪)
- NumPy (数值计算)
```

### Go技术栈

```
推荐组合:
- Gin (Web框架)
- GoCV (OpenCV绑定)
- 或调用Python算法
```

---

## 📖 详细文档索引

### 对接相关

| 文档 | 内容 | 适用 |
|------|------|------|
| **ALGORITHM_INTEGRATION_QUICK_REFERENCE.md** | 快速参考 | 快速上手 |
| **ALGORITHM_SERVICE_INTEGRATION_GUIDE.md** | 完整指南 | 详细对接 |
| **algorithm_service_example.py** | 示例代码 | 直接使用 |

### 功能相关

| 文档 | 内容 |
|------|------|
| **TRIPWIRE_COUNTING_ALGORITHM.md** | 绊线统计算法说明 |
| **LINE_DIRECTION_PERPENDICULAR_ARROWS.md** | 线条方向配置 |
| **INFERENCE_CONFIG_URL_FEATURE.md** | 配置URL功能 |

### 系统相关

| 文档 | 内容 |
|------|------|
| **ALL_FEATURES_SUMMARY.md** | 所有功能汇总 |
| **使用说明.txt** | 系统使用说明 |

---

## ⚡ 常见场景

### 场景1: 只做检测，不做跟踪

```python
def simple_detection(image, config):
    """简单检测（不跟踪）"""
    persons = detect_persons(image)
    
    return {
        "total_count": len(persons),
        "detections": persons,
        "avg_confidence": avg(persons)
    }
```

### 场景2: 使用配置文件URL

```python
def infer(data):
    # 优先使用algo_config
    config = data.get('algo_config')
    
    # 备用：下载配置文件
    if not config and data.get('algo_config_url'):
        config = requests.get(data['algo_config_url']).json()
    
    # 执行推理
    return run_inference(image, config)
```

### 场景3: 批量处理

```python
# 累积请求，批量推理
batch_queue = []

@app.route('/infer', methods=['POST'])
def infer():
    batch_queue.append(request.json)
    
    if len(batch_queue) >= BATCH_SIZE:
        results = batch_inference(batch_queue)
        batch_queue.clear()
        return results[0]  # 返回当前请求的结果
```

---

## 🐛 故障排查

| 问题 | 检查方法 | 解决方案 |
|------|---------|---------|
| 注册失败 | `curl http://10.1.6.230:5066/api/v1/ai_analysis/services` | 检查EasyDarwin状态 |
| 收不到请求 | 查看EasyDarwin日志 | 检查task_types匹配 |
| 图片下载失败 | 复制URL到浏览器测试 | 检查网络/URL有效性 |
| 心跳失败 | 查看服务日志 | 检查service_id正确性 |

---

## 📞 获取帮助

### 查看日志

```bash
# EasyDarwin日志
tail -f logs/sugar.log | grep "推理请求\|算法服务"

# 您的服务日志
python algorithm_service.py 2>&1 | tee service.log
```

### 测试工具

```bash
# 测试注册
curl -X POST http://10.1.6.230:5066/api/v1/ai_analysis/register \
  -H "Content-Type: application/json" \
  -d @service_info.json

# 查看已注册服务
curl http://10.1.6.230:5066/api/v1/ai_analysis/services | jq

# 测试推理接口
curl -X POST http://localhost:8000/infer \
  -H "Content-Type: application/json" \
  -d @test_request.json
```

### 技术支持

- **GitHub**: https://github.com/EasyDarwin/EasyDarwin
- **Issues**: 提交问题和建议
- **文档**: 持续更新中

---

## ✅ 对接清单

### 开发阶段

```
□ 阅读快速参考文档
□ 下载示例代码
□ 修改配置
□ 实现算法逻辑
□ 本地测试推理接口
```

### 集成阶段

```
□ 确认EasyDarwin运行中
□ 注册服务成功
□ 心跳正常发送
□ 能接收推理请求
□ 配置文件能正确解析
□ 推理结果格式正确
```

### 验证阶段

```
□ 创建测试任务
□ 配置检测区域
□ 启动任务
□ 查看推理日志
□ 检查告警生成
□ 验证统计准确性
```

---

## 🎓 学习路径

### 新手路径（0-1天）

```
1. 快速参考卡片（5分钟）
   ↓
2. 运行示例代码（30分钟）
   ↓
3. 测试注册和心跳（1小时）
   ↓
4. 替换为简单算法（2小时）
```

### 进阶路径（1-3天）

```
1. 阅读完整对接指南
   ↓
2. 实现真实检测算法
   ↓
3. 实现轨迹跟踪
   ↓
4. 实现绊线判断
   ↓
5. 性能优化
```

### 专家路径（3-7天）

```
1. 深入理解系统架构
   ↓
2. 实现多种算法类型
   ↓
3. 批量推理优化
   ↓
4. 分布式部署
   ↓
5. 监控和告警
```

---

## 📊 当前支持的算法类型

```
✅ 人数统计
✅ 绊线人数统计 ⭐ (推荐优先支持)
✅ 人员跌倒
✅ 人员离岗
✅ 吸烟检测
✅ 区域入侵
✅ 徘徊检测
✅ 物品遗留
✅ 安全帽检测
```

**建议**: 先实现1-2种算法类型，验证流程正常后再扩展

---

## 🎯 推荐实现顺序

### 第1阶段: 基础对接

```
1. 实现简单的推理接口
   - 能下载图片
   - 能返回固定结果
   
2. 实现服务注册
   - 启动时注册
   - 退出时注销
   
3. 实现心跳机制
   - 定时发送心跳
```

### 第2阶段: 算法实现

```
4. 实现目标检测
   - 加载检测模型
   - 解析检测结果
   
5. 解析配置文件
   - 读取检测线
   - 读取算法参数
   
6. 基础统计逻辑
   - 统计检测数量
   - 计算置信度
```

### 第3阶段: 高级功能

```
7. 轨迹跟踪
   - 多目标跟踪
   - ID分配和维护
   
8. 绊线判断
   - 线段交叉检测
   - 方向判断
   
9. 性能优化
   - GPU加速
   - 批量推理
   - 缓存优化
```

---

## 💡 重要提示

### ⚠️ 必须注意

```
1. image_url 有效期1小时，及时下载
2. 推理超时设置30秒，需快速响应
3. success=true 时才会保存告警
4. total_count 用于显示检测数量
5. 心跳超时90秒会被标记离线
```

### ✅ 最佳实践

```
1. 优先使用 algo_config（已解析）
2. algo_config_url 作为备用或调试
3. 实现错误重试机制
4. 记录详细的处理日志
5. 监控服务性能指标
```

---

## 🔗 快速链接

### 核心文档

- [快速参考](./ALGORITHM_INTEGRATION_QUICK_REFERENCE.md) - 5分钟上手
- [完整指南](./ALGORITHM_SERVICE_INTEGRATION_GUIDE.md) - 深入学习
- [示例代码](./algorithm_service_example.py) - 直接运行

### 算法功能

- [绊线统计](./TRIPWIRE_COUNTING_ALGORITHM.md) - 算法详细说明
- [线条方向](./LINE_DIRECTION_PERPENDICULAR_ARROWS.md) - 方向配置
- [配置URL](./INFERENCE_CONFIG_URL_FEATURE.md) - URL功能

### 系统说明

- [功能汇总](./ALL_FEATURES_SUMMARY.md) - 所有功能一览
- [使用说明](./使用说明.txt) - 系统基础使用

---

## 📞 技术支持

### 问题优先级

**P0 - 紧急**:
- 服务无法注册
- 推理接口报错
- 系统无响应

**P1 - 重要**:
- 配置解析问题
- 性能瓶颈
- 结果不准确

**P2 - 一般**:
- 功能优化建议
- 文档改进
- 新功能需求

### 反馈渠道

- **GitHub Issues**: 功能问题和Bug
- **文档补充**: PR欢迎
- **技术交流**: 开发者社区

---

## 🎉 开始对接

**准备好了吗？**

1. ✅ 阅读快速参考（5分钟）
2. ✅ 运行示例代码（10分钟）
3. ✅ 实现您的算法（根据复杂度）
4. ✅ 测试验证（1小时）

**祝您对接顺利！** 🚀

如有问题，请查看详细文档或联系技术支持。

---

**版本**: v1.0  
**更新**: 2025-10-20  
**维护**: EasyDarwin Team



