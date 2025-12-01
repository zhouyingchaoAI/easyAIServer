package videortsp

import (
	"context"
	"easydarwin/pkg/lalmax/hook"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storer 数据存储接口
type Storer interface {
	Create(task *StreamTask) error
	Update(task *StreamTask) error
	Delete(id string) error
	GetByID(id string) (*StreamTask, error)
	GetByStreamName(streamName string) (*StreamTask, error)
	List(limit, offset int, search string) ([]*StreamTask, int64, error)
}

// ProcessManager 进程管理器接口
type ProcessManager interface {
	StartProcess(task *StreamTask, rtspURL string) error
	StopProcess(streamName string) error
	IsRunning(streamName string) bool
	GetProcessStatus(streamName string) (string, error)
	GetStderr(streamName string) string // 获取进程的stderr输出（用于错误诊断）
	BuildFFmpegCommand(task *StreamTask, rtspURL string) (string, []string, error) // 构建FFmpeg命令（用于调试）
}

// Core 业务核心
type Core struct {
	store          Storer
	processManager ProcessManager
	rtspHost       string // RTSP播放地址，如 "127.0.0.1:15544"（用于生成播放URL）
	rtmpHost       string // RTMP推流服务器地址，如 "127.0.0.1:21935"（用于FFmpeg推流）
	logger         *slog.Logger
}

// NewCore 创建核心服务
func NewCore(store Storer, rtspHost, rtmpHost string, logger *slog.Logger) *Core {
	pm := NewFFmpegProcessManager(logger)
	core := &Core{
		store:          store,
		rtspHost:       rtspHost,
		rtmpHost:       rtmpHost,
		logger:         logger,
		processManager: pm,
	}
	return core
}

// CreateTask 创建流任务
func (c *Core) CreateTask(input CreateTaskInput) (*StreamTask, error) {
	// 生成任务ID（用于自动生成流名称）
	taskID := generateID()
	
	// 完全自动生成流名称，参考直播服务的 stream_<id> 格式
	// 使用不同的前缀 "video_" 避免与直播服务冲突（直播服务使用 "stream_"）
	// 使用UUID的前12位（去除连字符）确保唯一性和简洁性
	cleanID := strings.ReplaceAll(taskID, "-", "")[:12]
	streamName := fmt.Sprintf("video_%s", cleanID)
	
	// 检查streamName是否已存在，如果存在则使用完整UUID（去除连字符）
	existing, _ := c.store.GetByStreamName(streamName)
	if existing != nil {
		streamName = fmt.Sprintf("video_%s", strings.ReplaceAll(taskID, "-", ""))
		// 再次检查，理论上UUID是唯一的，不会重复
		existing, _ = c.store.GetByStreamName(streamName)
		if existing != nil {
			return nil, fmt.Errorf("failed to generate unique stream name, please try again")
		}
	}
	
	c.logger.Info("auto-generated stream name", "task_id", taskID, "stream_name", streamName)

	// 验证视频路径格式
	if input.VideoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}
	
	// 清理路径（移除前后空格）
	videoPath := strings.TrimSpace(input.VideoPath)
	if videoPath == "" {
		return nil, fmt.Errorf("video path cannot be empty")
	}
	
	// 确保使用绝对路径
	if !filepath.IsAbs(videoPath) {
		// 尝试使用GetRealPath转换为绝对路径（如果可能是相对路径）
		// 但对于VOD路径，应该已经是绝对路径了
		return nil, fmt.Errorf("video path must be absolute path, got: %s. Please provide full path like /path/to/video.mp4", videoPath)
	}
	
	// 规范化路径（处理 .. 和 .）
	videoPath = filepath.Clean(videoPath)
	
	// 验证视频文件是否存在
	fileInfo, err := os.Stat(videoPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", videoPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to access video file %s: %w", videoPath, err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("video path is a directory, not a file: %s", videoPath)
	}

	task := &StreamTask{
		ID:         taskID, // 使用之前生成的ID
		Name:       input.Name,
		VideoPath:  videoPath, // 使用处理后的绝对路径
		StreamName: streamName, // 使用清理后的流名称或自动生成的流名称
		Status:     StatusStopped,
		Enabled:    input.Enabled,
		Loop:       input.Loop,
		VideoCodec: "libx264",
		AudioCodec: "aac",
		Preset:     "superfast", // 使用superfast（参考用户测试成功的命令）
		Tune:       "zerolatency",
	}

	if input.VideoCodec != "" {
		task.VideoCodec = input.VideoCodec
	}
	if input.AudioCodec != "" {
		task.AudioCodec = input.AudioCodec
	}
	if input.Preset != "" {
		task.Preset = input.Preset
	}
	if input.Tune != "" {
		task.Tune = input.Tune
	}

	err = c.store.Create(task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	c.updateRTSPURL(task)

	// 如果启用，立即启动
	if task.Enabled {
		if err := c.StartStream(task.ID); err != nil {
			c.logger.Warn("failed to start stream after creation", "error", err, "task_id", task.ID)
		}
	}

	return task, nil
}

// UpdateTask 更新任务
func (c *Core) UpdateTask(id string, input UpdateTaskInput) (*StreamTask, error) {
	task, err := c.store.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	wasRunning := task.Status == StatusRunning

	// 如果streamName改变，需要先停止旧流
	if input.StreamName != "" && input.StreamName != task.StreamName {
		if wasRunning {
			c.StopStream(task.ID)
		}
		// 检查新streamName是否已存在
		existing, _ := c.store.GetByStreamName(input.StreamName)
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("stream name '%s' already exists", input.StreamName)
		}
		task.StreamName = input.StreamName
	}

	if input.Name != nil && *input.Name != "" {
		task.Name = *input.Name
	}
	if input.VideoPath != "" {
		task.VideoPath = input.VideoPath
	}
	if input.VideoCodec != nil {
		task.VideoCodec = *input.VideoCodec
	}
	if input.AudioCodec != nil {
		task.AudioCodec = *input.AudioCodec
	}
	if input.Preset != nil {
		task.Preset = *input.Preset
	}
	if input.Tune != nil {
		task.Tune = *input.Tune
	}
	if input.Loop != nil {
		task.Loop = *input.Loop
	}

	err = c.store.Update(task)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	c.updateRTSPURL(task)

	// 如果之前在运行，重启
	if wasRunning {
		c.StopStream(task.ID)
		if task.Enabled {
			time.Sleep(500 * time.Millisecond)
			c.StartStream(task.ID)
		}
	}

	return task, nil
}

