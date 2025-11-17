package frameextractor

import (
    "bytes"
    "context"
    "errors"
    "easydarwin/internal/conf"
    "easydarwin/utils/pkg/system"
    "fmt"
    "io"
    "log/slog"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "sync"
    "time"
    
    "github.com/minio/minio-go/v7"
)

type Service struct {
    cfg        *conf.FrameExtractorConfig
    log        *slog.Logger
    wg         sync.WaitGroup
    stop       chan struct{}
    mu         sync.Mutex
    configPath string // path to config.toml for persistence
    minio      *minioClient // minio client if store=minio
    // per-task stop channel
    taskStops map[string]chan struct{}
    // monitoring stats
    stats      TaskStats
    statsMu    sync.RWMutex
    // frame cleanup counters (per task)
    cleanupCounters map[string]*cleanupCounter
    cleanupMu       sync.Mutex
    // 队列检查回调函数（用于检查图片是否在推理队列中，避免清理队列中的图片）
    queueChecker func(string) bool
    queueCheckerMu sync.RWMutex
    // 抽帧速率统计（每秒抽帧数量）
    frameRateStats frameRateMonitor
    frameRateMu    sync.RWMutex
}

// frameRateMonitor 抽帧速率监控器
type frameRateMonitor struct {
    countInWindow    int64     // 窗口内的抽帧数量
    windowStartTime  time.Time // 窗口开始时间
    framesPerSec     float64   // 每秒抽帧数量
    totalFrames      int64     // 总抽帧数量
    lastFrameTime    time.Time // 上次抽帧时间
}

// cleanupCounter 清理计数器，用于限流
type cleanupCounter struct {
    uploadCount  int       // 上传计数
    lastCleanup  time.Time // 上次清理时间
}

// SetQueueChecker 设置队列检查回调函数（用于检查图片是否在推理队列中）
// 这个函数由AI分析服务在启动时注册，用于避免清理队列中的图片
func (s *Service) SetQueueChecker(checker func(imagePath string) bool) {
	s.queueCheckerMu.Lock()
	defer s.queueCheckerMu.Unlock()
	s.queueChecker = checker
	s.log.Info("queue checker registered", slog.String("note", "images in inference queue will be protected from cleanup"))
}

// recordFrameExtracted 记录一次抽帧成功（用于计算每秒抽帧数量）
func (s *Service) recordFrameExtracted() {
	s.frameRateMu.Lock()
	defer s.frameRateMu.Unlock()
	
	now := time.Now()
	s.frameRateStats.countInWindow++
	s.frameRateStats.totalFrames++
	s.frameRateStats.lastFrameTime = now
	
	// 计算窗口内的每秒抽帧数（使用最近1秒的数据）
	windowDuration := now.Sub(s.frameRateStats.windowStartTime).Seconds()
	if windowDuration >= 1.0 {
		// 窗口已满1秒，计算每秒抽帧数
		s.frameRateStats.framesPerSec = float64(s.frameRateStats.countInWindow) / windowDuration
		// 重置窗口
		s.frameRateStats.countInWindow = 0
		s.frameRateStats.windowStartTime = now
	} else if windowDuration > 0 {
		// 窗口未满1秒，使用当前数据估算
		s.frameRateStats.framesPerSec = float64(s.frameRateStats.countInWindow) / windowDuration
	}
}

// ResetFrameStats 重置抽帧统计数据
func (s *Service) ResetFrameStats() {
	s.frameRateMu.Lock()
	defer s.frameRateMu.Unlock()
	
	now := time.Now()
	s.frameRateStats.countInWindow = 0
	s.frameRateStats.totalFrames = 0
	s.frameRateStats.framesPerSec = 0
	s.frameRateStats.windowStartTime = now
	s.frameRateStats.lastFrameTime = time.Time{}
	
	s.log.Info("frame extraction statistics reset")
}

