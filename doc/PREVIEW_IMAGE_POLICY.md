# 预览图保留策略说明

## 📋 概述

预览图（Preview Image）是添加抽帧任务后自动抽取的第一张图片，用于算法配置界面的绘图参考。

**重要**：预览图不会被删除，永久保留作为配置参考。

---

## 🎯 预览图特征

### 文件命名规则

```
preview_20241017-143520.000.jpg
^^^^^^^
预览图标识
```

**规则**：
- 文件名以 `preview_` 开头
- 后跟时间戳
- 扩展名：`.jpg`

### 存储路径

```
frames/{task_type}/{task_id}/preview_*.jpg
```

**示例**：
```
frames/人数统计/cam_entrance_001/preview_20241017-143520.000.jpg
```

---

## 🔒 保留策略

### 1. 不参与AI推理

预览图**不会**被扫描器扫描，因此：
- ❌ 不会被发送给算法服务推理
- ❌ 不会因为无检测结果被删除
- ❌ 不会生成告警记录

### 2. 永久保留

预览图会一直保留在MinIO中，直到：
- 用户手动删除任务（连同所有文件）
- 用户手动删除MinIO中的预览图

### 3. 可重新生成

如果预览图不清晰或角度不佳：
1. 删除旧预览图
2. 重新添加任务
3. 系统会自动生成新预览图

---

## 🔧 技术实现

### Scanner过滤逻辑

```go
// scanNewImages 扫描MinIO中的新图片
func (s *Scanner) scanNewImages() ([]ImageInfo, error) {
    for object := range objectCh {
        // 1. 过滤非图片文件
        if !isImageFile(object.Key) {
            continue
        }
        
        // 2. 跳过预览图（不参与推理）
        filename := object.Key[strings.LastIndex(object.Key, "/")+1:]
        if strings.HasPrefix(filename, "preview_") {
            s.log.Debug("skipping preview image", slog.String("path", object.Key))
            continue  // ← 关键：跳过preview图片
        }
        
        // 3. 跳过配置文件
        if strings.HasSuffix(object.Key, ".json") {
            continue
        }
        
        // 4. 其他图片正常处理
        newImages = append(newImages, ImageInfo{...})
    }
}
```

### 文件类型过滤

**会被扫描的文件**：
- ✅ `20241017-143520.000.jpg` - 正常抽帧图片
- ✅ `20241017-143521.000.jpg` - 正常抽帧图片

**不会被扫描的文件**：
- ❌ `preview_20241017-143520.000.jpg` - 预览图
- ❌ `algo_config.json` - 算法配置
- ❌ `.keep` - 标记文件

---

## 📊 文件分类

### MinIO目录结构

```
frames/
└── 人数统计/
    └── cam_entrance_001/
        ├── preview_20241017-143520.000.jpg  ← 预览图（永久保留）
        ├── algo_config.json                 ← 配置文件（永久保留）
        ├── 20241017-143530.000.jpg          ← 正常图片（可能被删除）
        ├── 20241017-143531.000.jpg          ← 正常图片（可能被删除）
        └── 20241017-143532.000.jpg          ← 正常图片（可能被删除）
```

### 文件处理规则

| 文件类型 | 扫描 | 推理 | 删除 | 用途 |
|---------|------|------|------|------|
| `preview_*.jpg` | ❌ | ❌ | ❌ | 配置参考 |
| `algo_config.json` | ❌ | ❌ | ❌ | 配置存储 |
| `.keep` | ❌ | ❌ | ❌ | 路径标记 |
| `时间戳.jpg` | ✅ | ✅ | ⚠️ | 正常抽帧（无检测时删除） |

---

## 🎯 使用场景

### 场景1：正常使用

```
1. 添加任务
   → 生成 preview_20241017-143520.jpg

2. 配置算法（使用预览图）
   → 绘制区域
   → 保存配置

3. 启动任务
   → 生成 20241017-143530.jpg
   → 生成 20241017-143531.jpg
   → ...

4. AI推理
   → 只推理时间戳命名的图片
   → preview图片不参与推理
   → preview图片永久保留
```

### 场景2：更新配置

```
1. 停止任务
2. 点击"算法配置"
3. 使用原有预览图调整区域
4. 保存新配置
5. 重新启动任务
```

