package main

import (
	"easydarwin/internal/conf"
	"easydarwin/internal/core/source"
	"easydarwin/internal/data"
	"easydarwin/internal/plugin/videortsp"
	"easydarwin/internal/web/api"
	"fmt"
	"log/slog"
	"net/http"
)

func wireApp(cfg *conf.Bootstrap) (http.Handler, error) {
	db, err := data.SetupDB(cfg)
	if err != nil {
		return nil, err
	}

	// 消除 sqlite 空闲页，防止数据库过大
	db.Exec("VACUUM;")

	liveStreamcore := api.NewLiveStream(db)
	api.NewUserCore(db)

	api.NewVodCore(db)

	source.InitDb(liveStreamcore)
	
	// 初始化视频转RTSP流插件
	// logger will be initialized later, use nil for now
	logger := slog.Default()
	if err := videortsp.InitService(cfg, logger); err != nil {
		return nil, fmt.Errorf("failed to init video rtsp service: %w", err)
	}
	
	handler := api.NewHTTPHandler(cfg)
	if handler == nil {
		return nil, fmt.Errorf("handle is nil")
	}
	data.SetConfig(cfg)
	return handler, nil
}
