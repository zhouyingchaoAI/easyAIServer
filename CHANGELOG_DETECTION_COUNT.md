# 更新日志 - 智能告警增强功能

## 📅 2024-10-17 (v1.2.1)

### 🔧 队列丢弃优化

**功能描述：**
优化推理队列的图片丢弃逻辑，丢弃图片时同步删除MinIO中对应的文件，避免存储空间浪费。

**核心特性：**
- ✅ **同步删除**：丢弃图片时自动删除MinIO文件
- ✅ **异步处理**：使用goroutine异步删除，不阻塞队列
- ✅ **支持所有策略**：丢弃最旧/最新/清空队列都会删除文件
- ✅ **节省空间**：可额外节省60%的存储空间（丢弃图片部分）
- ✅ **错误容忍**：删除失败不影响推理流程

**修改文件：**
- 后端：
  - `internal/plugin/aianalysis/queue.go` - 添加MinIO删除逻辑
  - `internal/plugin/aianalysis/service.go` - 传递MinIO客户端给队列

**适用场景：**
- 推理速度慢于抽帧，队列经常积压
- 长期运行的生产环境
- 存储空间有限的场景

**详细文档：** [FEATURE_QUEUE_DROP_OPTIMIZATION.md](doc/FEATURE_QUEUE_DROP_OPTIMIZATION.md)

### 📋 total_count 参数优先级调整

**功能描述：**
调整检测个数的提取优先级，**优先使用算法返回的 `total_count` 参数**。

**核心特性：**
- ✅ **优先级最高**：`total_count` 作为检测个数的首选字段
- ✅ **明确控制**：算法服务可明确指定检测总数
- ✅ **性能提升**：直接读取字段，无需计算数组长度
- ✅ **支持复杂场景**：可只返回部分检测详情，total_count 仍准确

**提取优先级：**
1. `result.total_count` ⭐⭐⭐⭐⭐（最高优先级）
2. `result.count` ⭐⭐⭐⭐
3. `result.num` ⭐⭐⭐
4. `result.detections.length` ⭐⭐
5. `result.objects.length` ⭐

**修改文件：**
- 后端：
  - `internal/plugin/aianalysis/scheduler.go` - 调整 `extractDetectionCount` 函数
- 示例：
  - `examples/algorithm_service.py` - 添加 `total_count` 返回
- 文档：
  - 新增多个文档说明 `total_count` 使用

**重要提示：**
⚠️ **当 `total_count = 0` 时，原始图片会被删除！** 算法服务必须确保只在真正无检测结果时返回 0。

**详细文档：**
- [total_count 参数说明](UPDATE_SUMMARY_TOTAL_COUNT.md)
- [算法快速参考](ALGORITHM_QUICK_REFERENCE.md)
- [算法返回格式规范](doc/ALGORITHM_RESPONSE_FORMAT.md)
- [total_count 详细说明](doc/TOTAL_COUNT_PARAMETER.md)

---

## 📅 2024-10-17 (v1.2.0)

### 🆕 只保存有检测结果的告警

**功能描述：**
新增智能过滤功能，只保存和推送有检测结果的告警，没有检测到目标的图片将被自动删除。

**核心特性：**
- ✅ **自动过滤**：检测个数为 0 的告警不保存到数据库
- ✅ **智能删除**：无检测结果的图片自动从 MinIO 删除
- ✅ **节省空间**：可节省 70-90% 的存储空间
- ✅ **可配置**：通过 `save_only_with_detection` 配置开关控制
- ✅ **不推送空告警**：避免无效消息推送到 Kafka

**配置示例：**
```toml
[ai_analysis]
save_only_with_detection = true  # 启用功能（默认推荐）
```

**修改文件：**
- 后端：
  - `internal/plugin/aianalysis/scheduler.go` - 添加检测结果判断和图片删除逻辑
  - `internal/plugin/aianalysis/service.go` - 传递配置参数
  - `internal/conf/model.go` - 新增 `SaveOnlyWithDetection` 配置项
  - `configs/config.toml` - 添加配置示例

**存储节省示例：**
```
场景：24小时监控，每秒1帧，有人时间10%
- 关闭功能：25.9 GB
- 启用功能：2.6 GB
- 节省：23.3 GB (90%)
```

**详细文档：** [FEATURE_SAVE_ONLY_WITH_DETECTION.md](doc/FEATURE_SAVE_ONLY_WITH_DETECTION.md)

---

## 📅 2024-10-17 (v1.1.1)

### 🆕 任务ID自动下拉选择

**功能描述：**
告警列表的任务ID筛选器从手动输入改为自动下拉选择，提升用户体验。

**改进点：**
- ✅ 自动获取所有任务ID并显示在下拉列表中
- ✅ 支持搜索过滤（模糊匹配，不区分大小写）
- ✅ 任务ID按字母顺序自动排序
- ✅ 避免因拼写错误导致查询失败

**修改文件：**
- 后端：
  - `internal/web/api/ai_analysis.go` - 新增 `/alerts/task_ids` API
  - `internal/data/alert.go` - 新增 `GetDistinctTaskIDs` 函数
- 前端：
  - `web-src/src/api/alert.js` - 新增 `getTaskIds` 方法
  - `web-src/src/views/alerts/index.vue` - 改为下拉选择框

**详细文档：** [FEATURE_TASK_ID_DROPDOWN.md](doc/FEATURE_TASK_ID_DROPDOWN.md)

---

## 📅 2024-10-17 (v1.1.0)

### 🎉 新增功能

#### 1. 智能告警增加检测实例个数统计

**后端改动：**

