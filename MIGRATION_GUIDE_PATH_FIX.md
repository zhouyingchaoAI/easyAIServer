# 迁移指南：修复路径后的数据清理

## 问题说明

修复路径问题后，旧的图片和配置文件仍然在使用 `OutputPath` 的旧路径中，会导致：
1. **告警混淆**：旧图片被扫描时，TaskID会被错误解析
2. **配置错乱**：旧配置文件在错误的路径，新任务找不到配置
3. **图片不匹配**：告警显示的图片可能来自错误的任务

## 解决方案

### 方案1：停止所有任务并清理（推荐）

这是最彻底的解决方案，适用于测试环境或可以接受短暂中断的场景。

#### 步骤：

1. **停止所有抽帧任务**
```bash
# 停止EasyDarwin服务
systemctl stop easydarwin
# 或
./stop.sh
```

2. **清理MinIO中的旧图片**

**方式A：通过MinIO Console清理**
```
1. 访问 MinIO Console: http://IP:9000
2. 进入 images bucket
3. 删除所有旧的抽帧图片目录
```

**方式B：使用mc命令行工具**
```bash
# 安装mc（如果未安装）
wget https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x mc
./mc alias set myminio http://172.16.5.207:9000 admin admin123

# 删除所有抽帧图片（保留alerts目录）
./mc rm --recursive --force myminio/images/人数统计/
./mc rm --recursive --force myminio/images/绊线人数统计/
./mc rm --recursive --force myminio/images/人员跌倒/
# ... 对所有任务类型重复
```

**方式C：使用Python脚本批量清理**
```python
#!/usr/bin/env python3
# cleanup_old_frames.py
from minio import Minio

# MinIO配置
client = Minio(
    "172.16.5.207:9000",
    access_key="admin",
    secret_key="admin123",
    secure=False
)

bucket = "images"
alert_prefix = "alerts/"  # 保留告警图片

# 列出所有对象
objects = client.list_objects(bucket, recursive=True)

deleted_count = 0
for obj in objects:
    # 跳过告警图片
    if obj.object_name.startswith(alert_prefix):
        print(f"保留告警图片: {obj.object_name}")
        continue
    
    # 删除抽帧图片
    try:
        client.remove_object(bucket, obj.object_name)
        deleted_count += 1
        if deleted_count % 100 == 0:
            print(f"已删除 {deleted_count} 个图片...")
    except Exception as e:
        print(f"删除失败 {obj.object_name}: {e}")

print(f"\n总共删除 {deleted_count} 个旧图片")
```

3. **清理数据库中的旧告警记录**（可选）
```sql
-- 如果需要，可以清空旧的告警记录
-- 注意：这会删除所有历史告警！
sqlite3 ./configs/data.db
DELETE FROM alerts;
.quit
```

4. **重启服务**
```bash
# 启动EasyDarwin服务
systemctl start easydarwin
# 或
./start.sh
```

5. **验证新路径**
```bash
# 等待几分钟让任务运行
# 检查MinIO中的新路径
./mc ls myminio/images/人数统计/

# 应该看到格式为：任务ID/图片文件
# 例如：测试1/20251105-120000.jpg
```

---

### 方案2：仅清理指定任务（适用于生产环境）

如果无法停止所有任务，可以逐个清理有问题的任务。

#### 步骤：

1. **识别混淆的任务**
```bash
# 查看告警记录，找出错误的task_id
curl http://localhost:5066/api/v1/alerts | jq '.items[] | {task_id, task_type}'
```

2. **停止单个任务**
```bash
# 通过API停止任务
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks/{TASK_ID}/stop
```

3. **清理该任务的旧图片**
```bash
# 使用mc删除特定任务的旧路径
./mc rm --recursive --force myminio/images/人数统计/{旧的OutputPath}/
```

4. **删除该任务的错误告警**
```sql
sqlite3 ./configs/data.db
DELETE FROM alerts WHERE task_id = '{错误的task_id}';
.quit
```

5. **删除并重建任务**
```bash
# 删除任务
curl -X DELETE http://localhost:5066/api/v1/frame_extractor/tasks/{TASK_ID}

# 重新创建任务（确保不指定output_path，或output_path等于id）
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "camera001",
    "task_type": "人数统计",
    "rtsp_url": "rtsp://..."
  }'
```

6. **重新配置算法参数**
```
通过前端界面为任务重新配置区域参数
```

7. **启动任务**
```bash
curl -X POST http://localhost:5066/api/v1/frame_extractor/tasks/{TASK_ID}/start
```

---

### 方案3：等待自动清理（最简单，但需要时间）

如果启用了抽帧清理机制（`max_frame_count`），旧图片会逐渐被清理。

#### 特点：
- **优点**：无需手动干预
- **缺点**：
  - 需要等待一段时间（取决于抽帧速度和max_frame_count设置）
  - 期间仍可能出现告警混淆
  - 配置文件不会自动清理

#### 配置：
```toml
[frame_extractor]
max_frame_count = 100  # 设置较小的值加快清理
```

---

## 验证修复是否成功

### 1. 检查路径结构

**正确的路径格式：**
```
MinIO路径：{basePath}/{任务类型}/{任务ID}/{文件名}
示例：     人数统计/camera001/20251105-120000.jpg
```

**检查方法：**
```bash
# 使用mc工具查看
./mc ls --recursive myminio/images/ | head -20

# 或通过API查看抽帧图片列表
curl http://localhost:5066/api/v1/frame_extractor/snapshots/{TASK_ID}
```