// DeleteTask 删除任务
func (c *Core) DeleteTask(id string) error {
	task, err := c.store.GetByID(id)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 如果正在运行，先停止
	if task.Status == StatusRunning {
		c.StopStream(id)
	}

	return c.store.Delete(id)
}

// StartStream 启动流
func (c *Core) StartStream(id string) error {
	task, err := c.store.GetByID(id)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 检查当前任务状态
	if task.Status == StatusRunning {
		// 检查进程是否真的在运行
		if c.processManager.IsRunning(task.StreamName) {
			return fmt.Errorf("stream is already running")
		}
		// 如果状态是running但进程不存在，更新状态
		task.Status = StatusStopped
		c.store.Update(task)
	}

	// 检查是否有其他任务使用相同的流名称且正在运行
	// 如果有，先停止它们（避免RTMP服务器检测到重复推流）
	allTasks, _, err := c.store.List(1000, 0, "")
	if err == nil {
		for _, t := range allTasks {
			if t.ID != id && t.StreamName == task.StreamName {
				if t.Status == StatusRunning || c.processManager.IsRunning(t.StreamName) {
					c.logger.Warn("stopping duplicate stream task", "old_task_id", t.ID, "stream_name", t.StreamName)
					if err := c.processManager.StopProcess(t.StreamName); err != nil {
						c.logger.Warn("failed to stop duplicate stream", "error", err, "task_id", t.ID)
					}
					// 更新旧任务状态
					t.Status = StatusStopped
					t.Error = "stopped due to duplicate stream name"
					c.store.Update(t)
				}
			}
		}
	}

	// 确保当前流名称的所有进程都已停止（防止RTMP服务器检测到重复推流）
	// 参考流直播功能：确保停止时正确清理Session
	if c.processManager.IsRunning(task.StreamName) {
		c.logger.Warn("stopping existing process before starting new one", "stream_name", task.StreamName)
		if err := c.processManager.StopProcess(task.StreamName); err != nil {
			c.logger.Warn("failed to stop existing process", "error", err, "stream_name", task.StreamName)
		}
	}
	
	// 再次确认所有使用相同流名称的进程都已停止
	for _, t := range allTasks {
		if t.ID != id && t.StreamName == task.StreamName && c.processManager.IsRunning(t.StreamName) {
			c.logger.Warn("found another task with same stream name still running, stopping it", "old_task_id", t.ID, "stream_name", task.StreamName)
			c.processManager.StopProcess(t.StreamName)
		}
	}
	
	// 智能等待：确保旧的hook session被清理，避免"流已存在"错误
	c.logger.Info("waiting for old session to be cleaned up", "stream_name", task.StreamName)
	if !c.waitForSessionCleanup(task.StreamName, 10*time.Second) {
		c.logger.Warn("old session may still exist, but continuing anyway", "stream_name", task.StreamName)
	}
	
	// 额外等待时间，确保RTMP服务器完全清理内部会话
	// RTMP服务器可能需要额外时间清理内部状态
	time.Sleep(2 * time.Second)

	// 使用RTMP推流URL（EasyDarwin会自动转发为RTSP）
	// RTMP推流URL格式: rtmp://host:port/live/streamName
	// 注意：使用 /live/ 路径（与直播服务一致，用户测试成功的命令也使用此路径）
	// task.StreamName 应该已经是清理过的纯流名称（格式：video_xxx）
	rtmpURL := fmt.Sprintf("rtmp://%s/live/%s", c.rtmpHost, task.StreamName)
	err = c.processManager.StartProcess(task, rtmpURL)
	if err != nil {
		task.Status = StatusError
		task.Error = err.Error()
		c.store.Update(task)
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// 等待FFmpeg进程启动并建立RTMP连接
	// 增加等待时间，让FFmpeg有时间建立连接并开始推流
	// FFmpeg需要时间：1) 启动进程 2) 读取视频文件 3) 建立RTMP连接 4) 开始推流
	time.Sleep(3 * time.Second)
	
	// 检查进程是否还在运行
	if !c.processManager.IsRunning(task.StreamName) {
		// 进程已退出，获取FFmpeg的实际错误信息
		stderr := c.processManager.GetStderr(task.StreamName)
		errorMsg := "ffmpeg process exited after start"
		
		// 分析stderr，提取真正的错误信息
		if stderr != "" {
			lines := strings.Split(stderr, "\n")
			var errorLines []string
			var hasRealError bool
			
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				
				// 忽略正常的编码输出（这些不是错误）
				lowerLine := strings.ToLower(line)
				if strings.Contains(lowerLine, "frame=") ||
					strings.Contains(lowerLine, "fps=") ||
					strings.Contains(lowerLine, "bitrate=") ||
					strings.Contains(lowerLine, "size=") ||
					strings.Contains(lowerLine, "time=") ||
					strings.Contains(lowerLine, "speed=") ||
					strings.Contains(lowerLine, "encoder") ||
					strings.Contains(lowerLine, "metadata") ||
					strings.Contains(lowerLine, "side data") {
					continue // 跳过正常的编码输出
				}
				
				// 查找真正的错误信息
				if strings.Contains(lowerLine, "error") ||
					strings.Contains(lowerLine, "failed") ||
					strings.Contains(lowerLine, "connection refused") ||
					strings.Contains(lowerLine, "connection timed out") ||
					strings.Contains(lowerLine, "no such file") ||
					strings.Contains(lowerLine, "invalid") ||
					strings.Contains(lowerLine, "cannot") ||
					strings.Contains(lowerLine, "unable") {
					errorLines = append(errorLines, line)
					hasRealError = true
					if len(errorLines) >= 3 {
						break
					}
				}
			}
			
			if hasRealError && len(errorLines) > 0 {
				errorMsg = fmt.Sprintf("%s: %s", errorMsg, strings.Join(errorLines, "; "))
			} else {
				// 如果没有找到明显的错误，可能是RTMP连接问题
				errorMsg += ": RTMP connection may have failed, check RTMP server and network"
			}
		} else {
			errorMsg += ", check video file, ffmpeg parameters, and RTMP server connection"
		}
		
		task.Status = StatusError
		task.Error = errorMsg
		c.store.Update(task)
		c.logger.Error("ffmpeg process exited", 
			"stream_name", task.StreamName,
			"error", errorMsg,
			"stderr", stderr)
		return fmt.Errorf("ffmpeg process exited: %s", task.Error)
	}

	// 等待FFmpeg进程启动并让RTMP服务器注册会话
	// 这给RTMP服务器时间创建hook session和接收序列头
	// 由于已经等待了3秒检查进程，这里可以减少等待时间
	time.Sleep(1 * time.Second)

	// 关键：智能等待流就绪，通过检查序列头而不是固定等待时间
	// 这有助于避免RTMP转RTSP时的"invalid video and audio info"错误
	// 增加超时时间，确保有足够时间接收序列头
	c.logger.Info("waiting for stream to be ready", "stream_name", task.StreamName)
	if !c.waitForStreamReady(task.StreamName, 30*time.Second) {
		c.logger.Warn("stream not ready after timeout, but continuing anyway", "stream_name", task.StreamName)
		// 即使超时，也再等待一段时间，确保序列头有时间被处理
		time.Sleep(2 * time.Second)
	}

	task.Status = StatusRunning
	task.Error = ""
	task.Enabled = true
	err = c.store.Update(task)
	if err != nil {
		c.logger.Error("failed to update task status", "error", err, "task_id", id)
	}

	c.logger.Info("stream started", "task_id", id, "stream_name", task.StreamName, "rtmp_url", rtmpURL, "rtsp_url", task.RTSPURL)
	return nil
}