- ✅ `internal/data/model/alert.go`
  - 添加 `DetectionCount` 字段到 Alert 模型
  - 添加 `MinDetections` 和 `MaxDetections` 过滤条件

- ✅ `internal/data/alert.go`
  - 在 `ListAlerts` 函数中添加按检测个数过滤的逻辑

- ✅ `internal/plugin/aianalysis/scheduler.go`
  - 添加 `extractDetectionCount` 函数，从推理结果中智能提取检测个数
  - 支持多种格式：`detections`、`objects`、`count`、`num`
  - 在保存告警时自动填充 `detection_count` 字段

- ✅ `internal/plugin/aianalysis/alert.go`
  - 系统告警添加 `DetectionCount` 字段（默认为0）

**前端改动：**

- ✅ `web-src/src/views/alerts/index.vue`
  - 添加"检测数"列到告警列表表格
  - 添加"最少检测数"和"最多检测数"过滤器
  - 添加"重置"按钮清除所有筛选条件
  - 在告警详情中显示检测个数
  - 使用徽章组件美化检测个数显示

#### 2. 算法服务界面优化

**界面改进：**

- ✅ `web-src/src/views/alerts/services.vue`
  - 添加横向滚动支持（`scroll: { x: 1400 }`）
  - 优化列宽度，避免内容挤压
  - 服务ID和端点添加省略号和 Tooltip
  - 任务类型标签支持自动换行和滚动
  - 优化长文本显示，防止超出界面

### 📋 文件清单

**修改的文件：**
1. `internal/data/model/alert.go` - Alert 模型
2. `internal/data/alert.go` - 数据访问层
3. `internal/plugin/aianalysis/scheduler.go` - 推理调度器
4. `internal/plugin/aianalysis/alert.go` - 告警管理器
5. `web-src/src/views/alerts/index.vue` - 告警列表界面
6. `web-src/src/views/alerts/services.vue` - 算法服务界面

**新增的文件：**
1. `doc/DATABASE_MIGRATION.md` - 数据库迁移指南
2. `doc/FEATURE_UPDATE_DETECTION_COUNT.md` - 功能详细说明
3. `CHANGELOG_DETECTION_COUNT.md` - 本文件

### 🔧 数据库变更

```sql
-- 添加检测个数字段
ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;

-- 添加索引（提升查询性能）
CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
```

### 📊 功能对比

| 功能 | 更新前 | 更新后 |
|------|--------|--------|
| 检测个数显示 | ❌ 无 | ✅ 在列表和详情中显示 |
| 按检测个数过滤 | ❌ 不支持 | ✅ 支持范围过滤 |
| 检测个数统计 | ❌ 需手动分析 | ✅ 自动提取和存储 |
| 算法服务界面 | ⚠️ 内容超出 | ✅ 响应式布局 |
| 过滤器重置 | ⚠️ 手动清除 | ✅ 一键重置 |

### 🎯 使用示例

#### 示例 1：查询高密度人群告警

```bash
# API 查询
curl "http://localhost:5066/api/v1/ai_analysis/alerts?task_type=人数统计&min_detections=10&page=1&page_size=20"

# Web 界面
# 1. 进入告警列表页面
# 2. 选择任务类型：人数统计
# 3. 设置最少检测数：10
# 4. 点击"查询"
```

#### 示例 2：过滤空告警

```bash
# 只显示检测到目标的告警
curl "http://localhost:5066/api/v1/ai_analysis/alerts?min_detections=1"
```

#### 示例 3：统计检测情况

```sql
-- 查看各任务类型的检测统计
SELECT 
    task_type,
    COUNT(*) as total_alerts,
    AVG(detection_count) as avg_detections,
    MAX(detection_count) as max_detections
FROM alerts
WHERE created_at >= datetime('now', '-1 day')
GROUP BY task_type
ORDER BY avg_detections DESC;
```

### 🚀 升级指南

#### 1. 现有用户（有历史数据）

```bash
# 1. 停止服务
pkill -f easydarwin

# 2. 备份数据库
cp configs/data.db configs/data.db.backup

# 3. 更新数据库结构
sqlite3 configs/data.db
> ALTER TABLE alerts ADD COLUMN detection_count INTEGER DEFAULT 0;
> CREATE INDEX idx_alerts_detection_count ON alerts(detection_count);
> .exit

# 4. 更新代码
git pull

# 5. 重新编译
make build/linux

# 6. 启动服务
cd build/EasyDarwin-lin-*
./easydarwin
```

#### 2. 新用户

直接编译启动即可，GORM 会自动创建包含新字段的表结构：

```bash
make build/linux
cd build/EasyDarwin-lin-*
./easydarwin
```

### ✅ 验证清单

- [ ] 数据库 `alerts` 表包含 `detection_count` 字段
- [ ] 告警列表显示"检测数"列
- [ ] 筛选器可以按检测个数过滤
- [ ] 告警详情显示检测个数
- [ ] 算法服务列表不会超出界面
- [ ] 新的告警记录自动填充检测个数

### 📝 注意事项

1. **历史数据**：已存在的告警记录 `detection_count` 默认为 0
2. **算法服务**：需要返回包含检测结果的 JSON 数据
3. **兼容性**：支持多种推理结果格式，自动适配
4. **性能**：已添加索引，查询性能不受影响

### 🐛 已知问题

无

### 📖 相关文档

- [功能详细说明](doc/FEATURE_UPDATE_DETECTION_COUNT.md)
- [数据库迁移指南](doc/DATABASE_MIGRATION.md)
- [完整使用文档](README_CN.md)

---

**版本**：v1.1.0  
**提交者**：yanying team  
**审核者**：待定

