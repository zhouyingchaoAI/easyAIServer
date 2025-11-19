# MinIO监控页面实现说明

## ✅ 已实现的功能

### 1. 后端API更新

**文件**: `internal/plugin/aianalysis/service.go`

**更新内容**:
- ✅ 在 `InferenceStats` 结构体中添加MinIO监控字段
- ✅ 在 `GetInferenceStats` 方法中从 `PerformanceMonitor` 获取MinIO监控指标
- ✅ 返回完整的MinIO操作监控数据

**新增字段**:
```go
MinIOMoveTotal       int64   // 总移动次数
MinIOMoveSuccess     int64   // 成功次数
MinIOMoveFailed      int64   // 失败次数
MinIOMoveAvgTimeMs   float64 // 平均耗时（毫秒）
MinIOMoveMaxTimeMs   int64   // 最大耗时（毫秒）
MinIOMoveSuccessRate float64 // 成功率（0.0-1.0）
```

### 2. 前端API接口

**文件**: `web-src/src/api/aiAnalysis.js`

**功能**:
- ✅ `getInferenceStats()` - 获取推理统计信息（包含MinIO监控指标）
- ✅ `resetInferenceStats()` - 重置统计数据
- ✅ `listServices()` - 获取算法服务列表
- ✅ `getLoadBalanceInfo()` - 获取负载均衡信息

### 3. MinIO监控页面

**文件**: `web-src/src/views/ai-analysis/minio-monitor.vue`

**功能特性**:
- ✅ 实时显示MinIO图片移动监控指标
- ✅ 自动刷新（可配置刷新间隔：1秒/3秒/5秒/10秒）
- ✅ 性能指标可视化（颜色编码）
- ✅ 性能状态提示

**显示内容**:
1. **概览统计卡片**：
   - 总移动次数
   - 成功次数
   - 失败次数
   - 成功率

2. **性能指标**：
   - 平均响应时间
   - 最大响应时间
   - 并发限制

3. **性能趋势**：
   - 详细统计信息
   - 性能状态提示

### 4. 路由配置

**文件**: `web-src/src/router/rootRoute.js`

**路由**:
- 路径: `/ai-analysis/minio-monitor`
- 名称: `MinIOMonitor`
- 标题: `MinIO监控`
- 图标: `mdi:database`

## 📊 页面功能说明

### 概览统计

- **总移动次数**: 显示所有图片移动操作的总数
- **成功次数**: 显示成功完成的移动操作数（绿色）
- **失败次数**: 显示失败的移动操作数（红色，如果有失败）
- **成功率**: 显示成功率的百分比（颜色编码：绿色≥99%，橙色≥95%，红色<95%）

### 性能指标

- **平均响应时间**: MinIO操作的平均耗时
  - 绿色: < 200ms（正常）
  - 橙色: 200-500ms（需要注意）
  - 红色: > 500ms（有问题）

- **最大响应时间**: MinIO操作的最大耗时
  - 绿色: < 1000ms（正常）
  - 橙色: 1000-2000ms（需要注意）
  - 红色: > 2000ms（有问题）

- **并发限制**: 当前配置的图片移动最大并发数（默认50）

### 性能状态提示

根据成功率和响应时间自动显示性能状态：
- ✅ **正常**: 成功率≥99% 且 平均响应时间<200ms
- ⚠️ **警告**: 成功率<95% 或 平均响应时间>500ms

## 🔍 如何访问

### 方式1：通过菜单

1. 启动前端服务
2. 在左侧菜单中找到 **"MinIO监控"** 菜单项
3. 点击进入监控页面

### 方式2：直接访问URL

```
http://localhost:端口/#/ai-analysis/minio-monitor
```

### 方式3：通过API

```bash
# 获取监控数据
curl http://localhost:5066/api/v1/ai_analysis/inference_stats | jq

# 查看MinIO相关指标
curl http://localhost:5066/api/v1/ai_analysis/inference_stats | jq '.minio_move_*'
```

## 📈 监控指标说明

### 正常情况

- **成功率**: ≥ 99%
- **平均响应时间**: < 200ms
- **最大响应时间**: < 1000ms

### 需要关注

- **成功率**: 95% - 99%
- **平均响应时间**: 200ms - 500ms
- **最大响应时间**: 1000ms - 2000ms

### 有问题

- **成功率**: < 95%
- **平均响应时间**: > 500ms
- **最大响应时间**: > 2000ms

## 🎨 界面特性

1. **自动刷新**: 默认每3秒自动刷新数据
2. **颜色编码**: 根据性能指标自动显示不同颜色
3. **响应式设计**: 支持不同屏幕尺寸
4. **实时更新**: 数据实时更新，无需手动刷新

## 📝 使用建议

1. **监控频率**: 建议设置为3-5秒刷新一次
2. **关注指标**: 
   - 成功率（应该≥99%）
   - 平均响应时间（应该<200ms）
3. **异常处理**: 
   - 如果成功率<95%，检查MinIO服务器状态
   - 如果响应时间>500ms，考虑优化MinIO配置或降低并发数

## ✅ 实现总结

**实现状态**: ✅ **已完成**

**实现内容**:
1. ✅ 后端API返回MinIO监控指标
2. ✅ 前端API接口文件
3. ✅ MinIO监控页面组件
4. ✅ 路由配置

**文件清单**:
- `internal/plugin/aianalysis/service.go` - 后端API更新
- `web-src/src/api/aiAnalysis.js` - 前端API接口
- `web-src/src/views/ai-analysis/minio-monitor.vue` - 监控页面
- `web-src/src/router/rootRoute.js` - 路由配置
- `web-src/src/api/index.js` - API导出更新

---

**实现时间**：2025年11月19日  
**实现状态**：✅ 已完成