// StopStream 停止流
func (c *Core) StopStream(id string) error {
	task, err := c.store.GetByID(id)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.Status != StatusRunning {
		return nil
	}

	err = c.processManager.StopProcess(task.StreamName)
	if err != nil {
		task.Status = StatusError
		task.Error = err.Error()
		c.store.Update(task)
		return fmt.Errorf("failed to stop stream: %w", err)
	}

	task.Status = StatusStopped
	task.Error = ""
	c.store.Update(task)

	c.logger.Info("stream stopped", "task_id", id, "stream_name", task.StreamName)
	return nil
}

// GetTask 获取任务
func (c *Core) GetTask(id string) (*StreamTask, error) {
	task, err := c.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	c.updateRTSPURL(task)
	return task, nil
}

// ListTasks 列出任务
func (c *Core) ListTasks(limit, offset int, search string) ([]*StreamTask, int64, error) {
	tasks, total, err := c.store.List(limit, offset, search)
	if err != nil {
		return nil, 0, err
	}

	// 更新RTSP URL并检查运行状态
	for _, task := range tasks {
		c.updateRTSPURL(task)
		// 检查进程是否还在运行
		if task.Status == StatusRunning && !c.processManager.IsRunning(task.StreamName) {
			task.Status = StatusStopped
			c.store.Update(task)
		}
	}

	return tasks, total, nil
}

