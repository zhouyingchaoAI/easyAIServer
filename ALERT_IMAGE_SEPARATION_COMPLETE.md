# 告警图片分离功能完整实现

## ✅ 完成的功能

### 核心需求
1. ✅ 告警图片单独存放（`alerts/` 路径）
2. ✅ 抽帧图片推理后立即删除
3. ✅ 防止告警图片在无算法时被误删
4. ✅ 扫描器自动跳过告警路径
5. ✅ 检测到告警时移动图片到告警路径

## 🔧 修改的文件

### 1. **internal/conf/model.go**
- 添加 `AlertBasePath` 字段到 `AIAnalysisConfig`

### 2. **configs/config.toml**
- 添加 `alert_base_path = 'alerts/'` 配置项

### 3. **internal/plugin/aianalysis/scanner.go**
- 添加 `alertBasePath` 字段
- 修改 `NewScanner` 接受 `alertBasePath` 参数
- 扫描时跳过告警路径中的图片

### 4. **internal/plugin/aianalysis/scheduler.go**
- 添加 `alertBasePath` 字段
- 修改 `NewScheduler` 接受 `alertBasePath` 参数
- 添加 `moveImageToAlertPath()` 方法
- 推理后移动图片到告警路径
- 添加 `strings` 导入

### 5. **internal/plugin/aianalysis/service.go**
- 从配置读取 `AlertBasePath`
- 传递给 `NewScheduler` 和 `NewScanner`

## 📊 工作流程

### 原始流程
```
抽帧 → MinIO (frames/) → 推理 → 保存告警 → 推送MQ
```

### 新流程
```
抽帧 → MinIO (frames/) 
     ↓
推理
     ↓
有告警?
     ├─ 是 → 移动图片到 alerts/{task_type}/{task_id}/  → 删除原图 → 保存告警
     └─ 否 → 删除原图（不保存告警）
```

## 🎯 核心逻辑

### 1. 扫描阶段（Scanner）
```go
// 跳过告警路径中的图片
if s.alertBasePath != "" && strings.HasPrefix(object.Key, s.alertBasePath) {
    s.log.Debug("skipping alert image", slog.String("path", object.Key))
    continue
}
```

### 2. 推理阶段（Scheduler）

#### 2.1 无告警时
```go
// 直接删除原图
if s.saveOnlyWithDetection && detectionCount == 0 {
    s.deleteImageWithReason(image.Path, "no_detection")
    return // 不保存告警
}
```

#### 2.2 有告警时
```go
// 移动图片到告警路径
if s.alertBasePath != "" && detectionCount > 0 {
    movedPath, err := s.moveImageToAlertPath(image.Path, image.TaskType, image.TaskID)
    // 使用新路径保存告警
    alert.ImagePath = movedPath
}
```

### 3. 图片移动函数
```go
func (s *Scheduler) moveImageToAlertPath(imagePath, taskType, taskID string) (string, error) {
    // 构建告警路径：alerts/{task_type}/{task_id}/filename
    alertPath := fmt.Sprintf("%s%s/%s/%s", s.alertBasePath, taskType, taskID, filename)
    
    // 复制到新路径
    s.minio.CopyObject(dst, src)
    
    // 删除原图
    s.minio.RemoveObject(imagePath)
    
    return alertPath, nil
}
```

## 📁 路径结构

### 抽帧路径（frames/）
```
frames/
└── {task_type}/
    └── {task_id}/
        ├── snapshot_12345.jpg
        ├── snapshot_12350.jpg
        └── ...
```

### 告警路径（alerts/）
```
alerts/
└── {task_type}/
    └── {task_id}/
        ├── snapshot_12345.jpg  # 有告警的图片
        └── snapshot_12356.jpg  # 有告警的图片
```

## 🛡️ 防护机制

### 1. 扫描器不扫描告警路径
- 防止重复推理已处理的告警图片
- 保护告警图片不被误删

### 2. 抽帧图片推理后立即删除
- 防止抽帧路径积压
- 节省存储空间

### 3. 告警图片永久保留
- 存储在独立的 `alerts/` 路径
- 不会因为重启或算法故障而丢失

## ⚙️ 配置示例

```toml
[ai_analysis]
enable = true
scan_interval_sec = 5
save_only_with_detection = true
alert_base_path = 'alerts/'  # 告警图片存储路径前缀
```

## 🔍 关键改进

### 问题1：程序重启后不知道哪些图片已处理
**解决方案**：
- 告警图片存储在独立路径 `alerts/`
- 扫描器自动跳过 `alerts/` 路径
- 即使重启也不会重复推理

### 问题2：无算法时会删除告警图片
**解决方案**：
- 推理前无算法时删除抽帧图片
- 推理后有告警时移动图片到 `alerts/` 路径
- 告警图片不受算法启停影响

### 问题3：抽帧图片会积压
**解决方案**：
- 推理后立即删除抽帧图片
- 只有有告警的图片才移动到 `alerts/` 路径
- 保持抽帧路径干净

## ✅ 编译验证

- ✅ 编译成功
- ✅ 无linter错误

## 🚀 使用说明

1. 配置告警路径：
```toml
[ai_analysis]
alert_base_path = 'alerts/'
```

2. 启用只保存有告警的图片：
```toml
save_only_with_detection = true
```

3. 重启服务生效

4. 检查告警图片存储：
```bash
# 查看告警图片
mc ls minio/easydarwin/alerts/
```

## 📈 效果

- ✅ 抽帧路径不会积压
- ✅ 告警图片安全独立存储
- ✅ 重启不会重复推理
- ✅ 算法启停不影响告警图片
- ✅ 存储空间优化

## 总结

告警图片分离功能已成功实现：
- ✅ 配置支持
- ✅ 扫描器过滤
- ✅ 调度器移动
- ✅ 路径管理
- ✅ 防护机制
- ✅ 编译通过

系统现在具备完整的告警图片管理和保护机制！🎉