### 2. 检查告警关联

```bash
# 查看最新的告警记录
curl http://localhost:5066/api/v1/alerts?page=1&page_size=10 | jq '.items[] | {task_id, image_path}'

# task_id应该与image_path中的第二层目录一致
# 例如：
# task_id: "camera001"
# image_path: "人数统计/camera001/20251105-120000.jpg"
```

### 3. 检查配置文件

```bash
# 检查配置文件路径
curl http://localhost:5066/api/v1/frame_extractor/tasks/{TASK_ID}/algo_config

# 配置文件应该在：人数统计/{TASK_ID}/algo_config.json
```

### 4. 前端验证

1. 打开告警列表页面
2. 查看告警详情
3. 检查：
   - 告警任务ID是否正确
   - 图片是否显示正确
   - 配置区域是否绘制在正确位置
   - 检测结果框是否匹配图片内容

---

## 预防未来问题

### 1. API创建任务时的最佳实践

```javascript
// ✅ 推荐：不指定output_path
{
  "id": "camera001",
  "task_type": "人数统计",
  "rtsp_url": "rtsp://..."
}

// ✅ 可以：output_path等于id
{
  "id": "camera001",
  "output_path": "camera001",  // 与id相同
  "task_type": "人数统计",
  "rtsp_url": "rtsp://..."
}

// ❌ 避免：output_path不等于id
{
  "id": "camera001",
  "output_path": "entrance_camera",  // 不同于id
  "task_type": "人数统计",
  "rtsp_url": "rtsp://..."
}
```

### 2. 配置文件中的任务定义

```toml
[[frame_extractor.tasks]]
id = 'camera001'
task_type = '人数统计'
rtsp_url = 'rtsp://...'
# output_path = 'camera001'  # 可以省略，系统会自动使用id
enabled = true
```

### 3. 监控告警质量

定期检查告警是否正确关联：
```bash
# 监控脚本
#!/bin/bash
# check_alert_quality.sh

echo "检查告警关联质量..."
curl -s http://localhost:5066/api/v1/alerts?page=1&page_size=100 | \
  jq -r '.items[] | "\(.task_id)\t\(.image_path)"' | \
  while read task_id image_path; do
    # 从image_path提取任务ID（第二层目录）
    path_task_id=$(echo "$image_path" | cut -d'/' -f2)
    
    if [ "$task_id" != "$path_task_id" ]; then
      echo "❌ 告警混淆: task_id=$task_id, path中的id=$path_task_id"
    fi
  done

echo "检查完成"
```

---

## 常见问题

### Q1: 为什么修复后还是有错乱？
**A:** 旧的图片还在旧路径中，需要手动清理或等待自动清理。

### Q2: 清理后会丢失历史数据吗？
**A:** 
- 抽帧图片会被删除（这些图片用于推理，推理后可以删除）
- 告警图片（在 `alerts/` 目录）会保留
- 告警记录（数据库）会保留
- 建议在清理前备份重要数据

### Q3: 如何确认我的任务使用了正确的路径？
**A:** 检查最新抽取的图片路径，格式应该是：`任务类型/任务ID/文件名`

### Q4: 配置文件也需要迁移吗？
**A:** 是的。如果之前保存过算法配置，清理后需要重新配置。

### Q5: 生产环境可以使用自动清理吗？
**A:** 建议手动清理，因为自动清理：
- 需要时间
- 期间可能仍有错误告警
- 配置文件不会自动清理

---

## 技术细节

### 路径解析逻辑

```go
// internal/plugin/aianalysis/scanner.go
func parseImagePath(objectKey, basePath string) (taskType, taskID, filename string) {
    path := strings.TrimPrefix(objectKey, basePath)
    path = strings.TrimPrefix(path, "/")
    
    parts := strings.Split(path, "/")
    if len(parts) < 3 {
        return "", "", ""
    }
    
    taskType = parts[0]  // 第一层：任务类型
    taskID = parts[1]    // 第二层：任务ID（必须与真实ID一致！）
    filename = parts[len(parts)-1]  // 最后一层：文件名
    return
}
```

### 配置读取逻辑

```go
// internal/plugin/frameextractor/service.go
func (s *Service) GetAlgorithmConfig(taskID string) ([]byte, error) {
    // 查找任务
    var task *conf.FrameExtractTask
    for i := range s.cfg.Tasks {
        if s.cfg.Tasks[i].ID == taskID {  // 使用taskID查找
            task = &s.cfg.Tasks[i]
            break
        }
    }
    
    // 构建配置文件路径：任务类型/任务ID/algo_config.json
    configKey := filepath.ToSlash(filepath.Join(
        s.minio.base, 
        taskType, 
        task.ID,  // 使用task.ID而不是task.OutputPath
        "algo_config.json"
    ))
    
    // 从MinIO读取
    return readFromMinIO(configKey)
}
```

---

## 总结

**核心原则**：路径的第二层目录必须是任务的真实ID，不能是OutputPath。

**迁移步骤**：
1. 升级到修复版本（已完成）
2. 清理旧数据（手动或自动）
3. 验证新路径（检查告警质量）
4. 遵循最佳实践（避免未来问题）

如有问题，请查看日志：
```bash
tail -f logs/sugar.log | grep -E "(任务ID|task_id|image_path)"
```