// TaskStats 任务统计信息
type TaskStats struct {
    TotalTasks      int                `json:"total_tasks"`       // 总任务数
    RunningTasks    int                `json:"running_tasks"`     // 运行中的任务数
    StoppedTasks    int                `json:"stopped_tasks"`     // 已停止的任务数
    ConfiguredTasks int                `json:"configured_tasks"`  // 已配置的任务数
    PendingTasks    int                `json:"pending_tasks"`     // 待配置的任务数
    TaskDetails     []TaskMonitorInfo  `json:"task_details"`      // 各任务详情
    SystemInfo      SystemMonitorInfo  `json:"system_info"`       // 系统信息
    FramesPerSec    float64            `json:"frames_per_sec"`   // 每秒抽帧数量（张/秒）
    TotalFrames     int64              `json:"total_frames"`     // 总抽帧数量
    UpdatedAt       time.Time          `json:"updated_at"`        // 更新时间
}

// TaskMonitorInfo 单个任务监控信息
type TaskMonitorInfo struct {
    ID              string    `json:"id"`                // 任务ID
    TaskType        string    `json:"task_type"`         // 任务类型
    Status          string    `json:"status"`            // 状态: running/stopped
    ConfigStatus    string    `json:"config_status"`     // 配置状态
    IntervalMs      int       `json:"interval_ms"`       // 抽帧间隔
    OutputPath      string    `json:"output_path"`       // 输出路径
    LastFrameTime   time.Time `json:"last_frame_time"`   // 最后抽帧时间
    FrameCount      int64     `json:"frame_count"`       // 已抽取的帧数
    ErrorCount      int64     `json:"error_count"`       // 错误计数
    Uptime          int64     `json:"uptime"`            // 运行时长(秒)
    StartTime       time.Time `json:"start_time"`        // 启动时间
}

// SystemMonitorInfo 系统监控信息
type SystemMonitorInfo struct {
    Goroutines      int       `json:"goroutines"`        // Goroutine数量
    MemoryUsageMB   float64   `json:"memory_usage_mb"`   // 内存使用(MB)
    CPUCores        int       `json:"cpu_cores"`         // CPU核心数
}

func New(cfg *conf.FrameExtractorConfig) *Service {
    return &Service{
        cfg:             cfg,
        log:             slog.Default(),
        stop:            make(chan struct{}),
        taskStops:       make(map[string]chan struct{}),
        cleanupCounters: make(map[string]*cleanupCounter),
        frameRateStats: frameRateMonitor{
            windowStartTime: time.Now(),
        },
    }
}

// SetConfigPath sets the path to config.toml for persistence
func (s *Service) SetConfigPath(path string) {
    s.configPath = path
}

func (s *Service) Start() error {
    if s.cfg == nil {
        return errors.New("frameextractor: nil config")
    }
    if !s.cfg.Enable {
        return nil
    }

    // 🔧 自动迁移旧配置，补全缺失字段（向后兼容）
    MigrateConfig(s.cfg, s.log)
    
    // 验证配置
    if warnings := ValidateConfig(s.cfg); len(warnings) > 0 {
        for _, w := range warnings {
            s.log.Warn("config validation warning", slog.String("warning", w))
        }
    }

    // prepare output dir when using local store
    if s.cfg.Store == "local" {
        out := s.cfg.OutputDir
        if out == "" {
            out = filepath.Join(system.GetCWD(), "snapshots")
        }
        if err := os.MkdirAll(out, 0o755); err != nil {
            return err
        }
    } else if s.cfg.Store == "minio" {
        if err := s.initMinio(); err != nil {
            return err
        }
    }

    s.log.Info("frameextractor started", slog.Int("default_interval_ms", s.cfg.IntervalMs), slog.String("store", s.cfg.Store))

    // boot predefined tasks (no decoding yet; placeholder goroutine)
    for _, t := range s.cfg.Tasks {
        if strings.TrimSpace(t.RtspURL) == "" {
            continue
        }
        _ = s.startTask(t)
    }
    return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
    select {
    case <-s.stop:
    default:
        close(s.stop)
    }
    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-done:
        return nil
    }
}

