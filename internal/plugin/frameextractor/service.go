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
}

func New(cfg *conf.FrameExtractorConfig) *Service {
    return &Service{
        cfg:       cfg,
        log:       slog.Default(),
        stop:      make(chan struct{}),
        taskStops: make(map[string]chan struct{}),
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
func (s *Service) ListTasks() []conf.FrameExtractTask {
    s.mu.Lock()
    defer s.mu.Unlock()
    out := make([]conf.FrameExtractTask, len(s.cfg.Tasks))
    copy(out, s.cfg.Tasks)
    return out
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
		key := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.OutputPath, filename))
		
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
		localDir := filepath.Join(s.cfg.OutputDir, task.OutputPath)
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
	configKey := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.OutputPath, "algo_config.json"))
	
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
	configKey := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.OutputPath, "algo_config.json"))
	
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

// global accessor for API layer
var gService *Service

func SetGlobal(s *Service) { gService = s }
func GetGlobal() *Service { return gService }


