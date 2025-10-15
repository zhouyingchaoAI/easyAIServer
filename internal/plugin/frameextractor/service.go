package frameextractor

import (
    "context"
    "errors"
    "easydarwin/internal/conf"
    "easydarwin/utils/pkg/system"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "sync"
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
    if _, ok := s.taskStops[t.ID]; ok {
        return nil
    }
    done := make(chan struct{})
    s.taskStops[t.ID] = done
    s.wg.Add(1)
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
    return s.startTask(t)
}

// RemoveTask stops and removes a task by id
func (s *Service) RemoveTask(id string) bool {
    s.mu.Lock()
    ch, ok := s.taskStops[id]
    var removedTask *conf.FrameExtractTask
    if ok {
        close(ch)
        delete(s.taskStops, id)
    }
    // remove from cfg slice and keep reference for minio cleanup
    if ok {
        tasks := s.cfg.Tasks[:0]
        for _, it := range s.cfg.Tasks {
            if it.ID != id {
                tasks = append(tasks, it)
            } else {
                removedTask = &it
            }
        }
        s.cfg.Tasks = tasks
    }
    s.mu.Unlock()
    
    // delete minio path if store is minio
    if ok && removedTask != nil && s.cfg.Store == "minio" && s.minio != nil {
        if err := s.deleteMinioPath(*removedTask); err != nil {
            s.log.Warn("failed to delete minio path", slog.String("task", id), slog.String("err", err.Error()))
        }
    }
    
    // persist to config file
    if ok {
        if err := s.saveConfigToFile(s.configPath); err != nil {
            s.log.Warn("failed to persist config", slog.String("err", err.Error()))
        }
    }
    return ok
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
    
    // reinitialize minio if store changed to minio
    if s.cfg.Store == "minio" {
        if err := s.initMinio(); err != nil {
            return err
        }
    }
    
    // persist to config file
    if err := s.saveConfigToFile(s.configPath); err != nil {
        s.log.Warn("failed to persist config", slog.String("err", err.Error()))
    }
    return nil
}

// global accessor for API layer
var gService *Service

func SetGlobal(s *Service) { gService = s }
func GetGlobal() *Service { return gService }