func getIntervalMs(t conf.FrameExtractTask, c *conf.FrameExtractorConfig) int {
    if t.IntervalMs > 0 {
        return t.IntervalMs
    }
    if c.IntervalMs > 0 {
        return c.IntervalMs
    }
    return 1000
}

// startTask starts a single task and tracks its stop channel
func (s *Service) startTask(t conf.FrameExtractTask) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // skip if already running
    if _, ok := s.taskStops[t.ID]; ok {
        return nil
    }
    
    // skip if not enabled
    if !t.Enabled {
        s.log.Info("task disabled, skipping", slog.String("task", t.ID))
        return nil
    }
    
    done := make(chan struct{})
    s.taskStops[t.ID] = done
    s.wg.Add(1)
    
    s.log.Info("starting task", slog.String("task", t.ID), slog.String("rtsp", t.RtspURL), slog.Int("interval_ms", getIntervalMs(t, s.cfg)))
    
    if s.cfg.Store == "local" {
        go s.runLocalSinkLoopCtx(t, done)
    } else if s.cfg.Store == "minio" {
        go s.runMinioSinkLoopCtx(t, done)
    } else {
        go s.runLocalSinkLoopCtx(t, done)
    }
    return nil
}

// AddTask adds or replaces a task at runtime
func (s *Service) AddTask(t conf.FrameExtractTask) error {
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.RtspURL) == "" {
		return errors.New("invalid task")
	}
	// stop existing
	s.RemoveTask(t.ID)
	
	// 初始状态设置为unconfigured，不启动
	t.Enabled = false
	t.ConfigStatus = "unconfigured"
	t.PreviewImage = ""
	
	// ensure output_path defaults to task ID if empty
	if strings.TrimSpace(t.OutputPath) == "" {
		t.OutputPath = t.ID
	}
	
	// ensure task_type has a value (default to first type or "未分类")
	if strings.TrimSpace(t.TaskType) == "" {
		if len(s.cfg.TaskTypes) > 0 {
			t.TaskType = s.cfg.TaskTypes[0]
		} else {
			t.TaskType = "未分类"
		}
	}
	
	// record into cfg
	s.cfg.Tasks = append(s.cfg.Tasks, t)
	
	// create minio path if store is minio
	if s.cfg.Store == "minio" && s.minio != nil {
		if err := s.createMinioPath(t); err != nil {
			s.log.Warn("failed to create minio path", slog.String("task", t.ID), slog.String("err", err.Error()))
		}
	}
	
	// persist to config file
	if err := s.saveConfigToFile(s.configPath); err != nil {
		s.log.Warn("failed to persist config", slog.String("err", err.Error()))
	}
	
	// 抽取单张预览图（异步）
	go s.extractSinglePreviewFrame(t)
	
	return nil
}

// RemoveTask stops and removes a task by id
func (s *Service) RemoveTask(id string) bool {
    s.mu.Lock()
    ch, running := s.taskStops[id]
    var removedTask *conf.FrameExtractTask
    
    // stop task if running
    if running {
        close(ch)
        delete(s.taskStops, id)
    }
    
    // remove cleanup counter
    s.cleanupMu.Lock()
    delete(s.cleanupCounters, id)
    s.cleanupMu.Unlock()
    
    // remove from cfg slice regardless of running state
    tasks := s.cfg.Tasks[:0]
    found := false
    for _, it := range s.cfg.Tasks {
        if it.ID != id {
            tasks = append(tasks, it)
        } else {
            removedTask = &it
            found = true
        }
    }
    s.cfg.Tasks = tasks
    s.mu.Unlock()
    
    if !found {
        s.log.Warn("task not found for removal", slog.String("task", id))
        return false
    }
    
    // delete minio path if store is minio
    if removedTask != nil && s.cfg.Store == "minio" && s.minio != nil {
        if err := s.deleteMinioPath(*removedTask); err != nil {
            s.log.Warn("failed to delete minio path", slog.String("task", id), slog.String("err", err.Error()))
        }
    }
    
    // persist to config file
    if err := s.saveConfigToFile(s.configPath); err != nil {
        s.log.Warn("failed to persist config", slog.String("err", err.Error()))
    }
    
    s.log.Info("task removed", slog.String("task", id), slog.Bool("was_running", running))
    return true
}