// updateRTSPURL 更新RTSP播放URL
// 使用 /live/ 路径（与直播服务一致），流名称使用 video_ 前缀避免冲突
func (c *Core) updateRTSPURL(task *StreamTask) {
	task.RTSPURL = fmt.Sprintf("rtsp://%s/live/%s", c.rtspHost, task.StreamName)
}

// GetRTMPHost 获取RTMP服务器地址（用于API）
func (c *Core) GetRTMPHost() string {
	return c.rtmpHost
}

// generateID 生成任务ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// waitForSessionCleanup 智能等待旧的hook session被清理
// 返回true表示session已清理，false表示超时
// 注意：RTMP推流URL格式是 rtmp://host:port/live/streamName
// 其中video是app name，streamName是流名称
// lal的hook session使用的流名称就是streamName（不包含app name）
func (c *Core) waitForSessionCleanup(streamName string, timeout time.Duration) bool {
	startTime := time.Now()
	checkInterval := 200 * time.Millisecond // 每200ms检查一次
	
	for time.Since(startTime) < timeout {
		// 检查hook session是否还存在
		// streamName就是完整的流名称（如 video_xxx），不包含app name
		ok, _ := hook.GetHookSessionManagerInstance().GetHookSession(streamName)
		if !ok {
			// Session已清理
			c.logger.Info("old session cleaned up", 
				"stream_name", streamName,
				"wait_duration_ms", time.Since(startTime).Milliseconds())
			return true
		}
		
		// 等待一段时间后再次检查
		time.Sleep(checkInterval)
	}
	
	c.logger.Warn("old session may still exist after timeout", 
		"stream_name", streamName,
		"timeout_seconds", timeout.Seconds())
	return false
}

