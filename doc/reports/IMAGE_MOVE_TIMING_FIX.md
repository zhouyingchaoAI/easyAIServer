# 图片移动时序问题修复方案

**问题**: 批量移动图片时，虽然路径正确，但在高并发情况下可能出现图片内容错位

## 诊断结果

从日志分析发现：

```
⚠️ 同一秒内有相同task_id的多次移动
例如: 
- 时间: 2025-11-06 09:17:17 | 并发数: 2
  - TaskID=测试10, File=20251106-091714.167.jpg
  - TaskID=测试10, File=20251106-091713.161.jpg  ← 同一task_id
```

## 问题分析

当前的异步移动实现：

```go
// 当前代码（scheduler.go 第310行）
go func(srcPath, dstPath, taskID, taskType, filename string) {
    if err := s.moveImageToAlertPathAsync(srcPath, dstPath); err != nil {
        // 错误处理...
    }
}(image.Path, targetAlertPath, image.TaskID, image.TaskType, image.Filename)
```

**潜在问题**:
1. 异步移动是并发的，没有顺序保证
2. MinIO的CopyObject + RemoveObject不是原子操作
3. 在高并发时，可能出现：
   - 图片A正在复制时，图片B已经开始复制
   - 删除操作可能删除了错误的文件
   - 路径虽然正确，但内容可能错位

## 解决方案

### 方案1: 添加移动锁（推荐）

为每个task_id添加移动锁，确保同一任务的图片按顺序移动：

```go
// 在Scheduler结构体中添加
type Scheduler struct {
    // ... 现有字段 ...
    
    // 移动锁：确保同一task_id的图片顺序移动
    moveLocks map[string]*sync.Mutex
    moveLockMu sync.Mutex
}

// 获取或创建移动锁
func (s *Scheduler) getMoveLock(taskID string) *sync.Mutex {
    s.moveLockMu.Lock()
    defer s.moveLockMu.Unlock()
    
    if s.moveLocks == nil {
        s.moveLocks = make(map[string]*sync.Mutex)
    }
    
    if _, ok := s.moveLocks[taskID]; !ok {
        s.moveLocks[taskID] = &sync.Mutex{}
    }
    
    return s.moveLocks[taskID]
}

// 修改移动逻辑
go func(srcPath, dstPath, taskID, taskType, filename string) {
    // 获取该task_id的锁
    lock := s.getMoveLock(taskID)
    lock.Lock()
    defer lock.Unlock()
    
    if err := s.moveImageToAlertPathAsync(srcPath, dstPath); err != nil {
        // 错误处理...
    }
}(image.Path, targetAlertPath, image.TaskID, image.TaskType, image.Filename)
```

### 方案2: 同步移动（更简单，但可能影响性能）

将异步移动改为同步：

```go
// 直接在主流程中移动
if s.alertBasePath != "" && detectionCount > 0 {
    targetAlertPath := fmt.Sprintf("%s%s/%s/%s", s.alertBasePath, image.TaskType, image.TaskID, image.Filename)
    
    // 同步移动（阻塞，但保证顺序）
    if err := s.moveImageToAlertPathInternal(image.Path, targetAlertPath); err != nil {
        s.log.Error("failed to move image",
            slog.String("src", image.Path),
            slog.String("dst", targetAlertPath),
            slog.String("err", err.Error()))
        // 移动失败，使用原路径
        alertImagePath = image.Path
    } else {
        alertImagePath = targetAlertPath
    }
} else {
    alertImagePath = image.Path
}
```

### 方案3: 使用移动队列

为每个task_id创建一个移动队列，保证顺序：

```go
type MoveTask struct {
    srcPath  string
    dstPath  string
    taskID   string
    taskType string
    filename string
}

// 在Scheduler中添加
moveQueues map[string]chan MoveTask
queueMu    sync.Mutex

// 启动队列处理器
func (s *Scheduler) startMoveWorker(taskID string) {
    queue := make(chan MoveTask, 100)
    s.moveQueues[taskID] = queue
    
    go func() {
        for task := range queue {
            if err := s.moveImageToAlertPathInternal(task.srcPath, task.dstPath); err != nil {
                s.log.Error("move failed", ...)
            } else {
                s.log.Info("move succeeded", ...)
            }
        }
    }()
}

// 提交移动任务
func (s *Scheduler) submitMoveTask(task MoveTask) {
    s.queueMu.Lock()
    if _, ok := s.moveQueues[task.taskID]; !ok {
        s.startMoveWorker(task.taskID)
    }
    queue := s.moveQueues[task.taskID]
    s.queueMu.Unlock()
    
    queue <- task
}
```

## 推荐实施方案

**使用方案1（添加移动锁）**，原因：
1. 最小改动，对现有逻辑影响小
2. 保持异步移动的性能优势
3. 确保同一任务的图片按顺序移动
4. 不同任务之间仍然可以并发移动

## 实施步骤

1. 修改 `Scheduler` 结构体，添加移动锁map
2. 实现 `getMoveLock` 方法
3. 修改异步移动逻辑，在闭包开始时获取锁
4. 重新编译和部署
5. 验证并发场景下的正确性

## 验证方法

部署后运行验证脚本：

```bash
# 持续监控移动日志
tail -f logs/20251106_*.log | grep "async image move" | \
while read line; do
    echo "$line" | python3 -c "
import sys, json
for line in sys.stdin:
    try:
        data = json.loads(line)
        print(f\"{data.get('ts', '')} | {data.get('task_id', ''):12s} | {data.get('filename', '')}\")
    except: pass
"
done
```

观察是否还有同一task_id的移动操作交错。

## 补充说明

如果使用方案1后仍有问题，可能需要：
1. 检查MinIO的一致性设置
2. 增加移动操作的重试机制
3. 添加文件完整性校验（比如MD5）
4. 考虑使用MinIO的原子操作API