// ListTasks returns current tasks
// 如果使用MinIO存储，会检查每个任务是否有preview图片
func (s *Service) ListTasks() []conf.FrameExtractTask {
    s.mu.Lock()
    out := make([]conf.FrameExtractTask, len(s.cfg.Tasks))
    copy(out, s.cfg.Tasks)
    s.mu.Unlock()
    
    // 如果使用MinIO存储，检查每个任务的preview图片
    if s.cfg.Store == "minio" && s.minio != nil {
        for i := range out {
            task := &out[i]
            // 如果配置中没有preview_image，尝试从MinIO查找
            if task.PreviewImage == "" {
                previewPath := s.findPreviewImageInMinIO(task)
                if previewPath != "" {
                    task.PreviewImage = previewPath
                }
            }
        }
    }
    
    return out
}

// findPreviewImageInMinIO 在MinIO中查找任务的preview图片
func (s *Service) findPreviewImageInMinIO(task *conf.FrameExtractTask) string {
    if s.minio == nil {
        return ""
    }
    
    taskType := task.TaskType
    if taskType == "" {
        taskType = "未分类"
    }
    
    // 构建任务路径前缀
    prefix := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID)) + "/"
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // 列举该任务目录下的所有文件
    objectCh := s.minio.client.ListObjects(ctx, s.minio.bucket, minio.ListObjectsOptions{
        Prefix:    prefix,
        Recursive: false,
    })
    
    // 查找preview_开头的图片
    for object := range objectCh {
        if object.Err != nil {
            continue
        }
        
        name := filepath.Base(object.Key)
        // 检查是否是preview图片
        if strings.HasPrefix(name, "preview_") && 
           (strings.HasSuffix(strings.ToLower(name), ".jpg") || strings.HasSuffix(strings.ToLower(name), ".jpeg")) {
            return object.Key
        }
    }
    
    return ""
}

// GetConfig returns current config
func (s *Service) GetConfig() *conf.FrameExtractorConfig {
    s.mu.Lock()
    defer s.mu.Unlock()
    // return a copy
    cfg := *s.cfg
    return &cfg
}

// UpdateConfig updates storage config (store type, minio settings, etc)
func (s *Service) UpdateConfig(newCfg *conf.FrameExtractorConfig) error {
    s.mu.Lock()
    // update config fields (skip tasks, they are managed separately)
    s.cfg.Enable = newCfg.Enable
    s.cfg.IntervalMs = newCfg.IntervalMs
    s.cfg.OutputDir = newCfg.OutputDir
    s.cfg.Store = newCfg.Store
    s.cfg.MinIO = newCfg.MinIO
    s.mu.Unlock()
    
    s.log.Info("updating config", 
        slog.Bool("enable", s.cfg.Enable),
        slog.String("store", s.cfg.Store),
        slog.String("minio_endpoint", s.cfg.MinIO.Endpoint),
        slog.String("minio_bucket", s.cfg.MinIO.Bucket))
    
    // reinitialize minio if store changed to minio
    if s.cfg.Store == "minio" {
        if err := s.initMinio(); err != nil {
            s.log.Error("failed to init minio", slog.String("err", err.Error()))
            return err
        }
    }
    
    // persist to config file
    if err := s.saveConfigToFile(s.configPath); err != nil {
        s.log.Error("failed to persist config", slog.String("path", s.configPath), slog.String("err", err.Error()))
        return err
    }
    s.log.Info("config persisted", slog.String("path", s.configPath))
    return nil
}

