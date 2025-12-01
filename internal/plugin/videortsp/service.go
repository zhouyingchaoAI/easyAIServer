package videortsp

import (
	"context"
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
	"log/slog"
	"strings"
	"sync"
)

var (
	globalService *Core
	globalMu      sync.RWMutex
)

// GetGlobal 获取全局服务实例
func GetGlobal() *Core {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalService
}

// InitService 初始化服务
func InitService(cfg *conf.Bootstrap, logger *slog.Logger) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalService != nil {
		return nil // 已经初始化
	}

	// 获取RTSP播放地址（用于生成播放URL）
	rtspHost := "127.0.0.1:15544" // 默认值
	if cfg.RtspConfig.Addr != "" {
		// 使用 RtspConfig (lal的配置，字符串格式如 ":15544")
		addr := cfg.RtspConfig.Addr
		if strings.HasPrefix(addr, ":") {
			addr = "127.0.0.1" + addr
		} else if !strings.Contains(addr, ":") {
			addr = "127.0.0.1:" + addr
		}
		rtspHost = addr
		logger.Info("RTSP config found", "addr", cfg.RtspConfig.Addr, "enabled", cfg.RtspConfig.Enable, "resolved_host", rtspHost)
	} else {
		logger.Warn("RTSP config addr is empty, using default", "default_host", rtspHost)
	}

	// 检查RTSP服务是否启用
	if !cfg.RtspConfig.Enable {
		logger.Warn("RTSP service is disabled in config, RTSP streams will not be available")
	}

	// 获取RTMP推流服务器地址
	rtmpHost := "127.0.0.1:1935" // 默认值（标准RTMP端口）
	if cfg.RtmpConfig.Addr != "" {
		// 使用 RtmpConfig (lal的配置，字符串格式如 ":1935" 或 ":21935")
		addr := cfg.RtmpConfig.Addr
		if strings.HasPrefix(addr, ":") {
			addr = "127.0.0.1" + addr
		} else if !strings.Contains(addr, ":") {
			addr = "127.0.0.1:" + addr
		}
		rtmpHost = addr
		logger.Info("RTMP config found", "addr", cfg.RtmpConfig.Addr, "enabled", cfg.RtmpConfig.Enable, "resolved_host", rtmpHost)
	} else {
		logger.Warn("RTMP config addr is empty, using default", "default_host", rtmpHost)
	}

	// 检查RTMP服务是否启用
	if !cfg.RtmpConfig.Enable {
		logger.Error("RTMP service is disabled in config, video-to-RTSP streaming will not work! Please enable RTMP in config.toml")
	}

	// 创建存储
	store := NewStore(data.GetDatabase())
	
	// 自动迁移数据库
	if err := store.AutoMigrate(); err != nil {
		logger.Error("failed to auto migrate video rtsp tables", "error", err)
		return err
	}

	// 创建核心服务
	core := NewCore(store, rtspHost, rtmpHost, logger)

	// 保存全局实例
	globalService = core

	// 启动所有启用的流
	ctx := context.Background()
	if err := core.Init(ctx); err != nil {
		logger.Error("failed to init video rtsp service", "error", err)
		// 不返回错误，允许服务继续运行
	}

	logger.Info("video rtsp service initialized", "rtsp_host", rtspHost, "rtmp_host", rtmpHost)
	return nil
}

// StopService 停止服务
func StopService() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalService != nil {
		globalService.Cleanup()
		globalService = nil
	}
}

