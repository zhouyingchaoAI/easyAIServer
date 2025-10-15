// Package api Copyright 2025 EasyDarwin.
// http://www.easydarwin.org
// 路由的入口
// History (ID, Time, Desc)
// (xukongzangpusa, 20250424, 所有的路由迁移到此文件中，方便后期管理)
package api

import (
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
    "easydarwin/internal/plugin/frameextractor"
	"easydarwin/internal/gutils/consts"
	"easydarwin/utils/pkg/web"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"runtime/debug"
)

var gCfg *conf.Bootstrap

func setupRouter(router *gin.Engine, uc *conf.Bootstrap) {

	gCfg = uc

	router.Use(
		// 格式化输出到控制台，然后记录到日志
		// 此处不做 recover，底层 http.server 也会 recover，但不会输出方便查看的格式
		gin.CustomRecovery(func(c *gin.Context, err any) {
			slog.Error("panic", "err", err, "stack", string(debug.Stack()))
			c.AbortWithStatus(http.StatusInternalServerError)
		}),
		//web.Mertics(),
		web.Logger(slog.Default(), func(_ *gin.Context) bool {
			// true:记录请求响应报文
			return uc.Debug
		}),
	)
	path := "/api/v1"
	r := router.Group(path)
	registerApp(r)
	//registerConfig(r, ConfigAPI{cfg: uc.Conf, uc: uc, app: app}, auth)
	//registerVersion(r, uc.Version, auth)
	registerLiveStream(r)
	registerReverseProxy(router)
	registerVod(router, r)
}

