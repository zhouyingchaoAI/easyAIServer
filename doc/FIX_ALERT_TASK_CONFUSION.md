# 修复：告警结果在不同任务之间混淆问题

## 问题描述

告警结果会在不同任务之间搞混，导致A任务的告警被关联到B任务上。

## 根本原因

**路径结构与解析逻辑不一致**：

### 原问题

1. **抽帧路径使用**：`basePath/任务类型/task.OutputPath/文件名`
2. **AI分析解析**：`basePath/任务类型/任务ID/文件名`（scanner.go 的 parseImagePath 函数）
3. **冲突**：解析器将 `OutputPath` 当成了 `TaskID`

如果用户创建任务时 `output_path` 和 `id` 不一致（例如通过API创建任务），AI分析就会从路径中提取错误的任务ID，导致告警关联到错误的任务。

### 示例场景

```go
// 用户创建任务
task := {
    ID: "camera001",
    OutputPath: "entrance_camera",  // 不同于ID
    TaskType: "人数统计"
}

// 抽帧路径：人数统计/entrance_camera/20251105-100000.jpg
// AI分析解析：taskID = "entrance_camera" ❌ (错误！应该是 "camera001")
// 告警关联到：taskID = "entrance_camera"
```

## 解决方案

**统一使用 `task.ID` 作为路径的第二层目录**，确保路径解析逻辑能正确提取任务ID。

### 修改前后对比

**修改前**：
```go
// 抽帧路径
key := filepath.Join(basePath, taskType, task.OutputPath, filename)
// 例如：人数统计/entrance_camera/20251105-100000.jpg

// AI解析
taskID = parts[1]  // "entrance_camera"  ← 错误！
```

**修改后**：
```go
// 抽帧路径
key := filepath.Join(basePath, taskType, task.ID, filename)
// 例如：人数统计/camera001/20251105-100000.jpg

// AI解析
taskID = parts[1]  // "camera001"  ← 正确！
```

## 修改的文件

### 1. internal/plugin/frameextractor/minio.go
- `createMinioPath()` - 创建MinIO路径
- `deleteMinioPath()` - 删除MinIO路径
- `runMinioSinkLoopCtx()` - 上传抽帧图片
- `cleanupOldFrames()` - 清理旧图片

### 2. internal/plugin/frameextractor/service.go
- `extractSinglePreviewFrame()` - 预览图保存路径
- `SaveAlgorithmConfig()` - 算法配置保存路径
- `GetAlgorithmConfig()` - 算法配置读取路径
- `GetAlgorithmConfigPath()` - 算法配置路径获取

### 3. internal/plugin/frameextractor/worker.go
- `runLocalSinkLoop()` - 本地存储路径
- `runLocalSinkLoopCtx()` - 本地存储路径（带上下文）

### 4. internal/plugin/frameextractor/gallery.go
- `ListSnapshots()` - 本地图片列表路径
- `ListSnapshotsFromMinIO()` - MinIO图片列表路径

## 路径结构规范

### 统一路径格式

```
{basePath}/{任务类型}/{任务ID}/{文件名}
```

**强制要求**：
- 第一层：任务类型（task_type）
- 第二层：任务ID（task.ID）**← 关键修改点**
- 第三层：文件名

### 示例

```
抽帧图片：     人数统计/camera001/20251105-100000.jpg
预览图：       人数统计/camera001/preview_20251105-100000.jpg
算法配置：     人数统计/camera001/algo_config.json
告警图片：     alerts/人数统计/camera001/20251105-100000.jpg
```

## OutputPath 字段的作用

修改后，`OutputPath` 字段仅用于以下场景：
1. **显示用途**：在日志和统计信息中显示
2. **兼容性**：保留字段不破坏API接口
3. **默认值**：如果用户不指定，默认等于 `ID`

**重要**：`OutputPath` 不再用于构建存储路径。

## 向后兼容性

### 自动迁移

在 `service.go` 的 `AddTask` 函数中：

```go
// ensure output_path defaults to task ID if empty
if strings.TrimSpace(t.OutputPath) == "" {
    t.OutputPath = t.ID
}
```

### 兼容性检查

在 `compat.go` 中有迁移逻辑：

```go
if task.OutputPath == "" {
    task.OutputPath = task.ID
    migrated = true
}
```

## 验证修复

### 1. 创建测试任务

```bash
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test_camera_001",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://...",
    "output_path": "custom_name"  // 故意使用不同的名称
  }'
```

### 2. 检查抽帧路径

```bash
# 检查MinIO中的实际路径
# 应该是：人数统计/test_camera_001/...
# 而不是：人数统计/custom_name/...
```

### 3. 验证告警关联

```bash
# 查看告警记录中的 task_id
curl http://localhost:5066/api/v1/alerts

# task_id 应该是 "test_camera_001"
# 而不是 "custom_name"
```

## 注意事项

### 已有任务

修复后，**新抽取的帧会使用新路径**（`task.ID`），而旧的帧仍在旧路径（`task.OutputPath`）中。

**建议**：
1. 对于已运行的任务，如果发现告警混淆，可以：
   - 停止任务
   - 删除旧的抽帧图片
   - 重新启动任务
2. 或者等待自动清理机制清除旧图片

### API用户

如果你通过API创建任务：
- **推荐**：不指定 `output_path`，让系统自动使用 `id`
- **可选**：如果需要指定，确保 `output_path` 等于 `id`
- **避免**：`output_path` 和 `id` 不一致（虽然系统会忽略 `output_path`）

## 修复日期

- **日期**：2025-11-05
- **版本**：v8.3.3+
- **影响范围**：所有使用抽帧和AI分析的功能

## 相关问题

- [抽帧图片自动清理功能](FRAME_CLEANUP_FEATURE.md)
- [AI分析功能文档](AI_ANALYSIS.md)

