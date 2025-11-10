# 快速参考卡片

## 🔄 配置迁移（抽帧监控）

### 问题现象
```
❌ 旧配置无法使用
❌ 任务无法启动
❌ 状态显示异常
```

### 解决方案
```bash
# 方案1：自动迁移（推荐）✅
./easydarwin
# 系统会自动补全缺失字段，无需手动操作

# 方案2：手动迁移脚本
cd scripts && ./migrate_config.sh ../configs/config.toml
```

### 新增字段
```toml
[[frame_extractor.tasks]]
id = 'task_001'
# ... 其他字段 ...
config_status = 'configured'  # ⚠️ 新增
preview_image = ''            # ⚠️ 新增
```

---

## 🗑️ 告警图片删除机制

### 核心规则
```
✅ 有检测结果 (detection_count > 0)
   → 保存告警 + 推送消息 + 保留图片

❌ 无检测结果 (detection_count = 0)
   → 删除图片 + 不保存告警 + 不推送
```

### 配置开关
```toml
[ai_analysis]
save_only_with_detection = true   # 节省存储
# save_only_with_detection = false  # 保留所有（调试用）
```

### 删除场景
| 场景 | 是否删除 | 是否告警 |
|------|---------|---------|
| 有检测结果 | ❌ 不删除 | ✅ 产生告警 |
| 无检测结果 | ✅ 删除 | ❌ 不产生告警 |
| 队列满丢弃 | ✅ 删除 | ❌ 未推理 |
| 推理失败 | ✅ 删除 | ❌ 技术故障 |
| 无算法服务 | ✅ 删除 | ❌ 无法处理 |

### 结论
**已产生告警推送的图片不会被删除！** ✅

---

## 📋 配置检查清单

### 升级前
```bash
- [ ] 备份配置文件: cp config.toml config.toml.backup
- [ ] 备份数据库: cp data.db data.db.backup
- [ ] 记录当前版本: ./easydarwin --version
- [ ] 停止服务: systemctl stop easydarwin
```

### 升级后
```bash
- [ ] 替换可执行文件
- [ ] 启动服务: systemctl start easydarwin
- [ ] 查看迁移日志: tail -f logs/sugar.log | grep migration
- [ ] 验证Web界面: http://localhost:5066
- [ ] 检查任务状态: curl localhost:5066/api/frame-extractor/tasks
```

---

## 🔍 问题诊断

### 日志检查
```bash
# 查看迁移日志
tail -f logs/sugar.log | grep -E "(migration|frameextractor)"

# 查看删除日志
tail -f logs/sugar.log | grep -E "(delete|remove)"

# 查看告警日志
tail -f logs/sugar.log | grep -E "(alert|inference)"
```

### 常见错误

#### 错误1：任务无法启动
```
原因: config_status 缺失或无效
解决: 自动迁移会补全，或手动设置为 'configured'
```

#### 错误2：预览图显示失败
```
原因: preview_image 为空
解决: 系统会自动生成，或手动触发:
      curl -X POST localhost:5066/api/frame-extractor/tasks/{id}/preview
```

#### 错误3：图片被意外删除
```
原因: save_only_with_detection = true 且无检测结果
解决: 检查算法返回的 total_count 字段
      或临时设置 save_only_with_detection = false
```

---

## ⚙️ 推荐配置

### 生产环境
```toml
[frame_extractor]
enable = true
interval_ms = 1000
store = 'minio'

[ai_analysis]
enable = true
save_only_with_detection = true   # 节省存储
scan_interval_sec = 5
max_concurrent_infer = 20

[[frame_extractor.tasks]]
id = 'task_001'
task_type = '人数统计'
rtsp_url = 'rtsp://...'
interval_ms = 1000
output_path = 'task_001'
enabled = true
config_status = 'configured'
preview_image = ''
```

### 开发测试
```toml
[frame_extractor]
enable = true
interval_ms = 3000              # 降低频率
store = 'local'                 # 使用本地存储

[ai_analysis]
enable = true
save_only_with_detection = false  # 保留所有图片
scan_interval_sec = 10          # 降低扫描频率
max_concurrent_infer = 3        # 降低并发

[[frame_extractor.tasks]]
enabled = false                 # 初始不启动
config_status = 'unconfigured'  # 标记为未配置
```

---

## 📞 获取帮助

### 文档
- [完整迁移指南](doc/CONFIG_MIGRATION_GUIDE.md)
- [升级指南](UPGRADE_GUIDE.md)
- [告警机制文档](AI_INFERENCE_AUTO_DELETE.md)

### 日志
```bash
# 完整日志
tail -f logs/sugar.log

# 过滤特定模块
tail -f logs/sugar.log | grep frameextractor
tail -f logs/sugar.log | grep aianalysis
```

### 联系方式
- 提交Issue（推荐）
- 查看已知问题

---

**最后更新**: 2025-10-22  
**适用版本**: v2.0.0+

