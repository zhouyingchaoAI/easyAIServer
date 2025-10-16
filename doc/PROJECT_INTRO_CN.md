# yanying 项目介绍

## 项目愿景

yanying致力于打造一个**开箱即用的视频智能分析平台**，让AI算法开发者、系统集成商、企业用户都能快速构建智能视频分析应用。

### 我们要解决的问题

#### 传统视频分析系统的痛点

1. **技术门槛高**
   - 需要深入了解流媒体协议（RTSP/RTMP/HLS等）
   - 视频解码、抽帧需要处理复杂的FFmpeg
   - 算法集成需要修改底层代码

2. **开发周期长**
   - 从零搭建流媒体服务器需要数周
   - 算法服务与平台耦合，难以复用
   - 每个新场景都需要重新开发

3. **部署运维难**
   - 多个组件独立部署，配置复杂
   - 缺乏统一的监控告警
   - 扩展性差，难以支持大规模部署

4. **算法接入难**
   - 算法服务与平台强耦合
   - 算法更新需要重启系统
   - 多算法融合困难

### yanying的解决方案

#### 1. 一体化平台

```
传统架构：
├── 流媒体服务器（Nginx-RTMP/SRS）
├── 视频抽帧服务（自己开发）
├── 算法推理服务（自己开发）
├── 存储服务（MinIO/OSS）
├── 消息队列（Kafka）
├── 数据库（MySQL）
└── Web界面（自己开发）

yanying架构：
└── yanying（单个可执行文件）
    ├── 内置流媒体服务器 ✓
    ├── 内置抽帧引擎 ✓
    ├── 内置AI调度引擎 ✓
    ├── 内置SQLite数据库 ✓
    ├── 内置Web界面 ✓
    └── 对接外部MinIO/Kafka（可选）
```

**优势**：
- ✅ 单个可执行文件，零依赖启动
- ✅ 5分钟完成基础部署
- ✅ 统一配置，统一管理

#### 2. 开放的算法生态

```
传统方式：算法与平台耦合
┌─────────────────────────┐
│      视频平台（Go）      │
│  ├─ 算法A（Go重写）      │
│  ├─ 算法B（Go重写）      │
│  └─ 算法C（Go重写）      │  ❌ 算法需要用Go重写
└─────────────────────────┘  ❌ 算法更新需要重启
                             ❌ 难以集成第三方算法

yanying方式：算法服务化
┌─────────────────────────┐
│     yanying平台（Go）    │
│  算法服务注册中心         │
└─────────────────────────┘
         ↑  ↑  ↑
    HTTP │  │  │ HTTP
         │  │  │
    ┌────┘  │  └────┐
    │       │       │
┌───────┐ ┌───────┐ ┌───────┐
│算法A   │ │算法B   │ │算法C   │
│Python │ │C++    │ │Java   │  ✅ 任意语言开发
│Port   │ │Port   │ │Port   │  ✅ 独立部署更新
│8001   │ │8002   │ │8003   │  ✅ 易于扩展
└───────┘ └───────┘ └───────┘
```

**优势**：
- ✅ 算法开发者用熟悉的语言和框架
- ✅ 算法服务独立部署，热插拔
- ✅ 第三方算法零门槛接入

#### 3. 智能化的任务分类

传统方式：所有图片混在一起
```
/snapshots/
  ├── camera1_001.jpg  ❓ 这是什么场景？
  ├── camera1_002.jpg  ❓ 用什么算法？
  ├── camera2_001.jpg  ❓ 参数怎么设置？
  └── ...
```

yanying方式：按任务类型自动分类
```
/snapshots/
  ├── 人数统计/
  │   ├── task_1/  ← 商场1F
  │   │   ├── frame_001.jpg
  │   │   └── frame_002.jpg
  │   └── task_2/  ← 商场2F
  ├── 安全帽检测/
  │   └── task_3/  ← 工地A区
  └── 人员跌倒/
      └── task_4/  ← 养老院
```

**优势**：
- ✅ 算法自动匹配任务类型
- ✅ 清晰的目录结构，便于管理
- ✅ 支持多算法并行处理同一类型

#### 4. 自动化的处理流程

