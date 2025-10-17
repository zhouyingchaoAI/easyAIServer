# 算法配置功能开发进度

## 📊 总体进度：60% 完成

### ✅ 已完成 (60%)

#### 1. 算法配置规范设计 ✅
- 📄 创建 `doc/ALGORITHM_CONFIG_SPEC.md` 完整规范文档
- 支持多种区域类型：线、矩形、多边形
- 支持多个分析区域
- 通用算法参数配置
- 5种任务类型示例

#### 2. 后端数据模型 ✅
- `FrameExtractTask` 扩展字段：
  * `ConfigStatus` - 配置状态（unconfigured/configured）
  * `PreviewImage` - 预览图片路径
- `InferenceRequest` 添加 `AlgoConfig` 字段

#### 3. 抽帧服务核心功能 ✅
- 添加任务后自动抽取单张预览图
- 预览图保存到MinIO
- 算法配置保存到MinIO (algo_config.json)
- 算法配置加载
- 配置完成后启动抽帧

#### 4. API接口 ✅
- `GET /frame_extractor/tasks/:id/preview` - 获取预览图
- `POST /frame_extractor/tasks/:id/config` - 保存算法配置
- `GET /frame_extractor/tasks/:id/config` - 获取算法配置
- `POST /frame_extractor/tasks/:id/start_with_config` - 配置后启动

#### 5. AI分析插件集成 ✅
- 推理请求自动包含算法配置
- 算法服务可以读取区域配置进行推理

---

### 🚧 进行中 (30%)

#### 6. 前端实现
- [ ] 安装 Fabric.js 依赖
- [ ] 创建算法配置弹窗组件
- [ ] 实现Canvas绘图功能
  * 绘制线
  * 绘制矩形
  * 绘制多边形
  * 删除/编辑区域
- [ ] 区域配置面板
- [ ] 与后端API集成
- [ ] 任务列表添加"算法配置"按钮

#### 7. 算法服务自动触发
- [ ] 监听算法服务注册事件
- [ ] 自动启动已配置任务

#### 8. 算法服务示例更新
- [ ] 更新 `algorithm_service.py` 支持读取配置
- [ ] 添加区域检测示例代码

#### 9. 算法对接说明书
- [ ] 创建独立的对接文档
- [ ] Python示例代码
- [ ] 常见问题解答

---

### 📦 已提交代码

**Commit**: `0b36970f`
**消息**: feat: 后端实现算法配置功能

**变更文件**:
- `doc/ALGORITHM_CONFIG_SPEC.md` (新增)
- `internal/conf/model.go` (修改)
- `internal/plugin/frameextractor/service.go` (修改)
- `internal/plugin/frameextractor/worker.go` (修改)
- `internal/web/api/api.go` (修改)
- `internal/plugin/aianalysis/scheduler.go` (修改)

---

## 🎯 核心特性说明

### 工作流程

```
添加任务
    ↓
自动抽取1张预览图
    ↓
任务停止（待配置）
    ↓
点击"算法配置"按钮
    ↓
绘图界面配置区域
    ↓
保存配置到MinIO
    ↓
【手动启动】或【算法服务上线自动启动】
    ↓
正常抽帧 + 推理
```

### JSON配置存储

**路径**: `frames/{task_type}/{task_id}/algo_config.json`

**示例**:
```json
{
  "task_id": "cam_entrance_001",
  "task_type": "人数统计",
  "regions": [
    {
      "id": "region_001",
      "name": "入口区域",
      "type": "polygon",
      "points": [[100,200], [300,200], [300,400], [100,400]],
      "properties": {
        "color": "#FF0000",
        "threshold": 0.5
      }
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7
  }
}
```

### 推理请求格式

```json
{
  "image_url": "https://...",
  "task_id": "cam_entrance_001",
  "task_type": "人数统计",
  "image_path": "frames/...",
  "algo_config": {
    "regions": [...],
    "algorithm_params": {...}
  }
}
```

---

## 📅 预计完成时间

- **前端实现**: 4-5小时
- **自动触发**: 2小时
- **文档和测试**: 2小时
- **总计剩余**: 8-9小时

---

## 🔧 下一步操作

1. 安装 Fabric.js
2. 创建算法配置弹窗组件
3. 实现绘图功能
4. 测试完整流程
5. 编写算法对接说明书

---

**更新时间**: 2024-10-17
**当前阶段**: 前端实现中