// waitForStreamReady 智能等待流就绪，检查是否有视频/音频序列头
// 返回true表示流已就绪，false表示超时
func (c *Core) waitForStreamReady(streamName string, timeout time.Duration) bool {
	startTime := time.Now()
	checkInterval := 500 * time.Millisecond // 每500ms检查一次，给RTMP服务器更多时间
	minWaitTime := 5 * time.Second // 增加最小等待时间，确保FFmpeg有足够时间发送序列头
	
	// 先等待最小时间，让FFmpeg有足够时间推流并让RTMP服务器接收序列头
	time.Sleep(minWaitTime)
	
	// 连续检查次数，确保序列头稳定存在
	readyCount := 0
	requiredReadyCount := 2 // 需要连续2次检查都成功才认为就绪
	
	for time.Since(startTime) < timeout {
		// 检查流会话是否存在
		ok, session := hook.GetHookSessionManagerInstance().GetHookSession(streamName)
		if ok && session != nil {
			// 检查是否有视频序列头
			videoHeader := session.GetVideoSeqHeaderMsg()
			if videoHeader != nil && len(videoHeader.Payload) > 0 {
				// 有视频序列头且有效
				// 检查音频序列头（可选，因为有些流可能没有音频）
				audioHeader := session.GetAudioSeqHeaderMsg()
				hasAudio := audioHeader != nil && len(audioHeader.Payload) > 0
				
				readyCount++
				if readyCount >= requiredReadyCount {
					// 连续多次检查都成功，流已稳定就绪
					c.logger.Info("stream is ready", 
						"stream_name", streamName,
						"has_video", true,
						"has_audio", hasAudio,
						"wait_duration_ms", time.Since(startTime).Milliseconds(),
						"ready_checks", readyCount)
					return true
				}
			} else {
				// 序列头不存在，重置计数
				readyCount = 0
			}
		} else {
			// 会话不存在，重置计数
			readyCount = 0
		}
		
		// 等待一段时间后再次检查
		time.Sleep(checkInterval)
	}
	
	c.logger.Warn("stream not ready after timeout", 
		"stream_name", streamName,
		"timeout_seconds", timeout.Seconds(),
		"final_ready_count", readyCount)
	return false
}

// CreateTaskInput 创建任务输入
type CreateTaskInput struct {
	Name        string `json:"name" binding:"required"`
	VideoPath   string `json:"videoPath" binding:"required"`
	StreamName  string `json:"streamName"` // 已废弃，流名称完全自动生成（参考直播服务的 stream_<id> 格式）
	Enabled     bool   `json:"enabled"`
	Loop        bool   `json:"loop"`
	VideoCodec  string `json:"videoCodec"`
	AudioCodec  string `json:"audioCodec"`
	Preset      string `json:"preset"`
	Tune        string `json:"tune"`
}

// UpdateTaskInput 更新任务输入
type UpdateTaskInput struct {
	Name        *string `json:"name"`
	VideoPath   string  `json:"videoPath"`
	StreamName  string  `json:"streamName"`
	Enabled     *bool   `json:"enabled"`
	Loop        *bool   `json:"loop"`
	VideoCodec  *string `json:"videoCodec"`
	AudioCodec  *string `json:"audioCodec"`
	Preset      *string `json:"preset"`
	Tune        *string `json:"tune"`
}

// Cleanup 清理资源
func (c *Core) Cleanup() {
	// 停止所有运行中的流
	tasks, _, err := c.store.List(1000, 0, "")
	if err != nil {
		c.logger.Error("failed to load tasks for cleanup", "error", err)
		return
	}

	for _, task := range tasks {
		if task.Status == StatusRunning {
			if err := c.processManager.StopProcess(task.StreamName); err != nil {
				c.logger.Error("failed to stop stream during cleanup", "error", err, "stream_name", task.StreamName)
			}
		}
	}
}

// Init 初始化，启动所有启用的流
func (c *Core) Init(ctx context.Context) error {
	tasks, _, err := c.store.List(1000, 0, "")
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	for _, task := range tasks {
		if task.Enabled && task.Status != StatusRunning {
			// 异步启动，避免阻塞
			go func(t *StreamTask) {
				time.Sleep(1 * time.Second) // 延迟启动，避免同时启动太多
				if err := c.StartStream(t.ID); err != nil {
					c.logger.Error("failed to start stream during init", "error", err, "task_id", t.ID)
				}
			}(task)
		}
	}

	return nil
}