// StartTaskByID starts a stopped task
func (s *Service) StartTaskByID(id string) error {
    s.mu.Lock()
    var task *conf.FrameExtractTask
    for i := range s.cfg.Tasks {
        if s.cfg.Tasks[i].ID == id {
            task = &s.cfg.Tasks[i]
            break
        }
    }
    s.mu.Unlock()
    
    if task == nil {
        return fmt.Errorf("task not found")
    }
    
    // update enabled state
    s.mu.Lock()
    task.Enabled = true
    s.mu.Unlock()
    
    // persist
    if err := s.saveConfigToFile(s.configPath); err != nil {
        s.log.Warn("failed to persist config", slog.String("err", err.Error()))
    }
    
    return s.startTask(*task)
}

// StopTaskByID stops a running task
func (s *Service) StopTaskByID(id string) error {
    s.mu.Lock()
    ch, ok := s.taskStops[id]
    if ok {
        close(ch)
        delete(s.taskStops, id)
    }
    // update enabled state
    for i := range s.cfg.Tasks {
        if s.cfg.Tasks[i].ID == id {
            s.cfg.Tasks[i].Enabled = false
            break
        }
    }
    s.mu.Unlock()
    
    // persist
    if ok {
        if err := s.saveConfigToFile(s.configPath); err != nil {
            s.log.Warn("failed to persist config", slog.String("err", err.Error()))
        }
    }
    
    if !ok {
        return fmt.Errorf("task not running")
    }
    return nil
}

// UpdateTaskInterval updates interval and restarts if running
func (s *Service) UpdateTaskInterval(id string, intervalMs int) error {
    if intervalMs < 200 {
        return fmt.Errorf("interval too small")
    }
    
    s.mu.Lock()
    var task *conf.FrameExtractTask
    wasRunning := false
    if _, ok := s.taskStops[id]; ok {
        wasRunning = true
    }
    for i := range s.cfg.Tasks {
        if s.cfg.Tasks[i].ID == id {
            s.cfg.Tasks[i].IntervalMs = intervalMs
            task = &s.cfg.Tasks[i]
            break
        }
    }
    s.mu.Unlock()
    
    if task == nil {
        return fmt.Errorf("task not found")
    }
    
    // persist
    if err := s.saveConfigToFile(s.configPath); err != nil {
        s.log.Warn("failed to persist config", slog.String("err", err.Error()))
    }
    
    // restart if was running
    if wasRunning {
        _ = s.StopTaskByID(id)
        time.Sleep(100 * time.Millisecond)
        return s.StartTaskByID(id)
    }
    
    return nil
}

// GetTaskStatus returns current running status
func (s *Service) GetTaskStatus(id string) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    _, ok := s.taskStops[id]
    return ok
}

// extractSinglePreviewFrame 抽取单张预览图
func (s *Service) extractSinglePreviewFrame(task conf.FrameExtractTask) {
	s.log.Info("extracting preview frame", slog.String("task", task.ID))
	
	// 使用ffmpeg抽取一帧
	args := buildSingleFrameArgs(task.RtspURL)
	ff := getFFmpegPath()
	cmd := exec.Command(ff, args...)
	
	var frameBuffer bytes.Buffer
	cmd.Stdout = &frameBuffer
	
	if err := cmd.Run(); err != nil {
		s.log.Error("failed to extract preview frame", 
			slog.String("task", task.ID), 
			slog.String("err", err.Error()))
		return
	}
	
	// 生成预览图文件名
	timestamp := time.Now().Format("20060102-150405.000")
	filename := fmt.Sprintf("preview_%s.jpg", timestamp)
	
	// 保存到MinIO或本地
	var imagePath string
	if s.cfg.Store == "minio" && s.minio != nil {
		// 保存到MinIO
		taskType := task.TaskType
		if taskType == "" {
			taskType = "未分类"
		}
		key := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, filename))
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		_, err := s.minio.client.PutObject(ctx, s.minio.bucket, key, 
			bytes.NewReader(frameBuffer.Bytes()), 
			int64(frameBuffer.Len()), 
			minio.PutObjectOptions{ContentType: "image/jpeg"})
		
		if err != nil {
			s.log.Error("failed to upload preview to minio", 
				slog.String("task", task.ID), 
				slog.String("err", err.Error()))
			return
		}
		imagePath = key
		s.log.Info("preview frame uploaded to minio", 
			slog.String("task", task.ID), 
			slog.String("path", key))
	} else {
		// 保存到本地
		localDir := filepath.Join(s.cfg.OutputDir, task.ID)
		if err := os.MkdirAll(localDir, 0o755); err != nil {
			s.log.Error("failed to create local dir", 
				slog.String("task", task.ID), 
				slog.String("err", err.Error()))
			return
		}
		localPath := filepath.Join(localDir, filename)
		if err := os.WriteFile(localPath, frameBuffer.Bytes(), 0o644); err != nil {
			s.log.Error("failed to save preview locally", 
				slog.String("task", task.ID), 
				slog.String("err", err.Error()))
			return
		}
		imagePath = localPath
		s.log.Info("preview frame saved locally", 
			slog.String("task", task.ID), 
			slog.String("path", localPath))
	}
	
	// 更新任务配置
	s.mu.Lock()
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == task.ID {
			s.cfg.Tasks[i].PreviewImage = imagePath
			break
		}
	}
	s.mu.Unlock()
	
	// 持久化
	if err := s.saveConfigToFile(s.configPath); err != nil {
		s.log.Warn("failed to persist config after preview", 
			slog.String("err", err.Error()))
	}
}

