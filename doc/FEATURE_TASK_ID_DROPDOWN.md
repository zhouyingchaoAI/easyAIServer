# 功能更新：任务ID自动下拉选择

## 📝 更新概览

告警列表的任务ID筛选器已从手动输入改为自动下拉选择，提升用户体验。

## ✨ 新增功能

### 1. 任务ID自动获取

- **自动加载**：页面加载时自动从数据库获取所有不重复的任务ID
- **下拉选择**：将输入框改为下拉选择框，避免输入错误
- **搜索过滤**：支持在下拉列表中输入关键词搜索任务ID
- **自动排序**：任务ID按字母顺序排序，方便查找

### 2. 用户体验提升

- **无需记忆**：不需要记住具体的任务ID
- **快速筛选**：直接从列表中选择，无需手动输入
- **防止错误**：避免因拼写错误导致查询不到数据
- **清空选项**：可以一键清空选择，查看所有任务

## 🎯 功能特性

### 下拉选择框

```vue
<a-select 
  v-model:value="filter.task_id" 
  placeholder="任务ID" 
  allow-clear          <!-- 支持清空 -->
  show-search          <!-- 支持搜索 -->
  size="large"
  :filter-option="filterOption"
  @change="fetchData"
>
  <a-select-option value="">全部任务</a-select-option>
  <a-select-option v-for="taskId in taskIds" :key="taskId" :value="taskId">
    {{ taskId }}
  </a-select-option>
</a-select>
```

### 搜索功能

支持模糊搜索，不区分大小写：
- 输入 `task` → 匹配所有包含 "task" 的任务ID
- 输入 `123` → 匹配所有包含 "123" 的任务ID
- 输入 `人数` → 匹配所有包含 "人数" 的任务ID

## 🔧 技术实现

### 后端 API

#### 新增接口

**请求：**
```bash
GET /api/v1/alerts/task_ids
```

**响应：**
```json
{
  "task_ids": [
    "11111",
    "task_1",
    "task_2",
    "人数统计1",
    "工地A区监控"
  ]
}
```

#### 数据库查询

```go
// GetDistinctTaskIDs 获取所有不重复的任务ID列表
func GetDistinctTaskIDs() ([]string, error) {
    var taskIDs []string
    err := GetDatabase().Model(&model.Alert{}).
        Distinct("task_id").          // 去重
        Where("task_id != ''").       // 过滤空值
        Order("task_id ASC").         // 按字母排序
        Pluck("task_id", &taskIDs).Error
    
    return taskIDs, nil
}
```

### 前端实现

#### API调用

```javascript
// api/alert.js
export default {
  // 获取所有任务ID列表
  getTaskIds(){
    return request({
      url: '/alerts/task_ids',
      method: 'get'
    });
  }
}
```

#### 页面逻辑

```javascript
// 获取任务ID列表
const fetchTaskIds = async () => {
  try {
    const { data } = await alertApi.getTaskIds()
    taskIds.value = data?.task_ids || []
  } catch (e) {
    console.error('fetch task ids failed', e)
  }
}

// 搜索过滤函数
const filterOption = (input, option) => {
  return option.value.toLowerCase().includes(input.toLowerCase())
}

// 页面加载时获取
onMounted(() => {
  fetchData()
  fetchTaskTypes()
  fetchTaskIds()  // 新增
})
```

## 📊 使用示例

### 场景 1：选择特定任务

1. 打开告警列表页面
2. 点击"任务ID"下拉框
3. 从列表中选择目标任务ID（如 `task_1`）
4. 自动触发查询，显示该任务的所有告警

### 场景 2：搜索任务

1. 点击"任务ID"下拉框
2. 输入搜索关键词（如 `人数`）
3. 下拉列表自动过滤，只显示包含"人数"的任务ID
4. 选择匹配的任务ID

### 场景 3：清空筛选

1. 点击"任务ID"输入框右侧的清空图标（×）
2. 任务ID筛选被清除
3. 自动查询所有任务的告警

## 🎨 界面效果

### 下拉列表示例

```
┌──────────────────────────┐
│ 任务ID ▼                 │
├──────────────────────────┤
│ 全部任务                 │
│ 11111                    │
│ task_1                   │
│ task_2                   │
│ 人数统计1                │
│ 工地A区监控              │
└──────────────────────────┘
```

### 搜索示例

输入 "task" 后：
```
┌──────────────────────────┐
│ task ▼                   │
├──────────────────────────┤
│ task_1                   │
│ task_2                   │
└──────────────────────────┘
```

## 🚀 性能优化

### 1. 缓存策略

任务ID列表在页面加载时获取一次，后续不再重复请求：
- 减少服务器压力
- 提升用户体验
- 降低网络延迟

### 2. 数据库优化

使用 `DISTINCT` 和 `Pluck` 提高查询效率：
```sql
SELECT DISTINCT task_id 
FROM alerts 
WHERE task_id != '' 
ORDER BY task_id ASC
```

### 3. 前端优化

- 使用 `show-search` 支持本地搜索，无需请求服务器
- 自动过滤，响应速度快
- 列表项懒加载（Ant Design Vue 自动处理）

## 📦 修改的文件

**后端（3个文件）：**
1. `internal/web/api/ai_analysis.go` - 添加获取任务ID列表的API
2. `internal/data/alert.go` - 添加查询不重复任务ID的函数
3. (无需修改数据库结构)

**前端（2个文件）：**
1. `web-src/src/api/alert.js` - 添加获取任务ID的API调用
2. `web-src/src/views/alerts/index.vue` - 将输入框改为下拉选择框

## 🔄 兼容性

- ✅ 向下兼容：不影响现有功能
- ✅ 数据兼容：无需修改数据库结构
- ✅ API兼容：新增API，不影响现有API

## 💡 最佳实践

### 1. 任务命名建议

为了更好的用户体验，建议使用有意义的任务ID：

**推荐：**
- `商场1F入口`
- `工地A区`
- `人数统计_001`

**不推荐：**
- `task_1`
- `11111`
- `abc`

### 2. 任务ID管理

定期清理无用的任务：
```sql
-- 查看30天前的任务
SELECT DISTINCT task_id, COUNT(*) as alert_count, MAX(created_at) as last_alert
FROM alerts
GROUP BY task_id
HAVING MAX(created_at) < datetime('now', '-30 days')
ORDER BY last_alert DESC;

-- 删除指定任务的所有告警
DELETE FROM alerts WHERE task_id = 'old_task_id';
```

## 🐛 已知问题

无

## 📝 待优化

1. **任务ID分组**：按任务类型对任务ID进行分组显示
2. **最近使用**：优先显示最近查询过的任务ID
3. **统计信息**：显示每个任务ID的告警数量
4. **批量选择**：支持同时选择多个任务ID

示例效果：
```
┌──────────────────────────┐
│ 任务ID ▼                 │
├──────────────────────────┤
│ 📊 人数统计              │
│   ├─ task_1 (125条)     │
│   └─ task_2 (89条)      │
│ ⚠️  安全监控             │
│   ├─ 工地A区 (45条)     │
│   └─ 工地B区 (32条)     │
└──────────────────────────┘
```

## 📞 技术支持

如有问题，请查阅：
- [完整文档](README_CN.md)
- [API文档](EasyDarwin.api.html)
- [检测个数功能](FEATURE_UPDATE_DETECTION_COUNT.md)

---

**版本**：v1.1.1  
**更新日期**：2024-10-17  
**作者**：yanying team

