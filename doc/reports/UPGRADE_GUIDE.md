# 升级指南

## 抽帧监控配置兼容性问题已解决 ✅

### 问题

升级到新版本后，旧的抽帧监控配置可能无法直接使用，因为新版本增加了必需的字段：
- `config_status` - 任务配置状态
- `preview_image` - 预览图片路径

### 解决方案

**已内置自动迁移功能**，无需手动操作！

```bash
# 直接启动即可，系统会自动迁移配置
./easydarwin
```

**系统会自动：**
1. ✅ 检测旧配置缺失的字段
2. ✅ 自动补全默认值
3. ✅ 记录迁移日志
4. ✅ 验证配置有效性

**日志示例：**
```
INFO  config migration completed  migrated_tasks=5 total_tasks=5
INFO  frameextractor started  default_interval_ms=1000 store=minio
```

### 字段自动补全规则

| 字段 | 旧配置 | 新配置（自动设置） |
|------|--------|-------------------|
| `config_status` | (不存在) | `configured` (如果enabled=true)<br>`unconfigured` (如果enabled=false) |
| `preview_image` | (不存在) | `""` (空字符串，后续自动生成) |
| `task_type` | (不存在) | 自动设置为第一个任务类型 |
| `output_path` | (不存在) | 自动设置为任务ID |

### 验证升级结果

```bash
# 查看迁移日志
tail -f logs/sugar.log | grep "migration"

# 访问Web界面
open http://localhost:5066/#/frame-extractor

# API检查
curl http://localhost:5066/api/frame-extractor/tasks
```

### 手动迁移（可选）

如果您希望永久修改配置文件：

```bash
cd scripts
./migrate_config.sh ../configs/config.toml
```

### 详细文档

查看完整的迁移指南：[doc/CONFIG_MIGRATION_GUIDE.md](doc/CONFIG_MIGRATION_GUIDE.md)

---

## 告警图片删除机制说明 ℹ️

### 问题：产生告警的图片会被删除吗？

**不会！** 已产生告警推送的图片不会被删除。

### 删除规则

图片只会在以下情况被删除：

1. **无检测结果** (`detection_count = 0`)
   - 配置：`save_only_with_detection = true`
   - 删除前：不保存告警记录
   - 删除前：不推送消息

2. **队列满时丢弃** (未推理的图片)
   - 队列容量已满
   - 还未进入推理流程

3. **推理失败** (技术故障)
   - 预签名URL生成失败
   - 算法服务返回错误

4. **无算法服务** (无法处理)
   - 没有匹配的算法

### 保护机制

```go
// 代码逻辑（scheduler.go）
if detectionCount == 0 {
    // 删除图片，不保存告警
    deleteImage(image.Path)
    return
}

// 有检测结果，保存告警（图片不会被删除）
alert := &Alert{...}
data.CreateAlert(alert)
mq.PublishAlert(alert)
```

**结论**：告警记录和图片删除是**互斥**的，只有一个会发生。

### 配置说明

```toml
[ai_analysis]
save_only_with_detection = true  # 只保存有检测结果的告警

# true:  删除无检测结果的图片，节省存储空间
# false: 保留所有图片，用于调试和分析
```

**推荐配置**：
- 生产环境：`true` (节省存储)
- 开发测试：`false` (便于调试)

---

## 常见问题

### Q: 升级需要停服吗？

A: 是的，建议：
1. 备份配置文件和数据库
2. 停止服务
3. 替换可执行文件
4. 启动服务（自动迁移）

### Q: 旧配置会被修改吗？

A: **不会**。自动迁移只在内存中补全字段，不修改配置文件。如果需要永久保存，使用迁移脚本。

### Q: 如何回滚？

A: 
```bash
# 停止服务
systemctl stop easydarwin

# 恢复旧版本
cp easydarwin.backup easydarwin

# 启动服务
systemctl start easydarwin
```

### Q: 数据库需要迁移吗？

A: 不需要。抽帧监控配置存储在 `config.toml` 中，不涉及数据库迁移。

---

**版本**: v2.0.0  
**更新日期**: 2025-10-22

