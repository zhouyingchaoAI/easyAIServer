package videortsp

import (
	"bytes"
	"context"
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
)

// FFmpegProcessManager FFmpeg进程管理器
type FFmpegProcessManager struct {
	logger        *slog.Logger
	processes     map[string]*exec.Cmd
	stderrBuffers map[string]*bytes.Buffer // 保存每个进程的stderr输出，用于错误诊断
	mu            sync.RWMutex
}

// NewFFmpegProcessManager 创建FFmpeg进程管理器
func NewFFmpegProcessManager(logger *slog.Logger) *FFmpegProcessManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &FFmpegProcessManager{
		logger:        logger,
		processes:     make(map[string]*exec.Cmd),
		stderrBuffers: make(map[string]*bytes.Buffer),
	}
}

// BuildFFmpegCommand 构建FFmpeg命令（用于调试和测试）
func (m *FFmpegProcessManager) BuildFFmpegCommand(task *StreamTask, rtspURL string) (string, []string, error) {
	// 验证视频文件是否存在
	if _, err := os.Stat(task.VideoPath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("video file not found: %s", task.VideoPath)
	}

	// 构建FFmpeg命令
	args := []string{}

	// 循环播放
	if task.Loop {
		args = append(args, "-stream_loop", "-1")
	}

	// 实时模式（按视频帧率推流）
	args = append(args, "-re")

	// 输入文件
	args = append(args, "-i", task.VideoPath)

	// 视频编码设置（简化，参考用户测试成功的命令）
	args = append(args, "-c:v", task.VideoCodec)
	if task.VideoCodec != "copy" {
		// 只有在转码时才添加这些参数
		preset := task.Preset
		if preset == "" {
			preset = "superfast" // 默认使用superfast（用户测试成功的命令使用superfast）
		}
		args = append(args, "-preset", preset)

		tune := task.Tune
		if tune == "" {
			tune = "zerolatency" // 默认使用zerolatency（用户测试成功的命令使用zerolatency）
		}
		args = append(args, "-tune", tune)
	}

	// 音频编码设置（简化，参考用户测试成功的命令）
	if task.AudioCodec != "" && task.AudioCodec != "copy" {
		args = append(args, "-c:a", task.AudioCodec)
		if task.AudioCodec == "aac" {
			// AAC编码的额外参数（参考用户测试成功的命令）
			args = append(args, "-ar", "44100")
			args = append(args, "-ac", "2") // 立体声
		}
	} else if task.AudioCodec == "" {
		// 如果没有指定音频编码，尝试copy（如果视频有音频轨道）
		args = append(args, "-c:a", "copy")
	}

	// RTMP推流格式
	args = append(args, "-f", "flv")

	// RTMP URL格式: rtmp://host:port/live/streamName
	args = append(args, rtspURL)

	// 获取FFmpeg路径（会尝试多个位置）
	ffmpegPath, err := getFFmpegPath()
	if err != nil {
		return "", nil, fmt.Errorf("ffmpeg not found: %w", err)
	}

	return ffmpegPath, args, nil
}

// StartProcess 启动FFmpeg进程推送RTSP流
func (m *FFmpegProcessManager) StartProcess(task *StreamTask, rtspURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已在运行
	if cmd, exists := m.processes[task.StreamName]; exists && cmd != nil && cmd.Process != nil {
		// 检查进程是否还在运行
		if err := cmd.Process.Signal(os.Signal(nil)); err == nil {
			return fmt.Errorf("stream '%s' is already running", task.StreamName)
		}
		// 进程已退出，清理
		delete(m.processes, task.StreamName)
	}

	// 使用BuildFFmpegCommand构建命令
	ffmpegPath, args, err := m.BuildFFmpegCommand(task, rtspURL)
	if err != nil {
		return err
	}

	m.logger.Info("using ffmpeg", "path", ffmpegPath)

	// 输出完整的FFmpeg命令用于调试
	cmdStr := ffmpegPath + " " + strings.Join(args, " ")
	m.logger.Info("ffmpeg command",
		"stream_name", task.StreamName,
		"command", cmdStr,
		"args_count", len(args))

	// 创建命令
	cmd := exec.CommandContext(context.Background(), ffmpegPath, args...)

	// 捕获stderr以便在进程失败时获取错误信息
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	cmd.Stdout = os.Stdout

	// 启动进程
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// 保存进程
	m.processes[task.StreamName] = cmd

	// 保存stderr缓冲区（用于错误诊断）
	m.stderrBuffers[task.StreamName] = &stderrBuf

	// 监控进程
	go m.monitorProcess(task.StreamName, cmd)

	m.logger.Info("ffmpeg process started",
		"stream_name", task.StreamName,
		"rtmp_url", rtspURL, // 注意：这里实际是RTMP URL
		"pid", cmd.Process.Pid,
		"video_path", task.VideoPath,
		"ffmpeg_path", ffmpegPath)

	return nil
}

