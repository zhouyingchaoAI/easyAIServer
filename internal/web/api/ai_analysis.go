package api

import (
	"easydarwin/internal/conf"
	"easydarwin/internal/data"
	"easydarwin/internal/data/model"
	"easydarwin/internal/plugin/aianalysis"
	"log/slog"

	"github.com/gin-gonic/gin"
)

// registerAIAnalysisAPI 注册AI分析相关API
func registerAIAnalysisAPI(g gin.IRouter) {
	ai := g.Group("/ai_analysis")

	// 算法服务注册
	ai.POST("/register", func(c *gin.Context) {
		var service conf.AlgorithmService
		if err := c.ShouldBindJSON(&service); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		registry := srv.GetRegistry()
		if registry == nil {
			c.JSON(500, gin.H{"error": "registry not ready"})
			return
		}

		if err := registry.Register(service); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"ok": true, "service_id": service.ServiceID})
	})

	// 算法服务注销
	ai.DELETE("/unregister/:id", func(c *gin.Context) {
		serviceID := c.Param("id")

		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		registry := srv.GetRegistry()
		if registry == nil {
			c.JSON(500, gin.H{"error": "registry not ready"})
			return
		}

		if err := registry.Unregister(serviceID); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	// 算法服务心跳
	ai.POST("/heartbeat/:id", func(c *gin.Context) {
		serviceID := c.Param("id")

		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		registry := srv.GetRegistry()
		if registry == nil {
			c.JSON(500, gin.H{"error": "registry not ready"})
			return
		}

		if err := registry.Heartbeat(serviceID); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	// 查询注册的算法服务
	ai.GET("/services", func(c *gin.Context) {
		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		registry := srv.GetRegistry()
		if registry == nil {
			c.JSON(500, gin.H{"error": "registry not ready"})
			return
		}

		services := registry.ListAllServices()
		c.JSON(200, gin.H{"services": services, "total": len(services)})
	})

	// 获取推理统计信息
	ai.GET("/inference_stats", func(c *gin.Context) {
		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		stats := srv.GetInferenceStats()
		c.JSON(200, stats)
	})

	// 重置推理统计信息
	ai.POST("/inference_stats/reset", func(c *gin.Context) {
		srv := aianalysis.GetGlobal()
		if srv == nil {
			c.JSON(500, gin.H{"error": "AI analysis service not ready"})
			return
		}

		// 记录操作日志
		slog.Warn("inference stats reset requested",
			slog.String("remote_addr", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()))

		if err := srv.ResetInferenceStats(); err != nil {
			slog.Error("failed to reset inference stats",
				slog.String("err", err.Error()))
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		slog.Info("inference stats reset successfully",
			slog.String("remote_addr", c.ClientIP()))

		c.JSON(200, gin.H{"ok": true, "message": "推理统计数据已清零"})
	})
}

// registerAlertAPI 注册告警相关API
func registerAlertAPI(g gin.IRouter) {
	alerts := g.Group("/alerts")

	// 查询告警列表
	alerts.GET("", func(c *gin.Context) {
		var filter model.AlertFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		list, total, err := data.ListAlerts(filter)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"items": list, "total": total})
	})

	// 获取告警详情
	alerts.GET("/:id", func(c *gin.Context) {
		var id uint
		if err := c.ShouldBindUri(&struct {
			ID uint `uri:"id" binding:"required"`
		}{ID: id}); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		alert, err := data.GetAlertByID(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "alert not found"})
			return
		}

		c.JSON(200, gin.H{"alert": alert})
	})

	// 删除告警
	alerts.DELETE("/:id", func(c *gin.Context) {
		var uriParam struct {
			ID uint `uri:"id" binding:"required"`
		}
		if err := c.ShouldBindUri(&uriParam); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if err := data.DeleteAlert(uriParam.ID); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})
	
	// 批量删除告警
	alerts.POST("/batch_delete", func(c *gin.Context) {
		var req struct {
			IDs []uint `json:"ids" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if len(req.IDs) == 0 {
			c.JSON(400, gin.H{"error": "ids cannot be empty"})
			return
		}

		// 批量删除
		successCount, err := data.BatchDeleteAlerts(req.IDs)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error(), "success_count": successCount})
			return
		}

		c.JSON(200, gin.H{"ok": true, "deleted_count": successCount})
	})
	
	// 获取所有任务ID列表
	alerts.GET("/task_ids", func(c *gin.Context) {
		taskIDs, err := data.GetDistinctTaskIDs()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"task_ids": taskIDs})
	})
}

