# 抽帧监控 - 历史信息清零功能

## 功能概述

在抽帧监控页面新增**统计数据清零**功能，允许用户手动重置推理统计历史数据，方便在优化调整后重新统计性能指标。

## 新增功能

### 1. 统计数据清零按钮

**位置**：抽帧监控页面右上角工具栏

**按钮样式**：红色危险按钮，带清除图标

**功能**：一键清零所有AI推理统计数据

### 2. 清零确认对话框

**安全机制**：点击清零按钮时弹出确认对话框

**确认内容**：
- 将清零的数据项列表
- 不可恢复警告提示

### 3. 清零的数据项

点击确认后，以下数据将被重置为0：

#### AI推理队列统计
- ✓ 累计处理数（processed_total）
- ✓ 累计丢弃数（dropped_total）
- ✓ 图片丢弃率（drop_rate）

#### 推理性能统计
- ✓ 平均推理时间（avg_inference_ms）
- ✓ 最大推理时间（max_inference_ms）
- ✓ 推理成功次数（total_inferences）
- ✓ 推理失败次数（failed_inferences）
- ✓ 慢推理计数（slow_count）

#### 保留的数据
- ✗ 当前队列大小（queue_size）- 保持不变
- ✗ 队列最大容量（queue_max_size）- 保持不变
- ✗ 队列策略（strategy）- 保持不变

## 使用场景

### 场景1：性能优化后重新统计
```
1. 增加了算法服务实例
2. 点击"清零统计"
3. 观察优化后的丢弃率
```

### 场景2：系统调试
```
1. 调整了抽帧间隔
2. 清零历史数据
3. 重新评估新配置的效果
```

### 场景3：定期重置
```
1. 每天/每周清零一次
2. 获取最近时段的统计数据
3. 对比不同时期的性能
```

## 操作步骤

### 步骤1：访问监控页面
导航至：**抽帧监控** (`/frame-extractor/monitor`)

### 步骤2：点击清零按钮
点击右上角的红色 **"清零统计"** 按钮

### 步骤3：确认操作
在弹出的确认对话框中：
1. 查看将要清零的数据项
2. 阅读不可恢复警告
3. 点击 **"确定"** 执行清零

### 步骤4：验证结果
- 页面自动刷新数据
- 所有累计统计归零
- 显示成功提示消息

## API接口

### 重置推理统计
```http
POST /api/v1/ai_analysis/inference_stats/reset

Response:
{
  "ok": true,
  "message": "推理统计数据已清零"
}
```

### 错误响应
```http
# AI服务未启用
HTTP 500
{
  "error": "AI analysis service not ready"
}

# 服务未初始化
HTTP 500
{
  "error": "inference service not initialized"
}
```

## 操作日志

每次清零操作都会记录到系统日志：

### 成功日志
```json
{
  "level": "info",
  "msg": "inference stats reset successfully",
  "remote_addr": "10.1.4.246"
}
```

### 请求日志
```json
{
  "level": "warn",
  "msg": "inference stats reset requested",
  "remote_addr": "10.1.4.246",
  "user_agent": "Mozilla/5.0..."
}
```

### 队列重置日志
```json
{
  "level": "info",
  "msg": "inference queue stats reset",
  "remaining_queue_size": 15
}
```

### 性能监控重置日志
```json
{
  "level": "info",
  "msg": "performance monitor stats reset"
}
```

## 注意事项

### ⚠️ 重要提醒

1. **不可恢复**：清零操作无法撤销，历史数据将永久丢失
2. **仅清零统计**：不影响当前队列中的待处理图片
3. **不影响服务**：不会停止或重启任何服务
4. **仅清零计数**：不删除任何实际数据（告警、图片等）

### 🎯 最佳实践

1. **定期清零**：建议每天或每周清零一次，便于观察趋势
2. **优化后清零**：调整配置或增加服务后清零，评估改进效果
3. **记录指标**：清零前截图保存关键指标，便于对比
4. **配合告警**：结合告警日志全面评估系统状态

### 🔍 清零前检查清单

- [ ] 确认当前统计数据已记录（截图或导出）
- [ ] 确认清零是必要的（不是误操作）
- [ ] 了解清零后无法恢复历史数据
- [ ] 准备好观察清零后的新数据

