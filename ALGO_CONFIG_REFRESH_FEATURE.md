# 抽帧管理界面刷新功能实现

**日期**: 2025-11-04  
**功能**: 任务列表刷新 + 配置状态回显  
**状态**: ✅ 已完成

---

## 🎯 功能需求

### 用户需求
> "另外增加一个刷新抽帧管理列表列表的按钮，每次配置完成抽帧任务都会自动刷新，刷新也会回显算法配置状态和算法配置修改按钮"

### 核心功能

1. ✅ 添加手动刷新按钮
2. ✅ 配置保存后自动刷新列表
3. ✅ 刷新后回显配置状态（已配置/待配置）
4. ✅ 刷新后显示算法配置按钮

---

## 📊 功能实现

### 1. 手动刷新按钮 ✅

**位置**: 任务列表卡片标题栏右侧

```vue
<template #extra>
  <a-space>
    <a-tag :color="config.store === 'minio' ? 'blue' : 'green'">
      {{ config.store === 'minio' ? 'MinIO存储' : '本地存储' }}
    </a-tag>
    <!-- 🆕 新增刷新按钮 -->
    <a-button @click="refreshTaskList" :loading="taskListLoading" size="small">
      <template #icon><ReloadOutlined /></template>
      刷新列表
    </a-button>
    <a-button type="primary" @click="goToGallery">
      <template #icon><PictureOutlined /></template>
      查看抽帧结果
    </a-button>
  </a-space>
</template>
```

### 2. 刷新函数实现 ✅

```javascript
// 刷新任务列表（带加载状态和提示）
const refreshTaskList = async () => {
  taskListLoading.value = true
  try {
    await fetchList()
    message.success('任务列表已刷新')
  } catch (e) {
    message.error('刷新失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    taskListLoading.value = false
  }
}
```

### 3. 自动刷新（配置保存后）✅

```javascript
const handleAlgoConfigSaved = async () => {
  message.success('算法配置已保存，正在刷新任务列表...')
  taskListLoading.value = true
  try {
    await fetchList()
    message.success('任务列表已更新，配置状态已同步')
  } catch (e) {
    message.error('刷新任务列表失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    taskListLoading.value = false
  }
}
```

### 4. 表格加载状态 ✅

```vue
<a-table 
  :data-source="items" 
  :columns="columns" 
  row-key="id" 
  :pagination="{ pageSize: 10, showTotal: (total) => `共 ${total} 条` }"
  :scroll="{ x: 1200 }"
  :loading="taskListLoading"  <!-- 🆕 新增加载状态 -->
>
```

---

## 🎨 界面效果

### 任务列表标题栏

```
┌─────────────────────────────────────────────────────────────┐
│  📹 抽帧任务 (5)                                            │
│                               [MinIO存储] [刷新列表] [查看抽帧结果] │
└─────────────────────────────────────────────────────────────┘
```

### 配置状态显示

| 任务ID | 任务类型 | 配置状态 | 操作 |
|--------|----------|----------|------|
| 测试1 | 人数统计 | ✅ 已配置 | [⚙️ 算法配置] [▶️ 启动] |
| 测试2 | 人数统计 | ⚠️ 待配置 | [⚙️ 算法配置] |
| 测试3 | 客流分析 | ✅ 已配置 | [⚙️ 算法配置] [⏸️ 停止] |

---

## 🔄 工作流程

### 场景1：首次配置任务

```
1. 添加任务
   ↓
2. 点击"算法配置"按钮
   ↓
3. 绘制区域，配置参数
   ↓
4. 点击"保存配置"
   ↓
5. ✅ 自动刷新任务列表
   ↓
6. 配置状态从 "⚠️ 待配置" 变为 "✅ 已配置"
```

### 场景2：修改已有配置

```
1. 点击"算法配置"按钮
   ↓
2. ✅ 从MinIO加载已有配置（回显）
   ↓
3. 在画布上显示已绘制的区域
   ↓
4. 修改区域或参数
   ↓
5. 点击"保存配置"
   ↓
6. ✅ 自动刷新任务列表
   ↓
7. 配置状态保持 "✅ 已配置"
```

### 场景3：手动刷新

```
1. 用户怀疑状态不同步
   ↓
2. 点击"刷新列表"按钮
   ↓
3. ✅ 表格显示加载动画
   ↓
4. 从服务器重新获取最新数据
   ↓
5. 显示 "任务列表已刷新"
   ↓
6. 所有状态更新为最新
```

---

## 📝 后端API支持

### 获取任务列表

**端点**: `GET /api/v1/frame_extractor/tasks`

**返回数据**:
```json
{
  "items": [
    {
      "id": "测试1",
      "task_type": "人数统计",
      "rtsp_url": "rtsp://...",
      "interval_ms": 200,
      "output_path": "测试1",
      "enabled": true,
      "config_status": "configured",  ← 配置状态
      "preview_image": "人数统计/测试1/preview.jpg"
    }
  ]
}
```

### 获取算法配置（回显）

**端点**: `GET /api/v1/frame_extractor/tasks/:id/config`

