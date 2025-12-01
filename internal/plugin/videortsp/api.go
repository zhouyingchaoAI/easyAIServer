package videortsp

import (
	"easydarwin/utils/pkg/web"
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// API 处理器
type API struct {
	core   *Core
	logger *slog.Logger
}

// NewAPI 创建API处理器
func NewAPI(core *Core, logger *slog.Logger) *API {
	return &API{
		core:   core,
		logger: logger,
	}
}

// RegisterRoutes 注册路由
func RegisterRoutes(router *gin.RouterGroup, api *API) {
	streams := router.Group("/video_rtsp")
	{
		streams.POST("", web.WarpH(api.CreateTask))
		streams.GET("", api.ListTasksHandler)
		streams.GET("/:id", api.GetTaskHandler)
		streams.PUT("/:id", web.WarpH(api.UpdateTask))
		streams.DELETE("/:id", api.DeleteTaskHandler)
		streams.POST("/:id/start", api.StartStreamHandler)
		streams.POST("/:id/stop", api.StopStreamHandler)
		streams.GET("/:id/ffmpeg_command", api.GetFFmpegCommandHandler)
		streams.GET("/files", api.ListVideoFilesHandler)
	}
}

// CreateTask 创建任务
func (a *API) CreateTask(c *gin.Context, in *CreateTaskInput) (any, error) {
	// 记录接收到的数据
	a.logger.Info("creating stream task",
		"name", in.Name,
		"stream_name", in.StreamName,
		"video_path", in.VideoPath,
		"enabled", in.Enabled,
		"loop", in.Loop)
	
	if in.VideoPath == "" {
		return nil, web.ErrBadRequest.With("video path is required")
	}
	
	task, err := a.core.CreateTask(*in)
	if err != nil {
		a.logger.Error("failed to create task", "error", err, "input", in)
		return nil, web.ErrBadRequest.With(err.Error())
	}
	return gin.H{
		"code": http.StatusOK,
		"msg":  "创建成功",
		"data": task,
	}, nil
}

// UpdateTask 更新任务
func (a *API) UpdateTask(c *gin.Context, in *UpdateTaskInput) (any, error) {
	id := c.Param("id")
	task, err := a.core.UpdateTask(id, *in)
	if err != nil {
		return nil, web.ErrBadRequest.With(err.Error())
	}
	return gin.H{
		"code": http.StatusOK,
		"msg":  "更新成功",
		"data": task,
	}, nil
}

// DeleteTaskHandler 删除任务处理器
func (a *API) DeleteTaskHandler(c *gin.Context) {
	id := c.Param("id")
	err := a.core.DeleteTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "删除成功"})
}

// StartStreamHandler 启动流处理器
func (a *API) StartStreamHandler(c *gin.Context) {
	id := c.Param("id")
	err := a.core.StartStream(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "启动成功"})
}

// StopStreamHandler 停止流处理器
func (a *API) StopStreamHandler(c *gin.Context) {
	id := c.Param("id")
	err := a.core.StopStream(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "停止成功"})
}

// GetTaskHandler 获取任务处理器
func (a *API) GetTaskHandler(c *gin.Context) {
	id := c.Param("id")
	task, err := a.core.GetTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": task})
}

// ListTasksHandler 列出任务处理器
func (a *API) ListTasksHandler(c *gin.Context) {
	// 解析分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("start", "0"))
	search := c.DefaultQuery("q", "")

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	tasks, total, err := a.core.ListTasks(limit, offset, search)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": gin.H{
			"rows":  tasks,
			"total": total,
		},
	})
}

// ListVideoFilesHandler 列出视频文件处理器
func (a *API) ListVideoFilesHandler(c *gin.Context) {
	vodDir := c.DefaultQuery("dir", "")
	if vodDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "dir parameter is required"})
		return
	}

	files, err := GetVideoFileList(vodDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": files})
}

// GetFFmpegCommandHandler 获取FFmpeg命令（用于调试）
func (a *API) GetFFmpegCommandHandler(c *gin.Context) {
	id := c.Param("id")
	task, err := a.core.GetTask(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	// 构建RTMP URL
	rtmpURL := fmt.Sprintf("rtmp://%s/live/%s", a.core.GetRTMPHost(), task.StreamName)
	
	// 获取FFmpeg命令
	ffmpegPath, args, err := a.core.processManager.BuildFFmpegCommand(task, rtmpURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	// 构建完整命令字符串
	cmdStr := ffmpegPath + " " + strings.Join(args, " ")
	
	// 构建格式化的命令（每行一个参数）
	formattedCmd := ffmpegPath
	for _, arg := range args {
		formattedCmd += " \\\n  " + arg
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": gin.H{
			"ffmpegPath":    ffmpegPath,
			"rtmpUrl":       rtmpURL,
			"command":       cmdStr,
			"formattedCmd": formattedCmd,
			"args":          args,
			"argsCount":     len(args),
		},
	})
}

