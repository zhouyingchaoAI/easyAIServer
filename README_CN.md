# yanying 视频智能分析平台

<div align="center">

**🎯 开源 · 高效 · 智能**

一个集流媒体服务、视频抽帧、AI智能分析于一体的综合视频平台

[快速开始](#快速开始) · [智能分析](#智能分析核心能力) · [文档](#详细文档) · [架构](#系统架构)

</div>

---

## ✨ 核心特性

### 🤖 智能分析能力（核心优势）

yanying平台内置强大的AI智能分析引擎，支持灵活的算法服务接入和实时智能分析：

#### 🎯 算法服务生态
- **开放式算法注册中心**：支持任何编程语言开发的算法服务动态接入
- **任务类型分类体系**：预设20+种智能分析场景（人数统计、人员跌倒、吸烟检测、安全帽检测等）
- **智能匹配机制**：一种任务类型可关联多个算法服务，自动负载均衡
- **服务健康管理**：心跳检测、自动注销异常服务、服务状态实时监控

#### 🔄 自动化处理流程
- **智能扫描器**：自动发现MinIO中的新增图片（可配置扫描间隔）
- **去重机制**：跟踪已处理图片，避免重复推理
- **并发调度**：支持多算法并发推理，可配置并发数限制
- **结果聚合**：多算法推理结果智能合并与汇总

#### 📊 结果管理
- **告警存储**：SQLite数据库持久化存储所有分析结果
- **实时推送**：Kafka消息队列实时分发告警信息
- **Web可视化**：内置告警查看、算法服务管理、统计分析界面
- **结构化输出**：标准化的JSON格式推理结果

### 🎬 视频流处理能力

- **多协议支持**：RTSP/RTMP/HLS/HTTP-FLV/WebSocket-FLV/WebRTC
- **推拉流灵活配置**：支持RTMP推流、RTSP拉流，自动转换分发
- **按需播放**：无观看者时自动断流节省带宽
- **视频预览**：Web界面实时预览所有流
- **视频点播**：支持录制视频的点播功能

### 📸 智能抽帧引擎

- **可配置抽帧间隔**：毫秒级精度控制抽帧频率
- **任务类型分类**：按AI分析场景自动归类图片
- **双存储支持**：本地文件系统 / MinIO对象存储
- **目录结构化**：`{任务类型}/{任务ID}/frames/` 层次化管理
- **Web管理界面**：可视化任务创建、启停、快照浏览
- **断线重连**：指数退避算法，自动恢复连接

### 🖥️ 易用性

- **集成Web界面**：现代化的Vue3前端，无需额外部署
- **RESTful API**：完整的API文档（apidoc）
- **跨平台支持**：Linux/Windows/macOS + X86_64/ARM/RISCV等多架构
- **零依赖运行**：单个可执行文件，内置SQLite数据库
- **系统服务**：支持注册为系统服务，开机自启

---

## 🚀 快速开始

### 环境准备

```bash
# 基础依赖
- Go 1.23.0+
- (可选) MinIO - 用于对象存储
- (可选) Kafka - 用于消息推送
```

### 一键启动

```bash
# 1. 克隆项目
git clone https://github.com/EasyDarwin/EasyDarwin.git
cd EasyDarwin

# 2. 编译安装
make build/linux  # Linux
# 或
make build/windows  # Windows

# 3. 启动服务
cd build/EasyDarwin-lin-*
./easydarwin

# 4. 访问Web界面
# 打开浏览器：http://localhost:10086
```

### 启用智能分析（5分钟配置）

#### 步骤1：配置抽帧插件

编辑 `configs/config.toml`：

```toml
[frame_extractor]
enable = true
store = 'minio'  # 使用MinIO存储
scan_only = false

[frame_extractor.minio]
endpoint = 'localhost:9000'
access_key = 'minioadmin'
secret_key = 'minioadmin'
bucket = 'snapshots'
use_ssl = false
```

#### 步骤2：配置AI分析插件

```toml
[ai_analysis]
enable = true
scan_interval_sec = 10  # 每10秒扫描一次MinIO
mq_type = 'kafka'
mq_address = 'localhost:9092'
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 5  # 最多5个并发推理任务
```

#### 步骤3：启动MinIO（使用Docker）

```bash
docker run -d \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  --name minio \
  minio/minio server /data --console-address ":9001"
```

#### 步骤4：创建抽帧任务

Web界面操作：
1. 打开 http://localhost:10086/#/frame-extractor
2. 点击"新增抽帧任务"
3. 填写配置：
   - **任务类型**：选择"人数统计"
   - **RTSP地址**：输入摄像头地址
   - **抽帧间隔**：5000（5秒一帧）
4. 点击"启动抽帧"

#### 步骤5：启动算法服务

```bash
# 使用示例算法服务
cd examples
pip install -r requirements.txt

# 启动人数统计算法服务
python3 algorithm_service.py \
  --service-id people_counter_v1 \
  --name "人数统计算法" \
  --task-types 人数统计 \
  --port 8000

# 算法服务会自动注册到yanying平台
# 并开始处理"人数统计"类型的图片
```

#### 步骤6：查看智能分析结果

- **查看告警**：http://localhost:10086/#/alerts
- **查看算法服务**：http://localhost:10086/#/ai-services
- **查看快照图库**：http://localhost:10086/#/frame-extractor/gallery

---

## 🏗️ 智能分析核心能力

### 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                     yanying 视频平台                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  📹 视频流接入层                                              │
│  ├─ RTSP拉流（摄像头）                                        │
│  ├─ RTMP推流（直播）                                          │
│  └─ 多协议分发（HLS/FLV/WebRTC...）                           │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  🎬 智能抽帧层                                                │
│  ├─ 按任务类型分类抽帧                                         │
│  ├─ 可配置间隔（毫秒级）                                       │
│  ├─ MinIO对象存储                                            │
│  └─ 层次化目录：任务类型/任务ID/时间戳.jpg                      │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  🤖 AI智能分析层（核心）                                       │
│  │                                                           │
│  ├─ 📋 算法服务注册中心                                        │
│  │   ├─ 服务注册/注销 API                                     │
│  │   ├─ 心跳监测（90秒超时）                                  │
│  │   ├─ 任务类型→算法映射                                     │
│  │   └─ 服务健康状态管理                                      │
│  │                                                           │
│  ├─ 🔍 MinIO智能扫描器                                       │
│  │   ├─ 定时扫描新图片（10秒间隔）                             │
│  │   ├─ 按任务类型组织                                        │
│  │   ├─ 去重机制（已处理图片跟踪）                             │
│  │   └─ 批量处理优化                                          │
│  │                                                           │
│  ├─ ⚡ 推理调度引擎                                           │
│  │   ├─ 任务类型自动匹配算法                                   │
│  │   ├─ 并发控制（可配置并发数）                               │
│  │   ├─ HTTP异步调用算法服务                                  │
│  │   ├─ 失败重试与超时处理                                     │
│  │   └─ 多算法结果聚合                                         │
│  │                                                           │
│  └─ 💾 结果处理层                                             │
│      ├─ SQLite数据库持久化                                    │
│      ├─ Kafka实时消息推送                                     │
│      ├─ Web API查询接口                                      │
│      └─ 告警统计与分析                                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                    ↓                           ↓
    ┌──────────────────────────┐   ┌──────────────────────────┐
    │  外部算法服务（任意语言）  │   │    外部系统集成           │
    │                          │   │                          │
    │  • Python/C++/Go/Java    │   │  • 前端实时告警           │
    │  • 深度学习框架           │   │  • 大屏展示              │
    │  • 标准HTTP API          │   │  • 第三方平台对接         │
    │  • 自动注册与心跳         │   │  • 数据分析               │
    └──────────────────────────┘   └──────────────────────────┘
```

### 预设任务类型（20+场景）

| 类别 | 任务类型 | 应用场景 |
|------|---------|---------|
| 👥 人员分析 | 人数统计 | 商场客流、景区人流 |
| | 人员跌倒 | 养老院、医院监护 |
| | 人脸识别 | 门禁、考勤系统 |
| | 人员聚集 | 公共安全、人群预警 |
| 🔥 安全监测 | 吸烟检测 | 禁烟区域监控 |
| | 火焰检测 | 消防安全 |
| | 安全帽检测 | 施工工地安全 |
| | 反光衣检测 | 工地规范检查 |
| 🚗 交通分析 | 车辆计数 | 交通流量统计 |
| | 车牌识别 | 停车场管理 |
| | 违停检测 | 交通执法 |
| 🏭 工业检测 | 缺陷检测 | 质量控制 |
| | 设备异常 | 设备巡检 |
| 🐾 其他场景 | 动物识别 | 野生动物监测 |
| | 行为分析 | 异常行为检测 |
| | 通用目标检测 | 自定义场景 |

---

## 📖 详细文档

### 智能分析相关

- [📘 AI分析插件完整文档](doc/AI_ANALYSIS.md) - 架构、API、示例代码
- [📗 任务类型分类系统](doc/TASK_TYPES.md) - 所有预设任务类型
- [📙 任务类型使用示例](doc/TASK_TYPE_EXAMPLES.md) - 各场景配置示例
- [📕 抽帧插件文档](doc/FRAME_EXTRACTOR.md) - 抽帧配置与使用
- [🔧 抽帧问题排查](doc/TROUBLESHOOTING_FRAME_EXTRACTOR.md) - 常见问题解决

### 快速入门

- [⚡ 快速开始指南](QUICKSTART.md) - 5分钟上手
- [🔗 直播流地址说明](doc/LIVE_STREAM_URL.md) - 各种协议地址格式

---

## 🔌 算法服务开发指南

### 标准API接口

算法服务只需实现一个简单的HTTP推理接口：

```python
# POST /infer
{
  "image_url": "http://minio:9000/snapshots/人数统计/task_1/frame_001.jpg",
  "task_type": "人数统计",
  "task_id": "task_1"
}

# Response
{
  "success": true,
  "detections": [
    {
      "class_name": "person",
      "confidence": 0.95,
      "bbox": [100, 150, 200, 350]
    }
  ],
  "inference_time_ms": 50,
  "alert": true,
  "alert_message": "检测到6人，超过阈值5人"
}
```

### 5分钟创建算法服务

```python
from flask import Flask, request, jsonify
import requests

app = Flask(__name__)

@app.route('/infer', methods=['POST'])
def infer():
    data = request.json
    image_url = data['image_url']
    
    # 1. 下载图片
    response = requests.get(image_url)
    image_data = response.content
    
    # 2. 运行你的AI模型
    detections = your_model.predict(image_data)
    
    # 3. 返回结果
    return jsonify({
        'success': True,
        'detections': detections,
        'inference_time_ms': 50,
        'alert': len(detections) > 5,  # 示例：超过5个检测结果报警
        'alert_message': f'检测到{len(detections)}个目标'
    })

# 启动时自动注册到yanying
def register_service():
    requests.post('http://localhost:10086/api/v1/ai_analysis/register', json={
        'service_id': 'my_algorithm_v1',
        'name': '我的算法服务',
        'task_types': ['人数统计', '人员跌倒'],
        'endpoint': 'http://localhost:8000/infer',
        'version': '1.0.0'
    })

if __name__ == '__main__':
    register_service()
    app.run(port=8000)
```

### 完整示例

参考 `examples/algorithm_service.py` 获取完整的算法服务实现，包括：
- 服务注册与心跳
- 图片下载与处理
- YOLO/其他模型集成
- 错误处理与日志
- 健康检查接口

---

## 🌟 应用场景

### 智慧商场
- **客流统计**：实时统计各区域人数，优化商品布局
- **热力图分析**：分析顾客行为路径
- **VIP识别**：人脸识别会员，自动推送优惠

### 智慧工地
- **安全帽检测**：实时监控工人是否佩戴安全帽
- **人员考勤**：人脸识别自动签到
- **危险区域告警**：检测未授权人员进入

### 智慧养老
- **跌倒检测**：老人跌倒自动告警通知
- **异常行为**：长时间静止、徘徊等异常检测
- **活动统计**：老人活动量分析

### 智慧交通
- **车流统计**：各路口车流量实时统计
- **违章检测**：违停、闯红灯自动抓拍
- **车牌识别**：停车场自动计费

### 智慧安防
- **吸烟检测**：禁烟区域自动告警
- **火焰检测**：早期火灾预警
- **人员聚集**：异常人群聚集告警

---

## 🎨 系统架构

### 技术栈

**后端**
- Go 1.23+ - 高性能服务端
- Gin - Web框架
- GORM - ORM框架
- SQLite - 嵌入式数据库
- lalmax - 流媒体处理

**前端**
- Vue 3 - 渐进式框架
- Ant Design Vue - UI组件库
- Vite - 构建工具

**中间件**
- MinIO - 对象存储
- Kafka - 消息队列
- Redis - 缓存（可选）

### 目录结构

```
yanying/
├── cmd/                          # 可执行程序
│   └── server/                   # 主服务
├── configs/                      # 配置文件
│   └── config.toml              # 主配置
├── internal/                     # 私有业务逻辑
│   ├── core/                    # 核心业务
│   │   ├── livestream/         # 直播流管理
│   │   ├── source/             # 视频源管理
│   │   └── video/              # 视频处理
│   ├── data/                    # 数据层
│   ├── plugin/                  # 插件系统
│   │   ├── frameextractor/     # 抽帧插件
│   │   └── aianalysis/         # AI分析插件
│   └── web/                     # Web API
│       └── api/                 # RESTful接口
├── web-src/                     # 前端源码
│   ├── src/
│   │   ├── views/              # 页面组件
│   │   │   ├── frame-extractor/  # 抽帧管理
│   │   │   ├── alerts/          # 告警查看
│   │   │   └── live/            # 直播管理
│   │   └── api/                # API调用
│   └── public/                 # 静态资源
├── web/                         # 编译后的前端
├── examples/                    # 示例代码
│   ├── algorithm_service.py    # 算法服务示例
│   └── requirements.txt        # Python依赖
├── doc/                         # 文档
│   ├── AI_ANALYSIS.md          # AI分析文档
│   ├── TASK_TYPES.md           # 任务类型
│   └── FRAME_EXTRACTOR.md      # 抽帧文档
└── README_CN.md                # 本文档
```

---

## 🔧 进阶配置

### 性能优化

```toml
# 并发控制
[ai_analysis]
max_concurrent_infer = 10  # 增加并发数（需要更多资源）

# 扫描优化
scan_interval_sec = 5  # 降低扫描间隔（更实时，但CPU占用更高）

# 数据库优化
[database]
max_open_conns = 100
max_idle_conns = 10
```

### 集群部署

```bash
# 多实例部署（需要负载均衡）
# 实例1
./easydarwin -conf ./configs/config1.toml

# 实例2
./easydarwin -conf ./configs/config2.toml

# Nginx负载均衡
upstream yanying_cluster {
    server 192.168.1.10:10086;
    server 192.168.1.11:10086;
}
```

### 生产环境建议

1. **使用独立的MinIO集群**（高可用）
2. **Kafka集群部署**（3节点以上）
3. **算法服务容器化**（Docker/K8s）
4. **监控告警**（Prometheus + Grafana）
5. **日志收集**（ELK Stack）

---

## 📊 性能指标

| 指标 | 性能 |
|------|------|
| 并发流处理 | 100+ 路同时处理 |
| 抽帧延迟 | < 100ms |
| AI推理调度 | < 50ms（不含推理时间） |
| 单实例QPS | 1000+ |
| 内存占用 | < 500MB（基础） |
| CPU占用 | < 10%（空闲时） |

---

## 🤝 支持与贡献

### 获取帮助

- **文档**：查看 `doc/` 目录完整文档
- **示例**：参考 `examples/` 目录示例代码
- **Issue**：提交问题到GitHub Issues

### 开发计划

- [ ] 用户认证与权限管理
- [ ] 多租户支持
- [ ] Web端算法训练
- [ ] 模型市场
- [ ] 移动端App
- [ ] 更多预设算法

### 贡献指南

欢迎提交PR！请确保：
1. 代码符合Go规范
2. 添加必要的测试
3. 更新相关文档
4. 提供清晰的commit message

---

## 📄 开源协议

MIT License - 详见 [LICENSE.txt](LICENSE.txt)

---

## 🙏 致谢

本项目基于以下优秀开源项目开发：
- [EasyDarwin](https://github.com/EasyDarwin/EasyDarwin) - 流媒体服务器
- [lalmax](https://github.com/q191201771/lalmax) - 流媒体库
- [Gin](https://github.com/gin-gonic/gin) - Web框架
- [GORM](https://github.com/go-gorm/gorm) - ORM框架
- [Vue](https://github.com/vuejs/vue) - 前端框架
- [Ant Design Vue](https://github.com/vueComponent/ant-design-vue) - UI组件库

---

<div align="center">

**让视频智能分析触手可及**

Made with ❤️ by yanying Team

[⬆ 回到顶部](#yanying-视频智能分析平台)

</div>