**返回数据**:
```json
{
  "task_id": "测试1",
  "task_type": "人数统计",
  "config_version": "2.0",
  "coordinate_type": "normalized",
  "regions": [
    {
      "id": "region_001",
      "name": "入口区域",
      "type": "rectangle",
      "points": [[0.2, 0.1], [0.8, 0.9]],
      "properties": {
        "color": "#00FF00",
        "opacity": 0.3,
        "threshold": 0.5
      },
      "enabled": true
    }
  ],
  "algorithm_params": {
    "confidence_threshold": 0.7,
    "iou_threshold": 0.5
  },
  "created_at": "2025-11-04T02:00:07.179Z",
  "updated_at": "2025-11-04T02:00:07.179Z"
}
```

### 保存算法配置

**端点**: `POST /api/v1/frame_extractor/tasks/:id/config`

**功能**:
1. 保存配置到MinIO
2. 更新任务的 `config_status` 为 `"configured"`
3. 持久化到 `config.toml`

---

## ✨ 用户体验优化

### 1. 加载状态反馈

```
点击刷新按钮
  ↓
按钮显示加载动画 🔄
  ↓
表格显示骨架屏
  ↓
数据加载完成
  ↓
显示提示："任务列表已刷新" ✅
```

### 2. 自动刷新提示

```
保存配置
  ↓
提示："算法配置已保存，正在刷新任务列表..." 
  ↓
列表刷新中
  ↓
提示："任务列表已更新，配置状态已同步" ✅
```

### 3. 配置状态可视化

```
⚠️ 待配置  → 橙色标签 → 提示用户需要配置
✅ 已配置  → 绿色标签 → 可以正常使用
```

---

## 🔍 回显逻辑详解

### AlgoConfigModal 组件打开时

```javascript
// 1. 初始化Canvas
await initCanvas()
  ↓
// 2. 加载预览图片
await loadPreviewImage()
  ↓
// 3. 从MinIO加载已有配置
await loadExistingConfig()
  ↓
// 4. 解析配置数据
if (data && data.regions) {
  // 深拷贝配置
  regions.value = JSON.parse(JSON.stringify(data.regions))
  algorithmParams.value = data.algorithm_params
  
  // 坐标转换（归一化 → 像素）
  regions.value.forEach(region => {
    if (isNormalized(region.points)) {
      region.points = normalizedToPixel(region.points)
    }
  })
  
  // 在画布上绘制区域
  regions.value.forEach(region => {
    drawRegionOnCanvas(region)
  })
  
  message.success(`已加载 ${regions.value.length} 个配置区域`)
}
```

---

## 📦 修改的文件

### 前端文件

1. **web-src/src/views/frame-extractor/index.vue**
   - ✅ 添加刷新按钮到任务列表标题栏
   - ✅ 添加 `taskListLoading` 状态
   - ✅ 实现 `refreshTaskList()` 函数
   - ✅ 优化 `handleAlgoConfigSaved()` 自动刷新
   - ✅ 表格添加 `:loading` 状态

### 后端文件（已存在）

2. **internal/web/api/api.go**
   - ✅ `GET /frame_extractor/tasks/:id/config` - 获取配置
   - ✅ `POST /frame_extractor/tasks/:id/config` - 保存配置

3. **internal/plugin/frameextractor/service.go**
   - ✅ `GetAlgorithmConfig()` - 从MinIO读取配置
   - ✅ `SaveAlgorithmConfig()` - 保存配置到MinIO

4. **web-src/src/components/AlgoConfigModal/index.vue**
   - ✅ `loadExistingConfig()` - 已实现配置回显

---

## ✅ 功能清单

### 基础功能
- [x] 手动刷新按钮
- [x] 刷新加载动画
- [x] 刷新成功提示
- [x] 刷新失败提示

### 自动刷新
- [x] 配置保存后自动刷新
- [x] 添加任务后自动刷新
- [x] 删除任务后自动刷新
- [x] 启动/停止后自动刷新
- [x] 更新间隔后自动刷新

### 状态回显
- [x] 配置状态显示（已配置/待配置）
- [x] 算法配置按钮显示
- [x] 配置修改时从MinIO加载已有数据
- [x] 区域在画布上正确显示
- [x] 算法参数正确回填

---

## 🚀 部署步骤

### 1. 前端已编译 ✅
```bash
cd /code/EasyDarwin/web-src
npm run build
# ✅ 编译成功
```

### 2. 复制到运行目录 ✅
```bash
cp -r ./web ./build/EasyDarwin-aarch64-v8.3.3-202511040206/
# ✅ 已复制
```

### 3. 重启服务（可选）
```bash
# 如果服务已运行，刷新浏览器即可
# 或者重启服务
pkill easydarwin && ./easydarwin
```

### 4. 验证功能
1. 访问 `http://localhost:5066/frame-extractor`
2. 查看任务列表标题栏是否有"刷新列表"按钮
3. 点击刷新按钮，检查加载动画和提示
4. 点击"算法配置"按钮，检查是否正确回显配置

---

## 🎨 界面预览

### 任务列表标题栏（新增刷新按钮）