// SaveAlgorithmConfig 保存算法配置到MinIO
func (s *Service) SaveAlgorithmConfig(taskID string, config []byte) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	// 查找任务
	s.mu.Lock()
	var task *conf.FrameExtractTask
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == taskID {
			task = &s.cfg.Tasks[i]
			break
		}
	}
	s.mu.Unlock()
	
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	// 保存配置文件到MinIO
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	configKey := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, "algo_config.json"))
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	_, err := s.minio.client.PutObject(ctx, s.minio.bucket, configKey, 
		bytes.NewReader(config), 
		int64(len(config)), 
		minio.PutObjectOptions{ContentType: "application/json"})
	
	if err != nil {
		return fmt.Errorf("failed to save config to minio: %w", err)
	}
	
	// 更新任务状态为已配置
	s.mu.Lock()
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == taskID {
			s.cfg.Tasks[i].ConfigStatus = "configured"
			break
		}
	}
	s.mu.Unlock()
	
	// 持久化
	if err := s.saveConfigToFile(s.configPath); err != nil {
		s.log.Warn("failed to persist config", slog.String("err", err.Error()))
	}
	
	s.log.Info("algorithm config saved", 
		slog.String("task", taskID), 
		slog.String("path", configKey))
	
	return nil
}

// GetAlgorithmConfig 获取算法配置
func (s *Service) GetAlgorithmConfig(taskID string) ([]byte, error) {
	if s.minio == nil {
		return nil, fmt.Errorf("minio not initialized")
	}
	
	// 查找任务
	s.mu.Lock()
	var task *conf.FrameExtractTask
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == taskID {
			task = &s.cfg.Tasks[i]
			break
		}
	}
	s.mu.Unlock()
	
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	
	// 从MinIO读取配置
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	configKey := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, "algo_config.json"))
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	object, err := s.minio.client.GetObject(ctx, s.minio.bucket, configKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get config from minio: %w", err)
	}
	defer object.Close()
	
	// 读取全部内容
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, object); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	return buf.Bytes(), nil
}

// GetAlgorithmConfigPath 获取算法配置文件在MinIO中的路径
func (s *Service) GetAlgorithmConfigPath(taskID string) string {
	if s.minio == nil {
		return ""
	}
	
	// 查找任务
	s.mu.Lock()
	var task *conf.FrameExtractTask
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == taskID {
			task = &s.cfg.Tasks[i]
			break
		}
	}
	s.mu.Unlock()
	
	if task == nil {
		return ""
	}
	
	// 构建配置文件路径
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	configKey := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, "algo_config.json"))
	
	return configKey
}