```
传统方式：手动编排
┌──────────┐    ┌──────────┐    ┌──────────┐
│ 1.下载   │ -> │ 2.推理   │ -> │ 3.存储   │
│   图片   │    │   处理   │    │   结果   │
└──────────┘    └──────────┘    └──────────┘
     ↓               ↓               ↓
❌ 需要写代码     ❌ 需要写代码     ❌ 需要写代码

yanying方式：全自动
┌──────────────────────────────────────┐
│          yanying自动引擎              │
│                                       │
│  1. 扫描MinIO → 发现新图片            │
│  2. 识别任务类型 → 匹配算法服务        │
│  3. 调度推理 → 并发HTTP请求           │
│  4. 汇总结果 → 存储+推送              │
│  5. 重复循环（10秒间隔）               │
└──────────────────────────────────────┘
     ↓
✅ 零代码，配置即可
```

---

## 技术创新点

### 1. 插件化架构

yanying采用插件化设计，核心功能都以插件形式实现：

```go
// 插件接口
type Plugin interface {
    Name() string
    Init(config Config) error
    Start() error
    Stop() error
}

// 抽帧插件
type FrameExtractorPlugin struct {
    tasks map[string]*Task
    minio *MinIOClient
}

// AI分析插件
type AIAnalysisPlugin struct {
    registry *ServiceRegistry
    scheduler *InferenceScheduler
    scanner *MinIOScanner
}
```

**优势**：
- 核心系统与业务插件解耦
- 插件可独立开发、测试、部署
- 易于扩展新功能

### 2. 任务类型驱动

所有处理流程围绕"任务类型"展开：

```
任务类型 = 业务场景标识

抽帧任务创建 → 指定任务类型 → 图片按类型存储
                    ↓
算法服务注册 → 声明支持的任务类型
                    ↓
            自动匹配与调度
```

**优势**：
- 业务语义清晰
- 算法与场景自动匹配
- 便于业务扩展

### 3. 心跳机制

算法服务通过心跳保持活跃状态：

```
算法服务                 yanying平台
   │                        │
   │──── 注册 ────────────→│
   │                        │ 记录服务信息
   │←─── 200 OK ───────────│
   │                        │
   │──── 心跳(30s) ────────→│
   │                        │ 更新last_heartbeat
   │←─── 200 OK ───────────│
   │                        │
   │        ...             │
   │                        │
   │  ✗ 90秒无心跳          │
   │                        │ 自动注销服务
```

**优势**：
- 自动发现服务异常
- 避免调用失效服务
- 提高系统稳定性

### 4. 智能调度算法

```go
// 推理调度器
func (s *Scheduler) ScheduleInference(images []Image) {
    // 1. 按任务类型分组
    grouped := groupByTaskType(images)
    
    // 2. 为每个类型匹配算法服务
    for taskType, imgs := range grouped {
        services := s.registry.GetServicesByTaskType(taskType)
        
        // 3. 负载均衡（轮询/随机/最少连接）
        service := s.loadBalance(services)
        
        // 4. 并发控制
        s.semaphore.Acquire()
        go func() {
            defer s.semaphore.Release()
            // 5. HTTP调用推理
            result := s.callInference(service, imgs)
            // 6. 存储结果
            s.saveResult(result)
            // 7. 推送消息
            s.pushAlert(result)
        }()
    }
}
```

**优势**：
- 并发处理，提高吞吐量
- 负载均衡，充分利用资源
- 错误隔离，单个失败不影响整体

---

## 应用场景深度解析

### 场景1：智慧商场客流分析

#### 业务需求
- 统计各楼层、各区域实时客流
- 分析客流高峰时段
- 优化商品陈列位置

#### yanying解决方案

**第一步：部署摄像头**
```
商场1F入口 → RTSP摄像头1 → rtsp://10.1.1.1/stream
商场2F电梯口 → RTSP摄像头2 → rtsp://10.1.1.2/stream
商场3F美食区 → RTSP摄像头3 → rtsp://10.1.1.3/stream
```

**第二步：创建抽帧任务**
```toml
# 任务1：1F入口
task_type = "人数统计"
rtsp_url = "rtsp://10.1.1.1/stream"
interval_ms = 5000  # 5秒一帧

# 任务2：2F电梯口
task_type = "人数统计"
rtsp_url = "rtsp://10.1.1.2/stream"
interval_ms = 5000

# 任务3：3F美食区
task_type = "人数统计"
rtsp_url = "rtsp://10.1.1.3/stream"
interval_ms = 5000
```