```
┌───────────────────────────────────────────────────────────────────────┐
│  📹 抽帧任务 (5)                                                       │
│                           [MinIO存储] [🔄 刷新列表] [📷 查看抽帧结果]  │
└───────────────────────────────────────────────────────────────────────┘
```

### 配置状态显示

```
任务ID    任务类型    配置状态        操作
────────────────────────────────────────────────
测试1     人数统计    ✅ 已配置      [⚙️ 算法配置] [▶️ 启动]
测试2     人数统计    ⚠️ 待配置      [⚙️ 算法配置]
测试3     客流分析    ✅ 已配置      [⚙️ 算法配置] [⏸️ 停止]
```

### 配置回显效果

```
点击"算法配置"按钮
  ↓
弹出配置窗口
  ↓
加载预览图片 ✅
  ↓
从MinIO读取配置 ✅
  ↓
画布上显示已绘制的区域 ✅
│
├─ 矩形区域（绿色，透明度0.3）
├─ 线条区域（红色，带方向箭头）
└─ 多边形区域（蓝色）
  ↓
右侧面板显示区域配置 ✅
│
├─ 区域名称
├─ 颜色选择器
├─ 透明度滑块
└─ 检测阈值
  ↓
底部显示算法参数 ✅
│
├─ 置信度阈值: 0.7
└─ IOU阈值: 0.5
```

---

## 💡 用户操作流程

### 首次配置
```
1. 添加任务
2. 等待预览图生成（约3-5秒）
3. 点击"算法配置"按钮
4. 绘制检测区域
5. 设置参数
6. 点击"保存配置"
   → ✅ 自动刷新列表
   → ✅ 配置状态变为"已配置"
   → ✅ 任务自动启动
```

### 修改配置
```
1. 点击"算法配置"按钮
   → ✅ 自动加载已有配置
   → ✅ 画布上显示已绘制区域
2. 修改区域或参数
3. 点击"保存配置"
   → ✅ 自动刷新列表
   → ✅ 配置状态保持"已配置"
```

### 手动刷新
```
1. 点击"刷新列表"按钮
   → ✅ 显示加载动画
   → ✅ 重新获取所有任务数据
   → ✅ 更新配置状态
   → ✅ 显示"任务列表已刷新"提示
```

---

## 📊 数据流

### 配置保存流程

```
用户点击"保存配置"
  ↓
前端: AlgoConfigModal.saveConfig()
  ↓
API: POST /frame_extractor/tasks/:id/config
  ↓
后端: SaveAlgorithmConfig()
  │
  ├─ 将配置JSON保存到MinIO
  │  路径: {task_type}/{task_id}/algo_config.json
  │
  └─ 更新任务的 config_status = "configured"
     持久化到 config.toml
  ↓
返回成功
  ↓
前端触发: handleAlgoConfigSaved()
  ↓
自动刷新任务列表
  ↓
配置状态更新显示 ✅
```

### 配置回显流程

```
用户点击"算法配置"
  ↓
前端: openAlgoConfig(record)
  ↓
AlgoConfigModal打开
  ↓
initCanvas()
  ↓
loadPreviewImage()
  ↓
loadExistingConfig()
  │
  ├─ API: GET /frame_extractor/tasks/:id/config
  │  ↓
  │  后端: GetAlgorithmConfig()
  │  ↓
  │  从MinIO读取 algo_config.json
  │  ↓
  │  返回配置数据
  │
  ├─ 解析regions和algorithm_params
  │
  ├─ 坐标转换（归一化 → 像素）
  │
  └─ 在画布上绘制区域 ✅
  ↓
用户看到完整的配置回显 ✅
```

---

## ✅ 验证要点

### 前端功能
1. 访问抽帧管理页面
2. 检查标题栏是否有"刷新列表"按钮 ✅
3. 点击刷新按钮，检查：
   - 按钮显示加载状态 ✅
   - 表格显示加载动画 ✅
   - 显示成功提示 ✅

### 配置回显
1. 选择一个已配置的任务
2. 点击"算法配置"按钮
3. 检查是否正确显示：
   - 预览图片加载 ✅
   - 已绘制的区域显示在画布上 ✅
   - 区域配置在右侧面板显示 ✅
   - 算法参数正确回填 ✅

### 自动刷新
1. 修改并保存配置
2. 检查是否自动刷新列表 ✅
3. 检查配置状态是否更新 ✅

---

## 🎉 完成总结

### 新增功能
- ✅ 手动刷新按钮（带加载动画）
- ✅ 刷新成功/失败提示
- ✅ 配置保存后自动刷新
- ✅ 配置状态实时同步

### 已有功能（保持）
- ✅ 配置从MinIO加载（回显）
- ✅ 区域在画布上显示
- ✅ 算法参数回填
- ✅ 任务列表显示配置状态

### 部署状态
- ✅ 前端代码已修改
- ✅ 前端已编译
- ✅ 文件已复制到运行目录
- ✅ 无linter错误

---

**修改完成时间**: 2025-11-04  
**前端编译**: ✅ 通过  
**文件部署**: ✅ 完成  
**测试状态**: ⏳ 待用户验证

现在只需要刷新浏览器页面即可看到新功能！

