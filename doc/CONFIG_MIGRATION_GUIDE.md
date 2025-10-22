# 抽帧监控配置迁移指南

## 问题说明

如果您从旧版本升级到新版本，可能会遇到**抽帧监控无法复用之前配置**的问题。

### 原因分析

新版本在 `FrameExtractTask` 中增加了两个新字段：

```toml
[[frame_extractor.tasks]]
id = 'task_001'
task_type = '人数统计'
rtsp_url = 'rtsp://...'
interval_ms = 1000
output_path = 'task_001'
enabled = true
config_status = 'configured'  # ⚠️ 新增字段
preview_image = ''            # ⚠️ 新增字段
```

旧配置文件缺少这两个字段，可能导致：
- 任务无法正常启动
- 任务状态显示异常
- 预览图片功能不可用

## 解决方案

### 方案1：自动迁移（推荐）✅

**新版本已内置自动迁移功能**，启动时会自动补全缺失字段。

只需直接启动服务即可：

```bash
# 直接使用旧配置启动
./easydarwin

# 查看日志，确认迁移成功
tail -f logs/sugar.log | grep "config migration"
```

**日志示例：**
```
INFO config migration completed migrated_tasks=5 total_tasks=5
```

### 方案2：手动迁移（可选）

如果您希望手动更新配置文件，可以使用提供的迁移脚本：

#### 使用Shell脚本

```bash
cd scripts
./migrate_config.sh ../configs/config.toml
```

#### 使用Go脚本

```bash
cd scripts
go run migrate_config.go ../configs/config.toml
```

**迁移后的配置示例：**

```toml
# 迁移前
[[frame_extractor.tasks]]
id = 'task_001'
task_type = '人数统计'
rtsp_url = 'rtsp://127.0.0.1:15544/live/stream_1'
interval_ms = 1000
output_path = 'task_001'
enabled = true

# 迁移后（自动添加）
[[frame_extractor.tasks]]
id = 'task_001'
task_type = '人数统计'
rtsp_url = 'rtsp://127.0.0.1:15544/live/stream_1'
interval_ms = 1000
output_path = 'task_001'
enabled = true
config_status = 'configured'  # ✅ 自动添加
preview_image = ''            # ✅ 自动添加
```

## 字段说明

### config_status

**作用**：标识任务的配置状态

**可选值**：
- `unconfigured`: 未配置（新建任务的初始状态）
- `configured`: 已配置（可以正常运行）

**自动迁移规则**：
- 如果任务 `enabled = true`，自动设置为 `configured`
- 如果任务 `enabled = false`，自动设置为 `unconfigured`

### preview_image

**作用**：保存任务的预览图片路径

**说明**：
- 新建任务时自动抽取一帧作为预览
- 用于Web界面展示任务画面
- 留空时系统会自动生成

**自动迁移规则**：
- 旧任务的 `preview_image` 初始为空
- 系统会在需要时自动生成预览图

## 验证迁移结果

### 1. 检查日志

```bash
tail -f logs/sugar.log | grep -E "(migration|frameextractor)"
```

**成功日志：**
```
INFO config migration completed migrated_tasks=5 total_tasks=5
INFO frameextractor started default_interval_ms=1000 store=minio
INFO starting task task=task_001 rtsp=rtsp://...
```

### 2. 检查任务状态

访问Web管理界面：
```
http://localhost:5066/#/frame-extractor
```

确认：
- ✅ 所有任务显示正常
- ✅ 任务状态正确（运行中/已停止）
- ✅ 配置状态显示为"已配置"

### 3. API检查

```bash
# 获取任务列表
curl http://localhost:5066/api/frame-extractor/tasks

# 检查返回的JSON中是否包含新字段
```

## 常见问题

### Q1: 迁移后任务无法启动？

**原因**：可能是配置验证失败