// getFFmpegPath 获取FFmpeg可执行文件路径
// 参考直播服务的 FFMPEG() 函数实现，使用当前工作目录中的 ffmpeg
// 如果工作目录中没有，则尝试在系统 PATH 中查找
func getFFmpegPath() (string, error) {
	dir, _ := os.Getwd() // 使用当前工作目录，与直播服务保持一致

	var localPath string
	switch runtime.GOOS {
	case "windows":
		localPath = filepath.Join(dir, "ffmpeg.exe")
	case "linux":
		// Linux 系统与直播服务保持一致：直接返回路径并设置执行权限
		localPath = filepath.Join(dir, "ffmpeg")
		if _, err := os.Stat(localPath); err == nil {
			os.Chmod(localPath, 0755)
			return localPath, nil
		}
	default:
		// macOS 等其他系统
		localPath = filepath.Join(dir, "ffmpeg")
	}

	// 检查当前工作目录中的 ffmpeg 是否存在
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// 如果工作目录中没有，尝试在系统 PATH 中查找
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}

	// 都没找到，返回错误
	return "", fmt.Errorf("ffmpeg not found in working directory (%s) or system PATH. Please install ffmpeg or place it in the working directory", dir)
}

// StopProcess 停止FFmpeg进程
func (m *FFmpegProcessManager) StopProcess(streamName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd, exists := m.processes[streamName]
	if !exists || cmd == nil || cmd.Process == nil {
		return nil // 已经停止
	}

	// 发送中断信号
	err := cmd.Process.Kill()
	if err != nil {
		m.logger.Warn("failed to kill process", "error", err, "stream_name", streamName)
	}

	// 等待进程退出（参考流直播功能的停止机制）
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// 进程已退出
		m.logger.Info("ffmpeg process exited", "stream_name", streamName)
	case <-time.After(5 * time.Second):
		// 超时，强制杀死
		m.logger.Warn("ffmpeg process timeout, force killing", "stream_name", streamName)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}

	// 参考流直播功能：停止后等待一段时间，确保RTMP服务器完全清理Session
	// 这有助于避免"流已存在"的错误
	// 增加等待时间，确保RTMP服务器内部会话完全清理
	time.Sleep(3 * time.Second)

	// 从map中移除
	delete(m.processes, streamName)

	m.logger.Info("ffmpeg process stopped", "stream_name", streamName)
	return nil
}

// IsRunning 检查进程是否在运行
func (m *FFmpegProcessManager) IsRunning(streamName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd, exists := m.processes[streamName]
	if !exists || cmd == nil || cmd.Process == nil {
		m.logger.Info("ffmpeg process not found in manager",
			"stream_name", streamName)
		return false
	}

	// 如果cmd.ProcessState已存在且标记已退出，直接返回false
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		m.logger.Info("ffmpeg process state indicates exit",
			"stream_name", streamName,
			"pid", cmd.Process.Pid)
		return false
	}

	return true
}

// GetProcessStatus 获取进程状态
func (m *FFmpegProcessManager) GetProcessStatus(streamName string) (string, error) {
	if m.IsRunning(streamName) {
		return StatusRunning, nil
	}
	return StatusStopped, nil
}

// GetStderr 获取进程的stderr输出（用于错误诊断）
func (m *FFmpegProcessManager) GetStderr(streamName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buf, exists := m.stderrBuffers[streamName]
	if !exists || buf == nil {
		return ""
	}

	// 返回最后512字节的错误信息（通常足够诊断问题）
	content := buf.String()
	if len(content) > 512 {
		return content[len(content)-512:]
	}
	return content
}

// monitorProcess 监控进程
func (m *FFmpegProcessManager) monitorProcess(streamName string, cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查进程是否还在map中（可能已经被StopProcess移除了）
	existingCmd, exists := m.processes[streamName]
	if !exists {
		// 进程已经被StopProcess移除，不需要再次处理
		return
	}

	// 确认是同一个进程（防止并发问题）
	if existingCmd != cmd {
		return
	}

	// 进程已退出，从map中移除
	delete(m.processes, streamName)

	// 清理stderr缓冲区
	delete(m.stderrBuffers, streamName)

	// 检查错误类型，忽略"no child processes"错误（这通常发生在进程已经被停止后）
	if err != nil {
		errStr := err.Error()
		// 如果错误是"no child processes"，说明进程已经被StopProcess停止，这是正常的
		if errStr == "wait: no child processes" || errStr == "no child processes" {
			m.logger.Info("ffmpeg process was stopped",
				"stream_name", streamName)
		} else {
			m.logger.Error("ffmpeg process exited with error",
				"stream_name", streamName,
				"error", err)
		}
	} else {
		m.logger.Info("ffmpeg process exited normally",
			"stream_name", streamName)
	}
}

// GetVideoFileList 获取视频文件列表（用于选择视频文件）
func GetVideoFileList(vodDir string) ([]VideoFile, error) {
	var files []VideoFile

	err := filepath.Walk(vodDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续遍历
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".flv", ".wmv", ".m4v", ".3gp", ".3gpp"}
		for _, vext := range videoExts {
			if ext == vext {
				files = append(files, VideoFile{
					Name: info.Name(),
					Path: path,
					Size: info.Size(),
				})
				break
			}
		}

		return nil
	})

	return files, err
}

// VideoFile 视频文件信息
type VideoFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}