**第三步：部署算法服务**
```bash
# 人数统计算法（使用YOLOv8）
python algorithm_service.py \
  --service-id people_counter_yolo \
  --task-types 人数统计 \
  --model yolov8n.pt \
  --port 8001
```

**第四步：自动分析**
```
yanying自动执行：
1. 每5秒从3个摄像头各抽取1帧
2. 图片保存到MinIO: 人数统计/task_1/、task_2/、task_3/
3. 每10秒扫描MinIO，发现新图片
4. 调用算法服务推理
5. 返回结果：{"task_1": 23人, "task_2": 15人, "task_3": 8人}
6. 存储到数据库 + 推送到Kafka
7. 前端实时显示客流数据
```

**第五步：数据分析**
```sql
-- 查询各楼层客流趋势
SELECT 
    task_id,
    DATE_FORMAT(created_at, '%H:00') as hour,
    AVG(person_count) as avg_people
FROM alerts
WHERE task_type = '人数统计'
  AND DATE(created_at) = CURDATE()
GROUP BY task_id, hour
ORDER BY task_id, hour;
```

#### 业务价值
- 📊 实时了解客流分布
- ⏰ 发现客流高峰规律
- 🏪 优化商品布局和促销策略
- 👥 合理安排人员配置

---

### 场景2：工地安全监控

#### 业务需求
- 检测工人是否佩戴安全帽
- 检测是否穿戴反光衣
- 危险区域入侵告警

#### yanying解决方案

**部署多种算法**
```bash
# 算法1：安全帽检测
python algorithm_service.py \
  --service-id helmet_detector \
  --task-types 安全帽检测 \
  --model helmet_yolo.pt \
  --port 8001

# 算法2：反光衣检测
python algorithm_service.py \
  --service-id vest_detector \
  --task-types 反光衣检测 \
  --model vest_yolo.pt \
  --port 8002

# 算法3：危险区域入侵
python algorithm_service.py \
  --service-id intrusion_detector \
  --task-types 区域入侵检测 \
  --model intrusion_model.pt \
  --port 8003
```

**创建混合任务**
```javascript
// 前端创建任务时可选择多个类型
{
  "task_name": "工地A区监控",
  "rtsp_url": "rtsp://10.2.1.1/stream",
  "task_types": [
    "安全帽检测",
    "反光衣检测",
    "区域入侵检测"
  ],
  "interval_ms": 3000
}
```

**自动告警**
```
检测结果示例：
┌─────────────────────────────────┐
│ 时间：2024-10-16 14:30:25       │
│ 位置：工地A区                    │
│ 告警：                           │
│  ⚠️  检测到3人未戴安全帽         │
│  ⚠️  检测到1人未穿反光衣         │
│  ⚠️  危险区域有人员入侵           │
│ 图片：[查看现场图片]             │
└─────────────────────────────────┘
```

---

### 场景3：养老院智能监护

#### 业务需求
- 老人跌倒立即告警
- 异常行为检测（长时间静止、徘徊）
- 活动轨迹分析

#### yanying解决方案

**高频抽帧配置**
```toml
[frame_extractor.tasks.elderly_care]
task_type = "人员跌倒"
rtsp_url = "rtsp://10.3.1.1/stream"
interval_ms = 1000  # 1秒1帧（跌倒检测需要高频）
```

**实时告警处理**
```python
# Kafka消费者（告警处理程序）
from kafka import KafkaConsumer
import requests

consumer = KafkaConsumer('easydarwin.alerts')

for message in consumer:
    alert = json.loads(message.value)
    
    if alert['task_type'] == '人员跌倒' and alert['alert']:
        # 1. 发送短信通知家属
        send_sms(alert['alert_message'])
        
        # 2. 推送到护工App
        push_notification(alert)
        
        # 3. 自动拨打急救电话（严重情况）
        if alert['confidence'] > 0.95:
            call_emergency(alert)
```

---

## 性能与扩展性

### 单机性能

**测试环境**：
- CPU: Intel Xeon E5-2680 v4 (14核28线程)
- 内存: 64GB DDR4
- 存储: SSD 500GB
- 网络: 千兆网卡

**测试结果**：
```
✅ 并发流处理：120路 RTSP流同时拉取
✅ 抽帧性能：每路1秒1帧，稳定运行24小时无丢帧
✅ AI推理调度：QPS 800+（调度开销，不含算法推理时间）
✅ Web API响应：P99 < 100ms
✅ 内存占用：基础500MB + 每路流10MB ≈ 1.7GB
✅ CPU占用：平均15%
```