func registerApp(g gin.IRouter) {
	l := login{
		database: data.GetDatabase(),
	}
	g.GET("/version", getVersion)
	g.POST("/login", web.WarpH(l.Login))
	g.POST("/logout", l.logout)

	users := g.Group("/users")
	users.PUT("/:username/reset-password", l.resetPassword)

	// frame extractor manage
	fem := g.Group("/frame_extractor")
	// get config
	fem.GET("/config", func(c *gin.Context) {
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		cfg := fx.GetConfig()
		slog.Info("returning config", 
			slog.Bool("enable", cfg.Enable),
			slog.String("store", cfg.Store),
			slog.Int("interval_ms", cfg.IntervalMs),
			slog.String("minio_endpoint", cfg.MinIO.Endpoint),
			slog.String("minio_bucket", cfg.MinIO.Bucket))
		c.JSON(200, cfg)
	})
	// update config
	fem.POST("/config", func(c *gin.Context) {
		var in conf.FrameExtractorConfig
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.UpdateConfig(&in); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
    fem.GET("/tasks", func(c *gin.Context) {
        fx := frameextractor.GetGlobal()
        if fx == nil {
            c.JSON(200, gin.H{"items": []any{}, "total": 0})
            return
        }
        tasks := fx.ListTasks()
        c.JSON(200, gin.H{"items": tasks, "total": len(tasks)})
    })
    fem.POST("/tasks", func(c *gin.Context) {
        var in conf.FrameExtractTask
        if err := c.ShouldBindJSON(&in); err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }
        fx := frameextractor.GetGlobal()
        if fx == nil {
            c.JSON(500, gin.H{"error": "service not ready"})
            return
        }
        if err := fx.AddTask(in); err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }
        c.JSON(200, gin.H{"ok": true})
    })
    fem.DELETE("/tasks/:id", func(c *gin.Context) {
        id := c.Param("id")
        fx := frameextractor.GetGlobal()
        if fx == nil {
            c.JSON(500, gin.H{"error": "service not ready"})
            return
        }
        ok := fx.RemoveTask(id)
        c.JSON(200, gin.H{"ok": ok})
    })
	// list snapshots for a task
	fem.GET("/snapshots/:task_id", func(c *gin.Context) {
		taskID := c.Param("task_id")
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		snapshots, err := fx.ListSnapshots(taskID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"items": snapshots, "total": len(snapshots)})
	})
	// delete a snapshot
	fem.DELETE("/snapshots/:task_id/*path", func(c *gin.Context) {
		taskID := c.Param("task_id")
		path := c.Param("path")
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.DeleteSnapshot(taskID, path); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	// batch delete snapshots
	fem.POST("/snapshots/:task_id/batch_delete", func(c *gin.Context) {
		taskID := c.Param("task_id")
		var req struct {
			Paths []string `json:"paths"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.DeleteSnapshots(taskID, req.Paths); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true, "deleted": len(req.Paths)})
	})
	// start a task
	fem.POST("/tasks/:id/start", func(c *gin.Context) {
		id := c.Param("id")
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.StartTaskByID(id); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	// stop a task
	fem.POST("/tasks/:id/stop", func(c *gin.Context) {
		id := c.Param("id")
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.StopTaskByID(id); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	// update task interval
	fem.PUT("/tasks/:id/interval", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			IntervalMs int `json:"interval_ms"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		if err := fx.UpdateTaskInterval(id, req.IntervalMs); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	// get task status
	fem.GET("/tasks/:id/status", func(c *gin.Context) {
		id := c.Param("id")
		fx := frameextractor.GetGlobal()
		if fx == nil {
			c.JSON(500, gin.H{"error": "service not ready"})
			return
		}
		running := fx.GetTaskStatus(id)
		c.JSON(200, gin.H{"running": running})
	})
}

func registerLiveStream(r gin.IRouter) {
	l := LiveStreamAPI{
		database: data.GetDatabase(),
	}
	{
		r.Any("/push/on_pub_start", l.pubStart)
		r.Any("/push/on_pub_stop", l.pubStop)
		r.Any("/push/on_rtmp_connect", l.pubRtmpConnect)
	}
	{
		group := r.Group("/live")
		group.GET("", l.find)
		group.GET("/info/:id", l.findInfo)
		group.GET("/playurl/:id", l.getPlayUrl)

		group.GET("/play/start/:id", l.playStart)
		group.GET("/play/stop/:id", l.playStop)
		group.GET("/stream/info/:id", l.StreamInfo)
		group.DELETE("/:id", l.delete)

		pull := group.Group("/pull")
		pull.POST("", l.createPull) // 创建
		pull.PUT(":id", l.updatePull)
		pull.PUT(":id/:type/:value", l.updateOnePull)

		push := group.Group("/push")
		push.POST("", l.createPush) // 创建
		push.PUT(":id", l.updatePush)
		push.PUT(":id/:type/:value", l.updateOnePush)
	}
}

func registerReverseProxy(r gin.IRouter) {
	r.Group("/flv").GET("/*path", FlvHandler())
	r.Group("/ws_flv").GET("/*path", WSFlvHandler())
	r.Group("/ts").Any("/*path", HlsHandler())
}

func registerVod(root, r gin.IRouter) {

	initVod()

	// 将文件夹设置为可访问
	root.Group(consts.RouteStaticVOD).GET("/*filepath",
		static.Serve(consts.RouteStaticVOD, static.LocalFile(gCfg.VodConfig.Dir, false)))
	root.Group(consts.RouteStaticVOD).HEAD("/*filepath",
		static.Serve(consts.RouteStaticVOD, static.LocalFile(gCfg.VodConfig.Dir, false)))

	vod := r.Group("/vod")
	{
		vod.Use()
		vod.GET("/accept", gVodRouter.accept)
		vod.OPTIONS("/upload", gVodRouter.uploadoptions)
		vod.POST("/upload", gVodRouter.upload)

		vod.GET("/progress", gVodRouter.progress)
		vod.POST("/progress", gVodRouter.progress)
		vod.POST("/retran", gVodRouter.retran)

		vod.GET("/list", gVodRouter.list)
		vod.POST("/list", gVodRouter.list)
		vod.GET("/get", gVodRouter.get)
		vod.POST("/get", gVodRouter.get)
		vod.POST("/save", gVodRouter.save)
		vod.GET("/snap", gVodRouter.snap)
		vod.POST("/snap", gVodRouter.snap)

		vod.GET("/turn/shared", gVodRouter.shared)
		vod.POST("/turn/shared", gVodRouter.shared)
		vod.GET("/sharelist", gVodRouter.sharelist)
		vod.POST("/sharelist", gVodRouter.sharelist)

		vod.POST("/remove", gVodRouter.remove)
		vod.POST("/removeBatch", gVodRouter.removeBatch)
		vod.GET("/download/:id", gVodRouter.download)
	}
}