**解决**：
1. 查看日志中的 `config validation warning`
2. 检查必填字段：`id`, `rtsp_url`
3. 确保 `config_status` 为 `configured`

```bash
# 查看警告信息
tail -f logs/sugar.log | grep "validation warning"
```

### Q2: 旧任务的预览图丢失？

**原因**：旧配置没有保存预览图路径

**解决**：
1. 预览图会在任务重启时自动生成
2. 或手动触发预览图生成（通过API）

```bash
# 通过API重新生成预览图
curl -X POST http://localhost:5066/api/frame-extractor/tasks/{task_id}/preview
```

### Q3: 能否回滚到旧版本？

**可以**。迁移脚本会自动创建备份：

```bash
# 查看备份文件
ls -la configs/config.toml.backup*

# 恢复备份（如果使用了手动迁移脚本）
cp configs/config.toml.backup configs/config.toml
```

**注意**：自动迁移不会修改配置文件，只在内存中补全字段。

### Q4: 如何确认是否需要迁移？

检查配置文件中的任务是否包含新字段：

```bash
# 检查是否有 config_status 字段
grep "config_status" configs/config.toml

# 如果没有输出，说明需要迁移
```

## 最佳实践

### 1. 升级前备份

```bash
# 备份配置文件
cp configs/config.toml configs/config.toml.backup.$(date +%Y%m%d)

# 备份数据库（如果有）
cp configs/data.db configs/data.db.backup.$(date +%Y%m%d)
```

### 2. 新建任务的建议配置

```toml
[[frame_extractor.tasks]]
id = 'unique_task_id'           # 必填：唯一ID
task_type = '人数统计'            # 必填：任务类型
rtsp_url = 'rtsp://...'         # 必填：RTSP地址
interval_ms = 1000              # 可选：抽帧间隔（毫秒）
output_path = 'unique_task_id'  # 可选：输出路径（默认=id）
enabled = false                 # 初始设为false，配置完成后再启动
config_status = 'unconfigured'  # 初始为unconfigured
preview_image = ''              # 留空，系统自动生成
```

### 3. 配置管理流程

**推荐工作流程**：

1. **新建任务**（Web界面或API）
   - 状态：`unconfigured`
   - 启用：`false`
   - 系统自动抽取预览图

2. **配置算法参数**
   - 在Web界面配置区域、算法参数
   - 配置保存到MinIO

3. **启动任务**
   - 状态自动变更为：`configured`
   - 启用：`true`
   - 开始持续抽帧

## 技术细节

### 自动迁移实现

代码位置：`internal/plugin/frameextractor/compat.go`

```go
func MigrateConfig(cfg *conf.FrameExtractorConfig, logger *slog.Logger) {
    for i := range cfg.Tasks {
        task := &cfg.Tasks[i]
        
        // 补全 ConfigStatus
        if task.ConfigStatus == "" {
            if task.Enabled {
                task.ConfigStatus = "configured"
            } else {
                task.ConfigStatus = "unconfigured"
            }
        }
        
        // PreviewImage 留空，由系统自动生成
    }
}
```

### 配置验证

```go
func ValidateConfig(cfg *conf.FrameExtractorConfig) []string {
    // 检查必填字段
    // 检查字段有效性
    // 返回警告列表
}
```

## 更新日志

### v2.0.0 (2025-01-xx)

**新增功能**：
- ✅ 自动配置迁移
- ✅ 配置验证
- ✅ 任务配置状态管理
- ✅ 预览图自动生成

**破坏性变更**：
- 新增必需字段：`config_status`
- 新增可选字段：`preview_image`

**兼容性**：
- ✅ 自动兼容旧版本配置
- ✅ 无需手动修改配置文件

---

## 获取帮助

如遇到问题，请：

1. **查看日志**：`logs/sugar.log`
2. **查看文档**：本指南及其他文档
3. **提交Issue**：提供日志和配置文件（脱敏后）

---

**最后更新**：2025-10-22