### 横向扩展

#### 方案1：单机多实例

```bash
# 实例1：处理前50路摄像头
./easydarwin -conf config1.toml -port 10086

# 实例2：处理后50路摄像头
./easydarwin -conf config2.toml -port 10087

# Nginx负载均衡
upstream yanying {
    server 127.0.0.1:10086;
    server 127.0.0.1:10087;
}
```

#### 方案2：分布式部署

```
┌────────────────────────────────────────┐
│           Load Balancer (Nginx)         │
└────────────────────────────────────────┘
         │           │           │
    ┌────┘      ┌────┘      └────┐
    │           │                │
┌───────┐  ┌───────┐      ┌───────┐
│yanying1│  │yanying2│      │yanying3│
│ 流1-50 │  │流51-100│      │流101-150│
└───────┘  └───────┘      └───────┘
    │           │                │
    └────┬──────┴──────┬─────────┘
         │             │
    ┌─────────┐   ┌─────────┐
    │ MinIO   │   │ Kafka   │
    │ 集群    │   │ 集群    │
    └─────────┘   └─────────┘
```

---

## 开发路线图

### ✅ 已完成 (v1.0)

- [x] 流媒体服务器（RTSP/RTMP/HLS等）
- [x] 视频抽帧插件
- [x] AI分析插件
- [x] 任务类型分类系统
- [x] 算法服务注册中心
- [x] Web管理界面
- [x] RESTful API
- [x] SQLite数据存储
- [x] Kafka消息推送

### 🚧 开发中 (v1.1 - Q1 2025)

- [ ] 用户认证与权限管理
- [ ] RBAC权限模型
- [ ] 多租户支持
- [ ] 数据统计与报表
- [ ] 更丰富的Web界面

### 🔮 计划中 (v2.0 - Q2 2025)

- [ ] 模型训练平台（Web端训练）
- [ ] 算法市场（算法发布与订阅）
- [ ] 移动端App（iOS/Android）
- [ ] 视频存储与回放
- [ ] GPU加速支持
- [ ] 边缘计算部署方案

### 💡 愿景 (v3.0+)

- [ ] AutoML自动调参
- [ ] 联邦学习支持
- [ ] 区块链溯源
- [ ] 3D可视化
- [ ] VR/AR集成

---

## 社区与生态

### 算法开发者

**我们提供**：
- 📚 完整的API文档
- 🎯 标准的接口规范
- 💻 多语言SDK（Python/Go/Java/C++）
- 🧪 测试工具与Mock服务
- 📦 算法模板工程

**你只需要**：
1. 使用熟悉的语言和框架开发算法
2. 实现标准的HTTP推理接口
3. 启动服务并注册到yanying
4. 专注于算法优化，平台自动调度

### 系统集成商

**我们提供**：
- 🏗️ 开箱即用的平台
- 🔌 灵活的插件机制
- 🎨 可定制的Web界面
- 📡 标准的API接口
- 🛠️ 完整的部署文档

**你可以**：
1. 快速为客户搭建智能监控系统
2. 集成客户现有的算法服务
3. 定制化开发特色功能
4. 提供专业的运维支持

### 企业用户

**我们提供**：
- 💰 MIT协议，免费商用
- 🔒 本地化部署，数据安全
- 📈 稳定可靠的性能
- 🌐 活跃的社区支持
- 📞 可选的商业支持服务

**你可以获得**：
1. 降低视频AI项目成本
2. 缩短项目开发周期
3. 灵活扩展业务场景
4. 持续的技术迭代

---

## 商业支持

### 免费版（社区版）

✅ 所有核心功能
✅ MIT开源协议
✅ 社区技术支持
✅ 文档和示例代码

### 企业版（计划中）

✅ 所有社区版功能
✅ 专业技术支持（SLA保障）
✅ 定制化开发服务
✅ 私有化部署指导
✅ 性能优化咨询
✅ 算法调优服务

### 联系我们

- **技术咨询**：tech@yanying.com
- **商务合作**：business@yanying.com
- **社区讨论**：GitHub Discussions

---

<div align="center">

**让每个开发者都能构建智能视频应用**

*yanying - 视频智能分析的基础设施*

[⬆ 返回顶部](#yanying-项目介绍)

</div>

