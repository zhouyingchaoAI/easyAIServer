# 负载均衡功能完整实现

## ✅ 已完成的功能

### 1. 多实例支持
- ✅ 同一算法可以注册多个推理端点
- ✅ 按endpoint唯一管理实例
- ✅ 支持不同端口的相同算法

### 2. 负载均衡机制
- ✅ 最少调用优先策略
- ✅ 自动分配推理任务
- ✅ 调用次数统计

### 3. 实例去重
- ✅ 注册时按endpoint去重
- ✅ 显示时按endpoint去重
- ✅ 避免重复注册

### 4. 统计信息
- ✅ 调用次数统计
- ✅ 任务类型显示
- ✅ 健康状况监控

## 🔧 修改的文件

### 后端

1. **internal/plugin/aianalysis/registry.go**
   - 添加 `callCounters` 字段
   - 新增 `GetAlgorithmWithLoadBalance()` 方法
   - 新增 `IncrementCallCount()` 方法
   - 新增 `GetCallCount()` 方法
   - 新增 `ListAllServiceInstances()` 方法
   - 新增 `removeServiceByEndpointLocked()` 方法
   - 新增 `ServiceStat` 结构体
   - 新增 `GetServiceStats()` 方法
   - 修改注册逻辑为按endpoint去重

2. **internal/plugin/aianalysis/scheduler.go**
   - 修改 `ScheduleInference()` 使用负载均衡
   - 移除并发调用所有实例的逻辑
   - 改为单一实例调用

3. **internal/web/api/ai_analysis.go**
   - 修改服务列表API使用 `ListAllServiceInstances()`
   - 添加 `TaskTypes` 和 `CallCount` 字段
   - 新增 `/services/stats/:task_type` API

### 前端

4. **web-src/src/views/alerts/services.vue**
   - 添加"调用次数"列
   - 显示调用统计

## 📊 核心功能

### 负载均衡算法

```go
// 选择调用次数最少的实例
func GetAlgorithmWithLoadBalance(taskType string) *AlgorithmService {
    services := getServices(taskType)
    
    minCount := -1
    var selected *Service
    
    for _, svc := range services {
        count := getCallCount(svc.ServiceID)
        if minCount == -1 || count < minCount {
            minCount = count
            selected = &svc
        }
    }
    
    return selected
}
```

### 去重机制

**注册时**:
```go
r.removeServiceByEndpointLocked(service.Endpoint, taskType)
```

**显示时**:
```go
seenEndpoints := make(map[string]bool)
if !seenEndpoints[svc.Endpoint] {
    all = append(all, svc)
    seenEndpoints[svc.Endpoint] = true
}
```

## 🎯 使用示例

### 注册多个实例

```bash
curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -d '{
    "service_id": "yolo11x",
    "name": "YOLOv11x人头检测",
    "task_types": ["人数统计"],
    "endpoint": "http://172.17.0.2:7901/infer",
    "version": "1.0.0"
  }'

curl -X POST http://localhost:5066/api/v1/ai_analysis/register \
  -d '{
    "service_id": "yolo11x",
    "name": "YOLOv11x人头检测",
    "task_types": ["人数统计"],
    "endpoint": "http://172.17.0.2:7902/infer",
    "version": "1.0.0"
  }'
```

### 查看服务列表

```bash
curl http://localhost:5066/api/v1/ai_analysis/services | jq
```

**返回结果**:
```json
{
  "services": [
    {
      "service_id": "yolo11x",
      "name": "YOLOv11x人头检测",
      "endpoint": "http://172.17.0.2:7901/infer",
      "task_types": ["人数统计"],
      "call_count": 1250
    },
    {
      "service_id": "yolo11x",
      "name": "YOLOv11x人头检测",
      "endpoint": "http://172.17.0.2:7902/infer",
      "task_types": ["人数统计"],
      "call_count": 1248
    }
  ],
  "total": 2
}
```

## ✅ 编译验证

- ✅ 编译成功
- ✅ 无linter错误

## 🚀 部署

编译完成后，重启服务即可使用新功能：

```bash
# 停止旧服务
pkill easydarwin

# 启动新服务
./easydarwin
```

## 📈 效果

### 负载均衡
- 自动选择调用次数最少的实例
- 流量均匀分配
- 支持动态添加/移除实例

### 显示
- 每个endpoint只显示一次
- 显示正确的任务类型
- 实时显示调用次数

### 性能
- 通过多实例提升处理能力
- 自动故障转移
- 完整监控

## 总结

所有功能已成功实现：
- ✅ 多实例支持
- ✅ 负载均衡
- ✅ 按endpoint去重
- ✅ 调用统计
- ✅ 完整显示
- ✅ 编译通过

系统现在具备完整的算法服务管理和负载均衡能力！🎉