**说明**：预览图一直存在，可以反复使用！

### 场景3：预览图不清晰

```
1. 删除任务（预览图也会被删除）
2. 重新添加任务
3. 系统生成新预览图
4. 使用新预览图配置
```

---

## 💡 优势

### 1. 避免误删

预览图不参与推理流程，因此：
- 不会因为配置原因被删除
- 不会因为算法服务问题被删除
- 不会因为无检测结果被删除

### 2. 可重复使用

- 可以多次打开配置界面
- 每次都能看到同一张参考图
- 配置更加一致

### 3. 节省资源

- 预览图只生成一次
- 不需要重复抽帧
- 减少RTSP连接次数

### 4. 便于调试

- 预览图固定不变
- 便于对比配置效果
- 便于问题追踪

---

## 🔍 验证方法

### 1. 检查MinIO

访问MinIO控制台：
```
http://10.1.6.230:9000
```

路径：
```
images/frames/{task_type}/{task_id}/
```

应该看到：
```
preview_20241017-143520.000.jpg  ← 一直存在
algo_config.json                 ← 配置文件
20241017-143530.000.jpg          ← 正常图片
20241017-143531.000.jpg          ← 正常图片
...
```

### 2. 查看扫描日志

```bash
tail -f logs/sugar.log | grep "skipping preview"
```

应该看到：
```
[DEBUG] skipping preview image path=frames/人数统计/cam_001/preview_*.jpg
```

### 3. 验证推理流程

```bash
# 查看推理日志，不应该包含preview图片
tail -f logs/sugar.log | grep "inference"

# 应该只看到时间戳命名的图片
[INFO] inference result received image=frames/.../20241017-143530.jpg
```

---

## 📝 日志示例

### 正常扫描

```log
[INFO] found new images count=5

图片列表：
- frames/人数统计/cam_001/20241017-143530.000.jpg  ← 扫描
- frames/人数统计/cam_001/20241017-143531.000.jpg  ← 扫描
- frames/人数统计/cam_001/20241017-143532.000.jpg  ← 扫描
- frames/人数统计/cam_001/20241017-143533.000.jpg  ← 扫描
- frames/人数统计/cam_001/20241017-143534.000.jpg  ← 扫描

跳过的文件：
[DEBUG] skipping preview image path=frames/人数统计/cam_001/preview_*.jpg
```

---

## ⚠️ 注意事项

### 1. 预览图大小

预览图会永久保留，建议：
- 控制预览图质量（JPEG 85%）
- 分辨率不要太高（1280x720即可）
- 每个任务只占用约100-200KB

### 2. 定期清理

如果任务很多，建议定期清理：
- 删除已停用的任务
- 预览图会随任务一起删除

### 3. 手动管理

如需手动删除预览图：
```bash
# MinIO CLI
mc rm minio/images/frames/人数统计/cam_001/preview_*.jpg
```

**注意**：删除后需要重新添加任务才能重新生成。

---

## 🔄 完整生命周期

```
添加任务
    ↓
生成预览图 preview_*.jpg
    ↓
【预览图永久保留】
    ↓
用户配置算法（使用预览图）
    ↓
保存配置 algo_config.json
    ↓
启动任务
    ↓
正常抽帧 时间戳.jpg
    ↓
AI推理（只推理时间戳图片）
    ↓
无检测结果时删除时间戳图片
    ↓
【预览图仍然保留】
```

---

## 📚 相关文档

- **算法配置规范**: `doc/ALGORITHM_CONFIG_SPEC.md`
- **算法对接指南**: `doc/ALGORITHM_INTEGRATION_GUIDE.md`
- **用户使用手册**: `ALGO_CONFIG_USER_GUIDE.md`
- **界面优化说明**: `ALGO_CONFIG_UI_OPTIMIZATION.md`

---

## ✅ 检查清单

部署前检查：

- [x] Scanner跳过preview图片
- [x] Scanner跳过配置文件
- [x] 预览图不参与推理
- [x] 预览图永久保留
- [x] 日志记录完整

---

**预览图保留策略已实施！** ✅

预览图会永久保留作为配置参考，不会被AI推理流程删除。