## 使用示例

### 示例1：优化后评估

**背景**：图片丢弃率74%，严重不足

**操作流程**：
```
1. 记录当前数据：
   - 丢弃率：74.45%
   - 累计丢弃：854张
   - 累计处理：293张

2. 优化措施：
   - 启动3个新的算法服务实例
   - 调整抽帧间隔从200ms改为1000ms

3. 清零统计：
   - 点击"清零统计"按钮
   - 确认操作

4. 观察新数据：
   - 等待10-30分钟
   - 查看新的丢弃率
   - 对比优化效果
```

**预期结果**：
- 丢弃率应降至 < 10%
- 队列使用率 < 50%
- 推理时间稳定

### 示例2：日常监控

**每日清零流程**：
```
每天上午9:00：
1. 查看昨日统计数据
2. 记录关键指标到Excel
3. 截图保存监控面板
4. 点击清零开始新一天的统计
```

## 技术实现

### 后端实现

#### 1. InferenceQueue.ResetStats()
```go
// 重置队列统计
- droppedCount = 0
- processedCount = 0
- lastAlertTime 清空
```

#### 2. PerformanceMonitor.Reset()
```go
// 重置性能统计
- totalInferences = 0
- failedInferences = 0
- totalInferenceTime = 0
- avgInferenceTime = 0
- maxInferenceTime = 0
- slowCount = 0
```

#### 3. Service.ResetInferenceStats()
```go
// 统一调用以上两个重置方法
queue.ResetStats()
monitor.Reset()
```

### 前端实现

#### 1. 清零按钮
```vue
<a-button danger>
  <template #icon><ClearOutlined /></template>
  清零统计
</a-button>
```

#### 2. 确认对话框
```vue
<a-popconfirm
  title="确认清零统计数据？"
  @confirm="resetStats"
>
  <!-- 详细说明 -->
</a-popconfirm>
```

#### 3. 清零方法
```javascript
const resetStats = async () => {
  const response = await request({ 
    url: '/ai_analysis/inference_stats/reset', 
    method: 'post' 
  })
  message.success('统计数据已清零')
  await fetchStats() // 刷新显示
}
```

## 修改文件列表

### 后端
- `internal/plugin/aianalysis/queue.go` - 添加 ResetStats()
- `internal/plugin/aianalysis/monitor.go` - 完善 Reset()
- `internal/plugin/aianalysis/service.go` - 添加 ResetInferenceStats()
- `internal/web/api/ai_analysis.go` - 添加清零API接口

### 前端
- `web-src/src/views/frame-extractor/monitor.vue` - 添加清零按钮和逻辑

## 编译和部署

### 编译
```bash
cd /code/EasyDarwin
make
```

### 前端构建
```bash
cd web-src
npm run build
```

### 重启服务
```bash
# 停止旧进程
pkill -f easydarwin

# 启动新版本
./build/EasyDarwin-lin-v8.3.3-*/easydarwin.com &
```

### 验证
```bash
# 测试清零API
curl -X POST http://localhost:5066/api/v1/ai_analysis/inference_stats/reset

# 预期返回
{"ok":true,"message":"推理统计数据已清零"}
```

## 常见问题

### Q1: 清零后队列大小还显示数字？
**A**: 正常现象。清零只重置累计统计，不清空当前队列中的待处理图片。

### Q2: 清零后能恢复吗？
**A**: 不能。历史统计数据会永久丢失，请清零前做好记录。

### Q3: 多久清零一次比较好？
**A**: 建议：
- 开发测试阶段：每次优化后清零
- 生产环境：每天或每周清零一次
- 特殊场景：根据需要随时清零

### Q4: 清零会影响正在运行的任务吗？
**A**: 不会。清零只重置统计计数器，不影响：
- 抽帧任务继续运行
- 队列中的图片继续处理
- 算法服务继续工作

## 扩展功能（后续）

- [ ] 支持导出清零前的统计报告
- [ ] 添加定时自动清零功能
- [ ] 支持分任务类型清零
- [ ] 清零历史记录查询

## 相关文档

- [AI推理队列监控](./INFERENCE_QUEUE_MONITOR.md)
- [抽帧服务监控](./FRAME_EXTRACTOR_MONITOR.md)

