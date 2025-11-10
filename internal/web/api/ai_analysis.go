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
	
	// 清空所有算法服务（用于清理测试数据或重置）
	ai.POST("/clear_all", func(c *gin.Context) {
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

		count := registry.ClearAllServices()
		
		slog.Warn("algorithm services cleared by API request",
			slog.String("remote_addr", c.ClientIP()),
			slog.Int("cleared_count", count))

		c.JSON(200, gin.H{"ok": true, "cleared_count": count, "message": "所有算法服务已清空"})
	})

	// 算法服务心跳（支持按ServiceID或Endpoint更新，可选携带性能统计）
	ai.POST("/heartbeat/:id", func(c *gin.Context) {
		id := c.Param("id")

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

		// 解析心跳请求体（可选的性能统计数据）
		var heartbeatReq conf.HeartbeatRequest
		if err := c.ShouldBindJSON(&heartbeatReq); err != nil {
			// 如果没有请求体或解析失败，当作普通心跳处理（向后兼容）
			heartbeatReq = conf.HeartbeatRequest{}
		}

		// 尝试按ServiceID更新心跳
		err := registry.HeartbeatWithStats(id, &heartbeatReq)
		if err != nil {
			// 如果失败，尝试按Endpoint更新
			err = registry.HeartbeatByEndpointWithStats(id, &heartbeatReq)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
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

		// 获取所有服务实例（按endpoint去重 - 一个endpoint代表一个物理服务实例）
		allServices := registry.ListAllServiceInstances()
		serviceStats := make([]aianalysis.ServiceStat, len(allServices))
		for i, svc := range allServices {
			serviceStats[i] = aianalysis.ServiceStat{
				ServiceID:     svc.ServiceID,
				Name:          svc.Name,
				Endpoint:      svc.Endpoint,
				Version:       svc.Version,
				TaskTypes:     svc.TaskTypes,
				CallCount:     registry.GetCallCount(svc.Endpoint), // 使用endpoint作为key
				LastHeartbeat: svc.LastHeartbeat,
				RegisterAt:    svc.RegisterAt,
			}
		}
		
		// 添加调试日志：记录实际有多少不同的endpoint
		slog.Info("listing algorithm services",
			slog.Int("unique_endpoints", len(serviceStats)),
			slog.Int("returned_count", len(allServices)))
		
		c.JSON(200, gin.H{"services": serviceStats, "total": len(serviceStats)})
	})
	
	// 获取负载均衡信息
	ai.GET("/load_balance/info", func(c *gin.Context) {
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

		// 获取所有任务类型的负载均衡信息
		info := registry.GetAllLoadBalanceInfo()
		
		c.JSON(200, gin.H{
			"load_balance": info,
			"total_task_types": len(info),
		})
	})
	
	// 调试接口：查看所有服务详情（不去重，显示内部存储的所有记录）
	ai.GET("/services/debug", func(c *gin.Context) {
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

		// 获取所有任务类型的服务
		debugInfo := make(map[string][]aianalysis.ServiceStat)
		totalRecords := 0
		
		for _, taskType := range []string{"人数统计", "客流分析", "人头检测", "绊线人数统计", "人员跌倒", "人员离岗", "吸烟检测", "区域入侵", "徘徊检测", "物品遗留", "安全帽检测"} {
			services := registry.GetAlgorithms(taskType)
			if len(services) > 0 {
				stats := make([]aianalysis.ServiceStat, len(services))
				for i, svc := range services {
					stats[i] = aianalysis.ServiceStat{
						ServiceID:     svc.ServiceID,
						Name:          svc.Name,
						Endpoint:      svc.Endpoint,
						Version:       svc.Version,
						TaskTypes:     svc.TaskTypes,
						CallCount:     registry.GetCallCount(svc.Endpoint),
						LastHeartbeat: svc.LastHeartbeat,
						RegisterAt:    svc.RegisterAt,
					}
				}
				debugInfo[taskType] = stats
				totalRecords += len(services)
			}
		}
		
		c.JSON(200, gin.H{
			"task_types":    debugInfo,
			"total_records": totalRecords,
			"unique_endpoints": len(registry.ListAllServiceInstances()),
		})
	})
	
	// 负载均衡分析接口
	ai.GET("/load_balance/analysis", func(c *gin.Context) {
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

		// 按任务类型统计负载分布
		analysis := make(map[string]interface{})
		
		for _, taskType := range []string{"人数统计", "客流分析", "人头检测", "绊线人数统计"} {
			services := registry.GetAlgorithms(taskType)
			if len(services) == 0 {
				continue
			}
			
			// 统计每个服务的调用次数
			serviceStats := make([]map[string]interface{}, 0)
			totalCalls := 0
			minCalls := -1
			maxCalls := 0
			
			for _, svc := range services {
				callCount := registry.GetCallCount(svc.Endpoint)
				totalCalls += callCount
				
				if minCalls == -1 || callCount < minCalls {
					minCalls = callCount
				}
				if callCount > maxCalls {
					maxCalls = callCount
				}
				
				serviceStats = append(serviceStats, map[string]interface{}{
					"service_id": svc.ServiceID,
					"endpoint":   svc.Endpoint,
					"call_count": callCount,
				})
			}
			
			// 计算负载均衡度（方差）
			avgCalls := 0.0
			if len(services) > 0 {
				avgCalls = float64(totalCalls) / float64(len(services))
			}
			
			variance := 0.0
			for _, svc := range services {
				callCount := float64(registry.GetCallCount(svc.Endpoint))
				diff := callCount - avgCalls
				variance += diff * diff
			}
			if len(services) > 0 {
				variance /= float64(len(services))
			}
			
			// 负载均衡质量评估
			balanceQuality := "excellent"
			if len(services) > 1 {
				diff := maxCalls - minCalls
				if diff > int(avgCalls*0.5) {
					balanceQuality = "poor"
				} else if diff > int(avgCalls*0.2) {
					balanceQuality = "fair"
				} else {
					balanceQuality = "good"
				}
			}
			
			analysis[taskType] = map[string]interface{}{
				"service_count":    len(services),
				"total_calls":      totalCalls,
				"avg_calls":        avgCalls,
				"min_calls":        minCalls,
				"max_calls":        maxCalls,
				"variance":         variance,
				"balance_quality":  balanceQuality,
				"services":         serviceStats,
			}
		}
		
		c.JSON(200, gin.H{"analysis": analysis})
	})
	
	// 获取指定任务类型的服务统计
	ai.GET("/services/stats/:task_type", func(c *gin.Context) {
		taskType := c.Param("task_type")
		
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

		stats := registry.GetServiceStats(taskType)
		c.JSON(200, gin.H{"services": stats, "total": len(stats), "task_type": taskType})
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
		if err != nil{
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// 为告警动态生成预签名URL（按需生成，不阻塞推理）
		srv := aianalysis.GetGlobal()
		if srv != nil {
			for i := range list {
				if list[i].ImageURL == "" && list[i].ImagePath != "" {
					url, err := srv.GeneratePresignedURL(list[i].ImagePath)
					if err == nil {
						list[i].ImageURL = url
					} else {
						slog.Warn("failed to generate presigned URL for alert",
							slog.Uint64("alert_id", uint64(list[i].ID)),
							slog.String("image_path", list[i].ImagePath),
							slog.String("err", err.Error()))
					}
				}
			}
		}

		c.JSON(200, gin.H{"items": list, "total": total})
	})

	// 获取告警详情
	alerts.GET("/:id", func(c *gin.Context) {
		var uriParam struct {
			ID uint `uri:"id" binding:"required"`
		}
		if err := c.ShouldBindUri(&uriParam); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		alert, err := data.GetAlertByID(uriParam.ID)
		if err != nil {
			c.JSON(404, gin.H{"error": "alert not found"})
			return
		}

		// 为告警动态生成预签名URL
		srv := aianalysis.GetGlobal()
		if srv != nil && alert.ImageURL == "" && alert.ImagePath != "" {
			url, err := srv.GeneratePresignedURL(alert.ImagePath)
			if err == nil {
				alert.ImageURL = url
			} else {
				slog.Warn("failed to generate presigned URL for alert",
					slog.Uint64("alert_id", uint64(alert.ID)),
					slog.String("image_path", alert.ImagePath),
					slog.String("err", err.Error()))
			}
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