// StartWithConfig 配置完成后启动抽帧
func (s *Service) StartWithConfig(taskID string) error {
	s.mu.Lock()
	var task *conf.FrameExtractTask
	for i := range s.cfg.Tasks {
		if s.cfg.Tasks[i].ID == taskID {
			if s.cfg.Tasks[i].ConfigStatus != "configured" {
				s.mu.Unlock()
				return fmt.Errorf("task not configured yet")
			}
			task = &s.cfg.Tasks[i]
			break
		}
	}
	s.mu.Unlock()
	
	if task == nil {
		return fmt.Errorf("task not found")
	}
	
	// 启动任务
	return s.StartTaskByID(taskID)
}

// GetTasksByType 获取指定类型的任务列表
func (s *Service) GetTasksByType(taskType string) []conf.FrameExtractTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var result []conf.FrameExtractTask
	for _, task := range s.cfg.Tasks {
		if task.TaskType == taskType {
			result = append(result, task)
		}
	}
	return result
}

// GetPresignedURL 获取MinIO预签名URL
func (s *Service) GetPresignedURL(objectPath string, expiry time.Duration) (string, error) {
	if s.minio == nil {
		return "", fmt.Errorf("minio not initialized")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	presignedURL, err := s.minio.client.PresignedGetObject(ctx, s.minio.bucket, objectPath, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	return presignedURL.String(), nil
}

// GetStats 获取监控统计信息
func (s *Service) GetStats() TaskStats {
    s.mu.Lock()
    s.statsMu.Lock()
    defer s.statsMu.Unlock()
    defer s.mu.Unlock()
    
    // 计算统计数据
    totalTasks := len(s.cfg.Tasks)
    runningTasks := len(s.taskStops)
    stoppedTasks := totalTasks - runningTasks
    configuredTasks := 0
    pendingTasks := 0
    
    taskDetails := make([]TaskMonitorInfo, 0, totalTasks)
    
    for _, task := range s.cfg.Tasks {
        isRunning := false
        if _, ok := s.taskStops[task.ID]; ok {
            isRunning = true
        }
        
        status := "stopped"
        if isRunning {
            status = "running"
        }
        
        if task.ConfigStatus == "configured" {
            configuredTasks++
        } else {
            pendingTasks++
        }
        
        info := TaskMonitorInfo{
            ID:           task.ID,
            TaskType:     task.TaskType,
            Status:       status,
            ConfigStatus: task.ConfigStatus,
            IntervalMs:   getIntervalMs(task, s.cfg),
            OutputPath:   task.OutputPath,
            // 以下字段需要从运行时数据获取，暂时使用默认值
            LastFrameTime: time.Time{},
            FrameCount:    0,
            ErrorCount:    0,
            Uptime:        0,
            StartTime:     time.Time{},
        }
        
        taskDetails = append(taskDetails, info)
    }
    
    // 获取系统信息
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    
    systemInfo := SystemMonitorInfo{
        Goroutines:    runtime.NumGoroutine(),
        MemoryUsageMB: float64(memStats.Alloc) / 1024 / 1024,
        CPUCores:      runtime.NumCPU(),
    }
    
    // 获取抽帧速率统计
    s.frameRateMu.RLock()
    framesPerSec := s.frameRateStats.framesPerSec
    totalFrames := s.frameRateStats.totalFrames
    s.frameRateMu.RUnlock()
    
    stats := TaskStats{
        TotalTasks:      totalTasks,
        RunningTasks:    runningTasks,
        StoppedTasks:    stoppedTasks,
        ConfiguredTasks: configuredTasks,
        PendingTasks:    pendingTasks,
        TaskDetails:     taskDetails,
        SystemInfo:      systemInfo,
        FramesPerSec:    framesPerSec,
        TotalFrames:     totalFrames,
        UpdatedAt:       time.Now(),
    }
    
    // 缓存统计数据
    s.stats = stats
    
    return stats
}

// global accessor for API layer
var gService *Service

func SetGlobal(s *Service) { gService = s }
func GetGlobal() *Service { return gService }


